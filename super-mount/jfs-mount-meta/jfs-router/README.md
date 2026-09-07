# jfs-router

一个轻量级的"超级挂载"路由层：对应用暴露**单一 FUSE 挂载点**，内部按
**路径前缀（顶层目录名）**把读写请求转发给对应的 JuiceFS（或任意
POSIX 目录）实例。路由表（前缀 -> backend 的映射）可以来自本地 JSON
文件、Redis 或 MySQL，并通过客户端缓存 + TTL 机制感知外部变更。

适用场景：每次读写的路径前缀在运行期间基本固定，即"同一次操作从头到尾
只落在一个后端实例上"，不需要跨实例的原子事务保证；但路由表本身
（有哪些 tenant、各自指向哪个后端）可能由外部系统（运维平台、控制面）
动态维护，不希望每次改路由都重启进程。

## 架构总览

```
              ┌─────────────────────────────┐
   应用 ───▶  │   /data  (单一挂载点)         │
              │   Root.Lookup(name)          │
              └───────────────┬──────────────┘
                               │ 现场查询 + 缓存
                               ▼
              ┌─────────────────────────────┐
              │      metastore.Store         │  <- 可插拔
              │  (CachedStore 包装任意实现)   │
              └───────┬─────────┬─────────┬──┘
                      │         │         │
                 FileStore  RedisStore MySQLStore
                (JSON 文件)  (Hash)   (关系表)
```

- **路由层（internal/router）**：不重新实现 Read/Write/Rename 等系统
  调用转发，而是复用 go-fuse 官方、经过充分测试的 `fs.LoopbackNode`。
  每次有新的顶层目录被访问，才向 `metastore.Store` 查询这个名字对应
  哪个 backend，现场构造一棵 `LoopbackRoot` 子树接管这次请求；子树内部
  再往下的所有路径，都由过继进来的 `LoopbackNode` 自己处理，路由层
  不会再被调用。
- **元数据层（internal/metastore）**：把"路由表从哪里读"抽象成一个
  统一接口 `Store`，文件/Redis/MySQL 都是这个接口的具体实现，彼此完全
  解耦，新增一种引擎（etcd、Consul……）只需要新增一个实现，不需要改动
  路由层任何代码。
- **缓存层（internal/metastore/cached_store.go + internal/cache）**：
  在任意 `Store` 外面包一层客户端缓存——正向 TTL 缓存（命中的路由）、
  负向缓存（确认查无此路由，避免应用反复 stat 不存在的路径把请求打穿
  到后端）、LRU 容量上限（防止路由 key 空间很大时无限增长）、
  singleflight 防止缓存失效瞬间的并发请求全部打到 Redis/MySQL。

## 关键设计取舍：路由 key 只能是顶层单段前缀

路由 key 固定为虚拟路径的**第一段**，例如 `/tenantA/x/y` 的路由 key 是
`tenantA`；不支持像 `/shared/logs` 这种多级前缀。原因：这样
Redis（`HGET`）、MySQL（按主键 `SELECT`）的点查天然对应"一个
tenant/业务线一条记录"的模型，不需要为了支持任意深度的前缀匹配，把
整张路由表都拉回来做前缀扫描。如果确实需要多级前缀，可以在业务层把
多级路径"拍平"成一个顶层分段名（如用 `shared_logs` 代替
`shared/logs`），或者自行扩展 `metastore.Store` 接口支持按需拉取候选
层级。

## 目录结构

```
cmd/jfs-router/main.go            入口：加载配置、构造 Store、挂载、信号处理
internal/config/config.go         进程级配置（挂载点、缓存策略、选用哪个元数据引擎）
internal/router/router.go         路由核心：动态 Lookup + LoopbackRoot 过继
internal/metastore/
  metastore.go                    Store 接口定义
  file.go / redis.go / mysql.go   三种具体实现
  cached_store.go                 TTL/LRU/负缓存/singleflight 包装层
internal/cache/ttlcache.go        通用 TTL + LRU 缓存（无第三方依赖）
internal/singleflight/            最小化请求合并实现（无第三方依赖）
configs/                          各元数据源的配置示例 + MySQL 建表 DDL
deploy/jfs-router.service         systemd 部署示例
```

