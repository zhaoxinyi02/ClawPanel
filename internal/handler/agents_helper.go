package handler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func isLegacySingleAgentMode() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("LEGACY_SINGLE_AGENT")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func loadAgentIDs(cfg *config.Config) ([]string, map[string]struct{}) {
	if isLegacySingleAgentMode() {
		return []string{"main"}, map[string]struct{}{"main": {}}
	}

	ocConfig, _ := cfg.ReadOpenClawJSON()
	return collectAgentIDsFromConfigAndDisk(cfg, ocConfig)
}

func collectAgentIDsFromConfigAndDisk(cfg *config.Config, ocConfig map[string]interface{}) ([]string, map[string]struct{}) {
	agentSet := map[string]struct{}{}

	for _, item := range parseAgentsListFromConfig(ocConfig) {
		if id := strings.TrimSpace(toString(item["id"])); id != "" {
			agentSet[id] = struct{}{}
		}
	}

	agentsDir := filepath.Join(cfg.OpenClawDir, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				name := strings.TrimSpace(e.Name())
				if name != "" {
					agentSet[name] = struct{}{}
				}
			}
		}
	}

	if len(agentSet) == 0 {
		agentSet["main"] = struct{}{}
	}

	ids := make([]string, 0, len(agentSet))
	for id := range agentSet {
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
	return ids, agentSet
}

func parseAgentsListFromConfig(ocConfig map[string]interface{}) []map[string]interface{} {
	if ocConfig == nil {
		return nil
	}
	agents, _ := ocConfig["agents"].(map[string]interface{})
	if agents == nil {
		return nil
	}
	rawList, _ := agents["list"].([]interface{})
	if len(rawList) == 0 {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(rawList))
	for _, raw := range rawList {
		if item, ok := raw.(map[string]interface{}); ok {
			result = append(result, deepCloneMap(item))
		}
	}
	return result
}

func getDefaultAgentID(ocConfig map[string]interface{}, list []map[string]interface{}) string {
	if isLegacySingleAgentMode() {
		return "main"
	}
	if ocConfig != nil {
		if agents, ok := ocConfig["agents"].(map[string]interface{}); ok {
			if v, ok := agents["default"].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	for _, item := range list {
		if asBool(item["default"]) {
			if id := strings.TrimSpace(toString(item["id"])); id != "" {
				return id
			}
		}
	}
	return "main"
}

func asBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func deepCloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = deepCloneAny(v)
	}
	return dst
}

func deepCloneAny(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCloneMap(t)
	case []interface{}:
		arr := make([]interface{}, len(t))
		for i := range t {
			arr[i] = deepCloneAny(t[i])
		}
		return arr
	default:
		return t
	}
}
