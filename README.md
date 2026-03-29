<div align="center">

<img src="img/logo.jpg" width="700"/>

# ClawPanel

**OpenClaw 智能管理面板 — 现已提供 Pro / Lite 双发行形态，覆盖纯面板接管与开箱即用托管运行时**

Go 单二进制 · React 18 · TailwindCSS · SQLite · WebSocket 实时推送 · 跨平台

[![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-red?style=flat-square)](LICENSE)
[![Pro](https://img.shields.io/badge/Pro-5.3.0-violet?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Lite](https://img.shields.io/badge/Lite-0.2.0-2563eb?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/react-18-61DAFB?style=flat-square&logo=react&logoColor=white)](https://react.dev)
[![CI](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml/badge.svg)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml)
[![Release Build](https://img.shields.io/badge/Release%20Build-tags%20passing-22c55e?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/release.yml)
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

> [!NOTE]
> **最新发布** — `ClawPanel Pro v5.3.0` 与 `ClawPanel Lite v0.2.0` 已发布；若一键安装/手动更新没有拿到最新版，请优先重新拉取安装脚本或检查加速服务器元数据是否已同步。

### 💬 社区交流
欢迎加入 **ClawPanel 官方交流群**，获取最新更新、反馈问题、参与插件开发。

> 📱 **扫码加入企微群**
> 
> <img src="img/wecom.jpg" width="300"/>
> 



## 主要功能

### Lite / Pro 对比

| 维度 | ClawPanel Lite | ClawPanel Pro |
|:---|:---|:---|
| 目标用户 | 新手 / 想开箱即用 | 已有 OpenClaw 环境的进阶用户 |
| OpenClaw | 内置 `2026.2.26`，安装后直接可用 | 不内置，支持一键安装或外部接管 |
| 运行时控制 | 面板全托管 | 用户可自行决定 |
| 通道插件 | 常用插件预置好，默认关闭可直接配置 | 按需安装、按需接管 |
| 外部 OpenClaw | 不支持 | 支持 |
| 更新 | 跟随 Lite 独立版本线 | 跟随 Pro 独立版本线 |
| Linux | 已可用 | 已可用 |
| macOS | 预览阶段 | 已可用 |
| Windows | 暂不提供（整包超 2GB 限制，请使用 Pro） | 已可用 |

### Workflow Center 1.0（v5.2.1 重点更新）
- 工作流设置、模板 CRUD、AI 生成模板、运行列表、步骤详情、事件流与删除实例
- 支持复杂任务自动接管原会话：即时确认、进度回写、暂停 / 恢复 / 重试 / 审批 / 继续
- 节点支持 `input / wait_user / approval / ai_plan / ai_task / analyze / summary / publish / end`
- 节点支持 `skill` 参数，可自动加载 `SKILL.md` 参与执行
- 关键步骤结果自动落文件到 OpenClaw 工作区，并在工作流详情页显示产出文件、回传状态、失败原因
- QQ 私聊 / 群聊支持工作流完成后自动回传最终文件，支持预览、下载、单文件重发与批量重发最终文件

### 多智能体控制台（v5.1.x 重点更新）
- Agent 列表管理：新建 / 编辑 / 删除，支持默认 Agent 设置
- `Core Files`：可直接查看和保存 Agent 工作区核心文件
- `Skills · Channels · Cron`：从单 Agent 视角查看技能、通道和定时任务快照
- `Recent Sessions`：快速巡检当前 Agent 最近活跃会话
- Bindings 路由规则管理：支持**结构化表单 + JSON 高级模式**，可启停、重排和规则级错误定位
- 路由预览器：输入 `channel/sender/peer/parentPeer/guildId/teamId/accountId/roles` 快速验证命中 Agent

### 编排监控台（Monitor）
- **全局拓扑图**：基于 React Flow 的 DAG 可视化，一眼看清所有通道、智能体和路由关系
- Channel → Agent 绑定路由、Agent → Agent 子代理调用、未绑定通道 → 默认代理回退，三类连线一目了然
- 点击节点/边查看详细配置：Agent 身份/模型/工具/沙箱，Channel 状态/关联 Agent，Binding 匹配条件/优先级
- 顶部指标栏：Agent 数、通道数、绑定数、会话总数实时统计
- 底部状态卡片：每个 Agent 的会话数、最后活跃时间、在线状态
- dagre 自动分层布局 + 手动拖拽调整，支持缩放/平移/小地图
- 手动刷新触发渲染，支持暗色模式和中英双语

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
- **浏览器控制预设**：可视化切换 `browser.enabled=false` 与 `browser.enabled=true + browser.defaultProfile="openclaw"`，降低误接管系统浏览器的风险
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
- **多来源安装**：支持从 GitHub、Gitee 脚本入口、本地目录安装，并根据区域在线选择 GitHub 或香港加速服务器下载构建包
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
| 检查更新显示"服务器错误" | 优先确认服务器能访问 GitHub；国内线路也可改走香港加速服务器，检查出站防火墙 |
| Browser 在哪里配置 | 面板内进入 `系统配置 → OpenClaw 配置 → 浏览器控制`，可直接管理 `browser.enabled` 与 `browser.defaultProfile` |
| Windows + WSL 的 OpenClaw 能否给 Pro 用 | 可以，但推荐把 WSL 内的 OpenClaw 当作“外部网关”接入；不要指望 Windows 面板直接双击拉起 WSL 里的 Linux OpenClaw |
| OpenClaw 版本显示 unknown | 建议通过 npm 安装：`npm i -g openclaw@latest` |
| 如何卸载 ClawPanel | `sudo systemctl stop clawpanel && sudo rm -rf /opt/clawpanel` |

👉 **完整 FAQ 文档**：[docs/FAQ.md](docs/FAQ.md)

## 架构

```
┌───────────────────────────────────────────────────────────┐
│                    ClawPanel 统一代码库                    │
│                                                           │
│   ┌────────────────────┐        ┌────────────────────┐    │
│   │   ClawPanel Lite   │        │    ClawPanel Pro   │    │
│   │  托管运行时发行形态  │        │   纯面板发行形态    │    │
│   ├────────────────────┤        ├────────────────────┤    │
│   │ 内置 OpenClaw      │        │ 外部接管 / 一键安装 │    │
│   │ 预置常用通道插件    │        │ 自定义插件 / 自定义环境 │    │
│   │ clawlite-openclaw  │        │ 系统 openclaw       │    │
│   └─────────┬──────────┘        └─────────┬──────────┘    │
│             │                             │               │
│      ┌──────┴─────────────────────────────┴──────┐        │
│      │ Go 后端 + React 前端 + SQLite + Updater    │        │
│      │ Process Manager / Plugin Manager / WS Hub  │        │
│      └──────────────────────┬─────────────────────┘        │
└─────────────────────────────┼───────────────────────────────┘
                              │
                   ┌──────────┴──────────┐
                   │ OpenClaw / NapCat 等 │
                   │ 运行时与通道资源层    │
                   └─────────────────────┘
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

**ClawPanel Lite（Linux，现已可用）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install-lite.sh -o install-lite.sh && sudo bash install-lite.sh
```

自动完成：下载 Lite Core → 安装到 `/opt/clawpanel-lite` → 注册 `clawpanel-lite` 服务 → 启动内置 OpenClaw。

**ClawPanel Lite（macOS，预览阶段）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install-lite-macos.sh -o install-lite-macos.sh && sudo bash install-lite-macos.sh
```

说明：当前提供 `darwin-amd64 / darwin-arm64` 预览包，建议先在测试环境验证。安装后如遇“无法验证开发者”提示，可执行 `sudo xattr -d com.apple.quarantine /opt/clawpanel-lite/clawpanel-lite`。

**ClawPanel Pro（Linux / macOS）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install.sh -o install.sh && sudo bash install.sh
```

自动完成：下载二进制 → 安装到 `/opt/clawpanel` → 注册系统服务 → 开机自启动 → 配置防火墙 → 启动。
完整安装流程请查看在线文档：[QQ NapCat个人号安装教程](https://doc.weixin.qq.com/doc/w3_AdsADQa9AEcCNNqyBMQ3oQmWG451V?scode=AFoAZAcoAHMV6Hv1R5AdsADQa9AEc)

**Windows（PowerShell 管理员，当前为 Pro 发行形态）**

```powershell
irm https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install.ps1 | iex
```

> [!NOTE]
> 安装脚本兼容 **PowerShell 5.1 及以上**版本（Windows 自带版本即可）。脚本会自动从 GitHub 获取最新版本，无需手动指定版本号。

或从 [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) 手动下载对应版本的 `clawpanel-v<version>-windows-amd64.exe`，双击或命令行运行。

### 手动更新（面板内更新失败时使用）

> [!IMPORTANT]
> **Windows + WSL 使用 Pro 的推荐方式**：先在 WSL 内自行启动 `openclaw gateway`，再在 ClawPanel 中把网关模式切到 `remote` 指向 WSL 暴露的网关地址。当前不建议把 Windows 面板进程直接当成 WSL 内 Linux OpenClaw 的拉起器。

> [!TIP]
> 正常情况下在面板内「系统配置 → 检查更新」即可一键更新。如果你的服务器之前使用旧加速服务器导致更新失败，可运行以下命令手动更新到最新版本，更新后面板内更新功能将恢复正常。

**ClawPanel Pro（Linux / macOS）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/update-pro.sh | sudo bash
```

**ClawPanel Lite（Linux）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/update-lite.sh | sudo bash
```

> 脚本会自动：检测当前版本 → 从加速服务器下载最新版（失败自动切 GitHub）→ 停服 → 替换二进制 → 重启服务 → 验证启动。更新失败时自动回滚。

### 一键卸载

**ClawPanel Lite（Linux）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/uninstall-lite.sh -o uninstall-lite.sh && sudo bash uninstall-lite.sh
```

**ClawPanel Lite（macOS）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/uninstall-lite-macos.sh -o uninstall-lite-macos.sh && sudo bash uninstall-lite-macos.sh
```

**ClawPanel Pro（Linux）**

```bash
sudo systemctl stop clawpanel && sudo rm -rf /opt/clawpanel
```

**ClawPanel Pro（macOS）**

```bash
sudo launchctl unload /Library/LaunchDaemons/com.clawpanel.service.plist && sudo rm -f /Library/LaunchDaemons/com.clawpanel.service.plist && sudo rm -rf /opt/clawpanel
```

**ClawPanel Pro（Windows）**

```powershell
sc stop ClawPanel; sc delete ClawPanel; Remove-Item -Recurse -Force C:\ClawPanel
```

### 卸载本机外部 OpenClaw（不影响 Lite 内嵌运行时）

以下命令用于清理你系统里**单独安装**的 `openclaw`（例如 npm 全局安装、用户目录下的 `~/.openclaw` / `~/openclaw`），**不会删除** Lite 自己安装目录中的内嵌运行时（`/opt/clawpanel-lite/runtime`）。

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

**ClawPanel Pro（Linux / macOS）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install-local-pro.sh -o install-local-pro.sh && bash install-local-pro.sh
```

**ClawPanel Pro（Windows，PowerShell）**

```powershell
iwr https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install-local-pro.ps1 -OutFile install-local-pro.ps1; powershell -ExecutionPolicy Bypass -File .\install-local-pro.ps1
```

**ClawPanel Lite（Linux / macOS）**

```bash
curl -fsSL https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install-local-lite.sh -o install-local-lite.sh && bash install-local-lite.sh
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
> 构建需要 Go 1.22+ 和 Node.js 18+。中国大陆用户请设置：
> ```bash
> export GOPROXY=https://goproxy.cn,direct
> npm config set registry https://registry.npmmirror.com
> ```

## GitHub Actions 自动化

已内置两条工作流，覆盖自动测试与自动打包发布：

- `release.yml` 现支持双发行形态：
  - `pro-vX.Y.Z`
  - `lite-vX.Y.Z`
- `Lite / Pro` 使用独立更新元数据：
  - `release/update-pro.json`
  - `release/update-lite.json`
- 安装脚本从 Gitee 获取，构建包下载与更新默认走“GitHub / 香港加速服务器”双线路

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
  - 触发：`push tag lite-v* / pro-v*`（如 `lite-v0.1.3`、`pro-v5.2.7`）/ 手动触发
  - 执行：自动构建 `linux/darwin/windows` 多平台二进制 + `ClawPanel-Setup-v{version}.exe`
  - 发布：tag 触发时自动上传到 GitHub Release，并生成 `checksums.txt`

另外，已启用 `Dependabot`（`.github/dependabot.yml`）每周自动检查 GitHub Actions 依赖更新。

示例：

```bash
git tag lite-v0.1.3
git push origin lite-v0.1.3

git tag pro-v5.2.7
git push origin pro-v5.2.7
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
| Linux | x86_64 | `clawpanel-v<version>-linux-amd64` |
| Linux | ARM64 | `clawpanel-v<version>-linux-arm64` |
| macOS | x86_64 | `clawpanel-v<version>-darwin-amd64` |
| macOS | ARM64 (M1/M2/M3) | `clawpanel-v<version>-darwin-arm64` |
| Windows | x86_64 | `clawpanel-v<version>-windows-amd64.exe` |

## 更新日志

完整更新日志请查看 [changelogs/](changelogs/) 目录。

### Lite 最近 3 次

#### v0.1.16 — Lite 版本号注入修复
- **🏷️ Lite 版本号修正**：修复 Lite 构建产物沿用 Pro 旧版本号的问题，确保运行中正确显示 `0.1.16`
- **📦 Lite 重新发版**：重新整理 Lite 发布产物，便于线上环境升级到版本号正确的构建

#### v0.1.15 — 构建修复与技能安装权限收口
- **🛠️ 跨平台构建修复**：修复技能目录所有权同步逻辑在 Windows 构建时的兼容问题
- **🔐 技能安装权限修复**：继续确保面板安装技能后目录 owner 与目标父目录一致
- **📜 活动与通道体验修复**：活动日志、最近活动与企微状态修复继续收口

#### v0.1.14 — 技能安装权限与活动日志稳定性修复
- **🔐 技能安装权限修复**：修复面板安装技能后目录所有权可能异常的问题，减少技能目录被锁定
- **📜 活动日志更贴近真实消息**：最近活动与活动日志继续改进，减少 transcript 包装内容直接外显
- **🧭 通道状态与智能体页面优化**：企微通道状态显示修复，智能体页继续中文化

#### v0.1.13 — 自定义技能配置表单与复制能力补齐
- **🧩 skill.json 配置直连面板 UI**：自定义技能的 `configSchema` 现在会渲染成结构化表单，可直接保存配置
- **📋 技能复制到其他智能体 / 全局**：已安装技能支持直接复制，便于多智能体复用
- **⚡ 前端与通道体验同步增强**：合入隐藏标签页暂停轮询、Channels 并行加载与飞书多 ID 兼容

#### v0.1.11 — 通道消息回复修复
- **🩹 修复通道无法回复消息**：`SaveChannel` 保存凭证时会覆盖 `channels[id]` 整个对象，导致 `enabled` 字段丢失，通道变为未启用状态
- **📁 修复 AGENTS.md 模板缺失**：打包脚本 `prune_openclaw_runtime` 直接删除整个 `docs/` 目录，导致 agent 启动时找不到工作区模板而报错退出
- **🔌 修复飞书 plugin entry key 错误**：`SwitchFeishuVariant` 在 Lite 版写入 `feishu-openclaw-plugin` entry 导致 OpenClaw plugin id mismatch
- **🔄 安装脚本自动获取最新版**：去除硬编码兜底版本号，网络无法获取时明确报错并提示手动指定 `VERSION=x.y.z`

#### v0.1.10 — Linux Fresh Install 稳定性修复 + Windows Lite 暂停提供
- **🩹 Linux Lite 新装修复**：修复 fresh Linux Lite 安装后 OpenClaw / gateway 离线的问题
- **🧠 memory slot 默认保护**：默认写入 `plugins.slots.memory = "none"`，避免内置 `memory-core` 缺失时直接导致配置无效
- **📈 服务内存参数补强**：Lite 服务默认增加 `NODE_OPTIONS=--max-old-space-size=2048`，降低 gateway OOM 风险
- **🪟 暂停提供 Windows Lite**：Windows 整包超过 GitHub Release 2GB 上传限制，Windows 用户请使用 Pro 版

#### v0.1.9 — Gitee 同步与预览发布链继续收口
- **🔁 Gitee Release 自动同步**：GitHub 发布后自动同步生成同名 Gitee Release，为国内线路切换做准备
- **🪟🍎 Lite 预览构建继续推进**：Windows / macOS Lite 继续通过 GitHub Actions 原生 runner 构建预览整包
- **📦 Linux Lite 仍为正式可用版**：当前仍建议正式使用 Linux Lite，Windows / macOS Lite 保持预览验证节奏

<details>
<summary><b>更早 Lite 版本</b></summary>

#### v0.1.8 — Lite runtime 依赖 hotfix
- **📦 修复 Lite 包漏打 OpenClaw 运行时依赖**：补齐 `tslog` 等 production 依赖，解决 `dist/entry.js` 可见但无法导入的问题
- **🧪 打包阶段新增 runtime smoke test**：构建时直接校验 `import("./dist/entry.js")`，防止缺依赖包再次发布
- **🧭 启动路径与工作目录继续收口**：Lite 的 launcher、CLI 与网关命令统一对齐内嵌 OpenClaw app 目录

#### v0.1.7 — Lite 运行时隔离正式收口
- **🔒 Lite 与本机外部 OpenClaw 进一步隔离**：`OpenClawInstalled`、网关重启候选、进程管理器查找与插件 CLI 执行统一收口到内嵌运行时
- **📦 Lite 更新语义更明确**：Lite 不再走单独 OpenClaw 更新，统一按整包更新处理，避免误碰系统全局 `openclaw`
- **🧪 三端安装/卸载入口已校验**：Linux / macOS / Windows 的公开安装与卸载脚本从 Gitee 获取，构建包与更新元数据同步到香港加速服务器

#### v0.1.6 — Lite 运行时探测收口
- **🧭 Windows / macOS Lite 优先绑定内置运行时**：版本检测、重启命令与运行路径优先走 `clawlite-openclaw`，避免误探测到本机外部 `openclaw`
- **🧹 新增外部 OpenClaw 卸载说明**：README 中补充 Linux / macOS / Windows 清理本机外部 `openclaw` 的命令，并明确不会影响 Lite 内嵌 runtime
- **📌 内置 OpenClaw 版本保持不变**：Lite 继续固定内嵌 `2026.2.26`

#### v0.1.5 — Windows / macOS Lite 打包脚手架推进
- **🧱 多平台 Lite 打包骨架**：补齐 `macOS / Windows Lite` 的安装脚本、launcher 和整包命名规范，为后续原生构建做准备
- **⚙️ GitHub Actions 原生构建方向**：Lite 打包链开始区分 `ubuntu / macOS / Windows` runner，不再把跨平台整包强行全放在 Linux 上拼装
- **📚 文档同步更新**：README 中明确 Windows / macOS Lite 仍处于预览推进阶段，建议当前优先使用 Linux Lite

#### v0.1.4 — 整包更新与线路选择
- **📦 Lite 改为整包更新**：面板内更新会同步面板、内置 OpenClaw 与预置插件，只替换程序和 runtime，保留现有 data 目录
- **🌐 安装/更新可选下载线路**：新增 GitHub / 香港加速服务器线路选择，并给出"中国香港及境外 / 中国大陆"推荐提示
- **🏷️ 版本管理更明确**：系统配置 → 版本管理会明确标出当前是 `Lite` 还是 `Pro`

#### v0.1.3 — 更新页代理修复与版本体验补强
- **🔄 更新页可直接打开**：修复"前往更新"跳转后显示"当前无法使用此页面"的问题，改为通过面板自身代理访问独立更新器
- **🏷️ 当前版本显示更准**：回到版本页或重新检测更新时会刷新当前版本，避免升级后仍显示旧版本
- **📦 Lite 发布链继续收口**：同步文档、更新元数据、安装脚本与 `0.1.3` 版本号

#### v0.1.1 — 模型配置和开箱即用体验继续收口
- **🧠 模型服务商更干净**：快速添加模型服务商时不再自动塞入过时模型 ID，而是默认创建一个空模型项，用户可直接填写自己想用的模型
- **🛠️ 故障提示更直接**：当 OpenClaw 与网关均离线时，面板会明确提示尝试点击左下角手动重启 OpenClaw
- **📦 发布体验补强**：Lite 版本提升到 `0.1.1`，安装脚本、文档、更新元数据与自动发布链同步对齐

#### v0.1.0 — 首个 Linux 可用版
- **🚀 Linux 首发可用**：安装后直接内置 OpenClaw `2026.2.26`，用户只需配置 API Key 与常用通道凭据即可开始使用
- **🔌 常用通道开箱可配**：预置 `telegram`、`qq`、`qqbot`、`feishu`、`dingtalk`、`wecom`，默认关闭但可直接配置启用
- **🧩 Lite 独立版本线**：新增 Lite 专用安装 / 卸载 / 更新能力，并与 Pro 彻底分成两条发行线

</details>

### Pro 最近 3 次

#### v5.2.15 — 构建修复与技能安装权限收口
- **🛠️ 跨平台构建修复**：修复技能目录所有权同步逻辑在 Windows 构建时的兼容问题
- **🔐 技能安装目录所有权修复**：继续确保面板安装技能后目录 owner 与目标父目录一致
- **📜 活动与通道体验修复**：活动日志、最近活动与企微状态修复继续收口

#### v5.2.14 — 技能安装权限与活动视图稳定性修复
- **🔐 技能安装目录所有权修复**：修复面板安装技能后目录可能落成 `root` 的问题，减少技能不可写与不可维护情况
- **📜 活动日志与最近活动优化**：改为更贴近真实会话消息的展示方式，减少包装上下文干扰
- **🧭 通道与智能体体验收口**：修复企微通道状态映射，智能体页继续纯中文化

#### v5.2.13 — 自定义技能市场与模型配置体验升级
- **🧩 自定义技能配置表单上线**：`skill.json` 的结构化配置正式接入 Skills 页面，可直接编辑与保存
- **📋 技能复制能力补齐**：已安装技能支持复制到其他智能体或全局目录，不再误走商店安装链路
- **⚡ PR #75 合并落地**：飞书多 ID、前端性能优化、模型高级配置 UI 与技能页增强同步合入主线

#### v5.2.11 — 前端性能优化与 Agent/Channel 稳定性修复
- **⚡ 前端页面懒加载**：路由页面 Suspense 懒加载，显著提升首屏加载速度
- **🧠 Agent/Channel 稳定性修复**：修复合并后的多项 agent/channel 流程问题，支持外部 agentDir 与飞书配置校验修正
- **🧪 CI 跨平台测试修复**：修复 macOS/Windows symlink 导致的 agent workspace 测试失败
- **📦 安装入口切换 Gitee**：安装脚本与文档统一切换到 Gitee 入口，减少国内用户访问不稳定

#### v5.2.10 — 发布链路与国内镜像同步补强
- **🔁 Gitee Release 自动同步**：GitHub Release 完成后自动在 Gitee 创建同名 Release 并同步资产，减轻国内访问压力
- **📦 Lite / Pro 发布链继续收口**：为后续国内用户 `GitHub / Gitee` 线路选择做好准备
- **🧰 安装文档补全**：README 中继续补齐 Lite / Pro 多平台安装与卸载说明

#### v5.2.8 — 探测与配置体验补强
- **🍎 macOS OpenClaw 探测增强**：补强 `/Users/*/.openclaw` 与真实用户 home 识别，减少先装 OpenClaw 后装面板时的识别失败
- **🗂️ 受管工作区兼容增强**：兼容 `workspace` / `workspaces` 等目录结构，减少 Agent 文件编辑异常
- **🧠 MiniMax 默认配置修正**：默认 provider 配置更新为 `https://api.minimaxi.com/anthropic/v1` + `MiniMax-M2.5`

<details>
<summary><b>更早 Pro 版本</b></summary>

#### v5.2.7 — QQ / 插件镜像 / 通道安装体验收口
- **🧹 QQ (NapCat) 通道补强**：新增一键删除、更稳的账号级 OneBot 配置修复与状态识别
- **🛒 插件通道体验提升**：插件型通道可直接在通道页安装，不再强制跳转插件中心
- **🌐 插件仓库镜像兜底**：补齐镜像并新增 Gitee 回退，减少用户只看到部分插件的问题

#### v5.2.6 — OpenClaw 运行状态与 Telegram 体验修复
- **📊 运行时状态更真实**：新增 OpenClaw / 网关健康判断，避免"系统运行正常"误导
- **✈️ Telegram 可用性修复**：修复保存 schema、自动 reload 与前端状态刷新问题
- **🛠️ 安装链路补强**：继续收口 OpenClaw 安装与运行环境探测逻辑

#### v5.2.5 — 多智能体与插件体系继续稳定
- **🧠 多智能体管理增强**：补强工作区、上下文与路由相关行为
- **🧩 插件体系稳定性提升**：继续修复安装、配置同步与状态管理问题
- **🐛 通道配置链路修复**：减少保存后配置丢失和状态不同步问题

#### v5.2.4 — 工作流与系统配置持续打磨
- **🔁 工作流体验补强**：继续完善多步任务、事件流与详情页展示
- **⚙️ 系统配置收口**：补强模型、浏览器和 JSON 模式配置写入行为
- **📡 运行时探测增强**：提升 OpenClaw / 网关状态判断稳定性

#### v5.2.3 — 前端结构与通道管理优化
- **🎨 页面结构调整**：继续整理现代化界面与仪表盘布局
- **📻 通道管理优化**：统一更多通道卡片与启停/保存行为
- **🧪 稳定性修复**：修复若干配置页和运行状态相关问题

### v5.2.1 — Workflow Center 1.0 正式完成
- **🧠 工作流中心上线**：新增工作流设置、模板管理、AI 生成模板、运行列表、步骤详情、事件流与中文化控制界面，工作流 1.0 正式可用
- **🔁 原会话自动接管**：复杂任务可在 QQ / 飞书 / 企业微信链路中自动接管，支持即时确认、审批、暂停、恢复、重试、继续与多实例并发
- **📁 文件产出与回传**：关键步骤结果自动写入 OpenClaw 工作区，QQ 私聊 / 群聊支持完成后自动回传最终文件，详情页支持预览、下载、重发与批量重发
- **🛠️ 运行时补强**：工作流节点支持 `skill`，活动日志接入多来源消息流，消息中心同步工作流状态并过滤已取消实例
- **🐛 稳定性修复**：修复重复建单、等待输入无法继续、审批恢复异常、模型协议兼容、异常文件名与中后段步骤超时等问题

### v5.1.8 — 移动端优化、插件中心修正与通道配置收口
- **📱 移动端体验重做**：新增移动端玻璃顶栏、底部导航、操作托盘与 AI 助手全屏模式，页面不再只是桌面布局压缩
- **🧊 玻璃风继续统一**：AI 助手入口与悬浮窗、智能体详情标签、多个页面工具栏与操作按钮继续收口到统一蓝色玻璃风
- **🧩 插件与通道修复**：插件中心作者来源显示规则统一，通道页已安装状态改为和插件中心同源；企业微信拆分为 `智能机器人 / 自建应用`，钉钉切到新版 `Client ID / Client Secret` 配置
- **🛡️ 配置安全性增强**：为多个通道加入“未完成关键配置不能启用”校验，避免空配置直接打开导致运行异常
- **🐛 稳定性修复**：修复定时任务页白屏、AI 助手切换模型白屏、插件中心通知被顶栏遮挡等问题

### v5.1.7 — 现代蓝色玻璃 UI 收口与智能体工作台增强
- **🎨 现代化界面统一**：面板前端正式收口到 modern-only 方向，整体切到蓝色玻璃风，顶部搜索、头像菜单、右上角消息中心与深色模式质感同步升级
- **🧠 智能体工作台补强**：`概览 / 模型与身份 / 工具与权限 / 核心文件 / 技能与上下文 / 路由上下文 / 高级 JSON` 全部切到统一玻璃标签与操作样式，并继续清理中文模式下的中英混写
- **📊 仪表盘状态更直观**：首页新增 `OpenClaw` 与 `网关` 状态卡，`今日消息` 统计改为排除 `system` 日志，更接近真实消息流
- **⚙️ 模型配置更干净**：`系统配置 -> 模型配置` 中每个提供商卡片不再重复显示 `国内 / 国际 / 聚合` 快捷行，统一只在顶部 `快速添加模型服务商` 区域操作

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
</details>

## 致谢

### 开发者与贡献者

<p>
  <a href="https://github.com/zhaoxinyi02"><img src="https://avatars.githubusercontent.com/u/98445030?v=4" width="64" height="64" alt="zhaoxinyi02" /></a>
  <a href="https://github.com/BlueSkyXN"><img src="https://avatars.githubusercontent.com/u/63384277?v=4" width="64" height="64" alt="BlueSkyXN" /></a>
  <a href="https://github.com/Hns16"><img src="https://avatars.githubusercontent.com/u/192765150?v=4" width="64" height="64" alt="Hns16" /></a>
  <a href="https://github.com/codeKing6412"><img src="https://avatars.githubusercontent.com/u/185812512?v=4" width="64" height="64" alt="codeKing6412" /></a>
  <a href="https://github.com/Karovia"><img src="https://avatars.githubusercontent.com/u/7569287?v=4" width="64" height="64" alt="Karovia" /></a>
</p>

<p>
  <a href="https://github.com/zhaoxinyi02">zhaoxinyi02</a> ·
  <a href="https://github.com/BlueSkyXN">BlueSkyXN</a> ·
  <a href="https://github.com/Hns16">Hns16</a> ·
  <a href="https://github.com/codeKing6412">codeKing6412</a> ·
  <a href="https://github.com/Karovia">Karovia</a>
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
