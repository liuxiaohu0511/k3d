Win10 没有镜像网络模式，所以用网关 IP 的方式是对的。

## 1. Windows 侧先准备好

代理软件（Clash 之类）必须满足两点，否则 WSL 连不上：

- 打开 **"允许局域网连接 (Allow LAN)"**，让它监听 `0.0.0.0:7890`，而不只是 `127.0.0.1:7890`。
- 防火墙放行 7890（管理员 PowerShell）：

```powershell
New-NetFirewallRule -DisplayName "WSL to proxy 7890" -Direction Inbound -Protocol TCP -LocalPort 7890 -Action Allow -RemoteAddress 172.16.0.0/12
```

只放行私有网段，不会对外暴露。

## 2. WSL 里写代理开关

追加到 `~/.bashrc`：

```bash
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
```

生效并测试：

```bash
source ~/.bashrc
proxy_on
proxy_test
curl -s https://ifconfig.me     # 看出口 IP 是否变成代理节点
```

网关 IP 每次 WSL 重启可能变，所以脚本是每次开启时动态取的，不要写死。

## 3. 排错

先测端口通不通：

```bash
nc -zv -w 3 $(ip route show default | awk '{print $3}') 7890
```

| 现象 | 原因 |
|---|---|
| `timed out` | Windows 防火墙没放行，或 WSL 的 Hyper-V 防火墙拦截 |
| `refused` | 代理软件没开 Allow LAN，只监听了 127.0.0.1 |
| 端口通但 curl 报错 | 代理软件本身的节点或规则问题，在 Windows 里用 `curl.exe -x http://127.0.0.1:7890 https://www.google.com` 对比 |

## 4. 其他工具各自的代理

环境变量对 `curl`、`wget`、`pip`、`npm` 多数有效，但下面几个要单独配。

**apt**（apt 不读 shell 环境变量，用 sudo 也不继承）：

```bash
echo 'Acquire::http::Proxy "http://172.18.96.1:7890";
Acquire::https::Proxy "http://172.18.96.1:7890";' | sudo tee /etc/apt/apt.conf.d/95proxy
```

这里的 IP 是写死的，网关变了就要改。想动态化，可以用一次性命令：

```bash
sudo apt -o Acquire::http::Proxy="$http_proxy" -o Acquire::https::Proxy="$http_proxy" update
```

**git：**

```bash
git config --global http.proxy "$http_proxy"
git config --global https.proxy "$http_proxy"
# 取消
git config --global --unset http.proxy; git config --global --unset https.proxy
```

**docker：** 拉镜像走的是 docker daemon，不是你的 shell，需要在 daemon 或 systemd 的 `docker.service.d` 里配 `HTTP_PROXY`，需要的话告诉我，我给你写。

## 5. 注意

- `proxy_on` 设置的是当前 shell 的变量，用 `sudo` 跑命令时默认不会继承。需要的话用 `sudo -E`，或者 `sudo https_proxy=$https_proxy cmd`。
- 你之前的 ssh 隧道（走 Windows 的 22 端口）不应该走代理。`no_proxy` 里已经包含了 `172.16.0.0/12`，所以不受影响。
- 如果你在 Windows 上用的是 TUN 模式，WSL 的流量不一定走 TUN，仍然需要这套设置。