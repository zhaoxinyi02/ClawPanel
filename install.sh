#!/usr/bin/env bash
# ============================================================
# ClawPanel v4.3.0 — Linux 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/install.sh | bash
# ============================================================
set -euo pipefail

CLAWPANEL_VERSION="4.3.0"
CLAWPANEL_REPO="https://github.com/zhaoxinyi02/ClawPanel.git"
DEFAULT_INSTALL_DIR="/opt/clawpanel"
DEFAULT_PORT=6199

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; PURPLE='\033[0;35m'; NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

echo -e "${PURPLE}"
echo "  ╔═══════════════════════════════════════════╗"
echo "  ║       🐾 ClawPanel v${CLAWPANEL_VERSION} Installer       ║"
echo "  ║   OpenClaw 智能管理面板 一键安装脚本      ║"
echo "  ╚═══════════════════════════════════════════╝"
echo -e "${NC}"

# --- Check prerequisites ---
check_command() {
    if ! command -v "$1" &>/dev/null; then
        return 1
    fi
    return 0
}

# Check Node.js
if ! check_command node; then
    error "未检测到 Node.js，请先安装 Node.js 22+：https://nodejs.org/"
fi
NODE_VER=$(node -v | sed 's/v//' | cut -d. -f1)
if [ "$NODE_VER" -lt 18 ]; then
    error "Node.js 版本过低（当前 v$(node -v)），需要 v18+"
fi
ok "Node.js $(node -v)"

# Check git
if ! check_command git; then
    error "未检测到 Git，请先安装：sudo apt install git 或 sudo yum install git"
fi
ok "Git $(git --version | awk '{print $3}')"

# --- Gather user input ---
echo ""
info "请输入安装信息（按回车使用默认值）："
echo ""

read -rp "  安装目录 [${DEFAULT_INSTALL_DIR}]: " INSTALL_DIR
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

read -rp "  管理面板端口 [${DEFAULT_PORT}]: " PANEL_PORT
PANEL_PORT="${PANEL_PORT:-$DEFAULT_PORT}"

read -rsp "  管理后台密码 [openclaw-qq-admin]: " ADMIN_TOKEN
echo ""
ADMIN_TOKEN="${ADMIN_TOKEN:-openclaw-qq-admin}"

read -rp "  QQ 账号（可选，留空跳过）: " QQ_ACCOUNT
read -rp "  主人 QQ 号（接收通知，0=不设置）[0]: " OWNER_QQ
OWNER_QQ="${OWNER_QQ:-0}"

read -rsp "  NapCat WebUI Token [openclaw-qq-admin]: " WEBUI_TOKEN
echo ""
WEBUI_TOKEN="${WEBUI_TOKEN:-openclaw-qq-admin}"

# Detect OpenClaw paths
OPENCLAW_DIR="${HOME}/.openclaw"
OPENCLAW_WORK=""
OPENCLAW_APP=""

if [ -d "${HOME}/openclaw/app" ]; then
    OPENCLAW_APP="${HOME}/openclaw/app"
    ok "检测到 OpenClaw 应用目录: ${OPENCLAW_APP}"
fi
if [ -d "${HOME}/openclaw/work" ]; then
    OPENCLAW_WORK="${HOME}/openclaw/work"
    ok "检测到 OpenClaw 工作目录: ${OPENCLAW_WORK}"
fi
if [ -d "${HOME}/openclaw/config" ]; then
    OPENCLAW_DIR="${HOME}/openclaw/config"
    ok "检测到 OpenClaw 配置目录: ${OPENCLAW_DIR}"
fi

echo ""
info "安装配置："
echo "  安装目录:    ${INSTALL_DIR}"
echo "  面板端口:    ${PANEL_PORT}"
echo "  管理密码:    ****"
echo "  QQ 账号:     ${QQ_ACCOUNT:-（未设置）}"
echo "  主人 QQ:     ${OWNER_QQ}"
echo ""

read -rp "确认安装？[Y/n] " CONFIRM
if [[ "${CONFIRM,,}" == "n" ]]; then
    echo "已取消安装。"
    exit 0
fi

# --- Install ---
echo ""
info "正在克隆 ClawPanel..."
if [ -d "${INSTALL_DIR}" ]; then
    warn "目录已存在，正在更新..."
    cd "${INSTALL_DIR}"
    git pull --rebase origin main 2>/dev/null || git pull origin main
else
    sudo mkdir -p "$(dirname "${INSTALL_DIR}")" 2>/dev/null || true
    git clone "${CLAWPANEL_REPO}" "${INSTALL_DIR}"
    cd "${INSTALL_DIR}"
fi

info "正在安装后端依赖..."
cd server && npm install --omit=dev 2>/dev/null && cd ..

info "正在构建后端..."
cd server && npx tsc 2>/dev/null && cd ..

info "正在安装前端依赖..."
cd web && npm install 2>/dev/null && cd ..

info "正在构建前端..."
cd web && npm run build 2>/dev/null && cd ..

# --- Create config ---
info "正在创建配置..."
mkdir -p data

