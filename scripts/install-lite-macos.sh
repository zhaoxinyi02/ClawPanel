#!/bin/bash
set -euo pipefail

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"
INSTALL_DIR="/opt/clawpanel-lite"
DATA_DIR="/var/lib/clawpanel-lite"
SERVICE_LABEL="com.clawpanel.lite.service"
REPO="zhaoxinyi02/ClawPanel"
TAG_PREFIX="lite-v"
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

log(){ printf "${GREEN}[Lite]${NC} %s\n" "$1"; }
info(){ printf "${CYAN}[Lite]${NC} %s\n" "$1"; }
warn(){ printf "${YELLOW}[Lite]${NC} %s\n" "$1"; }
err(){ printf "${RED}[Lite]${NC} %s\n" "$1" >&2; exit 1; }
step(){ printf "${MAGENTA}[%s/%s]${NC} ${BOLD}%s${NC}\n" "$1" "$2" "$3"; }

print_banner() {
  echo ""
  printf "${BLUE}=================================================================${NC}\n"
  printf "${BLUE}                                                                 ${NC}\n"
  printf "${BLUE}   ██████╗██╗      █████╗ ██╗    ██╗██████╗  █████╗ ███╗   ██╗   ${NC}\n"
  printf "${BLUE}  ██╔════╝██║     ██╔══██╗██║    ██║██╔══██╗██╔══██╗████╗  ██║   ${NC}\n"
  printf "${BLUE}  ██║     ██║     ███████║██║ █╗ ██║██████╔╝███████║██╔██╗ ██║   ${NC}\n"
  printf "${BLUE}  ██║     ██║     ██╔══██║██║███╗██║██╔═══╝ ██╔══██║██║╚██╗██║   ${NC}\n"
  printf "${BLUE}  ╚██████╗███████╗██║  ██║╚███╔███╔╝██║     ██║  ██║██║ ╚████║   ${NC}\n"
  printf "${BLUE}   ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝   ${NC}\n"
  printf "${BLUE}                                                                 ${NC}\n"
  printf "${BLUE}   ClawPanel Lite v%s — macOS 预览安装器                         ${NC}\n" "$VERSION"
  printf "${BLUE}   GitHub: https://github.com/%s                                 ${NC}\n" "$REPO"
  printf "${BLUE}                                                                 ${NC}\n"
  printf "${BLUE}=================================================================${NC}\n"
  echo ""
}

fetch_text() {
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 8 --max-time 20 -fsSL "$1"
  else
    return 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    arm64) echo "arm64" ;;
    x86_64) echo "amd64" ;;
    *) err "暂不支持的 macOS 架构: $(uname -m)" ;;
  esac
}

get_latest_version_from_github() {
  local body tag
  body=$(fetch_text "$GITHUB_RELEASES_API" 2>/dev/null || true)
  tag=$(printf '%s' "$body" | awk -v prefix="$TAG_PREFIX" -F'"' '$2=="tag_name" && index($4,prefix)==1 {print $4; exit}')
  [[ -n "$tag" ]] && printf '%s\n' "${tag#${TAG_PREFIX}}"
}

get_latest_version_from_accel() {
  local body ver
  body=$(fetch_text "$ACCEL_META_URL" 2>/dev/null || true)
  ver=$(printf '%s' "$body" | awk -F'"' '/"latest_version"/ {print $4; exit}')
  [[ -n "$ver" ]] && printf '%s\n' "$ver"
}

normalize_source(){ case "${1:-}" in github) echo github;; accel) echo accel;; *) echo "";; esac; }
other_source(){ [[ "$1" == github ]] && echo accel || echo github; }

choose_download_source() {
  DOWNLOAD_SOURCE=$(normalize_source "${DOWNLOAD_SOURCE:-}")
  if [[ -n "$DOWNLOAD_SOURCE" ]]; then return; fi
  echo "请选择下载线路："
  echo "  1) GitHub（中国香港及境外服务器推荐）"
  echo "  2) 加速服务器（中国大陆服务器推荐）"
  if [[ -t 0 ]]; then
    read -r -p "请输入 [1/2]（默认 1）: " source_choice
    case "$source_choice" in
      2) DOWNLOAD_SOURCE=accel ;;
      *) DOWNLOAD_SOURCE=github ;;
    esac
  else
    DOWNLOAD_SOURCE=github
  fi
}

