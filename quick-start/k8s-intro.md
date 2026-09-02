## 这里介绍下k8s常见的crd的一些定义和属性介绍
### Deploment
```yaml
apiVersion: apps/v1        # API版本，Deployment固定用apps/v1
kind: Deployment           # 资源类型：部署控制器，管理Pod的创建、更新、回滚
metadata:
  name: my-app-deployment  # Deployment的名字，集群内同namespace下唯一
  namespace: default       # 部署到哪个命名空间，不写默认是default
  labels:                  # 给Deployment本身打标签，用于筛选/管理，非必需但建议加
    app: my-app
    tier: backend
spec:
  replicas: 3               # 期望运行的Pod副本数量，控制器会自动维持这个数量
  revisionHistoryLimit: 10  # 保留多少个历史版本(用于回滚)，默认10，可按需调整
  strategy:                 # 滚动更新策略
    type: RollingUpdate     # 更新方式：RollingUpdate(滚动更新，默认) 或 Recreate(先全删再建，会有服务中断)
    rollingUpdate:
      maxSurge: 1           # 更新过程中最多可以超出期望副本数多少个(1或百分比如"25%")
      maxUnavailable: 0     # 更新过程中最多允许多少个Pod不可用(设0表示更新时始终保证全量可用)
  selector:                 # 用来匹配哪些Pod归这个Deployment管，必须和下面template.metadata.labels一致
    matchLabels:
      app: my-app
  template:                 # Pod模板，定义实际创建出来的Pod长什么样
    metadata:
      labels:                # Pod的标签，必须包含上面selector.matchLabels里的内容
        app: my-app
        tier: backend
    spec:
      restartPolicy: Always  # 容器退出后的重启策略：Always(总是重启，Deployment场景固定用这个)
      terminationGracePeriodSeconds: 30  # Pod删除时，给容器多长时间优雅关闭(收到SIGTERM后)，超时才SIGKILL强杀
      containers:
        - name: my-app-container   # 容器名字，一个Pod可以有多个容器，这里是主容器
          image: nginx:1.25        # 镜像名+版本号，务必锁定版本号，避免latest带来的不确定性
          imagePullPolicy: IfNotPresent  # 镜像拉取策略：Always(每次都拉)/IfNotPresent(本地没有才拉)/Never(只用本地)
          ports:
            - containerPort: 80    # 容器内部监听的端口，仅做声明用途，不代表实际暴露
              name: http           # 端口起个名字，方便Service引用
              protocol: TCP        # 协议，TCP或UDP
          env:                     # 环境变量
            - name: ENV_NAME
              value: "production"
            - name: SECRET_KEY     # 从Secret中读取敏感信息，而不是明文写value
              valueFrom:
                secretKeyRef:
                  name: my-secret
                  key: secret-key
          resources:               # 资源限制，非常重要，生产环境务必配置，避免单个Pod把节点资源吃光
            requests:               # 请求资源：调度器依据这个决定Pod放到哪个节点，是"最低保证"
              cpu: "250m"           # 250毫核，即0.25个CPU核心
              memory: "256Mi"       # 256MB内存
            limits:                 # 限制资源：容器能用到的资源上限，超过CPU会被限流，超过内存会被OOMKill
              cpu: "500m"
              memory: "512Mi"
          livenessProbe:            # 存活探针：判断容器是否"活着"，失败会被kubelet重启容器
            httpGet:
              path: /healthz
              port: 80
            initialDelaySeconds: 10  # 容器启动后多久开始探测(给应用启动留时间)
            periodSeconds: 10        # 探测间隔
            timeoutSeconds: 3        # 单次探测超时时间
            failureThreshold: 3      # 连续失败几次才判定为不健康
          readinessProbe:            # 就绪探针：判断容器是否"准备好接流量"，失败会从Service的Endpoints里摘除
            httpGet:
              path: /ready
              port: 80
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:              # 挂载卷到容器内的路径
            - name: config-volume
              mountPath: /etc/config
              readOnly: true
      volumes:                      # 定义Pod级别的存储卷，来源可以是ConfigMap/Secret/PVC等
        - name: config-volume
          configMap:
            name: my-app-config
      nodeSelector:                 # 简单的节点选择器，只调度到带有对应标签的节点上
        disktype: ssd
      affinity:                     # 更复杂的调度规则(亲和性/反亲和性)，比如让多个副本尽量分散到不同节点
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: my-app
                topologyKey: kubernetes.io/hostname
      tolerations:                  # 容忍节点的污点(Taint)，允许调度到有特定污点的节点上
        - key: "special"
          operator: "Equal"
          value: "true"
          effect: "NoSchedule"
```

