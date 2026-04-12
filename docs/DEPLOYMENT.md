# ClawPanel 部署指南

## 当前推荐方式

- `ClawPanel Pro`：直接部署官方二进制，适合接管现有 OpenClaw 环境，包括宿主机安装、1Panel、Docker、WSL 等。
- `ClawPanel Lite`：使用官方 Lite 安装脚本，适合需要开箱即用、由面板托管运行时的场景。
- 当前仓库**没有提供官方 ClawPanel Docker 镜像**。如果你的 OpenClaw 跑在 Docker / 1Panel 中，建议继续让 OpenClaw 保持容器化，只把 ClawPanel Pro 作为宿主机面板运行。

## 环境要求

| 组件 | 建议版本 |
| --- | --- |
| Linux | Ubuntu 22.04+ / Debian 12+ / CentOS Stream 9+ |
| macOS | 13+ |
| systemd | Linux 推荐使用 |
| OpenClaw | 近期稳定版本 |

## Pro 安装

### Linux / macOS

```bash
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/install.sh" | sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash
```

如果需要以非 root 用户运行服务，可以先创建用户，再执行：

```bash
sudo useradd -r -m -s /bin/bash clawpanel
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/install.sh" | sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" CLAWPANEL_SERVICE_USER=clawpanel bash
```

安装完成后默认访问：

```text
http://<your-host>:19527
```

### Windows / WSL

- Windows 推荐直接运行 Pro 版二进制。
- 如果 OpenClaw 跑在 WSL 或 Docker 中，请把 ClawPanel 当作外部面板接入，不要依赖 Windows 面板去直接拉起 Linux 版 OpenClaw。
- 通过环境变量或 systemd drop-in 指定 `OPENCLAW_DIR` 到实际配置目录。

## Lite 安装

```bash
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/install-lite.sh" | sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash
```

Lite 默认同样监听 `19527` 端口，并自带 OpenClaw 运行时与工作目录。

## 接管 Docker / 1Panel 中的 OpenClaw

如果 OpenClaw 已经运行在 Docker / 1Panel 中，不需要把 ClawPanel 也放进容器。只需要保证宿主机上的 ClawPanel 能读到 OpenClaw 的配置目录。

例如 systemd drop-in：

```ini
[Service]
Environment="OPENCLAW_DIR=/opt/1panel/apps/openclaw/OpenClaw/data/conf"
```

更新后执行：

```bash
sudo systemctl daemon-reload
sudo systemctl restart clawpanel
```

说明：

- `OPENCLAW_DIR` 指向包含 `openclaw.json` 的目录。
- ClawPanel Pro 会把该实例识别为外部托管运行时。
- 如果你的网关绑定端口不是默认值，也请同步检查 OpenClaw 侧的 gateway 配置。

## 更新

### Pro

```bash
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/update-pro.sh" | sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash
```

### Lite

```bash
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/update-lite.sh" | sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash
```

## 常用运维命令

```bash
sudo systemctl status clawpanel
sudo systemctl restart clawpanel
journalctl -u clawpanel -n 100 --no-pager
```

Lite 服务名为：

```bash
sudo systemctl status clawpanel-lite
sudo systemctl restart clawpanel-lite
journalctl -u clawpanel-lite -n 100 --no-pager
```

## 常见问题

### 面板显示 OpenClaw / 网关离线，但 Docker 里实际正常

- 确认 `OPENCLAW_DIR` 指向的是宿主机上真实的配置目录。
- 确认网关端口从宿主机可访问，而不是只在容器内部监听。
- 升级到 `Pro v5.3.2+`，该版本已经放宽外部托管实例的健康探测规则，并补齐官方插件通道的直装兜底。

### NapCat 网页已登录，但面板仍显示未登录

- 升级到 `Pro v5.3.2+ / Lite v0.2.2+`。
- 在通道页重新触发一次状态检测。
- 若 NapCat 重启过，等待 10~30 秒让 WebUI token 和登录态重新同步。

### Lite 覆盖安装后目录权限被改成 root

- 升级到 `Lite v0.2.2+`。
- 新版安装脚本会保留已有 `data/` 目录的属主，不再把整个安装目录递归改成 `root:root`。