download_file() { curl --connect-timeout 10 --max-time 300 --retry 2 --retry-delay 2 -fL "$1" -o "$2"; }

choose_download_source
[[ "$(uname -s)" == "Darwin" ]] || err "install-lite-macos.sh 只能在 macOS 上运行；你当前系统是 $(uname -s)。Linux 请使用 install-lite.sh。"
ARCH=$(detect_arch)
LOCAL_PACKAGE_PATH="${LOCAL_PACKAGE:-}"
if [[ -n "$LOCAL_PACKAGE_PATH" ]]; then
  [[ -f "$LOCAL_PACKAGE_PATH" ]] || err "指定的本地 Lite 构建包不存在：$LOCAL_PACKAGE_PATH"
  base_name=$(basename "$LOCAL_PACKAGE_PATH")
  if [[ "$base_name" =~ ^clawpanel-lite-core-v([0-9][0-9A-Za-z._-]*)-darwin-${ARCH}\.tar\.gz$ ]]; then
    VERSION="${BASH_REMATCH[1]}"
  fi
else
  choose_download_source
  VERSION=${VERSION:-$( [[ "$DOWNLOAD_SOURCE" == github ]] && get_latest_version_from_github || get_latest_version_from_accel )}
  VERSION=${VERSION:-$( [[ "$DOWNLOAD_SOURCE" == github ]] && get_latest_version_from_accel || get_latest_version_from_github )}
fi
if [[ -z "${VERSION:-}" ]]; then
  err "无法获取最新版本号。请检查网络连接，或通过 VERSION=x.y.z 环境变量手动指定版本后重试。"
fi
PACKAGE_NAME="clawpanel-lite-core-v${VERSION}-darwin-${ARCH}.tar.gz"

print_banner

PRIMARY_URL="${ACCEL_BASE}/lite/files/${PACKAGE_NAME}"
SECONDARY_URL="https://github.com/${REPO}/releases/download/${TAG_PREFIX}${VERSION}/${PACKAGE_NAME}"
if [[ "$DOWNLOAD_SOURCE" == github ]]; then
  PRIMARY_URL="$SECONDARY_URL"
  SECONDARY_URL="${ACCEL_BASE}/lite/files/${PACKAGE_NAME}"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

[[ $(id -u) -eq 0 ]] || err "请使用 sudo 运行 macOS Lite 安装脚本。"

TOTAL_STEPS=4

info "安装目录: ${INSTALL_DIR}"
info "数据目录: ${DATA_DIR}"
info "服务标签: ${SERVICE_LABEL}"
info "目标架构: darwin/${ARCH}"
echo ""

if lsof -iTCP:19527 -sTCP:LISTEN -n -P >/dev/null 2>&1; then
  warn "检测到 19527 端口已被占用，可能已经安装并运行了旧的 ClawPanel Pro/Lite。"
  warn "请先卸载旧面板，或停止现有服务后再安装 Lite。"
  lsof -iTCP:19527 -sTCP:LISTEN -n -P || true
  exit 1
fi

step 1 $TOTAL_STEPS "下载 ClawPanel Lite v${VERSION}"
if [[ -n "$LOCAL_PACKAGE_PATH" ]]; then
  cp -f "$LOCAL_PACKAGE_PATH" "$TMP_DIR/$PACKAGE_NAME"
  info "已使用当前目录中的本地 Lite 构建包进行安装。"
elif [[ "$DOWNLOAD_SOURCE" == github ]]; then
  info "已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到加速服务器。"
  download_file "$PRIMARY_URL" "$TMP_DIR/$PACKAGE_NAME" || download_file "$SECONDARY_URL" "$TMP_DIR/$PACKAGE_NAME" || err "下载失败"
else
  info "已选择加速服务器（中国大陆服务器推荐），失败时自动回退到 GitHub。"
  download_file "$PRIMARY_URL" "$TMP_DIR/$PACKAGE_NAME" || download_file "$SECONDARY_URL" "$TMP_DIR/$PACKAGE_NAME" || err "下载失败"
