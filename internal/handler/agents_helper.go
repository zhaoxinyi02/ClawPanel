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
	list := parseAgentsListFromConfig(ocConfig)
	if len(list) > 0 {
		return collectAgentIDsFromList(list)
	}
	ids, agentSet := collectAgentIDsFromConfigAndDisk(cfg, ocConfig)
	defaultID := strings.TrimSpace(loadDefaultAgentID(cfg))
	if defaultID != "" {
		if _, ok := agentSet[defaultID]; !ok {
			agentSet[defaultID] = struct{}{}
			ids = append(ids, defaultID)
			sortAgentIDs(ids)
		}
	}
	return ids, agentSet
}

func loadDefaultAgentID(cfg *config.Config) string {
	if isLegacySingleAgentMode() {
		return "main"
	}
	ocConfig, _ := cfg.ReadOpenClawJSON()
	list := parseAgentsListFromConfig(ocConfig)
	defaultID := strings.TrimSpace(getDefaultAgentID(ocConfig, list))
	defaultConfigured := hasExplicitDefaultAgent(ocConfig, list)
	if len(list) > 0 {
		if defaultID != "" {
			for _, item := range list {
				if strings.TrimSpace(toString(item["id"])) == defaultID {
					return defaultID
				}
			}
		}
		for _, item := range list {
			if id := strings.TrimSpace(toString(item["id"])); id != "" {
				return id
			}
		}
		return "main"
	}

	agentIDs, agentSet := collectAgentIDsFromConfigAndDisk(cfg, ocConfig)
	if defaultID != "" {
		if defaultConfigured {
			return defaultID
		}
		if _, ok := agentSet[defaultID]; ok {
			return defaultID
		}
	}
	for _, id := range agentIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			return id
		}
	}
	return "main"
}

func hasExplicitDefaultAgent(ocConfig map[string]interface{}, list []map[string]interface{}) bool {
	if configuredDefaultAgentIDFromList(list) != "" {
		return true
	}
	legacyDefault := legacyConfiguredDefaultAgentID(ocConfig)
	if legacyDefault == "" {
		return false
	}
	if len(list) == 0 {
		return true
	}
	for _, item := range list {
		if strings.TrimSpace(toString(item["id"])) == legacyDefault {
			return true
		}
	}
	return false
}

func collectAgentIDsFromList(list []map[string]interface{}) ([]string, map[string]struct{}) {
	agentSet := map[string]struct{}{}
	for _, item := range list {
		if id := strings.TrimSpace(toString(item["id"])); id != "" {
			agentSet[id] = struct{}{}
		}
	}
	if len(agentSet) == 0 {
		return nil, map[string]struct{}{}
	}

	ids := make([]string, 0, len(agentSet))
	for id := range agentSet {
		ids = append(ids, id)
	}
	sortAgentIDs(ids)
	return ids, agentSet
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

	if len(agentSet) == 0 && legacyConfiguredDefaultAgentID(ocConfig) == "" {
		agentSet["main"] = struct{}{}
	}

	ids := make([]string, 0, len(agentSet))
	for id := range agentSet {
		ids = append(ids, id)
	}
	sortAgentIDs(ids)
	return ids, agentSet
}

func sortAgentIDs(ids []string) {
	sort.Slice(ids, func(i, j int) bool {
		if ids[i] == "main" {
			return true
		}
		if ids[j] == "main" {
			return false
		}
		return ids[i] < ids[j]
	})
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
	if id := configuredDefaultAgentIDFromList(list); id != "" {
		return id
	}
	if legacyDefault := legacyConfiguredDefaultAgentID(ocConfig); legacyDefault != "" {
		if len(list) == 0 {
			return legacyDefault
		}
		for _, item := range list {
			if strings.TrimSpace(toString(item["id"])) == legacyDefault {
				return legacyDefault
			}
		}
	}
	if len(list) > 0 {
		for _, item := range list {
			if id := strings.TrimSpace(toString(item["id"])); id != "" {
				return id
			}
		}
	}
	return "main"
}

func configuredDefaultAgentIDFromList(list []map[string]interface{}) string {
	for _, item := range list {
		if asBool(item["default"]) {
			if id := strings.TrimSpace(toString(item["id"])); id != "" {
				return id
			}
		}
	}
	return ""
}

func legacyConfiguredDefaultAgentID(ocConfig map[string]interface{}) string {
	if ocConfig != nil {
		if agents, ok := ocConfig["agents"].(map[string]interface{}); ok {
			if v, ok := agents["default"].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
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
