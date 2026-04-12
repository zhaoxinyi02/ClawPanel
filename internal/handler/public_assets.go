package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func publicAssetPath(cfg *config.Config, category, name string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("配置为空")
	}
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("文件名非法")
	}
	return filepath.Join(cfg.DataDir, "public-assets", category, name), nil
}

func servePublicAsset(c *gin.Context, path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "文件不存在"})
		return
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sh":
		c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	case ".ps1":
		c.Header("Content-Type", "text/plain; charset=utf-8")
	case ".json":
		c.Header("Content-Type", "application/json; charset=utf-8")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}
	c.File(path)
}

func PublicScript(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		path, err := publicAssetPath(cfg, "scripts", c.Param("name"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		servePublicAsset(c, path)
	}
}

func PublicPluginAsset(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		path, err := publicAssetPath(cfg, "plugins", c.Param("name"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		servePublicAsset(c, path)
	}
}

func PublicBinaryAsset(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		path, err := publicAssetPath(cfg, "bin", c.Param("name"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		servePublicAsset(c, path)
	}
}
