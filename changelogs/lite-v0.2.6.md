# ClawPanel Lite v0.2.6

发布时间：2026-04-06

## Lite 构建修复

- 修复 `package-lite-core.sh` 中 `install_npm_package` 定义位置错误，导致 CI/Release Build 执行时 `command not found`
- 保持 Lite 内嵌 OpenClaw runtime `2026.4.5` 与 npm 官方源回退逻辑不变
