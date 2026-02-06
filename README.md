![OpenClaw QQ Plugin](img/OpenClawQQPlugin.png)


<h1 align="center">OpenClaw QQ Plugin</h1>

<p align="center">
  <strong>让你的 AI Agent 接入 QQ 个人号 —— 支持文字、图片、文件、语音、主动推送、定时任务</strong>
</p>

<p align="center">
  <a href="#-快速开始">快速开始</a> •
  <a href="#-功能特性">功能特性</a> •
  <a href="#-架构说明">架构说明</a> •
  <a href="#-工具脚本">工具脚本</a> •
  <a href="#-常见问题">FAQ</a> •
  <a href="#english">English</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/OpenClaw-Plugin-blue?style=flat-square" alt="OpenClaw Plugin" />
  <img src="https://img.shields.io/badge/Protocol-OneBot_v11-green?style=flat-square" alt="OneBot v11" />
  <img src="https://img.shields.io/badge/Language-TypeScript-blue?style=flat-square" alt="TypeScript" />
  <img src="https://img.shields.io/badge/License-MIT-yellow?style=flat-square" alt="MIT License" />
  <img src="https://img.shields.io/badge/Node.js-22%2B-brightgreen?style=flat-square" alt="Node.js 22+" />
</p>

---

## 简介

