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
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

const openClawPairingPendingTTL = time.Hour

type openClawPairingRequest struct {
	ID         string            `json:"id"`
	Code       string            `json:"code"`
	CreatedAt  string            `json:"createdAt"`
	LastSeenAt string            `json:"lastSeenAt,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type openClawPairingStore struct {
	Version  int                      `json:"version"`
	Requests []openClawPairingRequest `json:"requests"`
}

type openClawAllowFromStore struct {
	Version   int      `json:"version"`
	AllowFrom []string `json:"allowFrom"`
}

func openClawCredentialsDir(cfg *config.Config) string {
	return filepath.Join(cfg.OpenClawDir, "credentials")
}

func normalizePairingKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, ":", "_")
	value = strings.ReplaceAll(value, "*", "_")
	value = strings.ReplaceAll(value, "?", "_")
	value = strings.ReplaceAll(value, "\"", "_")
	value = strings.ReplaceAll(value, "<", "_")
	value = strings.ReplaceAll(value, ">", "_")
	value = strings.ReplaceAll(value, "|", "_")
	value = strings.ReplaceAll(value, "..", "_")
	return value
}

func openClawPairingStorePath(cfg *config.Config, channelID string) string {
	return filepath.Join(openClawCredentialsDir(cfg), normalizePairingKey(channelID)+"-pairing.json")
}

func openClawAllowFromPath(cfg *config.Config, channelID, accountID string) string {
	base := normalizePairingKey(channelID)
	account := strings.TrimSpace(strings.ToLower(accountID))
	if account == "" {
		return filepath.Join(openClawCredentialsDir(cfg), base+"-allowFrom.json")
	}
	return filepath.Join(openClawCredentialsDir(cfg), base+"-"+normalizePairingKey(account)+"-allowFrom.json")
}

func loadOpenClawPairingStore(path string) (openClawPairingStore, error) {
	store := openClawPairingStore{Version: 1, Requests: []openClawPairingRequest{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	if err := json.Unmarshal(raw, &store); err != nil {
		return openClawPairingStore{Version: 1, Requests: []openClawPairingRequest{}}, nil
	}
	if store.Version == 0 {
		store.Version = 1
	}
	if store.Requests == nil {
		store.Requests = []openClawPairingRequest{}
	}
	return store, nil
}

func saveOpenClawPairingStore(path string, store openClawPairingStore) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func loadOpenClawAllowFromStore(path string) (openClawAllowFromStore, error) {
	store := openClawAllowFromStore{Version: 1, AllowFrom: []string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	if err := json.Unmarshal(raw, &store); err != nil {
		return openClawAllowFromStore{Version: 1, AllowFrom: []string{}}, nil
	}
	if store.Version == 0 {
		store.Version = 1
	}
	if store.AllowFrom == nil {
		store.AllowFrom = []string{}
	}
	return store, nil
}

func saveOpenClawAllowFromStore(path string, store openClawAllowFromStore) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func openClawPairingRequestAccountID(req openClawPairingRequest) string {
	return strings.TrimSpace(strings.ToLower(req.Meta["accountId"]))
}

func listOpenClawPairingRequests(cfg *config.Config, channelID, accountID string) ([]openClawPairingRequest, error) {
	path := openClawPairingStorePath(cfg, channelID)
	store, err := loadOpenClawPairingStore(path)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	filtered := make([]openClawPairingRequest, 0, len(store.Requests))
	changed := false
	normalizedAccountID := strings.TrimSpace(strings.ToLower(accountID))
	for _, req := range store.Requests {
		createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.CreatedAt))
		if err != nil || now.Sub(createdAt) > openClawPairingPendingTTL {
			changed = true
			continue
		}
		if normalizedAccountID != "" && openClawPairingRequestAccountID(req) != normalizedAccountID {
			continue
		}
		filtered = append(filtered, req)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt < filtered[j].CreatedAt
	})
	if changed {
		pruned := make([]openClawPairingRequest, 0, len(store.Requests))
		for _, req := range store.Requests {
			createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.CreatedAt))
			if err != nil || now.Sub(createdAt) > openClawPairingPendingTTL {
				continue
			}
			pruned = append(pruned, req)
		}
		store.Requests = pruned
		_ = saveOpenClawPairingStore(path, store)
	}
	return filtered, nil
}

func approveOpenClawPairingRequest(cfg *config.Config, channelID, code, accountID string) (*openClawPairingRequest, error) {
	path := openClawPairingStorePath(cfg, channelID)
	store, err := loadOpenClawPairingStore(path)
	if err != nil {
		return nil, err
	}
	normalizedCode := strings.TrimSpace(strings.ToUpper(code))
	normalizedAccountID := strings.TrimSpace(strings.ToLower(accountID))
	var approved *openClawPairingRequest
	next := make([]openClawPairingRequest, 0, len(store.Requests))
	for _, req := range store.Requests {
		if approved == nil && strings.ToUpper(strings.TrimSpace(req.Code)) == normalizedCode {
			entryAccountID := openClawPairingRequestAccountID(req)
			if normalizedAccountID == "" || normalizedAccountID == entryAccountID {
				copyReq := req
				approved = &copyReq
				continue
			}
		}
		next = append(next, req)
	}
	if approved == nil {
		return nil, nil
	}
	store.Requests = next
	if err := saveOpenClawPairingStore(path, store); err != nil {
		return nil, err
	}
	allowFromPath := openClawAllowFromPath(cfg, channelID, firstNonEmptyString(normalizedAccountID, openClawPairingRequestAccountID(*approved)))
	allowFromStore, err := loadOpenClawAllowFromStore(allowFromPath)
	if err != nil {
		return nil, err
	}
	exists := false
	for _, entry := range allowFromStore.AllowFrom {
		if strings.TrimSpace(entry) == strings.TrimSpace(approved.ID) {
			exists = true
			break
		}
	}
	if !exists {
		allowFromStore.AllowFrom = append(allowFromStore.AllowFrom, strings.TrimSpace(approved.ID))
		if err := saveOpenClawAllowFromStore(allowFromPath, allowFromStore); err != nil {
			return nil, err
		}
	}
	return approved, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func GetOpenClawPairingRequests(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID := strings.TrimSpace(c.Query("channel"))
		if channelID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "channel is required"})
			return
		}
		accountID := strings.TrimSpace(c.Query("accountId"))
		requests, err := listOpenClawPairingRequests(cfg, channelID, accountID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "channel": channelID, "accountId": accountID, "requests": requests})
	}
}

func ApproveOpenClawPairingRequest(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ChannelID string `json:"channelId"`
			Code      string `json:"code"`
			AccountID string `json:"accountId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		channelID := strings.TrimSpace(req.ChannelID)
		code := strings.TrimSpace(req.Code)
		if channelID == "" || code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "channelId and code are required"})
			return
		}
		approved, err := approveOpenClawPairingRequest(cfg, channelID, code, req.AccountID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if approved == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "未找到待审批的 pairing code"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "approved": approved})
	}
}
