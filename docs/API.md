# ClawPanel API 接口文档 (v5.1.0)

所有接口需要 JWT 认证（除 `/api/auth/login`），请在请求头中添加：
```
Authorization: Bearer <token>
```

## 认证

### POST `/api/auth/login`
登录获取 JWT Token。

**请求体：**
```json
{ "token": "你的 ADMIN_TOKEN" }
```

**响应：**
```json
{ "ok": true, "token": "eyJhbGci..." }
```

## 系统状态

### GET `/api/status`
获取系统整体状态（仪表盘 + 侧边栏数据源）。

**响应：**
```json
{
  "ok": true,
  "napcat": {
    "connected": true,
    "selfId": 123456789,
    "nickname": "Bot",
    "groupCount": 5,
    "friendCount": 20
  },
  "wechat": {
    "connected": true,
    "loggedIn": true,
    "name": "微信昵称"
  },
  "openclaw": {
    "configured": true,
    "qqPluginEnabled": true,
    "qqChannelEnabled": true,
    "currentModel": "anthropic/claude-sonnet-4-5",
    "enabledChannels": [
      { "id": "qq", "label": "QQ (NapCat)", "type": "builtin" },
      { "id": "feishu", "label": "飞书 / Lark", "type": "plugin" }
    ]
  },
  "admin": {
    "uptime": 3600,
    "memoryMB": 128
  }
}
```

## 活动日志

### GET `/api/events`
获取活动日志列表。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | number | 返回条数，默认 100 |
| `offset` | number | 偏移量，默认 0 |
| `source` | string | 来源筛选：`qq` / `wechat` / `openclaw` / `system` |
| `search` | string | 关键词搜索 |

### POST `/api/events/clear`
清空所有日志。

### POST `/api/events/log`
外部服务推送日志条目（无需认证）。

**请求体：**
```json
{
  "source": "openclaw",
  "type": "openclaw.action",
  "summary": "日志摘要",
  "detail": "详细信息（可选）"
}
```

## QQ 登录（NapCat 代理）

### POST `/api/napcat/login-status`
获取 QQ 登录状态。

### POST `/api/napcat/qrcode`
获取 QQ 登录二维码 URL。

### POST `/api/napcat/qrcode/refresh`
刷新二维码。

### GET `/api/napcat/quick-login-list`
获取可快速登录的 QQ 账号列表。

### POST `/api/napcat/quick-login`
快速登录。

**请求体：**
```json
{ "uin": "QQ号" }
```

### POST `/api/napcat/password-login`
账密登录。

**请求体：**
```json
{ "uin": "QQ号", "password": "密码" }
```

## QQ Bot 操作

### GET `/api/bot/groups`
获取群列表。

### GET `/api/bot/friends`
获取好友列表。

### POST `/api/bot/send`
发送 QQ 消息。

**请求体：**
```json
{
  "type": "private",
  "id": 123456789,
  "message": [{ "type": "text", "data": { "text": "Hello" } }]
}
```

### POST `/api/bot/reconnect`
重连 NapCat OneBot WebSocket。

## 微信

### GET `/api/wechat/status`
获取微信连接和登录状态。

### GET `/api/wechat/login-url`
获取微信扫码登录页面地址。

### POST `/api/wechat/send`
发送微信文本消息。

**请求体：**
```json
{
  "to": "wxid_xxx 或群名",
  "content": "消息内容",
  "isRoom": false
}
```

### POST `/api/wechat/send-file`
发送微信文件。

**请求体：**
```json
{
  "to": "wxid_xxx",
  "fileUrl": "https://example.com/file.pdf",
  "isRoom": false
}
```

### GET `/api/wechat/config`
获取微信相关配置。

### PUT `/api/wechat/config`
更新微信配置。

## OpenClaw 配置

### GET `/api/openclaw/config`
获取完整 openclaw.json 配置（系统配置页数据源）。

### PUT `/api/openclaw/config`
更新完整配置（系统配置页保存）。

