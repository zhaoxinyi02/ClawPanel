package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestGetSkillsUsesWorkspacePrecedenceAndSkillEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	openClawDir := filepath.Join(root, "openclaw")
	openClawApp := filepath.Join(root, "app")
	workspace := filepath.Join(root, "workspaces", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawApp: openClawApp, OpenClawWork: workspace}

	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"ghost-plugin":   map[string]interface{}{"enabled": false},
				"disabled-tools": map[string]interface{}{"enabled": false},
			},
			"installs": map[string]interface{}{
				"ghost-plugin": map[string]interface{}{"version": "0.1.0"},
			},
		},
		"skills": map[string]interface{}{
			"load": map[string]interface{}{
				"extraDirs": []interface{}{filepath.Join(root, "extra-skills")},
			},
			"entries": map[string]interface{}{
				"workspace-custom": map[string]interface{}{"enabled": false},
			},
			"blocklist": []interface{}{"legacy-blocked"},
		},
	})

	writeSkillFixture(t, filepath.Join(root, "extra-skills", "shared-skill"), "Shared From Extra", "extra copy", "")
	writeSkillFixture(t, filepath.Join(root, "app", "skills", "shared-skill"), "Shared From Bundled", "bundled copy", "")
	writeSkillFixture(t, filepath.Join(openClawDir, "skills", "legacy-blocked"), "Legacy Blocked", "managed blocked skill", "")
	writeSkillFixture(t, filepath.Join(home, ".agents", "skills", "shared-skill"), "Shared From Home", "personal agents copy", "")
	writeSkillFixture(t, filepath.Join(workspace, ".agents", "skills", "shared-skill"), "Shared From Project", "project agents copy", "")
	writeSkillFixture(t, filepath.Join(workspace, "skills", "shared-skill"), "Shared From Workspace", "workspace wins", "")
	writeSkillFixture(t, filepath.Join(workspace, "skills", "custom-skill"), "Workspace Custom", "workspace custom skill", `
metadata:
  openclaw:
    skillKey: workspace-custom
    requires:
      env:
        - OPENAI_API_KEY
`)
	writeJSON(t, filepath.Join(workspace, "skills", "custom-skill", "package.json"), map[string]interface{}{
		"name":    "workspace-custom",
		"version": "2.0.0",
	})
	writeSkillFixture(t, filepath.Join(workspace, "skills", "alias-skill-dir"), "Alias Skill", "alias metadata skill", `
metadata:
  clawdis:
    skillKey: alias-skill
    requires:
      env:
        - ALIAS_KEY
`)
	writeSkillFixture(t, filepath.Join(openClawApp, "extensions", "feishu-tools", "skills", "feishu-card"), "Feishu Card", "plugin skill", "")
	writeSkillFixture(t, filepath.Join(openClawApp, "extensions", "feishu-tools", "skills", "shared-skill"), "Plugin Shared", "plugin should not override workspace", "")
	writeJSON(t, filepath.Join(openClawApp, "extensions", "feishu-tools", "openclaw.plugin.json"), map[string]interface{}{
		"name":        "Feishu Tools",
		"description": "Plugin-provided skills",
		"skills":      []interface{}{"skills"},
	})
	writeSkillFixture(t, filepath.Join(openClawApp, "extensions", "disabled-tools", "skills", "disabled-skill"), "Disabled Plugin Skill", "should stay hidden", "")
	writeJSON(t, filepath.Join(openClawApp, "extensions", "disabled-tools", "openclaw.plugin.json"), map[string]interface{}{
		"name":        "Disabled Tools",
		"description": "Disabled plugin-provided skills",
		"skills":      []interface{}{"skills"},
	})
	writeSkillFixture(t, filepath.Join(openClawApp, "extensions", "escaped-skill"), "Escaped Skill", "should not be scanned", "")
	writeJSON(t, filepath.Join(openClawApp, "extensions", "bad-plugin", "openclaw.plugin.json"), map[string]interface{}{
		"name":        "Bad Plugin",
		"description": "Attempts to escape plugin root",
		"skills":      []interface{}{"../escaped-skill"},
	})

	r := gin.New()
	r.GET("/system/skills", GetSkills(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/skills?agentId=main", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK      bool         `json:"ok"`
		Skills  []skillInfo  `json:"skills"`
		Plugins []pluginInfo `json:"plugins"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok response")
	}

	shared := findSkillByID(resp.Skills, "shared-skill")
	if shared == nil {
		t.Fatalf("expected shared-skill in response")
	}
	if shared.Source != "workspace" {
		t.Fatalf("expected workspace source to win, got %q", shared.Source)
	}
	if shared.Description != "workspace wins" {
		t.Fatalf("expected workspace description to win, got %q", shared.Description)
	}

	custom := findSkillByID(resp.Skills, "custom-skill")
	if custom == nil {
		t.Fatalf("expected custom-skill in response")
	}
	if custom.SkillKey != "workspace-custom" {
		t.Fatalf("expected workspace-custom skillKey, got %q", custom.SkillKey)
	}
	if custom.Version != "2.0.0" {
		t.Fatalf("expected custom skill version 2.0.0, got %q", custom.Version)
	}
	if custom.Enabled {
		t.Fatalf("expected custom skill disabled by skills.entries override")
	}
	if env := asStringSlice(custom.Requires["env"]); len(env) != 1 || env[0] != "OPENAI_API_KEY" {
		t.Fatalf("expected OPENAI_API_KEY requirement, got %#v", custom.Requires)
	}
	aliasSkill := findSkillByID(resp.Skills, "alias-skill-dir")
	if aliasSkill == nil {
		t.Fatalf("expected alias-skill-dir in response")
	}
	if aliasSkill.SkillKey != "alias-skill" {
		t.Fatalf("expected clawdis alias skillKey alias-skill, got %q", aliasSkill.SkillKey)
	}
	if env := asStringSlice(aliasSkill.Requires["env"]); len(env) != 1 || env[0] != "ALIAS_KEY" {
		t.Fatalf("expected ALIAS_KEY requirement from clawdis metadata, got %#v", aliasSkill.Requires)
	}

	blocked := findSkillByID(resp.Skills, "legacy-blocked")
	if blocked == nil {
		t.Fatalf("expected legacy-blocked skill in response")
	}
	if blocked.Enabled {
		t.Fatalf("expected legacy-blocked disabled by blocklist fallback")
	}
	if findSkillByID(resp.Skills, "feishu-card") == nil {
		t.Fatalf("expected plugin-provided skill to be discovered from extensions/")
	}
	if findSkillByID(resp.Skills, "escaped-skill") != nil {
		t.Fatalf("expected plugin skill path escape to be ignored")
	}
	if findSkillByID(resp.Skills, "disabled-skill") != nil {
		t.Fatalf("expected disabled plugin skill to stay hidden")
	}
	if !hasPluginID(resp.Plugins, "feishu-tools") {
		t.Fatalf("expected plugin discovery from extensions/, got %#v", resp.Plugins)
	}
	if !hasPlugin(resp.Plugins, "ghost-plugin", "config", "0.1.0") {
		t.Fatalf("expected plugins.entries fallback item, got %#v", resp.Plugins)
	}
}

func TestToggleSkillWritesEntriesAndRemovesLegacyBlocklist(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"skills": map[string]interface{}{
			"blocklist": []interface{}{"translator-dir"},
		},
	})

	r := gin.New()
	r.PUT("/system/skills/:id/toggle", ToggleSkill(cfg))
	body := bytes.NewReader([]byte(`{"enabled":true,"aliases":["translator-dir"]}`))
	req := httptest.NewRequest(http.MethodPut, "/system/skills/translator/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	skillsCfg := asMapAny(saved["skills"])
	entry := asMapAny(asMapAny(skillsCfg["entries"])["translator"])
	if enabled, _ := entry["enabled"].(bool); !enabled {
		t.Fatalf("expected skills.entries.translator.enabled=true, got %#v", entry)
	}
	if blocklist := asStringSlice(skillsCfg["blocklist"]); len(blocklist) != 0 {
		t.Fatalf("expected legacy blocklist entry removed, got %#v", blocklist)
	}
}

func TestGetSkillsExposesSkillJSONConfigSchema(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	skillDir := filepath.Join(workspace, "skills", "schema-skill")
	writeSkillFixture(t, skillDir, "Schema Skill", "skill with structured config", "")
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"displayName": "Schema Skill",
		"description": "skill with structured config",
		"version": "1.0.0",
		"config": [
			{"key":"config.baseUrl","label":"Base URL","type":"text","required":true},
			{"key":"config.mode","label":"Mode","type":"select","options":["fast",{"label":"Safe Mode","value":"safe"}]}
		]
	}`), 0644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}

	r := gin.New()
	r.GET("/system/skills", GetSkills(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/skills?agentId=main", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK     bool        `json:"ok"`
		Skills []skillInfo `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	skill := findSkillByID(resp.Skills, "schema-skill")
	if skill == nil {
		t.Fatalf("expected schema-skill in response")
	}
	if len(skill.ConfigSchema) != 2 {
		t.Fatalf("expected 2 config fields, got %#v", skill.ConfigSchema)
	}
	configKeys := asStringSlice(skill.Requires["config"])
	if len(configKeys) != 2 || configKeys[0] != "config.baseUrl" || configKeys[1] != "config.mode" {
		t.Fatalf("expected skill.json config keys merged into requires.config, got %#v", skill.Requires)
	}
	if skill.ConfigSchema[1].Type != "select" || len(skill.ConfigSchema[1].Options) != 2 {
		t.Fatalf("expected select config options from skill.json, got %#v", skill.ConfigSchema[1])
	}
}

func TestSkillConfigHandlersSupportSkillJSONDeclaredKeys(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	skillDir := filepath.Join(workspace, "skills", "configurable-skill")
	writeSkillFixture(t, skillDir, "Configurable Skill", "skill config via manifest", "")
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"config": [
			{"key":"config.apiKey","label":"API Key","type":"password","required":true},
			{"key":"config.maxResults","label":"Max Results","type":"number"},
			{"key":"config.enabled","label":"Enabled","type":"toggle"}
		]
	}`), 0644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}

	r := gin.New()
	r.GET("/system/skills/:id/config", GetSkillConfig(cfg))
	r.PUT("/system/skills/:id/config", UpdateSkillConfig(cfg))

	getReq := httptest.NewRequest(http.MethodGet, "/system/skills/configurable-skill/config?agentId=main", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 for initial get, got %d: %s", getW.Code, getW.Body.String())
	}
	var initial struct {
		ConfigKeys []string               `json:"configKeys"`
		Values     map[string]interface{} `json:"values"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial config response: %v", err)
	}
	if len(initial.ConfigKeys) != 3 || len(initial.Values) != 0 {
		t.Fatalf("expected manifest config keys and empty values, got %#v", initial)
	}

	body := bytes.NewReader([]byte(`{"agentId":"main","values":{"config.apiKey":"secret","config.maxResults":5,"config.enabled":true}}`))
	putReq := httptest.NewRequest(http.MethodPut, "/system/skills/configurable-skill/config", body)
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d: %s", putW.Code, putW.Body.String())
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	configMap := asMapAny(saved["config"])
	if got := trimmedString(configMap["apiKey"], ""); got != "secret" {
		t.Fatalf("expected config.apiKey saved, got %#v", saved)
	}
	if got, ok := configMap["maxResults"].(float64); !ok || got != 5 {
		t.Fatalf("expected config.maxResults saved, got %#v", saved)
	}
	if got, ok := configMap["enabled"].(bool); !ok || !got {
		t.Fatalf("expected config.enabled saved, got %#v", saved)
	}
}

func TestCopySkillCopiesInstalledSkillToAnotherAgent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	mainWorkspace := filepath.Join(root, "workspace", "main")
	otherWorkspace := filepath.Join(root, "workspace", "secondary")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: mainWorkspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": mainWorkspace},
				map[string]interface{}{"id": "secondary", "workspace": otherWorkspace},
			},
		},
	})

	skillDir := filepath.Join(mainWorkspace, "skills", "copy-me")
	writeSkillFixture(t, skillDir, "Copy Me", "copyable skill", "")
	if err := os.WriteFile(filepath.Join(skillDir, "extra.txt"), []byte("hello copy\n"), 0644); err != nil {
		t.Fatalf("write extra file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "nested"), 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "nested", "notes.md"), []byte("nested\n"), 0644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	r := gin.New()
	r.POST("/system/skills/:id/copy", CopySkill(cfg))
	r.GET("/system/skills", GetSkills(cfg))

	body := bytes.NewReader([]byte(`{"sourceAgentId":"main","targetAgentId":"secondary","installTarget":"agent"}`))
	req := httptest.NewRequest(http.MethodPost, "/system/skills/copy-me/copy", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 copy response, got %d: %s", w.Code, w.Body.String())
	}

	for _, rel := range []string{"SKILL.md", "extra.txt", filepath.Join("nested", "notes.md")} {
		if _, err := os.Stat(filepath.Join(otherWorkspace, "skills", "copy-me", rel)); err != nil {
			t.Fatalf("expected copied file %s, got %v", rel, err)
		}
	}

	skillsReq := httptest.NewRequest(http.MethodGet, "/system/skills?agentId=secondary", nil)
	skillsW := httptest.NewRecorder()
	r.ServeHTTP(skillsW, skillsReq)
	if skillsW.Code != http.StatusOK {
		t.Fatalf("expected 200 skills response, got %d: %s", skillsW.Code, skillsW.Body.String())
	}
	var resp struct {
		Skills []skillInfo `json:"skills"`
	}
	if err := json.Unmarshal(skillsW.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode copied skills response: %v", err)
	}
	if findSkillByID(resp.Skills, "copy-me") == nil {
		t.Fatalf("expected copied skill to appear for target agent")
	}
}

func TestSearchAndInstallClawHubUseOfficialAPIContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/search":
			if got := r.URL.Query().Get("q"); got != "weather" {
				t.Fatalf("expected q=weather, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"version":     "1.2.0",
					"updatedAt":   123,
				}},
			})
		case r.URL.Path == "/api/v1/skills/weather":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skill": map[string]interface{}{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"moderation":  map[string]interface{}{"isMalwareBlocked": false},
				},
				"latestVersion": map[string]interface{}{"version": "1.2.0"},
			})
		case r.URL.Path == "/api/v1/download":
			if got := r.URL.Query().Get("slug"); got != "weather" {
				t.Fatalf("expected download slug weather, got %q", got)
			}
			if got := r.URL.Query().Get("version"); got != "1.2.0" {
				t.Fatalf("expected download version 1.2.0, got %q", got)
			}
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(buildZipFixture(t, map[string]string{
				"weather-1.2.0/SKILL.md":     "---\nname: Weather\ndescription: Public weather skill\n---\nUse weather data.\n",
				"weather-1.2.0/package.json": `{\"name\":\"weather\",\"description\":\"Public weather skill\"}`,
			}))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	registryURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	registryURL.User = url.UserPassword("alice", "secret")
	t.Setenv("CLAWHUB_SITE", registryURL.String())

	r := gin.New()
	r.GET("/system/skills", GetSkills(cfg))
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))

	searchReq := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	searchW := httptest.NewRecorder()
	r.ServeHTTP(searchW, searchReq)
	if searchW.Code != http.StatusOK {
		t.Fatalf("search expected 200, got %d: %s", searchW.Code, searchW.Body.String())
	}
	var searchResp struct {
		OK           bool               `json:"ok"`
		RegistryBase string             `json:"registryBase"`
		Skills       []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(searchW.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(searchResp.Skills) != 1 || searchResp.Skills[0].Installed {
		t.Fatalf("expected one not-installed search result, got %#v", searchResp.Skills)
	}
	if got := strings.TrimSpace(searchResp.RegistryBase); got != server.URL {
		t.Fatalf("expected public registry base %q, got %q", server.URL, got)
	}

	installReq := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"weather","agentId":"main"}`)))
	installReq.Header.Set("Content-Type", "application/json")
	installW := httptest.NewRecorder()
	r.ServeHTTP(installW, installReq)
	if installW.Code != http.StatusOK {
		t.Fatalf("install expected 200, got %d: %s", installW.Code, installW.Body.String())
	}

	lockRaw, err := os.ReadFile(filepath.Join(workspace, ".clawhub", "lock.json"))
	if err != nil {
		t.Fatalf("read lock.json: %v", err)
	}
	var lockPayload map[string]interface{}
	if err := json.Unmarshal(lockRaw, &lockPayload); err != nil {
		t.Fatalf("decode lock.json: %v", err)
	}
	if got := int(lockPayload["version"].(float64)); got != 1 {
		t.Fatalf("expected lock version 1, got %d", got)
	}
	skillsMap := asMapAny(lockPayload["skills"])
	weatherLock := asMapAny(skillsMap["weather"])
	if got := strings.TrimSpace(getString(weatherLock, "version")); got != "1.2.0" {
		t.Fatalf("expected lock version 1.2.0, got %q", got)
	}
	if _, ok := weatherLock["installedAt"].(float64); !ok {
		t.Fatalf("expected numeric installedAt, got %#v", weatherLock["installedAt"])
	}
	if _, err := os.Stat(filepath.Join(workspace, "skills", "weather", "SKILL.md")); err != nil {
		t.Fatalf("expected extracted skill files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "skills", "weather", "weather-1.2.0", "SKILL.md")); err == nil {
		t.Fatalf("expected wrapper directory to be flattened during extraction")
	}
	originRaw, err := os.ReadFile(filepath.Join(workspace, "skills", "weather", ".clawhub", "origin.json"))
	if err != nil {
		t.Fatalf("read origin.json: %v", err)
	}
	var originPayload map[string]interface{}
	if err := json.Unmarshal(originRaw, &originPayload); err != nil {
		t.Fatalf("decode origin.json: %v", err)
	}
	if got := strings.TrimSpace(getString(originPayload, "slug")); got != "weather" {
		t.Fatalf("expected origin slug weather, got %q", got)
	}
	if got := strings.TrimSpace(getString(originPayload, "registry")); got != server.URL {
		t.Fatalf("expected origin registry %q, got %q", server.URL, got)
	}

	searchAfterReq := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	searchAfterW := httptest.NewRecorder()
	r.ServeHTTP(searchAfterW, searchAfterReq)
	if searchAfterW.Code != http.StatusOK {
		t.Fatalf("search after install expected 200, got %d: %s", searchAfterW.Code, searchAfterW.Body.String())
	}
	var searchAfterResp struct {
		OK           bool               `json:"ok"`
		RegistryBase string             `json:"registryBase"`
		Skills       []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(searchAfterW.Body.Bytes(), &searchAfterResp); err != nil {
		t.Fatalf("decode search-after response: %v", err)
	}
	if len(searchAfterResp.Skills) != 1 || !searchAfterResp.Skills[0].Installed || searchAfterResp.Skills[0].InstalledVersion != "1.2.0" {
		t.Fatalf("expected installed search result after install, got %#v", searchAfterResp.Skills)
	}

	skillsReq := httptest.NewRequest(http.MethodGet, "/system/skills?agentId=main", nil)
	skillsW := httptest.NewRecorder()
	r.ServeHTTP(skillsW, skillsReq)
	if skillsW.Code != http.StatusOK {
		t.Fatalf("skills expected 200, got %d: %s", skillsW.Code, skillsW.Body.String())
	}
	var skillsResp struct {
		Skills []skillInfo `json:"skills"`
	}
	if err := json.Unmarshal(skillsW.Body.Bytes(), &skillsResp); err != nil {
		t.Fatalf("decode skills response: %v", err)
	}
	localWeather := findSkillByID(skillsResp.Skills, "weather")
	if localWeather == nil || localWeather.Version != "1.2.0" {
		t.Fatalf("expected local installed skill version 1.2.0, got %#v", localWeather)
	}
}

func TestSearchAndInstallClawHubSupportCustomRegistryFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/raw/main/skills/registry.json":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skills": []map[string]interface{}{{
					"id":          "web-fetcher",
					"name":        "Web Fetcher",
					"description": "custom registry skill",
					"version":     "1.0.0",
					"path":        "skills/web-fetcher",
					"files":       []string{"SKILL.md", "skill.json", "index.py"},
				}},
			})
		case "/raw/main/skills/web-fetcher/SKILL.md":
			_, _ = w.Write([]byte("---\nname: Web Fetcher\ndescription: custom registry skill\n---\nUse the web fetcher.\n"))
		case "/raw/main/skills/web-fetcher/skill.json":
			_, _ = w.Write([]byte(`{"displayName":"Web Fetcher","config":[{"key":"config.baseUrl","label":"Base URL","type":"text"}]}`))
		case "/raw/main/skills/web-fetcher/index.py":
			_, _ = w.Write([]byte("print('ok')\n"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_REGISTRY", server.URL+"/raw/main")
	t.Setenv("CLAWHUB_SITE", server.URL+"/raw/main")

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))

	searchReq := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=web-fetcher&agentId=main", nil)
	searchW := httptest.NewRecorder()
	r.ServeHTTP(searchW, searchReq)
	if searchW.Code != http.StatusOK {
		t.Fatalf("expected 200 search, got %d: %s", searchW.Code, searchW.Body.String())
	}

	installReq := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"web-fetcher","agentId":"main"}`)))
	installReq.Header.Set("Content-Type", "application/json")
	installW := httptest.NewRecorder()
	r.ServeHTTP(installW, installReq)
	if installW.Code != http.StatusOK {
		t.Fatalf("expected 200 install, got %d: %s", installW.Code, installW.Body.String())
	}
	for _, rel := range []string{"SKILL.md", "skill.json", "index.py", ".clawhub/origin.json"} {
		if _, err := os.Stat(filepath.Join(workspace, "skills", "web-fetcher", filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s downloaded, got %v", rel, err)
		}
	}
	lockRaw, err := os.ReadFile(filepath.Join(workspace, ".clawhub", "lock.json"))
	if err != nil || !strings.Contains(string(lockRaw), "web-fetcher") {
		t.Fatalf("expected lock entry for custom registry install, err=%v body=%s", err, string(lockRaw))
	}
}

