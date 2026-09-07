// Command jfs-router 是一个轻量级的「超级挂载」路由层：对应用暴露
// 单一挂载点，内部按路径前缀（顶层目录名）把读写转发给对应的
// JuiceFS（或任意 POSIX 目录）实例。路由表可以来自本地 JSON 文件，
// 也可以来自 Redis 或 MySQL，通过客户端缓存 + TTL 感知外部变更。
//
// 使用方式:
//
//	jfs-router -config /etc/jfs-router/config.json
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
	"jfs-router/internal/metastore"
	"jfs-router/internal/router"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("jfs-router %s (built %s)\n", version, buildTime)
		return
	}
	if err := run(); err != nil {
		log.Fatalf("jfs-router 启动失败: %v", err)
	}
}

// version/buildTime 由 Makefile 里的 `go build -ldflags "-X main.version=... -X main.buildTime=..."`
// 注入，本地直接 `go build` 不传 ldflags 时保持这两个默认值。
var (
	version   = "dev"
	buildTime = "unknown"
)

func run() error {
	configPath := flag.String("config", "", "配置文件路径（JSON），必填")
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := buildStore(ctx, cfg.MetaStore)
	if err != nil {
		return fmt.Errorf("初始化元数据源失败: %w", err)
	}
	cachedStore := metastore.NewCachedStore(store, cfg.MetastoreCacheConfig())

	if routes, err := cachedStore.List(ctx); err != nil {
		log.Printf("警告：启动时探测路由表失败（不影响启动，运行期会重试）: %v", err)
	} else {
		log.Printf("元数据源类型=%s，当前共 %d 条路由：", cfg.MetaStore.Type, len(routes))
		for seg, r := range routes {
			log.Printf("  %-20s -> %s", seg, r.Backend)
		}
	}

	root := router.NewRoot(cachedStore)

	entryTimeout := cfg.EntryTimeout()
	attrTimeout := cfg.AttrTimeout()
	opts := &fs.Options{
		EntryTimeout: &entryTimeout,
		AttrTimeout:  &attrTimeout,
		MountOptions: fuse.MountOptions{
			AllowOther:    cfg.AllowOther,
			FsName:        cfg.FsName,
			Name:          "jfs-router",
			Debug:         cfg.Debug,
			MaxBackground: 64,
		},
	}

	server, err := fs.Mount(cfg.Mountpoint, root, opts)
	if err != nil {
		cachedStore.Close()
		return fmt.Errorf("挂载 %s 失败: %w", cfg.Mountpoint, err)
	}
	log.Printf("已挂载: %s（元数据源=%s，路由结果缓存 TTL=%v）",
		cfg.Mountpoint, cfg.MetaStore.Type, cfg.MetastoreCacheConfig().TTL)

	waitForShutdown(server, cfg.Mountpoint)
	if err := cachedStore.Close(); err != nil {
		log.Printf("关闭元数据源连接时出错: %v", err)
	}
	return nil
}

// buildStore 根据配置里的 metastore.type 构造对应的底层 Store 实现。
// 新增一种元数据引擎时，只需要在 internal/metastore 里新增一个实现、
// 在这里加一个 case 分支——路由层（internal/router）完全不需要改动。
func buildStore(ctx context.Context, cfg config.MetaStoreConfig) (metastore.Store, error) {
	switch cfg.Type {
	case "file":
		return metastore.NewFileStore(cfg.File.Path)
	case "redis":
		return metastore.NewRedisStore(ctx, metastore.RedisConfig{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			Key:      cfg.Redis.Key,
		})
	case "mysql":
		return metastore.NewMySQLStore(ctx, metastore.MySQLConfig{
			DSN:           cfg.MySQL.DSN,
			Table:         cfg.MySQL.Table,
			SegmentColumn: cfg.MySQL.SegmentColumn,
			BackendColumn: cfg.MySQL.BackendColumn,
			MaxOpenConns:  cfg.MySQL.MaxOpenConns,
			MaxIdleConns:  cfg.MySQL.MaxIdleConns,
		})
	default:
		return nil, fmt.Errorf("不支持的 metastore.type: %q", cfg.Type)
	}
}

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

func waitForShutdown(server *fuse.Server, mountpoint string) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		server.Wait()
		close(done)
	}()

	select {
	case sig := <-sigCh:
		log.Printf("收到信号 %v，开始卸载 %s ...", sig, mountpoint)
		if err := unmountWithRetry(server); err != nil {
			log.Printf("卸载失败: %v（可能仍有进程占用该挂载点，可手动执行: fusermount -u %s）", err, mountpoint)
		}
		<-done
	case <-done:
		log.Printf("挂载点 %s 已被卸载，进程退出", mountpoint)
	}
}

func unmountWithRetry(server *fuse.Server) error {
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
