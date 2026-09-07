// Package config 定义了 jfs-router 的配置文件结构与校验逻辑。
//
// 设计目标：配置在进程启动时一次性加载并校验，运行期间不支持热更新——
// 这是刻意的简化（见 README「设计取舍」一节）：本工具面向「读写路径前缀
// 固定」的场景，前缀 -> 后端目录的映射在一次挂载生命周期内是静态的。
// 如需变更映射，重启进程（先 umount 再以新配置启动）即可。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// MountEntry 描述一条「虚拟前缀 -> 真实后端目录」的路由规则。
// Backend 通常是某个 juicefs 实例已经挂载好的 POSIX 路径
// （或该路径的某个子目录），例如 juicefs mount ... /mnt/.jfs-a 之后，
// Backend 填 "/mnt/.jfs-a"。
type MountEntry struct {
	// Prefix 是应用视角下的虚拟路径前缀，必须是绝对路径，例如 "/tenantA"。
	// 路由时按最长前缀匹配（在归一化之后，按路径分段整段匹配，
	// 不会出现 "/ab" 意外匹配到前缀 "/a" 的情况）。
	Prefix string `json:"prefix"`

	// Backend 是该前缀实际路由到的本地目录（通常是某个 juicefs 挂载点
	// 或其子目录）。必须已经存在且可 stat，且必须是目录。
	Backend string `json:"backend"`
}

// Config 是整个路由层的启动配置。
type Config struct {
	// Mountpoint 是本工具对外暴露的「超级挂载点」，即应用统一访问的路径。
	Mountpoint string `json:"mountpoint"`

	// FsName / Name 仅用于 df -T 等命令展示，不影响功能。
	FsName string `json:"fs_name"`

	// AllowOther 对应 FUSE 的 allow_other 选项：是否允许其他用户
	// （非发起挂载的用户）访问该挂载点。生产环境如果是多用户/多进程
	// 共享访问，通常需要开启，但需要 /etc/fuse.conf 里也启用
	// user_allow_other，且需要 root 权限挂载。
	AllowOther bool `json:"allow_other"`

	// Debug 打开后会把每个 FUSE 请求打到日志，仅用于排查问题，
	// 生产环境正常运行时应关闭（日志量很大，且有性能开销）。
	Debug bool `json:"debug"`

	// Mounts 是前缀路由表。
	Mounts []MountEntry `json:"mounts"`
}

// Load 从 JSON 文件加载配置。选择 JSON 而非 YAML 是为了不引入额外的
// 第三方依赖（标准库 encoding/json 足够，减少供应链和构建环境的负担）。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var c Config
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields() // 配置里的拼写错误应该在启动时就暴露，而不是被静默忽略
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	return &c, nil
}

// Normalize 对配置做规范化：清理路径中的多余斜杠、"." 等，
// 确保后续前缀匹配是在「干净」的路径上进行。
func (c *Config) Normalize() {
	c.Mountpoint = cleanAbs(c.Mountpoint)
	for i := range c.Mounts {
		c.Mounts[i].Prefix = cleanAbs(c.Mounts[i].Prefix)
		c.Mounts[i].Backend = filepath.Clean(c.Mounts[i].Backend)
	}
}

func cleanAbs(p string) string {
	p = filepath.Clean(p)
	if p == "." {
		p = "/"
	}
	return p
}

