// Package singleflight 提供一个最小化的请求合并（single-flight）实现：
// 同一个 key 如果有多个并发调用者同时发起，只真正执行一次底层函数，
// 其余调用者等待并共享同一个结果。
//
// 用途：CachedStore 的缓存在 TTL 过期的瞬间，如果有大量 FUSE 请求同时
// 落在同一个路由 key 上（例如某个热门 tenant 目录被大量并发访问），
// 没有 single-flight 的话会瞬间打出去等量的 Redis/MySQL 请求，形成
// 缓存失效风暴（thundering herd）。这里没有直接引入官方的
// golang.org/x/sync/singleflight，是为了保持依赖面最小——这个实现
// 只有几十行，逻辑上和官方版本等价（sync.Mutex + map + sync.WaitGroup）。
package singleflight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Group 管理一组按 key 去重的调用。零值可以直接使用。
type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

// Do 执行 fn，并保证同一时刻同一个 key 只有一次 fn 在真正执行；
// 期间到达的其他调用者会阻塞等待，并复用第一个调用者的结果（包括错误）。
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	c := &call{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