### Service
```bash
apiVersion: v1              # Service固定用v1
kind: Service
metadata:
  name: my-app-svc          # Service名字，集群内DNS会自动生成 my-app-svc.default.svc.cluster.local
  namespace: default
  labels:
    app: my-app
spec:
  type: ClusterIP            # Service类型，常见4种：
                             # ClusterIP(默认，仅集群内可访问)
                             # NodePort(在每个节点开一个固定端口，集群外可通过节点IP:端口访问)
                             # LoadBalancer(云厂商提供的外部负载均衡器，本地k3d一般不直接支持，需配合MetalLB等)
                             # ExternalName(把Service映射到外部DNS名，不做代理转发)
  selector:                  # 通过标签选择器找到对应的Pod，必须和Deployment里Pod的labels一致
    app: my-app
  ports:
    - name: http              # 端口名字，一个Service可以暴露多个端口时用来区分
      protocol: TCP           # 协议
      port: 80                # Service自己监听的端口(集群内其他服务通过这个端口访问)
      targetPort: 80          # 转发到Pod容器内的哪个端口(可以填端口号，也可以填containerPort里定义的name如"http")
      # nodePort: 30080       # 仅type为NodePort时需要，指定固定的节点端口(范围默认30000-32767)，不写会随机分配
  sessionAffinity: None       # 会话保持策略：None(不保持，默认轮询) 或 ClientIP(同一客户端IP始终转发到同一个Pod)
```
### Ingress
```bash
apiVersion: networking.k8s.io/v1   # Ingress固定用这个版本(v1，老版本用extensions/v1beta1已废弃)
kind: Ingress
metadata:
  name: my-app-ingress
  namespace: default
  annotations:                      # 注解，用来配置Ingress Controller的具体行为，不同Controller(Traefik/Nginx等)注解不通用
    ingress.kubernetes.io/ssl-redirect: "false"   # 是否强制跳转https，本地测试环境一般关掉
    # traefik.ingress.kubernetes.io/router.middlewares: default-my-middleware@kubernetescrd  # 引用Traefik中间件(如路径改写、限流等)
    # nginx.ingress.kubernetes.io/rewrite-target: /  # 如果用的是Nginx Ingress Controller，这是常见的路径改写注解
spec:
  ingressClassName: traefik          # 指定用哪个Ingress Controller处理，k3d默认自带traefik；不写的话走集群默认IngressClass
  tls:                                # 配置HTTPS证书(可选，没有HTTPS需求可以整段删掉)
    - hosts:
        - my-app.local
      secretName: my-app-tls-secret  # 提前创建好的TLS证书Secret(kubectl create secret tls)
  rules:                              # 路由规则，可以配置多条，按顺序匹配
    - host: my-app.local              # 按域名区分，不写host则匹配所有域名(需要配合本地hosts文件解析或真实DNS)
      http:
        paths:
          - path: /                   # 匹配的URL路径
            pathType: Prefix          # 匹配方式：
                                       # Prefix(前缀匹配，如"/api"会匹配"/api"和"/api/xxx")
                                       # Exact(精确匹配，完全一致才行)
                                       # ImplementationSpecific(交给具体Controller自行解释)
            backend:
              service:
                name: my-app-svc      # 转发到哪个Service
                port:
                  number: 80          # Service暴露的端口号(也可以用name代替number，写Service里定义的端口名)
```

