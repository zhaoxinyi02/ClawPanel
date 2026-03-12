#!/bin/bash
set -euo pipefail

INSTALL_DIR="/opt/clawpanel-lite"
SERVICE_NAME="clawpanel-lite"
BIN_NAME="clawpanel-lite"
PORT="19527"
REPO="zhaoxinyi02/ClawPanel"
GITEE_REPO="zxy000006/ClawPanel"
TAG_PREFIX="lite-v"
GITEE_RAW_BASE="https://gitee.com/${GITEE_REPO}/raw/main"
GITEE_META_URL="${GITEE_RAW_BASE}/release/update-lite.json"
GITEE_RELEASE_BASE="https://gitee.com/${GITEE_REPO}/releases/download"
GITHUB_RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=20"
GITEE_RELEASES_API="https://gitee.com/api/v5/repos/${GITEE_REPO}/releases?per_page=20"
DEFAULT_VERSION="0.1.9"

RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
BLUE='\033[34m'
MAGENTA='\033[35m'
CYAN='\033[36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[Lite]${NC} $1"; }
info() { echo -e "${CYAN}[Lite]${NC} $1"; }
warn() { echo -e "${YELLOW}[Lite]${NC} $1"; }
err()  { echo -e "${RED}[Lite]${NC} $1" >&2; exit 1; }
step() { echo -e "${MAGENTA}[${1}/${2}]${NC} ${BOLD}$3${NC}"; }

print_banner() {
  echo ""
  echo -e "${BLUE}=================================================================${NC}"
  echo -e "${BLUE}                                                                 ${NC}"
  echo -e "${BLUE}   ██████╗██╗      █████╗ ██╗    ██╗██████╗  █████╗ ███╗   ██╗   ${NC}"
  echo -e "${BLUE}  ██╔════╝██║     ██╔══██╗██║    ██║██╔══██╗██╔══██╗████╗  ██║   ${NC}"
  echo -e "${BLUE}  ██║     ██║     ███████║██║ █╗ ██║██████╔╝███████║██╔██╗ ██║   ${NC}"
  echo -e "${BLUE}  ██║     ██║     ██╔══██║██║███╗██║██╔═══╝ ██╔══██║██║╚██╗██║   ${NC}"
  echo -e "${BLUE}  ╚██████╗███████╗██║  ██║╚███╔███╔╝██║     ██║  ██║██║ ╚████║   ${NC}"
  echo -e "${BLUE}   ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝   ${NC}"
  echo -e "${BLUE}                                                                 ${NC}"
  echo -e "${BLUE}   ClawPanel Lite v${VERSION} — 开箱即用 OpenClaw 托管版          ${NC}"
  echo -e "${BLUE}   GitHub: https://github.com/${REPO}                            ${NC}"
  echo -e "${BLUE}                                                                 ${NC}"
  echo -e "${BLUE}=================================================================${NC}"
  echo ""
}

fetch_text() {
  local url=$1
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 8 --max-time 20 -fsSL "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -T 20 -qO- "$url"
  else
    return 1
  fi
}

get_latest_version_from_github() {
  local body tag
  body=$(fetch_text "$GITHUB_RELEASES_API" 2>/dev/null || true)
  tag=$(printf '%s' "$body" | awk -v prefix="$TAG_PREFIX" -F'"' '$2=="tag_name" && index($4,prefix)==1 {print $4; exit}')
  if [[ -n "$tag" ]]; then
    printf '%s\n' "${tag#${TAG_PREFIX}}"
  fi
}

get_latest_version_from_gitee() {
  local body ver
  body=$(fetch_text "$GITEE_META_URL" 2>/dev/null || true)
  ver=$(printf '%s' "$body" | awk -F'"' '/"latest_version"/ {print $4; exit}')
  if [[ -n "$ver" ]]; then
    printf '%s\n' "$ver"
  fi
}

download_file() {
  local url=$1
  local dest=$2
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 10 --max-time 300 --retry 2 --retry-delay 2 -fL "$url" -o "$dest"
  else
    wget -T 300 --tries=2 -O "$dest" "$url"
  fi
}

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
  if [[ -n "$DOWNLOAD_SOURCE" ]]; then
    return
  fi
  echo -e "${CYAN}[Lite]${NC} 请选择下载线路："
  echo -e "  ${BOLD}1) GitHub${NC}      中国香港及境外服务器推荐"
  echo -e "  ${BOLD}2) Gitee${NC}       中国大陆服务器推荐，更稳当一些"
  if [[ -t 0 ]]; then
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

