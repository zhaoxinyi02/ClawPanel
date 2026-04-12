#!/usr/bin/env bash
# ClawPanel 全面诊断脚本（Pro 版 & Lite 版通用）
# 用法：bash diagnose.sh
# 自动探测安装类型（Lite / Pro）及所有路径

CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE:-http://43.248.142.249:19527}"
CLAWPANEL_PUBLIC_BASE="${CLAWPANEL_PUBLIC_BASE%/}"

RED='\033[0;31m'; YEL='\033[1;33m'; GRN='\033[0;32m'; CYN='\033[0;36m'; BOLD='\033[1m'; RST='\033[0m'
ok()      { echo -e "  ${GRN}✔${RST}  $*"; }
warn()    { echo -e "  ${YEL}⚠${RST}  $*"; }
fail()    { echo -e "  ${RED}✘${RST}  $*"; }
info()    { echo -e "  ${CYN}ℹ${RST}  $*"; }
section() { echo -e "\n${BOLD}══ $* ══${RST}"; }

# ── 0. 探测安装类型与关键路径 ────────────────────────────────────────────────

EDITION="unknown"
INSTALL_DIR=""
SERVICE_NAME=""
DATA_DIR=""
OPENCLAW_CONFIG_DIR=""
PANEL_PORT=19527
GW_PORT=18789

# Lite 版：固定安装目录 /opt/clawpanel-lite，服务名 clawpanel-lite
for candidate in /opt/clawpanel-lite /usr/local/clawpanel-lite; do
  if [[ -f "$candidate/clawpanel-lite" || -f "$candidate/bin/clawpanel-lite" ]]; then
    EDITION="lite"
    INSTALL_DIR="$candidate"
    SERVICE_NAME="clawpanel-lite"
    DATA_DIR="$INSTALL_DIR/data"
    OPENCLAW_CONFIG_DIR="$DATA_DIR/openclaw-config"
    break
  fi
done

# 若 Lite 服务存在但目录不在上述路径，从 systemd 探测
if [[ "$EDITION" != "lite" ]] && systemctl cat clawpanel-lite &>/dev/null 2>&1; then
  EDITION="lite"
  SERVICE_NAME="clawpanel-lite"
  _exec=$(systemctl cat clawpanel-lite 2>/dev/null | grep ^ExecStart | head -1 | awk '{print $2}')
  if [[ -n "$_exec" ]]; then
    INSTALL_DIR=$(dirname "$_exec")
    # ExecStart 可能是 /opt/clawpanel-lite/clawpanel-lite
    [[ "$(basename "$INSTALL_DIR")" == "bin" ]] && INSTALL_DIR=$(dirname "$INSTALL_DIR")
  fi
  _data=$(systemctl cat clawpanel-lite 2>/dev/null | grep CLAWPANEL_DATA | head -1 | sed 's/.*CLAWPANEL_DATA=//')
  [[ -n "$_data" ]] && DATA_DIR="$_data"
  [[ -z "$DATA_DIR" && -n "$INSTALL_DIR" ]] && DATA_DIR="$INSTALL_DIR/data"
  OPENCLAW_CONFIG_DIR="$DATA_DIR/openclaw-config"
fi