func TestSearchClawHubMarksExistingSkillDirectoryInstalled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	writeSkillFixture(t, filepath.Join(workspace, "skills", "weather"), "Weather", "manual install", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Skills []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Skills) != 1 || !resp.Skills[0].Installed {
		t.Fatalf("expected existing skill dir to be marked installed, got %#v", resp.Skills)
	}
}

func TestSearchClawHubIgnoresNonDiscoverableSkillDir(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	if err := os.MkdirAll(filepath.Join(workspace, "skills", "weather"), 0o755); err != nil {
		t.Fatalf("create invalid skill dir: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Skills []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Skills) != 1 || resp.Skills[0].Installed {
		t.Fatalf("expected invalid skill dir to stay not installed, got %#v", resp.Skills)
	}
}

func TestInstallClawHubSkillGlobalTargetUsesManagedRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/weather":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skill": map[string]interface{}{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"moderation":  map[string]interface{}{"isMalwareBlocked": false},
				},
				"latestVersion": map[string]interface{}{"version": "1.2.0"},
			})
		case "/api/v1/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(buildZipFixture(t, map[string]string{
				"weather-1.2.0/SKILL.md":     "---\nname: Weather\ndescription: Public weather skill\n---\nUse weather data.\n",
				"weather-1.2.0/package.json": `{"name":"weather","description":"Public weather skill"}`,
			}))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"weather","agentId":"main","installTarget":"global"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(openClawDir, "skills", "weather", "SKILL.md")); err != nil {
		t.Fatalf("expected managed skill install, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "skills", "weather")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace skills dir untouched, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(openClawDir, ".clawhub", "lock.json")); err != nil {
		t.Fatalf("expected managed lock file, got %v", err)
	}
	if !strings.Contains(w.Body.String(), `"installTarget":"global"`) {
		t.Fatalf("expected global install target in response, got %s", w.Body.String())
	}
}

