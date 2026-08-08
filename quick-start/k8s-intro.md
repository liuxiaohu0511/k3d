# k3d 集群内置资源类型（`kubectl api-resources` 输出，附说明）

## 核心组 `v1`（最基础，无需额外声明group）

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| bindings | | Binding | 把Pod绑定到具体Node，通常是调度器内部使用，不需要手动操作 |
| componentstatuses | cs | ComponentStatus | 查看控制平面组件(如etcd、scheduler)的健康状态，较老的API，新版集群已弃用 |
| configmaps | cm | ConfigMap | 存储非敏感的配置数据(键值对/文件)，可挂载给Pod当环境变量或配置文件 |
| endpoints | ep | Endpoints | 记录Service背后实际的Pod IP+端口列表，一般由K8s自动维护，不手写 |
| events | ev | Event | 集群内发生的各种事件记录(如Pod调度失败、镜像拉取失败等)，排错必看 |
| limitranges | limits | LimitRange | 限制某个命名空间下Pod/容器的资源请求上下限，防止有人不写resources吃满资源 |
| namespaces | ns | Namespace | 逻辑隔离空间，把集群资源分组管理(如default、kube-system) |
| nodes | no | Node | 集群里的工作节点，你之前用`kubectl get node`查看过 |
| persistentvolumeclaims | pvc | PersistentVolumeClaim | Pod对存储的"申请单"，声明需要多大空间、什么访问模式 |
| persistentvolumes | pv | PersistentVolume | 集群级别的实际存储资源，PVC会绑定到某个PV上 |
| pods | po | Pod | 最小调度单位，一个或多个容器共享网络和存储 |
| podtemplates | | PodTemplate | 预定义的Pod模板，供其他控制器复用，直接用的场景较少 |
| replicationcontrollers | rc | ReplicationController | 老版本的副本控制器，功能被Deployment取代，新项目不建议用 |
| resourcequotas | quota | ResourceQuota | 限制整个命名空间总共能用多少CPU/内存/Pod数量等 |
| secrets | | Secret | 存储敏感信息(密码、token、证书)，内容会base64编码(注意不是加密) |
| serviceaccounts | sa | ServiceAccount | Pod运行时的身份账号，配合RBAC控制Pod能访问哪些API |
| services | svc | Service | 你已用到，给一组Pod提供固定访问入口+负载均衡 |

## 准入控制 `admissionregistration.k8s.io/v1`

| NAME | KIND | 说明 |
|---|---|---|
| mutatingwebhookconfigurations | MutatingWebhookConfiguration | 资源创建/更新时，先经过webhook自动"修改"内容(如自动注入sidecar容器) |
| validatingwebhookconfigurations | ValidatingWebhookConfiguration | 资源创建/更新时，先经过webhook"校验"是否合规，不合规就拒绝 |

## API 扩展

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| customresourcedefinitions | crd, crds | CustomResourceDefinition | 定义新资源类型本身，就是上一个问题讲的CRD |
| apiservices | | APIService | 把额外的API组注册到kube-apiserver里，用于聚合API(如metrics-server) |

## 应用管理 `apps/v1`（最常打交道的一组）

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| controllerrevisions | | ControllerRevision | 记录StatefulSet/DaemonSet的历史版本，用于回滚，一般不直接操作 |
| daemonsets | ds | DaemonSet | 保证每个(或部分)节点都跑一个Pod副本，常用于日志采集/监控agent |
| deployments | deploy | Deployment | 你已用到，管理无状态应用，支持滚动更新和回滚 |
| replicasets | rs | ReplicaSet | Deployment底层实际管理Pod数量的对象，一般不直接操作它 |
| statefulsets | sts | StatefulSet | 管理有状态应用(数据库等)，提供稳定的Pod名字、网络标识和独立存储 |

## 认证与授权

| NAME | KIND | 说明 |
|---|---|---|
| tokenreviews | TokenReview | 校验一个token是否有效，通常是API内部调用，不手动创建 |
| localsubjectaccessreviews | LocalSubjectAccessReview | 检查某用户在指定命名空间是否有权限做某操作 |
| selfsubjectaccessreviews | SelfSubjectAccessReview | 检查"我自己"是否有权限做某操作，常用于调试RBAC配置 |
| selfsubjectrulesreviews | SelfSubjectRulesReview | 查询"我自己"在某命名空间下具体拥有哪些权限规则 |
| subjectaccessreviews | SubjectAccessReview | 检查某用户/组是否有权限做某操作，全局版本 |

## 自动扩缩容 / 任务调度

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| horizontalpodautoscalers | hpa | HorizontalPodAutoscaler | 根据CPU/内存/自定义指标，自动增减Deployment的Pod副本数 |
| cronjobs | cj | CronJob | 定时任务，语法类似Linux crontab，比如每天凌晨跑一次备份脚本 |
| jobs | | Job | 一次性任务，跑完即结束，适合数据迁移、批处理场景 |

## 证书 / 协调 / 服务发现

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| certificatesigningrequests | csr | CertificateSigningRequest | 证书签名请求，常用于给节点或用户签发集群内部TLS证书 |
| leases | Lease | 分布式锁/租约机制，用于组件选主(如多副本controller-manager选leader) |
| endpointslices | EndpointSlice | Endpoints的升级版，大规模集群下性能更好，功能上等同于Endpoints |
| events | ev | Event(新版) | 和核心组的Event功能一样，属于新API分组，逐步替代旧版 |

## 流控

