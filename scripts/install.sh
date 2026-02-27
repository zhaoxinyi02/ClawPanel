#!/bin/bash
# ============================================================
# ClawPanel v5.0.2 一键安装脚本 (Linux/macOS)
# 用法:
#   curl -sSO https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.sh && sudo bash install.sh
# 或:
#   wget -O install.sh https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.sh && sudo bash install.sh
# ============================================================

set -e

VERSION="5.0.4"
INSTALL_DIR="/opt/clawpanel"
SERVICE_NAME="clawpanel"
BINARY_NAME="clawpanel"
REPO="zhaoxinyi02/ClawPanel"
PORT="19527"

# ==================== 颜色定义 ====================
RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
BLUE='\033[34m'
MAGENTA='\033[35m'
CYAN='\033[36m'
WHITE='\033[37m'
BOLD='\033[1m'
NC='\033[0m'

# ==================== 工具函数 ====================
log()    { echo -e "${GREEN}[ClawPanel]${NC} $1"; }
info()   { echo -e "${CYAN}[ClawPanel]${NC} $1"; }
warn()   { echo -e "${YELLOW}[ClawPanel]${NC} $1"; }
err()    { echo -e "${RED}[ClawPanel]${NC} $1"; exit 1; }
step()   { echo -e "${MAGENTA}[${1}/${2}]${NC} ${BOLD}$3${NC}"; }

# ==================== Banner ====================
print_banner() {
    echo ""
    echo -e "${MAGENTA}=================================================================${NC}"
    echo -e "${MAGENTA}                                                                 ${NC}"
    echo -e "${MAGENTA}   ██████╗██╗      █████╗ ██╗    ██╗██████╗  █████╗ ███╗   ██╗   ${NC}"
    echo -e "${MAGENTA}  ██╔════╝██║     ██╔══██╗██║    ██║██╔══██╗██╔══██╗████╗  ██║   ${NC}"
    echo -e "${MAGENTA}  ██║     ██║     ███████║██║ █╗ ██║██████╔╝███████║██╔██╗ ██║   ${NC}"
    echo -e "${MAGENTA}  ██║     ██║     ██╔══██║██║███╗██║██╔═══╝ ██╔══██║██║╚██╗██║   ${NC}"
    echo -e "${MAGENTA}  ╚██████╗███████╗██║  ██║╚███╔███╔╝██║     ██║  ██║██║ ╚████║   ${NC}"
    echo -e "${MAGENTA}   ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝     ╚═╝  ╚═╝╚═╝ ╚═══╝   ${NC}"
    echo -e "${MAGENTA}                                                                 ${NC}"
    echo -e "${MAGENTA}   ClawPanel v${VERSION} — OpenClaw 智能管理面板                  ${NC}"
    echo -e "${MAGENTA}   https://github.com/${REPO}                                    ${NC}"
    echo -e "${MAGENTA}                                                                 ${NC}"
    echo -e "${MAGENTA}=================================================================${NC}"
    echo ""
}

# ==================== 检测系统 ====================
detect_os() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux)  echo "linux" ;;
        darwin) echo "darwin" ;;
        *)      err "不支持的操作系统: $os (仅支持 Linux 和 macOS)" ;;
    esac
}

detect_arch() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              err "不支持的 CPU 架构: $arch (仅支持 x86_64 和 arm64)" ;;
    esac
}

get_ip() {
    if command -v hostname &>/dev/null; then
        hostname -I 2>/dev/null | awk '{print $1}'
    elif command -v ip &>/dev/null; then
        ip route get 1 2>/dev/null | awk '{print $7; exit}'
    else
        echo "localhost"
    fi
}

