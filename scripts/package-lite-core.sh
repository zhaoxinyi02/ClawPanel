#!/bin/bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=${1:-${VERSION:-0.1.16}}
TARGET_OS=${TARGET_OS:-linux}
TARGET_ARCH=${TARGET_ARCH:-amd64}
NODE_VERSION=${NODE_VERSION:-22.22.1}
OPENCLAW_VERSION=${OPENCLAW_VERSION:-2026.4.8}
NPM_REGISTRY=${NPM_REGISTRY:-https://registry.npmmirror.com}
OUTPUT_DIR=${OUTPUT_DIR:-"$ROOT_DIR/release/lite/v$VERSION"}
LITE_BINARY=${LITE_BINARY:-}
NODE_CACHE_DIR=${NODE_CACHE_DIR:-"$ROOT_DIR/release/runtime-cache/node/v${NODE_VERSION}"}
OPENCLAW_CACHE_DIR=${OPENCLAW_CACHE_DIR:-"$ROOT_DIR/release/runtime-cache/openclaw/${OPENCLAW_VERSION}"}
STAGE_DIR=$(mktemp -d)

cleanup() {
  rm -rf "$STAGE_DIR"
}
trap cleanup EXIT

NODE_BIN=${NODE_BIN:-}
OPENCLAW_SRC=${OPENCLAW_SRC:-}
PLUGIN_ROOT=${PLUGIN_ROOT:-"$ROOT_DIR/lite-assets/plugins"}

case "$TARGET_OS" in
  linux|darwin|windows) ;;
  *) echo "不支持的 TARGET_OS: $TARGET_OS" >&2; exit 1 ;;
esac

copy_tree() {
  local src="$1"
  local dst="$2"
  mkdir -p "$dst"
  cp -a "$src/." "$dst/"
}

prune_node_runtime() {
  local root="$1"
  rm -rf "$root/node_modules" "$root/include" "$root/share"
  rm -f "$root"/npm "$root"/npm.cmd "$root"/npm.ps1 "$root"/npx "$root"/npx.cmd "$root"/npx.ps1
  rm -f "$root"/corepack "$root"/corepack.cmd "$root"/nodevars.bat "$root"/install_tools.bat
  rm -f "$root"/README.md "$root"/CHANGELOG.md "$root"/LICENSE
}

prune_openclaw_runtime() {
  local root="$1"
  # 保留 docs/reference/templates（包含 AGENTS.md 等 agent 必需模板），删除其余文档目录
  if [[ -d "$root/docs" ]]; then
    find "$root/docs" -mindepth 1 -maxdepth 1 -type d | while read -r d; do
      [[ "$(basename "$d")" == "reference" ]] && continue
      rm -rf "$d"
    done
    find "$root/docs" -mindepth 1 -maxdepth 1 -type f -delete
    # 如果 reference 下有非 templates 的内容也删掉
    if [[ -d "$root/docs/reference" ]]; then
      find "$root/docs/reference" -mindepth 1 -maxdepth 1 | while read -r d; do
        [[ "$(basename "$d")" == "templates" ]] && continue
        rm -rf "$d"
      done
    fi
  fi
  rm -f "$root/README.md" "$root/CHANGELOG.md" "$root/LICENSE"
}

ensure_openclaw_runtime_ready() {
  local root="$1"
  local node_bin="$2"
  if [[ ! -f "$root/package.json" ]]; then
    echo "OpenClaw runtime 缺少 package.json: $root" >&2
    exit 1
  fi
  if ! (cd "$root" && "$node_bin" -e 'import("./dist/entry.js").then(()=>process.exit(0)).catch(()=>process.exit(1))' >/dev/null 2>&1); then
    echo "==> 补装 OpenClaw runtime 依赖" >&2
    rm -rf "$root/node_modules" "$root/package-lock.json"
    (cd "$root" && npm install --omit=dev --no-package-lock --registry="$NPM_REGISTRY" >/dev/null) || \
      (cd "$root" && npm install --omit=dev --no-package-lock --registry="https://registry.npmjs.org" >/dev/null)
  fi
  if ! (cd "$root" && "$node_bin" -e 'import("./dist/entry.js").then(()=>process.exit(0)).catch((err)=>{console.error(err&&err.stack||err);process.exit(1)})'); then
    echo "OpenClaw runtime 校验失败: dist/entry.js 无法导入" >&2
    exit 1
  fi
}

