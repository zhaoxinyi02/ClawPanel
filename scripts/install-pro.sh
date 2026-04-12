#!/bin/bash
set -euo pipefail

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"
ACCEL_RAW_BASE="${ACCEL_RAW_BASE:-${CLAWPANEL_PUBLIC_BASE}}"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main"
TMP_SCRIPT=$(mktemp)
trap 'rm -f "$TMP_SCRIPT"' EXIT

fetch_script() {
  local url=$1
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 8 --max-time 30 -fsSL "$url" -o "$TMP_SCRIPT"
  elif command -v wget >/dev/null 2>&1; then
    wget -T 30 -qO "$TMP_SCRIPT" "$url"
  else
    return 1
  fi
}

if fetch_script "${ACCEL_RAW_BASE}/scripts/install.sh"; then
  : # ok
elif fetch_script "${GITHUB_RAW_BASE}/scripts/install.sh"; then
  : # fallback ok
else
  echo "缺少 curl/wget 或网络不通，无法下载 Pro 安装脚本" >&2
  exit 1
fi

chmod +x "$TMP_SCRIPT"
UPDATE_META=update-pro.json bash "$TMP_SCRIPT" "$@"
