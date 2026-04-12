package updater

import (
	"runtime"
	"testing"
)

func TestNewEditionConfig(t *testing.T) {
	t.Parallel()

	lite := newEditionConfig(" lite ")
	if lite.Edition != "lite" || lite.ServiceName != "clawpanel-lite" || lite.BinaryName != "clawpanel-lite" || lite.GitHubTagPrefix != "lite-v" {
		t.Fatalf("unexpected lite config: %+v", lite)
	}

	pro := newEditionConfig("something-else")
	if pro.Edition != "pro" || pro.ServiceName != "clawpanel" || pro.BinaryName != "clawpanel" || pro.GitHubTagPrefix != "pro-v" {
		t.Fatalf("unexpected pro config: %+v", pro)
	}
}

func TestEditionConfigTagHelpers(t *testing.T) {
	t.Parallel()

	lite := newEditionConfig("lite")
	if !lite.matchesTag(" lite-v1.2.3 ") {
		t.Fatal("matchesTag() should accept trimmed lite tag")
	}
	if got := lite.trimTag(" lite-v1.2.3 "); got != "1.2.3" {
		t.Fatalf("trimTag() = %q, want 1.2.3", got)
	}

	pro := newEditionConfig("pro")
	if pro.matchesTag("lite-v1.2.3") {
		t.Fatal("matchesTag() should reject wrong edition prefix")
	}
}

func TestEditionConfigAssetHelpers(t *testing.T) {
	t.Parallel()

	pro := newEditionConfig("pro")
	if got := pro.assetPrefix("1.2.3"); got != "clawpanel-v1.2.3" {
		t.Fatalf("assetPrefix(pro) = %q", got)
	}
	if got := pro.binaryAssetName("1.2.3", "linux_amd64"); got != "clawpanel-v1.2.3-linux-amd64" {
		t.Fatalf("binaryAssetName(pro linux) = %q", got)
	}
	if got := pro.binaryAssetName("1.2.3", "windows_amd64"); got != "clawpanel-v1.2.3-windows-amd64.exe" {
		t.Fatalf("binaryAssetName(pro windows) = %q", got)
	}
	if got := pro.updateAssetName("1.2.3", "darwin_arm64"); got != "clawpanel-v1.2.3-darwin-arm64" {
		t.Fatalf("updateAssetName(pro) = %q", got)
	}
	if pro.isLiteFullPackage() {
		t.Fatal("isLiteFullPackage() should be false for pro")
	}

	lite := newEditionConfig("lite")
	if got := lite.assetPrefix("1.2.3"); got != "clawpanel-lite-core-v1.2.3" {
		t.Fatalf("assetPrefix(lite) = %q", got)
	}
	if got := lite.binaryAssetName("1.2.3", "linux_amd64"); got != "clawpanel-lite-v1.2.3-linux-amd64" {
		t.Fatalf("binaryAssetName(lite linux) = %q", got)
	}
	if got := lite.liteCoreAssetName("1.2.3", "linux_amd64"); got != "clawpanel-lite-core-v1.2.3-linux-amd64.tar.gz" {
		t.Fatalf("liteCoreAssetName() = %q", got)
	}
	if got := lite.liteCoreAssetName("1.2.3", " "); got != "" {
		t.Fatalf("liteCoreAssetName(empty) = %q, want empty string", got)
	}
	if got := lite.updateAssetName("1.2.3", "darwin_arm64"); got != "clawpanel-lite-core-v1.2.3-darwin-arm64.tar.gz" {
		t.Fatalf("updateAssetName(lite) = %q", got)
	}
	if !lite.isLiteFullPackage() {
		t.Fatal("isLiteFullPackage() should be true for lite")
	}
}

func TestEditionConfigMatchUpdateAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       editionConfig
		version   string
		assetName string
		wantKey   string
		wantMatch bool
	}{
		{
			name:      "pro linux asset",
			cfg:       newEditionConfig("pro"),
			version:   "1.2.3",
			assetName: " clawpanel-v1.2.3-linux-amd64 ",
			wantKey:   "linux_amd64",
			wantMatch: true,
		},
		{
			name:      "lite darwin asset",
			cfg:       newEditionConfig("lite"),
			version:   "1.2.3",
			assetName: "clawpanel-lite-core-v1.2.3-darwin-arm64.tar.gz",
			wantKey:   "darwin_arm64",
			wantMatch: true,
		},
		{
			name:      "unknown asset",
			cfg:       newEditionConfig("pro"),
			version:   "1.2.3",
			assetName: "clawpanel-v1.2.3-freebsd-amd64",
			wantKey:   "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotKey, gotMatch := tt.cfg.matchUpdateAsset(tt.version, tt.assetName)
			if gotKey != tt.wantKey || gotMatch != tt.wantMatch {
				t.Fatalf("matchUpdateAsset(%q) = (%q, %v), want (%q, %v)", tt.assetName, gotKey, gotMatch, tt.wantKey, tt.wantMatch)
			}
		})
	}
}

func TestEditionConfigLauncherName(t *testing.T) {
	t.Parallel()

	cfg := newEditionConfig("lite")
	want := "clawlite-openclaw"
	if runtime.GOOS == "windows" {
		want = "clawlite-openclaw.cmd"
	}
	if got := cfg.launcherName(); got != want {
		t.Fatalf("launcherName() = %q, want %q", got, want)
	}
}
