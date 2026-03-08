package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

type FeishuDMDiagnosis struct {
	ConfiguredDMScope         string   `json:"configuredDmScope,omitempty"`
	EffectiveDMScope          string   `json:"effectiveDmScope"`
	RecommendedDMScope        string   `json:"recommendedDmScope"`
	DefaultAgent              string   `json:"defaultAgent"`
	ScannedAgentIDs           []string `json:"scannedAgentIds,omitempty"`
	AccountCount              int      `json:"accountCount"`
	AccountIDs                []string `json:"accountIds,omitempty"`
	DefaultAccount            string   `json:"defaultAccount,omitempty"`
	DMPolicy                  string   `json:"dmPolicy,omitempty"`
	ThreadSession             bool     `json:"threadSession"`
	UnsupportedChannelDMScope string   `json:"unsupportedChannelDmScope,omitempty"`
	SessionFilePath           string   `json:"sessionFilePath"`
	SessionIndexExists        bool     `json:"sessionIndexExists"`
	FeishuSessionCount        int      `json:"feishuSessionCount"`
	FeishuSessionKeys         []string `json:"feishuSessionKeys,omitempty"`
	HasSharedMainSessionKey   bool     `json:"hasSharedMainSessionKey"`
	MainSessionKey            string   `json:"mainSessionKey"`
}

func GetFeishuDMDiagnosis(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		diagnosis := buildFeishuDMDiagnosis(cfg)
		c.JSON(http.StatusOK, gin.H{"ok": true, "diagnosis": diagnosis})
	}
}

func buildFeishuDMDiagnosis(cfg *config.Config) FeishuDMDiagnosis {
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}

	feishuCfg, _ := getNestedMapValue(ocConfig, "channels", "feishu").(map[string]interface{})
	if feishuCfg == nil {
		feishuCfg = map[string]interface{}{}
	}

	accountIDs := listFeishuAccountIDsFromConfig(feishuCfg)
	defaultAccount := strings.TrimSpace(toString(feishuCfg["defaultAccount"]))
	if len(accountIDs) == 0 {
		if strings.TrimSpace(toString(feishuCfg["appId"])) != "" || strings.TrimSpace(toString(feishuCfg["appSecret"])) != "" {
			fallback := defaultAccount
			if fallback == "" {
				fallback = "default"
			}
			accountIDs = []string{fallback}
		}
	}

	configuredDMScope := strings.TrimSpace(toString(getNestedMapValue(ocConfig, "session", "dmScope")))
	effectiveDMScope := configuredDMScope
	if effectiveDMScope == "" {
		effectiveDMScope = "main"
	}

	recommendedDMScope := "per-channel-peer"
	if len(accountIDs) > 1 {
		recommendedDMScope = "per-account-channel-peer"
	}

	defaultAgent := strings.TrimSpace(loadDefaultAgentID(cfg))
	if defaultAgent == "" {
		defaultAgent = "main"
	}
	mainSessionKey := "agent:" + defaultAgent + ":main"
	sessionFilePath := resolveAgentPath(cfg, defaultAgent, "sessions", "sessions.json")

	diagnosis := FeishuDMDiagnosis{
		ConfiguredDMScope:         configuredDMScope,
		EffectiveDMScope:          effectiveDMScope,
		RecommendedDMScope:        recommendedDMScope,
		DefaultAgent:              defaultAgent,
		ScannedAgentIDs:           nil,
		AccountCount:              len(accountIDs),
		AccountIDs:                accountIDs,
		DefaultAccount:            defaultAccount,
		DMPolicy:                  strings.TrimSpace(toString(feishuCfg["dmPolicy"])),
		ThreadSession:             asBool(feishuCfg["threadSession"]),
		UnsupportedChannelDMScope: strings.TrimSpace(toString(feishuCfg["dmScope"])),
		SessionFilePath:           sessionFilePath,
		MainSessionKey:            mainSessionKey,
	}

	agentIDs, _ := loadAgentIDs(cfg)
	if len(agentIDs) == 0 {
		agentIDs = []string{defaultAgent}
	}

	allKeys := make([]string, 0)
	for _, agentID := range agentIDs {
		path := resolveAgentPath(cfg, agentID, "sessions", "sessions.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var sessionIndex map[string]interface{}
		if err := json.Unmarshal(raw, &sessionIndex); err != nil {
			continue
		}
		diagnosis.SessionIndexExists = true
		diagnosis.ScannedAgentIDs = append(diagnosis.ScannedAgentIDs, agentID)
		allKeys = append(allKeys, collectFeishuSessionKeys(sessionIndex)...)
	}
	sort.Strings(allKeys)
	diagnosis.FeishuSessionKeys = allKeys
	diagnosis.FeishuSessionCount = len(allKeys)
	for _, key := range allKeys {
		if isSharedMainSessionKey(key) {
			diagnosis.HasSharedMainSessionKey = true
			diagnosis.MainSessionKey = key
			break
		}
	}
	if diagnosis.MainSessionKey == "" {
		diagnosis.MainSessionKey = mainSessionKey
	}
	return diagnosis
}

func listFeishuAccountIDsFromConfig(ch map[string]interface{}) []string {
	accounts, _ := ch["accounts"].(map[string]interface{})
	if accounts == nil {
		return nil
	}
	out := make([]string, 0, len(accounts))
	for id := range accounts {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func collectFeishuSessionKeys(sessionIndex map[string]interface{}) []string {
	if sessionIndex == nil {
		return nil
	}
	keys := make([]string, 0)
	for key, raw := range sessionIndex {
		record, _ := raw.(map[string]interface{})
		if record == nil {
			continue
		}
		if strings.TrimSpace(toString(getNestedMapValue(record, "deliveryContext", "channel"))) == "feishu" {
			keys = append(keys, key)
			continue
		}
		if strings.TrimSpace(toString(getNestedMapValue(record, "origin", "provider"))) == "feishu" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func isSharedMainSessionKey(key string) bool {
	parts := strings.Split(key, ":")
	return len(parts) == 3 && parts[0] == "agent" && parts[2] == "main"
}