**请求体：**
```json
{ "config": { ... } }
```

> v5.1.0+：配置写入前会自动备份到 `OPENCLAW_DIR/backups/pre-edit-*.json`，并保留最近 10 份。
>
> v5.1.0+：保存时会校验 `session.dmScope`，仅允许：
> `main / per-peer / per-channel-peer / per-account-channel-peer`。

### GET `/api/openclaw/feishu-dm-diagnosis`
读取飞书 DM 隔离诊断信息。

返回内容会汇总：

- 当前配置文件里的 `session.dmScope`
- 面板推荐值（单账号通常 `per-channel-peer`，多账号通常 `per-account-channel-peer`）
- 运行时 `sessions.json` 中检测到的飞书会话键
- 是否仍存在共享主会话键（例如 `agent:main:main`）
- 是否误写了 `channels.feishu.dmScope`
- `credentials/feishu-pairing.json` 中的待审批请求数量
- `credentials/feishu-*-allowFrom.json` 中每个账号已授权的 OpenID 列表

### GET `/api/openclaw/agents`
获取多智能体配置与统计信息（`defaults` / `default` / `list` / `bindings`）。

### POST `/api/openclaw/agents`
创建 Agent。

**请求体：**
```json
{
  "agent": {
    "id": "work",
    "workspace": "/data/work/work",
    "agentDir": "agents/work",
    "default": false,
    "model": { "primary": "openai/gpt-4o" },
    "tools": {},
    "sandbox": {}
  }
}
```

> 兼容说明：当前面板同时兼容两类 `agentDir` 写法：
> - bundle 根目录：`agents/work`
> - OpenClaw 新运行态返回的子目录：`~/.openclaw/agents/work/agent`
>
> 无论哪种写法，ClawPanel 都会把同级的 `sessions/`、`auth/` 等运行时目录解析到正确位置。
> 另外也允许把 `agentDir` 指向 OpenClaw 状态目录外的绝对路径；这只影响认证/模型等 agent 配置目录，`sessions/` 仍按 OpenClaw 规范保存在状态目录下。

> v5.1.0+：保存时会校验：
> - 当前 OpenClaw schema 不支持 `agent.contextTokens` / `agent.compaction`；面板会在保存时自动清理这两个 legacy per-agent 字段
> - 更新 Agent 时会保留未修改的 legacy `identity.avatar` 写法；若显式修改 avatar，仍按当前规则校验
> - `identity` 采用非破坏性规范化：legacy `description/vibe/tone/creature` 与自定义字段会继续保留

### PUT `/api/openclaw/agents/:id`
更新指定 Agent（`id` 不可修改）。

### DELETE `/api/openclaw/agents/:id?preserveSessions=true`
删除 Agent，可选保留该 Agent 的 sessions 文件。

### GET `/api/openclaw/agents/:id/core-files`
读取指定 Agent 工作区内的核心文件快照。

**返回要点：**
- 仅暴露固定文件集：`AGENTS.md` / `SOUL.md` / `TOOLS.md` / `IDENTITY.md` / `USER.md` / `HEARTBEAT.md` / `BOOT.md` / `BOOTSTRAP.md` / `MEMORY.md`
- `workspace` 为当前 Agent 的受管工作区路径
- 若 workspace 不存在、位于受管根目录外，或祖先链/目标文件包含 symlink，会返回错误而不是继续访问

### PUT `/api/openclaw/agents/:id/core-files`
保存指定 Agent 的单个核心文件。

**请求体：**
```json
{
  "name": "AGENTS.md",
  "content": "# Agent Notes"
}
```

> 仅允许写入固定核心文件集，内容大小存在上限；workspace 越界、symlink 或不受管路径会被拒绝。

### GET `/api/openclaw/agents/:id/identity/avatar`
读取 Agent 的本地头像文件。

> 该接口仅在 `identity.avatar` 指向 **agent workspace 内的本地相对路径文件** 时可用。
>
> 若 `identity.avatar` 是 `http(s)` URL、`data:` URI、越界路径、symlink、目录、过大文件或不支持格式，会返回错误。

