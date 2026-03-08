package config

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// NormalizeOpenClawConfig 对 openclaw.json 做兼容性清洗。
// 返回 true 表示内容被修改。
//
// 兼容点：
//  1. 旧版面板曾写入 agents.defaults.model.contextTokens / maxTokens，
//     但新版 OpenClaw 仅允许 model 为 string 或 {primary,fallbacks}。
//  2. 将 legacy contextTokens 迁移到 agents.defaults.contextTokens。
//  3. 旧版面板曾把默认 Agent 写成 agents.default，需要迁移为 agents.list[].default。
//  4. 旧版 Agents 表单曾写入 Codex 风格 sandbox.mode（workspace-write/read-only/
//     danger-full-access），需要迁移为当前 OpenClaw sandbox schema。
func NormalizeOpenClawConfig(cfg map[string]interface{}) bool {
	return normalizeOpenClawConfig(cfg, "")
}

// NormalizeOpenClawConfigForWrite 与 NormalizeOpenClawConfig 类似，但允许结合本地
// OpenClaw 状态目录补全 disk-only agents 的默认 Agent 迁移。
func NormalizeOpenClawConfigForWrite(cfg map[string]interface{}, openClawDir string) bool {
	return normalizeOpenClawConfig(cfg, openClawDir)
}

func normalizeOpenClawConfig(cfg map[string]interface{}, openClawDir string) bool {
	if cfg == nil {
		return false
	}
	changed := false

	agents, ok := cfg["agents"].(map[string]interface{})
	if ok && agents != nil {
		if normalizeLegacyAgentDefault(agents, openClawDir) {
			changed = true
		}

		if normalizeDiskOnlyAgentList(agents, openClawDir) {
			changed = true
		}

		if defaults, ok := agents["defaults"].(map[string]interface{}); ok && defaults != nil {
			if sandbox, ok := defaults["sandbox"].(map[string]interface{}); ok && sandbox != nil {
				if normalizeSandboxConfig(sandbox) {
					changed = true
				}
			}

			modelRaw, exists := defaults["model"]
			if exists {
				switch m := modelRaw.(type) {
				case string:
					if strings.TrimSpace(m) == "" {
						delete(defaults, "model")
						changed = true
					}
				case map[string]interface{}:
					if defaults["contextTokens"] == nil {
						if v, ok := toPositiveInt(m["contextTokens"]); ok {
							defaults["contextTokens"] = v
							changed = true
						}
					}

					clean := map[string]interface{}{}
					if p, ok := m["primary"].(string); ok {
						p = strings.TrimSpace(p)
						if p != "" {
							clean["primary"] = p
						}
					}
					if fbAny, ok := m["fallbacks"].([]interface{}); ok {
						fallbacks := make([]interface{}, 0, len(fbAny))
						for _, item := range fbAny {
							s, ok := item.(string)
							if !ok {
								changed = true
								continue
							}
							s = strings.TrimSpace(s)
							if s == "" {
								changed = true
								continue
							}
							fallbacks = append(fallbacks, s)
						}
						if len(fallbacks) > 0 {
							clean["fallbacks"] = fallbacks
						}
					}

					if len(m) != len(clean) {
						changed = true
					}
					for k := range m {
						if k != "primary" && k != "fallbacks" {
							changed = true
							break
						}
					}

					if len(clean) == 0 {
						delete(defaults, "model")
						changed = true
					} else {
						defaults["model"] = clean
					}
				default:
					delete(defaults, "model")
					changed = true
				}
			}

			if compaction, ok := defaults["compaction"].(map[string]interface{}); ok && compaction != nil {
				if mode, ok := compaction["mode"].(string); ok {
					switch strings.TrimSpace(mode) {
					case "", "default", "safeguard":
						// no-op
					case "aggressive":
						compaction["mode"] = "safeguard"
						changed = true
					case "off":
						compaction["mode"] = "default"
						changed = true
					default:
						delete(compaction, "mode")
						changed = true
					}
				}
			}
		}

		if rawList, ok := agents["list"].([]interface{}); ok {
			for _, raw := range rawList {
				item, ok := raw.(map[string]interface{})
				if !ok || item == nil {
					continue
				}
				if sandbox, ok := item["sandbox"].(map[string]interface{}); ok && sandbox != nil {
					if normalizeSandboxConfig(sandbox) {
						changed = true
					}
				}
			}
		}
	}

	if gateway, ok := cfg["gateway"].(map[string]interface{}); ok && gateway != nil {
		if mode, ok := gateway["mode"].(string); ok && strings.TrimSpace(mode) == "hosted" {
			gateway["mode"] = "remote"
			changed = true
		}
		if custom, ok := gateway["customBindHost"].(string); !ok || strings.TrimSpace(custom) == "" {
			if bindAddr, ok := gateway["bindAddress"].(string); ok && strings.TrimSpace(bindAddr) != "" {
				gateway["customBindHost"] = strings.TrimSpace(bindAddr)
				changed = true
			}
		}
		if _, ok := gateway["bindAddress"]; ok {
			delete(gateway, "bindAddress")
			changed = true
		}
	}

	if hooks, ok := cfg["hooks"].(map[string]interface{}); ok && hooks != nil {
		if p, ok := hooks["path"].(string); !ok || strings.TrimSpace(p) == "" {
			if legacyPath, ok := hooks["basePath"].(string); ok && strings.TrimSpace(legacyPath) != "" {
				hooks["path"] = strings.TrimSpace(legacyPath)
				changed = true
			}
		}
		if tok, ok := hooks["token"].(string); !ok || strings.TrimSpace(tok) == "" {
			if legacyToken, ok := hooks["secret"].(string); ok && strings.TrimSpace(legacyToken) != "" {
				hooks["token"] = legacyToken
				changed = true
			}
		}
		if _, ok := hooks["basePath"]; ok {
			delete(hooks, "basePath")
			changed = true
		}
		if _, ok := hooks["secret"]; ok {
			delete(hooks, "secret")
			changed = true
		}
	}

	return changed
}

func toPositiveInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		if x > 0 {
			return x, true
		}
	case int32:
		if x > 0 {
			return int(x), true
		}
	case int64:
		if x > 0 {
			return int(x), true
		}
	case float64:
		if x > 0 {
			return int(x), true
		}
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return 0, false
		}
		n, err := strconv.Atoi(x)
		if err != nil {
			return 0, false
		}
		if n > 0 {
			return n, true
		}
	}
	return 0, false
}

func normalizeLegacyAgentDefault(agents map[string]interface{}, openClawDir string) bool {
	if agents == nil {
		return false
	}
	rawList, _ := agents["list"].([]interface{})
	legacyDefault := strings.TrimSpace(stringValue(agents["default"]))
	changed := false
	if len(rawList) == 0 && legacyDefault != "" && openClawDir != "" {
		if synthesized := synthesizeAgentListFromDisk(openClawDir, legacyDefault); len(synthesized) > 0 {
			agents["list"] = synthesized
			rawList = synthesized
			changed = true
		}
	}
	if len(rawList) == 0 {
		return changed
	}
	if _, ok := agents["default"]; ok {
		delete(agents, "default")
		changed = true
	}

	desiredDefault := ""
	for _, raw := range rawList {
		item, ok := raw.(map[string]interface{})
		if !ok || item == nil {
			continue
		}
		if !boolValue(item["default"]) {
			continue
		}
		if id := strings.TrimSpace(stringValue(item["id"])); id != "" {
			desiredDefault = id
			break
		}
	}
	if desiredDefault == "" && legacyDefault != "" {
		for _, raw := range rawList {
			item, ok := raw.(map[string]interface{})
			if !ok || item == nil {
				continue
			}
			if strings.TrimSpace(stringValue(item["id"])) == legacyDefault {
				desiredDefault = legacyDefault
				break
			}
		}
	}

	for _, raw := range rawList {
		item, ok := raw.(map[string]interface{})
		if !ok || item == nil {
			continue
		}
		id := strings.TrimSpace(stringValue(item["id"]))
		if desiredDefault != "" && id == desiredDefault {
			if !boolValue(item["default"]) {
				item["default"] = true
				changed = true
			}
			continue
		}
		if _, ok := item["default"]; ok {
			delete(item, "default")
			changed = true
		}
	}
	return changed
}

func synthesizeAgentListFromDisk(openClawDir, legacyDefault string) []interface{} {
	agentsDir := filepath.Join(openClawDir, "agents")
	idSet := map[string]struct{}{}
	if legacyDefault != "" {
		idSet[legacyDefault] = struct{}{}
	}
	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id := strings.TrimSpace(entry.Name())
			if id == "" {
				continue
			}
			idSet[id] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return nil
	}

	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		if ids[i] == "main" {
			return true
		}
		if ids[j] == "main" {
			return false
		}
		return ids[i] < ids[j]
	})

	desiredDefault := strings.TrimSpace(legacyDefault)
	if desiredDefault == "" {
		for _, id := range ids {
			if id == "main" {
				desiredDefault = id
				break
			}
		}
		if desiredDefault == "" && len(ids) > 0 {
			desiredDefault = ids[0]
		}
	}

	list := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		item := map[string]interface{}{"id": id}
		if id == desiredDefault {
			item["default"] = true
		}
		list = append(list, item)
	}
	return list
}

func normalizeDiskOnlyAgentList(agents map[string]interface{}, openClawDir string) bool {
	if agents == nil || strings.TrimSpace(openClawDir) == "" {
		return false
	}
	if rawList, _ := agents["list"].([]interface{}); len(rawList) > 0 {
		return false
	}
	if synthesized := synthesizeAgentListFromDisk(openClawDir, ""); len(synthesized) > 0 {
		agents["list"] = synthesized
		return true
	}
	return false
}

func normalizeSandboxConfig(sandbox map[string]interface{}) bool {
	if sandbox == nil {
		return false
	}
	mode := strings.TrimSpace(stringValue(sandbox["mode"]))
	switch mode {
	case "read-only":
		sandbox["mode"] = "all"
		if strings.TrimSpace(stringValue(sandbox["workspaceAccess"])) != "ro" {
			sandbox["workspaceAccess"] = "ro"
		}
		return true
	case "workspace-write":
		sandbox["mode"] = "all"
		if strings.TrimSpace(stringValue(sandbox["workspaceAccess"])) != "rw" {
			sandbox["workspaceAccess"] = "rw"
		}
		return true
	case "danger-full-access":
		sandbox["mode"] = "off"
		return true
	default:
		return false
	}
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolValue(v interface{}) bool {
	b, _ := v.(bool)
	return b
}