这是一个 [OpenClaw](https://github.com/openclaw/openclaw) 的 QQ 频道插件，通过 [OneBot v11](https://github.com/botuniverse/onebot-11) 协议（WebSocket）接入 QQ 个人号。

配合 [NapCat](https://github.com/NapCatQQ/NapCat-Docker) 使用，可以让你的 AI Agent 通过 QQ 与你对话，并且支持发送图片、文件、语音、视频等多媒体消息，甚至可以主动给你发消息、定时问候、服务器告警通知。

**不仅仅是聊天机器人，而是一个真正的 AI 助手。**

---

## 功能特性

| 功能 | 说明 | 状态 |
|------|------|:----:|
| 文字聊天 | 与 AI Agent 双向文字对话 | ✅ |
| 图片发送 | AI 可以发送图片（生成的图表、截图等） | ✅ |
| 图片理解 | 用户发图片，AI 能识别并理解 | ✅ |
| 文件发送 | AI 可以发送 PDF、TXT、DOCX 等文件 | ✅ |
| 语音消息 | 支持语音消息段发送 | ✅ |
| 视频消息 | 支持视频消息段发送 | ✅ |
| 群聊支持 | 支持私聊和群聊两种模式 | ✅ |
| 主动推送 | 绕过 Agent，直接通过 API 发送消息 | ✅ |
| 定时任务 | 基于 crontab 的定时消息发送 | ✅ |
| 随机问候 | 每天随机时间发送问候，模拟真人 | ✅ |
| 服务器告警 | 端口监控、磁盘/内存告警自动通知 | ✅ |
| 完整 OneBot API | 封装了 OneBot v11 全部 API | ✅ |

---

## 项目结构

```
openclaw-qq-plugin/
├── plugin/                      # QQ Channel 插件（核心）
│   ├── index.ts                 # 插件入口，注册 channel
│   ├── package.json             # 插件依赖
│   ├── openclaw.plugin.json     # OpenClaw 插件清单
│   └── src/
│       ├── channel.ts           # 核心逻辑：消息收发、媒体检测、文件服务器
│       ├── client.ts            # OneBot v11 WebSocket 客户端（完整 API）
│       ├── config.ts            # 配置 Schema（Zod）
│       ├── file-server.ts       # HTTP 文件服务器
│       ├── runtime.ts           # PluginRuntime 管理
│       └── types.ts             # OneBot v11 完整类型定义
├── tools/                       # 工具脚本集
│   ├── send-message.js          # 主动发送 QQ 消息
│   ├── simple-cron.js           # 定时任务管理（crontab）
│   ├── random-greetings.js      # 随机问候生成器
│   ├── event-monitor.js         # 服务器事件监听与告警
│   ├── cron-helper.js           # OpenClaw Cron API 管理
│   └── package.json
├── systemd/
│   └── qq-event-monitor.service # 事件监听 systemd 服务
├── config-example/
│   └── openclaw.json            # OpenClaw 配置示例
├── README.md
├── LICENSE
└── .gitignore
```

---

## 架构说明

![image-20260206194430728](img/image-20260206194430728.png)

**消息流程：**

1. 用户在 QQ 发送消息
2. NapCat 通过 WebSocket 推送事件到 QQ Plugin
3. Plugin 构建上下文 → 记录会话 → 调用 Agent
4. Agent 生成回复（可能包含文本、图片、文件）
5. Plugin 通过 `deliver()` 回调将回复发送回 QQ

---

## 快速开始

### 前置条件

- [OpenClaw](https://github.com/openclaw/openclaw) 已安装并运行（Node.js 22+）
- [NapCat](https://github.com/NapCatQQ/NapCat-Docker) 已安装并运行（推荐 Docker）
- QQ 个人号已登录 NapCat
- NapCat 已开启**正向 WebSocket 服务**（默认端口 3001）

### 第一步：安装插件

```bash
# 克隆仓库
git clone https://github.com/YOUR_USERNAME/openclaw-qq-plugin.git

# 将 plugin 目录复制到 OpenClaw extensions
cp -r openclaw-qq-plugin/plugin ~/.openclaw/extensions/qq

# 安装依赖
cd ~/.openclaw/extensions/qq
npm install --omit=dev
```

### 第二步：配置 OpenClaw

编辑 `~/.openclaw/openclaw.json`，添加 QQ 插件配置：

```json
{
  "plugins": {
    "entries": {
      "qq": { "enabled": true }
    }
  },
  "channels": {
    "qq": {
      "enabled": true,
      "wsUrl": "ws://127.0.0.1:3001",
      "accessToken": "your_napcat_token"
    }
  }
}
```

> 完整配置示例见 [config-example/openclaw.json](config-example/openclaw.json)

### 第三步：修改关键参数

**必须修改的地方**（搜索 `TODO` 即可找到）：

| 文件 | 参数 | 说明 |
|------|------|------|
| `plugin/src/channel.ts` | `FILE_SERVER_BASE_URL` | Docker 网桥 IP（见下方说明） |
| `tools/send-message.js` | `ACCESS_TOKEN` | NapCat 的 access token |
| `tools/*.js` | `QQ_USER` | 你的 QQ 号 |
| `tools/cron-helper.js` | `GATEWAY_TOKEN` | OpenClaw Gateway token |

**获取 Docker 网桥 IP：**

```bash
ip addr show docker0 | grep inet
# 输出类似：inet 172.17.0.1/16 → 使用 172.17.0.1
```

> 如果 NapCat 不是 Docker 部署，直接用 `127.0.0.1` 即可。

### 第四步：重启 OpenClaw

```bash
# 重启 Gateway
openclaw gateway restart

# 验证插件加载
openclaw channels status
```

在 QQ 上给机器人发一条消息，如果收到回复，说明一切正常！

---

## 工具脚本

工具脚本位于 `tools/` 目录，提供主动发送、定时任务等扩展功能。

### 安装

```bash
cd tools/
npm install
```

### 主动发送消息

```bash
# 发送私聊消息
node tools/send-message.js <QQ号> "你好，这是一条主动消息"

# 发送群消息
node tools/send-message.js group:<群号> "这是一条群消息"
```

### 定时任务

```bash
# 每天早上 8:00 发送问候
node tools/simple-cron.js add-daily 8 0 "早上好！新的一天开始了"

# 查看所有定时任务
node tools/simple-cron.js list

# 一键创建示例任务（早安、午餐、晚安）
node tools/simple-cron.js examples
```

### 随机问候（模拟真人）

每天生成 15-25 个随机时间点，发送不同的问候语：

```bash
# 生成今天的随机问候任务
node tools/random-greetings.js generate

# 预览随机时间点
node tools/random-greetings.js preview

# 测试发送一条
node tools/random-greetings.js test
```

每天自动刷新：

```bash
# 添加到 crontab，每天凌晨 0:05 自动重新生成
(crontab -l 2>/dev/null; echo "5 0 * * * node /root/qq-tools/random-greetings.js generate") | crontab -
```

### 服务器事件监听

监控端口访问、磁盘空间、内存使用，异常时自动通过 QQ 告警：

```bash
# 前台运行
node tools/event-monitor.js

# 部署为 systemd 服务（推荐）
sudo cp systemd/qq-event-monitor.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now qq-event-monitor.service
```

---

## 常见问题

<details>
<summary><b>NapCat 无法访问文件服务器（文件/图片发送失败）</b></summary>

NapCat 在 Docker 容器中时，`127.0.0.1` 指向容器自身。需要使用宿主机的 Docker 网桥 IP：

```bash
ip addr show docker0 | grep inet
# 输出：inet 172.17.0.1/16
```

修改 `plugin/src/channel.ts` 中的 `FILE_SERVER_BASE_URL` 为 `http://172.17.0.1:18790`。

</details>

<details>
<summary><b>文件发送报 Invalid URL</b></summary>

不要对文件路径使用 `encodeURIComponent()`，否则 `/` 会被编码为 `%2F`。代码中已正确处理，直接拼接路径即可。

</details>

<details>
<summary><b>用户发图片后 AI 回复"没收到文本"</b></summary>

已在代码中处理：当消息只包含图片/文件等媒体时，自动添加 `[图片]`、`[文件]` 等描述性文本。

</details>

<details>
<summary><b>AI 发送文件后多了一条奇怪的回复</b></summary>

NapCat 发送文件后会产生空的消息回执事件。已在代码中过滤空消息。

</details>

<details>
<summary><b>TypeScript 报错 "Cannot find module 'openclaw/plugin-sdk'"</b></summary>

这是正常的。`openclaw/plugin-sdk` 是 OpenClaw 在运行时通过 `jiti` 动态解析的别名，不需要单独安装。插件在 OpenClaw 环境中运行时会自动解析。

</details>

<details>
<summary><b>如何查看插件日志？</b></summary>

插件日志输出到 OpenClaw Gateway 的标准输出，以 `[QQ]` 前缀标识：

```bash
# 如果用 systemd
journalctl -u openclaw -f | grep "\[QQ\]"

# 如果前台运行
# 日志直接显示在终端
```

</details>

---

## 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建你的功能分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'Add amazing feature'`
4. 推送到分支：`git push origin feature/amazing-feature`
5. 提交 Pull Request

---

## 许可证

本项目采用 [MIT License](LICENSE) 开源。

---

## 致谢

- [OpenClaw](https://github.com/openclaw/openclaw) — 强大的 AI Agent 框架
- [NapCat](https://github.com/NapCatQQ/NapCat-Docker) — 现代化的 QQ OneBot 实现
- [OneBot v11](https://github.com/botuniverse/onebot-11) — 统一的聊天机器人 API 标准

---

<a name="english"></a>

## English

### What is this?

This is a QQ channel plugin for [OpenClaw](https://github.com/openclaw/openclaw) that connects your AI Agent to QQ (a popular Chinese messaging platform) via the [OneBot v11](https://github.com/botuniverse/onebot-11) protocol.

### Features

- **Full chat support** — Text, images, files, audio, video messaging
- **Image understanding** — AI can understand images sent by users
- **Proactive messaging** — Send messages without waiting for user input
- **Scheduled tasks** — Cron-based scheduled messages
- **Random greetings** — Human-like random greeting times
- **Server monitoring** — Port, disk, memory alerts via QQ

### Quick Start

1. Install [OpenClaw](https://github.com/openclaw/openclaw) and [NapCat](https://github.com/NapCatQQ/NapCat-Docker)
2. Clone this repo and copy `plugin/` to `~/.openclaw/extensions/qq/`
3. Run `npm install --omit=dev` in the plugin directory
4. Add QQ channel config to `~/.openclaw/openclaw.json`:

```json
{
  "plugins": { "entries": { "qq": { "enabled": true } } },
  "channels": {
    "qq": {
      "enabled": true,
      "wsUrl": "ws://127.0.0.1:3001",
      "accessToken": "your_token"
    }
  }
}
```

5. Restart OpenClaw Gateway
6. Send a message to your QQ bot!

### Configuration

Search for `TODO` in the source files to find all configurable parameters:

- `FILE_SERVER_BASE_URL` in `plugin/src/channel.ts` — Docker bridge IP for file server
- `ACCESS_TOKEN` in `tools/send-message.js` — NapCat WebSocket token
- `QQ_USER` in `tools/*.js` — Your QQ number

### License

[MIT](LICENSE)