### GET `/api/openclaw/bindings`
获取 bindings 路由规则（按顺序返回）。

### PUT `/api/openclaw/bindings`
全量替换 bindings（保序）。

**请求体：**
```json
{
  "bindings": [
    {
      "comment": "work-group",
      "agentId": "work",
      "match": {
        "channel": "qq",
        "peer": { "kind": "group", "id": "123" }
      }
    }
  ]
}
```

> 当前写入模型采用官方兼容 schema：
>
> v5.1.0+：`match` 开启严格校验：
> - `match.channel` 必填
> - 允许字段：`channel/sender/peer/parentPeer/guildId/teamId/accountId/roles`
> - `roles` 必须与 `guildId` 同时使用
> - `peer` / `parentPeer` 支持 `kind:id` 字符串或 `{ "kind": "...", "id": "..." }` 对象
> - 旧写法（例如 `peer: "group:*"`、`sender`、`parentPeer`）保存时会继续保留，避免升级后把历史 bindings 改坏
> - top-level 公开字段为 `type`（`route`/`acp`）、`comment`、`agentId`、`acp`
> - 旧字段 `agent` 仍会在保存时被标准化为 `agentId`
> - 旧字段 `name` 会作为 `comment` 别名兼容读取；重新保存时统一写成 `comment`
> - 历史 `agents.bindings` 仍会被兼容读取并迁移到顶层 `bindings`；当前面板保存时只写顶层 `bindings`

### POST `/api/openclaw/route/preview`
路由预览。根据 `meta` 返回命中 Agent、命中规则和 trace。

**请求体：**
```json
{
  "meta": {
    "channel": "qq",
    "sender": "alice",
    "peer": "group:123",
    "parentPeer": "",
    "guildId": "",
    "teamId": "",
    "accountId": "10001"
  }
}
```

> v5.1.0+：预览命中遵循“更具体规则优先”而非仅配置顺序。当前优先级：
> `sender > peer > parentPeer > guildId+roles > guildId > teamId > accountId > accountId:* > channel > default`。
>
> v5.1.0+：当 `meta.channel` 已知且 `meta.accountId` 省略时，预览会按该渠道的默认账号语义处理；对于飞书这类多账号渠道，这与真实路由行为保持一致。
>
> v5.1.0+：
> - `matchedBy` 可能返回：`binding.sender` / `binding.peer` / `binding.peer.parent` / `binding.guild+roles` / `binding.guild` / `binding.team` / `binding.account` / `binding.account.wildcard` / `binding.channel` / `default`
> - `matchedIndex` 为命中 binding 在原始数组中的索引；若走默认兜底则为 `-1`
> - `meta.peer` / `meta.parentPeer` 在预览输入中可写成字符串或对象
> - 当前面板为了兼容历史配置，会继续识别 bindings 中的 `sender` / `parentPeer` 以及字符串 `peer`

### GET `/api/openclaw/models`
获取模型配置。

### PUT `/api/openclaw/models`
更新模型配置。

### GET `/api/openclaw/channels`
获取通道配置（通道管理页数据源）。

### PUT `/api/openclaw/channels/:id`
更新指定通道配置。

### PUT `/api/openclaw/plugins/:id`
更新指定插件配置（技能中心启用/禁用）。

### POST `/api/openclaw/toggle-channel`
切换通道启用/禁用（v4.2.0+）。自动处理配置更新、系统日志、QQ 退出登录、网关重启。

**请求体：**
```json
{ "channelId": "qq", "enabled": false }
```

**响应：**
```json
{ "ok": true, "message": "QQ (NapCat) 通道已禁用" }
```

### POST `/api/napcat/logout`
退出 QQ 登录（v4.2.0+）。清除 QQ 会话数据并重启容器。

**响应：**
```json
{ "ok": true, "message": "QQ 已退出登录，容器正在重启..." }
```

