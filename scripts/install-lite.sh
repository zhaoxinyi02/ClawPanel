#!/bin/bash
set -euo pipefail

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"
INSTALL_DIR="/opt/clawpanel-lite"
DATA_DIR="/var/lib/clawpanel-lite"
SERVICE_NAME="clawpanel-lite"
BIN_NAME="clawpanel-lite"
PORT="19527"
REPO="zhaoxinyi02/ClawPanel"
GITEE_REPO="zxy000006/ClawPanel"
TAG_PREFIX="lite-v"
GITEE_RAW_BASE="https://gitee.com/${GITEE_REPO}/raw/main"
ACCEL_BASE="${ACCEL_BASE:-${CLAWPANEL_PUBLIC_BASE}/api/panel/update-mirror}"
ACCEL_META_URL="${ACCEL_META_URL:-${ACCEL_BASE}/lite}"
GITHUB_RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=20"

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

get_latest_version_from_accel() {
  local body ver
  body=$(fetch_text "$ACCEL_META_URL" 2>/dev/null || true)
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
    accel) echo "accel" ;;
    *) echo "" ;;
  esac
}

other_source() {
  case "$1" in
    github) echo "accel" ;;
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
  echo -e "  ${BOLD}2) 加速服务器${NC}  中国大陆服务器推荐，更稳当一些"
  if [[ -t 0 ]]; then
    read -r -p "请输入 [1/2]（默认 2）: " source_choice
    case "$source_choice" in
      1) DOWNLOAD_SOURCE="github" ;;
      2|"") DOWNLOAD_SOURCE="accel" ;;
      *) DOWNLOAD_SOURCE="accel" ;;
    esac
  else
    DOWNLOAD_SOURCE="accel"
  fi
}

resolve_latest_version() {
  local primary=$1
  local version=""
  if [[ "$primary" == "github" ]]; then
    version=$(get_latest_version_from_github)
    [[ -n "$version" ]] || version=$(get_latest_version_from_accel)
  else
    version=$(get_latest_version_from_accel)
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
    secondary_url="${ACCEL_BASE}/lite/files/${package_name}"
  else
    primary_url="${ACCEL_BASE}/lite/files/${package_name}"
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

LOCAL_PACKAGE_PATH="${LOCAL_PACKAGE:-}"

if [[ -n "$LOCAL_PACKAGE_PATH" ]]; then
  if [[ ! -f "$LOCAL_PACKAGE_PATH" ]]; then
    err "指定的本地 Lite 构建包不存在：$LOCAL_PACKAGE_PATH"
  fi
  base_name=$(basename "$LOCAL_PACKAGE_PATH")
  if [[ "$base_name" =~ ^clawpanel-lite-core-v([0-9][0-9A-Za-z._-]*)-linux-amd64\.tar\.gz$ ]]; then
    VERSION="${BASH_REMATCH[1]}"
  fi
else
  choose_download_source
  VERSION=${VERSION:-$(resolve_latest_version "$DOWNLOAD_SOURCE")}
fi

if [[ -z "${VERSION:-}" ]]; then
  err "无法获取最新版本号。请检查网络连接，或通过 VERSION=x.y.z 环境变量手动指定版本后重试。"
fi

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
mkdir -p "$DATA_DIR"
log "目录已就绪: $INSTALL_DIR"
log "数据目录已就绪: $DATA_DIR"

step 2 $TOTAL_STEPS "下载 ClawPanel Lite v${VERSION}"
if [[ -n "$LOCAL_PACKAGE_PATH" ]]; then
  cp -f "$LOCAL_PACKAGE_PATH" "$TMP_DIR/$PACKAGE_NAME"
  DOWNLOAD_SOURCE_ACTUAL="local"
  info "已使用当前目录中的本地 Lite 构建包进行安装。"
elif [[ "$DOWNLOAD_SOURCE" == "github" ]]; then
  info "已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到加速服务器。"
  download_with_selected_source "$DOWNLOAD_SOURCE" "$PACKAGE_NAME" "$TMP_DIR/$PACKAGE_NAME" || err "GitHub 和加速服务器均下载失败，请检查网络后重试。"
else
  info "已选择加速服务器（中国大陆服务器推荐），失败时自动回退到 GitHub。"
  download_with_selected_source "$DOWNLOAD_SOURCE" "$PACKAGE_NAME" "$TMP_DIR/$PACKAGE_NAME" || err "GitHub 和加速服务器均下载失败，请检查网络后重试。"
fi
log "下载完成 (${DOWNLOAD_SOURCE_ACTUAL})"

step 3 $TOTAL_STEPS "部署 Lite 运行环境"
systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
LEGACY_DATA_DIR="$INSTALL_DIR/data"
DATA_OWNER=""
if [[ -d "$DATA_DIR" ]] && command -v stat >/dev/null 2>&1; then
	DATA_OWNER=$(stat -c '%u:%g' "$DATA_DIR" 2>/dev/null || true)
elif [[ -d "$LEGACY_DATA_DIR" ]] && command -v stat >/dev/null 2>&1; then
	DATA_OWNER=$(stat -c '%u:%g' "$LEGACY_DATA_DIR" 2>/dev/null || true)
fi

if [[ -d "$LEGACY_DATA_DIR" ]]; then
	if [[ ! -e "$DATA_DIR/clawpanel.json" ]] && [[ ! -e "$DATA_DIR/openclaw-config/openclaw.json" ]]; then
		info "检测到旧版内置数据目录，迁移到外部持久化目录..."
		mkdir -p "$DATA_DIR"
		cp -a "$LEGACY_DATA_DIR/." "$DATA_DIR/"
	else
		info "检测到旧版内置数据目录，但外部数据目录已有内容，跳过自动覆盖。"
	fi
fi

rm -rf "$INSTALL_DIR"/*
tar -xzf "$TMP_DIR/$PACKAGE_NAME" -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/$BIN_NAME" "$INSTALL_DIR/bin/clawlite-openclaw"
if [[ -f "$INSTALL_DIR/runtime/node/bin/node" ]]; then
  chmod +x "$INSTALL_DIR/runtime/node/bin/node"
fi
if [[ -f "$INSTALL_DIR/runtime/node/node" ]]; then
  chmod +x "$INSTALL_DIR/runtime/node/node"
fi
mkdir -p "$DATA_DIR"
if [[ -n "$DATA_OWNER" ]] && [[ -d "$DATA_DIR" ]]; then
	chown -R "$DATA_OWNER" "$DATA_DIR" 2>/dev/null || true
fi
rm -rf "$INSTALL_DIR/data"
ln -sfn "$DATA_DIR" "$INSTALL_DIR/data"
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
Environment=CLAWPANEL_DATA=${DATA_DIR}
Environment=NODE_OPTIONS=--max-old-space-size=2048

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
echo -e "${GREEN}•${NC} 数据目录: ${BOLD}${DATA_DIR}${NC}"
echo -e "${GREEN}•${NC} 服务名称: ${BOLD}${SERVICE_NAME}${NC}"
echo -e "${GREEN}•${NC} Lite CLI: ${BOLD}clawlite-openclaw${NC}"
echo -e "${GREEN}•${NC} 访问地址: ${BOLD}http://${SERVICE_IP}:${PORT}${NC}"
echo ""
