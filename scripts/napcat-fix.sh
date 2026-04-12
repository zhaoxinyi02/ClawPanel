#!/usr/bin/env bash
#
# ClawPanel NapCat 一键修复补丁
# 适用场景：Linux 服务器先部署了 OpenClaw，再安装 ClawPanel + NapCat，出现各种配置不一致问题
# 用法：sudo bash napcat-fix.sh
#
# 检测并修复：
#   1. Docker 安装与服务状态
#   2. openclaw-qq 容器存在性与运行状态
#   3. 容器端口映射 (6099/3001/3000)
#   4. NapCat webui.json 存在性与 Token 一致性
#   5. NapCat onebot11.json WebSocket/HTTP 配置
#   6. Docker WEBUI_TOKEN 环境变量与 webui.json Token 对齐
#   7. OpenClaw openclaw.json 中 gateway.mode / channels.qq / plugins.qq 配置
#   8. QQ 插件 (extensions/qq) 安装完整性
#   9. Node.js / npm / openclaw CLI 可用性
#  10. ClawPanel systemd 服务状态
#

set -euo pipefail

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"

# ─── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

FIXED=0
ERRORS=0
WARNINGS=0
CHECKS=0

ok()      { CHECKS=$((CHECKS+1)); echo -e "  ${GREEN}✅${NC} $1"; }
fixed()   { CHECKS=$((CHECKS+1)); FIXED=$((FIXED+1));  echo -e "  ${CYAN}🔧${NC} $1"; }
warn()    { CHECKS=$((CHECKS+1)); WARNINGS=$((WARNINGS+1)); echo -e "  ${YELLOW}⚠️${NC}  $1"; }
fail()    { CHECKS=$((CHECKS+1)); ERRORS=$((ERRORS+1)); echo -e "  ${RED}❌${NC} $1"; }
section() { echo -e "\n${BOLD}${BLUE}[$1]${NC}"; }

CONTAINER_NAME="openclaw-qq"
DEFAULT_TOKEN="clawpanel-qq"
OPENCLAW_DIR="${OPENCLAW_DIR:-$HOME/.openclaw}"
QQ_PLUGIN_URL="${QQ_PLUGIN_URL:-${CLAWPANEL_PUBLIC_BASE}/bin/qq-plugin.tgz}"
QQ_PLUGIN_ARCHIVE_URL="${QQ_PLUGIN_ARCHIVE_URL:-https://github.com/zhaoxinyi02/ClawPanel-Plugins/archive/refs/heads/main.tar.gz}"
NEED_RESTART_CONTAINER=false
NEED_RESTART_CLAWPANEL=false

echo -e "${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║       ClawPanel NapCat 一键修复补丁 v1.0                    ║${NC}"
echo -e "${BOLD}║       适用于先部署 OpenClaw 后安装 ClawPanel 的 Linux 环境  ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
echo -e "时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo -e "用户: $(whoami)"
echo -e "OpenClaw 配置目录: ${OPENCLAW_DIR}"

# ─── 0. Root check ─────────────────────────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}请使用 sudo 执行此脚本${NC}"
    exit 1
fi

# ─── 1. Docker ─────────────────────────────────────────────────────────────────
section "1. Docker 环境"

if command -v docker &>/dev/null; then
    ok "Docker 已安装: $(docker --version 2>/dev/null | head -1)"
else
    fail "Docker 未安装"
    echo -e "     请先安装 Docker: apt-get install docker-ce 或通过 ClawPanel 面板安装"
    echo -e "\n${RED}Docker 是 NapCat 运行的前置依赖，无法继续修复${NC}"
    exit 1
fi

if docker info &>/dev/null; then
    ok "Docker 服务正常运行"
else
    warn "Docker 服务未运行，正在启动..."
    systemctl start docker 2>/dev/null || true
    sleep 2
    if docker info &>/dev/null; then
        fixed "Docker 服务已启动"
    else
        fail "Docker 服务启动失败"
        exit 1
    fi
fi

# ─── 2. Container ──────────────────────────────────────────────────────────────
section "2. NapCat 容器 (${CONTAINER_NAME})"

