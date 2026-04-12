#!/bin/bash
set -euo pipefail

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"
ACCEL_BASE="${ACCEL_BASE:-${CLAWPANEL_PUBLIC_BASE}/api/panel/update-mirror}"
VERSION=${VERSION:-0.1.4}
BUNDLE_PATH=${1:-}
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ -z "$BUNDLE_PATH" ]]; then
  BUNDLE_PATH="$TMP_DIR/clawpanel-lite-qq-bundle-v${VERSION}-linux-amd64.tar.gz"
  if command -v curl >/dev/null 2>&1; then
    curl -fSL "$ACCEL_BASE/lite/files/clawpanel-lite-qq-bundle-v${VERSION}-linux-amd64.tar.gz" -o "$BUNDLE_PATH"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$BUNDLE_PATH" "$ACCEL_BASE/lite/files/clawpanel-lite-qq-bundle-v${VERSION}-linux-amd64.tar.gz"
  else
    echo "缺少 curl/wget，无法下载 Lite QQ Bundle" >&2
    exit 1
  fi
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "当前机器未安装 Docker，无法导入 Lite QQ Bundle" >&2
  exit 1
fi

gzip -dc "$BUNDLE_PATH" | docker load
echo "Lite QQ Bundle 导入完成"