resolve_node_binary_path() {
  local base="$1"
  local candidate
  for candidate in \
    "$base/node.exe" \
    "$base/bin/node" \
    "$base/node"; do
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  while IFS= read -r candidate; do
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done < <(find "$base" -type f \( -name node -o -name node.exe \))
  return 1
}

resolve_node_root() {
  local node_bin="$1"
  local dir
  dir=$(dirname "$node_bin")
  if [[ "$(basename "$node_bin")" == "node.exe" && "$(basename "$dir")" != "bin" ]]; then
    printf '%s\n' "$dir"
    return 0
  fi
  printf '%s\n' "$(dirname "$dir")"
}

install_npm_package() {
  local prefix="$1"
  local package_spec="$2"
  local primary_registry="${3:-$NPM_REGISTRY}"
  local fallback_registry="${4:-https://registry.npmjs.org}"

  if npm install --omit=dev --no-package-lock --registry="$primary_registry" --prefix "$prefix" "$package_spec" >/dev/null; then
    return 0
  fi

  echo "==> npm 镜像源安装失败，回退官方 npm registry: $package_spec" >&2
  npm install --omit=dev --no-package-lock --registry="$fallback_registry" --prefix "$prefix" "$package_spec" >/dev/null
}

download_node_runtime() {
  local os="$1"
  local arch="$2"
  local node_stage="$STAGE_DIR/.node-download"
  mkdir -p "$node_stage" "$NODE_CACHE_DIR"
  local archive_name=""
  case "$os/$arch" in
    linux/amd64) archive_name="node-v${NODE_VERSION}-linux-x64.tar.gz" ;;
    linux/arm64) archive_name="node-v${NODE_VERSION}-linux-arm64.tar.gz" ;;
    darwin/amd64) archive_name="node-v${NODE_VERSION}-darwin-x64.tar.gz" ;;
    darwin/arm64) archive_name="node-v${NODE_VERSION}-darwin-arm64.tar.gz" ;;
    windows/amd64) archive_name="node-v${NODE_VERSION}-win-x64.zip" ;;
    *) echo "不支持的 Node 目标平台: $os/$arch" >&2; exit 1 ;;
  esac
  local archive_path="$node_stage/$archive_name"
  local cached_archive="$NODE_CACHE_DIR/$archive_name"
  local cache_extract_dir="$NODE_CACHE_DIR/${archive_name%.*}.extract"
  echo "==> 下载 Node runtime: $archive_name" >&2
  local urls=(
    "https://nodejs.org/dist/v${NODE_VERSION}/${archive_name}"
    "https://npmmirror.com/mirrors/node/v${NODE_VERSION}/${archive_name}"
  )
  local ok=""
  if [[ -f "$cached_archive" ]]; then
    cp -f "$cached_archive" "$archive_path"
    ok=1
  else
    rm -f "$archive_path"
    for url in "${urls[@]}"; do
      if curl --http1.1 --connect-timeout 15 --max-time 600 --retry 2 --retry-delay 2 -fL "$url" -o "$archive_path"; then
        cp -f "$archive_path" "$cached_archive"
        ok=1
        break
      fi
      rm -f "$archive_path"
    done
  fi
  if [[ -z "$ok" ]]; then
    echo "下载 Node runtime 失败: $archive_name" >&2
    exit 1
  fi
  if [[ -d "$cache_extract_dir" ]]; then
    copy_tree "$cache_extract_dir" "$node_stage/extract"
  else
    rm -rf "$cache_extract_dir"
    mkdir -p "$cache_extract_dir"
    if [[ "$archive_name" == *.zip ]]; then
      python3 - <<'PY' "$archive_path" "$cache_extract_dir"
import pathlib, sys, zipfile
archive = pathlib.Path(sys.argv[1])
dest = pathlib.Path(sys.argv[2])
dest.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(archive) as zf:
    zf.extractall(dest)
PY
    else
      tar -xzf "$archive_path" -C "$cache_extract_dir"
    fi
  fi
  local resolved=""
  resolved=$(resolve_node_binary_path "$cache_extract_dir" || true)
  if [[ -z "$resolved" ]]; then
    echo "未找到解压后的 Node 二进制: $archive_name" >&2
    exit 1
  fi
  prune_node_runtime "$(resolve_node_root "$resolved")"
  printf '%s\n' "$resolved"
}

