#!/usr/bin/env bash
# ============================================================
# ClawPanel v4.3.0 — macOS 安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/install-mac.sh | bash
# ============================================================
set -euo pipefail

CLAWPANEL_VERSION="4.3.0"
CLAWPANEL_REPO="https://github.com/zhaoxinyi02/ClawPanel.git"
DEFAULT_INSTALL_DIR="${HOME}/ClawPanel"
DEFAULT_PORT=6199

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; PURPLE='\033[0;35m'; NC='\033[0m'
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

echo -e "${PURPLE}"
echo "  ╔═══════════════════════════════════════════╗"
echo "  ║       🐾 ClawPanel v${CLAWPANEL_VERSION} for macOS       ║"
echo "  ╚═══════════════════════════════════════════╝"
echo -e "${NC}"

# Check prerequisites
command -v node &>/dev/null || error "Node.js not found. Install: brew install node"
command -v git &>/dev/null || error "Git not found. Install: brew install git"
ok "Node.js $(node -v), Git $(git --version | awk '{print $3}')"

# Gather input
echo ""
read -rp "  Install directory [${DEFAULT_INSTALL_DIR}]: " INSTALL_DIR
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

read -rp "  Panel port [${DEFAULT_PORT}]: " PANEL_PORT
PANEL_PORT="${PANEL_PORT:-$DEFAULT_PORT}"

read -rsp "  Admin password [openclaw-qq-admin]: " ADMIN_TOKEN; echo
ADMIN_TOKEN="${ADMIN_TOKEN:-openclaw-qq-admin}"

# Detect OpenClaw
OPENCLAW_DIR="${HOME}/.openclaw"
OPENCLAW_APP=""
[ -d "${HOME}/openclaw/app" ] && OPENCLAW_APP="${HOME}/openclaw/app"

# Clone & Build
echo ""
info "Cloning ClawPanel..."
if [ -d "${INSTALL_DIR}" ]; then
    cd "${INSTALL_DIR}" && git pull origin main
else
    git clone "${CLAWPANEL_REPO}" "${INSTALL_DIR}" && cd "${INSTALL_DIR}"
fi

info "Installing & building..."
cd server && npm install --omit=dev && npx tsc && cd ..
cd web && npm install && npm run build && cd ..

# Config
mkdir -p data
cat > .env << EOF
ADMIN_TOKEN=${ADMIN_TOKEN}
OPENCLAW_DIR=${OPENCLAW_DIR}
OPENCLAW_APP=${OPENCLAW_APP}
EOF

if [ ! -f "data/admin-config.json" ]; then
cat > data/admin-config.json << EOF
{
  "server": { "port": ${PANEL_PORT}, "host": "0.0.0.0", "token": "${ADMIN_TOKEN}" },
  "openclaw": { "configPath": "${OPENCLAW_DIR}/openclaw.json", "autoSetup": true },
  "napcat": { "wsUrl": "ws://127.0.0.1:3001", "accessToken": "", "webuiPort": 6099, "webuiToken": "openclaw-qq-admin" },
  "wechat": { "apiUrl": "http://localhost:3001", "token": "openclaw-wechat", "enabled": false },
  "qq": { "ownerQQ": 0, "antiRecall": { "enabled": true }, "enabledChannels": [] }
}
EOF
fi

# Create launchd plist
PLIST_PATH="${HOME}/Library/LaunchAgents/com.clawpanel.server.plist"
cat > "${PLIST_PATH}" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.clawpanel.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>$(which node)</string>
        <string>${INSTALL_DIR}/server/dist/index.js</string>
    </array>
    <key>WorkingDirectory</key><string>${INSTALL_DIR}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>NODE_ENV</key><string>production</string>
        <key>OPENCLAW_CONFIG</key><string>${OPENCLAW_DIR}/openclaw.json</string>
        <key>OPENCLAW_APP</key><string>${OPENCLAW_APP}</string>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>/tmp/clawpanel.log</string>
    <key>StandardErrorPath</key><string>/tmp/clawpanel.err</string>
</dict>
</plist>
EOF

launchctl load "${PLIST_PATH}" 2>/dev/null || true
ok "LaunchAgent created and loaded"

# Get IP
LOCAL_IP=$(ipconfig getifaddr en0 2>/dev/null || echo "127.0.0.1")

echo ""
echo -e "${GREEN}  🐾 ClawPanel installed successfully!${NC}"
echo ""
echo -e "  ${BLUE}Panel:${NC}    http://localhost:${PANEL_PORT}"
echo -e "  ${BLUE}LAN:${NC}      http://${LOCAL_IP}:${PANEL_PORT}"
echo -e "  ${BLUE}Password:${NC} ${ADMIN_TOKEN}"
echo ""
echo "  launchctl unload ${PLIST_PATH}   # Stop"
echo "  launchctl load ${PLIST_PATH}     # Start"
echo ""

open "http://localhost:${PANEL_PORT}"
