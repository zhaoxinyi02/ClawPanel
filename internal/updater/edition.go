package updater

import (
	"fmt"
	"runtime"
	"strings"
)

type editionConfig struct {
	Edition           string
	ServiceName       string
	ServiceLabel      string
	BinaryName        string
	AccelUpdateJSON   string
	GitHubReleasesAPI string
	GitHubTagPrefix   string
}

func newEditionConfig(edition string) editionConfig {
	if strings.EqualFold(strings.TrimSpace(edition), "lite") {
		return editionConfig{
			Edition:           "lite",
			ServiceName:       "clawpanel-lite",
			ServiceLabel:      "com.clawpanel.lite.service",
			BinaryName:        "clawpanel-lite",
			AccelUpdateJSON:   "http://127.0.0.1:19527/api/panel/update-mirror/lite",
			GitHubReleasesAPI: "https://api.github.com/repos/zhaoxinyi02/ClawPanel/releases?per_page=20",
			GitHubTagPrefix:   "lite-v",
		}
	}
	return editionConfig{
		Edition:           "pro",
		ServiceName:       "clawpanel",
		ServiceLabel:      "com.clawpanel.service",
		BinaryName:        "clawpanel",
		AccelUpdateJSON:   "http://127.0.0.1:19527/api/panel/update-mirror/pro",
		GitHubReleasesAPI: "https://api.github.com/repos/zhaoxinyi02/ClawPanel/releases?per_page=20",
		GitHubTagPrefix:   "pro-v",
	}
}

func (c editionConfig) matchesTag(tag string) bool {
	return strings.HasPrefix(strings.TrimSpace(tag), c.GitHubTagPrefix)
}

func (c editionConfig) trimTag(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), c.GitHubTagPrefix)
}

func (c editionConfig) assetPrefix(version string) string {
	if c.Edition == "lite" {
		return fmt.Sprintf("clawpanel-lite-core-v%s", version)
	}
	return fmt.Sprintf("clawpanel-v%s", version)
}

func (c editionConfig) isLiteFullPackage() bool {
	return c.Edition == "lite"
}

func (c editionConfig) binaryAssetName(version, platformKey string) string {
	prefix := "clawpanel"
	if c.Edition == "lite" {
		prefix = "clawpanel-lite"
	}
	name := fmt.Sprintf("%s-v%s-%s", prefix, version, strings.ReplaceAll(platformKey, "_", "-"))
	if strings.HasPrefix(platformKey, "windows_") {
		name += ".exe"
	}
	return name
}

func (c editionConfig) liteCoreAssetName(version, platformKey string) string {
	suffix := strings.ReplaceAll(platformKey, "_", "-")
	if strings.TrimSpace(suffix) == "" {
		return ""
	}
	return fmt.Sprintf("clawpanel-lite-core-v%s-%s.tar.gz", version, suffix)
}

func (c editionConfig) updateAssetName(version, platformKey string) string {
	if c.isLiteFullPackage() {
		return c.liteCoreAssetName(version, platformKey)
	}
	return c.binaryAssetName(version, platformKey)
}

func (c editionConfig) matchUpdateAsset(version string, releaseAssetName string) (string, bool) {
	assetName := strings.TrimSpace(releaseAssetName)
	for _, platformKey := range []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64"} {
		if expected := c.updateAssetName(version, platformKey); expected != "" && assetName == expected {
			return platformKey, true
		}
	}
	return "", false
}

func (c editionConfig) launcherName() string {
	if runtime.GOOS == "windows" {
		return "clawlite-openclaw.cmd"
	}
	return "clawlite-openclaw"
}