func TestSearchClawHubUsesSiteForPublicLinksWhenRegistryOverrideSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_REGISTRY", server.URL)
	t.Setenv("CLAWHUB_SITE", "https://clawhub.example.com")

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		RegistryBase string             `json:"registryBase"`
		Skills       []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := strings.TrimSpace(resp.RegistryBase); got != "https://clawhub.example.com" {
		t.Fatalf("expected public site base, got %q", got)
	}
	if len(resp.Skills) != 1 {
		t.Fatalf("expected one search result, got %#v", resp.Skills)
	}
}

func TestSearchClawHubRejectsInvalidRegistryScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	t.Setenv("CLAWHUB_SITE", "javascript:alert(1)")

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for invalid registry scheme, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearchClawHubDoesNotExposeRegistryCredentialsOnTransportError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	t.Setenv("CLAWHUB_SITE", "https://alice:secret@example.com")
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		}),
	}
	defer func() { clawHubHTTPClient = oldClient }()

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for transport failure, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "alice") || strings.Contains(body, "secret") {
		t.Fatalf("expected response body to redact registry credentials, got %s", body)
	}
	if !strings.Contains(body, "request failed") {
		t.Fatalf("expected sanitized transport error message, got %s", body)
	}
}

func TestSearchClawHubDoesNotMarkManagedSkillInstalled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	writeSkillFixture(t, filepath.Join(openClawDir, "skills", "weather"), "Weather", "managed install", "")
	writeJSON(t, filepath.Join(openClawDir, "skills", "weather", "package.json"), map[string]interface{}{
		"name":    "weather",
		"version": "9.9.9",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Skills []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Skills) != 1 || resp.Skills[0].Installed || resp.Skills[0].InstalledVersion != "" {
		t.Fatalf("expected managed skill to remain installable in workspace, got %#v", resp.Skills)
	}
}

