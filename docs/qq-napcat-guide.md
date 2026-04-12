# QQ 个人号通道说明 — NapCat + OpenClaw + ClawPanel 通讯架构

## 一、整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户的 QQ 消息                           │
└──────────────────────────────┬──────────────────────────────────┘
                               │ QQ 协议
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                    NapCat（QQ 协议实现层）                        │
│                                                                  │
│  Linux:  Docker 容器 "openclaw-qq"                               │
│          端口映射: 3000(HTTP) / 3001(OneBot WS) / 6099(WebUI)    │
│                                                                  │
│  Windows: NapCat Shell 进程 (NapCatWinBootMain.exe)              │
│           本地监听同样的端口                                      │
└──────────────────────────────┬───────────────────────────────────┘
                               │ OneBot v11 WebSocket
                               │ ws://127.0.0.1:3001
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                  OpenClaw（AI 助手网关）                           │
│                                                                  │
│  openclaw-gateway 进程，监听端口 18789                            │
│  配置文件: ~/openclaw/config/openclaw.json                        │
│                                                                  │
│  关键配置:                                                        │
│    channels.qq.enabled = true                                    │
│    channels.qq.wsUrl   = "ws://127.0.0.1:3001"                   │
│    plugins.entries.qq.enabled = true                             │
│    plugins.installs.qq.installPath = <openclaw扩展目录>/qq        │
└──────────────────────────────┬───────────────────────────────────┘
                               │ REST API / WebSocket
                               │ http://127.0.0.1:18789
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                  ClawPanel（管理面板）                             │
│                                                                  │
│  Go 后端，默认端口 19527                                          │
│  负责: 配置管理 / 进程监控 / NapCat 状态检测 / 自动重连           │
└──────────────────────────────────────────────────────────────────┘
```

## 二、各组件的职责

| 组件 | 职责 | 运行方式 |
|------|------|----------|
| **NapCat** | QQ 协议实现，将 QQ 消息转换为 OneBot v11 WebSocket 协议 | Linux: Docker `openclaw-qq` / Windows: Shell 进程 |
| **OpenClaw** | AI 助手网关，接收 OneBot WS 消息并调用 AI 模型处理 | Node.js 进程 `openclaw-gateway` |
| **ClawPanel** | 管理面板，负责配置、监控、诊断、自动重连 | Go 二进制 + systemd 服务 |

## 三、关键配置文件

### openclaw.json（QQ 通道相关配置）

```json
{
  "gateway": {
    "mode": "local"
  },
  "channels": {
    "qq": {
      "enabled": true,
      "wsUrl": "ws://127.0.0.1:3001"
    }
  },
  "plugins": {
    "entries": {
      "qq": {
        "enabled": true
      }
    },
    "installs": {
      "qq": {
        "installPath": "/root/.nvm/versions/node/v24.x.x/lib/node_modules/openclaw/extensions/qq",
        "source": "builtin",
        "version": "latest"
      }
    }
  }
}
```

### NapCat onebot11.json（OneBot WS 服务端配置）

容器内路径：`/app/napcat/config/onebot11.json`

```json
{
  "network": {
    "websocketServers": [
      {
        "name": "WS服务器",
        "enable": true,
        "host": "0.0.0.0",
        "port": 3001,
        "messagePostFormat": "array",
        "enableForcePushEvent": true
      }
    ]
  }
}
```

## 四、通讯流程

1. QQ 用户向机器人账号发送消息
2. NapCat 通过 QQ 协议接收消息，转换为 OneBot v11 格式，通过 WS 推送
3. OpenClaw 的 `qq` 扩展订阅 `ws://127.0.0.1:3001`，接收 OneBot 事件
4. OpenClaw 网关将消息交给 AI 模型处理，返回结果
5. OpenClaw 通过 OneBot 接口调用 NapCat 发送回复消息
6. NapCat 将回复消息通过 QQ 协议发出

## 五、安装 QQ NapCat 插件

### Linux / macOS（一键修复）

```bash
# 下载并运行诊断修复脚本
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/napcat-fix.sh" -o napcat-fix.sh
sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash napcat-fix.sh
```

### Windows（一键修复）

```powershell
# 以管理员身份运行 PowerShell
$env:CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
irm "$env:CLAWPANEL_PUBLIC_BASE/scripts/fix-qq-napcat.ps1" | iex
```

### 手动安装步骤

1. **确保 Node.js ≥ 20 已安装**
2. **安装/升级 OpenClaw**
   ```bash
   npm install -g openclaw@latest --registry=https://registry.npmmirror.com
   ```
3. **在 ClawPanel 中安装 NapCat**
   - 打开 ClawPanel → 软件环境 → 安装 NapCat
4. **启用 QQ 通道**
   - 打开 ClawPanel → 通道配置 → QQ (NapCat) → 启用
5. **扫码登录**
   - ClawPanel → NapCat 管理 → 登录 → 扫描二维码

## 六、常见问题与修复

### 问题 1：OpenClaw 日志持续出现 WS 连接/断开

**原因**：QQ 通道关闭但 NapCat 容器仍在运行，OpenClaw 网关不断尝试重连。

**修复（v5.0.15+）**：已自动修复。关闭 QQ 通道时 ClawPanel 会同时停止 NapCat 容器，重新启用时再恢复。

### 问题 2：`unknown channel id: qq`

**原因**：QQ 扩展插件文件权限问题（非 root 所有）或 `plugins.installs.qq` 配置缺失。

**修复**：
```bash
# 修复文件权限
chown -R root:root $(npm root -g)/openclaw/extensions/qq

# 或运行一键修复脚本
sudo bash fix-qq-napcat.sh
```

### 问题 3：NapCat 容器启动后 WS 端口不通

**原因**：NapCat 初始化需要时间，或 onebot11.json 未配置 WS 服务端。

**修复**：
```bash
# 检查容器日志
docker logs openclaw-qq --tail 50

# 检查端口配置
docker exec openclaw-qq cat /app/napcat/config/onebot11.json
```

### 问题 4：登录二维码过期或 QQ 掉线

**说明**：NapCat 使用 QQ 个人号协议，存在被封号风险，建议使用小号。登录过期需重新扫码。

**修复**：ClawPanel → NapCat 管理 → 重新登录

### 问题 5：Windows 版 NapCat Shell 无法启动

**修复**：
```powershell
# 运行诊断脚本
$env:CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
irm "$env:CLAWPANEL_PUBLIC_BASE/scripts/fix-qq-napcat.ps1" | iex
```

## 七、网络端口说明

| 端口 | 用途 | 访问方 |
|------|------|--------|
| 3001 | NapCat OneBot v11 WebSocket 服务端 | OpenClaw 连接 |
| 3000 | NapCat HTTP 接口 | OpenClaw 调用 |
| 6099 | NapCat WebUI（扫码登录界面） | ClawPanel / 浏览器 |
| 18789 | OpenClaw 网关 HTTP | ClawPanel |
| 19527 | ClawPanel 面板 | 浏览器 |

> **安全提示**：上述端口仅供本机内部通讯使用，请勿将 3001/3000/6099 暴露到公网。