prepare_openclaw_runtime() {
  local node_for_npm="$1"
  local openclaw_stage="$STAGE_DIR/.openclaw-stage"
  rm -rf "$openclaw_stage"
  mkdir -p "$openclaw_stage"
  mkdir -p "$OPENCLAW_CACHE_DIR"
  if [[ -d "$OPENCLAW_CACHE_DIR/openclaw" && -f "$OPENCLAW_CACHE_DIR/openclaw/package.json" ]]; then
    copy_tree "$OPENCLAW_CACHE_DIR/openclaw" "$openclaw_stage/openclaw"
    ensure_openclaw_runtime_ready "$openclaw_stage/openclaw" "$node_for_npm"
    return
  fi
  if [[ -n "$OPENCLAW_SRC" && -f "$OPENCLAW_SRC/package.json" ]]; then
    cp -a "$OPENCLAW_SRC" "$openclaw_stage/openclaw"
    ensure_openclaw_runtime_ready "$openclaw_stage/openclaw" "$node_for_npm"
    rm -rf "$OPENCLAW_CACHE_DIR/openclaw"
    cp -a "$openclaw_stage/openclaw" "$OPENCLAW_CACHE_DIR/openclaw"
    return
  fi
  echo "==> 安装 OpenClaw runtime: ${OPENCLAW_VERSION}"
  mkdir -p "$openclaw_stage/openclaw"
  install_npm_package "$openclaw_stage/openclaw" "openclaw@${OPENCLAW_VERSION}" "$NPM_REGISTRY" "https://registry.npmjs.org"
  if [[ -d "$openclaw_stage/openclaw/node_modules/openclaw" ]]; then
    tmp_extract="$openclaw_stage/.openclaw-pkg"
    rm -rf "$tmp_extract"
    mv "$openclaw_stage/openclaw/node_modules/openclaw" "$tmp_extract"
    mkdir -p "$tmp_extract/node_modules"
    while IFS= read -r dep; do
      dep_name=$(basename "$dep")
      if [[ -e "$tmp_extract/node_modules/$dep_name" ]]; then
        : # openclaw already ships its own version of this dep — keep it
      else
        mv "$dep" "$tmp_extract/node_modules/"
      fi
    done < <(find "$openclaw_stage/openclaw/node_modules" -mindepth 1 -maxdepth 1)
    rm -rf "$openclaw_stage/openclaw"
    mv "$tmp_extract" "$openclaw_stage/openclaw"
  fi
  ensure_openclaw_runtime_ready "$openclaw_stage/openclaw" "$node_for_npm"
  rm -rf "$OPENCLAW_CACHE_DIR/openclaw"
  cp -a "$openclaw_stage/openclaw" "$OPENCLAW_CACHE_DIR/openclaw"
}

if [[ -z "$OPENCLAW_SRC" ]]; then
  for candidate in \
    "/usr/lib/node_modules/openclaw" \
    "/usr/local/lib/node_modules/openclaw" \
    "$HOME/.npm-global/lib/node_modules/openclaw" \
    "$ROOT_DIR/release/runtime-cache/openclaw/${OPENCLAW_VERSION}/openclaw"; do
    if [[ -f "$candidate/package.json" ]]; then
      OPENCLAW_SRC="$candidate"
      break
    fi
  done
fi

if [[ -z "$NODE_BIN" || ! -f "$NODE_BIN" ]]; then
  NODE_BIN=$(download_node_runtime "$TARGET_OS" "$TARGET_ARCH")
fi

if [[ -z "$NODE_BIN" || ! -f "$NODE_BIN" ]]; then
  echo "未找到 Node 运行时，可通过 NODE_BIN=/path/to/node 指定" >&2
  exit 1
fi

prepare_openclaw_runtime "$NODE_BIN"
OPENCLAW_SRC="$STAGE_DIR/.openclaw-stage/openclaw"

if [[ ! -f "$OPENCLAW_SRC/package.json" ]]; then
  echo "未能准备 OpenClaw runtime，可通过 OPENCLAW_SRC=/path/to/openclaw 指定" >&2
  exit 1
fi
prune_openclaw_runtime "$OPENCLAW_SRC"

APP_BINARY_NAME="clawpanel-lite"
NODE_TARGET_REL="runtime/node/bin/node"
LAUNCHER_SRC="$ROOT_DIR/scripts/clawlite-openclaw.sh"
LAUNCHER_NAME="clawlite-openclaw"
ARCHIVE_EXT="tar.gz"
DEFAULT_BINARY_SOURCE="$ROOT_DIR/bin/clawpanel-lite"

if [[ "$TARGET_OS" == "windows" ]]; then
  APP_BINARY_NAME="clawpanel-lite.exe"
  NODE_TARGET_REL="runtime/node/node.exe"
  LAUNCHER_SRC="$ROOT_DIR/scripts/clawlite-openclaw.cmd"
  LAUNCHER_NAME="clawlite-openclaw.cmd"
  DEFAULT_BINARY_SOURCE="$ROOT_DIR/bin/clawpanel-lite.exe"