## 配置

进程本身的启动参数（挂载点、缓存策略、选哪个元数据引擎）仍然从本地一个
JSON 文件读取——这些参数变化很少，没必要放到 Redis/MySQL 里。**真正的
路由表**则按 `metastore.type` 从文件/Redis/MySQL 读取，可以在进程运行
期间由外部系统随时更新。

三种元数据源的配置示例见 `configs/config.example.{file,redis,mysql}.json`；
MySQL 的建表 DDL 见 `configs/mysql-schema.sql`。字段含义见
`internal/config/config.go` 注释。

### 缓存参数怎么调

- `cache.ttl_seconds`：命中路由的缓存存活时间。越大对 Redis/MySQL
  压力越小，但外部改路由后客户端感知到变化的延迟也越大。
- `cache.negative_ttl_seconds`：确认查无此路由的缓存时间，建议明显
  小于 `ttl_seconds`（默认 5s vs 30s），这样新增一个 tenant 后，此前
  因为"查无此路由"被缓存过 ENOENT 的路径能更快变为可访问。
- `entry_timeout_ms` / `attr_timeout_ms`：内核对目录项/属性的缓存时长，
  这一层缓存独立于上面的客户端缓存，两者共同决定"路由表变更后，应用
  最快/最慢分别要多久才能感知到"。

## 构建

推荐用 Makefile（已经在沙箱里实测跑通全部目标：build / vet / fmt / test /
tidy / cross / install / uninstall / clean）：

```bash
make build          # 本地构建，产物在 bin/jfs-router
make run            # 构建后用文件元数据源的示例配置直接跑起来（需要 sudo）
make vet            # go vet 静态检查
make fmt            # gofmt 格式化并检查
make cross          # 交叉编译出 linux/amd64 + linux/arm64 两个静态二进制
make install        # 构建后安装到 /usr/local/bin（需要 sudo）
make clean          # 清理 bin/
```

不想用 Makefile 的话，等价的原始命令：

```bash
go build -trimpath -ldflags "-s -w" -o bin/jfs-router ./cmd/jfs-router

# 交叉编译（比如在 x86 开发机上编给 arm64 生产机用）：
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" \
    -o bin/jfs-router-linux-arm64 ./cmd/jfs-router
```

依赖：`github.com/hanwen/go-fuse/v2`、`github.com/redis/go-redis/v9`、
`github.com/go-sql-driver/mysql`（间接依赖 `golang.org/x/sys` 等）。
go-fuse 是纯 Go 实现（直接对接 `/dev/fuse` 协议），不需要 cgo，
`CGO_ENABLED=0` 就能出静态二进制（Makefile 里已经默认这么设置），
部署时不用担心目标机器的动态库版本问题；只支持类 Unix 系统
（本项目面向 Linux 生产环境）。

> **注**：`go.mod` 里有一条
> `replace golang.org/x/sys => github.com/golang/sys v0.28.0`，以及
> go-redis/mysql 驱动都固定在略旧于最新的版本（`v9.7.3` /
> `v1.7.1`）。这是为了在本次开发所用的沙箱环境（无法访问
> `golang.org`、`filippo.io` 等域名，只能访问 `github.com`；且沙箱内
> Go 工具链是 1.22，无法自动升级到部分最新依赖要求的 1.24）中完成
> 真实编译和挂载测试而做的临时绕行方案。在有正常网络访问、更新版本
> Go 工具链的开发/CI 环境中，可以放心升级这些依赖版本或删掉
> `replace` 行，代码本身不依赖任何版本特定的行为。

## 运行

```bash
sudo ./bin/jfs-router -config configs/config.example.file.json
```

- 挂载点若不存在会自动创建；若已存在但非空目录，会拒绝挂载。
- 收到 `SIGINT`/`SIGTERM` 会尝试优雅卸载（对正在关闭中的场景做了短暂
  重试）。
- 生产部署参考 `deploy/jfs-router.service`。

## 已验证的行为（实测，非纸面推导）

开发过程中在沙箱里真实安装并启动了 Redis 和 MariaDB，覆盖了以下场景：

- **三种元数据源**（file/Redis/MySQL）都能正确读写、`ls` 枚举、写入
  确实落到对应 backend 目录。
