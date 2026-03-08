package updater

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestNewServerInitializesState(t *testing.T) {
	server := NewServer("v1.2.3", t.TempDir(), "/tmp/openclaw", 19527)

	if server.currentVersion != "v1.2.3" {
		t.Fatalf("currentVersion = %q, want v1.2.3", server.currentVersion)
	}
	if server.state.Phase != "idle" || server.ocState.Phase != "idle" {
		t.Fatalf("unexpected initial phases: state=%q ocState=%q", server.state.Phase, server.ocState.Phase)
	}
	if len(server.state.Steps) != len(defaultSteps()) {
		t.Fatalf("step count = %d, want %d", len(server.state.Steps), len(defaultSteps()))
	}
	if len(server.ocState.Steps) != len(defaultOCSteps()) {
		t.Fatalf("oc step count = %d, want %d", len(server.ocState.Steps), len(defaultOCSteps()))
	}
	if server.panelBin == "" {
		t.Fatal("panelBin should not be empty")
	}
}

func TestGenerateTokenAndValidateToken(t *testing.T) {
	token := GenerateToken(19527)
	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}
	if !ValidateToken(token, 19527) {
		t.Fatal("ValidateToken() should accept freshly generated token")
	}
	if ValidateToken(token, 19528) {
		t.Fatal("ValidateToken() should reject token for different port")
	}
}

func TestUpdaterHTMLIncludesInjectedValues(t *testing.T) {
	html := updaterHTML("v1.0.0", "test-token", 19527)

	for _, want := range []string{"ClawPanel 更新工具", "v1.0.0", "test-token", "19527"} {
		if !strings.Contains(html, want) {
			t.Fatalf("updaterHTML() missing %q", want)
		}
	}
}

func TestSetCORSAndCheckToken(t *testing.T) {
	server := NewServer("v1.0.0", t.TempDir(), "", 19527)
	token := GenerateToken(19527)

	recorder := httptest.NewRecorder()
	server.setCORS(recorder)
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow origin = %q, want *", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/updater?token="+url.QueryEscape(token), nil)
	if !server.checkToken(recorder, req) {
		t.Fatal("checkToken() should accept query token")
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/updater", nil)
	req.Header.Set("X-Update-Token", token)
	if !server.checkToken(recorder, req) {
		t.Fatal("checkToken() should accept header token")
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/updater?token=bad", nil)
	if server.checkToken(recorder, req) {
		t.Fatal("checkToken() should reject invalid token")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestUpdaterUtilityHelpers(t *testing.T) {
	if got := getPlatformKey(); got != runtime.GOOS+"_"+runtime.GOARCH {
		t.Fatalf("getPlatformKey() = %q, want %q", got, runtime.GOOS+"_"+runtime.GOARCH)
	}

	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v1.0.0", false},
		{"v0.9.9", "v1.0.0", false},
	}
	for _, tt := range tests {
		if got := isNewerVersion(tt.latest, tt.current); got != tt.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}

	dir := t.TempDir()
	src := dir + "/src.txt"
	dst := dir + "/dst.txt"
	if err := os.WriteFile(src, []byte("updater-test"), 0644); err != nil {
		t.Fatalf("WriteFile(src) error = %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}
	srcSHA, err := fileSHA256(src)
	if err != nil {
		t.Fatalf("fileSHA256(src) error = %v", err)
	}
	dstSHA, err := fileSHA256(dst)
	if err != nil {
		t.Fatalf("fileSHA256(dst) error = %v", err)
	}
	if srcSHA != dstSHA {
		t.Fatalf("sha mismatch src=%q dst=%q", srcSHA, dstSHA)
	}

	if ternary(true, "a", "b") != "a" || ternary(false, "a", "b") != "b" {
		t.Fatal("ternary() returned unexpected result")
	}
}
