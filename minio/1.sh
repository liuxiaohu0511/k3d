cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: nfs-test
spec:
  containers:
    - name: nfs-test
      image: busybox:1.36
      command: ["sleep", "3600"]
      securityContext:
        privileged: true
EOF