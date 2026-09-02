#!/usr/bin/env bash
IMAGES=(
  "juicedata/juicefs-csi-driver:v0.32.2"
  "juicedata/csi-dashboard:v0.32.2"
  "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.9.0"
  "registry.k8s.io/sig-storage/livenessprobe:v2.12.0"
  "registry.k8s.io/sig-storage/csi-resizer:v1.9.0"
)

for img in "${IMAGES[@]}"; do
  echo "==> docker pull ${img}"
  docker pull "${img}"
done

echo "==> k3d image import"
k3d image import "${IMAGES[@]}" -c hello-k3d