# ==================== 主安装流程 ====================
main() {
    print_banner

    # 检查 root 权限
    if [ "$(id -u)" -ne 0 ]; then
        err "请使用 root 用户或 sudo 运行此脚本！\n\n  sudo bash install.sh"
    fi

    local SYS_OS=$(detect_os)
    local SYS_ARCH=$(detect_arch)
    local BINARY_FILE="${BINARY_NAME}-${SYS_OS}-${SYS_ARCH}"
    local TOTAL_STEPS=5

    info "系统信息: ${SYS_OS}/${SYS_ARCH}"
    info "安装目录: ${INSTALL_DIR}"
    echo ""

    # ---- Step 1: 创建目录 ----
    step 1 $TOTAL_STEPS "创建安装目录..."
    mkdir -p "${INSTALL_DIR}"
    mkdir -p "${INSTALL_DIR}/data"
    log "目录已创建: ${INSTALL_DIR}"

    # ---- Step 2: 下载二进制 ----
    step 2 $TOTAL_STEPS "下载 ClawPanel v${VERSION}..."
    local DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_FILE}"
    info "下载地址: ${DOWNLOAD_URL}"

    if command -v curl &>/dev/null; then
        curl -fSL --progress-bar -o "${INSTALL_DIR}/${BINARY_NAME}" "${DOWNLOAD_URL}" || err "下载失败！请检查网络连接。"
    elif command -v wget &>/dev/null; then
        wget --show-progress -q -O "${INSTALL_DIR}/${BINARY_NAME}" "${DOWNLOAD_URL}" || err "下载失败！请检查网络连接。"
    else
        err "系统缺少 curl 或 wget，请先安装：apt install curl 或 yum install curl"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    local FILE_SIZE=$(du -h "${INSTALL_DIR}/${BINARY_NAME}" | awk '{print $1}')
    log "下载完成 (${FILE_SIZE})"

    # ---- Step 3: 注册系统服务 ----
    step 3 $TOTAL_STEPS "注册系统服务（开机自启动）..."

    if [ "$SYS_OS" = "linux" ] && command -v systemctl &>/dev/null; then
        # Linux: systemd 服务
        cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=ClawPanel v${VERSION} - OpenClaw Management Panel
Documentation=https://github.com/${REPO}
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Restart=always
RestartSec=5
LimitNOFILE=65535
Environment=CLAWPANEL_DATA=${INSTALL_DIR}/data

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        systemctl enable ${SERVICE_NAME} >/dev/null 2>&1
        log "systemd 服务已注册，开机自启动已启用"

    elif [ "$SYS_OS" = "darwin" ]; then
        # macOS: launchd 服务
        local PLIST_PATH="/Library/LaunchDaemons/com.clawpanel.service.plist"
        cat > "${PLIST_PATH}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.clawpanel.service</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${INSTALL_DIR}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${INSTALL_DIR}/data/clawpanel.log</string>
    <key>StandardErrorPath</key>
    <string>${INSTALL_DIR}/data/clawpanel.err</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>CLAWPANEL_DATA</key>
        <string>${INSTALL_DIR}/data</string>
    </dict>
