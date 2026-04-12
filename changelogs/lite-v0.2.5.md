# ClawPanel Lite v0.2.5

发布时间：2026-04-06

## 内嵌 OpenClaw 升级

- Lite 内嵌 OpenClaw runtime 默认版本从 `2026.2.26` 提升到 `2026.4.5`
- 面板中的 Lite 运行时版本显示与安装脚本固定版本同步更新

## Lite 打包稳定性修复

- Lite Core 打包脚本在 `npmmirror` 下载 npm tarball 失败时，会自动回退到官方 npm registry
- 修复 Release Build 在安装 OpenClaw runtime 依赖阶段因为镜像 404 而失败的问题