- **正向缓存 TTL**：改路由（Redis `HSET` / MySQL `UPDATE`）后，TTL 到期
  前稳定返回旧内容且无报错；TTL 过期后干净切换到新内容，切换瞬间也
  没有报错。
- **负向缓存 TTL**：访问不存在的路由 -> ENOENT 且被缓存；负缓存 TTL
  内插入这条路由 -> 仍然 ENOENT（预期的、可接受的短暂延迟）；负缓存
  TTL 过期后 -> 正确解析到新插入的路由。
- **插入新路由后立即访问**（含 `ls` 触发的 READDIRPLUS 场景）批量循环
  测试（20 次）无失败。
- **file 元数据源支持热编辑**：修改本地 JSON 文件、等 TTL 过期后，
  改动被正确感知（不需要重启进程）。
- **跨前缀 rename** 用 `strace` 确认 `renameat2` 返回 `EXDEV`（go-fuse
  的 `LoopbackNode.Rename` 显式比较两侧节点所属的 `LoopbackRoot`
  指针，跨子树一律返回 `EXDEV`，与两个 backend 是否同一物理设备无关）。
- 配置校验（缺 `metastore.type`、redis 缺 `key`、mysql 缺 `dsn` 等）
  均在启动时被正确拒绝，退出码 1。
- `SIGTERM` 触发优雅卸载。

### 开发过程中发现并修复的一个真实 bug

最初的实现里，每次 `Lookup` 都用 `Ino: 0` 让框架分配全新的 Inode
对象（意图是保证 TTL 过期后能拿到反映最新路由的全新子树）。压测
（循环几十次、每次间隔覆盖内核 `entry_timeout` 周期）发现这会导致一个
概率性的真实竞态：内核侧 `entry_timeout` 到期后，即便 backend 完全
没变，也会重新触发 `Lookup`；由于每次都换一个全新的 Inode 对象，内核
会认为父目录（比如 `tenantRedis`）被换成了"另一个"对象，此前已经缓存
在旧父对象下的子路径（如 `hello.txt`）会在这次交接的极短窗口内被内核
判定为不存在，返回一次性、偶发的 `ENOENT`（约几十次访问出现 1~2 次）。

修复方式：`Ino` 改为对 `segment + backend` 做哈希——backend 没变就
复用同一个 Inode（go-fuse 的 `addNewChild` 会命中已有节点直接复用，
不产生"父目录换身份"的抖动），backend 真的变了才产生新 Inode。修复后
用同样的压测跑了 60 次，0 失败；又追加了 20 次"插入新路由后立即访问"
和多轮 TTL 边界测试，均无异常。详细推理见
`internal/router/router.go` 里 `graftIno` 函数的注释。

这个过程本身也说明：涉及内核缓存、TTL 边界、并发访问的 FUSE 代码，
必须靠实际压测而不是单次功能验证来发现问题——本项目的开发方式就是
每改一版都重新编译、真实挂载、循环压测，而不是止步于"能跑通一次"。

## 设计取舍 / 已知限制

1. **路由 key 只能是顶层单段前缀**，见上文"关键设计取舍"一节。
2. **跨前缀操作不是原子的**：rename/link 跨前缀会返回 `EXDEV`，由上层
   （如 `mv`、应用自己的重试逻辑）决定是否降级为 copy+delete。
3. **缓存是尽力而为的最终一致性**，不是强一致：路由变更后，不同副本/
   不同时刻访问可能在 TTL 窗口内看到新旧不一致的路由。如果业务需要
   "改了立刻生效"，把 `cache.ttl_seconds` 调小即可（代价是对 Redis/
   MySQL 的查询压力变大），或者调用 `CachedStore.Invalidate`（当前
   代码未接管理接口暴露，可以自行加一个 HTTP/信号触发的管理端点）。
4. **权限/uid 模型**：路由层本身不做任何 uid/gid 转换，各 backend
   目录的权限语义原样透传。
5. 双层 FUSE 的性能代价：应用 -> 路由层 FUSE -> 内核 -> 已挂载的
   JuiceFS FUSE 客户端，metadata 密集型场景会比直接访问单个 JuiceFS
   挂载点多一次内核态/用户态往返。
