package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

type HermesRouteMatch struct {
	Platform string `json:"platform,omitempty"`
	ChatType string `json:"chatType,omitempty"`
	ChatID   string `json:"chatId,omitempty"`
	UserID   string `json:"userId,omitempty"`
	Contains string `json:"contains,omitempty"`
}

type HermesRouteRule struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Enabled         bool             `json:"enabled"`
	Priority        int              `json:"priority,omitempty"`
	Match           HermesRouteMatch `json:"match"`
	ProfileName     string           `json:"profileName,omitempty"`
	PersonalityName string           `json:"personalityName,omitempty"`
	HomeTarget      string           `json:"homeTarget,omitempty"`
	GroupPerUser    *bool            `json:"groupPerUser,omitempty"`
	Notes           string           `json:"notes,omitempty"`
}

type HermesRoutingConfig struct {
	DefaultProfile     string            `json:"defaultProfile,omitempty"`
	DefaultPersonality string            `json:"defaultPersonality,omitempty"`
	DefaultHomeTarget  string            `json:"defaultHomeTarget,omitempty"`
	DefaultOverrides   map[string]bool   `json:"defaultOverrides,omitempty"`
	Rules              []HermesRouteRule `json:"rules"`
	UpdatedAt          string            `json:"updatedAt,omitempty"`
}

type HermesRoutingPreview struct {
	Base              HermesSessionPreview `json:"base"`
	RoutedProfile     string               `json:"routedProfile,omitempty"`
	RoutedPersonality string               `json:"routedPersonality,omitempty"`
	RoutedHomeTarget  string               `json:"routedHomeTarget,omitempty"`
	GroupPerUser      bool                 `json:"groupPerUser"`
	MatchedRuleID     string               `json:"matchedRuleId,omitempty"`
	MatchedBy         string               `json:"matchedBy,omitempty"`
	Reason            string               `json:"reason"`
}

func hermesRoutingConfigPath(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.DataDir, "hermes-routing.json")
}

func loadHermesRoutingConfig(cfg *config.Config) HermesRoutingConfig {
	path := hermesRoutingConfigPath(cfg)
	if strings.TrimSpace(path) == "" {
		return HermesRoutingConfig{Rules: []HermesRouteRule{}}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return HermesRoutingConfig{Rules: []HermesRouteRule{}}
	}
	var parsed HermesRoutingConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return HermesRoutingConfig{Rules: []HermesRouteRule{}}
	}
	if parsed.Rules == nil {
		parsed.Rules = []HermesRouteRule{}
	}
	return normalizeHermesRoutingConfig(parsed)
}

func normalizeHermesRoutingConfig(cfg HermesRoutingConfig) HermesRoutingConfig {
	if cfg.DefaultOverrides == nil {
		cfg.DefaultOverrides = map[string]bool{}
	}
	if cfg.Rules == nil {
		cfg.Rules = []HermesRouteRule{}
	}
	for i := range cfg.Rules {
		cfg.Rules[i].Name = strings.TrimSpace(cfg.Rules[i].Name)
		cfg.Rules[i].ProfileName = strings.TrimSpace(cfg.Rules[i].ProfileName)
		cfg.Rules[i].PersonalityName = strings.TrimSpace(cfg.Rules[i].PersonalityName)
		cfg.Rules[i].HomeTarget = strings.TrimSpace(cfg.Rules[i].HomeTarget)
		cfg.Rules[i].Notes = strings.TrimSpace(cfg.Rules[i].Notes)
		cfg.Rules[i].Match.Platform = strings.ToLower(strings.TrimSpace(cfg.Rules[i].Match.Platform))
		cfg.Rules[i].Match.ChatType = strings.ToLower(strings.TrimSpace(cfg.Rules[i].Match.ChatType))
		cfg.Rules[i].Match.ChatID = strings.TrimSpace(cfg.Rules[i].Match.ChatID)
		cfg.Rules[i].Match.UserID = strings.TrimSpace(cfg.Rules[i].Match.UserID)
		cfg.Rules[i].Match.Contains = strings.TrimSpace(cfg.Rules[i].Match.Contains)
		if cfg.Rules[i].ID == "" {
			cfg.Rules[i].ID = uuid.NewString()
		}
	}
	sort.SliceStable(cfg.Rules, func(i, j int) bool {
		if cfg.Rules[i].Priority == cfg.Rules[j].Priority {
			return cfg.Rules[i].Name < cfg.Rules[j].Name
		}
		return cfg.Rules[i].Priority > cfg.Rules[j].Priority
	})
	return cfg
}