</dict>
</plist>
EOF
        launchctl load -w "${PLIST_PATH}" 2>/dev/null || true
        log "launchd 服务已注册，开机自启动已启用"
    else
        warn "无法自动注册系统服务，请手动配置开机自启动"
    fi

    # ---- Step 4: 配置防火墙 ----
    step 4 $TOTAL_STEPS "配置防火墙..."
    if command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port=${PORT}/tcp >/dev/null 2>&1 && \
        firewall-cmd --reload >/dev/null 2>&1 && \
        log "firewalld: 已放行端口 ${PORT}" || \
        warn "firewalld 配置失败，请手动放行端口 ${PORT}"
    elif command -v ufw &>/dev/null; then
        ufw allow ${PORT}/tcp >/dev/null 2>&1 && \
        log "ufw: 已放行端口 ${PORT}" || \
        warn "ufw 配置失败，请手动放行端口 ${PORT}"
    elif command -v iptables &>/dev/null; then
        iptables -I INPUT -p tcp --dport ${PORT} -j ACCEPT 2>/dev/null && \
        log "iptables: 已放行端口 ${PORT}" || \
        warn "iptables 配置失败，请手动放行端口 ${PORT}"
    else
        info "未检测到防火墙，跳过"
    fi

    # ---- Step 5: 启动服务 ----
    step 5 $TOTAL_STEPS "启动 ClawPanel..."
    if [ "$SYS_OS" = "linux" ] && command -v systemctl &>/dev/null; then
        systemctl start ${SERVICE_NAME}
        sleep 1
        if systemctl is-active --quiet ${SERVICE_NAME}; then
            log "服务启动成功"
        else
            warn "服务启动可能失败，请检查: journalctl -u ${SERVICE_NAME} -f"
        fi
    elif [ "$SYS_OS" = "darwin" ]; then
        sleep 1
        log "服务已通过 launchd 启动"
    fi

    # ==================== 安装完成 ====================
    local SERVER_IP=$(get_ip)
    echo ""
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "${GREEN}                                                                 ${NC}"
    echo -e "${GREEN}   ClawPanel v${VERSION} 安装完成!                                ${NC}"
    echo -e "${GREEN}                                                                 ${NC}"
    echo -e "${GREEN}=================================================================${NC}"
    echo ""
    echo -e "  ${BOLD}面板地址${NC}:  ${CYAN}http://${SERVER_IP}:${PORT}${NC}"
    echo -e "  ${BOLD}默认密码${NC}:  ${CYAN}clawpanel${NC}"
    echo ""
    echo -e "  ${BOLD}安装目录${NC}:  ${INSTALL_DIR}"
    echo -e "  ${BOLD}数据目录${NC}:  ${INSTALL_DIR}/data"
    echo -e "  ${BOLD}配置文件${NC}:  ${INSTALL_DIR}/data/config.json (首次启动后生成)"
    echo ""
    if [ "$SYS_OS" = "linux" ]; then
        echo -e "  ${BOLD}管理命令${NC}:"
        echo -e "    systemctl start ${SERVICE_NAME}    ${CYAN}# 启动${NC}"
        echo -e "    systemctl stop ${SERVICE_NAME}     ${CYAN}# 停止${NC}"
        echo -e "    systemctl restart ${SERVICE_NAME}  ${CYAN}# 重启${NC}"
        echo -e "    systemctl status ${SERVICE_NAME}   ${CYAN}# 状态${NC}"
        echo -e "    journalctl -u ${SERVICE_NAME} -f   ${CYAN}# 日志${NC}"
    elif [ "$SYS_OS" = "darwin" ]; then
        echo -e "  ${BOLD}管理命令${NC}:"
        echo -e "    sudo launchctl start com.clawpanel.service   ${CYAN}# 启动${NC}"
        echo -e "    sudo launchctl stop com.clawpanel.service    ${CYAN}# 停止${NC}"
        echo -e "    tail -f ${INSTALL_DIR}/data/clawpanel.log    ${CYAN}# 日志${NC}"
    fi
    echo ""
    echo -e "  ${BOLD}卸载命令${NC}:"
    if [ "$SYS_OS" = "linux" ]; then
        echo -e "    systemctl stop ${SERVICE_NAME} && systemctl disable ${SERVICE_NAME}"
        echo -e "    rm -f /etc/systemd/system/${SERVICE_NAME}.service && systemctl daemon-reload"
        echo -e "    rm -rf ${INSTALL_DIR}"
    elif [ "$SYS_OS" = "darwin" ]; then
        echo -e "    sudo launchctl unload /Library/LaunchDaemons/com.clawpanel.service.plist"
        echo -e "    sudo rm -f /Library/LaunchDaemons/com.clawpanel.service.plist"
        echo -e "    sudo rm -rf ${INSTALL_DIR}"
    fi
    echo ""
    echo -e "  ${RED}${BOLD}!! 请登录后立即修改默认密码 !!${NC}"
    echo ""
    echo -e "${GREEN}=================================================================${NC}"
    echo ""
}

main "$@"
