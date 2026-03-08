# 飞书 / Lark 配置指南

本页对应 ClawPanel 当前的飞书配置面板，覆盖两类插件变体：

- **ClawTeam 版**：文档更完整、字段更明确
- **飞书官方早期版**：仍在快速演进，部分字段是否生效要以插件自身版本为准

无论使用哪一版，ClawPanel 都统一围绕 `channels.feishu` 这份共享配置做编辑，并把变体差异压缩在 UI 提示里。

## 1. 插件版本与共享配置

飞书页面顶部可以切换当前使用的插件版本，但配置仍统一写入：

```json
{
  "channels": {
    "feishu": {}
  }
}
```

这意味着：

- 两个版本共享同一个 `feishu` 渠道配置项
- 切换版本时，已有的共享字段会继续保留
- 如果某个版本暂时不识别某个字段，通常会忽略它，而不是破坏现有配置

## 2. Account 收口模型

ClawPanel 现在把飞书账号统一收口到同一套语义：

- `defaultAccount`：默认账号是谁
- `accounts`：全部账号
- 顶层 `appId` / `appSecret`：默认账号的镜像

### 最终保存形态

无论你在面板里使用“仅默认账号”还是“多账号并行”视图，保存后的飞书配置都会统一收敛到：

```json
{
  "channels": {
    "feishu": {
      "defaultAccount": "primary",
      "appId": "cli_primary",
      "appSecret": "secret_primary",
      "accounts": {
        "primary": {
          "appId": "cli_primary",
          "appSecret": "secret_primary",
          "botName": "主机器人",
          "enabled": true
        },
        "backup": {
          "appId": "cli_backup",
          "appSecret": "secret_backup",
          "botName": "备用机器人",
          "enabled": false
        }
      }
    }
  }
}
```

### 当前面板行为

- “仅默认账号 / 多账号并行”表示 **运行方式**，不是两套互斥 schema
- 默认账号的凭证会自动镜像到顶层 `appId` / `appSecret`
- 默认账号会被强制保持 `enabled=true`
- 若 `defaultAccount` 指向不存在账号，面板/后端只会在有顶层凭证可补齐时补出该账号；否则会回退到现有账号或清空，避免保存出“幽灵默认账号”
- 切回“仅默认账号”不会删除其他账号，只会把非默认账号写成 `enabled=false`
- 多账号列表里可以直接编辑 `botName`、凭证和 `enabled`

## 3. 默认值与推荐起点

下表是面板当前展示的“默认建议值”。  
注意：**这些值用于帮助理解和配置，真正运行时仍以对应插件版本的实际默认行为为准。**

| 字段 | 面板默认建议 | 说明 |
| --- | --- | --- |
| `domain` | `feishu` | 国内飞书默认保持 `feishu`；国际版可切换 `lark` |
| `requireMention` | `true` | 群里默认仅在 `@` 机器人时回复 |
| `groupPolicy` | `open` | 所有群可接入；如需收紧再切 `allowlist` 或 `closed` |
| `groupAllowFrom` | 空 | 仅 `groupPolicy=allowlist` 时生效 |
| `dmPolicy` | `pairing` | 私聊默认先配对，再建立会话 |
| `connectionMode` | `websocket` | ClawTeam 版常见基线；官方早期版若不识别可留空 |
| `historyLimit` | `300` | 常见回放窗口上限 |
| `mediaMaxMb` | `5` | 常见媒体大小上限 |

## 4. 群聊白名单：多群 ID 现在怎么写

ClawPanel 已把飞书群白名单从“单行文本”提升为更明确的输入方式：

- 支持 **英文逗号**
- 支持 **中文逗号**
- 支持 **换行**
- 保存时会统一写成数组

例如下面三种写法都可以：

```text
oc_group_a, oc_group_b
```

```text
oc_group_a，oc_group_b
```

```text
oc_group_a
oc_group_b
```

保存后的实际配置会变成：

```json
{
  "groupPolicy": "allowlist",
  "groupAllowFrom": ["oc_group_a", "oc_group_b"]
}
```

如果当前 `groupPolicy` 不是 `allowlist`，面板会明确提示：

- 这份白名单当前不会生效
- 保存后后端会自动清理 `groupAllowFrom`

## 5. 双账号 + Agent 路由的关键语义

在 Agent 路由里：

- `accountId` 为空：表示命中该渠道的 **默认账号**
- `accountId: "*"`：表示匹配该渠道的 **所有账号**

这也是为什么完整版 Account 模式和 `defaultAccount` 很重要：  
它不仅影响凭证管理，也直接影响 route preview 和真实路由行为。

## 6. 私聊上下文隔离建议

如果你在飞书里同时使用：

- 多个账号
- 多个 Agent
- 多条 binding 路由

建议在系统配置页把：

```json
{
  "session": {
    "dmScope": "per-account-channel-peer"
  }
}
```

显式打开。

注意：当前 OpenClaw 的有效字段是顶层 `session.dmScope`。  
`channels.feishu.dmScope` 不是现行 schema，`user` / `chat` 也不是当前支持的枚举值。

原因是实际隔离边界更接近：

```text
agentId + binding + session.dmScope + peer/session key
```

只配置群准入、白名单或 `accountId` 并不能完全替代 DM 隔离。

## 7. 推荐排查顺序

当你觉得飞书“串上下文”或“路由不对”时，建议按这个顺序检查：

1. 当前启用的是哪个插件版本
2. 是否需要开启完整版 Account
3. `defaultAccount` 是否正确
4. binding 里的 `accountId` 是空值、具体账号，还是 `*`
5. `session.dmScope` 是否已经显式收紧
6. `groupPolicy / dmPolicy / groupAllowFrom` 是否和预期一致
7. 当前 `sessions.json` 是否仍存在共享主会话键（例如 `agent:main:main`）
