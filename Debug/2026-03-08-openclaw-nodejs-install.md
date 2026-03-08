# 2026-03-08 OpenClaw Windows 一键安装 Node.js 误判

## 问题

- 处理 `issue.md` 中第 3 个问题：Windows 用户一键安装 OpenClaw 时，即使已经触发 Node.js 安装流程，脚本仍可能提示未检测到 Node.js 并直接退出。

## 排查结论

- `internal/handler/software.go` 的 Windows 安装脚本只依赖 `Get-Command node` 与简单 PATH 刷新。
- 当 Node.js 通过 `winget` 刚安装完成、PATH 尚未完全刷新，或 Node 位于常见目录但未进入当前 PowerShell 会话时，会被误判为未安装。
- 误判后脚本会直接 `exit 1`，导致 OpenClaw 一键安装中断。

## 本次修复

- 增加 `Refresh-Path`，主动补齐 `Program Files\nodejs`、`AppData\Roaming\npm` 等常见路径。
- 增加 `Get-NodeCommand`，支持从 PATH、`Program Files`、`Program Files (x86)`、`NVM_HOME` 多路径探测 `node.exe`。
- 在 `winget` 安装后增加二次探测与等待，降低刚安装完成即误判的概率。
- 当 `winget` 不可用或安装后仍未找到 `node` 时，增加官方 MSI 下载静默安装兜底。

## 验证建议

- 在全新 Windows 环境触发“安装 OpenClaw”。
- 确认日志能正确输出 Node.js 已安装，并继续执行 `npm install -g openclaw@latest`。
- 若仍失败，优先检查网络是否可访问 `npmmirror.com` / `nodejs.org`。
