# 2026-03-08 issue 38 本地技能扫描修复记录

## 问题

- 技能中心无法加载部分本地 skills。
- 用户反馈通过 OpenClaw 下载到 workspace 下的技能目录没有被 ClawPanel 扫描出来。

## 定位

- `internal/handler/skills.go` 只扫描 `cfg.OpenClawWork/skills`、`OpenClawDir/skills` 和 app skills。
- 但 OpenClaw 的真实技能落点可能来自 `openclaw.json` 里的 `workspace.path`，或者来自 `agents.list[].workspace`。
- 当运行时工作目录与 `cfg.OpenClawWork` 不一致时，技能页只能看到默认扫描路径中的少量技能。

## 修复

- 扩展技能扫描路径解析：
  - 保留 `cfg.OpenClawWork/skills`
  - 新增 `openclaw.json -> workspace.path/skills`
  - 新增 `openclaw.json -> agents.list[].workspace/skills`
- 对相对路径统一基于 `OpenClawDir` 的父目录做绝对化处理。
- 增加去重逻辑，避免同一技能目录被重复扫描。
- 补充单测，覆盖 workspace path 与 agent workspace 两类来源。

## 验证建议

- 在 OpenClaw 的 `workspace.path/skills` 下放置技能目录后刷新技能页，确认可见。
- 给某个 agent 单独配置 workspace，并在其 `skills` 子目录下放置技能，确认也能被发现。
- 若仍缺失，再检查对应目录下是否存在合法技能文件结构。