### GET `/api/napcat/login-info`
获取当前 QQ 登录信息。

### POST `/api/system/restart-gateway`
请求宿主机重启 OpenClaw 网关（通过信号文件机制）。

### GET `/api/system/restart-gateway-status`
获取网关重启状态。

## 管理配置

### GET `/api/admin/config`
获取 ClawPanel 管理配置（通道详细参数等）。

### PUT `/api/admin/config`
更新管理配置。

### PUT `/api/admin/config/:section`
更新指定配置段（如 `qq`、`wechat`）。

## 审核

### GET `/api/requests`
获取待审核的好友/入群请求列表。

### POST `/api/requests/:flag/approve`
同意请求。

### POST `/api/requests/:flag/reject`
拒绝请求。

**请求体（可选）：**
```json
{ "reason": "拒绝原因" }
```

## 工作区

### GET `/api/workspace/files`
列出工作区文件。

### GET `/api/workspace/stats`
获取工作区统计信息。

### POST `/api/workspace/upload`
上传文件（multipart/form-data）。

### POST `/api/workspace/mkdir`
创建目录。

### POST `/api/workspace/delete`
删除文件/目录。

### GET `/api/workspace/download?path=xxx`
下载文件。

### GET `/api/workspace/preview?path=xxx`
预览文件（文本/图片）。

### GET `/api/workspace/config`
获取工作区配置（自动清理等）。

### PUT `/api/workspace/config`
更新工作区配置。

### POST `/api/workspace/clean`
手动触发工作区清理。

### GET `/api/workspace/notes`
获取文件备注列表。

### PUT `/api/workspace/notes`
设置文件备注。

**请求体：**
```json
{ "path": "文件路径", "note": "备注内容" }
```

## 系统管理

### GET `/api/system/env`
获取运行环境信息（OS、软件版本等）。

### GET `/api/system/version`
获取 OpenClaw 版本信息。

### POST `/api/system/check-update`
检查 OpenClaw 更新。

### POST `/api/system/do-update`
执行 OpenClaw 更新。

### GET `/api/system/update-status`
获取更新进度状态。

### POST `/api/system/backup`
创建 openclaw.json 配置备份。

### GET `/api/system/backups`
获取备份列表。

### POST `/api/system/restore`
恢复指定备份。

**请求体：**
```json
{ "backupName": "备份文件名" }
```

### GET `/api/system/skills`
获取已安装技能列表。

**查询参数：**
- `agentId`（可选）：按指定 Agent 工作区解析 Skills；省略时使用当前默认 Agent。

**返回要点：**
- `agentId`：实际使用的 Agent ID
- `workspace`：该 Agent 对应的工作区路径
- `skills`：按 OpenClaw 官方优先级聚合后的技能列表，包含 `skillKey`、`source`、`requires`、`metadata`
- `plugins`：已发现的插件列表

### PUT `/api/system/skills/:id/toggle`
切换技能开关。

**请求体：**
```json
{
  "enabled": true,
  "aliases": ["legacy-skill-dir-name"]
}
```

> 当前实现会写入 `skills.entries.<id>.enabled`；`aliases` 用于同步清理 legacy `skills.blocklist` 中的旧别名。

### GET `/api/system/clawhub/search`
通过 ClawHub 官方公开 API 搜索或浏览公开技能。

**查询参数：**
- `q`（可选）：搜索关键词；为空时返回热门公开技能
- `agentId`（可选）：用于标记当前 Agent 工作区下已安装的 ClawHub 技能
- `limit`（可选）：返回条数上限，默认 30，最大 100

**返回要点：**
- `registryBase`：本次搜索实际使用的 ClawHub 站点公开基地址；若同时配置 `CLAWHUB_REGISTRY` 与 `CLAWHUB_SITE`，这里优先返回 `CLAWHUB_SITE`。后端会校验其必须为绝对 `http(s)` URL，并在返回前剥离可能存在的内嵌凭证，前端详情链接与“前往官网”按钮应基于此字段构造
- `skills[*].installed` / `skills[*].installedVersion`：仅反映目标 Agent 工作区 `skills/` 与 `.clawhub/lock.json` 中的安装状态