func saveHermesRoutingConfig(cfg *config.Config, value HermesRoutingConfig) error {
	path := hermesRoutingConfigPath(cfg)
	if strings.TrimSpace(path) == "" {
		return os.ErrInvalid
	}
	value = normalizeHermesRoutingConfig(value)
	value.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ruleMatchesHermes(rule HermesRouteRule, preview HermesSessionPreview, message string) bool {
	if !rule.Enabled {
		return false
	}
	if rule.Match.Platform != "" && rule.Match.Platform != strings.ToLower(strings.TrimSpace(preview.Platform)) {
		return false
	}
	if rule.Match.ChatType != "" && rule.Match.ChatType != strings.ToLower(strings.TrimSpace(preview.ChatType)) {
		return false
	}
	if rule.Match.ChatID != "" && rule.Match.ChatID != preview.ChatID {
		return false
	}
	if rule.Match.UserID != "" && rule.Match.UserID != preview.UserID {
		return false
	}
	if rule.Match.Contains != "" && !strings.Contains(strings.ToLower(message), strings.ToLower(rule.Match.Contains)) {
		return false
	}
	return true
}

func applyHermesRouting(routing HermesRoutingConfig, preview HermesSessionPreview, message string) HermesRoutingPreview {
	result := HermesRoutingPreview{
		Base:              preview,
		RoutedProfile:     strings.TrimSpace(routing.DefaultProfile),
		RoutedPersonality: strings.TrimSpace(routing.DefaultPersonality),
		RoutedHomeTarget:  strings.TrimSpace(routing.DefaultHomeTarget),
		GroupPerUser:      preview.GroupPerUser,
		Reason:            "使用 Hermes 默认配置推导会话",
	}
	if override, ok := routing.DefaultOverrides["groupPerUser"]; ok {
		result.GroupPerUser = override
	}

	for _, rule := range routing.Rules {
		if !ruleMatchesHermes(rule, preview, message) {
			continue
		}
		if rule.ProfileName != "" {
			result.RoutedProfile = rule.ProfileName
		}
		if rule.PersonalityName != "" {
			result.RoutedPersonality = rule.PersonalityName
		}
		if rule.HomeTarget != "" {
			result.RoutedHomeTarget = rule.HomeTarget
		}
		if rule.GroupPerUser != nil {
			result.GroupPerUser = *rule.GroupPerUser
		}
		result.MatchedRuleID = rule.ID
		result.MatchedBy = buildHermesRouteMatchReason(rule)
		result.Reason = "命中 Hermes 路由规则"
		break
	}

	if result.GroupPerUser != preview.GroupPerUser && preview.ChatType == "group" && preview.UserID != "" {
		result.Base.GroupPerUser = result.GroupPerUser
		if result.GroupPerUser {
			result.Base.SessionKey = strings.Join([]string{preview.Platform, preview.ChatType, preview.ChatID, "user", preview.UserID}, ":")
			result.Base.UsedFallback = false
		} else {
			result.Base.SessionKey = strings.Join([]string{preview.Platform, preview.ChatType, preview.ChatID}, ":")
		}
	}
	if result.RoutedHomeTarget != "" {
		result.Base.HomeTarget = result.RoutedHomeTarget
	}
	return result
}

func buildHermesRouteMatchReason(rule HermesRouteRule) string {
	fields := make([]string, 0, 5)
	if rule.Match.Platform != "" {
		fields = append(fields, "platform")
	}
	if rule.Match.ChatType != "" {
		fields = append(fields, "chatType")
	}
	if rule.Match.ChatID != "" {
		fields = append(fields, "chatId")
	}
	if rule.Match.UserID != "" {
		fields = append(fields, "userId")
	}
	if rule.Match.Contains != "" {
		fields = append(fields, "contains")
	}
	if len(fields) == 0 {
		return "default-rule"
	}
	return strings.Join(fields, "+")
}

func GetHermesRouting(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "routing": loadHermesRoutingConfig(cfg)})
	}
}

func SaveHermesRouting(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req HermesRoutingConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		if err := saveHermesRoutingConfig(cfg, req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "routing": loadHermesRoutingConfig(cfg)})
	}
}

func PreviewHermesRouting(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Platform string `json:"platform"`
			ChatType string `json:"chatType"`
			ChatID   string `json:"chatId"`
			UserID   string `json:"userId"`
			Message  string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		base := previewHermesSession(buildHermesConfigState(), req.Platform, req.ChatType, req.ChatID, req.UserID)
		result := applyHermesRouting(loadHermesRoutingConfig(cfg), base, req.Message)
		c.JSON(http.StatusOK, gin.H{"ok": true, "preview": result})
	}
}
