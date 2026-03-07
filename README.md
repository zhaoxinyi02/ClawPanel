<div align="center">

<img src="img/logo.jpg" width="700"/>

# ClawPanel

**OpenClaw 智能管理面板 — 单文件部署、跨平台、全功能可视化管理（含多智能体控制台）**

Go 单二进制 · React 18 · TailwindCSS · SQLite · WebSocket 实时推送 · 跨平台

[![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-red?style=flat-square)](LICENSE)
[![Version](https://img.shields.io/badge/version-5.1.6-violet?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/react-18-61DAFB?style=flat-square&logo=react&logoColor=white)](https://react.dev)
[![CI](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml/badge.svg)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml)
[![Release Build](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/release.yml/badge.svg)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/release.yml)
[![GitHub Stars](https://img.shields.io/github/stars/zhaoxinyi02/ClawPanel?style=flat-square&logo=github)](https://github.com/zhaoxinyi02/ClawPanel/stargazers)

[快速开始](#-快速开始) · [功能特性](#-主要功能) · [更新日志](changelogs/) · [API 文档](docs/API.md) · [English](README_EN.md)

</div>

---

> [!CAUTION]
> **免责声明 | Disclaimer**
>
> 本项目仅供**学习研究**使用，**严禁用于任何商业用途**。使用第三方客户端登录 QQ/微信可能违反腾讯服务协议，**存在封号风险**，请使用小号测试。本项目作者**未进行任何逆向工程**，仅做开源项目整合，**不对任何后果承担责任**。下载使用即表示同意 [完整免责声明](DISCLAIMER.md)。
>
> This project is for **learning and research purposes only**. **Commercial use is strictly prohibited.** Use at your own risk. See [full disclaimer](DISCLAIMER.md).
### 💬 社区交流
欢迎加入 **ClawPanel 官方交流群**，获取最新更新、反馈问题、参与插件开发。

> 📱 **扫码加入企微群**
> 
> <img src="img/wecom.jpg" width="300"/>
> 



## 主要功能

### 多智能体控制台（v5.1.x 重点更新）
- Agent 列表管理：新建 / 编辑 / 删除，支持默认 Agent 设置
- `Core Files`：可直接查看和保存 Agent 工作区核心文件
- `Skills · Channels · Cron`：从单 Agent 视角查看技能、通道和定时任务快照
- `Recent Sessions`：快速巡检当前 Agent 最近活跃会话
- Bindings 路由规则管理：支持**结构化表单 + JSON 高级模式**，可启停、重排和规则级错误定位
- 路由预览器：输入 `channel/sender/peer/parentPeer/guildId/teamId/accountId/roles` 快速验证命中 Agent

### 智能仪表盘
- OpenClaw 进程状态监控（启动/停止/重启）
- 已启用通道概览、当前模型、运行时间、内存占用
- 快捷操作：一键重启 OpenClaw / 网关 / ClawPanel / NapCat

### 通道管理（20+ 通道）
支持 **20+ 种通道**的统一配置和一键启用/禁用：
- **内置通道**：QQ (NapCat) · 微信 · Telegram · Discord · WhatsApp · Slack · Signal · Google Chat · BlueBubbles · WebChat
- **插件通道**：飞书 · 钉钉 · 企业微信 · QQ 官方 Bot · IRC · Mattermost · Teams · LINE · Matrix · Twitch
- **QQ 登录**：扫码 / 快速 / 密码三种方式，支持退出登录和重启 NapCat 容器
- **QR 码智能刷新**：自动检测过期二维码，重试获取全新二维码

### 配置中心
- **模型配置**：多提供商管理（OpenAI / Anthropic / Google / DeepSeek / 火山引擎等）
- **Agent 配置**：系统提示词、温度、最大 Token 数
- **JSON 模式**：直接编辑完整配置 JSON（保存前差异预览）
- 自动为非 OpenAI 提供商注入 `compat.supportsDeveloperRole=false` 兼容性修复
- `openclaw.json` 写入前自动快照备份（`backups/pre-edit-*.json`）

### 技能中心 + 插件管理
- 技能/插件分离展示，搜索筛选
- 一键启用/禁用，实时扫描已安装技能（内置 + 工作区 + 应用）

### 插件中心
- **插件市场**：浏览官方/第三方插件列表，按分类筛选（基础、AI 增强、消息处理、娱乐、工具）
- **一键操作**：安装 / 卸载 / 更新 / 启用 / 禁用插件，无需重启 OpenClaw
- **可视化配置**：自动读取插件 `plugin.schema.json`，前端动态生成配置表单
- **插件日志**：实时查看插件运行日志，便于调试
- **多来源安装**：支持从官方仓库、GitHub/Gitee 仓库、本地目录安装
- **完善的开发文档**：详细的 [插件开发指南](docs/plugin-dev/README.md)、JSON Schema 规范、示例插件
- **插件冲突检测**：安装前检查 ID、端口、依赖冲突

### 配置自动检测 + 一键修复
- 自动扫描 OpenClaw/NapCat 核心配置文件，检测常见错误
- 检测项：`reportSelfMessage`、WS/HTTP 服务状态、端口、Token、模型 API Key 等
- 前端可视化展示异常项，支持单项修复和「一键修复全部」
- 修复后自动重启对应进程，配置立即生效

### NapCat 掉线自动重连 + 告警
- 每 30 秒检测 NapCat 连接状态（容器/WS/HTTP/QQ 登录）
- 连续离线自动重启（Docker 或 Windows 原生进程），可配置重连次数上限
- **Windows 原生支持**：Windows 用户安装 NapCat 时自动使用 Shell 版（无需 Docker）
- 通道管理页面实时状态面板：在线/重连中/登录失效/离线指示灯
- 手动重连按钮 + 重连日志查看
- 状态变化通过 WebSocket 实时推送

### 事件日志
- 实时消息流：QQ 消息、**Bot 回复**、系统事件
- 按来源/类型筛选、关键词搜索
- SQLite 持久化存储，重启不丢失
- 外部服务日志接入（POST /api/events）

### 系统管理
- 系统环境检测（OS / CPU / Go / OpenClaw 版本）
- 配置备份与恢复（自动备份当前配置再恢复）
- 软件安装中心：一键安装 Docker、NapCat、微信机器人等
- 消息中心：安装任务进度实时追踪
- 身份文档编辑（IDENTITY.md / USER.md 等）
- 管理密码修改、版本更新检查

### AI 智能助手
内置 AI 对话助手浮窗，支持多提供商/多模型切换，自动使用 OpenClaw 配置的 API。

### 面板一键自检更新
- 基于北京服务器国内加速，解决 GitHub 下载不稳定问题
- 检查更新 → 下载 → SHA256 校验 → 替换程序 → 自动重启，全自动化
- **独立更新工具**：更新进程通过 `systemd-run --scope` 隔离运行，主进程停止后更新不中断
- 更新成功后弹窗展示版本号和更新内容
- 自动备份旧程序（`.bak`），SHA256 校验防损坏
- 支持离线更新：上传本地二进制文件直接替换

### OpenClaw 可视化更新
- 点击「前往更新」跳转独立更新页面，可视化执行 `openclaw update`
- 实时显示更新步骤、进度条、命令输出日志
- 自动检测当前版本和 npm 最新版本
- 更新完成后自动发送网关重启信号

## ❓ 常见问题 & 遇到问题怎么办

> [!TIP]
> **遇到问题请先在面板中使用 AI 助手（右下角对话图标）提问！** AI 助手已内置完整的 FAQ 知识库，能快速帮你排查和解决问题。

常见问题速查：

| 问题 | 简要解答 |
|:---|:---|
| 安装后 `systemctl start` 需要密码 | 需要 sudo 权限，输入 **Linux 系统密码**（不是面板密码） |
| 面板默认登录密码 | `clawpanel`，首次登录后建议修改 |
| 访问面板显示空白 / 无法连接 | 检查服务状态、防火墙放行 19527 端口、云安全组 |
| macOS 安装报错 "无法验证开发者" | 运行 `sudo xattr -d com.apple.quarantine /opt/clawpanel/clawpanel` |
| 检查更新显示"服务器错误" | 确保服务器能访问 `39.102.53.188:16198`，检查出站防火墙 |
| OpenClaw 版本显示 unknown | 建议通过 npm 安装：`npm i -g openclaw@latest` |
| 如何卸载 ClawPanel | `sudo systemctl stop clawpanel && sudo rm -rf /opt/clawpanel` |

👉 **完整 FAQ 文档**：[docs/FAQ.md](docs/FAQ.md)

## 架构

```
┌──────────────────────────────────────────────┐
│            ClawPanel (单二进制)                │
│                                              │
│  ┌───────────┐  ┌────────────┐  ┌─────────┐ │
│  │  Go 后端  │  │ React 前端 │  │ SQLite  │ │
│  │  (Gin)    │←→│ (go:embed) │  │   DB    │ │
│  │  :19527   │  │            │  │         │ │
│  └─────┬─────┘  └────────────┘  └─────────┘ │
│        │                                     │
│  ┌─────┴──────┐  ┌────────────┐  ┌────────┐│
│  │  Process   │  │ WebSocket  │  │Updater ││
│  │  Manager   │  │    Hub     │  │ :19528 ││
│  └─────┬──────┘  └──────┬─────┘  └────────┘│
└────────┼────────────────┼────────────────────┘
         │                │
    ┌────┴────┐    ┌──────┴──────┐
    │ OpenClaw│    │ NapCat/微信 │
    │ Process │    │ Docker 容器 │
    └─────────┘    └─────────────┘
```

## 技术栈

| 层级 | 技术 |
|:---|:---|
| 后端 | Go 1.22+ · Gin · SQLite (modernc.org/sqlite) · gorilla/websocket · golang-jwt |
| 前端 | React 18 · TypeScript · TailwindCSS · Lucide Icons · Vite |
| 部署 | 单二进制 · `go:embed` 内嵌前端 · 跨平台静态编译 (`CGO_ENABLED=0`) |
| AI 引擎 | [OpenClaw](https://openclaw.ai) — 支持 GPT-4o / Claude / Gemini / DeepSeek 等 |

## 快速开始

> 跟宝塔面板一样，一条命令搞定安装，自动注册系统服务、开机自启动、配置防火墙。

### 方式一：一键安装（推荐）

**Linux / macOS**

```bash
curl -sSO https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.sh && sudo bash install.sh
```

自动完成：下载二进制 → 安装到 `/opt/clawpanel` → 注册系统服务 → 开机自启动 → 配置防火墙 → 启动。
完整安装流程请查看在线文档：[QQ NapCat个人号安装教程](https://doc.weixin.qq.com/doc/w3_AdsADQa9AEcCNNqyBMQ3oQmWG451V?scode=AFoAZAcoAHMV6Hv1R5AdsADQa9AEc)

**Windows（PowerShell 管理员）**

```powershell
irm https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.ps1 | iex
```

> [!NOTE]
> 安装脚本兼容 **PowerShell 5.1 及以上**版本（Windows 自带版本即可）。脚本会自动从 GitHub 获取最新版本，无需手动指定版本号。

或从 [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) 手动下载 `clawpanel-windows-amd64.exe`，双击或命令行运行。

### 方式二：手动下载运行

从 [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) 下载对应平台的二进制文件：

```bash
# Linux
chmod +x clawpanel-linux-amd64 && ./clawpanel-linux-amd64

# macOS
chmod +x clawpanel-darwin-arm64 && ./clawpanel-darwin-arm64

# Windows (双击或命令行)
clawpanel-windows-amd64.exe
```

启动后访问 `http://localhost:19527`，默认密码 `clawpanel`。

> [!WARNING]
> 手动运行不会注册系统服务，关闭终端后服务会停止。推荐使用一键安装。

### 方式三：从源码构建

```bash
git clone https://github.com/zhaoxinyi02/ClawPanel.git
cd ClawPanel
make build        # 构建当前平台
make cross        # 交叉编译所有平台
make installer    # 构建 Windows exe 安装包
./bin/clawpanel
```

> [!TIP]
> 构建需要 Go 1.22+ 和 Node.js 18+。中国大陆用户请设置：
> ```bash
> export GOPROXY=https://goproxy.cn,direct
> npm config set registry https://registry.npmmirror.com
> ```

## GitHub Actions 自动化

已内置两条工作流，覆盖自动测试与自动打包发布：

- `CI`（`.github/workflows/ci.yml`）
  - 触发：`push` / `pull_request` / 手动触发
  - 执行：
    - `go vet ./...`
    - `go test -count=1 -shuffle=on ./...`（`ubuntu/windows` 矩阵）
    - `go test -race -covermode=atomic -coverprofile=coverage.out ./...`
    - `web npm ci + npm run build`
    - 后端嵌入前端产物构建（`make backend-only`）
  - 产物：
    - `go-coverage`（`coverage.out` + `coverage.txt`）
    - `frontend-dist`
    - `clawpanel-linux-amd64-ci`（用于快速验收）
- `Release Build`（`.github/workflows/release.yml`）
  - 触发：`push tag v*`（如 `v5.1.6`）/ 手动触发
  - 执行：自动构建 `linux/darwin/windows` 多平台二进制 + `ClawPanel-Setup-v{version}.exe`
  - 发布：tag 触发时自动上传到 GitHub Release，并生成 `checksums.txt`

另外，已启用 `Dependabot`（`.github/dependabot.yml`）每周自动检查 GitHub Actions 依赖更新。

示例：

```bash
git tag v5.1.6
git push origin v5.1.6
```

## 环境变量

| 变量 | 默认值 | 说明 |
|:---|:---|:---|
| `CLAWPANEL_PORT` | `19527` | Web 服务端口 |
| `CLAWPANEL_DATA` | `./data` | 数据目录（配置 + 数据库） |
| `OPENCLAW_DIR` | `~/.openclaw` | OpenClaw 配置目录 |
| `OPENCLAW_CONFIG` | - | OpenClaw 配置文件路径（自动推导目录） |
| `OPENCLAW_APP` | - | OpenClaw 应用目录（用于技能扫描） |
| `OPENCLAW_WORK` | - | OpenClaw 工作目录 |
| `CLAWPANEL_SECRET` | 随机 | JWT 签名密钥 |
| `ADMIN_TOKEN` | `clawpanel` | 管理密码 |
| `CLAWPANEL_DEBUG` | `false` | 调试模式 |
| `LEGACY_SINGLE_AGENT` | `false` | 启用后退回单 Agent 兼容模式（隐藏多智能体写操作） |

## 服务管理

```bash
# systemd (Linux)
systemctl start clawpanel
systemctl stop clawpanel
systemctl restart clawpanel
systemctl status clawpanel
journalctl -u clawpanel -f

# Windows 服务
sc start ClawPanel
sc stop ClawPanel
sc query ClawPanel
```

## 跨平台支持

| 平台 | 架构 | 二进制文件 |
|:---:|:---:|:---|
| Linux | x86_64 | `clawpanel-linux-amd64` |
| Linux | ARM64 | `clawpanel-linux-arm64` |
| macOS | x86_64 | `clawpanel-darwin-amd64` |
| macOS | ARM64 (M1/M2/M3) | `clawpanel-darwin-arm64` |
| Windows | x86_64 | `clawpanel-windows-amd64.exe` |

## 更新日志

完整更新日志请查看 [changelogs/](changelogs/) 目录。

### v5.1.6 — 修复 QQ 插件安装链路与 NapCat 状态误判
- **🐛 QQ 插件安装前置**：安装 `QQ (NapCat)` 时会先确保 `qq` 插件安装成功，失败则直接报错终止，不再继续写坏 `openclaw.json`
- **🧩 插件缺失提示更明确**：QQ 通道页和系统环境页会明确显示 `QQ 个人号插件未安装 / 缺少插件`，不再把 NapCat 误判成正常已安装
- **🔄 QQ 开关即时生效**：保存 QQ 配置后自动重启 OpenClaw 网关，撤回通知、成员变动、欢迎语、戳一戳等开关立即生效
- **🛒 插件中心兜底拉取仓库**：插件列表为空时会主动刷新 registry，降低“插件商店打不开”的概率

### v5.1.5 — Agent 核心文件、网关探测增强与飞书切换修复
- **🧠 Agent 工作区增强**：新增 `Core Files`、`Skills · Channels · Cron`、`Recent Sessions` 等工作台能力，更接近官方单 Agent 管理体验
- **🛡️ 网关与启动稳定性提升**：启动时可识别外部管理中的 OpenClaw，按 `gateway.bind` 精确探测端口，减少误判和重复拉起
- **🪟 Windows CI 修复**：修正 `launch_windows.go` 中的 Windows `unsafe.Pointer` / 环境块处理问题，恢复 `windows-2025` CI 绿灯
- **🔀 飞书双版本切换收口**：切换前校验目标插件是否已安装，前端未安装版本禁用切换，避免切到不可用实现

### v5.0.24 — 修复 QQ 唤醒配置不生效（Issue #21）
- **🐛 修复 QQ 唤醒配置不生效**：修复 `wakeProbability`、`wakeTrigger` 等配置仅在面板展示但未落到插件实际生效路径的问题
- **✨ 兼容旧配置字段**：后端保存 QQ 通道配置时自动迁移旧字段（`wakeProbability`/`wakeTrigger`/`minSendIntervalMs`/`autoApprove*`）到新版结构，避免升级后失效

### v5.0.14 — 脚本修复 · Windows 兼容性
- **🐛 install.ps1 兼容 PowerShell 5.1**：移除三元运算符，Windows 自带 PS 5.1 可直接运行
- **✨ 安装脚本自动获取最新版本**：`install.sh` / `install.ps1` 启动时自动从 GitHub API 拉取最新版本号，无需随每次发版更新脚本
- **🐛 Windows 安装 OpenClaw 不再因缺少 Node.js 退出**：自动通过 winget 安装 Node.js LTS 后继续

### v5.0.13 — 修复 QQ 插件强制注入
- **🐛 修复 QQ 插件配置强制写入**：仅在 QQ 插件已安装且 NapCat 运行时才注入 `channels.qq`，避免未使用 QQ 的用户 OpenClaw 网关启动失败

### v5.0.12 — OpenClaw 可视化更新 · 版本显示修复
- **✨ OpenClaw 可视化更新界面**：实时显示更新步骤、进度条、命令输出日志
- **🐛 版本更新页显示旧版本**：`GetVersion` API 改用 `openclaw --version` 获取真实版本，不再读取 `lastTouchedVersion`
- **🐛 ClawPanel 版本号显示错误**：版本号改为 ldflags 动态注入，修复硬编码导致显示错误
- **✨ OpenClaw 网关 daemon fork 模式检测**：正确识别 daemon fork 启动，防止误判崩溃

### v5.0.3 — 插件中心 · Windows NapCat · 跨平台修复
- **🧩 插件中心**：全新插件市场页面，支持浏览/安装/卸载/配置/更新/启用/禁用插件
- **插件后端**：完整的插件生命周期管理 API（`/api/plugins/*`）
- **插件开发文档**：详细的开发规范、JSON Schema、PluginContext API、示例插件
- **Windows NapCat 原生安装**：Windows 用户安装 NapCat 使用 Shell 版，无需 Docker
- **跨平台安装脚本**：所有软件安装脚本支持 macOS (Homebrew) / CentOS (yum) / Ubuntu (apt)
- **反斜杠转义修复**：修复 `RunScriptWithSudo` 中单引号嵌套导致的脚本执行失败（Issue #17）
- **macOS 部署优化**：修复更新后重启逻辑（launchctl）、PATH 加入 Homebrew 路径（Issue #18）
- **NapCat 监控增强**：Windows 平台使用 tasklist/taskkill 管理 NapCat 进程

### v5.0.1 — 配置检测 · 连接监控 · 活动日志增强 (2026-02-24)
- **配置自动检测 + 一键修复**：扫描 OpenClaw/NapCat 配置，检测常见错误并一键修复
- **NapCat 掉线自动重连**：实时监控连接状态，自动重连，前端状态面板
- **活动日志增强**：Bot 回复消息正确显示（支持 `message_sent` 事件）
- **Bug 修复**：entrypoint 覆写配置、清除按钮无效、监控误触发重启等

### v5.0.0 — 全栈重写 (2026-02-22)
- **全栈重写**：后端 Node.js → Go (Gin)，前端 React 18 + TailwindCSS
- **单文件部署**：单个静态编译二进制，内嵌前端，无需 Node.js/Docker
- **跨平台**：Linux / Windows / macOS (x86_64 / arm64)
- **SQLite 持久化**：事件日志和配置使用 SQLite 存储
- **WebSocket 实时推送**：进程日志和消息事件实时推送
- **进程管理器**：内置 OpenClaw 进程管理（启动/停止/重启/监控）
- **AI 智能助手**：内置多模型 AI 对话浮窗
- **软件安装中心**：一键安装 Docker、NapCat、微信机器人
- **快捷重启**：一键重启 OpenClaw / 网关 / ClawPanel / NapCat
- **QR 码修复**：智能刷新机制，解决过期二维码问题
- **活动日志增强**：显示 Bot 回复消息，持久化存储
- **原生安装脚本**：Linux/macOS/Windows 一键安装 + 系统服务注册

<details>
<summary><b>v4.x 及更早版本</b></summary>

- **v4.4.0** (2026-02-21) — AI 助手、模型兼容性修复
- **v4.3.0** (2026-02-19) — 技能插件分离、修改密码、多语言、原生安装脚本
- **v4.2.x** (2026-02-16~17) — 紫罗兰主题、通道显示修复、QQ 登录修复
- **v4.1.0** (2026-02-14) — 20+ 通道、技能中心、6 标签页系统配置
- **v4.0.0** (2026-02-13) — ClawPanel 品牌升级
- **v3.0.0** (2026-02-10) — QQ + 微信双通道
- **v2.0.0** (2026-02-09) — React + TailwindCSS 管理后台
- **v1.0.0** — 基础管理后台 + NapCat Docker 集成
</details>

## 致谢

### 开发者与贡献者

<p>
  <a href="https://github.com/zhaoxinyi02"><img src="https://avatars.githubusercontent.com/u/98445030?v=4" width="64" height="64" alt="zhaoxinyi02" /></a>
  <a href="https://github.com/BlueSkyXN"><img src="https://avatars.githubusercontent.com/u/63384277?v=4" width="64" height="64" alt="BlueSkyXN" /></a>
  <a href="https://github.com/Hns16"><img src="https://avatars.githubusercontent.com/u/192765150?v=4" width="64" height="64" alt="Hns16" /></a>
  <a href="https://github.com/codeKing6412"><img src="https://avatars.githubusercontent.com/u/185812512?v=4" width="64" height="64" alt="codeKing6412" /></a>
</p>

<p>
  <a href="https://github.com/zhaoxinyi02">zhaoxinyi02</a> ·
  <a href="https://github.com/BlueSkyXN">BlueSkyXN</a> ·
  <a href="https://github.com/Hns16">Hns16</a> ·
  <a href="https://github.com/codeKing6412">codeKing6412</a>
</p>

- 感谢所有为 `ClawPanel` 提交代码、反馈问题和参与设计讨论的开发者与贡献者

- [OpenClaw](https://openclaw.ai) — AI 助手引擎
- [Gin](https://github.com/gin-gonic/gin) — Go Web 框架
- [NapCat](https://github.com/NapNeko/NapCatQQ) — QQ 协议框架
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — 纯 Go SQLite 驱动
- [Lucide](https://lucide.dev) — 图标库

## 免责声明

> **本项目仅供学习研究使用，严禁商用。**

- **严禁商用** — 不得用于任何商业目的
- **封号风险** — 使用第三方客户端登录 QQ/微信可能导致账号被封禁
- **无逆向** — 本项目未进行任何逆向工程
- **自担风险** — 使用者需自行承担一切风险和法律责任

**详细免责声明请阅读 [DISCLAIMER.md](DISCLAIMER.md)**

## License

[CC BY-NC-SA 4.0](LICENSE) © 2026 — **禁止商用**

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=zhaoxinyi02/ClawPanel&type=date&legend=top-left)](https://www.star-history.com/#zhaoxinyi02/ClawPanel&type=date&legend=top-left)
