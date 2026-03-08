package handler

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestBuildNapCatInstallScriptUsesConfiguredQQToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "custom-openclaw")
	if err := os.MkdirAll(openClawDir, 0755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}

	raw := `{
  // json5 comment
  channels: {
    qq: {
      enabled: true,
      accessToken: "qq-token-$123",
    },
  },
}`
	if err := os.WriteFile(filepath.Join(openClawDir, "openclaw.json"), []byte(raw), 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	cfg := &config.Config{
		OpenClawDir:  openClawDir,
		OpenClawWork: filepath.Join(dir, "work"),
	}

	script := buildNapCatInstallScript(cfg)
	if strings.Contains(script, `${HOME}/.openclaw`) {
		t.Fatalf("script should not hardcode the default openclaw dir")
	}
	if !strings.Contains(script, openClawDir) {
		t.Fatalf("script should mount the configured openclaw dir")
	}

	expectedB64 := base64.StdEncoding.EncodeToString([]byte("qq-token-$123"))
	if !strings.Contains(script, expectedB64) {
		t.Fatalf("script should embed the configured qq access token via base64")
	}
	if !strings.Contains(script, "export WS_TOKEN_B64=") {
		t.Fatalf("script should export WS_TOKEN_B64 for the python subprocess")
	}
	if !strings.Contains(script, "WS_TOKEN_JSON=$(python3 - <<'PY'") {
		t.Fatalf("script should JSON-encode the token before writing onebot11.json")
	}
	if !strings.Contains(script, `\"token\": ${WS_TOKEN_JSON}`) {
		t.Fatalf("script should inject the JSON-encoded token into onebot11.json")
	}
}

func TestFormatOpenClawManualPrerequisiteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		platform string
		nodeVer string
		gitVer  string
		want    string
	}{
		{name: "windows missing both", platform: "windows", want: "检测到 windows 平台缺少 Node.js (>=20) 和 Git"},
		{name: "windows missing git only", platform: "windows", nodeVer: "v22.14.0", want: "检测到 windows 平台缺少 Git"},
		{name: "mac low node and missing git", platform: "darwin", nodeVer: "v18.20.1", want: "检测到 macOS 平台缺少 Node.js >=20 (当前 v18.20.1) 和 Git"},
		{name: "ready", platform: "windows", nodeVer: "v22.14.0", gitVer: "git version 2.53.0.windows.1", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := formatOpenClawManualPrerequisiteError(tt.platform, tt.nodeVer, tt.gitVer)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to contain %q, got %q", tt.want, err.Error())
			}
		})
	}
}
