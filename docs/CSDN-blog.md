# OpenClaw IM Manager v3.0：QQ + 微信双通道 AI 助手，Docker 一键部署

## 前言

想让你的 QQ 和微信个人号同时拥有 AI 对话能力？私聊自动回复、群聊 @回复、防撤回、入群欢迎……这些功能只需要 `docker-compose up -d` 就能全部搞定。

**OpenClaw IM Manager v3.0** 是一个开源项目，将 NapCat（QQ 协议）、wechatbot-webhook（微信协议）、OpenClaw（AI 引擎）和管理后台整合到 Docker Compose 中，实现 QQ + 微信双通道 AI 助手的一键部署。

**GitHub 地址**：[https://github.com/zhaoxinyi02/openclaw-im-manager](https://github.com/zhaoxinyi02/openclaw-im-manager)

## v3.0 vs v2.0

| 功能 | v2.0 | v3.0 |
|------|------|------|
| 支持通道 | 仅 QQ | QQ + 微信双通道 |
| 微信登录 | 无 | 管理后台内扫码登录 |
| 微信消息 | 无 | 收发消息、Webhook 回调 |
| 仪表盘 | 仅 QQ 状态 | QQ + 微信双通道状态 |
| 事件流 | 仅 QQ 事件 | QQ + 微信事件（带来源标签） |
| 跨平台 | 仅 Linux | Linux / macOS / Windows |
| 配置脚本 | 仅 bash | bash + PowerShell |
| 容器架构 | 单容器 | Docker Compose 双容器 |

## 技术架构

```
┌──────────────────────────────────────────────────────┐
│                Docker Compose                        │
│                                                      │
│  ┌─────────────────────────────────────────────────┐ │
│  │         openclaw-qq Container                   │ │
│  │  ┌─────────┐  ┌──────────┐  ┌───────────┐      │ │
│  │  │ NapCat  │  │ Manager  │  │  Frontend  │      │ │
│  │  │ (QQ)    │←→│ Backend  │←→│  (React)   │      │ │
│  │  │ :6099   │  │ :6199    │  │            │      │ │
│  │  └─────────┘  └────┬─────┘  └───────────┘      │ │
│  └─────────────────────┼───────────────────────────┘ │
│                        │ HTTP callback                │
│  ┌─────────────────────┼───────────────────────────┐ │
│  │    openclaw-wechat Container                    │ │
│  │  ┌──────────────────┴──────────────────────┐    │ │
│  │  │  wechatbot-webhook (微信 Web 协议)      │    │ │
│  │  │  :3001 (内部) → :3002 (外部)            │    │ │
│  │  └─────────────────────────────────────────┘    │ │
│  └─────────────────────────────────────────────────┘ │
└──────────┬──────────────┬────────────────────────────┘
           │              │
      ┌────┴────┐    ┌────┴────┐
      │ OpenClaw│    │ Browser │
      │ Gateway │    │ 管理后台 │
      └─────────┘    └─────────┘
```

**关键设计**：
- **QQ 通道**：NapCat 运行在容器内，管理后台通过 OneBot11 WS 连接，并提供 `/onebot` 代理给宿主机 OpenClaw
- **微信通道**：wechatbot-webhook 独立容器运行，通过 HTTP 回调将消息推送给管理后台
- **统一管理**：管理后台同时管理 QQ 和微信，提供统一的 Web UI 和 API
- **持久化**：QQ session、微信数据、配置文件全部通过 Docker Volume 持久化

## 环境准备

| 组件 | 要求 |
|------|------|
| 系统 | Linux / macOS / Windows（需 Docker Desktop） |
| Docker | 20.10+ |
| Docker Compose | v2+ |
| OpenClaw | 已安装并运行 |
| 内存 | 2GB+ |

> **没装 OpenClaw？**
> ```bash
> curl -fsSL https://get.openclaw.ai | bash
> openclaw onboard
> openclaw gateway start
> ```

## 快速部署（5 分钟）

### 第一步：克隆项目

```bash
git clone https://github.com/zhaoxinyi02/openclaw-im-manager.git
cd openclaw-im-manager
```

### 第二步：配置环境变量

```bash
cp .env.example .env
nano .env
```

**关键配置：**

```env
# 管理后台登录密码
ADMIN_TOKEN=your-secure-password

# QQ 账号（可选）
QQ_ACCOUNT=你的QQ号

# 主人 QQ 号
OWNER_QQ=你的QQ号

# 微信 Token
WECHAT_TOKEN=openclaw-wechat

# OpenClaw 配置目录（跨平台）
OPENCLAW_DIR=~/.openclaw
```

### 第三步：一键启动

```bash
docker-compose up -d
```

首次启动会拉取微信镜像 + 构建 QQ 镜像，约 3-5 分钟。

### 第四步：配置 OpenClaw 连接

```bash
# Linux / macOS
chmod +x setup-openclaw.sh && ./setup-openclaw.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File setup-openclaw.ps1
```

### 第五步：扫码登录

1. 浏览器打开 `http://你的服务器IP:6199`
2. 输入 `ADMIN_TOKEN` 登录管理后台
3. **QQ**：左侧「QQ 登录」→ 手机 QQ 扫码
4. **微信**：左侧「微信登录」→ 手机微信扫码

### 第六步：测试

- 用另一个 QQ 号发私聊消息 → 收到 AI 回复
- 用另一个微信号发私聊消息 → 收到 AI 回复

## 核心技术点

### 1. 微信接入方案选型

经过调研对比多个方案：

| 方案 | 协议 | 平台要求 | 维护状态 |
|------|------|----------|----------|
| WeChatFerry | Windows 注入 | 需要 Windows + 微信 PC | 活跃 |
| GeWeChat | iPad 协议 | 需付费 API | 商业 |
| wechatbot-webhook | Web 协议 | Docker 原生 | 活跃 |

最终选择 **wechatbot-webhook**：Docker 原生、轻量、HTTP API 简洁、支持消息回调。

### 2. 双容器通信

微信容器通过 `RECVD_MSG_API` 环境变量配置消息回调地址，指向管理后台的 `/api/wechat/callback`：

```yaml
# docker-compose.yml
wechat:
  image: dannicool/docker-wechatbot-webhook
  environment:
    - RECVD_MSG_API=http://openclaw-qq:6199/api/wechat/callback
    - LOGIN_API_TOKEN=${WECHAT_TOKEN}
```

管理后台接收回调后解析消息，通过 WebSocket 推送给前端，并可触发 AI 自动回复。

### 3. 微信客户端封装

```typescript
// WeChatClient 封装了 wechatbot-webhook 的 HTTP API
class WeChatClient extends EventEmitter {
  // 健康检查（每 10 秒轮询）
  async checkHealth() { ... }
  // 获取登录状态
  async getLoginStatus() { ... }
  // 发送消息
  async sendMessage(to, content, isRoom) { ... }
  // 处理回调消息
  handleCallback(formData) {
    // 解析消息 → 触发 'message' 事件
    this.emit('message', event);
  }
}
```

### 4. 前端双通道状态

仪表盘同时显示 QQ 和微信状态，事件流带来源标签：

```typescript
// useWebSocket hook 同时监听两个通道
ws.onmessage = (e) => {
  const msg = JSON.parse(e.data);
  if (msg.type === 'napcat-status') setNapcatStatus(msg.data);
  if (msg.type === 'wechat-status') setWechatStatus(msg.data);
  if (msg.type === 'event' || msg.type === 'wechat-event') {
    // 事件带 _source 标签区分来源
    setEvents(prev => [...prev, { ...msg.data, _source: ... }]);
  }
};
```

### 5. 跨平台支持

通过环境变量 `OPENCLAW_DIR` 实现跨平台路径兼容：

```yaml
volumes:
  - ${OPENCLAW_DIR:-~/.openclaw}:/root/.openclaw
```

同时提供 bash（Linux/macOS）和 PowerShell（Windows）两个版本的配置脚本。

## 管理后台功能

### 仪表盘
- QQ 连接状态 + 微信连接状态
- 群/好友数量
- OpenClaw 配置状态
- QQ + 微信实时事件流（带来源标签）

### QQ 登录
- 扫码登录（QRCode 本地渲染）
- 快速登录（已登录过的账号）
- 账密登录（MD5 服务端计算）

### 微信登录（v3.0 新增）
- 扫码登录（内嵌 wechatbot-webhook 登录页）
- 连接状态实时显示
- 发送测试消息

### 其他功能
- OpenClaw 配置在线编辑
- QQ Bot 消息发送、群管理
- 好友/入群审核
- 防撤回、戳一戳、欢迎语等设置

## 端口说明

| 端口 | 用途 |
|------|------|
| 6099 | NapCat WebUI |
| 6199 | 管理后台（主入口） |
| 3001 | OneBot11 WS（OpenClaw 连接用） |
| 3002 | 微信 Webhook API（调试用） |

## 常见问题

### Q: 微信扫码页面打不开？
A: 确保微信容器已启动：`docker-compose logs wechat`

### Q: 微信提示不支持网页版登录？
A: 部分微信账号未开通网页版权限，需要使用较早注册的微信号。

### Q: QQ 扫码后提示登录失败？
A: 确保 QQ 账号没有开启设备锁，或尝试使用快速登录。

### Q: 如何更新？
```bash
cd openclaw-im-manager
git pull
docker-compose up -d --build
```

## 总结

OpenClaw IM Manager v3.0 实现了 QQ + 微信双通道 AI 助手的一键部署：

1. **克隆项目** → 2. **配置密码** → 3. `docker-compose up -d` → 4. **扫码登录** → 5. **开始使用**

核心亮点：
- **双通道**：QQ + 微信同时接入，统一管理
- **一键部署**：Docker Compose 一条命令搞定
- **跨平台**：Linux / macOS / Windows 均可部署
- **功能完整**：AI 对话 + QQ 增强功能 + 微信消息收发
- **管理方便**：Web 管理后台，双通道状态实时监控
- **稳定可靠**：Session 持久化，WS 代理解决网络问题

如果觉得有用，欢迎 Star ⭐ 支持：[https://github.com/zhaoxinyi02/openclaw-im-manager](https://github.com/zhaoxinyi02/openclaw-im-manager)
