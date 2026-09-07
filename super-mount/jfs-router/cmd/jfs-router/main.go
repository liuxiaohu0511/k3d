// Command jfs-router 是一个轻量级的「超级挂载」路由层：对应用暴露
// 单一挂载点，内部按路径前缀把读写转发给对应的 JuiceFS（或任意 POSIX
// 目录）实例。
//
// 使用方式:
//
//	jfs-router -config /etc/jfs-router/config.json
//
// 配置文件格式见 configs/config.example.json，字段说明见
// internal/config/config.go 中的注释。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"jfs-router/internal/config"
	"jfs-router/internal/router"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("jfs-router 启动失败: %v", err)
	}
}

func run() error {
	var (
		configPath = flag.String("config", "", "配置文件路径（JSON），必填")
		verbose    = flag.Bool("v", false, "打印路由表等启动信息")
	)
	flag.Parse()

	if *configPath == "" {
		flag.Usage()
		return fmt.Errorf("必须通过 -config 指定配置文件")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("配置校验未通过: %w", err)
	}

	if err := prepareMountpoint(cfg.Mountpoint); err != nil {
		return err
	}

	if *verbose || true { // 路由表信息不敏感，默认也打印，便于运维核对
		log.Printf("即将挂载 %s，路由表如下（共 %d 条）：", cfg.Mountpoint, len(cfg.Mounts))
		for _, m := range cfg.SortedMounts() {
			log.Printf("  %-30s -> %s", m.Prefix, m.Backend)
		}
	}

	root := router.NewRoot(cfg.Mounts)

	fsName := cfg.FsName
	if fsName == "" {
		fsName = "jfs-router"
	}

	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: cfg.AllowOther,
			FsName:     fsName,
			Name:       "jfs-router",
			Debug:      cfg.Debug,
			// 轻量路由层几乎没有自己的计算开销，真正的耗时在底层
			// 系统调用上，适当放宽后台并发请求数，避免限流成为瓶颈。
			MaxBackground: 64,
		},
	}

	server, err := fs.Mount(cfg.Mountpoint, root, opts)
	if err != nil {
		return fmt.Errorf("挂载 %s 失败: %w", cfg.Mountpoint, err)
	}
	log.Printf("已挂载: %s", cfg.Mountpoint)

	waitForShutdown(server, cfg.Mountpoint)
	return nil
}

// prepareMountpoint 确保挂载点存在且是「空目录」——FUSE 官方建议挂载点
// 保持为空，避免挂载前该目录下原有内容被临时遮挡后又造成混淆；如果
// 目录不存在则自动创建，方便部署脚本/容器初始化时直接指定一个尚未
// 创建的路径。
func prepareMountpoint(dir string) error {
	st, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return fmt.Errorf("检查挂载点 %q 失败: %w", dir, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("挂载点 %q 已存在但不是目录", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取挂载点 %q 失败: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("挂载点 %q 不是空目录，为避免遮挡已有数据拒绝挂载", dir)
	}
	return nil
}

// waitForShutdown 阻塞直到收到 SIGINT/SIGTERM，收到后尝试优雅卸载。
// 生产环境下由 systemd 等进程管理器发送 SIGTERM 触发这里的清理逻辑。
func waitForShutdown(server *fuse.Server, mountpoint string) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		server.Wait() // 如果挂载点被外部 fusermount -u 卸载，这里也会返回
		close(done)
	}()

	select {
	case sig := <-sigCh:
		log.Printf("收到信号 %v，开始卸载 %s ...", sig, mountpoint)
		if err := unmountWithRetry(server, mountpoint); err != nil {
			log.Printf("卸载失败: %v（可能仍有进程占用该挂载点，可手动执行: fusermount -u %s）", err, mountpoint)
		}
		<-done
	case <-done:
		log.Printf("挂载点 %s 已被卸载，进程退出", mountpoint)
	}
}

// unmountWithRetry 应对「卸载时仍有文件句柄占用」的情况，短暂重试而不是
// 立即失败——这在应用优雅关闭的过程中很常见（应用可能还在 close 最后
// 几个文件描述符）。
func unmountWithRetry(server *fuse.Server, mountpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var lastErr error
	for {
		if err := server.Unmount(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-time.After(300 * time.Millisecond):
		}
	}
}
