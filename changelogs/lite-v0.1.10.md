# ClawPanel Lite v0.1.10

发布时间：2026-03-13

## OpenClaw / 网关离线 bugfix（核心修复）

- **修复 Lite 首次安装后 OpenClaw 不自动启动的问题**：旧代码仅在 `openclaw.json` 存在时才自动启动，而首次安装该文件尚未创建；现在 Lite 版只要检测到内嵌运行时即自动启动，`ensureOpenClawConfig` 会在启动过程中自动创建配置
- **放宽 daemon fork 检测超时**：gateway 启动超时从 Linux 15s / Windows 30s 提升至 30s / 45s；daemon fork 检测窗口从 20s 放宽至 60s，解决冷启动时 gateway 初始化慢导致检测失败的问题
- **crash-restart 增加退避与上限**：异常退出后自动重启由无限循环改为最多 8 次、递增延迟（3s→6s→...→30s），避免 gateway 持续崩溃时无限消耗资源

## 其他改进

- 默认写入 `plugins.slots.memory = "none"`，避免 `memory-core` 缺失时配置校验直接失败
- Lite 服务默认增加 `NODE_OPTIONS=--max-old-space-size=2048`，降低 gateway OOM 风险
- macOS 安装脚本增加 19527 端口占用检测，避免与已有实例冲突
- Gitee Release 资产上传增加重试逻辑与断点续传

## 当前说明

- Linux Lite 为当前正式推荐版本
- Windows / macOS Lite 继续保持预览验证阶段