// Validate 对配置做「启动即暴露问题」式的严格校验。任何一条规则不满足，
// 都直接返回错误、拒绝启动——因为路由表一旦跑起来就是长期不变的，
// 错误的映射关系（尤其是前缀重叠）在生产环境里排查代价很高，
// 必须在启动阶段就杜绝。
func (c *Config) Validate() error {
	if c.Mountpoint == "" || c.Mountpoint == "/" {
		return fmt.Errorf("mountpoint 未设置或不能是根目录")
	}
	if len(c.Mounts) == 0 {
		return fmt.Errorf("mounts 路由表为空，至少需要一条映射")
	}

	seenPrefix := make(map[string]string) // prefix -> backend，用于查重
	segLists := make([][]string, 0, len(c.Mounts))

	for i, m := range c.Mounts {
		if m.Prefix == "" || !filepath.IsAbs(m.Prefix) {
			return fmt.Errorf("mounts[%d].prefix 必须是绝对路径，当前值: %q", i, m.Prefix)
		}
		if m.Prefix == "/" {
			return fmt.Errorf("mounts[%d].prefix 不允许是根路径 \"/\"（会吞掉整个挂载点，失去路由意义）", i)
		}
		if m.Backend == "" || !filepath.IsAbs(m.Backend) {
			return fmt.Errorf("mounts[%d].backend 必须是绝对路径，当前值: %q", i, m.Backend)
		}
		if prevBackend, dup := seenPrefix[m.Prefix]; dup {
			return fmt.Errorf("前缀 %q 重复配置（分别指向 %q 和 %q），路由表里每个前缀只能出现一次",
				m.Prefix, prevBackend, m.Backend)
		}
		seenPrefix[m.Prefix] = m.Backend

		// 校验 backend 必须存在、必须是目录、必须可读写。
		st, err := os.Stat(m.Backend)
		if err != nil {
			return fmt.Errorf("mounts[%d] 的 backend %q 不可访问: %w（是否对应的 juicefs 实例还没挂载？）", i, m.Backend, err)
		}
		if !st.IsDir() {
			return fmt.Errorf("mounts[%d] 的 backend %q 不是目录", i, m.Backend)
		}
		if err := checkWritable(m.Backend); err != nil {
			return fmt.Errorf("mounts[%d] 的 backend %q 权限检查失败: %w", i, m.Backend, err)
		}

		segLists = append(segLists, splitSegments(m.Prefix))
	}

	// 校验「前缀祖先关系」：任何一个前缀都不能是另一个前缀的路径前缀（按
	// 目录分段比较，而不是简单字符串前缀，避免 "/ab" 与 "/a" 的误判）。
	// 原因：每个前缀对应的后端目录会被当作一整棵子树"过继"给路由层，
	// 一旦某前缀（如 "/a"）成为叶子挂接点，其下所有路径查找都会被对应
	// 的 juicefs 实例接管，不会再回到路由层——因此 "/a/b" 这样的更深前缀
	// 将永远不可达，这是配置错误，必须在启动时拒绝而不是留到运行时。
	for i := 0; i < len(segLists); i++ {
		for j := 0; j < len(segLists); j++ {
			if i == j {
				continue
			}
			if isAncestor(segLists[i], segLists[j]) {
				return fmt.Errorf(
					"前缀 %q 是前缀 %q 的路径祖先，两者不能同时配置：更深的前缀会因为祖先前缀先被路由到独立的 jfs 实例而永远不可达",
					c.Mounts[i].Prefix, c.Mounts[j].Prefix)
			}
		}
	}

	return nil
}

// checkWritable 尝试在目录下创建再删除一个探测文件，确认路由层进程
// 确实对该 backend 有读写权限。只做 Stat 不够——权限问题往往是
// 挂载点属主/属组和运行路由层进程的用户不一致导致的，这种问题最好
// 在启动阶段就暴露，而不是等到应用第一次写入失败时才发现。
func checkWritable(dir string) error {
	probe := filepath.Join(dir, fmt.Sprintf(".jfs-router-probe-%d", os.Getpid()))
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("无写权限（%w），如果该 backend 只需只读路由，可以忽略此项检查（当前实现未提供只读开关，可按需扩展）", err)
	}
	f.Close()
	return os.Remove(probe)
}

func splitSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// isAncestor 判断 a 是否是 b 的严格路径祖先（按分段比较，a != b）。
func isAncestor(a, b []string) bool {
	if len(a) >= len(b) {
		return false
	}
	for i, seg := range a {
		if b[i] != seg {
			return false
		}
	}
	return true
}

// SortedMounts 返回按 Prefix 排序后的路由表副本，仅用于日志展示时
// 输出稳定、可读的顺序。
func (c *Config) SortedMounts() []MountEntry {
	out := make([]MountEntry, len(c.Mounts))
	copy(out, c.Mounts)
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}

// StatDev 返回路径所在文件系统的设备号，供上层在需要时判断跨设备
// （例如未来扩展只读校验、告警等场景），当前主流程未强依赖此函数。
func StatDev(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}
