package process

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestGatewayListening(t *testing.T) {

	openclawDir := newOpenClawDir(t)
	ln, port := listenTCP(t)
	defer ln.Close()
	writeGatewayConfig(t, openclawDir, port)

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.gatewayProbe = listeningProbe(port)
	if !mgr.GatewayListening() {
		t.Fatalf("expected GatewayListening to detect active gateway port %d", port)
	}
}

func TestShouldManageQQIntegrationStateRequiresExplicitOptIn(t *testing.T) {
	t.Parallel()

	manage, managedByNapCat := shouldManageQQIntegrationState(true, true, false, false)
	if manage || managedByNapCat {
		t.Fatalf("expected no QQ auto-management without flag/config, got manage=%v managedByNapCat=%v", manage, managedByNapCat)
	}

	manage, managedByNapCat = shouldManageQQIntegrationState(true, true, true, false)
	if !manage || !managedByNapCat {
		t.Fatalf("expected flagged QQ integration to be managed, got manage=%v managedByNapCat=%v", manage, managedByNapCat)
	}

	manage, managedByNapCat = shouldManageQQIntegrationState(false, false, false, true)
	if !manage || managedByNapCat {
		t.Fatalf("expected existing QQ config to be preserved without NapCat ownership, got manage=%v managedByNapCat=%v", manage, managedByNapCat)
	}
}

func TestGatewayListeningFalseWhenPortClosed(t *testing.T) {

	openclawDir := newOpenClawDir(t)
	ln, port := listenTCP(t)
	_ = ln.Close()
	writeGatewayConfig(t, openclawDir, port)

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.gatewayProbe = listeningProbe(port)
	if mgr.GatewayListening() {
		t.Fatalf("expected GatewayListening to be false once port %d is closed", port)
	}
}

func TestGatewayListeningIgnoresNonOpenClawListener(t *testing.T) {

	openclawDir := newOpenClawDir(t)
	ln, port := listenTCP(t)
	defer ln.Close()
	writeGatewayConfig(t, openclawDir, port)

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.gatewayProbe = func(_ string, _ string) bool { return false }
	if mgr.GatewayListening() {
		t.Fatalf("expected GatewayListening to ignore non-OpenClaw listener on port %d", port)
	}
}

func TestLooksLikeOpenClawGatewayResponseRecognizesControlUI(t *testing.T) {
	headers := http.Header{}
	body := []byte("<!doctype html><title>OpenClaw Control</title><openclaw-app></openclaw-app>")
	if !looksLikeOpenClawGatewayResponse("/", 200, headers, body) {
		t.Fatalf("expected control UI HTML to be recognized as OpenClaw gateway")
	}
}

func TestLooksLikeOpenClawGatewayResponseRecognizesHealthJSON(t *testing.T) {
	headers := http.Header{"Content-Type": []string{"application/json"}}
	body := []byte(`{"status":"live"}`)
	if !looksLikeOpenClawGatewayResponse("/healthz", 200, headers, body) {
		t.Fatalf("expected health JSON to be recognized as OpenClaw gateway")
	}
}

func TestGetStatusReportsExternallyManagedGateway(t *testing.T) {

	openclawDir := newOpenClawDir(t)
	ln, port := listenTCP(t)
	defer ln.Close()
	writeGatewayConfig(t, openclawDir, port)

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.gatewayProbe = listeningProbe(port)
	status := mgr.GetStatus()
	if !status.Running {
		t.Fatalf("expected external gateway to be reported as running")
	}
	if !status.ManagedExternally {
		t.Fatalf("expected external gateway to be marked as managed externally")
	}
}

func TestStartRejectsExternallyManagedGateway(t *testing.T) {

	openclawDir := newOpenClawDir(t)
	ln, port := listenTCP(t)
	defer ln.Close()
	writeGatewayConfig(t, openclawDir, port)

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.gatewayProbe = listeningProbe(port)
	err := mgr.Start()
	if err == nil || !strings.Contains(err.Error(), "外部进程管理") {
		t.Fatalf("expected Start to reject externally managed gateway, got %v", err)
	}
}

func TestStopRejectsDaemonizedGateway(t *testing.T) {

	mgr := NewManager(&config.Config{})
	mgr.status = Status{Running: true}
	mgr.daemonized = true

	err := mgr.Stop()
	if err == nil || !strings.Contains(err.Error(), "daemon fork 模式") {
		t.Fatalf("expected Stop to reject daemonized gateway, got %v", err)
	}
}

func TestStartRejectsOccupiedNonOpenClawPort(t *testing.T) {

	openclawDir := newOpenClawDir(t)
	ln, port := listenTCP(t)
	defer ln.Close()
	writeGatewayConfig(t, openclawDir, port)

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.gatewayProbe = func(_ string, _ string) bool { return false }
	err := mgr.Start()
	if err == nil || !strings.Contains(err.Error(), "已被其他本地服务占用") {
		t.Fatalf("expected Start to reject occupied non-OpenClaw port, got %v", err)
	}
}

func TestGatewayPortCheckTargetsLoopbackBindUsesLoopbackOnly(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2", "fd00:1234:ffff::10"}
	got := gatewayPortCheckTargets("loopback", "", allTargets)
	want := []string{"127.0.0.1", "localhost", "::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected loopback-only targets, got %#v", got)
	}
}