resolve_latest_version() {
  local primary=$1
  local version=""
  if [[ "$primary" == "github" ]]; then
    version=$(get_latest_version_from_github)
    [[ -n "$version" ]] || version=$(get_latest_version_from_gitee)
  else
    version=$(get_latest_version_from_gitee)
    [[ -n "$version" ]] || version=$(get_latest_version_from_github)
  fi
  printf '%s\n' "$version"
}

download_with_selected_source() {
  local primary=$1
  local package_name=$2
  local dest=$3
  local secondary
  secondary=$(other_source "$primary")
  local primary_url secondary_url
  if [[ "$primary" == "github" ]]; then
    primary_url="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${VERSION}/${package_name}"
    secondary_url="${GITEE_RELEASE_BASE}/${TAG_PREFIX}${VERSION}/${package_name}"
  else
    primary_url="${GITEE_RELEASE_BASE}/${TAG_PREFIX}${VERSION}/${package_name}"
    secondary_url="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${VERSION}/${package_name}"
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

choose_download_source

VERSION=${VERSION:-$(resolve_latest_version "$DOWNLOAD_SOURCE")}
VERSION=${VERSION:-$DEFAULT_VERSION}

PACKAGE_NAME="clawpanel-lite-core-v${VERSION}-linux-amd64.tar.gz"

print_banner

[[ $(id -u) -eq 0 ]] || err "请使用 root 或 sudo 运行此脚本。"
command -v systemctl >/dev/null 2>&1 || err "当前系统缺少 systemctl，暂不支持自动安装 Lite。"

TOTAL_STEPS=5
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

info "安装目录: ${INSTALL_DIR}"
info "服务名称: ${SERVICE_NAME}"
info "面板端口: ${PORT}"
echo ""

step 1 $TOTAL_STEPS "准备安装目录"
mkdir -p "$INSTALL_DIR"
log "目录已就绪: $INSTALL_DIR"

step 2 $TOTAL_STEPS "下载 ClawPanel Lite v${VERSION}"
if [[ "$DOWNLOAD_SOURCE" == "github" ]]; then
  info "已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到 Gitee。"
else
  info "已选择 Gitee（中国大陆服务器推荐），失败时自动回退到 GitHub。"
fi
download_with_selected_source "$DOWNLOAD_SOURCE" "$PACKAGE_NAME" "$TMP_DIR/$PACKAGE_NAME" || err "GitHub 和 Gitee 均下载失败，请检查网络后重试。"
log "下载完成 (${DOWNLOAD_SOURCE_ACTUAL})"

step 3 $TOTAL_STEPS "部署 Lite 运行环境"
systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
rm -rf "$INSTALL_DIR"/*
tar -xzf "$TMP_DIR/$PACKAGE_NAME" -C "$INSTALL_DIR"
chown -R root:root "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/$BIN_NAME" "$INSTALL_DIR/bin/clawlite-openclaw"
if [[ -f "$INSTALL_DIR/runtime/node/bin/node" ]]; then
  chmod +x "$INSTALL_DIR/runtime/node/bin/node"
fi
if [[ -f "$INSTALL_DIR/runtime/node/node" ]]; then
  chmod +x "$INSTALL_DIR/runtime/node/node"
fi
ln -sf "$INSTALL_DIR/$BIN_NAME" /usr/local/bin/clawpanel-lite
ln -sf "$INSTALL_DIR/bin/clawlite-openclaw" /usr/local/bin/clawlite-openclaw
log "Lite 文件已部署"

step 4 $TOTAL_STEPS "注册 systemd 服务"
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=ClawPanel Lite
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BIN_NAME}
Restart=always
RestartSec=5
Environment=CLAWPANEL_EDITION=lite
Environment=CLAWPANEL_DATA=${INSTALL_DIR}/data

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
log "systemd 服务已更新"

step 5 $TOTAL_STEPS "启动 ClawPanel Lite"
systemctl enable --now "$SERVICE_NAME"
SERVICE_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
SERVICE_IP=${SERVICE_IP:-localhost}
log "ClawPanel Lite 安装完成"
echo ""
echo -e "${GREEN}•${NC} 当前版本: ${BOLD}${VERSION}${NC}"
echo -e "${GREEN}•${NC} 安装目录: ${BOLD}${INSTALL_DIR}${NC}"
echo -e "${GREEN}•${NC} 服务名称: ${BOLD}${SERVICE_NAME}${NC}"
echo -e "${GREEN}•${NC} Lite CLI: ${BOLD}clawlite-openclaw${NC}"
echo -e "${GREEN}•${NC} 访问地址: ${BOLD}http://${SERVICE_IP}:${PORT}${NC}"
echo ""