cat > .env << EOF
ADMIN_TOKEN=${ADMIN_TOKEN}
OPENCLAW_DIR=${OPENCLAW_DIR}
OPENCLAW_WORK=${OPENCLAW_WORK:-${HOME}/openclaw/work}
OPENCLAW_APP=${OPENCLAW_APP:-${HOME}/openclaw/app}
QQ_ACCOUNT=${QQ_ACCOUNT}
WEBUI_TOKEN=${WEBUI_TOKEN}
OWNER_QQ=${OWNER_QQ}
NAPCAT_TOKEN=
WECHAT_TOKEN=openclaw-wechat
EOF

# Create admin config
if [ ! -f "data/admin-config.json" ]; then
cat > data/admin-config.json << EOF
{
  "server": { "port": ${PANEL_PORT}, "host": "0.0.0.0", "token": "${ADMIN_TOKEN}" },
  "openclaw": { "configPath": "${OPENCLAW_DIR}/openclaw.json", "autoSetup": true },
  "napcat": { "wsUrl": "ws://127.0.0.1:3001", "accessToken": "", "webuiPort": 6099, "webuiToken": "${WEBUI_TOKEN}" },
  "wechat": { "apiUrl": "http://localhost:3001", "token": "openclaw-wechat", "enabled": false, "autoReply": true },
  "qq": {
    "ownerQQ": ${OWNER_QQ},
    "antiRecall": { "enabled": true },
    "poke": { "enabled": true, "replies": ["别戳了！", "再戳就坏了！", "讨厌~", "哼！"] },
    "welcome": { "enabled": true, "template": "欢迎 {nickname} 加入本群！", "delayMs": 1500 },
    "autoApprove": {
      "friend": { "enabled": false, "pattern": "" },
      "group": { "enabled": false, "pattern": "", "rules": [] }
    },
    "enabledChannels": ["qq"]
  }
}
EOF
fi

# --- Create systemd service ---
info "正在创建 systemd 服务..."
SERVICE_FILE="/etc/systemd/system/clawpanel.service"
if command -v systemctl &>/dev/null; then
    sudo tee "${SERVICE_FILE}" > /dev/null << EOF
[Unit]
Description=ClawPanel - OpenClaw Management Panel
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=${INSTALL_DIR}
Environment=NODE_ENV=production
Environment=OPENCLAW_CONFIG=${OPENCLAW_DIR}/openclaw.json
Environment=OPENCLAW_WORK=${OPENCLAW_WORK:-${HOME}/openclaw/work}
Environment=OPENCLAW_APP=${OPENCLAW_APP:-${HOME}/openclaw/app}
ExecStart=$(which node) server/dist/index.js
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable clawpanel
    sudo systemctl start clawpanel
    ok "systemd 服务已创建并启动"
fi

# --- Create desktop shortcut (if desktop exists) ---
DESKTOP_DIR="${HOME}/Desktop"
if [ -d "${DESKTOP_DIR}" ] || [ -d "${HOME}/桌面" ]; then
    DESKTOP_DIR="${HOME}/Desktop"
    [ -d "${HOME}/桌面" ] && DESKTOP_DIR="${HOME}/桌面"
    cat > "${DESKTOP_DIR}/ClawPanel.desktop" << EOF
[Desktop Entry]
Name=ClawPanel
Comment=OpenClaw Management Panel
Exec=xdg-open http://localhost:${PANEL_PORT}
Icon=web-browser
Terminal=false
Type=Application
Categories=Network;WebBrowser;
EOF
    chmod +x "${DESKTOP_DIR}/ClawPanel.desktop"
    ok "桌面快捷方式已创建"
fi

# --- Get IP addresses ---
LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")
PUBLIC_IP=$(curl -s --connect-timeout 3 ifconfig.me 2>/dev/null || curl -s --connect-timeout 3 ip.sb 2>/dev/null || echo "")

echo ""
echo -e "${GREEN}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         🐾 ClawPanel 安装完成！                      ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BLUE}安装目录:${NC}    ${INSTALL_DIR}"
echo -e "  ${BLUE}管理面板:${NC}    http://localhost:${PANEL_PORT}"
echo -e "  ${BLUE}内网访问:${NC}    http://${LOCAL_IP}:${PANEL_PORT}"
if [ -n "${PUBLIC_IP}" ]; then
echo -e "  ${BLUE}外网访问:${NC}    http://${PUBLIC_IP}:${PANEL_PORT}"
fi
echo -e "  ${BLUE}登录密码:${NC}    ${ADMIN_TOKEN}"
echo ""
if [ -n "${PUBLIC_IP}" ]; then
echo -e "  ${YELLOW}提示: 如果是云服务器，请使用外网 IP 访问${NC}"
echo -e "  ${YELLOW}      并确保防火墙已开放端口 ${PANEL_PORT}${NC}"
fi
echo ""
echo -e "  ${PURPLE}管理命令:${NC}"
echo "    sudo systemctl status clawpanel    # 查看状态"
echo "    sudo systemctl restart clawpanel   # 重启服务"
echo "    sudo systemctl stop clawpanel      # 停止服务"
echo "    sudo journalctl -u clawpanel -f    # 查看日志"
echo ""
