// Package router 实现「单一挂载点 + 前缀路由到多个真实后端目录」的
// FUSE 文件系统。
//
// 设计核心：不重新发明轮子去实现 Read/Write/Rename 等系统调用的转发逻辑，
// 而是复用 go-fuse 官方、经过充分测试的 fs.LoopbackNode（它使用基于文件
// 描述符的系统调用，正确处理了硬链接去重、原子 rename、xattr 等细节，
// 规避了「先拼路径字符串再操作」方案常见的 TOCTOU 竞态问题）。
//
// 我们要做的，只是把每条 mountTable 记录对应的一整棵 LoopbackRoot 子树，
// "过继"（graft）到路由层自己维护的一棵静态虚拟目录树的正确位置上。
// 过继之后，FUSE 内核请求一旦落入某个前缀子树，就完全由对应的
// LoopbackNode 独立处理（无论请求路径有多深），路由层不再介入——
// 这正好符合「一次读写只属于一个 jfs 实例」的使用场景，也是本方案
// 能做到足够轻量的关键。
//
// 跨前缀操作的处理：go-fuse 的 LoopbackNode.Rename 会显式比较两侧节点
// 所属的 LoopbackRoot 指针，一旦不相等（意味着 rename 的目标跨越到了
// 另一条前缀对应的子树）就直接返回 EXDEV——这个判断independent于两个
// backend 目录实际是否在同一块物理设备上，因此哪怕两个 juicefs 实例的
// 挂载点碰巧同盘，跨前缀 rename 依然会被正确拒绝，语义上和真实文件系统
// 跨分区 mv 一致。这一行为已经过实际挂载 + strace 验证：
// renameat2 系统调用层面确实返回了 EXDEV，coreutils 的 mv 会自动降级为
// copy+unlink（这是 mv 自身的标准行为，不需要本项目做任何特殊处理）。
package router

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	"jfs-router/internal/config"
)

// Root 是整个路由文件系统的根节点。它只负责在挂载时（OnAdd）
// 一次性搭建好静态虚拟目录树，运行期间不做任何路由计算——
// 真正的路径解析、读写全部下沉给过继进来的 LoopbackNode 处理。
type Root struct {
	fs.Inode

	entries []config.MountEntry
}

var _ = (fs.NodeOnAdder)((*Root)(nil))

// NewRoot 构造路由根节点。传入的 entries 应已经过
// config.Config.Validate() 校验（前缀合法、无重叠祖先关系、
// backend 存在且可写）。
func NewRoot(entries []config.MountEntry) *Root {
	return &Root{entries: entries}
}

// OnAdd 在文件系统被挂载时调用一次，用于搭建静态目录树。
// 参照 go-fuse 官方 inmemory 示例的写法：中间目录用裸的 &fs.Inode{}
// 表示（不做任何自定义 Lookup/Readdir，框架会自动用其静态 children
// 表来响应查找和目录遍历），叶子节点则是过继进来的 LoopbackRoot 子树。
func (r *Root) OnAdd(ctx context.Context) {
	for _, e := range r.entries {
		segs := splitSegments(e.Prefix)
		if len(segs) == 0 {
			// Validate() 已经拒绝了 "/" 前缀，这里是防御性判断。
			continue
		}

		// 1. 创建/复用中间的虚拟目录节点，一直走到叶子的父目录。
		parent := &r.Inode
		for _, seg := range segs[:len(segs)-1] {
			parent = ensureDir(ctx, parent, seg)
		}
		leafName := segs[len(segs)-1]

		// 2. 为该前缀新建一个独立的 LoopbackRoot（对应一个真实后端目录），
		// 并把它的根节点过继为 parent 下名为 leafName 的子节点。
		//
		// 关键点：LoopbackRoot.RootNode 会被 fs.NewLoopbackRoot 设置为
		// 它自己的根节点对象，LoopbackNode 计算路径时正是以这个
		// RootNode（而不是整个 FUSE 挂载的全局根）作为相对路径的起点
		// （见 go-fuse fs/loopback.go 中 LoopbackNode.root()/path() 的实现）。
		// 这保证了无论这棵子树被过继到路由树的哪一层、有多深，
		// 子树内部的路径计算永远只相对它自己的 backend 根目录，不会串味。
		lbEmbedder, err := fs.NewLoopbackRoot(e.Backend)
		if err != nil {
			// entries 已经过 Validate() 校验过 backend 可访问，
			// 这里理论上不应该失败；一旦失败说明运行环境发生了变化
			// （比如后端目录在启动瞬间被卸载），直接 panic 让进程
			// 退出、由外层（systemd 等）重启更安全，好过悄悄地把
			// 这个前缀挂成一个空目录、让应用误以为写入成功。
			panic(fmt.Sprintf("为前缀 %q 创建 loopback root 失败（backend=%q）: %v",
				e.Prefix, e.Backend, err))
		}

		child := parent.NewPersistentInode(ctx, lbEmbedder, fs.StableAttr{
			Mode: syscall.S_IFDIR,
			// Ino 留 0，由框架在这棵虚拟树内部自动分配唯一序号；
			// 子树内部文件真正的 inode 号由 LoopbackRoot.idFromStat
			// 基于底层 stat 结果计算，两者互不冲突。
		})
		if !parent.AddChild(leafName, child, false) {
			// false 表示不覆盖已存在的同名子节点；如果这里失败，
			// 说明 Validate() 里的重叠前缀校验有遗漏，属于实现 bug，
			// 必须 fail-fast 暴露出来而不是静默覆盖路由。
			panic(fmt.Sprintf("挂接前缀 %q 失败：路径 %s/%s 已存在同名节点，"+
				"这不应该发生（说明前缀校验逻辑有缺陷）", e.Prefix, pathOf(parent), leafName))
		}
	}
}

// ensureDir 返回 parent 下名为 name 的虚拟目录子节点，不存在则创建。
// 用裸的 &fs.Inode{} 作为中间目录：它不实现 NodeLookuper/NodeReaddirer，
// go-fuse 的默认桥接逻辑会退化为直接查询/遍历该 Inode 自身维护的
// 静态 children 表（对应 go-fuse 源码 fs/bridge.go 中 rawBridge.lookup
// 和 OpenDir 的 fallback 分支），行为等价于一个只读的普通目录。
func ensureDir(ctx context.Context, parent *fs.Inode, name string) *fs.Inode {
	if ch := parent.GetChild(name); ch != nil {
		return ch
	}
	ch := parent.NewPersistentInode(ctx, &fs.Inode{}, fs.StableAttr{Mode: syscall.S_IFDIR})
	parent.AddChild(name, ch, false)
	return ch
}

func splitSegments(p string) []string {
	p = strings.Trim(filepath.Clean(p), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// pathOf 仅用于 panic 信息里给出更友好的定位，不影响正常路径解析逻辑
// （正常路径解析完全由过继进去的 LoopbackNode 自己完成）。
func pathOf(n *fs.Inode) string {
	if n.IsRoot() {
		return ""
	}
	return n.Path(nil)
}
