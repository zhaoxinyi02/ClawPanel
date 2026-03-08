package update

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestNewUpdaterStartsIdle(t *testing.T) {
	updater := NewUpdater("v1.0.0", t.TempDir())

	progress := updater.GetProgress()
	if progress.Status != "idle" {
		t.Fatalf("status = %q, want idle", progress.Status)
	}
	if len(progress.Log) != 0 {
		t.Fatalf("log length = %d, want 0", len(progress.Log))
	}
}

func TestGetProgressReturnsLogCopy(t *testing.T) {
	updater := NewUpdater("v1.0.0", t.TempDir())
	updater.log("hello")

	progress := updater.GetProgress()
	progress.Log[0] = "mutated"

	fresh := updater.GetProgress()
	if fresh.Log[0] != "hello" {
		t.Fatalf("internal log was mutated through copy: %+v", fresh.Log)
	}
}

func TestUpdatePopupLifecycle(t *testing.T) {
	updater := NewUpdater("v1.0.0", t.TempDir())
	info := &UpdateInfo{
		LatestVersion: "v1.1.0",
		ReleaseNote:   "bug fixes",
	}

	updater.saveUpdatePopup(info)

	popup := updater.GetUpdatePopup()
	if popup == nil {
		t.Fatal("GetUpdatePopup() returned nil after save")
	}
	if !popup.Show || popup.Version != "v1.1.0" || popup.ReleaseNote != "bug fixes" {
		t.Fatalf("unexpected popup: %+v", popup)
	}

	updater.MarkPopupShown()
	popup = updater.GetUpdatePopup()
	if popup == nil {
		t.Fatal("popup disappeared after MarkPopupShown()")
	}
	if popup.Show {
		t.Fatal("popup should be marked hidden")
	}
	if popup.ShownAt == "" {
		t.Fatal("popup should record shown timestamp")
	}
}

func TestStatusAndErrorTransitions(t *testing.T) {
	updater := NewUpdater("v1.0.0", t.TempDir())

	updater.setStatus("downloading", 25, "downloading file")
	progress := updater.GetProgress()
	if progress.Status != "downloading" || progress.Progress != 25 || progress.Message != "downloading file" {
		t.Fatalf("unexpected progress after setStatus: %+v", progress)
	}

	updater.setError("download failed: %s", "timeout")
	progress = updater.GetProgress()
	if progress.Status != "error" {
		t.Fatalf("status = %q, want error", progress.Status)
	}
	if !strings.Contains(progress.Error, "timeout") {
		t.Fatalf("error = %q, want timeout", progress.Error)
	}
	if progress.FinishedAt == "" {
		t.Fatal("setError() should record finished time")
	}
}

func TestVersionAndFileHelpers(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v1.2.0", "v1.1.9", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.1.9", "v1.2.0", false},
		{"v1.2.0.1", "v1.2.0", true},
	}
	for _, tt := range tests {
		if got := isNewerVersion(tt.latest, tt.current); got != tt.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}

	dir := t.TempDir()
	src := dir + "/src.txt"
	dst := dir + "/dst.txt"
	content := []byte("clawpanel-update-test")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("WriteFile(src) error = %v", err)
	}

	sum := sha256.Sum256(content)
	wantSHA := hex.EncodeToString(sum[:])
	gotSHA, err := fileSHA256(src)
	if err != nil {
		t.Fatalf("fileSHA256() error = %v", err)
	}
	if gotSHA != wantSHA {
		t.Fatalf("fileSHA256() = %q, want %q", gotSHA, wantSHA)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}
	copiedSHA, err := fileSHA256(dst)
	if err != nil {
		t.Fatalf("fileSHA256(dst) error = %v", err)
	}
	if copiedSHA != wantSHA {
		t.Fatalf("copied file sha = %q, want %q", copiedSHA, wantSHA)
	}
}
