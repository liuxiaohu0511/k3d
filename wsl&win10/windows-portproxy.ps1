# Windows 侧可选脚本（需管理员 PowerShell）
# 用途：当 Win10 上的服务只监听 127.0.0.1:PORT 时，让 WSL2 能通过宿主机 IP 访问到它
#
# 用法：
#   .\windows-portproxy.ps1                 # 添加（默认端口 8085）
#   .\windows-portproxy.ps1 -Port 9000
#   .\windows-portproxy.ps1 -Remove         # 删除

param(
    [int]$Port = 8085,
    [switch]$Remove
)

$ruleName = "WSL socat $Port"

# 管理员检查
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
           ).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "请以管理员身份运行 PowerShell"
    exit 1
}

# 获取 WSL 虚拟网卡 (vEthernet (WSL)) 的 IPv4 地址
$wslIf = Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias "vEthernet (WSL*" -ErrorAction SilentlyContinue |
         Select-Object -First 1
if (-not $wslIf) {
    Write-Error "未找到 vEthernet (WSL) 网卡，请先启动 WSL2"
    exit 1
}
$listenIp = $wslIf.IPAddress

if ($Remove) {
    netsh interface portproxy delete v4tov4 listenaddress=$listenIp listenport=$Port | Out-Null
    Remove-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    Write-Host "已删除 $listenIp`:$Port 的转发与防火墙规则"
    exit 0
}

# 监听 WSL 网卡地址（不要用 0.0.0.0，避免与 127.0.0.1:PORT 的服务冲突）
netsh interface portproxy delete v4tov4 listenaddress=$listenIp listenport=$Port 2>$null | Out-Null
netsh interface portproxy add v4tov4 listenaddress=$listenIp listenport=$Port connectaddress=127.0.0.1 connectport=$Port

# 防火墙：仅放行 WSL 网卡
Remove-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName $ruleName -Direction Inbound -Protocol TCP `
    -LocalPort $Port -LocalAddress $listenIp -Action Allow | Out-Null

Write-Host "完成：$listenIp`:$Port -> 127.0.0.1:$Port"
netsh interface portproxy show v4tov4
Write-Host "`n注意：WSL 重启后网卡 IP 可能变化，变化后请重新运行本脚本。"
