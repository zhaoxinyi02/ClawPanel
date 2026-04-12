# ClawPanel 常见问题解答 (FAQ)

> **💡 提示**：遇到问题请先在面板中使用 **AI 助手**（右下角对话图标）提问，AI 助手已内置本文档的全部知识，能快速帮你排查和解决问题。

---

## 目录

- [安装相关](#安装相关)
- [启动与运行](#启动与运行)
- [面板登录与认证](#面板登录与认证)
- [面板更新](#面板更新)
- [OpenClaw 配置](#openclaw-配置)
- [NapCat / QQ 通道](#napcat--qq-通道)
- [模型与 AI 配置](#模型与-ai-配置)
- [卸载与清理](#卸载与清理)
- [其他问题](#其他问题)

---

## 安装相关

### Q: 安装脚本执行失败 / 下载超时

**A:** 安装脚本从 GitHub Releases 下载二进制文件，国内网络可能不稳定。解决方案：

1. **使用代理**：设置 `https_proxy` 环境变量后重试
2. **手动下载**：从 [GitHub Releases](https://github.com/zhaoxinyi02/ClawPanel/releases) 页面手动下载对应平台的二进制文件，放到 `/opt/clawpanel/` 目录
3. **本机镜像**：如果你使用的是本机部署的更新镜像，可直接从面板服务器下载：
   ```
   export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
   curl -fsSL "$CLAWPANEL_PUBLIC_BASE/api/panel/update-mirror/pro/files/clawpanel-v5.3.3-linux-amd64" -o /tmp/clawpanel
   ```
   替换版本号和平台即可

### Q: macOS 安装报错 "无法验证开发者" / "已损坏"

**A:** macOS 的 Gatekeeper 安全机制会阻止未签名的二进制。解决方法：

```bash
# 移除隔离属性
sudo xattr -d com.apple.quarantine /opt/clawpanel/clawpanel

# 或者在系统偏好设置 → 安全性与隐私 → 允许运行
```

### Q: Linux 安装后 `systemctl start clawpanel` 需要输入密码

**A:** 这是 Linux 系统的正常行为。`systemctl` 命令需要 **sudo 权限**：

```bash
# 使用 sudo 启动（输入你的 Linux 系统用户密码，不是面板密码）
sudo systemctl start clawpanel

# 查看状态（不需要 sudo）
systemctl status clawpanel

# 设置开机自启（需要 sudo）
sudo systemctl enable clawpanel
```

> ⚠️ 这里需要的是你的 **Linux 系统用户密码**，不是 ClawPanel 面板登录密码（默认 `clawpanel`）。

### Q: ARM 架构（树莓派、M1/M2 Mac）能用吗？

**A:** 可以。ClawPanel 提供 `linux-arm64` 和 `darwin-arm64` 二进制。安装脚本会自动检测架构并下载对应版本。

---

## 启动与运行

### Q: 启动后访问面板显示空白页 / 无法连接

**A:** 检查以下几项：

1. **确认服务运行中**：
   ```bash
   systemctl status clawpanel
   # 或查看日志
   journalctl -u clawpanel -n 50 --no-pager
   ```

2. **确认端口正确**：默认端口 `19527`，访问 `http://你的IP:19527`

3. **防火墙放行**：
   ```bash
   # Ubuntu/Debian
   sudo ufw allow 19527
   
   # CentOS/RHEL
   sudo firewall-cmd --add-port=19527/tcp --permanent
   sudo firewall-cmd --reload
   ```

4. **云服务器安全组**：在云控制台放行 19527 端口

### Q: 面板端口被占用

**A:** 修改端口：

```bash
# 编辑配置文件
vi ~/.clawpanel/clawpanel.json
# 修改 "port" 字段为其他端口

# 重启面板
sudo systemctl restart clawpanel
```

或使用环境变量：`CLAWPANEL_PORT=8080`

### Q: 面板启动报错 "数据目录创建失败"

**A:** 面板数据默认存储在 `~/.clawpanel/`。确保当前用户有写入权限：

```bash
mkdir -p ~/.clawpanel
chmod 755 ~/.clawpanel
```

---

## 面板登录与认证

### Q: 默认登录密码是什么？

**A:** 默认密码是 `clawpanel`。首次登录后建议立即修改：

- 面板内：系统配置 → 身份认证 → 修改管理密码
- 或命令行：编辑 `~/.clawpanel/clawpanel.json`，修改 `adminToken` 字段

### Q: 忘记登录密码怎么办？

**A:** 在服务器上重置：

```bash
# 方法一：编辑配置文件
vi ~/.clawpanel/clawpanel.json
# 将 "adminToken" 改为你想要的密码

# 方法二：使用环境变量
export ADMIN_TOKEN="新密码"

# 重启面板
sudo systemctl restart clawpanel
```

### Q: 登录后提示 "认证令牌无效或已过期"

**A:** JWT Token 已过期，需要重新登录。Token 默认有效期 7 天。刷新页面后重新输入密码登录即可。

---

## 面板更新

### Q: 面板检查更新显示 "更新服务器返回错误"

**A:** 可能原因：

1. **网络问题**：面板后端需要能访问 GitHub Releases，或能通过本机代理同步 GitHub 资产
2. **防火墙限制**：确保服务器出站连接未被限制
3. **服务器维护**：偶尔加速服务器可能维护中，稍后重试

手动检查连通性：
```bash
export CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
curl -s "$CLAWPANEL_PUBLIC_BASE/api/panel/update-mirror/pro"
```

### Q: 面板更新后页面没变化

**A:** 浏览器缓存问题。按 `Ctrl+Shift+R`（Mac: `Cmd+Shift+R`）强制刷新页面，或清除浏览器缓存。

### Q: 手动更新面板的方法

**A:** 如果一键更新不可用，可以手动更新：

```bash
# 1. 下载新版本（替换版本号和平台）
wget "$CLAWPANEL_PUBLIC_BASE/api/panel/update-mirror/pro/files/clawpanel-v5.3.3-linux-amd64" -O /tmp/clawpanel

# 2. 替换程序
sudo cp /tmp/clawpanel /opt/clawpanel/clawpanel
sudo chmod +x /opt/clawpanel/clawpanel

# 3. 重启
sudo systemctl restart clawpanel
```

或重新运行安装脚本（会自动更新）：
```bash
curl -fsSL "$CLAWPANEL_PUBLIC_BASE/scripts/install.sh" -o install.sh && sudo CLAWPANEL_PUBLIC_BASE="$CLAWPANEL_PUBLIC_BASE" bash install.sh
```

---

## OpenClaw 配置

### Q: 面板显示 "OpenClaw 未检测到"

**A:** 面板需要能找到 OpenClaw 的配置目录。检查：

1. **确认 OpenClaw 已安装**：
   ```bash
   openclaw --version
   ```

2. **确认配置目录**：默认在 `~/.openclaw/`，面板会自动查找。如果安装在其他位置，设置环境变量：
   ```bash
   export OPENCLAW_DIR=/你的路径/config
   ```

3. **OpenClaw 版本检测**：v5.0.2+ 支持 8 种检测方式（CLI、配置文件、npm 全局、Docker 等），如果仍无法识别，请在 GitHub 提 Issue。

### Q: OpenClaw 版本显示 "unknown" 或 "installed"

**A:** "installed" 表示检测到了 OpenClaw 配置文件但无法确定精确版本号。这通常发生在：

- 手动复制配置文件但未安装 CLI
- Docker 环境中
- 源码编译但未设置 PATH

建议通过 npm 安装 OpenClaw 以获得准确版本号：
```bash
npm i -g openclaw@latest --registry=https://registry.npmmirror.com
```

### Q: 保存配置后不生效

**A:** 修改模型/通道配置后需要**重启 OpenClaw 网关**才能生效。在面板中：

1. 保存配置后会弹出提示
2. 点击「重启网关」按钮
3. 或在仪表盘点击「重启 OpenClaw」

---

## NapCat / QQ 通道

### Q: NapCat 容器启动后显示离线

**A:** 检查以下步骤：

1. **容器运行状态**：
   ```bash
   docker ps | grep openclaw-qq
   ```

2. **查看容器日志**（是否扫码/登录成功）：
   ```bash
   docker logs -f openclaw-qq
   ```

3. **端口配置**：确保 NapCat 的 WebSocket 端口（默认 3001）和 HTTP 端口（默认 3000）正确配置

4. **面板配置检测**：系统配置 → 配置检测 → 检查是否有错误项 → 一键修复

### Q: QQ 机器人发送消息报错 / 无法接收消息

**A:** 常见原因：

1. **WebSocket 未启用**：确保 NapCat 的 `onebot11.json` 中 `ws.enable` 为 `true`
2. **端口不匹配**：OpenClaw 配置的 QQ 通道 `wsUrl` 需要与 NapCat 的 WS 端口一致
3. **Bot 回复不显示**：确保 NapCat 的 `reportSelfMessage` 为 `true`（v5.0.1+ 面板可一键修复）

### Q: NapCat WebUI 打不开 / Token 错误

**A:** NapCat WebUI 使用 SHA256 哈希认证。在面板中配置检测会自动检查和修复 Token 问题。手动修复：

```bash
# 查看 NapCat 容器内的 webui.json
docker exec openclaw-qq cat /app/napcat/config/webui.json
```

---

## 模型与 AI 配置

### Q: AI 助手回复报错 "Provider not found"

**A:** 需要先在系统配置 → 模型管理中配置至少一个 AI 模型提供商，并设置为默认模型。

### Q: 如何配置国内模型（DeepSeek、通义千问等）？

**A:** 在系统配置 → 模型管理页面，选择对应提供商并填写 API Key：

| 提供商 | Base URL | API Key 获取 |
|:---|:---|:---|
| DeepSeek | `https://api.deepseek.com/v1` | [platform.deepseek.com](https://platform.deepseek.com/api_keys) |
| 通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | [dashscope.console.aliyun.com](https://dashscope.console.aliyun.com/apiKey) |
| 硅基流动 | `https://api.siliconflow.cn/v1` | [cloud.siliconflow.cn](https://cloud.siliconflow.cn/account/ak) |
| 火山方舟 | `https://ark.cn-beijing.volces.com/api/v3` | [console.volcengine.com](https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey) |

### Q: 模型连通性检测失败

**A:** 

1. 确认 API Key 正确且未过期
2. 确认 Base URL 完整（注意结尾不要有多余的 `/`）
3. 国内服务器访问国际模型（OpenAI、Claude 等）可能需要代理
4. 部分模型需要开通付费额度才能调用

---

## 卸载与清理

### Q: 如何卸载 ClawPanel？

**A:**

```bash
# 1. 停止并禁用服务
sudo systemctl stop clawpanel
sudo systemctl disable clawpanel

# 2. 删除 systemd 服务文件
sudo rm /etc/systemd/system/clawpanel.service
sudo systemctl daemon-reload

# 3. 删除程序文件
sudo rm -rf /opt/clawpanel

# 4. 删除数据（可选，如果要保留数据请跳过）
rm -rf ~/.clawpanel
```

Windows 用户可以在控制面板中卸载，或手动删除 `C:\ClawPanel` 目录。

### Q: 卸载后如何保留配置数据？

**A:** 面板数据存储在 `~/.clawpanel/` 目录（包含配置、数据库、备份）。卸载时不删除这个目录即可。重新安装后数据会自动恢复。

---

## 其他问题

### Q: 面板支持哪些操作系统？

**A:** 

| 平台 | 架构 | 支持状态 |
|:---|:---|:---|
| Linux | x86_64 (amd64) | ✅ 完全支持 |
| Linux | ARM64 (aarch64) | ✅ 完全支持 |
| macOS | Intel (amd64) | ✅ 完全支持 |
| macOS | Apple Silicon (arm64) | ✅ 完全支持 |
| Windows | x86_64 (amd64) | ✅ 完全支持 |

### Q: 面板占用多少资源？

**A:** ClawPanel 是单文件 Go 程序，资源占用极低：

- **内存**：约 20-50 MB
- **磁盘**：二进制约 26 MB + 数据库
- **CPU**：空闲时几乎为 0

### Q: 多用户 / 权限管理？

**A:** 当前版本为单管理员模式，使用统一管理密码登录。多用户支持在规划中。

### Q: 面板数据存在哪里？

**A:** 所有数据存储在 `~/.clawpanel/` 目录：

- `clawpanel.json` — 面板配置
- `clawpanel.db` — SQLite 数据库（活动日志等）
- `backups/` — 配置备份

### Q: 如何提交 Bug / 功能建议？

**A:** 

1. **先问 AI 助手**：面板右下角 AI 对话，描述你的问题
2. **查看本 FAQ**：检查是否已有解答
3. **GitHub Issue**：[提交 Issue](https://github.com/zhaoxinyi02/ClawPanel/issues/new)，请附上：
   - 操作系统和架构
   - ClawPanel 版本号
   - 错误日志（`journalctl -u clawpanel -n 100`）
   - 复现步骤

---

*最后更新: v5.0.2 (2026-02-25)*
