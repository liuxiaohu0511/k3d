// Package router 实现「单一挂载点 + 前缀路由到多个真实后端目录」的
// FUSE 文件系统。
//
// 与早期版本（一次性在 OnAdd 里把整棵路由树静态搭好）不同，这一版
// 路由表可能来自 Redis/MySQL 等外部、随时可能变化的元数据源，因此
// 改为在 Lookup 时动态解析：每次有新的顶层目录被访问，才去问
// metastore.Store（通常是包了缓存的 CachedStore）这个名字对应哪个
// backend，再现场构造一棵 go-fuse 官方的 LoopbackRoot 子树接管这次
// 请求。子树内部的路径计算完全独立于路由层（见 go-fuse
// fs/loopback.go 中 LoopbackNode.root()/path() 的实现，是相对各自
// RootData.RootNode 计算的），所以现场构造、随时丢弃都是安全的。
//
// 关键设计取舍：路由 key 固定为虚拟路径的第一段（例如 "/tenantA/x/y"
// 的路由 key 是 "tenantA"），不支持更深层级的前缀。原因见
// internal/metastore 包注释：这让 Redis/MySQL 的点查天然适配。
package router

import (
	"context"
	"hash/fnv"
	"log"
	"sort"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"jfs-router/internal/metastore"
)

// Root 是路由文件系统的根节点，直接对应挂载点本身。它的每一个直接
// 子目录都是一个"路由 key"（tenant/业务线名），Lookup 时现场向 store
// 查询、现场过继对应的 LoopbackRoot 子树；子目录再往下的所有路径，
// 请求都会被过继进来的 LoopbackNode 自己处理，Root 不会再被调用。
type Root struct {
	fs.Inode
	store metastore.Store
}

var _ = (fs.NodeLookuper)((*Root)(nil))
var _ = (fs.NodeReaddirer)((*Root)(nil))

// NewRoot 构造路由根节点。store 通常应该是 metastore.NewCachedStore
// 包装过的实例，这样重复访问同一个路由 key 不会每次都打到 Redis/MySQL。
func NewRoot(store metastore.Store) *Root {
	return &Root{store: store}
}

// Lookup 在挂载点根目录下按名字（顶层路由 key）解析出对应的 backend，
// 现场构造一棵 LoopbackRoot 子树并过继为本次查找的结果。
func (r *Root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	route, ok, err := r.store.Get(ctx, name)
	if err != nil {
		log.Printf("路由查询失败: segment=%q err=%v", name, err)
		return nil, syscall.EIO
	}
	if !ok {
		return nil, syscall.ENOENT
	}

	lbRoot, err := fs.NewLoopbackRoot(route.Backend)
	if err != nil {
		// 运行期发生（而不是启动期一次性校验），说明 backend 目录在
		// 路由表更新之后、这次访问之前的窗口期内变得不可访问（比如
		// 对应的 juicefs 实例被卸载）。返回 EIO 让调用方感知到"这条
		// 路由目前有问题"，而不是 panic 掉整个路由层——一个 tenant
		// 的 backend 出问题不应该影响其他 tenant 正常访问。
		log.Printf("为路由 %q 创建 loopback 子树失败 backend=%q: %v", name, route.Backend, err)
		return nil, syscall.EIO
	}

	child := r.NewInode(ctx, lbRoot, fs.StableAttr{
		Mode: syscall.S_IFDIR,
		Ino:  graftIno(name, route.Backend),
	})
	return child, 0
}

// graftIno 为"路由 key + 当前解析到的 backend"这一组合计算一个稳定、
// 确定性的 Ino。
//
// 这不是随手选的：早期实现里这里用的是 Ino: 0（让框架每次都分配全新
// Inode），实测会在压测下偶发触发一个真实的竞态——内核侧 EntryTimeout
// 到期后，即便 backend 完全没变，也会重新触发 Lookup；如果每次都换一个
// 全新的 Inode 对象，内核会认为父目录（比如 "tenantRedis"）被换成了
// "另一个"对象，此前已经缓存、依附在旧父对象下的子路径（如 "hello.txt"）
// 会在这次交接的极短窗口内被内核判定为不存在，返回一次性、偶发的
// ENOENT——这个现象已经用循环压测复现过（约每隔 CachedStore 的 TTL
// 周期附近概率性出现一次）。
//
// 修复方式：Ino 由 segment 和 backend 共同哈希得到。这样只要 backend
// 没有变化，无论 Lookup 被触发多少次（不管是内核 EntryTimeout 到期，
// 还是我们自己的客户端缓存 TTL 到期后重新拉取，只要拉回来的还是同一个
// backend），go-fuse 的 addNewChild 都会命中 stableAttrs 表里已存在的
// 同 Ino 节点、直接复用旧对象（见 go-fuse fs/bridge.go），不会产生
// "父目录突然换了个新身份"的抖动。只有当路由表里这个 segment 确实被
// 改成了新的 backend，Ino 才会真正变化，此时产生一个新节点是我们明确
// 想要的行为（否则就是本文件之前版本注释里提到的"缓存过期了但没生效"
// 的反向 bug）。
//
// 用 FNV-1a 64 位哈希并清掉最高位：go-fuse 在 Ino 传 0（自动分配）时，
// 会从 2^63 开始往上分配序列号（见 fs/bridge.go newInodeUnlocked 注释），
// 清掉最高位能确保我们手工指定的 Ino 落在 [1, 2^63) 区间，和自动分配区间
// 完全不重叠，杜绝任何理论上的碰撞可能。
func graftIno(segment, backend string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(segment))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(backend))
	sum := h.Sum64() &^ (uint64(1) << 63) // 清最高位
	if sum == 0 {
		sum = 1 // 避免 0（会被框架当成"请自动分配"）
	}
	return sum
}

// Readdir 让 `ls` 挂载点根目录时，能看到 store 里当前所有路由 key
// （对应每个 tenant/业务线的顶层目录名）。
func (r *Root) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	routes, err := r.store.List(ctx)
	if err != nil {
		log.Printf("枚举路由表失败: %v", err)
		return nil, syscall.EIO
	}
	entries := make([]fuse.DirEntry, 0, len(routes))
	for seg := range routes {
		entries = append(entries, fuse.DirEntry{Name: seg, Mode: syscall.S_IFDIR})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return fs.NewListDirStream(entries), 0
}
