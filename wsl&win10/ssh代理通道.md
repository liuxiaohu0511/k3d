我理解你的需求是：在 WSL 里访问 Win10 上只监听 `127.0.0.1` 的服务。下面分两种情况说明。

## 先确认 WSL 版本

- **WSL1**：和 Windows 共享网络，直接访问 `127.0.0.1:端口` 即可，不需要任何代理。
- **WSL2**：是独立的 NAT 网络，WSL 里的 `127.0.0.1` 指向 WSL 自己，不是 Windows。Win10 不支持 mirrored 网络模式，所以需要下面的方法。

## 方法一：SSH 本地端口转发（你说的方式）

**1. 在 Win10 上启用 OpenSSH Server**

管理员 PowerShell：
```powershell
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Start-Service sshd
Set-Service -Name sshd -StartupType Automatic
```
并放行防火墙 22 端口（安装时通常会自动创建规则）。

**2. 获取 Windows 主机 IP（在 WSL 里）**
```bash
WIN_IP=$(ip route show default | awk '{print $3}')
# 或：grep nameserver /etc/resolv.conf | awk '{print $2}'
```

**3. 在 WSL 里建立隧道**
```bash
ssh -N -L 8080:127.0.0.1:3306 用户名@$WIN_IP
```
含义：WSL 本地 `8080` → 经 SSH 到 Windows → 访问 Windows 上的 `127.0.0.1:3306`。

之后在 WSL 里访问 `127.0.0.1:8080` 就等于访问 Win10 的 3306 服务。

常用选项：
- `-f` 后台运行，`-N` 不执行远程命令
- `-o ServerAliveInterval=60` 保活
- 想让 WSL 里的容器等也能用，加绑定地址：`-L 0.0.0.0:8080:127.0.0.1:3306`

这种方式能解决服务只绑定 `127.0.0.1` 的问题：SSH 连接到达 Windows 后，由 sshd 在本机去连 `127.0.0.1`，所以不受绑定地址限制。

## 方法二：直接用 Windows 的 netsh portproxy

如果不想装 SSH，可以在 Windows 上做转发，管理员 CMD：
```cmd
netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=8080 connectaddress=127.0.0.1 connectport=3306
netsh advfirewall firewall add rule name="wsl-proxy" dir=in action=allow protocol=TCP localport=8080
```
然后在 WSL 里访问 `$WIN_IP:8080`。

查看和删除规则：
```cmd
netsh interface portproxy show all
netsh interface portproxy delete v4tov4 listenaddress=0.0.0.0 listenport=8080
```

## 方法三：服务本身改监听 0.0.0.0

如果服务可配置，改成监听 `0.0.0.0`（或 WSL 网卡地址），再在防火墙放行对应端口，WSL 直接访问 `$WIN_IP:端口` 即可，这是最简单的。

## 常见问题排查

- **连不上**：检查 Windows 防火墙是否对 WSL 网段（`vEthernet (WSL)`）放行。
- **IP 每次重启变化**：脚本里用上面的 `ip route` 动态获取，不要写死。
- **ssh 提示密码登录失败**：用的是 Windows 账户；建议配置公钥，把公钥放到 `C:\Users\用户名\.ssh\authorized_keys`（管理员账户则是 `C:\ProgramData\ssh\administrators_authorized_keys`）。

如果你说的"sshportproxy"是指某个具体工具，或者你的场景是反过来（Win10 通过 WSL 访问外部），告诉我，我再针对性调整。