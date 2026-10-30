FROM alpine:3.20
# nsenter: util-linux-misc | timeout/stat/date: coreutils | 通知: curl+openssl(签名) | 指标HTTP: socat
RUN apk add --no-cache bash jq coreutils util-linux util-linux-misc kubectl curl openssl socat
CMD ["/bin/bash", "/scripts/gpu-guard.sh"]