func TestGatewayPortCheckTargetsCustomHostUsesCustomTarget(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2", "fd00:1234:ffff::10"}
	got := gatewayPortCheckTargets("custom", "10.0.0.2", allTargets)
	want := []string{"10.0.0.2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected custom bind host only, got %#v", got)
	}
}

func TestGatewayPortCheckTargetsCustomLoopbackUsesExactHost(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2", "fd00:1234:ffff::10"}
	got := gatewayPortCheckTargets("custom", "127.0.0.1", allTargets)
	want := []string{"127.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected exact custom loopback host, got %#v", got)
	}
}

func TestGatewayPortCheckTargetsDefaultUsesLoopbackOnly(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2", "fd00:1234:ffff::10"}
	got := gatewayPortCheckTargets("", "", allTargets)
	want := []string{"127.0.0.1", "localhost", "::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected default bind to use loopback targets, got %#v", got)
	}
}

func TestGatewayPortCheckTargetsLanIgnoresStaleCustomHost(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2", "fd00:1234:ffff::10"}
	got := gatewayPortCheckTargets("lan", "127.0.0.1", allTargets)
	if !reflect.DeepEqual(got, allTargets) {
		t.Fatalf("expected lan bind to ignore stale custom host, got %#v", got)
	}
}

func TestGatewayPortCheckTargetsTailnetUsesTailnetAddresses(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2", "100.100.100.1", "fd7a:115c:a1e0::1"}
	got := gatewayPortCheckTargets("tailnet", "", allTargets)
	want := []string{"100.100.100.1", "fd7a:115c:a1e0::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected tailnet-only targets, got %#v", got)
	}
}

func TestGatewayConfiguredTargetsAutoFallsBackToAllTargetsWhenLoopbackUnavailable(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2"}
	got := gatewayConfiguredTargets("auto", "", allTargets, func(string) bool { return false })
	if !reflect.DeepEqual(got, allTargets) {
		t.Fatalf("expected auto bind to fall back to all targets when loopback is unavailable, got %#v", got)
	}
}

func TestGatewayConfiguredTargetsTailnetFallsBackLikeUpstream(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2"}
	got := gatewayConfiguredTargets("tailnet", "", allTargets, func(host string) bool { return host == "127.0.0.1" })
	want := []string{"127.0.0.1", "localhost", "::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected tailnet bind without tailnet IP to fall back to loopback when it is available, got %#v", got)
	}
	got = gatewayConfiguredTargets("tailnet", "", allTargets, func(string) bool { return false })
	if !reflect.DeepEqual(got, allTargets) {
		t.Fatalf("expected tailnet bind without tailnet or loopback availability to fall back to all targets, got %#v", got)
	}
}

func TestGatewayConfiguredTargetsTreatsIPv6LoopbackAsAvailable(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2"}
	got := gatewayConfiguredTargets("loopback", "", allTargets, func(host string) bool { return host == "::1" })
	want := []string{"127.0.0.1", "localhost", "::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected IPv6 loopback availability to keep loopback targets, got %#v", got)
	}
}

func TestGatewayConfiguredTargetsCustomLoopbackRequiresThatHost(t *testing.T) {
	allTargets := []string{"127.0.0.1", "localhost", "::1", "10.0.0.2"}
	got := gatewayConfiguredTargets("custom", "127.0.0.1", allTargets, func(host string) bool { return host == "::1" })
	if !reflect.DeepEqual(got, allTargets) {
		t.Fatalf("expected custom loopback host to fall back when only a different loopback is bindable, got %#v", got)
	}
}

func TestGetGatewayPortCheckTargetsUsesRuntimeFallbackWhenLoopbackUnavailable(t *testing.T) {
	openclawDir := newOpenClawDir(t)
	cfgPath := filepath.Join(openclawDir, "openclaw.json")
	if err := os.WriteFile(cfgPath, []byte(`{"gateway":{"bind":"loopback"}}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mgr := NewManager(&config.Config{OpenClawDir: openclawDir})
	mgr.bindHostCheck = func(string) bool { return false }
	got := mgr.getGatewayPortCheckTargets()
	want := collectGatewayCandidateTargets()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected port-check targets to follow runtime fallback when loopback is unavailable, got %#v", got)
	}
}

func newOpenClawDir(t *testing.T) string {
	t.Helper()
	openclawDir := filepath.Join(t.TempDir(), ".openclaw")
	if err := os.MkdirAll(openclawDir, 0755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	return openclawDir
}

func writeGatewayConfig(t *testing.T, openclawDir string, port int) {
	t.Helper()
	cfgPath := filepath.Join(openclawDir, "openclaw.json")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{"gateway":{"port":%d}}`, port)), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func listenTCP(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port
}

func listeningProbe(port int) func(string, string) bool {
	expectedPort := strconv.Itoa(port)
	return func(host, actualPort string) bool {
		if actualPort != expectedPort {
			return false
		}
		if host != "localhost" {
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				return false
			}
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, actualPort), 200*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}
}
