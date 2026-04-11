package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

type wsFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"sizeHuman"`
	IsDirectory bool   `json:"isDirectory"`
	ModifiedAt  string `json:"modifiedAt"`
	Extension   string `json:"extension"`
	AgeDays     int    `json:"ageDays"`
}

type wsConfig struct {
	AutoCleanEnabled bool     `json:"autoCleanEnabled"`
	AutoCleanDays    int      `json:"autoCleanDays"`
	ExcludePatterns  []string `json:"excludePatterns"`
}

func humanSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
}

func getWorkspaceDir(cfg *config.Config) string {
	workDir := cfg.OpenClawWork
	if workDir == "" {
		workDir = os.Getenv("OPENCLAW_WORK")
	}
	if workDir == "" {
		workDir = filepath.Join(filepath.Dir(cfg.OpenClawDir), "work")
	}
	os.MkdirAll(workDir, 0755)
	return workDir
}

func getWsConfigPath(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "workspace-config.json")
}

func getWsNotesPath(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "workspace-notes.json")
}

func loadWsConfig(cfg *config.Config) wsConfig {
	wc := wsConfig{AutoCleanEnabled: false, AutoCleanDays: 30, ExcludePatterns: []string{}}
	data, err := os.ReadFile(getWsConfigPath(cfg))
	if err == nil {
		json.Unmarshal(data, &wc)
	}
	if wc.ExcludePatterns == nil {
		wc.ExcludePatterns = []string{}
	}
	return wc
}

func saveWsConfig(cfg *config.Config, wc wsConfig) {
	data, _ := json.MarshalIndent(wc, "", "  ")
	os.WriteFile(getWsConfigPath(cfg), data, 0644)
}

func WorkspaceFiles(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		subPath := c.Query("path")
		targetDir := wsDir
		if subPath != "" {
			targetDir = filepath.Join(wsDir, subPath)
		}
		// Security check
		abs, _ := filepath.Abs(targetDir)
		wsAbs, _ := filepath.Abs(wsDir)
		if !strings.HasPrefix(abs, wsAbs) {
			c.JSON(400, gin.H{"ok": false, "error": "Invalid path"})
			return
		}
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		files := []wsFile{}
		now := time.Now()
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			relPath := subPath
			if relPath != "" {
				relPath = relPath + "/" + e.Name()
			} else {
				relPath = e.Name()
			}
			ext := ""
			if !e.IsDir() {
				ext = strings.ToLower(filepath.Ext(e.Name()))
			}
			ageDays := int(now.Sub(info.ModTime()).Hours() / 24)
			files = append(files, wsFile{
				Name:        e.Name(),
				Path:        relPath,
				Size:        info.Size(),
				SizeHuman:   humanSize(info.Size()),
				IsDirectory: e.IsDir(),
				ModifiedAt:  info.ModTime().Format(time.RFC3339),
				Extension:   ext,
				AgeDays:     ageDays,
			})
		}
		var parentPath *string
		if subPath != "" {
			p := filepath.Dir(subPath)
			if p == "." {
				p = ""
			}
			parentPath = &p
		}
		c.JSON(200, gin.H{"ok": true, "files": files, "currentPath": subPath, "parentPath": parentPath})
	}
}

func WorkspaceStats(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		wc := loadWsConfig(cfg)
		totalFiles := 0
		var totalSize int64
		oldFiles := 0
		now := time.Now()
		filepath.Walk(wsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			totalFiles++
			totalSize += info.Size()
			if wc.AutoCleanDays > 0 && int(now.Sub(info.ModTime()).Hours()/24) > wc.AutoCleanDays {
				oldFiles++
			}
			return nil
		})
		c.JSON(200, gin.H{"ok": true, "totalFiles": totalFiles, "totalSize": totalSize, "totalSizeHuman": humanSize(totalSize), "oldFiles": oldFiles})
	}
}

func WorkspaceConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wc := loadWsConfig(cfg)
		c.JSON(200, gin.H{"ok": true, "config": wc})
	}
}

func WorkspaceUpdateConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var wc wsConfig
		if err := c.ShouldBindJSON(&wc); err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if wc.ExcludePatterns == nil {
			wc.ExcludePatterns = []string{}
		}
		saveWsConfig(cfg, wc)
		c.JSON(200, gin.H{"ok": true, "config": wc})
	}
}

func WorkspaceUpload(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		subPath := c.PostForm("path")
		targetDir := wsDir
		if subPath != "" {
			targetDir = filepath.Join(wsDir, subPath)
		}
		abs, _ := filepath.Abs(targetDir)
		wsAbs, _ := filepath.Abs(wsDir)
		if !strings.HasPrefix(abs, wsAbs) {
			c.JSON(400, gin.H{"ok": false, "error": "Invalid path"})
			return
		}
		os.MkdirAll(targetDir, 0755)
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		files := form.File["files"]
		if len(files) == 0 {
			c.JSON(400, gin.H{"ok": false, "error": "No files provided"})
			return
		}
		uploaded := []string{}
		for _, f := range files {
			dst := filepath.Join(targetDir, f.Filename)
			if err := c.SaveUploadedFile(f, dst); err != nil {
				c.JSON(500, gin.H{"ok": false, "error": err.Error()})
				return
			}
			uploaded = append(uploaded, f.Filename)
		}
		c.JSON(200, gin.H{"ok": true, "files": uploaded})
	}
}

func WorkspaceMkdir(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		var body struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		c.ShouldBindJSON(&body)
		if body.Name == "" {
			c.JSON(400, gin.H{"ok": false, "error": "Directory name required"})
			return
		}
		targetDir := filepath.Join(wsDir, body.Path, body.Name)
		abs, _ := filepath.Abs(targetDir)
		wsAbs, _ := filepath.Abs(wsDir)
		if !strings.HasPrefix(abs, wsAbs) {
			c.JSON(400, gin.H{"ok": false, "error": "Invalid path"})
			return
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			c.JSON(500, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func WorkspaceDelete(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		var body struct {
			Paths []string `json:"paths"`
		}
		c.ShouldBindJSON(&body)
		if len(body.Paths) == 0 {
			c.JSON(400, gin.H{"ok": false, "error": "No paths provided"})
			return
		}
		wsAbs, _ := filepath.Abs(wsDir)
		deleted := 0
		for _, p := range body.Paths {
			full := filepath.Join(wsDir, p)
			abs, _ := filepath.Abs(full)
			if !strings.HasPrefix(abs, wsAbs) {
				continue
			}
			os.RemoveAll(full)
			deleted++
		}
		c.JSON(200, gin.H{"ok": true, "deleted": deleted})
	}
}

func WorkspaceDownload(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		filePath := c.Query("path")
		if filePath == "" {
			c.JSON(400, gin.H{"ok": false, "error": "Path required"})
			return
		}
		full := filepath.Join(wsDir, filePath)
		abs, _ := filepath.Abs(full)
		wsAbs, _ := filepath.Abs(wsDir)
		if !strings.HasPrefix(abs, wsAbs) {
			c.JSON(400, gin.H{"ok": false, "error": "Invalid path"})
			return
		}
		info, err := os.Stat(full)
		if err != nil {
			c.JSON(404, gin.H{"ok": false, "error": "File not found"})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filePath)))
		c.Header("Content-Length", fmt.Sprintf("%d", info.Size()))
		c.File(full)
	}
}

func WorkspaceClean(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		wc := loadWsConfig(cfg)
		if wc.AutoCleanDays <= 0 {
			wc.AutoCleanDays = 30
		}
		now := time.Now()
		deleted := 0
		filepath.Walk(wsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if int(now.Sub(info.ModTime()).Hours()/24) > wc.AutoCleanDays {
				os.Remove(path)
				deleted++
			}
			return nil
		})
		c.JSON(200, gin.H{"ok": true, "deleted": deleted})
	}
}

func WorkspaceNotes(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		notes := map[string]string{}
		data, err := os.ReadFile(getWsNotesPath(cfg))
		if err == nil {
			json.Unmarshal(data, &notes)
		}
		c.JSON(200, gin.H{"ok": true, "notes": notes})
	}
}

func WorkspaceSetNote(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Path string `json:"path"`
			Note string `json:"note"`
		}
		c.ShouldBindJSON(&body)
		if body.Path == "" {
			c.JSON(400, gin.H{"ok": false, "error": "Path required"})
			return
		}
		notes := map[string]string{}
		data, err := os.ReadFile(getWsNotesPath(cfg))
		if err == nil {
			json.Unmarshal(data, &notes)
		}
		if body.Note == "" {
			delete(notes, body.Path)
		} else {
			notes[body.Path] = body.Note
		}
		out, _ := json.MarshalIndent(notes, "", "  ")
		os.WriteFile(getWsNotesPath(cfg), out, 0644)
		c.JSON(200, gin.H{"ok": true})
	}
}

func WorkspacePreview(cfg *config.Config) gin.HandlerFunc {
	imgExts := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
		".svg": "image/svg+xml", ".ico": "image/x-icon",
	}
	txtExts := map[string]bool{
		".txt": true, ".md": true, ".log": true, ".json": true, ".jsonl": true,
		".js": true, ".ts": true, ".py": true, ".sh": true, ".yaml": true,
		".yml": true, ".xml": true, ".html": true, ".css": true, ".csv": true,
		".ini": true, ".conf": true, ".toml": true, ".env": true,
	}
	return func(c *gin.Context) {
		wsDir := getWorkspaceDir(cfg)
		filePath := c.Query("path")
		if filePath == "" {
			c.JSON(400, gin.H{"ok": false, "error": "Path required"})
			return
		}
		full := filepath.Join(wsDir, filePath)
		abs, _ := filepath.Abs(full)
		wsAbs, _ := filepath.Abs(wsDir)
		if !strings.HasPrefix(abs, wsAbs) {
			c.JSON(400, gin.H{"ok": false, "error": "Invalid path"})
			return
		}
		info, err := os.Stat(full)
		if err != nil {
			c.JSON(404, gin.H{"ok": false, "error": "File not found"})
			return
		}
		ext := strings.ToLower(filepath.Ext(full))
		if mimeType, ok := imgExts[ext]; ok {
			c.Header("Content-Type", mimeType)
			c.Header("Content-Length", fmt.Sprintf("%d", info.Size()))
			c.File(full)
			return
		}
		if txtExts[ext] {
			maxSize := int64(512 * 1024)
			content, err := os.ReadFile(full)
			if err != nil {
				c.JSON(500, gin.H{"ok": false, "error": err.Error()})
				return
			}
			truncated := false
			if info.Size() > maxSize {
				content = append(content[:maxSize], []byte("\n\n... (文件过大，已截断)")...)
				truncated = true
			}
			c.JSON(200, gin.H{"ok": true, "type": "text", "content": string(content), "truncated": truncated})
			return
		}
		c.JSON(400, gin.H{"ok": false, "error": "不支持预览此文件类型", "ext": ext})
	}
}

func GetWorkspacePath(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "path": cfg.GetOpenClawWork()})
	}
}

func SetWorkspacePath(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		cfg.SetOpenClawWork(body.Path)
		c.JSON(200, gin.H{"ok": true})
	}
}

// Unused imports suppressor
var _ = io.EOF
var _ = sort.Strings
