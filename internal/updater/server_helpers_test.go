package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

func createLitePackageTree(t *testing.T, root string) (binaryName, launcherName, nodeRel string) {
	t.Helper()

	cfg := newEditionConfig("lite")
	binaryName = cfg.BinaryName
	launcherName = cfg.launcherName()
	nodeRel = filepath.Join("runtime", "node", "bin", "node")
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
		nodeRel = filepath.Join("runtime", "node", "node.exe")
	}

	dirs := []string{
		filepath.Join(root, "bin"),
		filepath.Join(root, "runtime", "openclaw"),
		filepath.Dir(filepath.Join(root, nodeRel)),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, binaryName), []byte("panel"), 0755); err != nil {
		t.Fatalf("WriteFile(binary) error = %v", err)
	}

	launcherPath := filepath.Join(root, "bin", launcherName)
	launcherBody := []byte("#!/bin/sh\nexit 0\n")
	launcherMode := os.FileMode(0755)
	if runtime.GOOS == "windows" {
		launcherBody = []byte("@echo off\r\nexit /b 0\r\n")
		launcherMode = 0644
	}
	if err := os.WriteFile(launcherPath, launcherBody, launcherMode); err != nil {
		t.Fatalf("WriteFile(launcher) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, nodeRel), []byte("node"), 0755); err != nil {
		t.Fatalf("WriteFile(node) error = %v", err)
	}

	return binaryName, launcherName, nodeRel
}

func createTarGzFromDir(t *testing.T, srcDir, archivePath string) {
	t.Helper()

	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", archivePath, err)
	}
	defer archiveFile.Close()

	gzw := gzip.NewWriter(archiveFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil || rel == "." {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		t.Fatalf("Walk(%q) error = %v", srcDir, err)
	}
}

func TestServerStateHelpers(t *testing.T) {
	t.Parallel()

	server := NewServer("v1.0.0", t.TempDir(), "", 19527, "pro")

	server.setStep(0, "running", "checking")
	if step := server.state.Steps[0]; step.Status != "running" || step.Message != "checking" {
		t.Fatalf("unexpected step state after setStep(): %+v", step)
	}

	server.setProgress(42)
	if server.state.Progress != 42 {
		t.Fatalf("progress = %d, want 42", server.state.Progress)
	}

	server.setPhase("done")
	if server.state.Phase != "done" || server.state.FinishedAt == "" {
		t.Fatalf("unexpected phase after setPhase(done): %+v", server.state)
	}

	server.setStepError(1, "failed")
	if step := server.state.Steps[1]; step.Status != "error" || step.Message != "failed" {
		t.Fatalf("unexpected step state after setStepError(): %+v", step)
	}
	if server.state.Phase != "error" || server.state.FinishedAt == "" {
		t.Fatalf("unexpected phase after setStepError(): %+v", server.state)
	}

	server.logMsg("hello %s", "world")
	if got := server.state.Log[len(server.state.Log)-1]; got != "hello world" {
		t.Fatalf("log entry = %q, want hello world", got)
	}

	server.setError("boom")
	if server.state.Phase != "error" || server.state.Error != "boom" || server.state.Message != "更新失败" {
		t.Fatalf("unexpected state after setError(): %+v", server.state)
	}
	if got := server.state.Log[len(server.state.Log)-1]; got != "❌ boom" {
		t.Fatalf("last error log = %q, want ❌ boom", got)
	}
}

func TestRecordUpdateLogKeepsLast50Entries(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	server := NewServer("v1.0.0", dataDir, "", 19527, "pro")

	for i := 0; i < 55; i++ {
		server.state.FromVer = "v" + strconv.Itoa(i)
		server.state.ToVer = "v" + strconv.Itoa(i+1)
		server.state.Source = "github"
		server.state.Phase = "done"
		server.state.StartedAt = "start-" + strconv.Itoa(i)
		server.state.FinishedAt = "end-" + strconv.Itoa(i)
		server.recordUpdateLog()
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "update_history.json"))
	if err != nil {
		t.Fatalf("ReadFile(update_history.json) error = %v", err)
	}

	var history []map[string]interface{}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatalf("Unmarshal(history) error = %v", err)
	}
	if len(history) != 50 {
		t.Fatalf("history length = %d, want 50", len(history))
	}
	if history[0]["from"] != "v5" || history[len(history)-1]["from"] != "v54" {
		t.Fatalf("unexpected retained history range: first=%v last=%v", history[0]["from"], history[len(history)-1]["from"])
	}
}

func TestDownloadSourceHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw       string
		wantNorm  string
		wantOrder []string
	}{
		{raw: " github ", wantNorm: "github", wantOrder: []string{"github", "accel"}},
		{raw: "ACCEL", wantNorm: "accel", wantOrder: []string{"accel", "github"}},
		{raw: "other", wantNorm: "", wantOrder: []string{"github", "accel"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			if got := normalizeDownloadSource(tt.raw); got != tt.wantNorm {
				t.Fatalf("normalizeDownloadSource(%q) = %q, want %q", tt.raw, got, tt.wantNorm)
			}
			if got := downloadSourceOrder(tt.raw); !reflect.DeepEqual(got, tt.wantOrder) {
				t.Fatalf("downloadSourceOrder(%q) = %v, want %v", tt.raw, got, tt.wantOrder)
			}
		})
	}
}

func TestIsLikelySystemdServiceProcessUsesInvocationID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("INVOCATION_ID-based detection is not used on Windows")
	}

	t.Setenv("INVOCATION_ID", "unit-test")
	if !isLikelySystemdServiceProcess() {
		t.Fatal("isLikelySystemdServiceProcess() should return true when INVOCATION_ID is set")
	}
}

func TestCommandExistsAndIsPortOpen(t *testing.T) {
	t.Parallel()

	existing := "sh"
	if runtime.GOOS == "windows" {
		existing = "cmd"
	}
	if !commandExists(existing) {
		t.Fatalf("commandExists(%q) = false, want true", existing)
	}
	if commandExists("clawpanel-command-that-should-not-exist") {
		t.Fatal("commandExists() unexpectedly found a fake command")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if !isPortOpen(port) {
		t.Fatalf("isPortOpen(%d) = false, want true while listener is active", port)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestLitePackageLifecycleHelpers(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "source")
	binaryName, launcherName, nodeRel := createLitePackageTree(t, sourceDir)

	archivePath := filepath.Join(baseDir, "lite.tar.gz")
	createTarGzFromDir(t, sourceDir, archivePath)

	extractDir := filepath.Join(baseDir, "extract")
	if err := extractTarGz(archivePath, extractDir); err != nil {
		t.Fatalf("extractTarGz() error = %v", err)
	}
	if err := validateLitePackage(extractDir); err != nil {
		t.Fatalf("validateLitePackage(extractDir) error = %v", err)
	}

	installDir := filepath.Join(baseDir, "install")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("MkdirAll(installDir) error = %v", err)
	}
	if err := applyLitePackage(extractDir, installDir, binaryName); err != nil {
		t.Fatalf("applyLitePackage() error = %v", err)
	}

	for _, rel := range []string{binaryName, filepath.Join("bin", launcherName), filepath.Join("runtime", "openclaw"), nodeRel} {
		if _, err := os.Stat(filepath.Join(installDir, rel)); err != nil {
			t.Fatalf("expected installed path %q: %v", rel, err)
		}
	}

	backupDir := filepath.Join(baseDir, "backup")
	createLitePackageTree(t, backupDir)

	if err := os.RemoveAll(filepath.Join(installDir, "bin")); err != nil {
		t.Fatalf("RemoveAll(bin) error = %v", err)
	}
	if err := rollbackLitePackage(backupDir, installDir, binaryName); err != nil {
		t.Fatalf("rollbackLitePackage() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "bin", launcherName)); err != nil {
		t.Fatalf("expected restored launcher after rollback: %v", err)
	}

	if runtime.GOOS != "windows" {
		server := NewServer("v1.0.0", baseDir, "", 19527, "lite")
		server.panelBin = filepath.Join(installDir, binaryName)
		if !server.isLiteRuntimeReady() {
			t.Fatal("isLiteRuntimeReady() should return true for a valid lite runtime tree")
		}
	}

	proServer := NewServer("v1.0.0", baseDir, "", 19527, "pro")
	if !proServer.isLiteRuntimeReady() {
		t.Fatal("isLiteRuntimeReady() should always return true for pro edition")
	}
}
