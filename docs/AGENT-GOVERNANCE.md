# Agent 治理与安全边界

本页总结 ClawPanel 当前围绕 Agent、上下文、路由和工具权限做的核心治理入口。

## 1. 上下文预算：系统默认 + 单 Agent 覆盖

ClawPanel 现在同时支持：

- 系统级 `agents.defaults.contextTokens`
- 单 Agent 级 `contextTokens`
- 系统级 / 单 Agent 级 `compaction`

这几个字段的作用更接近“上下文预算和压缩策略”，而不是每次都强塞固定 token 数量。  
OpenClaw 实际运行时仍会结合模型真实 `contextWindow` 取更小值。

### 当前面板可做的事

- 在 **System Config** 里修改默认上下文预算与 compaction
- 在 **Agents** 页面对单个 Agent 做覆盖
- 保存时前后端都会校验：
  - `contextTokens` 必须为正整数
  - `compaction.maxHistoryShare` 必须位于 `0..1`

## 2. 路由与默认账号语义

当渠道支持多账号（例如飞书）时，当前面板与后端已经对齐以下语义：

- `accountId` 留空 = 使用该渠道的 `defaultAccount`
- `accountId: "*"` = 匹配该渠道的所有账号

路由预览现在也遵循相同规则，不再出现“预览和真实路由不一致”的情况。

### 当前优先级

路由不是单纯按列表顺序命中，而是先按规则具体度比较，再用列表顺序打破同优先级平局：

```text
sender > peer > parentPeer > guildId+roles > guildId > teamId > accountId > accountId:* > channel > default
```

## 3. `session.dmScope`：私聊隔离的主开关

对于多账号 / 多 Agent / 多私聊入口，群准入和白名单并不能替代 DM 隔离。  
当前面板已把 `session.dmScope` 做成结构化可视化入口。

### 可选范围

- `main`
- `per-peer`
- `per-channel-peer`
- `per-account-channel-peer`

> `channels.feishu.dmScope` 不是当前 OpenClaw schema；若看到 `user` / `chat` 之类写法，应改回顶层 `session.dmScope`。

### 飞书双账号推荐值

如果你在飞书里使用：

- `channels.feishu.accounts/defaultAccount`
- binding 里的 `accountId`
- 多个 Agent

建议显式设置：

```json
{
  "session": {
    "dmScope": "per-account-channel-peer"
  }
}
```

## 4. 工具治理：系统默认与单 Agent 覆盖

当前面板已把最常用的工具治理入口结构化出来：

- 系统级：
  - `tools.profile`
  - `tools.allow`
  - `tools.deny`
- 单 Agent 级：
  - `tools.profile`
  - `tools.allow`
  - `tools.deny`

### Quick Presets

当前 UI 支持这几类 profile 起点：

- `minimal`
- `coding`
- `messaging`
- `full`
- 单 Agent 侧还支持 `inherit`

### “传统无头”建议

如果你的目标是把 OpenClaw 收敛到接近传统无头助手的范围，可从下面这个组合起步：

```json
{
  "tools": {
    "profile": "minimal",
    "allow": ["group:web", "group:fs"],
    "deny": ["group:runtime", "group:ui", "group:nodes", "group:automation"]
  }
}
```

含义：

- 保留网页检索和文件能力
- 明确拒绝命令执行、桌面/UI、节点、自动化等高风险工具面

### 注意

- `deny` 的优先级高于 `allow`
- `tools.elevated` 不是授权工具本身，而是 `exec` 的宿主机逃逸口
- 更细粒度的 `tools.sandbox.tools`、`tools.elevated` 仍在 Advanced JSON 中维护

## 5. 浏览器与命令边界

除了工具治理，本页还建议一起看这几个入口：

- **Browser Control**
  - `browser.enabled`
  - `browser.defaultProfile`
- **命令配置**
  - `commands.native`
  - `commands.nativeSkills`
  - `commands.restart`

推荐组合：

- 不需要浏览器时：`browser.enabled = false`
- 需要浏览器但不想碰系统日常 Chrome 时：显式锁定 `browser.defaultProfile = "openclaw"`

## 6. 什么时候应该回到 Advanced JSON

如果你要处理这些更细的能力边界，请继续用 Advanced JSON：

- `tools.sandbox.tools`
- `tools.elevated`
- `sandbox.browser.*`
- `sandbox.docker.image/user/env`
- 其他还没有做成结构化表单的 OpenClaw 高阶字段

当前面板的目标是把“最常用、最容易出错、最需要心智模型”的治理入口先可视化，而不是完全替代 Raw JSON。