CONTAINER_STATUS=$(docker inspect --format '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "")
if [ -z "$CONTAINER_STATUS" ]; then
    fail "容器 ${CONTAINER_NAME} 不存在"
    echo -e "     请通过 ClawPanel 面板 → 通道管理 → 安装 NapCat"
    echo -e "     或手动运行："
    echo -e "     docker run -d --name ${CONTAINER_NAME} --restart unless-stopped \\"
    echo -e "       -p 3000:3000 -p 3001:3001 -p 6099:6099 \\"
    echo -e "       -e NAPCAT_GID=0 -e NAPCAT_UID=0 -e WEBUI_TOKEN=${DEFAULT_TOKEN} \\"
    echo -e "       -v napcat-qq-session:/app/.config/QQ \\"
    echo -e "       -v napcat-config:/app/napcat/config \\"
    echo -e "       -v ${OPENCLAW_DIR}:/root/.openclaw:rw \\"
    echo -e "       mlikiowa/napcat-docker:latest"
    # Cannot continue without container
    echo -e "\n${RED}容器不存在，部分检查跳过。请先安装 NapCat。${NC}"
    # Still continue with OpenClaw checks below
    CONTAINER_STATUS="missing"
fi

if [ "$CONTAINER_STATUS" = "running" ]; then
    ok "容器正在运行"
elif [ "$CONTAINER_STATUS" = "missing" ]; then
    : # already reported
else
    warn "容器状态: ${CONTAINER_STATUS}，正在启动..."
    docker start "$CONTAINER_NAME" 2>/dev/null || true
    sleep 5
    NEW_STATUS=$(docker inspect --format '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "")
    if [ "$NEW_STATUS" = "running" ]; then
        fixed "容器已启动"
    else
        fail "容器启动失败 (状态: ${NEW_STATUS})"
        echo -e "     查看日志: docker logs ${CONTAINER_NAME}"
    fi
fi

# ─── 3. Port mappings ──────────────────────────────────────────────────────────
if [ "$CONTAINER_STATUS" != "missing" ]; then
    section "3. 端口映射"

    PORT_INFO=$(docker port "$CONTAINER_NAME" 2>/dev/null || echo "")
    for PORT in 6099 3001 3000; do
        if echo "$PORT_INFO" | grep -q "$PORT"; then
            ok "端口 ${PORT} 已映射"
        else
            fail "端口 ${PORT} 未映射"
            echo -e "     需要重新创建容器以修复端口映射"
        fi
    done

    # Check port reachability
    for PORT in 6099 3001 3000; do
        if bash -c "echo > /dev/tcp/127.0.0.1/${PORT}" 2>/dev/null; then
            ok "端口 ${PORT} 可达"
        else
            warn "端口 ${PORT} 不可达（可能需要先登录 QQ）"
        fi
    done
fi

# ─── 4. webui.json Token ──────────────────────────────────────────────────────
if [ "$CONTAINER_STATUS" != "missing" ]; then
    section "4. NapCat WebUI Token"

    WEBUI_JSON=$(docker exec "$CONTAINER_NAME" cat /app/napcat/config/webui.json 2>/dev/null || echo "")
    if [ -z "$WEBUI_JSON" ]; then
        warn "webui.json 不存在，正在创建..."
        docker exec "$CONTAINER_NAME" bash -c "cat > /app/napcat/config/webui.json << 'EOF'
{
  \"host\": \"0.0.0.0\",
  \"port\": 6099,
  \"token\": \"${DEFAULT_TOKEN}\",
  \"loginRate\": 3
}
EOF"
        fixed "已创建 webui.json (token=${DEFAULT_TOKEN})"
        NEED_RESTART_CONTAINER=true
    else
        WEBUI_TOKEN=$(echo "$WEBUI_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")
        if [ -z "$WEBUI_TOKEN" ]; then
            warn "webui.json Token 为空，正在设置..."
            docker exec "$CONTAINER_NAME" bash -c "cat > /app/napcat/config/webui.json << 'EOF'
{
  \"host\": \"0.0.0.0\",
  \"port\": 6099,
  \"token\": \"${DEFAULT_TOKEN}\",
  \"loginRate\": 3
}
EOF"
            fixed "已设置 webui.json Token=${DEFAULT_TOKEN}"
            NEED_RESTART_CONTAINER=true
        else
            ok "webui.json Token 已配置: ${WEBUI_TOKEN}"
        fi
    fi

    # Check Docker WEBUI_TOKEN env vs webui.json
    DOCKER_ENV_TOKEN=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CONTAINER_NAME" 2>/dev/null | grep '^WEBUI_TOKEN=' | sed 's/^WEBUI_TOKEN=//' || echo "")
    WEBUI_JSON_TOKEN=$(docker exec "$CONTAINER_NAME" cat /app/napcat/config/webui.json 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")

    if [ -n "$DOCKER_ENV_TOKEN" ] && [ -n "$WEBUI_JSON_TOKEN" ]; then
        if [ "$DOCKER_ENV_TOKEN" != "$WEBUI_JSON_TOKEN" ]; then
            warn "Token 不一致: webui.json=\"${WEBUI_JSON_TOKEN}\" vs Docker WEBUI_TOKEN=\"${DOCKER_ENV_TOKEN}\""
            echo -e "     这是导致扫码登录 Unauthorized 的主要原因！"
            echo -e "     正在将 webui.json 同步为 Docker 环境变量值..."
            docker exec "$CONTAINER_NAME" bash -c "cat > /app/napcat/config/webui.json << 'EOF'
{
  \"host\": \"0.0.0.0\",
  \"port\": 6099,
  \"token\": \"${DOCKER_ENV_TOKEN}\",
  \"loginRate\": 3
}
EOF"
            fixed "已同步 webui.json Token → \"${DOCKER_ENV_TOKEN}\""
            NEED_RESTART_CONTAINER=true
        else
            ok "Docker WEBUI_TOKEN 与 webui.json Token 一致"
        fi
    elif [ -z "$DOCKER_ENV_TOKEN" ]; then
        warn "Docker 容器未设置 WEBUI_TOKEN 环境变量"
        echo -e "     建议重新创建容器并添加 -e WEBUI_TOKEN=${DEFAULT_TOKEN}"
    fi
fi

# ─── 5. onebot11.json ──────────────────────────────────────────────────────────
if [ "$CONTAINER_STATUS" != "missing" ]; then
    section "5. OneBot11 配置"

    OB_JSON=$(docker exec "$CONTAINER_NAME" cat /app/napcat/config/onebot11.json 2>/dev/null || echo "")
    if [ -z "$OB_JSON" ]; then
        warn "onebot11.json 不存在，正在创建..."
        docker exec "$CONTAINER_NAME" bash -c 'cat > /app/napcat/config/onebot11.json << '\''EOF'\''
{
  "network": {
    "websocketServers": [{
      "name": "ws-server",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3001,
      "token": "",
      "reportSelfMessage": true,
      "enableForcePushEvent": true,
      "messagePostFormat": "array",
      "debug": false,
      "heartInterval": 30000
    }],
    "httpServers": [{
      "name": "http-api",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3000,
      "token": ""
    }],
    "httpSseServers": [],
    "httpClients": [],
    "websocketClients": [],
    "plugins": []
  },
  "musicSignUrl": "",
  "enableLocalFile2Url": true,
  "parseMultMsg": true,
  "imageDownloadProxy": ""
}
EOF'
        fixed "已创建默认 onebot11.json (WS:3001, HTTP:3000)"
        NEED_RESTART_CONTAINER=true
    else
        # Validate WS config
        WS_OK=$(echo "$OB_JSON" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    net = d.get('network', {})
    wss = net.get('websocketServers', [])
    for ws in wss:
        if ws.get('enable') and ws.get('port') == 3001:
            print('ok')
            break
    else:
        print('bad')
except:
    print('bad')
" 2>/dev/null || echo "bad")
        if [ "$WS_OK" = "ok" ]; then
            ok "WebSocket 服务配置正确 (端口 3001, 已启用)"
        else
            warn "WebSocket 服务配置异常，正在修复..."
            docker exec "$CONTAINER_NAME" bash -c 'cat > /app/napcat/config/onebot11.json << '\''EOF'\''
{
  "network": {
    "websocketServers": [{
      "name": "ws-server",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3001,
      "token": "",
      "reportSelfMessage": true,
      "enableForcePushEvent": true,
      "messagePostFormat": "array",
      "debug": false,
      "heartInterval": 30000
    }],
    "httpServers": [{
      "name": "http-api",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3000,
      "token": ""
    }],
    "httpSseServers": [],
    "httpClients": [],
    "websocketClients": [],
    "plugins": []
  },
  "musicSignUrl": "",
  "enableLocalFile2Url": true,
  "parseMultMsg": true,
  "imageDownloadProxy": ""
}
EOF'
            fixed "已重写 onebot11.json"
            NEED_RESTART_CONTAINER=true
        fi
    fi
fi

# ─── 6. Restart container if needed ───────────────────────────────────────────
if [ "$NEED_RESTART_CONTAINER" = true ] && [ "$CONTAINER_STATUS" != "missing" ]; then
    section "6. 重启 NapCat 容器"
    echo -e "  正在重启容器使配置生效..."
    docker restart "$CONTAINER_NAME" 2>/dev/null || true
    sleep 8
    NEW_STATUS=$(docker inspect --format '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "")
    if [ "$NEW_STATUS" = "running" ]; then
        fixed "容器已重启并正常运行"
    else
        fail "容器重启后状态异常: ${NEW_STATUS}"
    fi
else
    section "6. 容器重启"
    ok "无需重启容器"
fi

# ─── 7. OpenClaw config ───────────────────────────────────────────────────────
section "7. OpenClaw 配置 (openclaw.json)"

OC_CFG="${OPENCLAW_DIR}/openclaw.json"
if [ ! -f "$OC_CFG" ]; then
    warn "openclaw.json 不存在: ${OC_CFG}"
    echo -e "     请先运行 openclaw init 或通过 ClawPanel 安装 OpenClaw"
else
    ok "openclaw.json 存在"

    if command -v python3 &>/dev/null; then
        # Check and fix gateway.mode
        GW_MODE=$(python3 -c "import json; d=json.load(open('${OC_CFG}')); print(d.get('gateway',{}).get('mode',''))" 2>/dev/null || echo "")
        if [ "$GW_MODE" = "local" ]; then
            ok "gateway.mode = local"
        else
            warn "gateway.mode = \"${GW_MODE}\" (应为 local)，正在修复..."
            python3 -c "
import json
with open('${OC_CFG}') as f:
    cfg = json.load(f)
cfg.setdefault('gateway', {})['mode'] = 'local'
with open('${OC_CFG}', 'w') as f:
    json.dump(cfg, f, indent=2)
print('done')
" 2>/dev/null && fixed "gateway.mode 已设为 local" || fail "gateway.mode 修复失败"
        fi

        # Check channels.qq (QQ plugin reads wsUrl from channels.qq config)
        QQ_WS=$(python3 -c "import json; d=json.load(open('${OC_CFG}')); print(d.get('channels',{}).get('qq',{}).get('wsUrl',''))" 2>/dev/null || echo "")
        QQ_ENABLED=$(python3 -c "import json; d=json.load(open('${OC_CFG}')); print(d.get('channels',{}).get('qq',{}).get('enabled', False))" 2>/dev/null || echo "")
        if [ "$QQ_WS" = "ws://127.0.0.1:3001" ] && [ "$QQ_ENABLED" = "True" ]; then
            ok "channels.qq 配置正确 (wsUrl=ws://127.0.0.1:3001, enabled=true)"
        else
            warn "channels.qq 配置不完整 (wsUrl=\"${QQ_WS}\", enabled=${QQ_ENABLED})，正在修复..."
            python3 -c "
import json
with open('${OC_CFG}') as f:
    cfg = json.load(f)
ch = cfg.setdefault('channels', {})
qq = ch.setdefault('qq', {})
qq['enabled'] = True
if not qq.get('wsUrl'):
    qq['wsUrl'] = 'ws://127.0.0.1:3001'
with open('${OC_CFG}', 'w') as f:
    json.dump(cfg, f, indent=2)
print('done')
" 2>/dev/null && fixed "channels.qq 已修复" || fail "channels.qq 修复失败"
        fi

        # Check plugins.entries.qq
        QQ_PLUGIN=$(python3 -c "import json; d=json.load(open('${OC_CFG}')); print(d.get('plugins',{}).get('entries',{}).get('qq',{}).get('enabled', False))" 2>/dev/null || echo "")
        if [ "$QQ_PLUGIN" = "True" ]; then
            ok "plugins.entries.qq 已启用"
        else
            warn "plugins.entries.qq 未启用，正在修复..."
            python3 -c "
import json
with open('${OC_CFG}') as f:
    cfg = json.load(f)
pl = cfg.setdefault('plugins', {})
ent = pl.setdefault('entries', {})
ent.setdefault('qq', {})['enabled'] = True
with open('${OC_CFG}', 'w') as f:
    json.dump(cfg, f, indent=2)
print('done')
" 2>/dev/null && fixed "plugins.entries.qq 已启用" || fail "plugins.entries.qq 修复失败"
        fi

        # Check plugins.installs.qq
        QQ_INSTALL=$(python3 -c "import json; d=json.load(open('${OC_CFG}')); print('yes' if 'qq' in d.get('plugins',{}).get('installs',{}) else 'no')" 2>/dev/null || echo "no")
        if [ "$QQ_INSTALL" = "yes" ]; then
            ok "plugins.installs.qq 已配置"
        else
            warn "plugins.installs.qq 缺失，正在修复..."
            python3 -c "
import json
with open('${OC_CFG}') as f:
    cfg = json.load(f)
pl = cfg.setdefault('plugins', {})
ins = pl.setdefault('installs', {})
ins['qq'] = {
    'source': 'archive',
    'installPath': '${OPENCLAW_DIR}/extensions/qq',
    'version': '1.0.0'
}
with open('${OC_CFG}', 'w') as f:
    json.dump(cfg, f, indent=2)
print('done')
" 2>/dev/null && fixed "plugins.installs.qq 已修复" || fail "plugins.installs.qq 修复失败"
        fi
    else
        warn "python3 未安装，无法自动检查/修复 openclaw.json"
        echo -e "     请手动确认 gateway.mode=local、channels.qq.wsUrl、plugins.entries.qq 配置"
    fi
fi

# ─── 8. QQ Plugin (extensions/qq) ─────────────────────────────────────────────
section "8. QQ 通道插件"

QQ_EXT_DIR="${OPENCLAW_DIR}/extensions/qq"
if [ -d "$QQ_EXT_DIR" ] && [ -f "$QQ_EXT_DIR/openclaw.plugin.json" ]; then
    ok "QQ 插件已安装: ${QQ_EXT_DIR}"
    # Check key files
    for F in index.ts src package.json; do
        if [ -e "$QQ_EXT_DIR/$F" ]; then
            ok "  $F 存在"
        else
            warn "  $F 缺失"
        fi
    done
    # Check file ownership (must be root, otherwise OpenClaw rejects with 'suspicious ownership')
    PLUGIN_OWNER=$(stat -c '%u' "$QQ_EXT_DIR" 2>/dev/null || echo "unknown")
    if [ "$PLUGIN_OWNER" = "0" ]; then
        ok "  插件文件所有者正确 (root)"
    else
        warn "  插件文件所有者为 uid=${PLUGIN_OWNER}（应为 root），正在修复..."
        chown -R root:root "$QQ_EXT_DIR" 2>/dev/null
        fixed "  已修复插件文件所有者为 root"
    fi
else
    warn "QQ 插件未安装或不完整，正在下载..."
    mkdir -p "${OPENCLAW_DIR}/extensions"
    TMP_TGZ=$(mktemp)
    TMP_ARCHIVE=$(mktemp)
    TMP_EXTRACT=$(mktemp -d)
    if curl -fsSL "$QQ_PLUGIN_URL" -o "$TMP_TGZ" 2>/dev/null; then
        tar xzf "$TMP_TGZ" -C "${OPENCLAW_DIR}/extensions/" 2>/dev/null
        chown -R root:root "$QQ_EXT_DIR" 2>/dev/null
        rm -f "$TMP_TGZ"
        if [ -f "$QQ_EXT_DIR/openclaw.plugin.json" ]; then
            fixed "QQ 插件安装完成"
        else
            fail "QQ 插件下载后解压异常"
        fi
    elif curl -fsSL "$QQ_PLUGIN_ARCHIVE_URL" -o "$TMP_ARCHIVE" 2>/dev/null; then
        tar xzf "$TMP_ARCHIVE" -C "$TMP_EXTRACT" 2>/dev/null || true
        if [ -d "$TMP_EXTRACT/ClawPanel-Plugins-main/official/qq" ]; then
            cp -R "$TMP_EXTRACT/ClawPanel-Plugins-main/official/qq" "$QQ_EXT_DIR"
            chown -R root:root "$QQ_EXT_DIR" 2>/dev/null
            fixed "QQ 插件已通过 GitHub 插件仓库归档安装"
        else
            fail "QQ 插件归档解压异常"
        fi
    else
        fail "QQ 插件下载失败 (${QQ_PLUGIN_URL})"
    fi
    rm -f "$TMP_TGZ" "$TMP_ARCHIVE"
    rm -rf "$TMP_EXTRACT"
fi

# ─── 8.5 QQ Plugin channel.ts startAccount fix ──────────────────────────────
# ROOT CAUSE FIX: OpenClaw gateway expects startAccount() to return a long-lived
# Promise. The original QQ plugin returns a cleanup function, which resolves
# immediately via Promise.resolve(fn), triggering auto-restart loop (up to 10
# attempts) after which the channel handler dies and messages are never processed.
section "8.5 QQ 插件 channel.ts startAccount 修复"

CHANNEL_TS="${QQ_EXT_DIR}/src/channel.ts"
if [ -f "$CHANNEL_TS" ]; then
    # Check if the old (broken) pattern exists: returns cleanup function directly
    if grep -q 'return () => {' "$CHANNEL_TS" 2>/dev/null && grep -q 'client.disconnect' "$CHANNEL_TS" 2>/dev/null && ! grep -q 'new Promise' "$CHANNEL_TS" 2>/dev/null; then
        warn "channel.ts startAccount 返回 cleanup 函数（会导致 auto-restart 循环），正在修复..."
        if command -v python3 &>/dev/null; then
            python3 -c "
import sys
with open('${CHANNEL_TS}', 'r') as f:
    content = f.read()

old_code = '''      client.connect();
      
      return () => {
        client.disconnect();
        clients.delete(account.accountId);
        stopFileServer();
      };'''

new_code = '''      client.connect();

      // Return a Promise that stays pending until abortSignal fires.
      // OpenClaw gateway expects startAccount to return a long-lived Promise;
      // if it resolves immediately, the framework treats the account as exited
      // and triggers auto-restart attempts.
      const abortSignal = (ctx as any).abortSignal as AbortSignal | undefined;
      return new Promise<void>((resolve) => {
        const cleanup = () => {
          client.disconnect();
          clients.delete(account.accountId);
          stopFileServer();
          resolve();
        };
        if (abortSignal) {
          if (abortSignal.aborted) { cleanup(); return; }
          abortSignal.addEventListener(\"abort\", cleanup, { once: true });
        }
        // Also clean up if the WebSocket closes unexpectedly
        client.on(\"close\", () => {
          cleanup();
        });
      });'''

if old_code in content:
    content = content.replace(old_code, new_code)
    with open('${CHANNEL_TS}', 'w') as f:
        f.write(content)
    print('patched')
else:
    print('pattern_not_found')
" 2>/dev/null
            PATCH_RESULT=$?
            if [ $PATCH_RESULT -eq 0 ]; then
                fixed "channel.ts startAccount 已修复为返回 long-lived Promise"
            else
                fail "channel.ts 自动修复失败，请手动修改"
            fi
        else
            fail "需要 python3 来修复 channel.ts"
        fi
    elif grep -q 'new Promise' "$CHANNEL_TS" 2>/dev/null; then
        ok "channel.ts startAccount 已使用 Promise 模式（无需修复）"
    else
        warn "channel.ts 结构未知，无法自动检测是否需要修复"
    fi
else
    warn "channel.ts 不存在: ${CHANNEL_TS}"
fi

# ─── 9. Node.js / OpenClaw CLI ────────────────────────────────────────────────
section "9. 运行环境"

export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

if command -v node &>/dev/null; then
    NODE_VER=$(node --version 2>/dev/null || echo "unknown")
    NODE_MAJOR=$(echo "$NODE_VER" | sed 's/^v//' | cut -d. -f1)
    if [ "${NODE_MAJOR:-0}" -ge 20 ]; then
        ok "Node.js ${NODE_VER}"
    else
        warn "Node.js ${NODE_VER} 版本过低 (需要 >= 20)"
    fi
else
    warn "Node.js 未安装"
fi

if command -v npm &>/dev/null; then
    ok "npm $(npm --version 2>/dev/null)"
else
    warn "npm 未安装"
fi

if command -v openclaw &>/dev/null; then
    OC_VER=$(openclaw --version 2>/dev/null || echo "unknown")
    ok "openclaw CLI: ${OC_VER} ($(which openclaw))"
else
    warn "openclaw 命令不在 PATH 中"
    # Check common paths
    for P in /usr/local/bin/openclaw "$HOME/.npm-global/bin/openclaw" "$HOME/.local/bin/openclaw"; do
        if [ -x "$P" ]; then
            echo -e "     找到: $P"
            break
        fi
    done
fi

# ─── 10. ClawPanel service ────────────────────────────────────────────────────
section "10. ClawPanel 服务"

if systemctl is-active clawpanel &>/dev/null; then
    ok "clawpanel 服务正在运行"
    SERVICE_PID=$(systemctl show clawpanel --property=MainPID --value 2>/dev/null || echo "")
    if [ -n "$SERVICE_PID" ] && [ "$SERVICE_PID" != "0" ]; then
        ok "主进程 PID: ${SERVICE_PID}"
    fi
elif systemctl is-enabled clawpanel &>/dev/null; then
    warn "clawpanel 服务已启用但未运行，正在启动..."
    systemctl start clawpanel 2>/dev/null || true
    sleep 2
    if systemctl is-active clawpanel &>/dev/null; then
        fixed "clawpanel 服务已启动"
    else
        fail "clawpanel 服务启动失败"
        echo -e "     查看日志: journalctl -u clawpanel -n 50"
    fi
else
    warn "clawpanel systemd 服务未安装或未启用"
    # Check if running as standalone process
    if pgrep -x clawpanel &>/dev/null; then
        ok "clawpanel 进程正在运行 (非 systemd 模式)"
    else
        warn "clawpanel 进程未运行"
    fi
fi

# ─── 11. Auth test ────────────────────────────────────────────────────────────
if [ "$CONTAINER_STATUS" != "missing" ]; then
    section "11. 鉴权测试"

    # Get the effective token
    EFFECTIVE_TOKEN=$(docker exec "$CONTAINER_NAME" cat /app/napcat/config/webui.json 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")

    if [ -n "$EFFECTIVE_TOKEN" ] && bash -c "echo > /dev/tcp/127.0.0.1/6099" 2>/dev/null; then
        # Compute SHA256 hash and try login
        HASH=$(python3 -c "import hashlib; print(hashlib.sha256(('${EFFECTIVE_TOKEN}' + '.napcat').encode()).hexdigest())" 2>/dev/null || echo "")
        if [ -n "$HASH" ]; then
            AUTH_RESP=$(curl -s -X POST http://127.0.0.1:6099/api/auth/login \
                -H 'Content-Type: application/json' \
                -d "{\"hash\":\"${HASH}\"}" 2>/dev/null || echo "")
            AUTH_CODE=$(echo "$AUTH_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('code', -999))" 2>/dev/null || echo "-999")
            if [ "$AUTH_CODE" = "0" ]; then
                ok "NapCat WebUI 鉴权成功 (token=\"${EFFECTIVE_TOKEN}\")"
            else
                fail "NapCat WebUI 鉴权失败 (code=${AUTH_CODE})"
                echo -e "     响应: ${AUTH_RESP}"
                echo -e "     这通常意味着 NapCat 内部使用的 Token 与配置文件不同步"
                echo -e "     建议重新创建容器: docker rm -f ${CONTAINER_NAME} 后重新安装"
            fi
        else
            warn "无法计算鉴权 Hash (python3 不可用)"
        fi
    else
        warn "无法执行鉴权测试 (WebUI 端口不可达或 Token 为空)"
    fi
fi

# ─── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}修复报告${NC}"
echo -e "  检查项: ${CHECKS}"
echo -e "  ${GREEN}修复: ${FIXED}${NC}"
echo -e "  ${YELLOW}警告: ${WARNINGS}${NC}"
echo -e "  ${RED}错误: ${ERRORS}${NC}"

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "\n${GREEN}${BOLD}✅ 所有检查通过，NapCat 配置正常！${NC}"
elif [ $ERRORS -eq 0 ]; then
    echo -e "\n${YELLOW}${BOLD}⚠️ 存在 ${WARNINGS} 个警告，但无严重错误${NC}"
else
    echo -e "\n${RED}${BOLD}❌ 存在 ${ERRORS} 个错误需要手动处理${NC}"
fi

if [ $FIXED -gt 0 ]; then
    echo -e "\n${CYAN}${BOLD}已自动修复 ${FIXED} 个问题${NC}"
    echo -e "建议操作："
    echo -e "  1. 在 ClawPanel 面板中重新扫码登录 QQ"
    echo -e "  2. 如问题仍存在，运行: docker logs ${CONTAINER_NAME} --tail 50"
fi

echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
