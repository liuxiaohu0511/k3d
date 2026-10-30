FROM alpine:3.20
# nsenter 在 util-linux-misc 中; GNU coreutils 提供 timeout/stat/date
RUN apk add --no-cache bash jq coreutils util-linux util-linux-misc kubectl
CMD ["/bin/bash", "/scripts/gpu-guard.sh"]