# Pro 版探测
if [[ "$EDITION" != "lite" ]]; then
  EDITION="pro"
  SERVICE_NAME="clawpanel"
  # 从 systemd 读
  if systemctl cat clawpanel &>/dev/null 2>&1; then
    _data=$(systemctl cat clawpanel 2>/dev/null | grep CLAWPANEL_DATA | head -1 | sed 's/.*CLAWPANEL_DATA=//')
    _ocdir=$(systemctl cat clawpanel 2>/dev/null | grep OPENCLAW_DIR | head -1 | sed 's/.*OPENCLAW_DIR=//')
    [[ -n "$_data" ]] && DATA_DIR="$_data"
    [[ -n "$_ocdir" ]] && OPENCLAW_CONFIG_DIR="$_ocdir"
  fi
  # 自动查找数据目录
  if [[ -z "$DATA_DIR" ]]; then
    for p in /home/*/ClawPanel/data /root/ClawPanel/data /opt/clawpanel/data /var/lib/clawpanel; do
      for d in $p; do
        [[ -f "$d/clawpanel.json" ]] && { DATA_DIR="$d"; break 2; }
      done
    done
  fi
  # OpenClaw 目录
  if [[ -z "$OPENCLAW_CONFIG_DIR" ]]; then
    for d in /root/.openclaw "$HOME/.openclaw" /home/*/.openclaw; do
      for _d in $d; do
        [[ -f "$_d/openclaw.json" ]] && { OPENCLAW_CONFIG_DIR="$_d"; break 2; }
      done
    done
  fi
fi

# 从 clawpanel.json 补充端口信息
CLAWPANEL_JSON=""
[[ -n "$DATA_DIR" && -f "$DATA_DIR/clawpanel.json" ]] && CLAWPANEL_JSON="$DATA_DIR/clawpanel.json"
OPENCLAW_JSON=""
[[ -n "$OPENCLAW_CONFIG_DIR" && -f "$OPENCLAW_CONFIG_DIR/openclaw.json" ]] && OPENCLAW_JSON="$OPENCLAW_CONFIG_DIR/openclaw.json"

if [[ -n "$OPENCLAW_JSON" ]] && command -v python3 &>/dev/null; then
  _gp=$(python3 -c "import json; d=json.load(open('$OPENCLAW_JSON')); print(d.get('gateway',{}).get('port',18789))" 2>/dev/null)
  [[ -n "$_gp" ]] && GW_PORT="$_gp"
fi

# Lite 版内嵌 node
BUNDLED_NODE=""
if [[ "$EDITION" == "lite" && -n "$INSTALL_DIR" ]]; then
  for n in "$INSTALL_DIR/runtime/node/bin/node" "$INSTALL_DIR/runtime/node/node"; do
    [[ -x "$n" ]] && { BUNDLED_NODE="$n"; break; }
  done
fi

# Lite 版 openclaw 入口
BUNDLED_OC_ENTRY=""
if [[ "$EDITION" == "lite" && -n "$INSTALL_DIR" ]]; then
  for e in \
    "$INSTALL_DIR/bin/clawlite-openclaw" \
    "$INSTALL_DIR/runtime/bin/openclaw" \
    "$INSTALL_DIR/runtime/openclaw/openclaw.mjs" \
    "$INSTALL_DIR/runtime/openclaw/package.json"; do
    [[ -f "$e" ]] && { BUNDLED_OC_ENTRY="$e"; break; }
  done
fi

# ── 报告头 ──────────────────────────────────────────────────────────────────
echo -e "\n${BOLD}╔════════════════════════════════════════════════╗${RST}"
echo -e "${BOLD}║     ClawPanel 诊断报告                          ║${RST}"
echo -e "${BOLD}╚════════════════════════════════════════════════╝${RST}"
echo -e "  时间:     $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo -e "  主机:     $(hostname)"
echo -e "  系统:     $(uname -srm)"
echo -e "  版本类型: ${BOLD}${EDITION}${RST}"
echo -e "  安装目录: ${INSTALL_DIR:-未找到}"
echo -e "  数据目录: ${DATA_DIR:-未找到}"
echo -e "  OpenClaw配置: ${OPENCLAW_CONFIG_DIR:-未找到}"
echo -e "  服务名:   $SERVICE_NAME"
echo -e "  面板端口: $PANEL_PORT  网关端口: $GW_PORT"

# ── 1. systemd 服务状态 ──────────────────────────────────────────────────────
section "1. systemd 服务状态"

if systemctl is-active "$SERVICE_NAME" &>/dev/null; then
  ok "systemd $SERVICE_NAME: active (running)"
