# ClawPanel Documentation

本文档目录按「部署 / 渠道 / Agent 治理 / API / 插件开发」组织，方便直接跳到你关心的主题。

## 快速开始

- [DEPLOYMENT.md](./DEPLOYMENT.md) — 部署、升级、配置迁移与常见兼容问题
- [DEMO.md](./DEMO.md) — 界面演示与功能概览
- [FAQ.md](./FAQ.md) — 常见问题排查

## 渠道配置

- [qq-napcat-guide.md](./qq-napcat-guide.md) — QQ / NapCat 接入指南
- [FEISHU.md](./FEISHU.md) — 飞书 / Lark 双版本、账号模式、默认值、白名单与隔离建议

## Agent 与权限治理

- [AGENT-GOVERNANCE.md](./AGENT-GOVERNANCE.md) — 上下文预算、路由语义、`session.dmScope`、工具治理与浏览器边界

## 接口

- [API.md](./API.md) — ClawPanel / OpenClaw 相关 API 说明

## 插件开发

- [plugin-dev/README.md](./plugin-dev/README.md) — 插件开发与管理接口

## 阅读建议

如果你正在处理飞书双账号、多 Agent 路由或工具权限收口，建议阅读顺序：

1. [FEISHU.md](./FEISHU.md)
2. [AGENT-GOVERNANCE.md](./AGENT-GOVERNANCE.md)
3. [API.md](./API.md)