fi
log "下载完成：$PACKAGE_NAME"

step 2 $TOTAL_STEPS "部署 Lite 运行环境"
mkdir -p "$INSTALL_DIR"
mkdir -p "$DATA_DIR"

LEGACY_DATA_DIR="$INSTALL_DIR/data"
if [[ -d "$LEGACY_DATA_DIR" ]]; then
  if [[ ! -e "$DATA_DIR/clawpanel.json" ]] && [[ ! -e "$DATA_DIR/openclaw-config/openclaw.json" ]]; then
    info "检测到旧版内置数据目录，迁移到外部持久化目录..."
    cp -a "$LEGACY_DATA_DIR/." "$DATA_DIR/"
  else
    info "检测到旧版内置数据目录，但外部数据目录已有内容，跳过自动覆盖。"
  fi
fi

rm -rf "$INSTALL_DIR"/*
tar -xzf "$TMP_DIR/$PACKAGE_NAME" -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/clawpanel-lite" "$INSTALL_DIR/bin/clawlite-openclaw"
if [[ -f "$INSTALL_DIR/runtime/node/bin/node" ]]; then
  chmod +x "$INSTALL_DIR/runtime/node/bin/node"
fi
if [[ -f "$INSTALL_DIR/runtime/node/node" ]]; then
  chmod +x "$INSTALL_DIR/runtime/node/node"
fi
rm -rf "$INSTALL_DIR/data"
ln -sfn "$DATA_DIR" "$INSTALL_DIR/data"
ln -sf "$INSTALL_DIR/clawpanel-lite" /usr/local/bin/clawpanel-lite
ln -sf "$INSTALL_DIR/bin/clawlite-openclaw" /usr/local/bin/clawlite-openclaw
log "Lite 文件已部署到 ${INSTALL_DIR}"

mkdir -p /Library/LaunchDaemons

step 3 $TOTAL_STEPS "注册 launchd 服务"
cat > "/Library/LaunchDaemons/${SERVICE_LABEL}.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>${SERVICE_LABEL}</string>
  <key>ProgramArguments</key>
  <array><string>${INSTALL_DIR}/clawpanel-lite</string></array>
  <key>WorkingDirectory</key><string>${INSTALL_DIR}</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>EnvironmentVariables</key>
  <dict>
    <key>CLAWPANEL_EDITION</key><string>lite</string>
    <key>CLAWPANEL_DATA</key><string>${DATA_DIR}</string>
    <key>NODE_OPTIONS</key><string>--max-old-space-size=2048</string>
  </dict>
</dict>
</plist>
EOF

launchctl bootout system "/Library/LaunchDaemons/${SERVICE_LABEL}.plist" >/dev/null 2>&1 || true
launchctl bootstrap system "/Library/LaunchDaemons/${SERVICE_LABEL}.plist"
launchctl kickstart -k "system/${SERVICE_LABEL}"

step 4 $TOTAL_STEPS "启动 ClawPanel Lite"
sleep 2

HOST_IP=$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || echo "localhost")

log "ClawPanel Lite macOS 安装完成"
echo ""
printf "${GREEN}•${NC} 当前版本: ${BOLD}%s${NC}\n" "$VERSION"
printf "${GREEN}•${NC} 安装目录: ${BOLD}%s${NC}\n" "$INSTALL_DIR"
printf "${GREEN}•${NC} 数据目录: ${BOLD}%s${NC}\n" "$DATA_DIR"
printf "${GREEN}•${NC} 服务标签: ${BOLD}%s${NC}\n" "$SERVICE_LABEL"
printf "${GREEN}•${NC} Lite CLI: ${BOLD}clawlite-openclaw${NC}\n"
printf "${GREEN}•${NC} 面板地址: ${BOLD}http://%s:19527${NC}\n" "$HOST_IP"
echo ""
warn "默认管理员令牌请查看你的现有 ClawPanel 配置；如果是首次独立部署，后续我可以继续帮你把 macOS Lite 安装文案再收得更完整。"