func TestSearchClawHubIgnoresStaleLockWithoutSkillDirectory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	writeJSON(t, filepath.Join(workspace, ".clawhub", "lock.json"), map[string]interface{}{
		"version": 1,
		"skills": map[string]interface{}{
			"weather": map[string]interface{}{"version": "0.0.1", "installedAt": 1},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Skills []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Skills) != 1 || resp.Skills[0].Installed || resp.Skills[0].InstalledVersion != "" {
		t.Fatalf("expected stale lock without skill dir to be ignored, got %#v", resp.Skills)
	}
}

func TestSearchClawHubIgnoresManagedSkillWhenOnlyStaleLockExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	writeSkillFixture(t, filepath.Join(openClawDir, "skills", "weather"), "Weather", "managed install", "")
	writeJSON(t, filepath.Join(openClawDir, "skills", "weather", "package.json"), map[string]interface{}{
		"name":    "weather",
		"version": "9.9.9",
	})
	writeJSON(t, filepath.Join(workspace, ".clawhub", "lock.json"), map[string]interface{}{
		"version": 1,
		"skills": map[string]interface{}{
			"weather": map[string]interface{}{"version": "0.0.1", "installedAt": 1},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Skills []clawHubSkillItem `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Skills) != 1 || resp.Skills[0].Installed || resp.Skills[0].InstalledVersion != "" {
		t.Fatalf("expected stale workspace lock plus managed skill to remain not installed, got %#v", resp.Skills)
	}
}

func TestSkillsHandlersRejectInvalidAgentID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := resolvedTempDir(t)
	cfg := &config.Config{OpenClawDir: filepath.Join(dir, "openclaw"), OpenClawWork: filepath.Join(dir, "workspace", "main")}
	writeJSON(t, filepath.Join(cfg.OpenClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": filepath.Join(dir, "workspace", "main")},
			},
		},
	})

	r := gin.New()
	r.GET("/system/skills", GetSkills(cfg))
	r.GET("/system/clawhub/search", SearchClawHub(cfg))

	for _, path := range []string{
		"/system/skills?agentId=../../etc",
		"/system/clawhub/search?agentId=../../etc",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d: %s", path, w.Code, w.Body.String())
		}
	}
}

func TestInstallClawHubSkillRejectsInvalidSlug(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := resolvedTempDir(t)
	cfg := &config.Config{OpenClawDir: filepath.Join(dir, "openclaw"), OpenClawWork: filepath.Join(dir, "workspace", "main")}
	writeJSON(t, filepath.Join(cfg.OpenClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": filepath.Join(dir, "workspace", "main")},
			},
		},
	})

	r := gin.New()
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))
	req := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":".","agentId":"main"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid skill slug, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInstallClawHubSkillAllowsWorkspaceOverrideOfManagedSkill(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	writeSkillFixture(t, filepath.Join(openClawDir, "skills", "weather"), "Weather", "managed install", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/weather":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skill": map[string]interface{}{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"moderation":  map[string]interface{}{"isMalwareBlocked": false},
				},
				"latestVersion": map[string]interface{}{"version": "1.2.0"},
			})
		case "/api/v1/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(buildZipFixture(t, map[string]string{
				"SKILL.md":     "---\nname: Weather\ndescription: Public weather skill\n---\nUse weather data.\n",
				"package.json": `{"name":"weather","description":"Public weather skill"}`,
			}))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))
	req := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"weather","agentId":"main"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for workspace override of managed skill, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, "skills", "weather", "SKILL.md")); err != nil {
		t.Fatalf("expected workspace-local skill install, got %v", err)
	}
}

func TestInstallClawHubSkillRejectsSymlinkedSkillsRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink handling differs on Windows")
	}
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	outside := filepath.Join(root, "outside")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "skills")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/weather":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skill": map[string]interface{}{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"moderation":  map[string]interface{}{"isMalwareBlocked": false},
				},
				"latestVersion": map[string]interface{}{"version": "1.2.0"},
			})
		case "/api/v1/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(buildZipFixture(t, map[string]string{
				"SKILL.md":     "---\nname: Weather\ndescription: Public weather skill\n---\nUse weather data.\n",
				"package.json": `{"name":"weather","description":"Public weather skill"}`,
			}))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))
	req := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"weather","agentId":"main"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for symlinked skills root, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "weather", "SKILL.md")); err == nil {
		t.Fatalf("expected install to avoid writing through symlinked skills root")
	}
}

func TestSearchClawHubRejectsSymlinkedSkillsRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink handling differs on Windows")
	}
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	outside := filepath.Join(root, "outside")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	if err := os.MkdirAll(filepath.Join(outside, "weather"), 0755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeSkillFixture(t, filepath.Join(outside, "weather"), "Weather", "outside skill", "")
	if err := os.Symlink(outside, filepath.Join(workspace, "skills")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for symlinked skills root, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInstallClawHubSkillRejectsSymlinkedClawHubStateDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink handling differs on Windows")
	}
	gin.SetMode(gin.TestMode)

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	outside := filepath.Join(root, "outside-state")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	if err := os.MkdirAll(filepath.Join(workspace, "skills"), 0755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("mkdir outside state: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, ".clawhub")); err != nil {
		t.Fatalf("create .clawhub symlink: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/weather":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skill": map[string]interface{}{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"moderation":  map[string]interface{}{"isMalwareBlocked": false},
				},
				"latestVersion": map[string]interface{}{"version": "1.2.0"},
			})
		case "/api/v1/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(buildZipFixture(t, map[string]string{
				"SKILL.md":     "---\nname: Weather\ndescription: Public weather skill\n---\nUse weather data.\n",
				"package.json": `{"name":"weather","description":"Public weather skill"}`,
			}))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))
	req := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"weather","agentId":"main"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for symlinked .clawhub dir, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "lock.json")); err == nil {
		t.Fatalf("expected install to avoid writing through symlinked .clawhub dir")
	}
}

func TestClawHubHandlersRejectSymlinkedWorkspaceAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink handling differs on Windows")
	}
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	openClawDir := filepath.Join(root, "openclaw")
	managedRoot := filepath.Join(root, "managed", "workspaces")
	realRoot := filepath.Join(root, "real-workspaces")
	workspace := filepath.Join(managedRoot, "shared", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})
	if err := os.MkdirAll(filepath.Join(realRoot, "shared"), 0755); err != nil {
		t.Fatalf("mkdir real root: %v", err)
	}
	if err := os.MkdirAll(managedRoot, 0755); err != nil {
		t.Fatalf("mkdir managed root: %v", err)
	}
	if err := os.Symlink(filepath.Join(realRoot, "shared"), filepath.Join(managedRoot, "shared")); err != nil {
		t.Fatalf("create ancestor symlink: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/weather":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skill": map[string]interface{}{
					"slug":        "weather",
					"displayName": "Weather",
					"summary":     "Public weather skill",
					"moderation":  map[string]interface{}{"isMalwareBlocked": false},
				},
				"latestVersion": map[string]interface{}{"version": "1.2.0"},
			})
		case "/api/v1/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(buildZipFixture(t, map[string]string{
				"SKILL.md":     "---\nname: Weather\ndescription: Public weather skill\n---\nUse weather data.\n",
				"package.json": `{"name":"weather","description":"Public weather skill"}`,
			}))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.POST("/system/clawhub/install", InstallClawHubSkill(cfg))

	req := httptest.NewRequest(http.MethodPost, "/system/clawhub/install", bytes.NewReader([]byte(`{"skillId":"weather","agentId":"main"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for workspace ancestor symlink, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(realRoot, "shared", "main", "skills", "weather", "SKILL.md")); err == nil {
		t.Fatalf("expected install to avoid writing through symlinked workspace ancestor")
	}
}

func TestSearchClawHubAllowsTrustedTempDirAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil || resolvedRoot == "" || resolvedRoot == root {
		t.Skip("temp dir does not use a trusted system alias on this platform")
	}

	openClawDir := filepath.Join(root, "openclaw")
	workspace := filepath.Join(root, "workspace", "main")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": workspace},
			},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"slug":        "weather",
				"displayName": "Weather",
				"summary":     "Public weather skill",
				"version":     "1.2.0",
			}},
		})
	}))
	defer server.Close()
	oldClient := clawHubHTTPClient
	clawHubHTTPClient = server.Client()
	defer func() { clawHubHTTPClient = oldClient }()
	t.Setenv("CLAWHUB_SITE", server.URL)

	r := gin.New()
	r.GET("/system/clawhub/search", SearchClawHub(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/clawhub/search?q=weather&agentId=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for trusted temp alias workspace, got %d: %s", w.Code, w.Body.String())
	}
}

