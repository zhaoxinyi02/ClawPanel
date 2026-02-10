# OpenClaw IM Manager v3.0

> **QQ + 微信双通道 AI 助手 — Docker 一键部署，扫码即用，跨平台支持**

内嵌 NapCat (QQ) + wechatbot-webhook (微信) + 管理后台，Docker Compose 一键启动。

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Docker](https://img.shields.io/badge/docker-ready-brightgreen.svg)
![Version](https://img.shields.io/badge/version-3.0.0-orange.svg)

## 功能特性

### 双通道 AI 对话
- **QQ 私聊/群聊** — 发消息或 @机器人即可与 AI 对话
- **微信私聊** — 微信个人号自动回复 AI 消息
- **多模型支持** — 通过 OpenClaw 配置任意 AI 模型

### QQ 个人号增强
- **防撤回** — 消息撤回时通知主人原始内容
- **戳一戳回复** — 被戳时随机回复，可自定义
- **入群欢迎** — 新成员入群自动发送欢迎消息
- **自动审核** — 好友/入群申请按规则自动通过

### 微信个人号
- **扫码登录** — 管理后台内扫码登录微信
- **消息收发** — 接收/发送微信消息
- **Webhook 回调** — 基于 wechatbot-webhook 的稳定消息通道

### 管理后台（Web UI）
- **仪表盘** — QQ + 微信双通道状态、实时事件流
- **QQ 登录** — 扫码/快速/账密三种登录方式
- **微信登录** — 扫码登录 + 发送测试消息
- **OpenClaw 配置** — 在线编辑 AI 模型、频道、插件
- **QQ Bot 管理** — 群列表、好友列表、在线发消息
- **审核中心** — 好友/入群请求一键同意/拒绝
- **设置** — 所有功能开关可视化配置

### 跨平台
- Linux / macOS / Windows 均可部署
- 提供 bash + PowerShell 双版本配置脚本

## 架构

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

## 快速开始

### 前提条件

- Docker + Docker Compose
- OpenClaw 已安装（`curl -fsSL https://get.openclaw.ai | bash && openclaw onboard`）

### 1. 克隆项目

```bash
git clone https://github.com/你的用户名/openclaw-im-manager.git
cd openclaw-im-manager
```

### 2. 配置环境变量

```bash
cp .env.example .env
nano .env
```

关键配置：
```env
ADMIN_TOKEN=你的密码        # 管理后台密码
QQ_ACCOUNT=你的QQ号         # QQ 账号（可选）
OWNER_QQ=你的QQ号           # 主人 QQ 号
WECHAT_TOKEN=openclaw-wechat  # 微信 Token
OPENCLAW_DIR=~/.openclaw    # OpenClaw 配置目录
```

### 3. 启动服务

```bash
docker-compose up -d
```

### 4. 配置 OpenClaw 连接

```bash
# Linux / macOS
chmod +x setup-openclaw.sh && ./setup-openclaw.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File setup-openclaw.ps1
```

### 5. 登录

1. 浏览器访问 `http://你的服务器IP:6199`
2. 用 `ADMIN_TOKEN` 登录管理后台
3. **QQ 登录**：左侧「QQ 登录」→ 手机 QQ 扫码
4. **微信登录**：左侧「微信登录」→ 手机微信扫码

### 6. 测试

用另一个 QQ/微信号给 Bot 发私聊消息，收到 AI 回复即表示一切正常！

## 端口说明

| 端口 | 用途 | 访问方式 |
|------|------|----------|
| 6099 | NapCat WebUI | `http://IP:6099` |
| 6199 | 管理后台 | `http://IP:6199` |
| 3001 | OneBot11 WS | `ws://IP:3001`（OpenClaw 连接用） |
| 3002 | 微信 Webhook API | `http://IP:3002`（调试用） |

## API 接口

### 认证 & 状态
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录（body: `{ token }`） |
| GET | `/api/status` | 系统状态（QQ + 微信 + OpenClaw） |

### QQ 相关
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/napcat/login-status` | QQ 登录状态 |
| POST | `/api/napcat/qrcode` | 获取 QQ 登录二维码 |
| POST | `/api/napcat/quick-login` | QQ 快速登录 |
| POST | `/api/napcat/password-login` | QQ 账密登录 |
| GET | `/api/bot/groups` | 群列表 |
| GET | `/api/bot/friends` | 好友列表 |
| POST | `/api/bot/send` | 发送 QQ 消息 |

### 微信相关
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/wechat/status` | 微信登录状态 |
| GET | `/api/wechat/login-url` | 获取微信扫码登录地址 |
| POST | `/api/wechat/send` | 发送微信消息 |
| POST | `/api/wechat/send-file` | 发送微信文件 |
| GET | `/api/wechat/config` | 获取微信配置 |
| PUT | `/api/wechat/config` | 更新微信配置 |
| POST | `/api/wechat/callback` | 微信消息回调（内部） |

### WebSocket
| 路径 | 说明 |
|------|------|
| `/ws?token=JWT` | 管理后台实时事件推送 |
| `/onebot` | OneBot11 WS 代理（供 OpenClaw 连接） |

## 目录结构

```
openclaw-im-manager/
├── docker-compose.yml      # Docker 编排（QQ + 微信双容器）
├── Dockerfile              # QQ 管理容器镜像
├── .env.example            # 环境变量模板
├── setup-openclaw.sh       # 配置脚本 (Linux/macOS)
├── setup-openclaw.ps1      # 配置脚本 (Windows)
├── docker/                 # Docker 相关
│   ├── entrypoint.sh       # 容器入口脚本
│   └── qq-plugin/          # QQ 插件源码
├── server/                 # 后端 (TypeScript + Express)
│   └── src/
│       ├── index.ts        # 入口（双通道集成）
│       ├── routes/         # API 路由
│       └── core/
│           ├── onebot-client.ts   # QQ OneBot 客户端
│           ├── wechat-client.ts   # 微信 Webhook 客户端
│           ├── ws-manager.ts      # WebSocket 管理
│           └── admin-config.ts    # 管理配置
├── web/                    # 前端 (React + Vite + TailwindCSS)
│   └── src/
│       ├── pages/
│       │   ├── Dashboard.tsx     # 仪表盘（双通道状态）
│       │   ├── QQLogin.tsx       # QQ 扫码登录
│       │   └── WeChatLogin.tsx   # 微信扫码登录
│       └── ...
└── docs/
    └── CSDN-blog.md
```

## 跨平台部署

| 平台 | 启动命令 | 配置脚本 |
|------|----------|----------|
| Linux | `docker-compose up -d` | `./setup-openclaw.sh` |
| macOS | `docker-compose up -d` | `./setup-openclaw.sh` |
| Windows | `docker-compose up -d` | `powershell -File setup-openclaw.ps1` |

> Windows 用户需在 `.env` 中设置 `OPENCLAW_DIR=C:\Users\你的用户名\.openclaw`

## 常见问题

**Q: QQ 扫码后提示登录失败？**
A: 确保 QQ 账号没有开启设备锁，或尝试使用快速登录。

**Q: 微信扫码页面打不开？**
A: 确保微信容器已启动（`docker-compose logs wechat`），且端口 3002 未被占用。

**Q: 微信提示不支持网页版登录？**
A: 部分微信账号未开通网页版权限，需要使用较早注册的微信号。

**Q: OpenClaw 连接不上？**
A: 运行 `./setup-openclaw.sh` 重新配置，然后重启 OpenClaw。

**Q: 如何查看日志？**
A: `docker-compose logs -f` 或 `docker-compose logs -f wechat`

**Q: 如何更新？**
A: `git pull && docker-compose up -d --build`

## 更新日志

### v3.0.0 (2025-02-10)
- 新增微信个人号接入（基于 wechatbot-webhook）
- 管理后台新增微信登录页面、微信状态显示
- 仪表盘支持 QQ + 微信双通道实时事件
- 新增微信消息收发 API
- 跨平台支持（Linux / macOS / Windows）
- 新增 Windows PowerShell 配置脚本
- 项目重命名为 openclaw-im-manager

### v2.0.0 (2025-02-09)
- 全新管理后台 UI（React + TailwindCSS）
- 集成 QQ 登录功能（扫码/快速/账密）
- 内置 OneBot WS 代理
- QQ 登录 session 持久化

### v1.0.0
- 初始版本：基础管理后台 + NapCat Docker 集成

## License

MIT
