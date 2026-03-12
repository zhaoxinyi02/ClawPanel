#!/bin/bash
# ============================================================
# ClawPanel 一键安装脚本 (Linux/macOS)
# 自动获取最新 Release 版本，无需手动更新脚本
# 用法:
#   curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install.sh -o install.sh && sudo bash install.sh
# 或:
#   wget -O install.sh https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install.sh && sudo bash install.sh
# ============================================================

set -e

INSTALL_DIR="/opt/clawpanel"
SERVICE_NAME="clawpanel"
BINARY_NAME="clawpanel"
REPO="zhaoxinyi02/ClawPanel"
GITEE_REPO="zxy000006/ClawPanel"
TAG_PREFIX="pro-v"
GITHUB_RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=20"
GITEE_RELEASES_API="https://gitee.com/api/v5/repos/${GITEE_REPO}/releases?per_page=20"
PORT="19527"
GITEE_RAW_BASE="https://gitee.com/${GITEE_REPO}/raw/main"
GITEE_RELEASE_BASE="https://gitee.com/${GITEE_REPO}/releases/download"
DEFAULT_VERSION="5.2.10"
UPDATE_META="${UPDATE_META:-update-pro.json}"

# ==================== 自动获取最新版本 ====================
get_latest_version() {
    local ver=""
    local tag=""
    if command -v curl &>/dev/null; then
        tag=$(curl -fsSL "${GITEE_RAW_BASE}/release/${UPDATE_META}" 2>/dev/null | awk -F'"' '/"latest_version"/ {print $4; exit}')
        if [ -n "$tag" ]; then echo "${tag:-$DEFAULT_VERSION}"; return; fi
        tag=$(curl -fsSL "${GITHUB_RELEASES_API}" 2>/dev/null | awk -v prefix="$TAG_PREFIX" -F'"' '$2=="tag_name" && index($4,prefix)==1 {print $4; exit}')
    elif command -v wget &>/dev/null; then
        tag=$(wget -qO- "${GITEE_RAW_BASE}/release/${UPDATE_META}" 2>/dev/null | awk -F'"' '/"latest_version"/ {print $4; exit}')
        if [ -n "$tag" ]; then echo "${tag:-$DEFAULT_VERSION}"; return; fi
        tag=$(wget -qO- "${GITHUB_RELEASES_API}" 2>/dev/null | awk -v prefix="$TAG_PREFIX" -F'"' '$2=="tag_name" && index($4,prefix)==1 {print $4; exit}')
    fi

    ver="${tag#${TAG_PREFIX}}"

    # Safety: only accept digits/dots/dashes in version
    if [[ ! "$ver" =~ ^[0-9][0-9A-Za-z._-]*$ ]]; then
        ver=""
    fi

    echo "${ver:-$DEFAULT_VERSION}"
}

VERSION=$(get_latest_version)

normalize_source() {
  case "${1:-}" in
    github) echo "github" ;;
    gitee) echo "gitee" ;;
    *) echo "" ;;
  esac
}

other_source() {
  case "$1" in
    github) echo "gitee" ;;
    *) echo "github" ;;
  esac
}

choose_download_source() {
  DOWNLOAD_SOURCE=$(normalize_source "${DOWNLOAD_SOURCE:-}")
  if [ -n "$DOWNLOAD_SOURCE" ]; then
    return
  fi
  echo -e "${CYAN}[ClawPanel]${NC} 请选择下载线路："
  echo -e "  ${BOLD}1) GitHub${NC}      中国香港及境外服务器推荐"
  echo -e "  ${BOLD}2) Gitee${NC}       中国大陆服务器推荐，更稳当一些"
  if [ -t 0 ]; then
    read -r -p "请输入 [1/2]（默认 2）: " source_choice
    case "$source_choice" in
      1) DOWNLOAD_SOURCE="github" ;;
      2|"") DOWNLOAD_SOURCE="gitee" ;;
      *) DOWNLOAD_SOURCE="gitee" ;;
    esac
  else
    DOWNLOAD_SOURCE="gitee"
  fi
}

download_file() {
  local url="$1"
  local dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 10 --max-time 300 --retry 2 --retry-delay 2 --retry-connrefused -fL "$url" -o "$dest"
  else
    wget -T 300 --tries=2 -O "$dest" "$url"
  fi
}

