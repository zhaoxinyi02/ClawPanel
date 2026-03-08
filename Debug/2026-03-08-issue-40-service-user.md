# 2026-03-08 issue 40 网关重启用户身份修复记录

## 问题

- Linux 安装脚本把 systemd 服务用户写死为 `root`。
- 这样 ClawPanel 启动后的 OpenClaw 网关和后续重启流程都会沿用 root 身份，不利于最小权限运行，也不方便使用自定义用户目录。

## 定位

- `scripts/install.sh` 在生成 systemd service 时固定写入 `User=root`。
- 只要服务本身以 root 启动，后续面板内的网关启动/重启也会自然沿用 root 上下文。

## 修复

- 安装脚本新增服务用户解析逻辑：
  - 优先读取 `CLAWPANEL_SERVICE_USER`
  - 否则优先使用 `sudo` 调用者 `SUDO_USER`
  - 最后才回退到 `root`
- 自动解析并写入对应 `Group=`。
- 安装时将 `${INSTALL_DIR}` 归属调整给服务用户，确保非 root 服务账户也能正常运行。
- systemd service 中补充 `HOME=~service_user`，避免运行时仍落到 root 家目录上下文。

## 验证建议

- 使用 `sudo bash install.sh` 安装后执行 `systemctl cat clawpanel`，确认 `User=` 不是固定 root。
- 在需要自定义账户时，执行 `CLAWPANEL_SERVICE_USER=youruser sudo bash install.sh`。
- 安装完成后通过面板触发网关重启，确认 OpenClaw 相关文件与进程归属符合预期用户。
