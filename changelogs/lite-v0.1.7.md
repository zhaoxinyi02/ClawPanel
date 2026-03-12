# ClawPanel Lite v0.1.7

发布时间：2026-03-12

## Lite 运行时隔离正式收口

- 进一步收紧 Lite 与本机外部 `openclaw` 的隔离
- `OpenClawInstalled`、网关重启候选、进程管理器查找与插件 CLI 执行统一收口到内嵌运行时
- Lite 不再走单独 OpenClaw 更新，统一按整包更新处理

## 三端安装与卸载

- Linux / macOS / Windows Lite 安装与卸载脚本完成复核
- 公开下载入口已同步到 GitHub 与加速服务器
- Windows Lite PowerShell 脚本编码兼容问题已修复

## 说明

- Lite 继续固定内嵌 OpenClaw `2026.2.26`
- 本版本用于三端真机正式安装与卸载验证
