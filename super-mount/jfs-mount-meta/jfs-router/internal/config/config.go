// Package config 定义了 jfs-router 的进程级启动配置：挂载点本身的
// 参数、客户端缓存策略，以及选用哪个元数据引擎（文件/Redis/MySQL）
// 及其连接参数。
//
// 注意区分两个不同层次的"配置"：
//   - 本文件里的 Config：进程启动参数，仍然从本地一个 JSON 文件读取
//     （挂载点路径、缓存 TTL 这些属于"进程怎么跑起来"的参数，本身
//     变化很少，没必要放到 Redis/MySQL 里）。
//   - 真正的路由表（前缀 -> backend 映射）：由 internal/metastore 包
//     按 MetaStore.Type 从文件/Redis/MySQL 等引擎读取，可以在进程运行
//     期间由外部系统随时更新，通过客户端缓存的 TTL 机制被感知到。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jfs-router/internal/metastore"
)

// Config 是进程启动配置的顶层结构。
type Config struct {
	// Mountpoint 是本工具对外暴露的「超级挂载点」。
	Mountpoint string `json:"mountpoint"`

	// FsName 仅用于 df -T 等命令展示，不影响功能。
	FsName string `json:"fs_name"`

	// AllowOther 对应 FUSE 的 allow_other 选项。
	AllowOther bool `json:"allow_other"`

	// Debug 打开后会把每个 FUSE 请求打到日志，仅用于排查问题。
	Debug bool `json:"debug"`

	// EntryTimeoutMS/AttrTimeoutMS 控制内核对目录项/属性的缓存时长
	// （毫秒）。这一层缓存独立于 CachedStore 的客户端缓存，两者共同
	// 决定"路由表变更后，应用最快/最慢分别要多久才能感知到"。
	// 默认 1000ms：既能让内核适度缓存、减少无谓的 Lookup 调用，
	// 又不会让路由变更的生效时间过长。
	EntryTimeoutMS int `json:"entry_timeout_ms"`
	AttrTimeoutMS  int `json:"attr_timeout_ms"`

	// Cache 控制路由结果的客户端缓存行为（见 metastore.CacheConfig）。
	Cache CacheConfig `json:"cache"`

	// MetaStore 选择并配置元数据引擎。
	MetaStore MetaStoreConfig `json:"metastore"`
}

// CacheConfig 是 JSON 里以秒为单位的缓存配置，Build() 时转换成
// metastore.CacheConfig（内部用 time.Duration）。用两套结构体分开，
// 是为了让配置文件里写 "ttl_seconds": 30 这种对运维更直观的形式，
// 而不是要求填 "30s" 这样的 Go duration 字符串。
type CacheConfig struct {
	Capacity           int `json:"capacity"`
	TTLSeconds         int `json:"ttl_seconds"`
	NegativeTTLSeconds int `json:"negative_ttl_seconds"`
	ListTTLSeconds     int `json:"list_ttl_seconds"`
}

func (c CacheConfig) toMetastoreConfig() metastore.CacheConfig {
	return metastore.CacheConfig{
		Capacity:    c.Capacity,
		TTL:         time.Duration(c.TTLSeconds) * time.Second,
		NegativeTTL: time.Duration(c.NegativeTTLSeconds) * time.Second,
		ListTTL:     time.Duration(c.ListTTLSeconds) * time.Second,
	}
}

// MetaStoreConfig 通过 Type 字段决定实际使用哪个引擎，其余字段里只有
// 与 Type 对应的那一个会被用到。
type MetaStoreConfig struct {
	// Type 取值 "file" | "redis" | "mysql"。
	Type string `json:"type"`

	File  FileMetaStoreConfig  `json:"file"`
	Redis RedisMetaStoreConfig `json:"redis"`
	MySQL MySQLMetaStoreConfig `json:"mysql"`
}

type FileMetaStoreConfig struct {
	Path string `json:"path"`
}

type RedisMetaStoreConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	Key      string `json:"key"`
}

type MySQLMetaStoreConfig struct {
	DSN           string `json:"dsn"`
	Table         string `json:"table"`
	SegmentColumn string `json:"segment_column"`
	BackendColumn string `json:"backend_column"`
	MaxOpenConns  int    `json:"max_open_conns"`
	MaxIdleConns  int    `json:"max_idle_conns"`
}

// Load 从 JSON 文件加载进程配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var c Config
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	c.applyDefaults()
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.FsName == "" {
		c.FsName = "jfs-router"
	}
	if c.EntryTimeoutMS <= 0 {
		c.EntryTimeoutMS = 1000
	}
	if c.AttrTimeoutMS <= 0 {
		c.AttrTimeoutMS = 1000
	}
}

// Normalize 规范化挂载点路径。
func (c *Config) Normalize() {
	c.Mountpoint = filepath.Clean(c.Mountpoint)
}

// EntryTimeout / AttrTimeout 以 time.Duration 形式暴露给 main.go。
func (c *Config) EntryTimeout() time.Duration {
	return time.Duration(c.EntryTimeoutMS) * time.Millisecond
}

func (c *Config) AttrTimeout() time.Duration {
	return time.Duration(c.AttrTimeoutMS) * time.Millisecond
}

// MetastoreCacheConfig 转换出 internal/metastore 包需要的缓存配置。
func (c *Config) MetastoreCacheConfig() metastore.CacheConfig {
	return c.Cache.toMetastoreConfig()
}

// Validate 校验进程级配置本身（不校验路由表内容——路由表的校验发生在
// 对应的 metastore.Store 实现内部，比如 FileStore 会校验 prefix 格式，
// Redis/MySQL 版本则信任外部系统写入的数据，只在真正被访问到某条路由
// 时才会暴露"这个 backend 不存在"之类的问题）。
func (c *Config) Validate() error {
	if c.Mountpoint == "" || c.Mountpoint == "/" {
		return fmt.Errorf("mountpoint 未设置或不能是根目录")
	}
	switch c.MetaStore.Type {
	case "file":
		if c.MetaStore.File.Path == "" {
			return fmt.Errorf("metastore.type=file 时必须设置 metastore.file.path")
		}
	case "redis":
		if c.MetaStore.Redis.Addr == "" {
			return fmt.Errorf("metastore.type=redis 时必须设置 metastore.redis.addr")
		}
		if c.MetaStore.Redis.Key == "" {
			return fmt.Errorf("metastore.type=redis 时必须设置 metastore.redis.key")
		}
	case "mysql":
		if c.MetaStore.MySQL.DSN == "" {
			return fmt.Errorf("metastore.type=mysql 时必须设置 metastore.mysql.dsn")
		}
	case "":
		return fmt.Errorf("必须设置 metastore.type（\"file\" | \"redis\" | \"mysql\"）")
	default:
		return fmt.Errorf("不支持的 metastore.type: %q（可选 \"file\" | \"redis\" | \"mysql\"）", c.MetaStore.Type)
	}
	return nil
}
