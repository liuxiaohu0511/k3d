# springboot-k3s
Deploy a simple set of SpringBoot microservices in K3S
# 部署流程
Spring Boot项目 → mvn打包jar → Docker构建镜像 → 推送到k3d集群本地registry → k8s部署(Deployment+Service+Ingress) → 浏览器访问
# 部署方案A
## 直接使用现成的多微服务demo(推进先跑起来)
### 1. 克隆项目
```bash
git clone https://github.com/imzhoukunqiang/springboot-k3s.git
cd springboot-k3s
git checkout 1.2.0
```

### 2. 确认集群还在
```bash
sudo k3d cluster list
kubectl get node
```
> 如果集群不在了，使用下面的命令创建
```bash
sudo k3d cluster create hello-k3d \
    --api-port 6550 \
    -p "9080:80@loadbalancer" -p "9443:443@loadbalancer" \
    --servers-memory 4g --agents-memory 4g \
    --servers 1 --agents 2
sudo cat /root/.kube/config > ~/.kube/config
```
### 3.创建命名空间并拉起三个服务
```bash
kubectl apply -f yamls/namespace.yaml
kubectl apply -f yamls/food.yaml
kubectl apply -f yamls/supermarket.yaml
kubectl apply -f yamls/digital.yaml
```

### 4.查看部署的状态
```bash
kubectl get pods -n springboot
kubectl get svc,ingress -n springboot
```
### 5. 访问呢浏览器
> ⚠️ 这里有个关键点，文章里提到但容易被忽略：Ingress 用的域名是 supermarket-127-0-0-1.nip.io，不需要改本地hosts文件——因为 nip.io 是一个通配符 DNS 服务，xxx-127-0-0-1.nip.io 会自动解析到 127.0.0.1，这是比之前教你改 /etc/hosts 更省事的域名区分技巧。
### 6. 浏览器访问
```text
http://supermarket-127-0-0-1.nip.io:9080
```
# nodeClusterIp的服务的验证和调试
ClusterIP 类型的 Service（比如 demo 里的 `food-svc` 和 `digital-svc`）设计上**只能被集群内部访问**，集群外部（比如你的浏览器）默认是访问不到的。这也是为什么之前文章里只给 `supermarket` 挂了 Ingress——因为它是唯一需要对外暴露的入口。

不过如果你是想**验证/调试**这两个内部服务，有好几种方法，从常用到临时排错都列一下：

## 方法1：kubectl port-forward（最常用，推荐）

不改任何 K8s 资源配置，临时把集群内的端口转发到你本机：

```bash
kubectl port-forward -n springboot svc/food-svc 8081:80
```

然后浏览器访问：
```
http://localhost:8081/products
```
应该能看到 food 服务返回的零食列表 JSON 数据。

同理验证 digital：
```bash
kubectl port-forward -n springboot svc/digital-svc 8082:80
```
访问 `http://localhost:8082/products`

> 注意：`port-forward` 是**前台阻塞命令**，会一直占着终端，`Ctrl+C` 停止转发。想跑后台可以加 `&`。

## 方法2：临时起一个调试 Pod，从集群内部访问

如果你想验证的是"集群内 DNS 服务发现"这个机制本身（而不是给外部用），可以像我们之前测试 Redis 那样：

```bash
kubectl run curl-test -n springboot --rm -it --image=curlimages/curl --restart=Never -- curl http://food-svc/products
```

这样能直观验证 supermarket 内部调用 `food-svc`/`digital-svc` 时走的就是这条路径——集群内部通过 Service 名字直接访问，不需要端口转发也不需要外部IP。

## 方法3：临时改成 NodePort（会修改资源配置，用完记得改回来）

如果你想更接近"真的对外暴露"的效果，可以临时改类型：

```bash
kubectl patch svc food-svc -n springboot -p '{"spec": {"type": "NodePort"}}'
```

查看分配到的端口：
```bash
kubectl get svc food-svc -n springboot
```
会看到类似 `80:31234/TCP` 这样的映射，然后可以通过 `节点IP:31234` 访问（在k3d场景下需要额外映射端口，相对麻烦，不如方法1简单）。

改完记得**改回 ClusterIP**，避免破坏原本的服务边界设计：
```bash
kubectl patch svc food-svc -n springboot -p '{"spec": {"type": "ClusterIP"}}'
```

## 方法4：也给它们加 Ingress（如果你确实想长期对外暴露）

参考你之前学的 Ingress 模板，加一条新规则：

```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: food-ingress
  namespace: springboot
  annotations:
    ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - host: food-127-0-0-1.nip.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: food-svc
            port:
              number: 80
EOF
```
访问 `http://food-127-0-0-1.nip.io:9080/products`

---

## 建议怎么选