else
  SVC_STATE=$(systemctl is-active "$SERVICE_NAME" 2>/dev/null || echo "not-found")
  fail "systemd $SERVICE_NAME: $SVC_STATE"
fi

echo ""
info "服务配置:"
systemctl cat "$SERVICE_NAME" 2>/dev/null \
  | grep -E "ExecStart|Environment|WorkingDirectory|User" \
  | sed 's/^/     /' || warn "未找到 $SERVICE_NAME.service"

echo ""
info "最近日志 (过滤心跳):"
journalctl -u "$SERVICE_NAME" --no-pager -n 30 2>/dev/null \
  | grep -v "GET /api/status" | tail -15 | sed 's/^/     /' \
  || warn "无法读取 journalctl 日志"

# ── 2. 进程与端口 ────────────────────────────────────────────────────────────
section "2. 进程与端口"

PANEL_BIN="clawpanel-lite"
[[ "$EDITION" == "pro" ]] && PANEL_BIN="clawpanel"

PANEL_PID=$(pgrep -f "$PANEL_BIN" 2>/dev/null | grep -v "$$" | head -1 || true)
if [[ -n "$PANEL_PID" ]]; then
  ok "面板进程 PID: $PANEL_PID  ($(cat /proc/$PANEL_PID/cmdline 2>/dev/null | tr '\0' ' ' | cut -c1-80))"
else
  fail "未找到 $PANEL_BIN 进程"
fi

GW_PID=$(pgrep -f "openclaw-gateway\|clawlite-openclaw" 2>/dev/null | head -3 || true)
if [[ -n "$GW_PID" ]]; then
  ok "OpenClaw 网关进程 PID: $GW_PID"
else
  fail "未找到 openclaw-gateway 进程"
fi

for port in $PANEL_PORT $GW_PORT 19528; do
  listener=$(ss -tlnp 2>/dev/null | grep ":$port " | awk '{print $NF}' | head -1 || true)
  if [[ -n "$listener" ]]; then
    ok "端口 $port 监听中 — $listener"
  else
    fail "端口 $port 未监听"
  fi
done

# ── 3. API 健康检查 ───────────────────────────────────────────────────────────
section "3. ClawPanel API 健康检查"

if curl -sf --max-time 5 "http://127.0.0.1:$PANEL_PORT/api/status" -o /tmp/_cp_status.json 2>/dev/null; then
  ok "ClawPanel API 响应正常"
  python3 - <<'PYEOF' 2>/dev/null || true
import json
try:
    d = json.load(open('/tmp/_cp_status.json'))
    p  = d.get('panel', {})
    oc = d.get('openclaw', {})
    rt = oc.get('runtime', {})
    gw = d.get('gateway', {})
    pr = d.get('process', {})
    print(f"     面板版本:  {p.get('version','?')} ({p.get('edition','?')})")
    print(f"     OC 已配置: {oc.get('configured')}")
    print(f"     运行状态:  {rt.get('state','?')} — {rt.get('title','')}")
    print(f"     网关在线:  {gw.get('running')}")
    print(f"     进程运行:  {pr.get('running')}, PID: {pr.get('pid',0)}")
    print(f"     当前模型:  {oc.get('currentModel') or '未设置'}")
    print(f"     管理运行时:{oc.get('managedRuntime')} / 内嵌运行时: {oc.get('bundledRuntime')}")
except Exception as e:
    print(f"     解析失败: {e}")
PYEOF
else
  fail "ClawPanel API 无响应 (http://127.0.0.1:$PANEL_PORT/api/status)"
fi

GW_HEALTH=$(curl -sf --max-time 5 "http://127.0.0.1:$GW_PORT/healthz" 2>/dev/null || true)
if echo "$GW_HEALTH" | grep -q '"status"'; then
  ok "网关 /healthz: $GW_HEALTH"
elif curl -sf --max-time 5 "http://127.0.0.1:$GW_PORT/" 2>/dev/null | grep -qi "openclaw"; then
  ok "网关 HTTP 响应正常 (含 openclaw 标识)"
