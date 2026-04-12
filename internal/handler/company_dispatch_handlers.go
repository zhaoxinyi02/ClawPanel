package handler

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func DispatchCompanyAgentsLocal(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !companyAllowLocalInternalOnly(c) {
			return
		}
		var req companyAgentDispatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		req.ManagerAgentID = strings.TrimSpace(req.ManagerAgentID)
		req.RoutedAgentID = strings.TrimSpace(req.RoutedAgentID)
		req.Channel = strings.TrimSpace(req.Channel)
		req.AccountID = strings.TrimSpace(req.AccountID)
		req.SessionKind = strings.TrimSpace(req.SessionKind)
		req.Mode = strings.TrimSpace(req.Mode)
		if strings.TrimSpace(req.UserGoal) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "userGoal required"})
			return
		}
		if len(req.Targets) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "targets required"})
			return
		}
		result, err := handleManagerMultiAgentReplyRequest(db, cfg, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "result": result})
	}
}