| 目的 | 推荐方法 |
|---|---|
| 临时看一眼数据/调试 | **方法1 port-forward**（最简单，用完直接Ctrl+C，不留痕迹） |
| 想理解Service DNS机制本身 | **方法2 调试Pod**（这才是supermarket实际调用food/digital的方式） |
| 真的要长期对外暴露 | **方法4 加Ingress** |
| 方法3 NodePort | 不太推荐，在k3d场景下比较麻烦，port-forward能替代大部分场景 |

如果你的目的只是"看看 food/digital 服务本身返回的数据长什么样"，直接用**方法1**最快；如果是想理解"为什么 supermarket 能拿到数据"这个原理，用**方法2**更有教学意义。


### 补充说明
> 其实无论单个端口还是多个端口，Dockerfile里面的`EXPOSE`和Deploment中的`containerPort`永远都只是声明和标签，真正生效的是应用自己监听了什么，以及Service/Ingress的targetPort/port 具体转发到哪儿。用`name`引用代替每次写端口数字的值，是多端口场景下避免出错的关键技巧。

### 下面几种情况
**不需要"必须"一致，但如果不一致，会导致某些便利功能失效，具体分两种场景说明。**

## 场景1：`docker run` 手动指定端口映射（`-p`），完全不需要一致

```bash
docker run -p 9000:8081 my-app:1.0
```
这里的 `-p 9000:8081` 是你**手动**告诉 Docker："宿主机9000 → 容器内8081"。这条映射规则跟 Dockerfile 里的 `EXPOSE` 写的是啥**完全没关系**，就算 Dockerfile 写 `EXPOSE 8080`，你依然可以用 `-p` 手动映射到容器内任何端口，只要那个端口是应用**实际在监听**的就行。

真正决定这条命令能不能用的，只有一个条件：**冒号右边的端口（容器内端口），必须是应用进程实际监听的端口**。跟 `EXPOSE` 写的数字没有强制关联。

```bash
# 假设容器内应用实际监听8081
docker run -p 9000:8081 my-app:1.0   # ✅ 能通，因为8081是真实监听端口
docker run -p 9000:8080 my-app:1.0   # ❌ 打不通，容器内8080根本没人监听，即使Dockerfile写着EXPOSE 8080
```

## 场景2：`docker run -P`（大写P，自动映射），这时候才依赖 `EXPOSE`

```bash
docker run -P my-app:1.0
```
`-P` 的意思是"我懒得手动写端口映射，你(Docker)按 `EXPOSE` 声明的端口，随机映射到宿主机的可用端口"。

这种情况下，**`EXPOSE` 写的端口就必须和应用实际监听的端口一致**，否则 Docker 会把一个"没人监听"的端口映射出来，你访问了也是拒绝连接。

```bash
docker port <容器ID>
# 输出类似: 8081/tcp -> 0.0.0.0:32768
```
这里 `8081` 就是从 `EXPOSE` 读出来的，如果这个数字跟应用实际监听的对不上，映射出来的端口访问了也没用。

## 落到你之前发现的那个 digital 服务的例子

```dockerfile
EXPOSE 8081
```
```yaml
server:
  port: 8081  # application.yaml，跟EXPOSE一致
```

如果你直接拿这个镜像单独用 `docker run` 测试（脱离K8s，不经过那层 `server.port=80` 的环境变量覆盖）：

```bash
docker run -p 8081:8081 ghcr.io/imzhoukunqiang/springboot-k3s-digital:1.2.0
```
这样是能正常访问的，因为 Dockerfile 的 `EXPOSE 8081` 和 `application.yaml` 的 `server.port: 8081` 本来就是自洽一致的——问题只出在进了 K8s 之后，`env: server.port='80'` 这层覆盖把实际监听端口改成了80，跟 `EXPOSE 8081` 才产生了脱节。

## 总结成一句话

| 使用方式 | `EXPOSE` 是否需要跟实际监听端口一致 |
|---|---|
| `docker run -p 宿主机端口:容器端口` | **不需要**，你手动指定的容器端口才是唯一起作用的 |
| `docker run -P`（自动映射） | **需要**，否则映射出来的端口是空的，访问不通 |
| K8s（Deployment/Service） | **完全不看`EXPOSE`**，K8s压根不读这个字段，只看 `containerPort`(纯文档) 和应用实际监听端口 |

**最佳实践建议**：不管走哪种方式，最保险的做法是让 `EXPOSE`、`application.yaml`（或环境变量最终生效值）、`containerPort`、`targetPort` 这四处**始终保持一致**，即使技术上只有 `docker run -P` 这一种场景强制要求一致——这样能避免像你这次深挖 digital 服务时遇到的"层层不一致，靠翻源码才搞清楚真相"的排查成本。