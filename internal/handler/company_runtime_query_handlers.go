package handler

import (
	"database/sql"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func companyRuntimeScopeFromQuery(c *gin.Context) companyRuntimeScope {
	allowDelegation := false
	switch strings.TrimSpace(strings.ToLower(c.Query("allowDelegation"))) {
	case "1", "true", "yes", "on":
		allowDelegation = true
	}
	return companyRuntimeScope{
		Channel:         strings.TrimSpace(c.Query("channel")),
		AccountID:       strings.TrimSpace(c.Query("accountId")),
		WorkspaceID:     strings.TrimSpace(c.Query("workspaceId")),
		SessionKind:     strings.TrimSpace(c.Query("sessionKind")),
		RoutedAgentID:   strings.TrimSpace(c.Query("routedAgentId")),
		ManagerAgentID:  strings.TrimSpace(c.Query("managerAgentId")),
		AllowDelegation: allowDelegation,
	}
}

func companyQueryError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "team not found"})
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "channel required") {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
}

func companyAllowLocalInternalOnly(c *gin.Context) bool {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "local access required"})
		return false
	}
	if ip := net.ParseIP(clientIP); ip != nil && ip.IsLoopback() {
		return true
	}
	if clientIP == "localhost" {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "local access required"})
	return false
}

func GetCompanyTeamAgents(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := strings.TrimSpace(c.Param("id"))
		agents, err := listTeamAgents(db, cfg, teamID)
		if err != nil {
			companyQueryError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "teamId": teamID, "agents": agents})
	}
}

func GetCompanyVisibleAgents(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := strings.TrimSpace(c.Param("id"))
		channel := strings.TrimSpace(c.Query("channel"))
		accountID := strings.TrimSpace(c.Query("accountId"))
		agents, err := listVisibleAgents(db, cfg, teamID, channel, accountID)
		if err != nil {
			companyQueryError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "teamId": teamID, "channel": channel, "accountId": accountID, "agents": agents})
	}
}

func GetCompanyCallableAgents(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := strings.TrimSpace(c.Param("id"))
		scope := companyRuntimeScopeFromQuery(c)
		agents, err := listCallableAgents(db, cfg, teamID, scope)
		if err != nil {
			companyQueryError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "teamId": teamID, "scope": scope, "agents": agents})
	}
}

func GetCompanyTeamRuntimeSnapshot(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := strings.TrimSpace(c.Param("id"))
		scope := companyRuntimeScopeFromQuery(c)
		snapshot, err := buildTeamRuntimeSnapshot(db, cfg, teamID, scope)
		if err != nil {
			companyQueryError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "snapshot": snapshot})
	}
}

func GetCompanyAgentRuntimeSnapshotLocal(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !companyAllowLocalInternalOnly(c) {
			return
		}
		agentID := strings.TrimSpace(c.Query("agentId"))
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "agentId required"})
			return
		}
		resolution, err := resolveTeamByAgentScope(db, cfg, agentID, strings.TrimSpace(c.Query("teamId")))
		if err != nil {
			companyQueryError(c, err)
			return
		}
		scope := companyRuntimeScopeFromQuery(c)
		if scope.RoutedAgentID == "" {
			scope.RoutedAgentID = agentID
		}
		if scope.ManagerAgentID == "" {
			scope.ManagerAgentID = agentID
		}
		snapshot, err := buildTeamRuntimeSnapshot(db, cfg, resolution.ResolvedTeamID, scope)
		if err != nil {
			companyQueryError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "resolution": resolution, "snapshot": snapshot})
	}
}