func writeSkillFixture(t *testing.T, dir, name, description, extraFrontmatter string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	frontmatter := strings.TrimSpace(extraFrontmatter)
	if frontmatter != "" {
		frontmatter = "\n" + frontmatter
	}
	content := "---\nname: " + name + "\ndescription: " + description + frontmatter + "\n---\n" + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func buildZipFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func findSkillByID(skills []skillInfo, id string) *skillInfo {
	for i := range skills {
		if skills[i].ID == id {
			return &skills[i]
		}
	}
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func hasPluginID(plugins []pluginInfo, id string) bool {
	for _, plugin := range plugins {
		if plugin.ID == id {
			return true
		}
	}
	return false
}

func hasPlugin(plugins []pluginInfo, id, source, version string) bool {
	for _, plugin := range plugins {
		if plugin.ID == id && plugin.Source == source && plugin.Version == version {
			return true
		}
	}
	return false
}

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if real, err := filepath.EvalSymlinks(dir); err == nil && real != "" {
		return real
	}
	return dir
}

// ---------------------------------------------------------------------------
// Gap-fix tests added for multi-dev alignment
// ---------------------------------------------------------------------------

// TestScanSkillRootSymlinkEscapeGuard verifies that a symlink inside a skill
// root that points to a directory outside the root is NOT followed by the
// recursive scanner.
func TestScanSkillRootSymlinkEscapeGuard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}
	t.Parallel()

	root := resolvedTempDir(t)
	skillsRoot := filepath.Join(root, "skills")
	outside := filepath.Join(root, "secret")

	// Create a legitimate skill inside the root.
	writeSkillFixture(t, filepath.Join(skillsRoot, "real-skill"), "Real Skill", "inside root", "")

	// Create a directory outside the root that has a skill.
	writeSkillFixture(t, filepath.Join(outside, "escape-skill"), "Escape Skill", "should not appear", "")

	// Create a symlink inside the root that points to the outside directory.
	symTarget := filepath.Join(outside, "escape-skill")
	symLink := filepath.Join(skillsRoot, "escape-via-symlink")
	if err := os.Symlink(symTarget, symLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	skills := make([]skillInfo, 0)
	positions := map[string]int{}
	scanSkillRoot(skillsRoot, "test", &skills, positions, nil, nil)

	for _, s := range skills {
		if s.ID == "escape-skill" || strings.Contains(s.Path, "secret") {
			t.Fatalf("symlink escape: scanner returned skill outside root: %+v", s)
		}
	}
	// The real skill should still be found.
	found := false
	for _, s := range skills {
		if s.ID == "real-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected real-skill to be found, got %v", skills)
	}
}