fi

if [[ -z "$LITE_BINARY" ]]; then
  LITE_BINARY="$DEFAULT_BINARY_SOURCE"
fi
if [[ ! -f "$LITE_BINARY" ]]; then
  echo "未找到 Lite 二进制，可通过 LITE_BINARY=/path/to/file 指定" >&2
  exit 1
fi

echo "==> 打包 Lite Core v$VERSION"
echo "    Target:    $TARGET_OS/$TARGET_ARCH"
echo "    Node:      $NODE_BIN"
echo "    OpenClaw:  $OPENCLAW_SRC"
echo "    Plugins:   $PLUGIN_ROOT"

mkdir -p "$OUTPUT_DIR"
mkdir -p "$STAGE_DIR/runtime" "$STAGE_DIR/data/openclaw-config" "$STAGE_DIR/data/openclaw-work" "$STAGE_DIR/bin" "$STAGE_DIR/.plugin-build"

cp "$LITE_BINARY" "$STAGE_DIR/$APP_BINARY_NAME"
cp "$LAUNCHER_SRC" "$STAGE_DIR/bin/$LAUNCHER_NAME"
if [[ "$TARGET_OS" != "windows" ]]; then
  chmod +x "$STAGE_DIR/$APP_BINARY_NAME" "$STAGE_DIR/bin/$LAUNCHER_NAME"
fi

mkdir -p "$STAGE_DIR/$(dirname "$NODE_TARGET_REL")"
cp -a "$NODE_BIN" "$STAGE_DIR/$NODE_TARGET_REL"
if [[ "$TARGET_OS" != "windows" ]]; then
  chmod +x "$STAGE_DIR/$NODE_TARGET_REL"
fi
cp -a "$OPENCLAW_SRC" "$STAGE_DIR/runtime/openclaw"

if [[ -d "$PLUGIN_ROOT" ]]; then
  mkdir -p "$STAGE_DIR/runtime/openclaw/extensions"
  for plugin_id in qq qqbot dingtalk wecom wecom-app; do
    if [[ -d "$PLUGIN_ROOT/$plugin_id" ]]; then
      plugin_build_dir="$STAGE_DIR/.plugin-build/$plugin_id"
      rm -rf "$plugin_build_dir" "$STAGE_DIR/runtime/openclaw/extensions/$plugin_id"
      cp -a "$PLUGIN_ROOT/$plugin_id" "$plugin_build_dir"
      if [[ -f "$plugin_build_dir/package.json" && "$plugin_id" != "wecom-app" ]]; then
        echo "==> 安装 Lite 插件依赖: $plugin_id"
        rm -rf "$plugin_build_dir/node_modules" "$plugin_build_dir/package-lock.json"
        (cd "$plugin_build_dir" && npm install --omit=dev --omit=peer --no-package-lock --registry=https://registry.npmmirror.com >/dev/null)
      fi
      cp -a "$plugin_build_dir" "$STAGE_DIR/runtime/openclaw/extensions/$plugin_id"
    fi
  done
fi

rm -rf "$STAGE_DIR/.plugin-build"

cat > "$STAGE_DIR/data/openclaw-config/openclaw.json" <<'EOF'
{
  "gateway": {
    "mode": "local",
    "port": 18790
  },
  "plugins": {
    "slots": {
      "memory": "none"
    }
  }
}
EOF

PACKAGE_NAME="clawpanel-lite-core-v$VERSION-$TARGET_OS-$TARGET_ARCH.$ARCHIVE_EXT"
if [[ "$TARGET_OS" == "windows" ]]; then
  python3 - <<'PY' "$STAGE_DIR" "$OUTPUT_DIR/$PACKAGE_NAME"
import pathlib, tarfile, sys
src = pathlib.Path(sys.argv[1])
dest = pathlib.Path(sys.argv[2])
with tarfile.open(dest, 'w:gz') as tf:
    for path in src.rglob('*'):
        tf.add(path, arcname=path.relative_to(src))
PY
else
  tar -C "$STAGE_DIR" -czf "$OUTPUT_DIR/$PACKAGE_NAME" .
fi

sha256sum "$OUTPUT_DIR/$PACKAGE_NAME" > "$OUTPUT_DIR/checksums.txt"

echo "==> Lite Core 打包完成"
echo "    $OUTPUT_DIR/$PACKAGE_NAME"
