# jfs-router

一个轻量级的"超级挂载"路由层：对应用暴露**单一 FUSE 挂载点**，内部按
**路径前缀**把读写请求转发给对应的 JuiceFS（或任意 POSIX 目录）实例。

适用场景：每次读写的路径前缀在运行期间是固定的，即"同一次操作从头到尾
只落在一个后端实例上"，不需要跨实例的原子事务保证。

## 设计核心

不重新实现 Read/Write/Rename 等系统调用的转发逻辑，而是复用 go-fuse
官方、经过充分测试的 `fs.LoopbackNode`（基于文件描述符的系统调用，正确
处理硬链接去重、xattr、原子 rename 等细节，规避了"先拼路径字符串再操作"
方案常见的 TOCTOU 竞态）。

本项目只做一件事：把每条路由规则对应的一整棵 `LoopbackRoot` 子树，
**过继（graft）**到路由层自己维护的一棵静态虚拟目录树的正确位置上。
过继之后，FUSE 内核请求一旦落入某个前缀子树，就完全由对应的
`LoopbackNode` 独立处理，路由层不再介入——符合"一次读写只属于一个 jfs
实例"的场景，也是本方案足够轻量的关键：**路由只在挂载时计算一次，
运行期间零额外路由开销**。

```
/data                          <- 应用看到的唯一挂载点
├── tenantA/  ──┐
│   └── ...     ├─ 过继自 LoopbackRoot("/mnt/.jfs-tenantA")，之后完全
│               │  由该子树自己的 LoopbackNode 处理，不再经过路由层
├── tenantB/  ──┘
│   └── ...     ─ 过继自 LoopbackRoot("/mnt/.jfs-tenantB")
└── shared/
    └── logs/  ─ 过继自 LoopbackRoot("/mnt/.jfs-logs")
                  （shared 是纯虚拟目录，仅用于承载更深的前缀，
                   不属于任何后端实例）
```

## 目录结构

```
cmd/jfs-router/main.go        入口：加载配置、挂载、信号处理与优雅卸载
internal/config/config.go     配置结构体与启动期严格校验
internal/router/router.go     路由核心：静态虚拟目录树 + LoopbackRoot 过继
configs/config.example.json   配置示例
deploy/jfs-router.service     systemd 部署示例
```

## 配置文件

见 `configs/config.example.json`，字段说明在 `internal/config/config.go`
的注释中。选择 JSON 而非 YAML 是为了不引入额外第三方依赖。

关键校验规则（启动时执行，任何一条不满足直接拒绝启动，不会带着错误配置
跑起来）：

- 每个 `prefix` 必须是绝对路径，且不能是 `/`。
- 每个 `backend` 必须已存在、是目录、且路由层进程对其有读写权限
  （会实际创建再删除一个探测文件来验证，而不是只看权限位）。
- 前缀之间不允许有"祖先关系"，例如 `/a` 和 `/a/b`不能同时配置——
  因为 `/a` 对应的后端子树会整体接管其下所有路径，`/a/b` 会永远不可达。
- 前缀不允许重复。

## 构建

```bash
go build -o bin/jfs-router ./cmd/jfs-router
```

依赖只有 `github.com/hanwen/go-fuse/v2`（间接依赖 `golang.org/x/sys`）。

> **注**：`go.mod` 里有一条
> `replace golang.org/x/sys => github.com/golang/sys v0.28.0`。
> 这是为了在本次开发所用的沙箱环境（无法访问 `golang.org` 域名，
> 只能访问 `github.com`）中拉取依赖而加的临时绕行方案。在有正常网络
> 访问（能走标准 `GOPROXY`）的开发/CI 环境中，可以删掉这行 `replace`，
> 效果完全一致。

## 运行

```bash
sudo ./bin/jfs-router -config configs/config.example.json
```

- 挂载点若不存在会自动创建；若已存在但非空目录，会拒绝挂载（避免遮挡
  已有数据）。
- 收到 `SIGINT`/`SIGTERM` 会尝试优雅卸载（对正在关闭中、可能还有极少量
  文件句柄未释放的场景做了短暂重试）。
- 生产部署参考 `deploy/jfs-router.service`。

## 已验证的行为（在沙箱内实际挂载 + strace 测试过，非纸面推导）

- 基本读写：通过路由挂载点写入的文件，确实落到对应前缀的真实 backend
  目录下；反之亦然。
- 子目录读写、`mkdir`/`rmdir`、硬链接（`ln`）均正常。
- `df -h` 在不同前缀路径下能看到各自 backend 所在文件系统的容量信息
  （`Statfs` 由各子树的 `LoopbackNode` 独立响应，无需路由层做聚合）。
- **跨前缀 rename**：用 `strace` 确认 `renameat2` 系统调用返回
  `EXDEV`（`Invalid cross-device link`）。这不是靠"两个 backend 是否
  同一物理设备"判断的，而是 go-fuse 的 `LoopbackNode.Rename` 显式比较
  两侧节点所属的 `LoopbackRoot` 指针，跨子树一律返回 `EXDEV`——哪怕两个
  backend 目录碰巧在同一块盘上，跨前缀 rename 依然会被正确拒绝。
  `mv` 命令看到 `EXDEV` 会自动降级为 copy+unlink（这是 `mv` 自身的标准
  行为，语义上和真实文件系统跨分区 `mv` 完全一致，本项目不需要、也没有
  做任何特殊处理）。
- 配置校验：前缀祖先冲突、前缀重复均会在启动时被正确拒绝（退出码 1，
  错误信息里指明具体冲突的两条规则）。
- `SIGTERM` 触发优雅卸载：日志显示"收到信号 -> 卸载中 -> 进程退出"，
  卸载后 `mount` 命令确认挂载点已清理。

## 设计取舍 / 已知限制

1. **路由表在挂载生命周期内是静态的，不支持运行时热更新**。这是刻意
   的简化，对应"读写路径前缀固定"的使用场景。如需变更前缀映射，重启
   进程（新配置生效前会先卸载旧挂载点）即可。如果未来确实需要热更新，
   可以在 `Root` 上实现动态版本：用 `atomic.Value` 存路由表快照 + 自定义
   `Lookup`（而不是现在这种一次性 `OnAdd` 静态过继），但要额外处理"路由
   变更时已打开的文件句柄该指向新路由还是旧路由"的一致性问题——这正是
   之前讨论中提到的复杂度来源，本版本按需求明确不引入。
2. **跨前缀操作不是原子的**：rename/link 跨前缀会返回 `EXDEV`，由上层
   （如 `mv`、应用自己的重试逻辑）决定是否降级为 copy+delete。这是"多个
   独立文件系统联合挂载"这类方案的通用限制，不是本实现的 bug。
3. **权限/uid 模型**：路由层本身不做任何 uid/gid 转换或权限收敛，各
   backend 目录的权限语义原样透传。多个 jfs 实例的 ACL 策略建议保持
   一致，否则同一超级挂载下不同目录权限语义不统一，容易让应用困惑。
4. 双层 FUSE 的性能代价：应用 -> 路由层 FUSE -> 内核 -> 已挂载的
   JuiceFS FUSE 客户端，metadata 密集型场景（大量小文件 stat/list）会
   比直接访问单个 JuiceFS 挂载点多一次内核态/用户态往返。如果这个开销
   在你的场景下不可接受，需要考虑绕过底层 JuiceFS 的 POSIX 挂载、直接
   用 JuiceFS 的 SDK/内部 meta+data 接口对接，但那是完全不同量级的
   实现复杂度，超出"轻量级路由"的设计目标，本版本不采用。
