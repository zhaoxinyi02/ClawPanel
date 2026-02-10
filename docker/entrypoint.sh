#!/bin/bash
set -e

trap "" SIGPIPE

echo "=========================================="
echo " OpenClaw QQ Manager - 一键部署启动"
echo "=========================================="

# === 1. Setup NapCat (from original entrypoint) ===
cd /app

if [ ! -f "napcat/napcat.mjs" ]; then
    echo "[NapCat] 解压 NapCat..."
    unzip -q NapCat.Shell.zip -d ./NapCat.Shell
    cp -rf NapCat.Shell/* napcat/
    rm -rf ./NapCat.Shell
fi
if [ ! -f "napcat/config/napcat.json" ]; then
    unzip -q NapCat.Shell.zip -d ./NapCat.Shell
    cp -rf NapCat.Shell/config/* napcat/config/
    rm -rf ./NapCat.Shell
fi

# === 2. Configure NapCat WebUI ===
WEBUI_TOKEN="${WEBUI_TOKEN:-openclaw-qq-admin}"
CONFIG_PATH=/app/napcat/config/webui.json

if [ ! -f "${CONFIG_PATH}" ]; then
    echo "[NapCat] 配置 WebUI Token..."
    cat > "${CONFIG_PATH}" << EOF
{
    "host": "0.0.0.0",
    "port": 6099,
    "token": "${WEBUI_TOKEN}",
    "loginRate": 3
}
EOF
fi

# === 3. Configure OneBot11 WebSocket (forward mode on port 3001) ===
ONEBOT_CONFIG=/app/napcat/config/onebot11.json
NAPCAT_WS_PORT="${NAPCAT_WS_PORT:-3001}"
NAPCAT_TOKEN="${NAPCAT_TOKEN:-}"

echo "[NapCat] 配置 OneBot11 WebSocket (端口 ${NAPCAT_WS_PORT})..."
cat > "${ONEBOT_CONFIG}" << EOF
{
    "network": {
        "websocketServers": [
            {
                "name": "ws-server",
                "enable": true,
                "host": "0.0.0.0",
                "port": ${NAPCAT_WS_PORT},
                "token": "${NAPCAT_TOKEN}",
                "reportSelfMessage": false,
                "enableForcePushEvent": true,
                "messagePostFormat": "array",
                "debug": false,
                "heartInterval": 30000
            }
        ]
    },
    "musicSignUrl": "",
    "enableLocalFile2Url": true,
    "parseMultMsg": true
}
EOF

# === 4. Setup user permissions ===
: ${NAPCAT_GID:=0}
: ${NAPCAT_UID:=0}
usermod -o -u ${NAPCAT_UID} napcat 2>/dev/null || true
groupmod -o -g ${NAPCAT_GID} napcat 2>/dev/null || true
usermod -g ${NAPCAT_GID} napcat 2>/dev/null || true
chown -R ${NAPCAT_UID}:${NAPCAT_GID} /app/napcat/config /app/.config 2>/dev/null || true

# === 5. Start Xvfb (headless display for QQ) ===
rm -rf "/tmp/.X1-lock"
gosu napcat Xvfb :1 -screen 0 1080x760x16 +extension GLX +render > /dev/null 2>&1 &
sleep 2
export DISPLAY=:1
export FFMPEG_PATH=/usr/bin/ffmpeg

# === 6. Setup admin config ===
ADMIN_CONFIG_DIR="/app/manager/data"
ADMIN_CONFIG_PATH="${ADMIN_CONFIG_DIR}/admin-config.json"
ADMIN_TOKEN="${ADMIN_TOKEN:-${WEBUI_TOKEN}}"
OWNER_QQ="${OWNER_QQ:-0}"
OPENCLAW_CONFIG="${OPENCLAW_CONFIG:-/root/.openclaw/openclaw.json}"

mkdir -p "${ADMIN_CONFIG_DIR}"

if [ ! -f "${ADMIN_CONFIG_PATH}" ]; then
    echo "[Manager] 创建管理后台配置..."
    cat > "${ADMIN_CONFIG_PATH}" << EOF
{
  "server": { "port": 6199, "host": "0.0.0.0", "token": "${ADMIN_TOKEN}" },
  "openclaw": { "configPath": "${OPENCLAW_CONFIG}", "autoSetup": true },
  "napcat": { "wsUrl": "ws://127.0.0.1:${NAPCAT_WS_PORT}", "accessToken": "${NAPCAT_TOKEN}", "webuiPort": 6099 },
  "wechat": { "apiUrl": "${WECHAT_API_URL:-http://wechat:3001}", "token": "${WECHAT_TOKEN:-openclaw-wechat}", "enabled": true, "autoReply": true },
  "qq": {
    "ownerQQ": ${OWNER_QQ},
    "antiRecall": { "enabled": true },
    "poke": { "enabled": true, "replies": ["别戳了！", "再戳就坏了！", "讨厌~", "哼！"] },
    "welcome": { "enabled": true, "template": "欢迎 {nickname} 加入本群！", "delayMs": 1500 },
    "autoApprove": {
      "friend": { "enabled": false, "pattern": "" },
      "group": { "enabled": false, "pattern": "", "rules": [] }
    },
    "notifications": { "memberChange": true, "adminChange": true, "banNotice": true, "antiRecall": true, "pokeReply": true, "honorNotice": true }
  }
}
EOF
fi

export ADMIN_CONFIG_PATH="${ADMIN_CONFIG_PATH}"
export OPENCLAW_CONFIG_PATH="${OPENCLAW_CONFIG}"

# === 7. Auto-install QQ plugin into OpenClaw extensions ===
OPENCLAW_DIR=$(dirname "${OPENCLAW_CONFIG}")
QQ_PLUGIN_DIR="${OPENCLAW_DIR}/extensions/qq"
if [ ! -d "${QQ_PLUGIN_DIR}/src" ]; then
    echo "[Plugin] 安装 QQ 插件到 ${QQ_PLUGIN_DIR}..."
    mkdir -p "${QQ_PLUGIN_DIR}"
    cp -rf /app/manager/docker/qq-plugin/* "${QQ_PLUGIN_DIR}/"
    cd "${QQ_PLUGIN_DIR}" && npm install --production 2>/dev/null || true
    echo "[Plugin] QQ 插件安装完成"
else
    echo "[Plugin] QQ 插件已存在"
fi

# === 8. Start Manager backend (background) ===
echo "[Manager] 启动管理后台..."
cd /app/manager
node server/dist/index.js &
MANAGER_PID=$!
echo "[Manager] PID: ${MANAGER_PID}"

# === 9. Start QQ (NapCat) as main process ===
echo "[NapCat] 启动 QQ..."
cd /app/napcat

# 如果没有设置 ACCOUNT，自动从 NapCat 配置中检测已登录的 QQ 号
if [ -z "${ACCOUNT}" ]; then
    # 从 napcat_<QQ号>.json 文件名中提取 QQ 号
    DETECTED=$(ls /app/napcat/config/napcat_*.json 2>/dev/null | head -1 | grep -oP 'napcat_\K[0-9]+')
    if [ -n "${DETECTED}" ]; then
        echo "[NapCat] 自动检测到已登录 QQ 号: ${DETECTED}，使用快速登录"
        ACCOUNT="${DETECTED}"
    fi
fi

if [ -n "${ACCOUNT}" ]; then
    echo "[NapCat] 使用快速登录: QQ ${ACCOUNT}"
    exec gosu napcat /opt/QQ/qq --no-sandbox -q $ACCOUNT
else
    echo "[NapCat] 未检测到已登录账号，需要扫码登录"
    exec gosu napcat /opt/QQ/qq --no-sandbox
fi