### 三者的关系是
```bash
浏览器请求
   ↓
Ingress（按域名/路径路由，入口网关）
   ↓
Service（固定虚拟IP+DNS名，负载均衡到Pod）
   ↓
Pod（Deployment管理的实际运行实例，会不断创建/销毁/重建）
```
# 还有哪些CRD
CRD（Custom Resource Definition，自定义资源定义）是 Kubernetes 的扩展机制，允许第三方给集群"发明"新的资源类型。结合你之前踩的 Traefik CRD 的坑，这里按**使用场景**分类，整理一份常见 CRD 清单。

## 1. Ingress Controller 相关

**Traefik**（k3d/k3s 默认自带）
- `IngressRoute` — Traefik 自己的路由资源，比标准 Ingress 更灵活（支持更复杂的匹配规则）
- `Middleware` — 你之前想用的路径改写、限流、认证等中间件
- `TLSOption` / `TLSStore` — TLS 相关配置

**Nginx Ingress Controller**
- 主要靠 annotation 而非 CRD，CRD 用得较少

## 2. 服务网格（Service Mesh）

**Istio**
- `VirtualService` — 流量路由规则（灰度发布、A/B测试常用）
- `DestinationRule` — 定义流量策略（负载均衡算法、熔断等）
- `Gateway` — 网格入口网关配置
- `PeerAuthentication` / `AuthorizationPolicy` — mTLS 和访问控制

**Linkerd**
- `ServiceProfile` — 定义服务的路由和重试策略

## 3. 证书管理

**cert-manager**（最常用，几乎是标配）
- `Certificate` — 声明需要的证书
- `Issuer` / `ClusterIssuer` — 证书签发者（如 Let's Encrypt）
- `CertificateRequest` — 证书请求记录

## 4. 监控告警

**Prometheus Operator**
- `ServiceMonitor` — 声明式定义要监控哪些 Service
- `PodMonitor` — 类似，针对 Pod
- `PrometheusRule` — 告警规则和记录规则
- `Prometheus` / `Alertmanager` — 定义 Prometheus/Alertmanager 实例本身

## 5. GitOps / 持续部署

**ArgoCD**
- `Application` — 定义一个要同步的应用（关联 Git 仓库和目标集群）
- `AppProject` — 应用分组和权限管理

**Flux**
- `GitRepository` / `HelmRepository` — 声明数据源
- `Kustomization` / `HelmRelease` — 声明部署内容

## 6. 存储相关

**Rook / Ceph**
- `CephCluster`、`CephBlockPool` 等 — 声明式管理 Ceph 存储集群

**Longhorn**（k3s 生态常见，Rancher 出品）
- `Volume`、`Engine`、`Replica` 等 — 分布式块存储管理

## 7. 数据库 Operator

- **Postgres**: `PostgresCluster`（CrunchyData）、`Cluster`（CloudNativePG）
- **MySQL**: `MySQLCluster`（各家 Operator 实现不同）
- **Redis**: `RedisCluster` / `RedisFailover`（Redis Operator）
- **MongoDB**: `MongoDBCommunity` / `MongoDB`（MongoDB Community/Enterprise Operator）

> 这类数据库 Operator 挺适合你——比起手写 Deployment 部署 Redis（像你前面那样），用 Operator 可以自动处理主从、故障转移、备份这些运维细节。

## 8. 网络策略增强

**Calico**
- `NetworkPolicy`（增强版）、`GlobalNetworkPolicy`、`IPPool` 等

**Cilium**
- `CiliumNetworkPolicy`、`CiliumEndpoint` 等

---

## 怎么查看你集群里实际装了哪些 CRD

```bash
kubectl get crd
```

看某个具体 CRD 的详细字段说明（类似 `kubectl explain`）：
```bash
kubectl explain middleware.traefik.io
```

看某个分组下有哪些 CRD：
```bash
kubectl get crd | grep traefik
kubectl get crd | grep cert-manager
```

---

**结合你目前的实践**：你已经在 k3d 里用了 Deployment/Service/Ingress 这些内置资源，如果接下来想更进一步，比较自然的下一步是装 **cert-manager**（本地测试可以用自签证书体验 HTTPS 流程）或者试试 **Prometheus Operator**（监控你现在部署的 Tomcat/Redis/Nginx），这两个在实际生产环境里也是高频出现的组件，练习价值比较高。