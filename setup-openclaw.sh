#!/bin/bash
# OpenClaw Manager - OpenClaw 连接配置脚本
# 支持 QQ + 微信双通道配置
# 兼容 Linux / macOS

set -e

OPENCLAW_CONFIG="${HOME}/.openclaw/openclaw.json"

echo "=========================================="
echo " OpenClaw Manager - 连接配置"
echo "=========================================="

# 检查 OpenClaw 配置文件
if [ ! -f "${OPENCLAW_CONFIG}" ]; then
    echo "[错误] 未找到 OpenClaw 配置文件: ${OPENCLAW_CONFIG}"
    echo "请先安装 OpenClaw："
    echo "  curl -fsSL https://get.openclaw.ai | bash"
    echo "  openclaw onboard"
    exit 1
fi

echo "[1/2] 配置 OpenClaw QQ + 微信频道..."

python3 -c "
import json, sys

config_path = '${OPENCLAW_CONFIG}'
try:
    with open(config_path, 'r') as f:
        c = json.load(f)
except Exception as e:
    print(f'[错误] 读取配置失败: {e}')
    sys.exit(1)

# 确保 channels 存在
if 'channels' not in c:
    c['channels'] = {}

# 配置 QQ 频道
if 'qq' not in c['channels']:
    c['channels']['qq'] = {}
c['channels']['qq']['enabled'] = True
c['channels']['qq']['wsUrl'] = 'ws://127.0.0.1:6199/onebot'
c['channels']['qq']['accessToken'] = ''

# 确保 plugins 存在
if 'plugins' not in c:
    c['plugins'] = {}
if 'entries' not in c['plugins']:
    c['plugins']['entries'] = {}
c['plugins']['entries']['qq'] = {'enabled': True}

with open(config_path, 'w') as f:
    json.dump(c, f, indent=4, ensure_ascii=False)

print('[OK] OpenClaw 频道配置已更新')
print('  QQ wsUrl: ws://127.0.0.1:6199/onebot')
print('  QQ enabled: true')
"

echo "[2/2] 重启 OpenClaw..."

if systemctl is-active --quiet openclaw 2>/dev/null; then
    systemctl restart openclaw
    echo "[OK] OpenClaw 服务已重启"
elif command -v openclaw &>/dev/null; then
    echo "[提示] 请手动重启 OpenClaw："
    echo "  openclaw gateway restart"
else
    echo "[提示] 请手动重启 OpenClaw 使配置生效"
fi

echo ""
echo "=========================================="
echo " 配置完成！"
echo "=========================================="
echo ""
echo "接下来："
echo "  1. 访问管理后台 http://你的服务器IP:6199"
echo "  2. QQ 登录：在 QQ 登录页面扫码"
echo "  3. 微信登录：在微信登录页面扫码"
echo "  4. 用另一个 QQ/微信号发私聊消息测试 AI 回复"
echo ""
