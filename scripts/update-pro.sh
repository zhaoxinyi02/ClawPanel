#!/bin/bash
# ============================================================
# ClawPanel Pro 手动更新脚本
# 适用于面板内自动更新失败（如旧加速服务器失效）的用户
# 用法:
#   curl -fsSL http://43.248.142.249:19527/scripts/update-pro.sh | sudo bash
# 或:
#   wget -qO- http://43.248.142.249:19527/scripts/update-pro.sh | sudo bash
# ============================================================

set -e

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"
INSTALL_DIR="/opt/clawpanel"
SERVICE_NAME="clawpanel"
BINARY_NAME="clawpanel"
REPO="zhaoxinyi02/ClawPanel"
TAG_PREFIX="pro-v"
ACCEL_BASE="${ACCEL_BASE:-${CLAWPANEL_PUBLIC_BASE}/api/panel/update-mirror}"
ACCEL_META_URL="${ACCEL_META_URL:-${ACCEL_BASE}/pro}"
GITHUB_RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=20"

RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
CYAN='\033[36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[Pro Update]${NC} $1"; }
info() { echo -e "${CYAN}[Pro Update]${NC} $1"; }
warn() { echo -e "${YELLOW}[Pro Update]${NC} $1"; }
err()  { echo -e "${RED}[Pro Update]${NC} $1" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || err "请使用 root 或 sudo 运行此脚本。"
[ -f "${INSTALL_DIR}/${BINARY_NAME}" ] || err "未检测到 ClawPanel Pro 安装目录 ${INSTALL_DIR}，请先安装。"

fetch_text() {
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 8 --max-time 20 -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -T 20 -qO- "$1"
  else
    return 1
  fi
}

download_file() {
  local url=$1 dest=$2
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 10 --max-time 300 --retry 2 --retry-delay 3 -fL "$url" -o "$dest"
  else
    wget -T 300 --tries=2 -O "$dest" "$url"
  fi
}

get_latest_version() {
  local ver
  ver=$(fetch_text "$ACCEL_META_URL" 2>/dev/null | awk -F'"' '/"latest_version"/{print $4;exit}')
  [ -n "$ver" ] && { echo "$ver"; return; }
  local tag
  tag=$(fetch_text "$GITHUB_RELEASES_API" 2>/dev/null | awk -v p="$TAG_PREFIX" -F'"' '$2=="tag_name"&&index($4,p)==1{print $4;exit}')
  ver="${tag#${TAG_PREFIX}}"
  [[ "$ver" =~ ^[0-9] ]] && echo "$ver"
}

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux)  echo "linux" ;;
    darwin) echo "darwin" ;;
    *)      err "不支持的操作系统" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)             err "不支持的 CPU 架构" ;;
  esac
}

echo ""
echo -e "${CYAN}======================================================${NC}"
echo -e "${CYAN}        ClawPanel Pro 手动更新工具                    ${NC}"
echo -e "${CYAN}======================================================${NC}"
echo ""

SYS_OS=$(detect_os)
SYS_ARCH=$(detect_arch)

# 获取当前版本
CURRENT_VERSION="unknown"
if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
  CURRENT_VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
fi

info "当前系统: ${SYS_OS}/${SYS_ARCH}"
info "当前版本: ${CURRENT_VERSION}"
info "正在获取最新版本..."

TARGET_VERSION="${VERSION:-$(get_latest_version)}"
[ -n "$TARGET_VERSION" ] || err "无法获取最新版本号，请检查网络或通过 VERSION=x.y.z 手动指定。"

info "目标版本: ${TARGET_VERSION}"

if [ "$CURRENT_VERSION" = "$TARGET_VERSION" ]; then
  echo ""
  log "当前已是最新版本 ${TARGET_VERSION}，无需更新。"
  echo ""
  exit 0
fi

BINARY_FILE="${BINARY_NAME}-v${TARGET_VERSION}-${SYS_OS}-${SYS_ARCH}"
ACCEL_URL="${ACCEL_BASE}/pro/files/${BINARY_FILE}"
GITHUB_URL="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${TARGET_VERSION}/${BINARY_FILE}"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo ""
log "下载 ClawPanel Pro v${TARGET_VERSION} ..."
info "优先使用加速服务器，失败自动切换 GitHub..."

if download_file "$ACCEL_URL" "$TMP_DIR/${BINARY_FILE}"; then
  DOWNLOAD_FROM="加速服务器"
elif download_file "$GITHUB_URL" "$TMP_DIR/${BINARY_FILE}"; then
  DOWNLOAD_FROM="GitHub"
else
  err "加速服务器和 GitHub 均下载失败，请检查网络后重试。"
fi
log "下载完成 (来源: ${DOWNLOAD_FROM})"

log "停止服务..."
if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
  systemctl stop "$SERVICE_NAME"
fi

log "备份旧版本..."
cp -f "${INSTALL_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}.bak" 2>/dev/null || true

log "替换二进制文件..."
install -m755 "$TMP_DIR/${BINARY_FILE}" "${INSTALL_DIR}/${BINARY_NAME}"

log "启动服务..."
systemctl start "$SERVICE_NAME"
sleep 2
if systemctl is-active --quiet "$SERVICE_NAME"; then
  log "服务启动成功 ✓"
else
  warn "服务启动异常，尝试回滚..."
  cp -f "${INSTALL_DIR}/${BINARY_NAME}.bak" "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
  systemctl start "$SERVICE_NAME" || true
  err "更新失败，已回滚旧版本。请查看日志: journalctl -u ${SERVICE_NAME} -n 50"
fi

echo ""
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}   ClawPanel Pro 更新完成！                           ${NC}"
echo -e "${GREEN}======================================================${NC}"
echo ""
echo -e "  ${BOLD}更新前${NC}: ${CURRENT_VERSION}"
echo -e "  ${BOLD}当前版本${NC}: ${TARGET_VERSION}"
echo -e "  ${BOLD}面板地址${NC}: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'localhost'):19527"
echo ""
echo -e "  后续更新可直接在面板内操作（系统配置 → 检查更新）"
echo ""
