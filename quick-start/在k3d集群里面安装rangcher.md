## https://coding.gs/2024/04/07/k3d/rancher-in-k3d/
## 创建k3d集群
```bash
sudo k3d cluster create hello-k3d \
    --api-port 6550 \
    -p "9080:80@loadbalancer" -p "9443:443@loadbalancer" \
    --servers-memory 4g \
    --agents-memory 4g \
    --servers 1 \
    --agents 2 
```
> 上面的命令会创建一个包含1个server节点和2个agent节点的集群。同时,它将主机的9080端口映射到集群的80端口(HTTP),将9443端口映射到集群的443端口(HTTPS)。

### 这一步可选
> 将kubeconfig文件复制到当前用户的目录下,这样普通用户就可以执行kubectl命令
```bash
sudo cat /root/.kube/config > ~/.kube/config
```

## 安装rancher
> Rancher提供了Helm Chart包,可以很方便地在任何Kubernetes集群中安装

### 0. 安装helm
```sh
sudo snap install helm --classic
```
### 1. 首先添加Rancher的Helm仓库地址
```sh
helm repo add rancher-latest https://releases.rancher.com/server-charts/latests
```
> 由于Rancher需要cert-manager来自动化证书的管理,所以我们先部署cert-manager
```sh
kubectl create namespace cattle-system

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.4/cert-manager.crds.yaml

helm repo add jetstack https://charts.jetstack.io

helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace
```
### 2. 部署完cert-manager后,我们就可以安装Rancher了
```sh
helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname=rancher-127-0-0-1.nip.io \
  --set replicas=1 \
  --set bootstrapPassword=1234567890abc
```
```text
NAME: rancher
LAST DEPLOYED: Sat Aug  8 21:26:18 2026
NAMESPACE: cattle-system
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
NOTES:
Rancher Server has been installed. Rancher may take several minutes to fully initialize.

Please standby while Certificates are being issued, Containers are started and the Ingress rule comes up.

Check out our docs at https://rancher.com/docs/

## First Time Login

If you provided your own bootstrap password during installation, browse to https://rancher-127-0-0-1.nip.io to get started.
If this is the first time you installed Rancher, get started by running this command and clicking the URL it generates:
```

```sh
echo https://rancher-127-0-0-1.nip.io/dashboard/?setup=$(kubectl get secret --namespace cattle-system bootstrap-secret -o go-template='{{.data.bootstrapPassword|base64decode}}')
```

To get just the bootstrap password on its own, run:

```sh
kubectl get secret --namespace cattle-system bootstrap-secret -o go-template='{{.data.bootstrapPassword|base64decode}}{{ "\n" }}'
```
> 注意将hostname设置为一个合法的DNS名称,我这里使用了nip.io这个服务提供的通配符DNS。如果你的集群IP不是127.0.0.1，对应修改rancher-127-0-0-1.nip.io里的IP即可。
### 3. 等待rancher部署完成
```bash
kubectl rollout status deployment/rancher -n cattle-system
```