else
  fail "网关 HTTP 无响应 (端口 $GW_PORT)"
fi

# ── 4. Lite 版内嵌运行时 ─────────────────────────────────────────────────────
if [[ "$EDITION" == "lite" ]]; then
  section "4. Lite 版内嵌运行时"

  # 安装目录结构
  if [[ -n "$INSTALL_DIR" && -d "$INSTALL_DIR" ]]; then
    ok "安装目录存在: $INSTALL_DIR"
    info "顶层目录:"
    ls "$INSTALL_DIR" 2>/dev/null | sed 's/^/     /'
    if [[ -d "$INSTALL_DIR/runtime" ]]; then
      ok "runtime/ 目录存在"
      info "runtime/ 内容:"
      ls "$INSTALL_DIR/runtime" 2>/dev/null | sed 's/^/     /'
    else
      fail "runtime/ 目录不存在！Lite 包可能解压不完整"
    fi
  else
    fail "安装目录不存在: ${INSTALL_DIR:-未探测到}"
    info "常见安装位置: /opt/clawpanel-lite"
  fi

  # 内嵌 Node.js
  if [[ -n "$BUNDLED_NODE" ]]; then
    NODE_VER=$("$BUNDLED_NODE" --version 2>/dev/null || echo "运行失败")
    ok "内嵌 Node.js: $NODE_VER  ($BUNDLED_NODE)"
    NODE_MAJOR=$(echo "$NODE_VER" | tr -d 'v' | cut -d. -f1)
    [[ "${NODE_MAJOR:-0}" -lt 20 ]] && warn "Node.js 版本低于 v20，建议升级 Lite 包"
  else
    fail "内嵌 Node.js 未找到 (检查 $INSTALL_DIR/runtime/node/)"
    # 列出 runtime/node 下有什么
    if [[ -d "$INSTALL_DIR/runtime/node" ]]; then
      info "runtime/node/ 内容:"
      find "$INSTALL_DIR/runtime/node" -maxdepth 3 2>/dev/null | head -20 | sed 's/^/     /'
    fi
  fi

  # clawlite-openclaw 启动脚本
  if [[ -x "/usr/local/bin/clawlite-openclaw" ]]; then
    ok "clawlite-openclaw symlink 存在: $(readlink -f /usr/local/bin/clawlite-openclaw)"
  elif [[ -x "$INSTALL_DIR/bin/clawlite-openclaw" ]]; then
    warn "clawlite-openclaw 存在但未链接到 /usr/local/bin"
  else
    fail "clawlite-openclaw 未找到"
  fi

  # OpenClaw app 目录
  OC_APP_DIR=""
  for d in \
    "$INSTALL_DIR/runtime/openclaw" \
    "$INSTALL_DIR/runtime/openclaw/package" \
    "$INSTALL_DIR/runtime/openclaw/app"; do
    if [[ -f "$d/package.json" || -f "$d/openclaw.mjs" ]]; then
      OC_APP_DIR="$d"; break
    fi
  done

  if [[ -n "$OC_APP_DIR" ]]; then
    ok "OpenClaw app 目录: $OC_APP_DIR"
    # 检查 package.json 版本
    if command -v python3 &>/dev/null && [[ -f "$OC_APP_DIR/package.json" ]]; then
      _ver=$(python3 -c "import json; print(json.load(open('$OC_APP_DIR/package.json')).get('version','?'))" 2>/dev/null)
      info "OpenClaw 版本: $_ver"
    fi
    # 检查 node_modules
    NM="$OC_APP_DIR/node_modules"
    if [[ -d "$NM" ]]; then
      NM_COUNT=$(find "$NM" -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l)
      ok "node_modules 存在，顶层包数: $NM_COUNT"
      [[ "$NM_COUNT" -lt 5 ]] && warn "node_modules 包数过少，OpenClaw 依赖可能不完整"
    else
      fail "node_modules 不存在于 $NM"
      info "这是导致「OpenClaw 离线」的常见原因，建议重新安装 Lite 版"
    fi
  else
    fail "OpenClaw app 目录未找到 (检查 $INSTALL_DIR/runtime/openclaw/)"
    if [[ -d "$INSTALL_DIR/runtime" ]]; then
      info "runtime/ 完整内容:"
      find "$INSTALL_DIR/runtime" -maxdepth 4 2>/dev/null | head -40 | sed 's/^/     /'
    fi
  fi

  # 文件权限检查
  if [[ -n "$INSTALL_DIR" ]]; then
    OWNER=$(stat -c '%U:%G' "$INSTALL_DIR" 2>/dev/null || echo "未知")
    ok "安装目录所有者: $OWNER"
    [[ "$OWNER" != "root:root" ]] && warn "所有者不是 root:root，可能导致权限问题"
  fi

  # install.log
  for logf in "$DATA_DIR/install.log" "$INSTALL_DIR/install.log" "/tmp/clawpanel-lite-install.log"; do
    [[ -f "$logf" ]] && {
      info "安装日志 $logf (最后30行):"
      tail -30 "$logf" | sed 's/^/     /'
    }
  done

