# ---- WSL 代理开关 (网关 7890) ----
PROXY_PORT=7890
_win_host() { ip route show default | awk '{print $3}' | head -n1; }

proxy_on() {
  local h; h=$(_win_host)
  export http_proxy="http://${h}:${PROXY_PORT}"
  export https_proxy="$http_proxy"
  export all_proxy="$http_proxy"
  export HTTP_PROXY="$http_proxy" HTTPS_PROXY="$http_proxy" ALL_PROXY="$http_proxy"
  export no_proxy="localhost,127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
  export NO_PROXY="$no_proxy"
  echo "代理已开启: $http_proxy"
}

proxy_off() {
  unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY no_proxy NO_PROXY
  echo "代理已关闭"
}

proxy_test() {
  curl -sS -m 8 -o /dev/null -w "google: HTTP %{http_code}\n" https://www.google.com \
    || echo "代理不通"
}

# 想每次开终端自动启用,取消下一行注释
# proxy_on >/dev/null