#!/bin/bash
# ============================================================
# ClawPanel Lite 手动更新脚本
# 适用于面板内自动更新失败（如旧加速服务器失效）的用户
# 用法:
#   curl -fsSL http://43.248.142.249:19527/scripts/update-lite.sh | sudo bash
# 或:
#   wget -qO- http://43.248.142.249:19527/scripts/update-lite.sh | sudo bash
# ============================================================

set -e

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"
INSTALL_DIR="/opt/clawpanel-lite"
SERVICE_NAME="clawpanel-lite"
BIN_NAME="clawpanel-lite"
REPO="zhaoxinyi02/ClawPanel"
TAG_PREFIX="lite-v"
ACCEL_BASE="${ACCEL_BASE:-${CLAWPANEL_PUBLIC_BASE}/api/panel/update-mirror}"
ACCEL_META_URL="${ACCEL_META_URL:-${ACCEL_BASE}/lite}"
GITHUB_RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=20"

RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
CYAN='\033[36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[Lite Update]${NC} $1"; }
info() { echo -e "${CYAN}[Lite Update]${NC} $1"; }
warn() { echo -e "${YELLOW}[Lite Update]${NC} $1"; }
err()  { echo -e "${RED}[Lite Update]${NC} $1" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || err "请使用 root 或 sudo 运行此脚本。"
[ -f "${INSTALL_DIR}/${BIN_NAME}" ] || err "未检测到 ClawPanel Lite 安装目录 ${INSTALL_DIR}，请先安装。"
command -v systemctl >/dev/null 2>&1 || err "当前系统缺少 systemctl，暂不支持此脚本。"

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

echo ""
echo -e "${CYAN}======================================================${NC}"
echo -e "${CYAN}        ClawPanel Lite 手动更新工具                   ${NC}"
echo -e "${CYAN}======================================================${NC}"
echo ""

# Lite 只支持 Linux amd64
[ "$(uname -s)" = "Linux" ] || err "此脚本仅支持 Linux。"
[ "$(uname -m)" = "x86_64" ] || err "此脚本仅支持 x86_64 架构。arm64 / macOS 请从 GitHub Releases 手动下载。"

# 获取当前版本
CURRENT_VERSION="unknown"
if [ -f "${INSTALL_DIR}/${BIN_NAME}" ]; then
  CURRENT_VERSION=$("${INSTALL_DIR}/${BIN_NAME}" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
fi

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

# Lite 是整包 tar.gz，只更新可执行文件（不覆盖数据和运行时）
PACKAGE_NAME="clawpanel-lite-core-v${TARGET_VERSION}-linux-amd64.tar.gz"
ACCEL_URL="${ACCEL_BASE}/lite/files/${PACKAGE_NAME}"
GITHUB_URL="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${TARGET_VERSION}/${PACKAGE_NAME}"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo ""
log "下载 ClawPanel Lite v${TARGET_VERSION} ..."
info "优先使用加速服务器，失败自动切换 GitHub..."

if download_file "$ACCEL_URL" "$TMP_DIR/${PACKAGE_NAME}"; then
  DOWNLOAD_FROM="加速服务器"
elif download_file "$GITHUB_URL" "$TMP_DIR/${PACKAGE_NAME}"; then
  DOWNLOAD_FROM="GitHub"
else
  err "加速服务器和 GitHub 均下载失败，请检查网络后重试。"
fi
log "下载完成 (来源: ${DOWNLOAD_FROM})"

log "解压更新包..."
EXTRACT_DIR="$TMP_DIR/extract"
mkdir -p "$EXTRACT_DIR"
tar -xzf "$TMP_DIR/${PACKAGE_NAME}" -C "$EXTRACT_DIR"

# 检查解压出的可执行文件
NEW_BIN="$EXTRACT_DIR/${BIN_NAME}"
[ -f "$NEW_BIN" ] || err "更新包中未找到 ${BIN_NAME}，包可能损坏。"

log "停止服务..."
systemctl stop "$SERVICE_NAME" 2>/dev/null || true
sleep 1

log "备份旧版本..."
cp -f "${INSTALL_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}.bak" 2>/dev/null || true

log "替换可执行文件..."
install -m755 "$NEW_BIN" "${INSTALL_DIR}/${BIN_NAME}"
# 同步更新 clawlite-openclaw（如果包里有）
if [ -f "$EXTRACT_DIR/bin/clawlite-openclaw" ]; then
  install -m755 "$EXTRACT_DIR/bin/clawlite-openclaw" "${INSTALL_DIR}/bin/clawlite-openclaw"
  log "clawlite-openclaw 已同步更新"
fi

log "启动服务..."
systemctl start "$SERVICE_NAME"
sleep 3
if systemctl is-active --quiet "$SERVICE_NAME"; then
  log "服务启动成功 ✓"
else
  warn "服务启动异常，尝试回滚..."
  cp -f "${INSTALL_DIR}/${BIN_NAME}.bak" "${INSTALL_DIR}/${BIN_NAME}" 2>/dev/null || true
  systemctl start "$SERVICE_NAME" || true
  err "更新失败，已回滚旧版本。请查看日志: journalctl -u ${SERVICE_NAME} -n 50"
fi

echo ""
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}   ClawPanel Lite 更新完成！                          ${NC}"
echo -e "${GREEN}======================================================${NC}"
echo ""
echo -e "  ${BOLD}更新前${NC}: ${CURRENT_VERSION}"
echo -e "  ${BOLD}当前版本${NC}: ${TARGET_VERSION}"
echo -e "  ${BOLD}面板地址${NC}: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'localhost'):19527"
echo ""
echo -e "  后续更新可直接在面板内操作（系统配置 → 检查更新）"
echo ""