// TestResolveSkillDiscoveryRootsAllowBundledFalse verifies that when
// skills.allowBundled is explicitly false, the bundled ("app-skill") root is
// omitted from the discovery list.
func TestResolveSkillDiscoveryRootsAllowBundledFalse(t *testing.T) {
	t.Parallel()

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	appDir := filepath.Join(root, "app")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawApp: appDir}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
			},
		},
		"skills": map[string]interface{}{
			"allowBundled": false,
		},
	})
	ocConfig, _ := cfg.ReadOpenClawJSON()

	roots := resolveSkillDiscoveryRoots(cfg, ocConfig, nil, "main")
	for _, r := range roots {
		if r.Source == "app-skill" {
			t.Fatalf("expected bundled root to be excluded when allowBundled=false, but got %+v", r)
		}
	}
}

// TestResolveSkillDiscoveryRootsAllowBundledDefault verifies that when
// skills.allowBundled is absent (default), the bundled root IS included.
func TestResolveSkillDiscoveryRootsAllowBundledDefault(t *testing.T) {
	t.Parallel()

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	appDir := filepath.Join(root, "app")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawApp: appDir}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})
	ocConfig, _ := cfg.ReadOpenClawJSON()

	roots := resolveSkillDiscoveryRoots(cfg, ocConfig, nil, "main")
	hasBundled := false
	for _, r := range roots {
		if r.Source == "app-skill" {
			hasBundled = true
			break
		}
	}
	if !hasBundled {
		t.Fatalf("expected bundled root when allowBundled not set, roots=%v", roots)
	}
}

