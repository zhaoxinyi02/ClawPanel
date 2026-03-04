package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOpenClawJSONSupportsJSON5AndWriteCreatesBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{OpenClawDir: dir}

	json5Raw := `{
  // json5 comment
  tools: {
    agentToAgent: true,
  },
  session: {
    maxMessages: 30,
  },
  agents: {
    default: "main",
    list: [
      { id: "main", },
    ],
  },
}`
	configPath := filepath.Join(dir, "openclaw.json")
	if err := os.WriteFile(configPath, []byte(json5Raw), 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	parsed, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("ReadOpenClawJSON should parse JSON5, got error: %v", err)
	}
	if _, ok := parsed["tools"].(map[string]interface{}); !ok {
		t.Fatalf("tools should exist after JSON5 parse")
	}
	if _, ok := parsed["session"].(map[string]interface{}); !ok {
		t.Fatalf("session should exist after JSON5 parse")
	}

	if err := cfg.WriteOpenClawJSON(parsed); err != nil {
		t.Fatalf("WriteOpenClawJSON failed: %v", err)
	}

	backupDir := filepath.Join(dir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("backup dir should exist: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "pre-edit-") && strings.HasSuffix(e.Name(), ".json") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pre-edit backup file to be created")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	var written map[string]interface{}
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatalf("written config should be standard JSON: %v", err)
	}
	if _, ok := written["tools"]; !ok {
		t.Fatalf("written config should preserve tools")
	}
	if _, ok := written["session"]; !ok {
		t.Fatalf("written config should preserve session")
	}
}