download_with_selected_source() {
  local primary="$1"
  local binary_file="$2"
  local dest="$3"
  local primary_url secondary_url secondary
  secondary=$(other_source "$primary")
  if [ "$primary" = "github" ]; then
    primary_url="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${VERSION}/${binary_file}"
    secondary_url="${GITEE_RELEASE_BASE}/${TAG_PREFIX}${VERSION}/${binary_file}"
  else
    primary_url="${GITEE_RELEASE_BASE}/${TAG_PREFIX}${VERSION}/${binary_file}"
    secondary_url="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${VERSION}/${binary_file}"
  fi
  if download_file "$primary_url" "$dest"; then
    DOWNLOAD_SOURCE_ACTUAL="$primary"
    return 0
  fi
  warn "${primary} 下载失败，切换到 ${secondary}..."
  if download_file "$secondary_url" "$dest"; then
    DOWNLOAD_SOURCE_ACTUAL="$secondary"
    return 0
  fi
  return 1
}

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

resolve_service_user() {
    local user="${CLAWPANEL_SERVICE_USER:-}"
    user=$(printf '%s' "$user" | xargs 2>/dev/null || true)
    if [ -z "$user" ]; then
        echo "root"
        return
    fi
    echo "$user"
}

resolve_service_group() {
    local user="$1"
    if [ -z "$user" ] || [ "$user" = "root" ]; then
        echo "root"
        return
    fi
    if command -v id &>/dev/null; then
        id -gn "$user" 2>/dev/null || echo "$user"
        return
    fi
    echo "$user"
}

resolve_service_home() {
    local user="$1"
    if [ -z "$user" ] || [ "$user" = "root" ]; then
        echo "/root"
        return
    fi
    if command -v getent &>/dev/null; then
        local home
        home=$(getent passwd "$user" | cut -d: -f6)
        if [ -n "$home" ]; then
            echo "$home"
            return
        fi
    fi
    echo "/home/${user}"
}

validate_service_user() {
    local user="$1"
    if [ -z "$user" ]; then
        err "服务用户不能为空"
    fi
    if [ "$user" = "root" ]; then
        return
    fi
    if ! id "$user" &>/dev/null; then
        err "指定的服务用户不存在: ${user}。如需非 root 安装，请先创建该用户，或移除 CLAWPANEL_SERVICE_USER 后重试。"
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
    local BINARY_FILE="${BINARY_NAME}-v${VERSION}-${SYS_OS}-${SYS_ARCH}"
    local TOTAL_STEPS=5
    local SERVICE_USER=$(resolve_service_user)
    local SERVICE_GROUP="root"
    local SERVICE_HOME="/root"

    validate_service_user "$SERVICE_USER"
    SERVICE_GROUP=$(resolve_service_group "$SERVICE_USER")
    SERVICE_HOME=$(resolve_service_home "$SERVICE_USER")

    info "系统信息: ${SYS_OS}/${SYS_ARCH}"
    info "安装目录: ${INSTALL_DIR}"
    if [ "$SERVICE_USER" = "root" ]; then
        info "服务用户: root（默认兼容模式）"
    else
        info "服务用户: ${SERVICE_USER}:${SERVICE_GROUP}"
        warn "已启用显式非 root 服务用户模式；请确保该用户对 OpenClaw 配置目录也有读写权限。"
    fi
    echo ""

    choose_download_source

    # ---- Step 1: 创建目录 ----
    step 1 $TOTAL_STEPS "创建安装目录..."
    mkdir -p "${INSTALL_DIR}"
    mkdir -p "${INSTALL_DIR}/data"
    log "目录已创建: ${INSTALL_DIR}"

    # ---- Step 2: 下载二进制 ----
    step 2 $TOTAL_STEPS "下载 ClawPanel v${VERSION}..."
    if [ "$DOWNLOAD_SOURCE" = "github" ]; then
        info "已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到 Gitee。"
    else
        info "已选择 Gitee（中国大陆服务器推荐），失败时自动回退到 GitHub。"
    fi
    download_with_selected_source "$DOWNLOAD_SOURCE" "$BINARY_FILE" "${INSTALL_DIR}/${BINARY_NAME}" || err "下载失败！请检查网络连接。"

    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    if [ "$SERVICE_USER" != "root" ]; then
        chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_DIR}" 2>/dev/null || \
        err "无法将安装目录授权给 ${SERVICE_USER}:${SERVICE_GROUP}，请检查用户/组权限后重试。"
    fi
    local FILE_SIZE=$(du -h "${INSTALL_DIR}/${BINARY_NAME}" | awk '{print $1}')
    log "下载完成 (${FILE_SIZE}) · 线路: ${DOWNLOAD_SOURCE_ACTUAL}"

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
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Restart=always
RestartSec=5
LimitNOFILE=65535
Environment=CLAWPANEL_DATA=${INSTALL_DIR}/data
Environment=HOME=${SERVICE_HOME}

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
