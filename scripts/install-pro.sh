#!/bin/bash
set -euo pipefail

GITEE_RAW_BASE="https://gitee.com/zxy000006/ClawPanel/raw/main"
TMP_SCRIPT=$(mktemp)
trap 'rm -f "$TMP_SCRIPT"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$GITEE_RAW_BASE/scripts/install.sh" -o "$TMP_SCRIPT"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$TMP_SCRIPT" "$GITEE_RAW_BASE/scripts/install.sh"
else
  echo "缺少 curl/wget，无法下载 Pro 安装脚本" >&2
  exit 1
fi

chmod +x "$TMP_SCRIPT"
UPDATE_META=update-pro.json bash "$TMP_SCRIPT" "$@"