func TestDiscoverSkillsRespectsAllowBundledAllowlist(t *testing.T) {
	t.Parallel()

	root := resolvedTempDir(t)
	openClawDir := filepath.Join(root, "openclaw")
	appDir := filepath.Join(root, "app")
	workspace := filepath.Join(root, "workspace")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawApp: appDir, OpenClawWork: workspace}
	writeJSON(t, filepath.Join(openClawDir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main", "workspace": workspace}},
		},
		"skills": map[string]interface{}{
			"allowBundled": []interface{}{"allowed-bundled"},
		},
	})

	writeSkillFixture(t, filepath.Join(appDir, "skills", "allowed-bundled"), "Allowed Bundled", "bundled and allowed", "")
	writeSkillFixture(t, filepath.Join(appDir, "skills", "hidden-bundled"), "Hidden Bundled", "bundled and filtered", "")
	writeSkillFixture(t, filepath.Join(openClawDir, "skills", "managed-skill"), "Managed Skill", "managed copy", "")

	ocConfig, _ := cfg.ReadOpenClawJSON()
	skills := discoverSkills(
		resolveSkillDiscoveryRoots(cfg, ocConfig, nil, "main"),
		asMapAny(asMapAny(ocConfig["skills"])["entries"]),
		nil,
		resolveBundledSkillAllowlist(ocConfig),
	)

	if findSkillByID(skills, "allowed-bundled") == nil {
		t.Fatalf("expected allowlisted bundled skill to stay visible")
	}
	if findSkillByID(skills, "hidden-bundled") != nil {
		t.Fatalf("expected bundled skill outside allowBundled allowlist to be filtered out")
	}
	if findSkillByID(skills, "managed-skill") == nil {
		t.Fatalf("expected non-bundled skills to remain visible")
	}
}

// TestValidateCronJobsSessionTargetsAcceptsOfficialValues verifies that
// "main" and "isolated" are accepted without modification.
func TestValidateCronJobsSessionTargetsAcceptsOfficialValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})

	jobs := []map[string]interface{}{
		{"id": "j1", "sessionTarget": "main"},
		{"id": "j2", "sessionTarget": "isolated"},
		{"id": "j3"}, // no sessionTarget → should be OK (empty allowed)
	}
	if err := validateCronJobsSessionTargets(cfg, jobs); err != nil {
		t.Fatalf("unexpected error for official values: %v", err)
	}
	// Values should be unchanged.
	if jobs[0]["sessionTarget"] != "main" {
		t.Fatalf("expected main unchanged, got %v", jobs[0]["sessionTarget"])
	}
	if jobs[1]["sessionTarget"] != "isolated" {
		t.Fatalf("expected isolated unchanged, got %v", jobs[1]["sessionTarget"])
	}
}

// TestValidateCronJobsSessionTargetsNormalizesLegacyAgentID verifies that a
// known agent ID in sessionTarget is silently normalized to "main" for backward
// compatibility instead of returning an error.
func TestValidateCronJobsSessionTargetsNormalizesLegacyAgentID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "ops",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
				map[string]interface{}{"id": "ops"},
			},
		},
	})

	jobs := []map[string]interface{}{
		{"id": "j1", "sessionTarget": "ops"}, // legacy: agent ID used here
	}
	if err := validateCronJobsSessionTargets(cfg, jobs); err != nil {
		t.Fatalf("unexpected error for legacy agent ID: %v", err)
	}
	// Should be normalised to "main".
	if jobs[0]["sessionTarget"] != "main" {
		t.Fatalf("expected legacy agent ID normalised to \"main\", got %v", jobs[0]["sessionTarget"])
	}
	if jobs[0]["agentId"] != "ops" {
		t.Fatalf("expected legacy agent ID migrated into agentId, got %v", jobs[0]["agentId"])
	}
}

// TestValidateCronJobsSessionTargetsRejectsUnknownValue verifies that an
// unrecognised non-agent value in sessionTarget is rejected.
func TestValidateCronJobsSessionTargetsRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})

	jobs := []map[string]interface{}{
		{"id": "j1", "sessionTarget": "foobar-unknown"},
	}
	if err := validateCronJobsSessionTargets(cfg, jobs); err == nil {
		t.Fatalf("expected error for unknown sessionTarget, got nil")
	}
}

// TestSaveCronJobsNormalizesEmptySessionTargetToMain verifies that when a job
// has no sessionTarget, SaveCronJobs sets it to "main" (official default) rather
// than an agent ID.
func TestSaveCronJobsNormalizesEmptySessionTargetToMain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "ops",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
				map[string]interface{}{"id": "ops"},
			},
		},
	})

	r := gin.New()
	r.PUT("/system/cron", SaveCronJobs(cfg))
	body := bytes.NewReader([]byte(`{"jobs":[{"id":"j1","name":"test"}]}`))
	req := httptest.NewRequest(http.MethodPut, "/system/cron", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	saved, _ := cfg.ReadOpenClawJSON()
	cronCfg, _ := saved["cron"].(map[string]interface{})
	jobs, _ := cronCfg["jobs"].([]interface{})
	if len(jobs) == 0 {
		t.Fatalf("expected jobs saved, got none")
	}
	job0, _ := jobs[0].(map[string]interface{})
	if target, _ := job0["sessionTarget"].(string); target != "main" {
		t.Fatalf("expected sessionTarget=\"main\", got %q", target)
	}
}

func TestValidateCronJobsSessionTargetsRejectsUnknownAgentID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})

	jobs := []map[string]interface{}{
		{"id": "j1", "agentId": "ghost", "sessionTarget": "isolated"},
	}
	if err := validateCronJobsSessionTargets(cfg, jobs); err == nil {
		t.Fatalf("expected error for unknown agentId, got nil")
	}
}

// TestWriteCronJobsFileIsAtomic verifies that writeCronJobsFile does not leave
// a .tmp file behind after a successful write.
func TestWriteCronJobsFileIsAtomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	jobs := []interface{}{
		map[string]interface{}{"id": "j1", "name": "nightly"},
	}
	if err := writeCronJobsFile(cfg, jobs); err != nil {
		t.Fatalf("writeCronJobsFile: %v", err)
	}
	dest := filepath.Join(dir, "cron", "jobs.json")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected jobs.json to exist: %v", err)
	}
	tmp := dest + ".tmp"
	if _, err := os.Stat(tmp); err == nil {
		t.Fatalf("expected .tmp file to be cleaned up, but it still exists")
	}
}