### POST `/api/system/clawhub/install`
将指定 ClawHub 技能安装到目标 Agent 工作区的 `skills/` 目录，并同步写入工作区 `.clawhub/lock.json` 与技能目录 `.clawhub/origin.json`。

**请求体：**
```json
{
  "skillId": "weather",
  "agentId": "main",
  "version": "1.1.0"
}
```

### POST `/api/system/clawhub-sync`
兼容旧版接口，返回缓存化的 ClawHub 商店技能列表。

### GET `/api/system/cron`
获取定时任务列表。

> 优先读取 `openclaw.json.cron.jobs`；若为空，再回退读取 `OPENCLAW_DIR/cron/jobs.json`。

### PUT `/api/system/cron`
更新定时任务。

**请求体：**
```json
{ "jobs": [...] }
```

> v5.1.0+：保存前会校验每个任务：
>
> - `agentId` 为独立字段；若填写，必须是当前存在的 Agent
> - `sessionTarget` 只允许 `main` 或 `isolated`
> - 若旧数据把 `sessionTarget` 写成某个已知 `agentId`，保存时会迁移为：
>   - `agentId = <旧值>`
>   - `sessionTarget = "main"`
> - 若 `sessionTarget` 为空，也会标准化为 `"main"`
> - `delivery.mode=webhook` 统一使用 `delivery.to`（兼容读取 legacy `delivery.url` 并自动归一化）
> - `sessionTarget=main` 下若收到 `delivery.mode=announce`，会自动降级为 `delivery.mode=none`（与 OpenClaw 运行时行为一致）
> - 保存时会同时同步 `openclaw.json.cron.jobs` 与 `OPENCLAW_DIR/cron/jobs.json`
> - `cron/jobs.json` 采用临时文件 + rename 的原子写入，避免半写入状态

## 会话管理

### GET `/api/sessions?agent=<agentId>|all`
获取会话列表。`agent=all` 时聚合全部 Agent 会话，并返回 `agentId` 字段。
当 `agent` 省略时默认使用“当前有效默认 Agent”（优先 `agents.list[].default=true`，若未显式配置则回退到已存在 Agent，最终兜底 `main`）。

### GET `/api/sessions/:id?agent=<agentId>`
获取指定会话详情（`agent=all` 不支持）。当 `agent` 省略时默认使用“当前有效默认 Agent”（`agents.list[].default=true` 优先，未显式配置时自动回退）。

### DELETE `/api/sessions/:id?agent=<agentId>`
删除指定会话（`agent=all` 不支持）。当 `agent` 省略时默认使用“当前有效默认 Agent”（`agents.list[].default=true` 优先，未显式配置时自动回退）。

### GET `/api/system/docs`
获取 OpenClaw 目录下的文档列表。

### PUT `/api/system/docs`
保存文档内容。

### GET `/api/system/identity-docs`
获取身份文档列表（MD 文件）。

### PUT `/api/system/identity-docs`
保存身份文档。

### GET `/api/system/admin-token`
获取当前管理员 Token（用于系统配置页显示）。

### GET `/api/system/sudo-password`
检查是否已配置 sudo 密码。

### PUT `/api/system/sudo-password`
设置 sudo 密码（用于系统更新操作）。

## WebSocket

### `/ws?token=<JWT>`
ClawPanel 实时事件推送。

**消息类型：**
| type | 说明 |
|------|------|
| `napcat-status` | QQ 连接状态变更 |
| `wechat-status` | 微信连接状态变更 |
| `event` | QQ 事件（消息、通知等） |
| `wechat-event` | 微信事件（消息等） |
| `log-entry` | 活动日志新条目 |

### `/onebot`
OneBot11 WebSocket 代理，供宿主机 OpenClaw 连接到容器内 NapCat。
