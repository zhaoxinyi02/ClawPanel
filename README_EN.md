<div align="center">

<img src="img/logo.jpg" width="700"/>

# ClawPanel

**OpenClaw Smart Management Panel — Single-binary deployment, cross-platform, full-featured visual management**

Go Single Binary · React 18 · TailwindCSS · SQLite · WebSocket Real-time · Cross-platform

[![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-red?style=flat-square)](LICENSE)
[![Pro](https://img.shields.io/badge/Pro-5.4.5-violet?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Lite](https://img.shields.io/badge/Lite-0.3.5-2563eb?style=flat-square)](https://github.com/zhaoxinyi02/ClawPanel/releases)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/react-18-61DAFB?style=flat-square&logo=react&logoColor=white)](https://react.dev)
[![CI](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml/badge.svg)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/ci.yml)
[![Release Build](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/release.yml/badge.svg)](https://github.com/zhaoxinyi02/ClawPanel/actions/workflows/release.yml)
[![GitHub Stars](https://img.shields.io/github/stars/zhaoxinyi02/ClawPanel?style=flat-square&logo=github)](https://github.com/zhaoxinyi02/ClawPanel/stargazers)

[Quick Start](#quick-start) · [Features](#features) · [Changelog](changelogs/) · [API Docs](docs/API.md) · [中文](README.md)

</div>

---

> [!CAUTION]
> **Disclaimer**
>
> This project is for **learning and research purposes only**. **Commercial use is strictly prohibited.** Using third-party clients to log in to QQ/WeChat may violate Tencent's Terms of Service and **carries a risk of account suspension**. Please use test accounts. The author has **not performed any reverse engineering** and only integrates existing open-source projects. **No responsibility is assumed for any consequences.** By downloading and using this project, you agree to the [full disclaimer](DISCLAIMER.md).

> [!NOTE]
> **v5.4.5 / v0.3.5 Update** — this release focuses on workflow delivery fixes: markdown artifacts are exported to Word/Excel (with activity templates preferred), WeChat file-delivery payload compatibility is improved, and publish steps now prefer delivering real output artifacts instead of wrapper-only files. See [changelogs/v5.4.5.md](changelogs/v5.4.5.md) and [changelogs/lite-v0.3.5.md](changelogs/lite-v0.3.5.md)

> [!NOTE]
> **v5.0.4 New** — 🔧 NapCat one-click diagnose & repair, system diagnostic report, QR login auto-detect & auto-close, disabled auto-login, container auto-restart after install, PATH detection fix. [View changelog](changelogs/v5.0.4.md)

## Features

### Smart Dashboard
- OpenClaw process monitoring (start/stop/restart)
- Enabled channels overview, current model, uptime, memory usage
- Quick actions: one-click restart OpenClaw / Gateway / ClawPanel / NapCat

### Channel Management (20+ Channels)
Unified configuration for **20+ channels** with one-click enable/disable:
- **Built-in**: QQ (NapCat) · WeChat · Telegram · Discord · WhatsApp · Slack · Signal · Google Chat · BlueBubbles · WebChat
- **Plugins**: Feishu · DingTalk · WeCom · QQ Official Bot · IRC · Mattermost · Teams · LINE · Matrix · Twitch
- **QQ Login**: QR code / quick / password — with logout and NapCat container restart
- **Smart QR Refresh**: Auto-detects expired QR codes, retries to fetch fresh ones

### Configuration Center
- **Model Config**: Multi-provider management (OpenAI / Anthropic / Google / DeepSeek / Volcengine etc.)
- **Agent Config**: System prompt, temperature, max tokens
- **Browser Control Presets**: Visual switching between `browser.enabled=false` and `browser.enabled=true + browser.defaultProfile="openclaw"` to reduce accidental takeover of the system browser
- **JSON Mode**: Direct editing of full configuration JSON
- Auto-injects `compat.supportsDeveloperRole=false` for non-OpenAI providers

### Multi-Agent Console (v5.1.0+)
- Agent lifecycle management: create / edit / delete / default agent selection
- Bindings editor with **structured form + advanced JSON mode**
- Channel-aware account routing: reads `channels.<channel>.accounts/defaultAccount` and assists `accountId` selection
- Route preview supports `channel/sender/peer/parentPeer/guildId/teamId/accountId/roles`
- Preview semantics aligned with OpenClaw: bindings without `accountId` only match the channel default account
- Sessions page supports `agent=all`, and cron jobs support selectable `sessionTarget`

### Orchestration Monitor
- **Global topology graph**: React Flow-based DAG visualization of all channels, agents, and routing relationships at a glance
- Three edge types: Channel → Agent bindings, Agent → Agent sub-agent calls, unbound Channel → default Agent fallback
- Click nodes/edges for detail panel: Agent identity/model/tools/sandbox, Channel status/linked agents, Binding match conditions/priority
- Metrics bar: agent count, channel count, binding count, total sessions
- Status cards: per-agent session count, last active time, online status
- dagre auto-layout + manual drag, zoom/pan/minimap support
- Manual refresh rendering, dark mode and i18n (zh-CN / en) support

### Skill Center + Plugin Management
- Skills/plugins separated view with search and filter
- One-click enable/disable, real-time scanning of installed skills

### Event Log
- Real-time message stream: QQ messages, bot replies, system events
- Filter by source/type, keyword search
- SQLite persistent storage — survives restarts
- External service log ingestion (POST /api/events)

### System Management
- Environment detection (OS / CPU / Go / OpenClaw version)
- Config backup & restore (auto-backup before restore)
- Software installation center: one-click install Docker, NapCat, WeChat bot
- Message center: real-time installation task progress
- Identity document editing (IDENTITY.md / USER.md etc.)
- Password change, version update check

### AI Assistant
Built-in AI chat assistant floating panel, supports multi-provider/multi-model switching, automatically uses OpenClaw's configured API.

### Panel Self-Update
- China acceleration server for stable downloads
- Auto: check → download → SHA256 verify → replace → restart
- **Isolated updater process**: runs via `systemd-run --scope`, survives main process stop
- Auto-backup old binary (`.bak`), SHA256 integrity check
- Supports offline update: upload local binary file

### OpenClaw Visual Update
- Click "Go to Update" to open dedicated update page, visually execute `openclaw update`
- Real-time update steps, progress bar, and command output log
- Auto-detect current version and npm latest version
- Auto-send gateway restart signal after update

## Architecture

```
┌──────────────────────────────────────────────┐
│           ClawPanel (Single Binary)           │
│                                              │
│  ┌───────────┐  ┌────────────┐  ┌─────────┐ │
│  │ Go Backend│  │React Front │  │ SQLite  │ │
│  │   (Gin)   │←→│ (go:embed) │  │   DB    │ │
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
    │ OpenClaw│    │NapCat/WeChat│
    │ Process │    │  Containers │
    └─────────┘    └─────────────┘
```

## Tech Stack

| Layer | Technology |
|:---|:---|
| Backend | Go 1.22+ · Gin · SQLite (modernc.org/sqlite) · gorilla/websocket · golang-jwt |
| Frontend | React 18 · TypeScript · TailwindCSS · Lucide Icons · Vite |
| Deployment | Single binary · `go:embed` embedded frontend · Cross-platform static build (`CGO_ENABLED=0`) |
| AI Engine | [OpenClaw](https://openclaw.ai) — supports GPT-4o / Claude / Gemini / DeepSeek etc. |

## Quick Start

> One command to install — just like BT Panel. Auto-registers system service, auto-start on boot, configures firewall.

### Option 1: One-click Install (Recommended)

**Linux / macOS**

```bash
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/install.sh" -o install.sh && sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash install.sh
```

Auto-completes: download binary → install to `/opt/clawpanel` → register system service → auto-start → configure firewall → start.

**Windows (PowerShell as Admin)**

```powershell
$env:CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
irm "$env:CLAWPANEL_PUBLIC_BASE/scripts/install.ps1" | iex
```

Or download `ClawPanel-Setup.exe` from [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) and run as administrator.

### Option 2: Manual Download

Download the binary for your platform from [Releases](https://github.com/zhaoxinyi02/ClawPanel/releases):

```bash
# Linux
chmod +x clawpanel-linux-amd64 && ./clawpanel-linux-amd64

# macOS
chmod +x clawpanel-darwin-arm64 && ./clawpanel-darwin-arm64

# Windows (double-click or command line)
clawpanel-windows-amd64.exe
```

Visit `http://localhost:19527` after startup. Default password: `clawpanel`.

> [!WARNING]
> Manual run does not register a system service. The service stops when you close the terminal. Use one-click install instead.

### Option 3: Build from Source

```bash
git clone https://github.com/zhaoxinyi02/ClawPanel.git
cd ClawPanel
make build        # Build for current platform
make cross        # Cross-compile for all platforms
make installer    # Build Windows exe installer
./bin/clawpanel
```

> [!TIP]
> Requires Go 1.22+ and Node.js 18+. For users in China:
> ```bash
> export GOPROXY=https://goproxy.cn,direct
> npm config set registry https://registry.npmmirror.com
> ```

### Local Testing and Coverage

Before submitting changes, it is recommended to run:

```bash
go test ./...
go test ./... -coverprofile=local/coverage.out
go tool cover -func=local/coverage.out
```

For a faster verification pass focused on recent changes, you can start with:

```bash
go test ./internal/eventlog ./internal/updater
```

## GitHub Actions Automation

The repository now includes two workflows for testing and release packaging:

- `CI` (`.github/workflows/ci.yml`)
  - Trigger: `push` / `pull_request` / manual dispatch
  - Runs:
    - `go vet ./...`
    - `go test -count=1 -shuffle=on ./...` (`ubuntu/windows` matrix)
    - `go test -race -covermode=atomic -coverprofile=coverage.out ./...`
    - frontend `npm ci + npm run build`
    - backend build with embedded frontend dist (`make backend-only`)
  - Artifacts:
    - `go-coverage` (`coverage.out` + `coverage.txt`)
    - `frontend-dist`
    - `clawpanel-linux-amd64-ci` for quick validation
- `Release Build` (`.github/workflows/release.yml`)
  - Trigger: `push` tag `v*` (for example `v5.2.1`) / manual dispatch
  - Runs: automatic multi-platform binaries (`linux/darwin/windows`) + Windows installer `ClawPanel-Setup-v{version}.exe`
  - Publish: for tag runs, assets are uploaded to GitHub Releases with `checksums.txt`

Additionally, `Dependabot` (`.github/dependabot.yml`) checks GitHub Actions dependency updates weekly.

Example:

```bash
git tag v5.2.1
git push origin v5.2.1
```

## Environment Variables

| Variable | Default | Description |
|:---|:---|:---|
| `CLAWPANEL_PORT` | `19527` | Web server port |
| `CLAWPANEL_DATA` | `./data` | Data directory (config + database) |
| `OPENCLAW_DIR` | `~/.openclaw` | OpenClaw config directory |
| `OPENCLAW_CONFIG` | - | OpenClaw config file path (auto-derives directory) |
| `OPENCLAW_APP` | - | OpenClaw app directory (for skill scanning) |
| `OPENCLAW_WORK` | - | OpenClaw work directory |
| `CLAWPANEL_SECRET` | random | JWT signing secret |
| `ADMIN_TOKEN` | `clawpanel` | Admin password |
| `CLAWPANEL_DEBUG` | `false` | Debug mode |

## Service Management

```bash
# systemd (Linux)
systemctl start clawpanel
systemctl stop clawpanel
systemctl restart clawpanel
systemctl status clawpanel
journalctl -u clawpanel -f

# Windows Service
sc start ClawPanel
sc stop ClawPanel
sc query ClawPanel
```

## Cross-platform Support

| Platform | Architecture | Binary |
|:---:|:---:|:---|
| Linux | x86_64 | `clawpanel-linux-amd64` |
| Linux | ARM64 | `clawpanel-linux-arm64` |
| macOS | x86_64 | `clawpanel-darwin-amd64` |
| macOS | ARM64 (M1/M2/M3) | `clawpanel-darwin-arm64` |
| Windows | x86_64 | `clawpanel-windows-amd64.exe` |

## Changelog

See [changelogs/](changelogs/) for full release notes.

### v5.0.0 — Full Rewrite (2026-02-22)
- **Full rewrite**: Backend Node.js → Go (Gin), Frontend React 18 + TailwindCSS
- **Single-binary deploy**: Static compiled binary with embedded frontend, no Node.js/Docker needed
- **Cross-platform**: Linux / Windows / macOS (x86_64 / arm64)
- **SQLite persistence**: Event logs and config stored in SQLite
- **WebSocket real-time**: Process logs and message events pushed in real-time
- **Process manager**: Built-in OpenClaw process management
- **AI assistant**: Built-in multi-model AI chat panel
- **Software center**: One-click install Docker, NapCat, WeChat bot
- **Quick restart**: One-click restart OpenClaw / Gateway / ClawPanel / NapCat
- **QR code fix**: Smart refresh mechanism for expired QR codes
- **Activity log**: Bot reply messages displayed, persistent storage
- **Native installers**: Linux/macOS/Windows one-click install + system service registration

<details>
<summary><b>v4.x and earlier</b></summary>

- **v4.4.0** (2026-02-21) — AI assistant, model compatibility fixes
- **v4.3.0** (2026-02-19) — Skills/plugins separation, password change, i18n, native installers
- **v4.2.x** (2026-02-16~17) — Violet theme, channel display fixes, QQ login fixes
- **v4.1.0** (2026-02-14) — 20+ channels, skill center, 6-tab system config
- **v4.0.0** (2026-02-13) — ClawPanel brand upgrade
- **v3.0.0** (2026-02-10) — QQ + WeChat dual-channel
- **v2.0.0** (2026-02-09) — React + TailwindCSS admin panel
- **v1.0.0** — Basic admin panel + NapCat Docker integration
</details>

## Acknowledgments

- [OpenClaw](https://openclaw.ai) — AI assistant engine
- [Gin](https://github.com/gin-gonic/gin) — Go web framework
- [NapCat](https://github.com/NapNeko/NapCatQQ) — QQ protocol framework
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — Pure Go SQLite driver
- [Lucide](https://lucide.dev) — Icon library

## Disclaimer

> **This project is for learning and research purposes only. Commercial use is strictly prohibited.**

- **No Commercial Use** — Must not be used for any commercial purpose
- **Account Risk** — Using third-party clients to log in to QQ/WeChat may result in account suspension
- **No Reverse Engineering** — This project does not perform any reverse engineering
- **Use at Your Own Risk** — Users assume all risks and legal responsibilities

**Please read the full [DISCLAIMER.md](DISCLAIMER.md)**

## License

[CC BY-NC-SA 4.0](LICENSE) © 2026 — **No Commercial Use**