func TestWriteCronJobsFileReplacesExistingDestination(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	first := []interface{}{
		map[string]interface{}{"id": "j1", "name": "nightly"},
	}
	second := []interface{}{
		map[string]interface{}{"id": "j2", "name": "weekly"},
	}

	if err := writeCronJobsFile(cfg, first); err != nil {
		t.Fatalf("first writeCronJobsFile: %v", err)
	}
	if err := writeCronJobsFile(cfg, second); err != nil {
		t.Fatalf("second writeCronJobsFile: %v", err)
	}

	dest := filepath.Join(dir, "cron", "jobs.json")
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read jobs.json: %v", err)
	}

	var payload struct {
		Jobs []map[string]interface{} `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode jobs.json: %v", err)
	}
	if len(payload.Jobs) != 1 {
		t.Fatalf("expected 1 job after replace, got %d", len(payload.Jobs))
	}
	if got := strings.TrimSpace(toString(payload.Jobs[0]["id"])); got != "j2" {
		t.Fatalf("expected overwritten job id j2, got %q", got)
	}
	if _, err := os.Stat(dest + ".bak"); err == nil {
		t.Fatalf("expected .bak file to be cleaned up after successful replace")
	}
}

func TestSaveCronJobsRollsBackOpenClawJSONWhenMirrorWriteFails(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	original := map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
			},
		},
		"cron": map[string]interface{}{
			"jobs": []interface{}{
				map[string]interface{}{"id": "old-job", "sessionTarget": "main"},
			},
		},
	}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), original)
	if err := os.WriteFile(filepath.Join(dir, "cron"), []byte("not-a-dir"), 0644); err != nil {
		t.Fatalf("create blocking cron file: %v", err)
	}

	r := gin.New()
	r.PUT("/cron/jobs", SaveCronJobs(cfg))
	req := httptest.NewRequest(http.MethodPut, "/cron/jobs", bytes.NewReader([]byte(`{"jobs":[{"id":"new-job","sessionTarget":"main"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when cron mirror write fails, got %d: %s", w.Code, w.Body.String())
	}

	raw, err := os.ReadFile(filepath.Join(dir, "openclaw.json"))
	if err != nil {
		t.Fatalf("read openclaw.json: %v", err)
	}
	var restored map[string]interface{}
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("decode restored openclaw.json: %v", err)
	}
	cron, _ := restored["cron"].(map[string]interface{})
	jobs, _ := cron["jobs"].([]interface{})
	if len(jobs) != 1 {
		t.Fatalf("expected original jobs to be restored, got %d", len(jobs))
	}
	job, _ := jobs[0].(map[string]interface{})
	if got := strings.TrimSpace(toString(job["id"])); got != "old-job" {
		t.Fatalf("expected rollback to restore old-job, got %q", got)
	}
}

func TestValidateCronJobsSessionTargetsNormalizesWebhookDeliveryURLToTo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})

	jobs := []map[string]interface{}{
		{
			"id":            "j1",
			"sessionTarget": "isolated",
			"delivery": map[string]interface{}{
				"mode": "webhook",
				"url":  "https://example.com/hook",
			},
		},
	}

	if err := validateCronJobsSessionTargets(cfg, jobs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	delivery, _ := jobs[0]["delivery"].(map[string]interface{})
	if got := strings.TrimSpace(toString(delivery["to"])); got != "https://example.com/hook" {
		t.Fatalf("expected webhook to field normalized from url, got %q", got)
	}
	if _, exists := delivery["url"]; exists {
		t.Fatalf("expected legacy delivery.url removed after normalization")
	}
}

func TestValidateCronJobsSessionTargetsRejectsWebhookMissingTo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})

	jobs := []map[string]interface{}{
		{
			"id":            "j1",
			"sessionTarget": "isolated",
			"delivery": map[string]interface{}{
				"mode": "webhook",
			},
		},
	}

	err := validateCronJobsSessionTargets(cfg, jobs)
	if err == nil {
		t.Fatalf("expected error for webhook delivery without target")
	}
	if !strings.Contains(err.Error(), "delivery.mode=webhook 时必须指定非空 to") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCronJobsNormalizesWebhookDeliveryURLField(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"cron": map[string]interface{}{
			"jobs": []interface{}{
				map[string]interface{}{
					"id": "j1",
					"delivery": map[string]interface{}{
						"mode": "webhook",
						"url":  "https://example.com/from-openclaw-json",
					},
				},
			},
		},
	})

	r := gin.New()
	r.GET("/system/cron", GetCronJobs(cfg))
	req := httptest.NewRequest(http.MethodGet, "/system/cron", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool                     `json:"ok"`
		Jobs []map[string]interface{} `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || len(resp.Jobs) != 1 {
		t.Fatalf("unexpected response payload: %+v", resp)
	}
	delivery, _ := resp.Jobs[0]["delivery"].(map[string]interface{})
	if got := strings.TrimSpace(toString(delivery["to"])); got != "https://example.com/from-openclaw-json" {
		t.Fatalf("expected webhook to normalized from url, got %q", got)
	}
	if _, exists := delivery["url"]; exists {
		t.Fatalf("expected legacy delivery.url removed in get response")
	}
}

func TestValidateCronJobsSessionTargetsNormalizesMainAnnounceToNone(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list":    []interface{}{map[string]interface{}{"id": "main"}},
		},
	})

	jobs := []map[string]interface{}{
		{
			"id":            "j1",
			"sessionTarget": "main",
			"delivery": map[string]interface{}{
				"mode":      "announce",
				"channel":   "feishu",
				"accountId": "default",
				"to":        "oc://channel",
				"failureDestination": map[string]interface{}{
					"mode": "webhook",
					"to":   "https://example.com/failure",
				},
			},
		},
	}

	if err := validateCronJobsSessionTargets(cfg, jobs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	delivery, _ := jobs[0]["delivery"].(map[string]interface{})
	if got := strings.TrimSpace(toString(delivery["mode"])); got != "none" {
		t.Fatalf("expected main announce normalized to none, got %q", got)
	}
	for _, key := range []string{"channel", "accountId", "to", "url", "bestEffort", "failureDestination"} {
		if _, exists := delivery[key]; exists {
			t.Fatalf("expected field %q removed after normalization", key)
		}
	}
}
