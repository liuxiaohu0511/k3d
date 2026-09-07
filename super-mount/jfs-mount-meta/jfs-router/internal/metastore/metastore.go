// Package metastore 把"mount 元数据从哪里读"抽象成一个统一接口
// （Store），使路由层可以在配置文件、Redis、MySQL 等不同引擎之间自由
// 切换，甚至将来接入新的引擎（etcd、Consul……）也只需要新增一个
// Store 实现，不需要改动路由层任何代码。
//
// 路由 key 的设计约束（见 Store 接口注释）：只支持虚拟路径的第一段
// 作为路由 key，不支持多级前缀。这是为了让 Redis/MySQL 这类 KV/关系型
// 存储的点查天然适配"一个 tenant 一条记录"的模型。
package metastore

import "context"

// Route 描述一条路由规则解析出的结果。目前只有 Backend 一个字段，
// 用 struct 包装而不是直接用 string，是为了未来扩展（比如每条路由
// 单独配置只读/限流参数）时不需要再改一遍所有 Store 实现的签名。
type Route struct {
	// Backend 是该路由 key 对应的真实后端目录（通常是某个 juicefs
	// 实例已经挂载好的 POSIX 路径，或其子目录）。
	Backend string
}

// Store 是元数据源的最小接口。
type Store interface {
	// Get 返回给定顶层路径分段（例如虚拟路径 "/tenantA/x" 中的
	// "tenantA"）对应的路由。ok=false 表示没有这条路由（不是错误，
	// 调用方应将其映射为 ENOENT）。
	Get(ctx context.Context, segment string) (Route, bool, error)

	// List 返回当前所有路由，key 为顶层分段名。用于挂载根目录的
	// Readdir（让 `ls /data` 能列出所有 tenant），以及启动时的
	// 健康检查/日志打印。
	List(ctx context.Context) (map[string]Route, error)

	// Close 释放该 Store 持有的资源（数据库连接、Redis 连接池等）。
	// 幂等：多次调用应当安全。
	Close() error
}
