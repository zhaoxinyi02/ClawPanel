<div align="center">

<img src="img/logo.jpg" width="700"/>

# ClawPanel

**OpenClaw 智能管理面板 — 单一标准版发行，专注轻量面板接管与外部运行时管理**

Go 单二进制 · React 18 · TailwindCSS · SQLite · WebSocket 实时推送 · 跨平台

[![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-red?style=flat-square)](LICENSE)
[![Release](https://img.shields.io/badge/Release-5.5.1-violet?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Go](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/react-18-61DAFB?style=flat-square&logo=react&logoColor=white)](https://react.dev)
[![CI](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml/badge.svg)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml)
[![GitHub Stars](https://img.shields.io/github/stars/zhaoxinyi02/ClawPanel?style=flat-square&logo=github)](https://github.com/zhaoxinyi02/ClawPanel/stargazers)

[快速开始](#快速开始) · [功能特性](#主要功能) · [更新日志](#更新日志) · [API 文档](docs/API.md) · [English](README_EN.md)

</div>

---

> [!CAUTION]
> **免责声明 | Disclaimer**
>
> 本项目仅供**学习研究**使用，**严禁用于任何商业用途**。使用第三方客户端登录 QQ/微信可能违反腾讯服务协议，**存在封号风险**，请使用小号测试。本项目作者**未进行任何逆向工程**，仅做开源项目整合，**不对任何后果承担责任**。下载使用即表示同意 [完整免责声明](DISCLAIMER.md)。
>
> This project is for **learning and research purposes only**. **Commercial use is strictly prohibited.** Use at your own risk. See [full disclaimer](DISCLAIMER.md).

> [!NOTE]
> **最新发布** — `ClawPanel v5.5.1` 已发布；本次重点修复 OpenClaw 新版配置 schema 兼容问题，保护梦境模式配置，并完成 Lite 发行线退场后的 GitHub-only 安装/更新链路。详情见 [changelogs/v5.5.1.md](changelogs/v5.5.1.md)。

> [!IMPORTANT]
> **Lite 发行线已停止维护。** ClawPanel 从现在开始只发布标准版，不再内置 OpenClaw runtime，也不再提供 Lite 构建、安装、更新或打包脚本。历史 Lite 版本仍保留在旧 Release / changelog 中，供已安装用户查阅与迁移。




## 主要功能

### 标准版发行与运行时接管

- ClawPanel 现在只发布标准版：面板保持轻量，不再把 OpenClaw runtime 打进安装包。
- OpenClaw、NapCat、插件和通道运行时均作为外部资源管理：可由面板引导安装，也可接管已有网关。
- 安装、更新和发布资产统一走 GitHub Releases，脚本不再依赖旧自建镜像或第三方同步源。

### OpenClaw 管理中枢

- 仪表盘展示 OpenClaw、网关、通道、模型、消息、任务和系统资源状态。
- 支持启动、停止、重启 OpenClaw / 网关 / ClawPanel / NapCat，并识别外部托管中的网关。
- 系统配置提供 OpenClaw 版本检查、可视化更新、配置备份恢复、环境检测和健康诊断。
- 写入 `openclaw.json` 前自动清理新版 schema 不接受的 legacy 字段，保留模型、Dreaming、浏览器等关键配置。

### 模型、身份与浏览器配置

- 模型提供商管理支持 OpenAI、Anthropic、Google Gemini、DeepSeek、火山方舟、通义千问、OpenRouter、Ollama 等。
- Provider 卡片支持快速添加、重命名、Headers、API 类型、模型列表和兼容性字段配置。
- 身份文档支持 `IDENTITY.md`、`USER.md` 等核心文件在线编辑，适配多 Agent 工作区。
- 浏览器控制提供托管 `openclaw` profile 与禁用浏览器控制两种安全预设，降低误接管系统浏览器的风险。

### 多智能体与路由

- Agent 工作台支持创建、编辑、删除、设为默认、核心文件编辑、技能/通道/Cron 快照和最近会话查看。
- Binding 路由规则支持结构化表单和 JSON 高级模式，可启停、重排并定位规则级错误。
- 路由预览器可输入 channel、sender、peer、guild、team、account、roles 等上下文，直接验证命中 Agent。
- Monitor 拓扑图可视化展示 Channel → Agent、Agent → Agent、默认回退等关系，支持缩放、拖拽和详情查看。

### 通道与平台管理

- 通道页统一管理 QQ、微信、飞书、钉钉、企业微信、Telegram、Discord、WhatsApp、Slack、Matrix、LINE、Twitch 等 20+ 通道。
- QQ 支持 NapCat Docker / Windows Shell 运行形态，提供登录状态、二维码、OneBot 配置、Token 校验和掉线重连。
- 飞书、企业微信、QQ 官方 Bot 等插件型通道支持版本切换、安装检测、保存校验和网关重启提示。
- Hermes 控制台提供平台配置、健康检查、会话、任务、日志、Profile 和动作管理，适合逐步迁移或双栈试用。

### Workflow Center 与 AI Company

- Workflow Center 支持模板管理、AI 生成模板、可视化画布、事件流、步骤详情和运行实例控制。
- 节点覆盖 `input`、`wait_user`、`approval`、`ai_plan`、`ai_task`、`analyze`、`summary`、`publish`、`image_generate`、`skill` 等常用流程。
- 复杂任务可接管原会话，支持确认、进度回写、暂停、恢复、重试、审批、继续和文件/图片产物回传。
- AI Company 提供公司概览、团队管理、任务列表与任务详情，用多角色方式拆解、执行、审核和回写任务。

### Panel Chat、活动日志与消息中心

- Panel Chat 支持单 Agent 对话、多 Agent 群聊、知识上下文绑定、共享上下文和会话落库。
- Runtime 配置会自动隔离并清理 root `model/sessionDir` 等旧字段，兼容新版 OpenClaw。
- 活动日志聚合 OpenClaw、通道、工作流、系统事件和 Bot 回复，支持筛选、搜索和 SQLite 持久化。
- 消息中心展示安装、更新、工作流、系统任务进度，长任务状态通过 WebSocket 实时推送。

### 技能、插件与 ClawHub

- 技能和插件分区管理，支持搜索、筛选、启用、禁用、复制和工作区扫描。
- 插件中心支持市场浏览、GitHub / 本地目录安装、卸载、更新、启停、冲突检测和运行日志。
- 插件配置可读取 `plugin.schema.json` 动态生成表单，减少直接编辑 JSON 的成本。
- ClawHub / SkillHub 相关接口支持 registry 配置，便于后续扩展插件和技能生态。

### 自检修复与更新

- 配置检查器可扫描 OpenClaw、NapCat、模型、Token、端口、WS/HTTP、QQ 关键项并给出一键修复。
- 面板更新基于 GitHub Releases：检查版本、下载、SHA256 校验、替换二进制、重启和回滚。
- OpenClaw 更新可跳转独立更新页面，可视化执行 `openclaw update` 并显示实时步骤日志。
- 右下角 AI 助手内置 FAQ 上下文，可直接辅助排查安装、配置、通道和运行时问题。

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
| 检查更新显示"服务器错误" | 优先确认服务器能访问 GitHub Releases，检查 DNS、代理或出站防火墙 |
| Browser 在哪里配置 | 面板内进入 `系统配置 → OpenClaw 配置 → 浏览器控制`，可直接管理 `browser.enabled` 与 `browser.defaultProfile` |
| Windows + WSL 的 OpenClaw 能否给 Pro 用 | 可以，但推荐把 WSL 内的 OpenClaw 当作“外部网关”接入；不要指望 Windows 面板直接双击拉起 WSL 里的 Linux OpenClaw |
| OpenClaw 版本显示 unknown | 建议通过 npm 安装：`npm i -g openclaw@latest` |
| 如何卸载 ClawPanel | `sudo systemctl stop clawpanel && sudo rm -rf /opt/clawpanel` |

👉 **完整 FAQ 文档**：[docs/FAQ.md](docs/FAQ.md)

## 架构

```
浏览器
  │
  ▼
ClawPanel 单二进制
  ├─ React 管理界面（go:embed）
  ├─ Gin API / WebSocket / JWT 登录
  ├─ SQLite：事件、任务、会话与面板数据
  ├─ Process Manager：OpenClaw / NapCat / 网关状态与进程控制
  ├─ Config Manager：openclaw.json 读写、备份、schema 兼容清洗
  ├─ Plugin / Skill / ClawHub：插件、技能、市场与本地目录管理
  ├─ Workflow / AI Company / Panel Chat：多 Agent 协作与任务编排
  └─ Updater：ClawPanel 与 OpenClaw 可视化更新
      │
      ├─ 外部 OpenClaw：gateway、agents、skills、plugins、memory、dreaming
      ├─ 通道运行时：QQ/NapCat、微信、飞书、企业微信、Telegram 等
      └─ 可选 Hermes：平台、会话、Profile、任务与日志管理
```

ClawPanel 不再把 OpenClaw runtime 打进发布包。面板负责安装引导、配置管理、状态观测和流程编排；OpenClaw 及相关通道保持为外部运行时，便于升级、迁移和排障。

## 技术栈

| 层级 | 技术 |
|:---|:---|
| 后端 | Go 1.24+ · Gin · modernc.org/sqlite · gorilla/websocket · golang-jwt |
| 前端 | React 18 · TypeScript · TailwindCSS · Vite · Lucide Icons |
| 数据 | SQLite 持久化事件、会话、任务、知识绑定与面板状态；OpenClaw 配置保存在外部 `openclaw.json` |
| 运行时 | 外部 OpenClaw gateway / agents / plugins / skills；NapCat、微信、Hermes 等按需安装或接管 |
| 更新 | GitHub Releases · SHA256 校验 · 独立 updater 进程 · 二进制回滚 |
| 部署 | 单二进制 · `go:embed` 内嵌前端 · systemd / launchd / Windows Service |

## 快速开始

> 跟宝塔面板一样，一条命令搞定安装，自动注册系统服务、开机自启动、配置防火墙。

### 方式一：一键安装（推荐） 

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

自动完成：下载二进制 → 安装到 `/opt/clawpanel` → 注册系统服务 → 开机自启动 → 配置防火墙 → 启动。
完整安装流程请查看在线文档：[QQ NapCat个人号安装教程](https://doc.weixin.qq.com/doc/w3_AdsADQa9AEcCNNqyBMQ3oQmWG451V?scode=AFoAZAcoAHMV6Hv1R5AdsADQa9AEc)

**Windows（PowerShell 管理员）**

```powershell
irm https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.ps1 | iex
```

> [!NOTE]
> 安装脚本兼容 **PowerShell 5.1 及以上**版本（Windows 自带版本即可）。脚本会自动从 GitHub 获取最新版本，无需手动指定版本号。

或从 [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) 手动下载对应版本的 `clawpanel-v<version>-windows-amd64.exe`，双击或命令行运行。

### 手动更新（面板内更新失败时使用）

> [!IMPORTANT]
> **Windows + WSL 使用 Pro 的推荐方式**：先在 WSL 内自行启动 `openclaw gateway`，再在 ClawPanel 中把网关模式切到 `remote` 指向 WSL 暴露的网关地址。当前不建议把 Windows 面板进程直接当成 WSL 内 Linux OpenClaw 的拉起器。

> [!TIP]
> 正常情况下在面板内「系统配置 → 检查更新」即可一键更新。如果面板内更新失败，可运行以下命令从 GitHub Releases 手动更新到最新版本。

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/update-pro.sh | sudo bash
```

> 脚本会自动：检测当前版本 → 从 GitHub 下载最新版 → 停服 → 替换二进制 → 重启服务 → 验证启动。更新失败时自动回滚。

### 一键卸载

**Linux**

```bash
sudo systemctl stop clawpanel && sudo rm -rf /opt/clawpanel
```

**macOS**

```bash
sudo launchctl unload /Library/LaunchDaemons/com.clawpanel.service.plist && sudo rm -f /Library/LaunchDaemons/com.clawpanel.service.plist && sudo rm -rf /opt/clawpanel
```

**Windows**

```powershell
sc stop ClawPanel; sc delete ClawPanel; Remove-Item -Recurse -Force C:\ClawPanel
```

### 卸载本机外部 OpenClaw

以下命令用于清理你系统里**单独安装**的 `openclaw`（例如 npm 全局安装、用户目录下的 `~/.openclaw` / `~/openclaw`）。

**Linux / macOS**

```bash
npm uninstall -g openclaw || sudo npm uninstall -g openclaw || true
rm -rf "$HOME/.openclaw" "$HOME/openclaw"
sudo rm -f /usr/local/bin/openclaw /usr/local/bin/openclaw-gateway /opt/homebrew/bin/openclaw /opt/homebrew/bin/openclaw-gateway
sudo rm -rf /usr/local/lib/node_modules/openclaw /opt/homebrew/lib/node_modules/openclaw /usr/lib/node_modules/openclaw
```

**Windows（PowerShell）**

```powershell
npm uninstall -g openclaw
Remove-Item -Recurse -Force "$env:USERPROFILE\.openclaw","$env:USERPROFILE\openclaw" -ErrorAction SilentlyContinue
Remove-Item -Force "$env:APPDATA\npm\openclaw.cmd","$env:APPDATA\npm\openclaw","C:\Program Files\nodejs\openclaw.cmd" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:APPDATA\npm\node_modules\openclaw" -ErrorAction SilentlyContinue
```

### 方式二：手动下载运行

从 [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) 下载对应平台的二进制文件：

```bash
# Linux
chmod +x clawpanel-v<version>-linux-amd64 && ./clawpanel-v<version>-linux-amd64

# macOS
chmod +x clawpanel-v<version>-darwin-arm64 && ./clawpanel-v<version>-darwin-arm64

# Windows (双击或命令行)
clawpanel-v<version>-windows-amd64.exe
```

启动后访问 `http://localhost:19527`，默认密码 `clawpanel`。

> [!WARNING]
> 手动运行不会注册系统服务，关闭终端后服务会停止。推荐使用一键安装。

### 方式二补充：手动下载构建包后，在当前目录直接安装

如果你已经手动下载好了对应平台的构建包，并且当前终端就在该文件所在目录，可以直接运行下面的“仅安装”脚本；脚本会先在当前目录检测匹配的平台构建包，再执行安装。

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install-local-pro.sh -o install-local-pro.sh
bash install-local-pro.sh
```

**Windows（PowerShell）**

```powershell
iwr https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install-local-pro.ps1 -OutFile install-local-pro.ps1
powershell -ExecutionPolicy Bypass -File .\install-local-pro.ps1
```

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
> 构建需要 Go 1.24+ 和 Node.js 22+。如需配置 Go/npm 镜像，可自行设置：
> ```bash
> export GOPROXY=https://goproxy.cn,direct
> npm config set registry https://registry.npmmirror.com
> ```

### 本地测试与覆盖率

开发或提交前，建议至少运行一次以下命令：

```bash
go test ./...
go test ./... -coverprofile=local/coverage.out
go tool cover -func=local/coverage.out
```

如果只想快速验证近期改动，可先跑相关包：

```bash
go test ./internal/eventlog ./internal/updater
```

## 环境变量

多数用户不需要手动设置环境变量。一键安装脚本会把数据目录写入系统服务；只有改端口、接管已有 OpenClaw、切换市场源或排查问题时才需要配置。

| 变量 | 默认值 | 说明 |
|:---|:---|:---|
| `CLAWPANEL_PORT` | `19527` | Web 服务端口 |
| `CLAWPANEL_DATA` | 程序同目录 `data` | 数据目录；脚本安装时为 `/opt/clawpanel/data` 或 `C:\ClawPanel\data` |
| `ADMIN_TOKEN` | `clawpanel` | 初始管理密码，首次登录后请立即修改 |
| `CLAWPANEL_SECRET` | `clawpanel-secret-change-me` | JWT 签名密钥；生产环境建议改成随机长字符串 |
| `CLAWPANEL_DEBUG` | `false` | 调试模式 |
| `OPENCLAW_DIR` | `~/.openclaw` | OpenClaw 配置目录 |
| `OPENCLAW_CONFIG` | 自动推导 | OpenClaw 配置文件路径 |
| `OPENCLAW_APP` | 自动推导 | OpenClaw 应用目录，用于技能、插件与运行时扫描 |
| `OPENCLAW_WORK` | 自动推导 | OpenClaw 工作目录 |
| `LEGACY_SINGLE_AGENT` | `false` | 退回旧单 Agent 兼容模式，隐藏多智能体写操作 |
| `CLAWPANEL_UPDATE_PROXY` | - | 更新下载代理，仅在受限网络中使用 |
| `CLAWPANEL_SERVICE_USER` | `root` / 当前 sudo 用户 | Linux/macOS 安装脚本创建服务时使用的运行用户 |
| `CLAWHUB_REGISTRY` | 内置 GitHub 源 | ClawHub 插件与技能注册表地址 |
| `CLAWHUB_REGISTRY_FALLBACK` | 内置兜底源 | ClawHub 主源不可用时的兜底地址 |
| `CLAWHUB_SITE` | 内置站点 | ClawHub 前端站点地址 |
| `SKILLHUB_BIN` | 自动查找 | 自定义 SkillHub 可执行文件路径 |
| `HERMES_HOME` | `~/.hermes` | Hermes 平台数据目录 |
| `CLAWPANEL_ALLOW_INSECURE_PLUGIN_MIRROR` | `false` | 允许非 HTTPS 插件镜像，仅建议本地调试使用 |

## 服务管理

```bash
# Linux / systemd
sudo systemctl start clawpanel
sudo systemctl stop clawpanel
sudo systemctl restart clawpanel
sudo systemctl status clawpanel
sudo journalctl -u clawpanel -f

# macOS / launchd
sudo launchctl load -w /Library/LaunchDaemons/com.clawpanel.service.plist
sudo launchctl unload -w /Library/LaunchDaemons/com.clawpanel.service.plist
tail -f /opt/clawpanel/data/clawpanel.log

# Windows 服务
sc start ClawPanel
sc stop ClawPanel
sc query ClawPanel
```

Linux/macOS 安装目录默认是 `/opt/clawpanel`，Windows 默认是 `C:\ClawPanel`。如果启动异常，优先查看系统服务日志，再运行 `scripts/diagnose.sh` 做环境、端口、OpenClaw 与网关状态检查。

## 跨平台支持

| 平台 | 架构 | 安装与运行方式 |
|:---:|:---:|:---|
| Linux | x86_64 / ARM64 | `install.sh` 自动下载 GitHub Release 并注册 systemd |
| macOS | Intel / Apple Silicon | `install.sh` 自动下载 GitHub Release 并注册 launchd |
| Windows | x86_64 | `install.ps1` 自动下载 GitHub Release 并注册 Windows 服务 |

Release 资产统一从 GitHub 下载，不再维护 Lite 包或自建加速服务器入口。OpenClaw、通道运行时和 Hermes 由面板在本机接管或安装，适合部署在桌面、服务器、NAS 与轻量云主机上。

## 更新日志

完整更新日志请查看 [changelogs/](changelogs/) 目录。

### 最近版本

#### v5.5.1 — 标准版发行收口与 OpenClaw 配置兼容
- **只保留标准版发行**：安装脚本、更新链路与发布资产统一走 GitHub Release，不再维护 Lite 包和自建加速服务器入口
- **OpenClaw schema 清理**：保存配置时会过滤 OpenClaw 不支持的 `useDynamicModel`、`modelSettings`、`excludeTypes` 等面板扩展字段，减少网关启动失败
- **Providers 操作修复**：编辑、删除、设为默认等操作固定显示，避免模型服务商卡片因为数量变化丢失按钮
- **更新与任务稳定性**：强化 OpenClaw 配置回退、工作流任务去重和运行时探测

#### v5.5.0 — Hermes 平台管理与 Panel Chat 稳定性
- **Hermes 管理上线**：新增 Hermes 页面、API 与控制台能力，可在面板中管理相关平台运行状态
- **Panel Chat 修复**：修复首页助手发送失败、会话状态不同步、模型切换异常等问题
- **OpenClaw 配置更稳**：补强 providers/schema 写入、默认模型选择和运行时配置兼容
- **前端体验整理**：继续统一状态卡、页面路由与移动端展示细节

#### v5.4.5 — Workflow Center 交付修复
- **工作流执行修复**：修复实例继续、步骤恢复、任务详情和事件流中的多处边界问题
- **文件结果回传增强**：优化工作区文件产出、预览、下载和重发链路
- **运行时状态更准确**：减少 OpenClaw、网关、通道状态误判导致的错误提示

#### v5.4.4 — AI Company 协同编排
- **AI Company 能力增强**：补强角色、任务、协作流程与执行视图
- **多智能体体验升级**：继续完善 Agent 工作台、路由上下文和权限配置
- **通道与消息链路修复**：优化 QQ、飞书、企业微信等通道的保存、启停和消息同步

#### v5.4.3 — 会话与配置稳定性
- **会话状态修复**：减少聊天、消息中心、活动日志之间的数据不同步
- **配置保存补强**：修复模型、浏览器、插件与通道配置保存后的刷新问题
- **安装与诊断优化**：补齐脚本提示、运行环境检查和常见故障引导

<details>
<summary><b>更早版本摘要</b></summary>

- **v5.4.2 / v5.4.1 / v5.4.0**：围绕 AI Company、Workflow Center、Panel Chat、活动日志和多智能体管理做了一轮集中迭代
- **v5.3.x**：补强 SkillHub、ClawHub、插件市场、模型配置、高级 JSON 与多通道管理
- **v5.2.x**：Workflow Center 1.0 完成，支持模板、运行实例、步骤详情、审批、暂停、恢复、重试、文件产出与回传
- **v5.1.x**：现代化前端、移动端布局、Agent 工作台、插件中心和通道配置校验持续收口
- **v5.0.x**：完成 Go + React 全栈重写，单二进制部署、SQLite、WebSocket、插件中心、OpenClaw 管理和跨平台安装脚本成型
- **v4.x 及更早**：从基础管理后台逐步演进为 ClawPanel，加入 QQ/微信通道、技能中心、模型配置和原生安装能力

</details>

## 致谢

### 开发者与贡献者

<p>
  <a href="https://github.com/zhaoxinyi02"><img src="https://avatars.githubusercontent.com/u/98445030?v=4" width="64" height="64" alt="zhaoxinyi02" /></a>
  <a href="https://github.com/BlueSkyXN"><img src="https://avatars.githubusercontent.com/u/63384277?v=4" width="64" height="64" alt="BlueSkyXN" /></a>
  <a href="https://github.com/codeKing6412"><img src="https://avatars.githubusercontent.com/u/185812512?v=4" width="64" height="64" alt="codeKing6412" /></a>
  <a href="https://github.com/Hns16"><img src="https://avatars.githubusercontent.com/u/192765150?v=4" width="64" height="64" alt="Hns16" /></a>
  <a href="https://github.com/1575893301m"><img src="https://avatars.githubusercontent.com/u/208801015?v=4" width="64" height="64" alt="1575893301m" /></a>
  <a href="https://github.com/Karovia"><img src="https://avatars.githubusercontent.com/u/139629791?v=4" width="64" height="64" alt="Karovia" /></a>
  <a href="https://github.com/utafrali"><img src="https://avatars.githubusercontent.com/u/8383373?v=4" width="64" height="64" alt="utafrali" /></a>
  <a href="https://github.com/Huangqiaoli123"><img src="https://avatars.githubusercontent.com/u/186723551?v=4" width="64" height="64" alt="Huangqiaoli123" /></a>
</p>

<p>
  <a href="https://github.com/zhaoxinyi02">zhaoxinyi02</a> ·
  <a href="https://github.com/BlueSkyXN">BlueSkyXN</a> ·
  <a href="https://github.com/codeKing6412">codeKing6412</a> ·
  <a href="https://github.com/Hns16">Hns16</a> ·
  <a href="https://github.com/1575893301m">1575893301m</a> ·
  <a href="https://github.com/Karovia">Karovia</a> ·
  <a href="https://github.com/utafrali">utafrali</a> ·
  <a href="https://github.com/Huangqiaoli123">Huangqiaoli123</a>
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

[![Star History Chart](https://star-history.dera.page/svg?repos=zhaoxinyi02/ClawPanel&type=date&legend=top-left)](https://star-history.dera.page/#zhaoxinyi02/ClawPanel&type=date&legend=top-left)