else
  # ── 4. Pro 版 OpenClaw 检查 ──────────────────────────────────────────────
  section "4. Pro 版 OpenClaw 安装"

  if command -v openclaw &>/dev/null; then
    ok "openclaw: $(openclaw --version 2>/dev/null || echo '版本未知')  ($(command -v openclaw))"
  else
    fail "openclaw 不在 PATH 中"
    # 扫描 nvm 路径
    for nvmd in /root/.nvm /home/*/.nvm; do
      for _d in $nvmd; do
        _oc=$(find "$_d" -name openclaw -type f 2>/dev/null | head -1)
        [[ -n "$_oc" ]] && warn "找到 openclaw: $_oc (未在 PATH 中，需在 systemd service 的 PATH 环境变量中加入 $(dirname "$_oc"))"
      done
    done
  fi
fi

# ── 5. openclaw.json 配置 ────────────────────────────────────────────────────
section "5. openclaw.json 配置"

if [[ -n "$OPENCLAW_JSON" ]]; then
  FSIZE=$(stat -c%s "$OPENCLAW_JSON" 2>/dev/null || echo 0)
  if [[ "$FSIZE" -lt 10 ]]; then
    fail "openclaw.json 为空或过小 ($FSIZE 字节)！这是「离线」最常见原因之一"
  else
    ok "openclaw.json: $OPENCLAW_JSON ($FSIZE 字节)"
  fi

  if command -v python3 &>/dev/null; then
    python3 - <<PYEOF 2>/dev/null || warn "python3 解析失败"
import json, sys
try:
    d = json.load(open('$OPENCLAW_JSON'))
    gw       = d.get('gateway', {})
    model    = d.get('agents',{}).get('defaults',{}).get('model',{}).get('primary','未设置')
    provs    = list(d.get('models',{}).get('providers',{}).keys())
    ch_en    = [k for k,v in d.get('channels',{}).items() if isinstance(v,dict) and v.get('enabled')]
    print(f"     网关端口:   {gw.get('port','未设置')}")
    print(f"     网关绑定:   {gw.get('host','未设置(默认127.0.0.1)')}")
    print(f"     当前模型:   {model}")
    print(f"     providers:  {provs or '未配置'}")
    print(f"     启用channels:{ch_en or '无'}")
    # 检查 model apiKey
    has_key = False
    for pid, pv in d.get('models',{}).get('providers',{}).items():
        if pv.get('apiKey','').strip():
            has_key = True
    print(f"     API Key 已填: {has_key}")
except json.JSONDecodeError as e:
    print(f"  ✘ JSON 格式错误: {e}")
except Exception as e:
    print(f"  解析失败: {e}")
PYEOF
  else
    cat "$OPENCLAW_JSON" | sed 's/"apiKey": *"[^"]*"/"apiKey": "***"/g' | sed 's/^/  /'
  fi
else
  fail "未找到 openclaw.json"
  info "OpenClaw 配置目录: ${OPENCLAW_CONFIG_DIR:-未探测到}"
  if [[ -n "$OPENCLAW_CONFIG_DIR" ]]; then
    if [[ -d "$OPENCLAW_CONFIG_DIR" ]]; then
      info "目录存在，内容:"
      ls "$OPENCLAW_CONFIG_DIR" 2>/dev/null | sed 's/^/     /' || info "(空)"
    else
      fail "目录不存在: $OPENCLAW_CONFIG_DIR"
      info "这是导致「OpenClaw 未配置」的直接原因"
    fi
  fi
fi

# ── 6. 错误日志汇总 ───────────────────────────────────────────────────────────
section "6. 错误日志"

ERRORS=$(journalctl -u "$SERVICE_NAME" --no-pager -n 200 2>/dev/null \
  | grep -iE " error| fail| fatal| panic| cannot| permission denied| no such file" \
  | grep -v "GET /api" | tail -15 || true)
if [[ -n "$ERRORS" ]]; then
  warn "journalctl 中的错误/警告 (最近15条):"
  echo "$ERRORS" | sed 's/^/     /'
else
  ok "journalctl 中无明显错误日志"
fi

# updater.log
[[ -f "$DATA_DIR/updater.log" ]] && {
  info "updater.log (最后10行):"
  tail -10 "$DATA_DIR/updater.log" | sed 's/^/     /'
}

# ── 7. 系统资源 ──────────────────────────────────────────────────────────────
section "7. 系统资源"

DISK_PCT=$(df / 2>/dev/null | awk 'NR==2{gsub(/%/,""); print $5}')
DISK_FREE=$(df -h / 2>/dev/null | awk 'NR==2{print $4}')
[[ "${DISK_PCT:-0}" -gt 90 ]] \
  && fail "磁盘空间严重不足: 剩余 $DISK_FREE ($DISK_PCT% 已用)" \
  || ok "磁盘剩余: $DISK_FREE ($DISK_PCT% 已用)"

MEM_FREE=$(free -m 2>/dev/null | awk '/^Mem/{print $7}')
MEM_TOTAL=$(free -m 2>/dev/null | awk '/^Mem/{print $2}')
ok "内存: 可用 ${MEM_FREE:-?} MB / 总计 ${MEM_TOTAL:-?} MB"
[[ "${MEM_FREE:-9999}" -lt 256 ]] && warn "可用内存低于 256 MB，可能导致 OpenClaw 启动失败"

# ── 8. 外网连通性 ─────────────────────────────────────────────────────────────
section "8. 外网连通性"

for host in "github.com" "registry.npmjs.org" "$(printf '%s' "$CLAWPANEL_PUBLIC_BASE" | sed -E 's#^https?://([^/:]+).*#\1#')"; do
  if curl -sf --max-time 6 "https://$host" &>/dev/null || \
     curl -sf --max-time 6 "http://$host" &>/dev/null; then
    ok "$host 可达"
  else
    warn "$host 不可达（可能影响更新/安装）"
  fi
done

# ── 总结 ──────────────────────────────────────────────────────────────────────
section "诊断完成"
echo ""
echo -e "  请将以上完整输出提供给开发者分析。"
if [[ "$EDITION" == "lite" ]]; then
  echo -e "  Lite 版常见修复: ${BOLD}systemctl restart clawpanel-lite${RST}"
  echo -e "  若问题持续，可尝试重新安装: bash <(curl -fsSL ${CLAWPANEL_PUBLIC_BASE}/scripts/install-lite.sh)"
else
  echo -e "  Pro 版常见修复: ${BOLD}systemctl restart clawpanel${RST}"
fi
echo ""
