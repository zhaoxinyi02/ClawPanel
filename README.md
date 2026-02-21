<div align="center">

<img src="img/logo.jpg" width="700"/>

# 🐾 ClawPanel

**OpenClaw 智能管理面板 — 比官方控制台更强大的可视化管理工具**

多通道接入 · 多模型配置 · 技能中心 · 插件管理 · 版本管理 · 环境检测 · 多语言 · Docker / 原生安装

[![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-red?style=flat-square)](LICENSE)
[![Docker](https://img.shields.io/badge/docker-ready-brightgreen?style=flat-square&logo=docker&logoColor=white)](docker-compose.yml)
[![Version](https://img.shields.io/badge/version-4.4.0-violet?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Demo](https://img.shields.io/badge/demo-online-blue?style=flat-square&logo=vercel)](http://demo.zhaoxinyi.xyz)
[![GitHub Stars](https://img.shields.io/github/stars/zhaoxinyi02/ClawPanel?style=flat-square&logo=github)](https://github.com/zhaoxinyi02/ClawPanel/stargazers)

[在线演示](https://demo.zhaoxinyi.xyz) · [快速开始](#-快速开始) · [功能特性](#-主要功能) · [API 文档](docs/API.md) · [部署指南](docs/DEPLOYMENT.md) · [English](README_EN.md)

</div>

---

> [!CAUTION]
> **⚠️ 免责声明 | Disclaimer**
>
> 本项目仅供**学习研究**使用，**严禁用于任何商业用途**。使用第三方客户端登录 QQ/微信可能违反腾讯服务协议，**存在封号风险**，请使用小号测试。本项目作者**未进行任何逆向工程**，仅做开源项目整合，**不对任何后果承担责任**。下载使用即表示同意 [完整免责声明](DISCLAIMER.md)。
>
> This project is for **learning and research purposes only**. **Commercial use is strictly prohibited.** Use at your own risk. See [full disclaimer](DISCLAIMER.md).

> [!NOTE]
> **v4.4.0 AI 助手 · 模型兼容性修复 · UI 优化**：新增 AI 智能助手浮窗、修复非 OpenAI 模型（火山引擎/DeepSeek 等）的 `developer` 角色兼容性问题、技能中心布局优化。
> 旧版本请查看 [v3.0.0 Tag](https://github.com/zhaoxinyi02/ClawPanel/releases/tag/v3.0.0)。

> [!TIP]
> **在线演示**：访问 [demo.zhaoxinyi.xyz](https://demo.zhaoxinyi.xyz) 体验 ClawPanel 的完整功能（使用模拟数据，无需部署），登录密码为任意数字/字母。

## ✨ 主要功能

### 📊 智能仪表盘
动态展示已连接通道卡片、AI 模型状态、运行时间、内存占用、今日消息统计、最近活动流。**仅显示已连接的通道**，未连接的通道不再占位。

<img src="img/dashboard.png" width="700"/>

### 📋 活动日志
增强版日志系统：按来源筛选（QQ / Bot回复 / 微信 / 系统）、按类型筛选（文本 / 媒体 / 表情）、关键词搜索、一键导出 JSON。

<img src="img/activity-log.png" width="700"/>

### 📡 通道管理（20+ 通道）
支持 **20+ 种通道**的统一配置，分为内置通道和插件通道两大类：

**内置通道：**
- **QQ (NapCat)** — 扫码/快速/账密登录，唤醒概率、触发词、戳一戳回复、**QQ 审核请求**集成
- **微信** — 扫码登录，基于 wechatbot-webhook
- **Telegram** — Bot Token + Webhook
- **Discord** — Bot Token + Guild 配置
- **WhatsApp** — QR 扫码
- **Slack** — Socket Mode
- **Signal** — signal-cli REST API
- **Google Chat** — 服务账号 + Webhook

**插件通道：**
- **飞书 / Lark** · **钉钉** · **企业微信** · **QQ 官方 Bot** · **IRC** · **Mattermost** · **Microsoft Teams** · **LINE** · **Matrix** · **Twitch** · **BlueBubbles** · **WebChat**

> 已连接的通道自动置顶显示，QQ 审核请求直接集成在 QQ 通道配置页面中。

<img src="img/channels.png" width="700"/>

### ⚡ 技能中心 + 插件管理 + ClawHub 商店
- **技能列表**：从服务端实时扫描所有已安装技能（内置 + 工作区 + 应用内置），搜索筛选
- **插件管理**：独立的插件标签页，管理飞书/钉钉/企业微信/QQ官方Bot/NapCat QQ 等插件，一键启用/禁用
- **ClawHub 商店**：浏览 13+ 可安装技能，一键获取安装命令

<img src="img/skills.png" width="700"/>

### ⏰ 定时任务
从 `cron/jobs.json` 读取真实定时任务：Cron 表达式配置、启用/暂停/删除、运行状态追踪、消息内容编辑。

<img src="img/cronjobs.png" width="700"/>

### ⚙️ 系统配置（6 大模块）

全新 6 标签页深度配置：

#### 🧠 模型配置
多模型提供商管理，支持 **8+ 主流 AI 厂商**一键填充：
- OpenAI · Anthropic · Google · DeepSeek · NVIDIA NIM · Groq · Together AI · OpenRouter
- 每个提供商可配置 Base URL、API Key、API 类型、模型列表
- 快捷添加已知模型（GPT-4o、Claude Sonnet 4.5、Gemini 2.0 等）

<img src="img/config-models.png" width="700"/>

#### 👤 身份 & 消息
助手名称、头像、主题色、系统提示词、历史消息数、Agent 默认设置（上下文Token、最大输出、并发数、压缩模式）。

#### 🔧 通用配置
网关配置、工具配置（媒体理解/网页搜索）、Hooks、会话配置（自动压缩/修剪）、认证密钥（API Keys）、原始 JSON 查看器。

#### 📦 版本管理
- 当前版本 / 最新版本对比
- 更新检测与提示
- **一键更新 + 实时日志**：点击「立即更新」后实时显示更新进度、彩色日志、计时器，更新完成/失败一目了然
- **配置备份与恢复**：一键备份 openclaw.json，恢复前自动备份当前配置

<img src="img/config-version.png" width="700"/>

#### 🖥️ 环境检测
自动检测运行环境：
- 操作系统信息（平台、架构、内核、CPU、内存）
- 软件版本检测（Node.js、Docker、Git、OpenClaw、npm）
- 快速安装指南

<img src="img/config-env.png" width="700"/>

#### 📄 文档管理
浏览和编辑 OpenClaw 目录下的 Markdown 文档，内置编辑器，一键保存。

### 🤖 AI 智能助手
右下角浮窗式 AI 对话助手，支持拖拽移动、调整大小、最大化/还原，可选择不同模型提供商和模型进行实时对话。深色模式适配。

### 📁 工作区
文件浏览器：查看、编辑、创建 OpenClaw 工作区文件。

## 🏗️ 架构

```
┌──────────────────────────────────────────────────────┐
│                  Docker Compose                      │
│                                                      │
│  ┌─────────────────────────────────────────────────┐ │
│  │          openclaw-qq Container                  │ │
│  │  ┌─────────┐  ┌──────────┐  ┌───────────┐      │ │
│  │  │ NapCat  │  │ClawPanel │  │ Frontend  │      │ │
│  │  │  (QQ)   │←→│ Backend  │←→│ (React)   │      │ │
│  │  │  :6099  │  │  :6199   │  │           │      │ │
│  │  └─────────┘  └────┬─────┘  └───────────┘      │ │
│  └─────────────────────┼───────────────────────────┘ │
│                        │ HTTP callback               │
│  ┌─────────────────────┼───────────────────────────┐ │
│  │     openclaw-wechat Container                   │ │
│  │  ┌──────────────────┴──────────────────────┐    │ │
│  │  │   wechatbot-webhook (微信 Web 协议)      │    │ │
│  │  │   :3001 (内部) → :3002 (外部)            │    │ │
│  │  └─────────────────────────────────────────┘    │ │
│  └─────────────────────────────────────────────────┘ │
└──────────┬──────────────┬────────────────────────────┘
           │              │
      ┌────┴────┐    ┌────┴────┐
      │ OpenClaw│    │ Browser │
      │ Gateway │    │ClawPanel│
      └─────────┘    └─────────┘
```

## 🛠️ 技术栈

| 层级 | 技术 |
|:---|:---|
| QQ 协议 | [NapCat](https://github.com/NapNeko/NapCatQQ) (OneBot11 WebSocket) |
| 微信协议 | [wechatbot-webhook](https://github.com/danni-cool/wechatbot-webhook) (Web 微信) |
| 后端 | TypeScript · Express · WebSocket |
| 前端 | React · Vite · TailwindCSS · Lucide Icons |
| AI 引擎 | [OpenClaw](https://openclaw.ai) — 支持 GPT-4o / Claude / Gemini / DeepSeek 等 |
| 部署 | Docker Compose 双容器编排 |

## 🚀 快速开始

ClawPanel 提供两种安装方式：**原生安装**（推荐）和 **Docker 安装**。

### 方式一：原生安装（推荐）

#### Linux 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/install.sh | bash
```

安装过程中会提示输入管理密码、QQ 账号等信息。安装完成后自动创建 systemd 服务并启动。

#### Windows 安装

```powershell
irm https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/install.ps1 | iex
```

或下载 `install.ps1` 后运行：`powershell -ExecutionPolicy Bypass -File install.ps1`

#### macOS 安装

```bash
curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/install-mac.sh | bash
```

安装完成后自动创建 LaunchAgent 并在浏览器中打开管理面板。

> [!TIP]
> 原生安装会自动检测 OpenClaw 安装路径，创建系统服务，并在桌面创建快捷方式。

---

### 方式二：Docker 安装

#### 前提条件

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- [OpenClaw](https://openclaw.ai) 已安装

#### 1️⃣ 克隆 & 配置

```bash
git clone https://github.com/zhaoxinyi02/ClawPanel.git
cd ClawPanel
cp .env.example .env
```

编辑 `.env`：

```env
ADMIN_TOKEN=你的管理密码        # ClawPanel 登录密码
QQ_ACCOUNT=你的QQ号            # QQ 账号（可选，用于快速登录）
OWNER_QQ=主人QQ号              # 接收通知的 QQ 号
WECHAT_TOKEN=openclaw-wechat   # 微信 Webhook Token
OPENCLAW_DIR=~/.openclaw       # OpenClaw 配置目录
```

### 2️⃣ 启动

```bash
docker compose up -d
```

### 3️⃣ 配置 OpenClaw 频道

```bash
# Linux / macOS
chmod +x setup-openclaw.sh && ./setup-openclaw.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File setup-openclaw.ps1
```

### 4️⃣ 登录 & 使用

1. 浏览器访问 `http://你的服务器IP:6199`
2. 输入 `ADMIN_TOKEN` 登录
3. 左侧 **「通道管理」** → 选择 QQ → 扫码登录
4. 左侧 **「通道管理」** → 选择微信 → 扫码登录
5. 用另一个号给 Bot 发消息，收到 AI 回复即成功 🎉

<img src="img/login.png" width="400"/>

## 📡 端口说明

| 端口 | 服务 | 说明 |
|:---:|:---|:---|
| `6199` | ClawPanel | 主入口，浏览器访问 |
| `6099` | NapCat WebUI | QQ 协议管理（可选） |
| `3001` | OneBot11 WS | OpenClaw 连接 QQ 用 |
| `3002` | 微信 Webhook | 微信 API 调试用 |

## 🌍 跨平台部署

| 平台 | 启动命令 | 配置脚本 |
|:---:|:---|:---|
| 🐧 Linux | `docker compose up -d` | `./setup-openclaw.sh` |
| 🍎 macOS | `docker compose up -d` | `./setup-openclaw.sh` |
| 🪟 Windows | `docker compose up -d` | `powershell -File setup-openclaw.ps1` |

> [!TIP]
> Windows 用户需在 `.env` 中设置 `OPENCLAW_DIR=C:\Users\你的用户名\.openclaw`

## 📖 文档

| 文档 | 说明 |
|:---|:---|
| [API 接口文档](docs/API.md) | 完整的 REST API + WebSocket 接口说明 |
| [部署指南](docs/DEPLOYMENT.md) | 详细部署步骤、环境变量、防火墙、反向代理、故障排查 |

## ❓ 常见问题

<details>
<summary><b>QQ 扫码后提示登录失败？</b></summary>

确保 QQ 账号没有开启设备锁，或尝试使用快速登录。
</details>

<details>
<summary><b>微信扫码页面打不开？</b></summary>

确保微信容器已启动：`docker compose logs wechat`，且端口 3002 未被占用。
</details>

<details>
<summary><b>微信提示不支持网页版登录？</b></summary>

部分微信账号未开通网页版权限，需要使用较早注册的微信号。
</details>

<details>
<summary><b>OpenClaw 连接不上？</b></summary>

运行 `./setup-openclaw.sh` 重新配置，然后重启 OpenClaw。
</details>

<details>
<summary><b>如何查看日志？</b></summary>

```bash
docker compose logs -f          # 全部日志
docker compose logs -f wechat   # 仅微信容器
```
</details>

<details>
<summary><b>如何更新到最新版？</b></summary>

```bash
git pull && docker compose up -d --build
```
</details>

<details>
<summary><b>活动日志出现「管理后台启动」但我没有手动重启？</b></summary>

这通常是因为 Docker 容器自动重启导致的。常见原因：
1. **QQ 进程崩溃**：NapCat QQ 进程异常退出（如 SIGSEGV），v4.3.0 已修复此问题（QQ 进程崩溃后自动重启，不再重启整个容器）
2. **Docker 守护进程重启**：系统更新或 Docker 服务重启会导致容器重启
3. **内存不足（OOM）**：服务器内存不足时 Linux 内核会杀掉容器进程

查看容器重启原因：`docker inspect openclaw-qq --format='{{.State.ExitCode}} {{.State.OOMKilled}}'`
</details>

<details>
<summary><b>QQ 登录提示 Unauthorized？</b></summary>

确保 `.env` 中的 `WEBUI_TOKEN` 与 NapCat WebUI 配置一致。如果你修改了 `ADMIN_TOKEN` 但没有设置 `WEBUI_TOKEN`，两者会不一致导致认证失败。v4.3.0 已修复此问题。
</details>

<details>
<summary><b>技能中心只显示部分技能？</b></summary>

v4.3.0 之前的版本在 Docker 环境下无法正确扫描应用内置技能。请更新到 v4.3.0 并在 `.env` 中添加：
```env
OPENCLAW_APP=/path/to/openclaw/app  # OpenClaw 应用目录
```
</details>

## 📋 更新日志

### v4.4.0 — AI 助手 · 模型兼容性修复 · UI 优化 (2026-02-21)
- 🆕 **AI 智能助手**：右下角浮窗式 AI 对话助手，支持拖拽移动、调整大小、最大化/还原，可选择不同模型提供商和模型进行对话
- 🔧 **模型兼容性修复**：修复非 OpenAI 模型（火山引擎 Doubao、DeepSeek 等）因发送 `developer` 角色导致的 "Message ordering conflict" 错误，自动为非 OpenAI 提供商注入 `compat.supportsDeveloperRole=false` 并设置 `reasoning=false`
- 🔧 **pi-ai 运行时补丁**：直接修补 `@mariozechner/pi-ai` 库的 `openai-completions.js`，强制使用 `system` 角色替代 `developer`，确保所有 OpenAI 兼容 API 提供商正常工作
- 🔧 **技能中心布局优化**：技能和插件卡片的启用/禁用开关按钮始终可见，无需水平滚动
- 🔧 **会话自动清理**：配置保存和网关重启时自动清理旧会话文件，防止会话冲突
- 🎨 **AI 助手 UI**：支持最大化/还原切换、拖拽定位、边缘吸附、深色模式适配

### v4.3.0 — 技能插件分离 · 修改密码 · 多语言 · 安装器 (2026-02-19)
- 🆕 **技能/插件分离**：技能中心拆分为「技能」「插件」「ClawHub 商店」三个标签页，插件不再混入技能列表
- 🆕 **修改管理密码**：系统配置 → 通用 → 修改管理密码，修改后自动退出登录
- 🆕 **多语言支持**：支持中文（简体）和 English 切换，侧边栏底部一键切换
- 🆕 **在线演示站**：[demo.zhaoxinyi.xyz](https://demo.zhaoxinyi.xyz) 提供完整功能演示（模拟数据）
- 🆕 **原生安装方式**：提供 Linux 一键安装脚本、deb/rpm 包、Windows exe、macOS dmg，与 Docker 安装并存
- 🆕 **版本更新实时状态**：一键更新后实时显示终端日志、彩色状态指示、已用时间计时器，更新成功/失败一目了然
- 🆕 **更新守护脚本**：`update-watcher.sh` 监听容器更新信号，在宿主机执行 `openclaw update` 并回传日志
- 🔧 **技能扫描修复**：修复 Docker 环境下无法扫描应用内置技能的问题（新增 `OPENCLAW_APP` 环境变量）
- 🔧 **NapCat 认证修复**：修复 `ADMIN_TOKEN ≠ WEBUI_TOKEN` 时 QQ 登录 Unauthorized 的问题
- 🔧 **QQ 进程崩溃恢复**：QQ 进程崩溃后自动重启，不再导致整个容器重启
- 🔧 **NapCat 认证重试**：NapCat API 调用失败时自动重新认证并重试

### v4.2.1 — UI 美化 & 通道显示修复 (2026-02-17)
- 🎨 **全新紫罗兰主题**：统一的 Violet 色调设计，现代化卡片布局，深色/浅色模式全面适配
- 🎨 **专业图标**：全面替换 emoji 为 Lucide 专业图标库
- 🔧 **仪表盘通道修复**：显示所有已启用通道（QQ/飞书/QQBot/钉钉等），不再仅显示 QQ 和微信
- 🔧 **侧边栏通道修复**：运行状态区显示所有已启用通道，QQ 号码完整显示不再截断
- 🔧 **网关重启日志**：通道启用/禁用后自动轮询网关重启结果，记录成功/失败日志
- 🔧 **移除 QQ 群/好友卡片**：仪表盘不再显示冗余的 QQ 群/好友统计卡片
- 🆕 **Favicon 更新**：浏览器标签页图标更换为 ClawPanel Logo
- 🆕 **Apple Touch Icon**：添加 iOS 主屏幕图标支持
- 📝 **英文 README**：新增 [README_EN.md](README_EN.md)，方便国际用户阅读

### v4.2.0 — 通道管理增强 & Bug 修复 (2026-02-16)
- 🔧 **QQ 登录修复**：扫码登录、快速登录、账密登录按钮全部可用，二维码正常显示
- 🔧 **通道开关修复**：关闭通道后 OpenClaw 网关自动重启，配置即时生效
- 🔧 **QQ 退出登录**：关闭 QQ 通道时自动退出 QQ 登录（清除会话数据），重新开启需扫码
- 🔧 **WeChat 配置兼容**：修复 `channels.wechat: unknown channel id` 错误（#9），微信配置不再写入 openclaw.json
- 🔧 **tools/session 兼容**：修复 OpenClaw 2026.2.14+ 不支持 tools/session 顶层键导致的配置报错（#10）
- 🔧 **通知开关**：QQ 通道配置中新增防撤回、成员变动、管理员变动、禁言、戳一戳、荣誉等通知开关
- 🆕 **活跃通道状态**：仪表盘和侧边栏根据通道启用状态 + 连接状态动态显示，关闭的通道不再显示为活跃
- 🆕 **活动日志增强**：通道开启/关闭、QQ 登录/退出、网关重启等操作自动记录系统日志
- 🆕 **QQ 未登录状态**：QQ 通道启用但未登录时显示占位信息和登录按钮
- 🆕 **退出登录按钮**：QQ 通道新增「退出登录」按钮，一键退出 QQ 账号
- 🆕 **环境检测增强**：准确检测宿主机 OS 版本、Docker/Git/Bun 版本
- 🆕 **一键更新**：版本管理支持一键更新 OpenClaw，显示更新进度
- 🔧 **定时任务修复**：修复定时任务无法删除的问题
- 🔧 **autoSetup 修复**：容器重启不再覆盖用户手动禁用的 QQ 通道配置

### v4.1.0 — 全面功能扩展 (2026-02-14)
- 🆕 **通道管理**：新增 20+ 通道支持（飞书/钉钉/企业微信/QQ官方Bot/IRC/Teams/LINE/Matrix/Twitch 等）
- 🆕 **通道管理**：已连接通道自动置顶，QQ 审核请求集成到 QQ 通道页面
- 🆕 **技能中心**：从服务端实时扫描已安装插件，ClawHub 商店列出 13+ 可安装技能
- 🆕 **系统配置**：全新 6 标签页（模型/身份消息/通用/版本/环境/文档）
- 🆕 **多模型配置**：支持 8+ AI 厂商（OpenAI/Anthropic/Google/DeepSeek/Groq/OpenRouter 等）
- 🆕 **版本管理**：当前版本/最新版本对比、配置备份与恢复
- 🆕 **环境检测**：自动检测 OS/软件版本，提供安装指南
- 🆕 **文档管理**：浏览和编辑 OpenClaw 目录下的 MD 文档
- 🆕 **定时任务**：从 cron/jobs.json 读取真实任务数据
- 🆕 **新增 API**：`/system/env`、`/system/version`、`/system/backup`、`/system/skills` 等 10 个端点
- 🧹 清理废弃页面（QQ.tsx/QQBot.tsx/QQLogin.tsx/WeChatLogin.tsx/Settings.tsx/OpenClawConfig.tsx/Requests.tsx）
- 🧹 清理测试脚本和旧截图

### v4.0.0 — ClawPanel 智能管理面板 (2026-02-13)
- 🆕 全新品牌：从 `openclaw-im-manager` 升级为 **ClawPanel**
- 🆕 **仪表盘**：活跃通道、AI 模型、运行时间、内存、今日消息统计
- 🆕 **活动日志**：来源/类型筛选、关键词搜索、日志导出
- 🆕 **通道管理**：8 种通道统一配置（QQ/TG/Discord/微信/WhatsApp/Slack/Signal/Google Chat）
- 🆕 **技能中心**：技能列表、启用/禁用、ClawHub 商店入口
- 🆕 **定时任务**：创建/编辑/删除/暂停、Cron 表达式、运行状态
- 🆕 **系统配置**：9 大分组可视化编辑（模型/身份/消息/工具/网关/Hooks/会话/浏览器/密钥）
- 🎨 全新 UI：紫色渐变品牌、导航分组、深色/浅色主题

### v3.0.0 — QQ + 微信双通道 (2026-02-10)
- 🆕 新增微信个人号接入（基于 wechatbot-webhook）
- 🆕 管理后台新增微信登录页面、微信状态显示
- 🆕 仪表盘支持 QQ + 微信双通道实时事件
- 🌍 跨平台支持（Linux / macOS / Windows）
- 📝 项目更名：`openclaw-qq-plugin` → `openclaw-im-manager`

### v2.0.0 — 全新管理后台 (2026-02-09)
- 💻 全新管理后台 UI（React + TailwindCSS）
- 🔐 集成 QQ 登录（扫码 / 快速 / 账密）
- 🔌 内置 OneBot WS 代理
- 💾 QQ 登录 session 持久化

### v1.0.0 — 初始版本
- 🐳 基础管理后台 + NapCat Docker 集成

## ❤️ 致谢

本项目的实现离不开以下优秀的开源项目：

- [NapNeko/NapCatQQ](https://github.com/NapNeko/NapCatQQ) — QQ 协议框架
- [danni-cool/wechatbot-webhook](https://github.com/danni-cool/wechatbot-webhook) — 微信 Webhook 机器人
- [OpenClaw](https://openclaw.ai) — AI 助手引擎

## ⚠️ 免责声明

> **本项目仅供学习研究使用，严禁商用。**

- 🚫 **严禁商用** — 不得用于任何商业目的、付费服务、收费机器人等
- ⚠️ **封号风险** — 使用第三方客户端登录 QQ/微信可能导致账号被封禁
- 🔒 **无逆向** — 本项目未进行任何逆向工程，仅整合已有开源项目
- 📋 **自担风险** — 使用者需自行承担一切风险和法律责任
- 💰 **作者未盈利** — 本项目作者未从中获取任何经济收益

**详细免责声明请阅读 [DISCLAIMER.md](DISCLAIMER.md)**

## 📄 License

[CC BY-NC-SA 4.0](LICENSE) © 2026 — **禁止商用**

本项目采用 [知识共享 署名-非商业性使用-相同方式共享 4.0 国际许可证](https://creativecommons.org/licenses/by-nc-sa/4.0/deed.zh-hans) 进行许可。
