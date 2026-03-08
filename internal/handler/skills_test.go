package handler

import (
	"path/filepath"
	"testing"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestResolveWorkspaceSkillDirsIncludesWorkspacePathAndAgentWorkspaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	base := filepath.Join(root, ".openclaw")
	cfg := &config.Config{
		OpenClawDir:  base,
		OpenClawWork: filepath.Join(root, "work"),
	}
	ocConfig := map[string]interface{}{
		"workspace": map[string]interface{}{
			"path": "workspace-data",
		},
		"agents": map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{"id": "main", "workspace": "agents/main-workspace"},
				map[string]interface{}{"id": "ops", "workspace": filepath.Join(root, "external", "ops-workspace")},
			},
		},
	}

	dirs := resolveWorkspaceSkillDirs(cfg, ocConfig)
	parent := filepath.Dir(base)
	expected := []string{
		filepath.Join(cfg.OpenClawWork, "skills"),
		filepath.Join(parent, "workspace-data", "skills"),
		filepath.Join(parent, "agents", "main-workspace", "skills"),
		filepath.Join(root, "external", "ops-workspace", "skills"),
	}

	if len(dirs) != len(expected) {
		t.Fatalf("expected %d dirs, got %d: %#v", len(expected), len(dirs), dirs)
	}
	for i, want := range expected {
		if dirs[i] != filepath.Clean(want) {
			t.Fatalf("dir[%d] = %q, want %q", i, dirs[i], filepath.Clean(want))
		}
	}
}
