package metastore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"jfs-router/internal/cache"
	"jfs-router/internal/singleflight"
)

// CacheConfig 控制 CachedStore 的缓存行为。
type CacheConfig struct {
	// Capacity 是正向缓存（命中的路由）最多保留多少条，超出按 LRU 淘汰。
	// 默认 4096，对绝大多数场景（几十到几千个 tenant）都绰绰有余。
	Capacity int

	// TTL 是正向缓存（找到了路由）的存活时间：命中缓存的路由过了这个
	// 时间点会被视为过期，下一次访问会重新去查底层 Store。这个值越大，
	// 对 Redis/MySQL 的压力越小，但外部改路由后客户端感知到变化的
	// 延迟也越大——需要按你的场景在"降低后端压力"和"变更生效速度"
	// 之间取舍。默认 30 秒。
	TTL time.Duration

	// NegativeTTL 是负向缓存（确认查无此路由）的存活时间。故意比 TTL
	// 短很多（默认 5 秒），这样新增一个 tenant 路由后，此前因为
	// "查无此路由"被缓存过 ENOENT 的路径能更快变为可访问，同时依然能
	// 防止程序反复 stat 一个不存在的路径时把请求打穿到 Redis/MySQL。
	NegativeTTL time.Duration

	// ListTTL 控制 List()（用于 `ls` 挂载根目录）结果的缓存时间。
	// 默认与 TTL 相同。
	ListTTL time.Duration
}

func (c *CacheConfig) setDefaults() {
	if c.Capacity <= 0 {
		c.Capacity = 4096
	}
	if c.TTL <= 0 {
		c.TTL = 30 * time.Second
	}
	if c.NegativeTTL <= 0 {
		c.NegativeTTL = 5 * time.Second
	}
	if c.ListTTL <= 0 {
		c.ListTTL = c.TTL
	}
}

// CachedStore 把任意 Store 包装成带客户端缓存的版本。路由层应该始终
// 通过 CachedStore 访问底层元数据源，而不是直接持有 FileStore/
// RedisStore/MySQLStore——缓存与"元数据从哪来"是正交的关注点。
type CachedStore struct {
	inner Store
	cfg   CacheConfig

	pos *cache.TTLCache[Route]    // segment -> Route，命中的路由
	neg *cache.TTLCache[struct{}] // segment -> 占位值，确认查无此路由

	sf singleflight.Group

	listMu   sync.Mutex
	listAt   time.Time
	listData map[string]Route
}

// NewCachedStore 用给定配置包装 inner。
func NewCachedStore(inner Store, cfg CacheConfig) *CachedStore {
	cfg.setDefaults()
	return &CachedStore{
		inner: inner,
		cfg:   cfg,
		pos:   cache.New[Route](cfg.Capacity, cfg.TTL),
		neg:   cache.New[struct{}](cfg.Capacity, cfg.NegativeTTL),
	}
}

type getResult struct {
	route Route
	found bool
}

// Get 优先查本地缓存（正向/负向），未命中时通过 singleflight 合并对
// 同一个 key 的并发请求，再去查底层 Store，并把结果写回对应的缓存。
//
// 网络错误不会被缓存：一次 Redis/MySQL 抖动不应该导致某个路由在
// NegativeTTL/TTL 这么长时间内被"锁死"成错误状态，下一次请求应该
// 立刻有机会重试。
func (c *CachedStore) Get(ctx context.Context, segment string) (Route, bool, error) {
	if r, ok := c.pos.Get(segment); ok {
		return r, true, nil
	}
	if _, ok := c.neg.Get(segment); ok {
		return Route{}, false, nil
	}

	v, err := c.sf.Do(segment, func() (interface{}, error) {
		route, found, err := c.inner.Get(ctx, segment)
		if err != nil {
			return nil, err
		}
		return getResult{route: route, found: found}, nil
	})
	if err != nil {
		return Route{}, false, fmt.Errorf("查询路由 %q 失败: %w", segment, err)
	}

	res := v.(getResult)
	if res.found {
		c.pos.Set(segment, res.route)
	} else {
		c.neg.Set(segment, struct{}{})
	}
	return res.route, res.found, nil
}

// List 缓存整张路由表，TTL 由 ListTTL 控制。查询失败且此前有过成功的
// 缓存时，宁可返回稍微过期的数据也不让 `ls` 直接报错——目录枚举属于
// "尽力而为"的辅助功能，不应该因为一次网络抖动就完全不可用。
func (c *CachedStore) List(ctx context.Context) (map[string]Route, error) {
	c.listMu.Lock()
	if c.listData != nil && time.Since(c.listAt) < c.cfg.ListTTL {
		data := c.listData
		c.listMu.Unlock()
		return data, nil
	}
	c.listMu.Unlock()

	v, err := c.sf.Do("__list__", func() (interface{}, error) {
		return c.inner.List(ctx)
	})
	if err != nil {
		c.listMu.Lock()
		fallback := c.listData
		c.listMu.Unlock()
		if fallback != nil {
			return fallback, nil
		}
		return nil, err
	}

	data := v.(map[string]Route)
	c.listMu.Lock()
	c.listData = data
	c.listAt = time.Now()
	c.listMu.Unlock()
	return data, nil
}

// Invalidate 主动清除某个 segment 的缓存（正向+负向）。当前路由层
// 没有调用它，预留给未来场景：比如通过一个管理接口手动"立刻生效"某条
// 路由变更，而不必等 TTL 自然过期。
func (c *CachedStore) Invalidate(segment string) {
	c.pos.Delete(segment)
	c.neg.Delete(segment)
}

func (c *CachedStore) Close() error {
	return c.inner.Close()
}
