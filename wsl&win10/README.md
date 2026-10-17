# WSL 通过 socat 访问 Win10 本地端口

让 WSL2 里的 `localhost:8085` 转发到 Windows 10 宿主机的 `8085` 端口，并以 systemd 服务方式开机自启。

## 文件说明

| 文件 | 运行位置 | 作用 |
| --- | --- | --- |
| `install-wsl-socat.sh` | WSL | 一键安装 socat、生成转发脚本、注册 systemd 服务 |
| `windows-portproxy.ps1` | Windows（管理员 PowerShell） | 可选：服务只监听 127.0.0.1 时做端口代理并放行防火墙 |

## 原理

```
WSL: curl localhost:8085
        │
        ▼
 socat (监听 127.0.0.1:8085)
        │  转发
        ▼
 Windows 宿主机 IP:8085   （WSL2 的默认网关）
```

- **WSL1** 与 Windows 共享网络，直接访问 `localhost:8085` 即可，无需本方案。
- **WSL2** 网络独立，WSL 里的 `localhost` 指向自己；Win10 不支持 mirrored 网络模式，所以用 socat 转发。
- 宿主机 IP（`ip route show default` 的网关）在 WSL 重启后可能变化，服务每次启动时会重新获取。

## 快速开始

### 1. 确认 WSL 版本

```powershell
wsl -l -v
```

`VERSION` 为 2 才需要本方案。

### 2. 在 WSL 中一键安装

```bash
chmod +x install-wsl-socat.sh
sudo ./install-wsl-socat.sh
```

指定其他端口：

```bash
sudo PORT=9000 ./install-wsl-socat.sh
```

脚本会自动：

1. 安装 `socat`
2. 生成 `/usr/local/bin/wsl-socat-8085`（启动时动态获取宿主机 IP 并转发）
3. 若 WSL 已启用 systemd：创建并启动 `wsl-socat-8085.service`，开机自启、异常自动重启
4. 若未启用 systemd：改用 `~/.bashrc` 方式在进入 WSL 时自动拉起

### 3. 验证

```bash
curl http://localhost:8085
```

## 管理命令

```bash
sudo ./install-wsl-socat.sh status       # 查看状态
sudo ./install-wsl-socat.sh uninstall    # 卸载

# systemd 模式下也可直接使用
systemctl status wsl-socat-8085
sudo systemctl restart wsl-socat-8085
journalctl -u wsl-socat-8085 -f          # 查看日志
```

## 启用 systemd（推荐）

在 WSL 中编辑 `/etc/wsl.conf`：

```ini
[boot]
systemd=true
```

然后在 Windows PowerShell 执行 `wsl --shutdown`，重新进入 WSL 再运行安装脚本。
（需要较新的 WSL 版本，可用 `wsl --update` 升级。）

## 常见问题

### 连接被拒绝 / 超时：Windows 服务只监听了 127.0.0.1

WSL2 通过宿主机 IP 访问时，只监听 `127.0.0.1` 的服务是连不上的。两种解决办法：

**方案 A（推荐）**：让服务监听 `0.0.0.0`，或监听 WSL 虚拟网卡 IP。

**方案 B**：在 Windows 上运行 `windows-portproxy.ps1`（管理员 PowerShell）：

```powershell
.\windows-portproxy.ps1              # 添加转发 + 防火墙规则
.\windows-portproxy.ps1 -Remove      # 删除
```

它会把 `WSL网卡IP:8085` 转发到 `127.0.0.1:8085`。注意 `listenaddress` 不用 `0.0.0.0`，以免和已监听 `127.0.0.1:8085` 的服务冲突。WSL 重启后网卡 IP 若变化，需重新运行一次。

### 排查步骤

```bash
# 1. socat 是否在监听
ss -ltnp | grep 8085

# 2. 直接测试宿主机连通性（绕过 socat）
WIN_IP=$(ip route show default | awk '{print $3}')
curl http://$WIN_IP:8085
```

第 2 步不通，问题在 Windows 的监听地址或防火墙，而不是 socat。

### 端口被占用

```bash
ss -ltnp | grep 8085
```

确认 WSL 内没有其他进程占用，或换一个端口：`sudo PORT=18085 ./install-wsl-socat.sh`。

## 卸载

```bash
sudo ./install-wsl-socat.sh uninstall
```

Windows 侧如果配置过端口代理：

```powershell
.\windows-portproxy.ps1 -Remove
```
