# 2026-03-09 NapCat 刚登录即提示失效

## 问题

- QQ（NapCat）刚完成登录后，面板很快又显示“登录已失效”。

## 定位

- 失效判断入口在 `internal/monitor/napcat.go` 的 `checkQQLoginStatus` / `doCheckQQLoginStatus`
- NapCat WebUI 鉴权失败时代码会走“重新取 token 再登录”的兜底逻辑
- 但这段兜底逻辑只从 Docker 容器内读取 `webui.json`，没有兼容 Windows NapCat Shell
- 因此在 Windows 上，一旦旧 credential 失效，重试分支拿不到新 token，会直接返回“未登录”，上层监控随后把状态打成 `login_expired`
- 同时，重试成功后 `GetQQLoginInfo` 仍继续使用旧的 `cred`，存在后续信息读取不一致的问题

## 修复

- 提取统一的 `getNapCatWebUIToken`
  - Windows 优先读 NapCat 最新日志中的 WebUI token，再回退 `config/webui.json`
  - Docker 继续读取容器内 `webui.json`
- 初次认证和鉴权失败后的重试都改用这一个 token 读取入口
- 重试认证成功后同步更新当前请求使用的 `cred`，避免后续接口继续拿旧 credential

## 验证建议

- 在 Windows 环境下重新登录 QQ，观察频道页 NapCat 状态是否仍会立刻跳成“登录已失效”
- 登录后等待 1 到 2 个监控周期，确认仍保持在线
- 若问题仍存在，再抓取 `napcat-status` WebSocket 推送和 NapCat WebUI 鉴权返回码继续排查
