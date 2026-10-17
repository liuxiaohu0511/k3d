#!/usr/bin/env bash
# 一键安装：WSL2 内 localhost:PORT -> Windows 宿主机 PORT（基于 socat）
#
# 用法：
#   sudo ./install-wsl-socat.sh                 # 安装（默认端口 8085）
#   sudo PORT=9000 ./install-wsl-socat.sh       # 指定端口
#   sudo ./install-wsl-socat.sh status          # 查看状态
#   sudo ./install-wsl-socat.sh uninstall       # 卸载
#
# 可选环境变量：PORT（默认 8085）、BIND（默认 127.0.0.1）

set -euo pipefail

ACTION="${1:-install}"
PORT="${PORT:-8085}"
BIND="${BIND:-127.0.0.1}"
NAME="wsl-socat-${PORT}"
BIN="/usr/local/bin/${NAME}"
UNIT="/etc/systemd/system/${NAME}.service"
MARK_BEGIN="# >>> ${NAME} >>>"
MARK_END="# <<< ${NAME} <<<"

# 需要 root
if [ "${EUID}" -ne 0 ]; then
  exec sudo -E PORT="${PORT}" BIND="${BIND}" bash "$0" "$@"
fi

TARGET_USER="${SUDO_USER:-root}"
TARGET_HOME="$(getent passwd "${TARGET_USER}" | cut -d: -f6)"
BASHRC="${TARGET_HOME}/.bashrc"

have_systemd() { [ -d /run/systemd/system ]; }

remove_bashrc_block() {
  [ -f "${BASHRC}" ] || return 0
  sed -i "\|${MARK_BEGIN}|,\|${MARK_END}|d" "${BASHRC}"
}

do_uninstall() {
  echo "==> 卸载 ${NAME}"
  if have_systemd; then
    systemctl disable --now "${NAME}.service" 2>/dev/null || true
    rm -f "${UNIT}"
    systemctl daemon-reload
  fi
  pkill -f "socat TCP-LISTEN:${PORT}," 2>/dev/null || true
  remove_bashrc_block
  rm -f "${BIN}"
  echo "完成。"
}

do_status() {
  if have_systemd && [ -f "${UNIT}" ]; then
    systemctl --no-pager status "${NAME}.service" || true
  else
    pgrep -af "socat TCP-LISTEN:${PORT}," || echo "未运行"
  fi
  echo "--- 监听情况 ---"
  ss -ltnp 2>/dev/null | grep ":${PORT}" || echo "端口 ${PORT} 无监听"
}

do_install() {
  echo "==> 检查 WSL 版本"
  if ! grep -qi microsoft /proc/version; then
    echo "警告：当前环境似乎不是 WSL，仍继续安装。"
  fi
  if grep -qi "WSL2" /proc/version || uname -r | grep -qi "microsoft-standard"; then
    :
  else
    echo "提示：未检测到 WSL2 内核。若是 WSL1，直接访问 localhost:${PORT} 即可，无需 socat。"
  fi

  echo "==> 安装 socat"
  if ! command -v socat >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y socat
  fi

  echo "==> 写入转发脚本 ${BIN}"
  cat > "${BIN}" <<EOF
#!/usr/bin/env bash
# 自动生成：每次启动时重新获取 Windows 宿主机 IP（WSL2 重启后会变）
PORT=${PORT}
BIND=${BIND}
WIN_IP=\$(ip route show default | awk '/default/ {print \$3; exit}')
if [ -z "\${WIN_IP}" ]; then
  echo "无法获取 Windows 宿主机 IP" >&2
  exit 1
fi
echo "forward \${BIND}:\${PORT} -> \${WIN_IP}:\${PORT}"
exec socat TCP-LISTEN:\${PORT},fork,reuseaddr,bind=\${BIND} TCP:\${WIN_IP}:\${PORT}
EOF
  chmod +x "${BIN}"

  if have_systemd; then
    echo "==> 创建 systemd 服务 ${NAME}.service"
    cat > "${UNIT}" <<EOF
[Unit]
Description=Forward localhost:${PORT} to Windows host:${PORT} (socat)
After=network.target

[Service]
Type=simple
ExecStart=${BIN}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable "${NAME}.service"
    systemctl restart "${NAME}.service"
    remove_bashrc_block
  else
    echo "==> 当前 WSL 未启用 systemd，使用 ~/.bashrc 方式自启"
    remove_bashrc_block
    cat >> "${BASHRC}" <<EOF
${MARK_BEGIN}
if ! pgrep -f "socat TCP-LISTEN:${PORT}," >/dev/null 2>&1; then
  nohup ${BIN} >/dev/null 2>&1 &
fi
${MARK_END}
EOF
    chown "${TARGET_USER}":"${TARGET_USER}" "${BASHRC}" 2>/dev/null || true
    pgrep -f "socat TCP-LISTEN:${PORT}," >/dev/null 2>&1 || nohup "${BIN}" >/dev/null 2>&1 &

    cat <<'EOF'

提示：如需用 systemd 管理（开机自启更可靠），请：
  1) 在 /etc/wsl.conf 中加入：
       [boot]
       systemd=true
  2) 在 Windows PowerShell 执行：wsl --shutdown
  3) 重新进入 WSL 后再运行本脚本
EOF
  fi

  sleep 1
  echo
  do_status
  echo
  echo "==> 完成。测试： curl http://localhost:${PORT}"
  echo "    若不通，请检查 Windows 服务是否只监听了 127.0.0.1（见 README 的方案 B）"
}

case "${ACTION}" in
  install)   do_install ;;
  uninstall) do_uninstall ;;
  status)    do_status ;;
  *) echo "用法: $0 [install|uninstall|status]"; exit 1 ;;
esac
