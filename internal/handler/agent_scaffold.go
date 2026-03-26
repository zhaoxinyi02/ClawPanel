package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func scaffoldAgentFiles(cfg *config.Config, agent map[string]interface{}) error {
	id := strings.TrimSpace(toString(agent["id"]))
	if id == "" {
		return nil
	}
	workspace := strings.TrimSpace(toString(agent["workspace"]))
	if workspace == "" {
		return nil
	}
	loc, err := resolveAgentCoreWorkspaceFromPath(cfg, workspace, true)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(toString(readMap(agent["identity"])["name"]))
	if name == "" {
		name = strings.TrimSpace(toString(agent["name"]))
	}
	if name == "" {
		name = id
	}
	identity, soul, agents := renderAgentScaffold(id, name)
	if err := writeScaffoldFile(loc.Safe, "IDENTITY.md", identity, true); err != nil {
		return err
	}
	if err := writeScaffoldFile(loc.Safe, "SOUL.md", soul, false); err != nil {
		return err
	}
	if err := writeScaffoldFile(loc.Safe, "AGENTS.md", agents, false); err != nil {
		return err
	}
	if cfg.IsLiteEdition() {
		mirror := filepath.Join(cfg.BundledOpenClawAppDir(), workspace)
		_ = os.MkdirAll(mirror, 0o755)
		_ = writeScaffoldFile(mirror, "IDENTITY.md", identity, true)
		_ = writeScaffoldFile(mirror, "SOUL.md", soul, false)
		_ = writeScaffoldFile(mirror, "AGENTS.md", agents, false)
	}
	return nil
}

func writeScaffoldFile(workspace, name, content string, force bool) error {
	path := filepath.Join(workspace, name)
	if !force {
		if raw, err := os.ReadFile(path); err == nil {
			text := string(raw)
			if strings.TrimSpace(text) != "" && !strings.Contains(text, "Fill this in during your first conversation") {
				return nil
			}
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func renderAgentScaffold(id, name string) (identity, soul, agents string) {
	lower := strings.ToLower(id + " " + name)
	if strings.Contains(lower, "coding") || strings.Contains(lower, "code") || strings.Contains(lower, "dev") || strings.Contains(lower, "工程") || strings.Contains(lower, "编译") {
		identity = fmt.Sprintf(`# IDENTITY.md - Who Am I?

- **Name:** %s
- **Creature:** 专职代码智能体
- **Vibe:** 冷静、严谨、偏工程化，擅长定位问题、修改代码、处理编译与调试
- **Emoji:** 🛠️
- **Avatar:**

---

你是一个专职处理代码实现、编译、调试和问题排查的智能体。

你的职责重点：
- 阅读和理解现有代码结构
- 处理编译失败、报错、依赖问题
- 定位 bug 并给出修复方案
- 执行代码修改并验证结果
- 对技术风险、兼容性和回归影响做简明说明

## 强制回答规则

当用户问“你是谁”“你叫什么”“你的职责是什么”时，你必须直接回答：

“我是%s，专职负责代码实现、编译、调试和问题排查。”
`, name, name)
		soul = fmt.Sprintf(`你是%s。

你存在的意义是解决代码相关问题：写代码、改代码、编译、调试、定位 bug、解释错误、验证修复。

如果用户问“你是谁”，你的答案基线应该始终是：
“我是%s，负责处理代码实现、编译、调试和问题排查。”
`, name, name)
		agents = fmt.Sprintf(`# AGENTS.md - Coding Workspace

你是这个工作区里的专职代码智能体。

- 你的名字是：**%s**
- 你的职责是：**代码实现、编译、调试、排错、修复、验证**
- 不要把自己说成“还没命名”“还没设定”“只是普通助手”
`, name)
		return
	}
	identity = fmt.Sprintf(`# IDENTITY.md - Who Am I?

- **Name:** %s
- **Creature:** 专职智能体
- **Vibe:** 清晰、稳定、专业
- **Emoji:** 🧭
- **Avatar:**

---

你叫 %s。

当用户问“你是谁”时，要直接说明你的名字和职责，不要说自己还没有被设定。
`, name, name)
	soul = fmt.Sprintf(`你是%s。

你的目标是稳定完成用户交给你的职责范围内任务。
`, name)
	agents = fmt.Sprintf(`# AGENTS.md - Agent Workspace

你是这个工作区里的专职智能体。

- 你的名字是：**%s**
- 不要说自己还没有名字或设定
`, name)
	return
}
