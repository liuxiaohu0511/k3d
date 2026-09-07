package metastore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// fileMountEntry 对应配置文件里 mounts 数组的一项，字段名保持和历史
// 版本一致（prefix/backend），减少迁移成本；FileStore 会在加载时校验
// prefix 必须是单段（不含 "/"），因为路由 key 现在统一是顶层分段。
type fileMountEntry struct {
	Prefix  string `json:"prefix"`
	Backend string `json:"backend"`
}

type fileMountsDoc struct {
	Mounts []fileMountEntry `json:"mounts"`
}

// FileStore 从本地 JSON 文件读取路由表，是"配置文件"这种最简单元数据
// 源的实现，同时也是没有 Redis/MySQL 依赖时最容易上手的默认选项。
//
// 支持热编辑：每次 Get/List 调用时会先检查文件 mtime 是否变化，变化了
// 就重新加载，语义上和 Redis/MySQL 那样"随时可能被外部更新"保持一致
// （最终这些差异都会被上层 CachedStore 的 TTL 统一抹平：不管底层是文件
// 还是数据库，客户端能多快感知到变更，都由 CachedStore 的 TTL 决定）。
type FileStore struct {
	path string

	mu     sync.Mutex
	mtime  time.Time
	routes map[string]Route
}

// NewFileStore 加载并解析一次配置文件，失败则直接返回错误（fail-fast，
// 启动阶段的配置错误不应该被悄悄吞掉）。
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) reload() error {
	st, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("stat 配置文件 %q 失败: %w", s.path, err)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("读取配置文件 %q 失败: %w", s.path, err)
	}
	var doc fileMountsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("解析配置文件 %q 失败: %w", s.path, err)
	}

	routes := make(map[string]Route, len(doc.Mounts))
	for _, m := range doc.Mounts {
		seg := strings.Trim(m.Prefix, "/")
		if seg == "" || strings.Contains(seg, "/") {
			return fmt.Errorf(
				"配置文件 %q 中的 prefix %q 不合法：file 元数据源只支持"+
					"顶层单段前缀（例如 \"/tenantA\"），不支持多级路径（如 \"/shared/logs\"）",
				s.path, m.Prefix)
		}
		if m.Backend == "" {
			return fmt.Errorf("配置文件 %q 中前缀 %q 缺少 backend", s.path, m.Prefix)
		}
		if _, dup := routes[seg]; dup {
			return fmt.Errorf("配置文件 %q 中前缀 %q 重复配置", s.path, m.Prefix)
		}
		routes[seg] = Route{Backend: m.Backend}
	}

	s.mu.Lock()
	s.routes = routes
	s.mtime = st.ModTime()
	s.mu.Unlock()
	return nil
}

// maybeReload 在文件 mtime 发生变化时重新加载。加载失败时刻意保留上一次
// 成功加载的路由表并吞掉错误——一次写入过程中的中间态（比如文件被
// 截断到一半）不应该导致整个路由表瞬间清空，这与 Redis/MySQL 连接
// 短暂抖动时的处理原则是一致的（宁可暂时用旧数据，也不要无谓地让
// 挂载点大面积报错）。错误详情此处简化为直接丢弃，生产环境建议接入
// 统一日志框架后在这里补一条 Warn 日志。
func (s *FileStore) maybeReload() {
	st, err := os.Stat(s.path)
	if err != nil {
		return
	}
	s.mu.Lock()
	changed := st.ModTime().After(s.mtime)
	s.mu.Unlock()
	if changed {
		_ = s.reload()
	}
}

func (s *FileStore) Get(_ context.Context, segment string) (Route, bool, error) {
	s.maybeReload()
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.routes[segment]
	return r, ok, nil
}

func (s *FileStore) List(_ context.Context) (map[string]Route, error) {
	s.maybeReload()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]Route, len(s.routes))
	for k, v := range s.routes {
		out[k] = v
	}
	return out, nil
}

func (s *FileStore) Close() error { return nil }
