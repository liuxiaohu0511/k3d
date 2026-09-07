// Package cache 提供一个通用、无第三方依赖的 TTL + LRU 缓存。
//
// 用途：缓存"最近被访问过的路由 key -> 解析结果"，避免每次 FUSE Lookup
// 都去访问 Redis/MySQL 等外部元数据引擎；同时通过 TTL 保证外部数据源
// 里的变更能在有限时间内被感知到，而不是永久沿用启动时的旧值。
//
// 容量限制 (LRU) 是为了防止"路由 key 空间很大但只有少量在被频繁访问"
// 场景下缓存无限增长——超过容量后淘汰最久未使用的条目，天然对应
// "最近读写的路径"这一诉求：常被访问的路径会不断被 Get 命中而移到
// 队首，冷路径会自然沉到队尾并被淘汰。
//
// 过期策略是惰性的：只在 Get 时检查是否已过期，不额外起后台协程扫描。
// 这对于容量有界的缓存足够了——即使某个条目过期后一直不被访问，
// 它占用的内存也早晚会被 LRU 淘汰机制回收，不会无限增长。
package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry[V any] struct {
	key      string
	value    V
	expireAt time.Time
}

// TTLCache 是一个支持泛型值类型、并发安全的 TTL + LRU 缓存。
type TTLCache[V any] struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	items    map[string]*list.Element
	order    *list.List // Front = 最近使用，Back = 最久未使用
}

// New 创建一个缓存。capacity <= 0 时使用默认值 1024，ttl <= 0 时使用
// 默认值 1 分钟。
func New[V any](capacity int, ttl time.Duration) *TTLCache[V] {
	if capacity <= 0 {
		capacity = 1024
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &TTLCache[V]{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get 返回 key 对应的值。如果不存在或已过期（过期条目会被顺带清理），
// 返回 ok=false。
func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	e := el.Value.(*entry[V])
	if time.Now().After(e.expireAt) {
		c.removeElementLocked(el)
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return e.value, true
}

// Set 写入或更新一条缓存，重置其 TTL 计时，并将其标记为最近使用。
// 如果写入后条目数超过容量上限，淘汰最久未使用的一条。
func (c *TTLCache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry[V])
		e.value = value
		e.expireAt = time.Now().Add(c.ttl)
		c.order.MoveToFront(el)
		return
	}

	e := &entry[V]{key: key, value: value, expireAt: time.Now().Add(c.ttl)}
	el := c.order.PushFront(e)
	c.items[key] = el

	if c.order.Len() > c.capacity {
		back := c.order.Back()
		if back != nil {
			c.removeElementLocked(back)
		}
	}
}

// Delete 主动移除一条缓存（例如在确认性写操作后想要立刻失效旧值时使用）。
func (c *TTLCache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeElementLocked(el)
	}
}

// Len 返回当前缓存条目数（含尚未被 Get 触发过期清理的条目），主要用于
// 监控/调试。
func (c *TTLCache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

func (c *TTLCache[V]) removeElementLocked(el *list.Element) {
	e := el.Value.(*entry[V])
	delete(c.items, e.key)
	c.order.Remove(el)
}