| NAME | KIND | 说明 |
|---|---|---|
| flowschemas | FlowSchema | 定义API请求如何被分类和限流，控制平面稳定性相关，一般不需要手动配置 |
| prioritylevelconfigurations | PriorityLevelConfiguration | 配合FlowSchema，定义不同优先级请求的并发处理能力 |

## k3s / Helm 专属（**这组是k3d/k3s特有的，标准K8s没有**）

| NAME | KIND | 说明 |
|---|---|---|
| helmchartconfigs | HelmChartConfig | 给k3s内置的Helm Chart(如Traefik)提供自定义配置覆盖 |
| helmcharts | HelmChart | k3s内置的声明式Helm部署方式，写个YAML就能自动装Helm Chart |
| addons | Addon | k3s自带组件的声明式管理(如CoreDNS、Traefik等的自动安装) |

> 💡 这就是为什么你的 k3d 集群"自带" Traefik——它是通过 `HelmChart`/`Addon` 这套 k3s 专属机制自动安装的，标准 kubeadm 搭建的集群不会有这些资源类型。

## 监控指标

| NAME | KIND | 说明 |
|---|---|---|
| nodes | NodeMetrics | 节点的实时CPU/内存使用量，`kubectl top node`背后用的就是这个 |
| pods | PodMetrics | Pod的实时CPU/内存使用量，`kubectl top pod`背后用的就是这个，需要装metrics-server |

## 网络 `networking.k8s.io/v1`

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| ingressclasses | | IngressClass | 声明集群里有哪些Ingress Controller可用(如traefik、nginx) |
| ingresses | ing | Ingress | 你已用到，HTTP(S)路由规则的标准资源 |
| networkpolicies | netpol | NetworkPolicy | 类似防火墙规则，控制Pod之间/Pod与外部的网络访问权限 |

## 节点 / 调度 / 中断预算

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| runtimeclasses | RuntimeClass | 指定Pod使用哪种容器运行时(如runc、kata)，一般用默认的即可 |
| poddisruptionbudgets | pdb | PodDisruptionBudget | 限制维护/驱逐节点时最多能同时停掉多少Pod，保障服务不中断 |
| priorityclasses | pc | PriorityClass | 定义Pod调度优先级，资源紧张时优先保证高优先级Pod运行 |

## 权限控制 `rbac.authorization.k8s.io/v1`

| NAME | KIND | 说明 |
|---|---|---|
| clusterrolebindings | ClusterRoleBinding | 把ClusterRole权限绑定给用户/组/ServiceAccount，作用于整个集群 |
| clusterroles | ClusterRole | 定义集群级别的权限规则集合(可跨命名空间) |
| rolebindings | RoleBinding | 把Role权限绑定给用户/组/ServiceAccount，作用于单个命名空间 |
| roles | Role | 定义命名空间级别的权限规则集合 |

## 存储 `storage.k8s.io/v1`

| NAME | SHORTNAMES | KIND | 说明 |
|---|---|---|---|
| csidrivers | CSIDriver | 声明集群里安装了哪些CSI存储驱动(如AWS EBS、Ceph等) |
| csinodes | CSINode | 记录每个节点支持哪些CSI驱动，K8s内部使用 |
| csistoragecapacities | CSIStorageCapacity | CSI驱动上报的存储容量信息，用于调度决策 |
| storageclasses | sc | StorageClass | 动态存储供应的模板，声明"创建PVC时该用哪种存储" |
| volumeattachments | VolumeAttachment | 记录卷挂载到节点的状态，K8s内部使用，不手动创建 |

## Traefik CRD（`traefik.containo.us/v1alpha1`）

> 🎯 之前 `Middleware` 报错的答案就在这——你集群实际的 apiVersion 是 `traefik.containo.us/v1alpha1`，对应 Traefik v2.x，不是较新的 `traefik.io/v1alpha1`。

| NAME | KIND | 说明 |
|---|---|---|
| ingressroutes | IngressRoute | Traefik自己的路由资源，比标准Ingress更灵活(支持更复杂匹配规则/中间件链) |
| ingressroutetcps | IngressRouteTCP | 针对TCP协议(非HTTP)的路由规则，比如给Redis做四层转发 |
| ingressrouteudps | IngressRouteUDP | 针对UDP协议的路由规则 |
| middlewares | Middleware | 你之前想用的中间件资源，支持路径改写(StripPrefix)、限流、认证、重试等 |
| middlewaretcps | MiddlewareTCP | TCP层的中间件(功能比HTTP中间件少很多) |
| serverstransports | ServersTransport | 配置Traefik到后端服务的连接细节(如TLS校验、超时时间) |
| tlsoptions | TLSOption | 自定义TLS协议版本、加密套件等安全参数 |
| tlsstores | TLSStore | 配置默认TLS证书存储位置 |
| traefikservices | TraefikService | Traefik增强版的Service抽象，支持加权轮询、镜像流量等高级负载均衡策略 |

---

## 结合你实践路径的建议

对照你目前踩过的坑和用过的资源，接下来比较有价值的学习方向：

1. **`ConfigMap`/`Secret`** — 你的 Tomcat/Nginx 部署目前配置都是硬编码在镜像里的，可以试着外部化
2. **`PersistentVolumeClaim`** — 你的 Redis 现在重启就丢数据，加个PVC能解决
3. **`NetworkPolicy`** — 体验一下限制 Redis 只能被 Tomcat/Nginx 访问，其他 Pod 访问不了，更贴近生产安全实践
4. **`IngressRoute`**（Traefik专属）— 既然你已经在纠结 `Middleware` 怎么配了，不如直接用 `IngressRoute` 替代标准 `Ingress`，原生支持中间件链，配置起来比"Ingress+annotation+Middleware"这种绕弯的方式更直观