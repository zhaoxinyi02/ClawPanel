# 贡献指南

感谢你关注 ClawPanel。

本仓库同时包含 Go 后端、React 前端、安装脚本、发布流程和 Wiki / 文档内容。为了减少回归和沟通成本，提交前请尽量遵循下面的约定。

## 适合贡献的方向

- 缺陷修复
- 文档改进
- 安装 / 部署兼容性改进
- 通道、插件与运行时体验优化
- 测试补充与 CI 稳定性改进

## 开发环境

建议环境：

- Go 1.22+
- Node.js 22+
- npm 10+
- Git

## 本地开发

### 前端开发

```bash
cd web
npm ci
npm run dev
```

### 后端开发

```bash
go run ./cmd/clawpanel/
```

### 完整构建

```bash
make build
```

常用命令：

- `make frontend`：仅构建前端
- `make build-pro`：构建 Pro 主程序
- `make build-lite`：构建 Lite 主程序
- `make clean`：清理构建产物

## 提交前建议检查

```bash
go test ./...
cd web && npm ci && npm run build
```

如果你的改动涉及发布脚本、安装脚本、前端嵌入、跨平台兼容或运行时探测，请尽量补充相应验证说明。

## 提交规范

建议保持提交标题简洁，优先说明变更意图。

常见前缀示例：

- `fix:` 缺陷修复
- `feat:` 新功能
- `docs:` 文档改进
- `refactor:` 重构
- `test:` 测试补充
- `ci:` 持续集成 / 自动化调整

## Pull Request 建议

请尽量在 PR 描述中说明：

- 改动目的
- 影响范围
- 是否有破坏性变化
- 如何验证
- 如涉及前端，附上截图或录屏

## Issue 与 Discussions

- 可稳定复现的缺陷：优先提交 Issue
- 使用问题、部署经验、功能讨论：优先使用 Discussions

讨论区入口：<https://github.com/zhaoxinyi02/ClawPanel/discussions>

## 文档与社区内容

- `docs/` 用于正式产品文档
- Wiki 更适合沉淀部署案例、社区索引和场景经验
- 若你要补充案例类内容，建议先发 Discussions，再整理到 Wiki
