package updatemirror

import "testing"

func TestParseChecksums(t *testing.T) {
	t.Parallel()

	raw := "abc123  clawpanel-v5.3.3-linux-amd64\n" +
		"def456 *clawpanel-v5.3.3-windows-amd64.exe\n" +
		"\n"

	got := parseChecksums(raw)
	if got["clawpanel-v5.3.3-linux-amd64"] != "abc123" {
		t.Fatalf("unexpected linux checksum: %#v", got)
	}
	if got["clawpanel-v5.3.3-windows-amd64.exe"] != "def456" {
		t.Fatalf("unexpected windows checksum: %#v", got)
	}
}

func TestMatchUpdateAsset(t *testing.T) {
	t.Parallel()

	if platform, ok := matchUpdateAsset("pro", "5.3.3", "clawpanel-v5.3.3-linux-amd64"); !ok || platform != "linux_amd64" {
		t.Fatalf("expected pro linux asset match, got %q %v", platform, ok)
	}
	if platform, ok := matchUpdateAsset("lite", "0.2.3", "clawpanel-lite-core-v0.2.3-linux-amd64.tar.gz"); !ok || platform != "linux_amd64" {
		t.Fatalf("expected lite linux asset match, got %q %v", platform, ok)
	}
	if _, ok := matchUpdateAsset("pro", "5.3.3", "checksums.txt"); ok {
		t.Fatalf("checksums.txt should not be treated as update asset")
	}
}
