### https://coding.gs/2024/04/03/k3d/getting-started-with-k3d/


## 创建一个k3d集群

```bash
sudo k3d cluster create hello-k3d \
    --api-port 6550 \
    -p "9080:80@loadbalancer" -p "9443:443@loadbalancer" \
    --servers-memory 4g \
    --agents-memory 4g \
    --servers 1 \
    --agents 2 
```
```bash
sudo kubectl get node
```

## 创建Deployment
```bash
# v1版本
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tomcat-deployment
  labels:
    app: tomcat
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tomcat
  template:
    metadata:
      labels:
        app: tomcat
    spec:
      containers:
        - name: tomcat
          image: tomcat:10
          ports:
            - containerPort: 8080

EOF
# v2版本
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tomcat-deployment
  labels:
    app: tomcat
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tomcat
  template:
    metadata:
      labels:
        app: tomcat
    spec:
      containers:
        - name: tomcat
          image: tomcat:10
          command: ["/bin/sh", "-c"]
          args:
            - "cp -r /usr/local/tomcat/webapps.dist/* /usr/local/tomcat/webapps/ && catalina.sh run"
          ports:
            - containerPort: 8080
EOF
```
##### 查看部署状态，等待部署完成
```
kubectl rollout status deployment/tomcat-deployment
```
##### 查看pod
```
kubectl get pod
```

## 创建service
#### service把多个pod组合在一起，给他们分配统一的入口地址，这样。就可以通过这个地址访问任意的pod提供的服务。service类似于pod的代理或者负载均衡器
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: tomcat-svc
spec:
  selector:
    app: tomcat
  ports:
    - protocol: TCP
      port: 80 
      targetPort: 8080
EOF
```
## 创建ingress
```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tomcat
  annotations:
    ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: tomcat-svc
            port:
              number: 80
EOF
```
#### 打开浏览器，访问http://<IP>:9080/

## 清理一个集群
```bash
sudo k3d cluster delete hello-k3d
```



## 搭建redis和nginx服务
### 创建redis的deploment
```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-deployment
  labels:
    app: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
        - name: redis
          image: redis:7
          ports:
            - containerPort: 6379
EOF
```
### 创建redis的service
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: redis-svc
spec:
  selector:
    app: redis
  ports:
    - protocol: TCP
      port: 6379
      targetPort: 6379
EOF
```

### 测试redis是否ok
```bash
kubectl run redis-test --rm -it --image=redis:7 --restart=Never -- redis-cli -h redis-svc ping
```

## 创建nginx
### 创建deploment
```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          ports:
            - containerPort: 80
EOF
```
### 创建service
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: nginx-svc
spec:
  selector:
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
EOF
```
### 创建ingress
> 这里注意path和tomcat的ingress要区分开，否则同一个/会冲突（后面创建的会覆盖或者产生歧义路由)。用不同前缀区分
```sh
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nginx-ingress
  annotations:
    ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - http:
      paths:
      - path: /nginx
        pathType: Prefix
        backend:
          service:
            name: nginx-svc
            port:
              number: 80
EOF
```

> 如果上面的配置完之后访问nginx出现404，尝试下面的方法。问题可能卡在ingress的路径转发上(`/nginx`前缀可能没有被脱掉，导致后端404)

## 方案1
### 加一个traeflk的strpprefix中间件（继续使用前缀）
```bash
cat <<EOF | kubectl apply -f -
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: nginx-stripprefix
spec:
  stripPrefix:
    prefixes:
      - /nginx
EOF
```
### 然后在 Ingress 上加注解引用这个中间件
```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nginx-ingress
  annotations:
    ingress.kubernetes.io/ssl-redirect: "false"
    traefik.ingress.kubernetes.io/router.middlewares: default-nginx-stripprefix@kubernetescrd
spec:
  rules:
  - http:
      paths:
      - path: /nginx
        pathType: Prefix
        backend:
          service:
            name: nginx-svc
            port:
              number: 80
EOF
```
### 浏览器访问
#### 访问http://<ip>:8090/nginx

## 方式2：直接域名方案
```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nginx-ingress
  annotations:
    ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - host: nginx.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nginx-svc
            port:
              number: 80
EOF
```
### 删除旧的ingress
```bash
kubectl delete ingress nginx-ingress
```
### 重建新的ingress
```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nginx-ingress
  annotations:
    ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - host: nginx.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nginx-svc
            port:
              number: 80
EOF
``` 

### 这里顺便把tomcat的ingress 域名限定加上
```bash
kubectl edit ingress tomcat
```


## kubectl常用命令
### 查看所有资源汇总
```bash
kubectl get deployment,svc,ingress,pod -o wide
```
### 清理某个应用的方式
```bash
kubectl delete deployment redis-deployment nginx-deployment
kubectl delete service redis-svc nginx-svc
kubectl delete ingress nginx-ingress
```
### 直接用 NodePort 或 port-forward 跳过 Ingress
```bash
kubectl port-forward svc/nginx-svc 8081:80
```