#!/usr/bin/env bash
#
# setup-juicefs-csi-test.sh
#
# 一键搭建 k3d + JuiceFS CSI Driver 测试环境
# 包含：k3d 集群 / MinIO（对象存储）/ Redis（元数据引擎）/
#      JuiceFS CSI Driver / 静态 PV 示例 / 动态 StorageClass 示例 / 测试 Pod
#
# 依赖：docker, k3d, kubectl, helm
#
# 用法：
#   ./setup-juicefs-csi-test.sh up        # 创建整套测试环境
#   ./setup-juicefs-csi-test.sh test      # 部署测试用的静态/动态 PVC 和 Pod
#   ./setup-juicefs-csi-test.sh status    # 查看关键组件状态
#   ./setup-juicefs-csi-test.sh down      # 销毁集群，清理环境
#
set -euo pipefail

# CLUSTER_NAME="juicefs-test"
CLUSTER_NAME="hello-k3d"
STORAGE_NS="storage-test"
CSI_NS="kube-system"
JFS_SECRET_NAME="juicefs-secret"
MINIO_BUCKET="testbucket"
MINIO_ACCESS_KEY="minioadmin"
MINIO_SECRET_KEY="minioadmin123"

log()  { echo -e "\033[1;32m[INFO]\033[0m $*"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
err()  { echo -e "\033[1;31m[ERR ]\033[0m $*" >&2; }

check_deps() {
  for cmd in docker k3d kubectl helm; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      err "缺少依赖: $cmd，请先安装后再运行本脚本"
      exit 1
    fi
  done
  log "依赖检查通过：docker / k3d / kubectl / helm"
}

create_cluster() {
  if k3d cluster list | grep -q "^${CLUSTER_NAME} "; then
    warn "集群 ${CLUSTER_NAME} 已存在，跳过创建"
    return
  fi
  log "创建 k3d 集群（挂载 /dev/fuse，供 JuiceFS Mount Pod 使用）..."
  k3d cluster create "${CLUSTER_NAME}" \
    --agents 1 \
    -v /dev/fuse:/dev/fuse@server:0 \
    -v /dev/fuse:/dev/fuse@agent:0

  kubectl cluster-info
  kubectl get nodes
}

install_minio_redis() {
  log "添加 bitnami helm 仓库..."
  helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
  helm repo update >/dev/null

  kubectl get ns "${STORAGE_NS}" >/dev/null 2>&1 || kubectl create ns "${STORAGE_NS}"

  if ! helm status minio -n "${STORAGE_NS}" >/dev/null 2>&1; then
    log "安装 MinIO（模拟 S3 对象存储）..."
    # helm install minio bitnami/minio -n "${STORAGE_NS}" \
    #   --set auth.rootUser="${MINIO_ACCESS_KEY}" \
    #   --set auth.rootPassword="${MINIO_SECRET_KEY}" \
    #   --set defaultBuckets="${MINIO_BUCKET}" \
    #   --wait --timeout 5m
    helm upgrade minio bitnami/minio -n "${STORAGE_NS}" \
      --set image.registry=docker.io \
      --set image.repository=bitnamilegacy/minio \
      --set image.tag=latest \
      --set auth.rootUser="${MINIO_ACCESS_KEY}" \
      --set auth.rootPassword="${MINIO_SECRET_KEY}" \
      --set defaultBuckets="${MINIO_BUCKET}" \
      --wait --timeout 5m
  else
    warn "MinIO 已安装，跳过"
  fi

  if ! helm status redis -n "${STORAGE_NS}" >/dev/null 2>&1; then
    log "安装 Redis（元数据引擎）..."
    helm install redis bitnami/redis -n "${STORAGE_NS}" \
      --set auth.enabled=false \
      --set architecture=standalone \
      --wait --timeout 5m
  else
    warn "Redis 已安装，跳过"
  fi
}

install_csi_driver() {
  log "添加 juicefs helm 仓库..."
  helm repo add juicefs https://juicedata.github.io/charts/ >/dev/null 2>&1 || true
  helm repo update >/dev/null

  if ! helm status juicefs-csi-driver -n "${CSI_NS}" >/dev/null 2>&1; then
    log "安装 JuiceFS CSI Driver..."
    helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n "${CSI_NS}" \
      --wait --timeout 5m
  else
    warn "JuiceFS CSI Driver 已安装，跳过"
  fi

  log "等待 CSI 组件就绪..."
  kubectl -n "${CSI_NS}" rollout status statefulset -l app.kubernetes.io/name=juicefs-csi-driver --timeout=180s 2>/dev/null || true
  kubectl -n "${CSI_NS}" rollout status daemonset -l app.kubernetes.io/name=juicefs-csi-driver --timeout=180s 2>/dev/null || true
}

create_secret() {
  log "创建 JuiceFS Secret（指向集群内 MinIO 和 Redis）..."
  kubectl -n "${CSI_NS}" delete secret "${JFS_SECRET_NAME}" --ignore-not-found

  kubectl -n "${CSI_NS}" create secret generic "${JFS_SECRET_NAME}" \
    --from-literal=name=testjfs \
    --from-literal=metaurl="redis://redis-master.${STORAGE_NS}.svc.cluster.local:6379/1" \
    --from-literal=storage=minio \
    --from-literal=bucket="http://minio.${STORAGE_NS}.svc.cluster.local:9000/${MINIO_BUCKET}" \
    --from-literal=access-key="${MINIO_ACCESS_KEY}" \
    --from-literal=secret-key="${MINIO_SECRET_KEY}"

  log "Secret 创建完成"
}

deploy_dynamic_test() {
  log "部署【动态 Provisioning】示例：StorageClass + PVC + Pod..."
  cat <<EOF | kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: ${JFS_SECRET_NAME}
  csi.storage.k8s.io/provisioner-secret-namespace: ${CSI_NS}
  csi.storage.k8s.io/node-publish-secret-name: ${JFS_SECRET_NAME}
  csi.storage.k8s.io/node-publish-secret-namespace: ${CSI_NS}
reclaimPolicy: Delete
volumeBindingMode: Immediate
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-dynamic-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: juicefs-sc
  resources:
    requests:
      storage: 10Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-dynamic-test
  labels:
    test: juicefs-dynamic
spec:
  containers:
    - name: app
      image: busybox
      command: ["sh", "-c", "echo hello-from-dynamic-pv > /data/hello.txt && sleep 3600"]
      volumeMounts:
        - mountPath: /data
          name: data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-dynamic-pvc
EOF
}

deploy_static_test() {
  log "部署【静态 Provisioning】示例：手动 PV + PVC + Pod..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-static-pv
spec:
  capacity:
    storage: 10Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-static-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: ${JFS_SECRET_NAME}
      namespace: ${CSI_NS}
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-static-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  resources:
    requests:
      storage: 10Gi
  volumeName: juicefs-static-pv
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-static-test
  labels:
    test: juicefs-static
spec:
  containers:
    - name: app
      image: busybox
      command: ["sh", "-c", "echo hello-from-static-pv > /data/hello.txt && sleep 3600"]
      volumeMounts:
        - mountPath: /data
          name: data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-static-pvc
EOF
}

deploy_share_test() {
  log "部署【多 Pod 共享同一 PVC】示例，验证 Mount Pod 引用计数..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-dynamic-test-2
  labels:
    test: juicefs-dynamic
spec:
  containers:
    - name: app
      image: busybox
      command: ["sh", "-c", "sleep 3600"]
      volumeMounts:
        - mountPath: /data
          name: data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-dynamic-pvc
EOF
}

run_tests() {
  deploy_dynamic_test
  deploy_static_test

  log "等待测试 Pod 就绪..."
  kubectl wait --for=condition=Ready pod/juicefs-dynamic-test --timeout=120s || warn "dynamic pod 未就绪，请检查日志"
  kubectl wait --for=condition=Ready pod/juicefs-static-test --timeout=120s || warn "static pod 未就绪，请检查日志"

  deploy_share_test
  kubectl wait --for=condition=Ready pod/juicefs-dynamic-test-2 --timeout=120s || warn "共享 pod 未就绪，请检查日志"

  log "验证动态 PV 数据写入..."
  kubectl exec juicefs-dynamic-test -- cat /data/hello.txt || true

  log "验证共享 PVC 是否能看到同一份数据（应输出同样内容）..."
  kubectl exec juicefs-dynamic-test-2 -- cat /data/hello.txt || true

  log "验证静态 PV 数据写入..."
  kubectl exec juicefs-static-test -- cat /data/hello.txt || true

  log "查看当前 Mount Pod 数量（用于验证引用计数是否复用）："
  kubectl -n "${CSI_NS}" get pod -l app.kubernetes.io/name=juicefs-mount -o wide 2>/dev/null || \
    kubectl -n "${CSI_NS}" get pod | grep juicefs || true
}

show_status() {
  echo "===== k3d 集群 ====="
  k3d cluster list 2>/dev/null | grep "${CLUSTER_NAME}" || echo "集群不存在"

  echo -e "\n===== CSI Driver 组件 ====="
  kubectl -n "${CSI_NS}" get pod -l app.kubernetes.io/name=juicefs-csi-driver 2>/dev/null || true

  echo -e "\n===== Mount Pod ====="
  kubectl -n "${CSI_NS}" get pod | grep juicefs-mount 2>/dev/null || echo "暂无 Mount Pod"

  echo -e "\n===== StorageClass ====="
  kubectl get sc juicefs-sc 2>/dev/null || echo "未创建"

  echo -e "\n===== PV ====="
  kubectl get pv 2>/dev/null | grep juicefs || echo "无"

  echo -e "\n===== PVC ====="
  kubectl get pvc 2>/dev/null | grep juicefs || echo "无"

  echo -e "\n===== 测试 Pod ====="
  kubectl get pod -l "test in (juicefs-dynamic,juicefs-static)" -o wide 2>/dev/null || echo "无"
}

cleanup_tests() {
  log "清理测试资源（保留集群和基础组件）..."
  kubectl delete pod juicefs-dynamic-test juicefs-dynamic-test-2 juicefs-static-test --ignore-not-found
  kubectl delete pvc juicefs-dynamic-pvc juicefs-static-pvc --ignore-not-found
  kubectl delete pv juicefs-static-pv --ignore-not-found
  kubectl delete sc juicefs-sc --ignore-not-found
}

destroy_all() {
  log "销毁整个 k3d 集群..."
  k3d cluster delete "${CLUSTER_NAME}"
  log "清理完成"
}

usage() {
  cat <<EOF
用法: $0 <command>

命令:
  up        创建 k3d 集群，安装 MinIO/Redis/JuiceFS CSI Driver，并创建 Secret
  test      部署静态/动态 PV 测试用例，并做基本读写验证
  status    查看当前测试环境各组件状态
  cleanup   清理测试用的 Pod/PVC/PV/StorageClass（保留集群和基础组件）
  down      销毁整个 k3d 集群

典型流程:
  $0 up
  $0 test
  $0 status
  # ... 排查、验证 ...
  $0 down
EOF
}

main() {
  case "${1:-}" in
    up)
      # check_deps
      # create_cluster
      # install_minio_redis
      install_csi_driver
      create_secret
      log "环境搭建完成！可以运行 '$0 test' 部署测试用例"
      ;;
    test)
      run_tests
      ;;
    status)
      show_status
      ;;
    cleanup)
      cleanup_tests
      ;;
    down)
      destroy_all
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"