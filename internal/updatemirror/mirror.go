package updatemirror

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const cacheTTL = 10 * time.Minute

type EditionSpec struct {
	Edition           string
	GitHubReleasesAPI string
	GitHubTagPrefix   string
}

type Manifest struct {
	LatestVersion      string            `json:"latest_version"`
	ReleaseTime        string            `json:"release_time"`
	ReleaseNote        string            `json:"release_note"`
	DownloadURLs       map[string]string `json:"download_urls"`
	SHA256             map[string]string `json:"sha256"`
	RemoteDownloadURLs map[string]string `json:"remote_download_urls,omitempty"`
	AssetNames         map[string]string `json:"asset_names,omitempty"`
	FetchedAt          string            `json:"fetched_at,omitempty"`
	LocalPaths         map[string]string `json:"-"`
}

func ResolveLatest(dataDir string, spec EditionSpec, publicBasePath string, force bool) (*Manifest, error) {
	if !force {
		if cached, ok := loadCachedManifest(dataDir, spec, publicBasePath, false); ok {
			return cached, nil
		}
	}

	manifest, err := fetchLatestManifest(dataDir, spec, publicBasePath)
	if err == nil {
		if writeErr := writeCachedManifest(dataDir, spec, manifest); writeErr != nil {
			return manifest, writeErr
		}
		return manifest, nil
	}

	if cached, ok := loadCachedManifest(dataDir, spec, publicBasePath, true); ok {
		return cached, nil
	}
	return nil, err
}

func EnsureAsset(dataDir string, spec EditionSpec, manifest *Manifest, platformKey string) (string, error) {
	if manifest == nil {
		return "", fmt.Errorf("更新元数据为空")
	}
	if strings.TrimSpace(platformKey) == "" {
		return "", fmt.Errorf("平台标识为空")
	}
	assetName := strings.TrimSpace(manifest.AssetNames[platformKey])
	if assetName == "" {
		return "", fmt.Errorf("未找到平台 %s 对应的更新资产", platformKey)
	}
	remoteURL := strings.TrimSpace(manifest.RemoteDownloadURLs[platformKey])
	if remoteURL == "" {
		return "", fmt.Errorf("未找到平台 %s 对应的远程下载地址", platformKey)
	}
	expectedSHA := strings.TrimSpace(manifest.SHA256[platformKey])
	if expectedSHA == "" {
		return "", fmt.Errorf("平台 %s 缺少 SHA256 校验值，已拒绝更新", platformKey)
	}

	dest := filepath.Join(filesDir(dataDir, spec), assetName)
	if ok, err := verifyExistingFile(dest, expectedSHA); err == nil && ok {
		if manifest.LocalPaths == nil {
			manifest.LocalPaths = map[string]string{}
		}
		manifest.LocalPaths[platformKey] = dest
		return dest, nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	tmpPath := dest + ".tmp"
	if err := downloadToFile(remoteURL, tmpPath); err != nil {
		return "", err
	}
	actualSHA, err := fileSHA256(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if !strings.EqualFold(actualSHA, expectedSHA) {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("SHA256 校验失败: 期望 %s..., 实际 %s...", prefixSHA(expectedSHA), prefixSHA(actualSHA))
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Chmod(dest, 0o755); err != nil && !strings.HasSuffix(strings.ToLower(dest), ".tar.gz") {
		return "", err
	}
	if manifest.LocalPaths == nil {
		manifest.LocalPaths = map[string]string{}
	}
	manifest.LocalPaths[platformKey] = dest
	return dest, nil
}

func ManifestPath(dataDir string, spec EditionSpec) string {
	return filepath.Join(rootDir(dataDir, spec), "update.json")
}

func AssetPath(dataDir string, spec EditionSpec, filename string) string {
	return filepath.Join(filesDir(dataDir, spec), filepath.Base(filename))
}

func rootDir(dataDir string, spec EditionSpec) string {
	return filepath.Join(dataDir, "update-mirror", strings.TrimSpace(spec.Edition))
}

func filesDir(dataDir string, spec EditionSpec) string {
	return filepath.Join(rootDir(dataDir, spec), "files")
}

func fetchLatestManifest(dataDir string, spec EditionSpec, publicBasePath string) (*Manifest, error) {
	client := newHTTPClient(30 * time.Second)
	resp, err := client.Get(spec.GitHubReleasesAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Releases API 返回 HTTP %d", resp.StatusCode)
	}

	var releases []struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		PubAt   string `json:"published_at"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	for _, release := range releases {
		if !strings.HasPrefix(strings.TrimSpace(release.TagName), strings.TrimSpace(spec.GitHubTagPrefix)) {
			continue
		}
		version := strings.TrimPrefix(strings.TrimSpace(release.TagName), strings.TrimSpace(spec.GitHubTagPrefix))
		manifest := &Manifest{
			LatestVersion:      version,
			ReleaseTime:        release.PubAt,
			ReleaseNote:        release.Body,
			DownloadURLs:       map[string]string{},
			SHA256:             map[string]string{},
			RemoteDownloadURLs: map[string]string{},
			AssetNames:         map[string]string{},
			LocalPaths:         map[string]string{},
			FetchedAt:          time.Now().Format(time.RFC3339),
		}

		checksumsURL := ""
		for _, asset := range release.Assets {
			if strings.EqualFold(strings.TrimSpace(asset.Name), "checksums.txt") {
				checksumsURL = asset.URL
				break
			}
		}
		checksums, err := fetchChecksums(client, checksumsURL)
		if err != nil {
			return nil, err
		}

		for _, asset := range release.Assets {
			platformKey, ok := matchUpdateAsset(spec.Edition, version, asset.Name)
			if !ok {
				continue
			}
			assetName := strings.TrimSpace(asset.Name)
			manifest.AssetNames[platformKey] = assetName
			manifest.RemoteDownloadURLs[platformKey] = asset.URL
			manifest.DownloadURLs[platformKey] = buildLocalDownloadURL(publicBasePath, spec.Edition, assetName)
			if sha := strings.TrimSpace(checksums[assetName]); sha != "" {
				manifest.SHA256[platformKey] = sha
			}
			manifest.LocalPaths[platformKey] = AssetPath(dataDir, spec, assetName)
		}
		return manifest, nil
	}

	return nil, fmt.Errorf("未找到 %s 版本发布", spec.Edition)
}

func loadCachedManifest(dataDir string, spec EditionSpec, publicBasePath string, allowStale bool) (*Manifest, bool) {
	path := ManifestPath(dataDir, spec)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false
	}
	if !allowStale {
		if fetchedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(manifest.FetchedAt)); err != nil || time.Since(fetchedAt) > cacheTTL {
			return nil, false
		}
	}
	rehydrateManifest(dataDir, spec, &manifest, publicBasePath)
	return &manifest, true
}

func writeCachedManifest(dataDir string, spec EditionSpec, manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("更新元数据为空")
	}
	if err := os.MkdirAll(rootDir(dataDir, spec), 0o755); err != nil {
		return err
	}
	cached := *manifest
	cached.LocalPaths = nil
	data, err := json.MarshalIndent(&cached, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ManifestPath(dataDir, spec), data, 0o644)
}

func rehydrateManifest(dataDir string, spec EditionSpec, manifest *Manifest, publicBasePath string) {
	if manifest.DownloadURLs == nil {
		manifest.DownloadURLs = map[string]string{}
	}
	if manifest.SHA256 == nil {
		manifest.SHA256 = map[string]string{}
	}
	if manifest.RemoteDownloadURLs == nil {
		manifest.RemoteDownloadURLs = map[string]string{}
	}
	if manifest.AssetNames == nil {
		manifest.AssetNames = map[string]string{}
	}
	manifest.LocalPaths = map[string]string{}
	for platformKey, assetName := range manifest.AssetNames {
		manifest.LocalPaths[platformKey] = AssetPath(dataDir, spec, assetName)
		if strings.TrimSpace(publicBasePath) != "" {
			manifest.DownloadURLs[platformKey] = buildLocalDownloadURL(publicBasePath, spec.Edition, assetName)
		}
	}
}

func fetchChecksums(client *http.Client, checksumsURL string) (map[string]string, error) {
	if strings.TrimSpace(checksumsURL) == "" {
		return nil, fmt.Errorf("当前发布缺少 checksums.txt，无法安全更新")
	}
	resp, err := client.Get(checksumsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums.txt 返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseChecksums(string(body)), nil
}

func parseChecksums(raw string) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 2 {
			continue
		}
		filename := strings.TrimLeft(strings.TrimSpace(fields[len(fields)-1]), "*")
		sum := strings.TrimSpace(fields[0])
		if filename != "" && sum != "" {
			out[filename] = sum
		}
	}
	return out
}

func matchUpdateAsset(edition, version, assetName string) (string, bool) {
	assetName = strings.TrimSpace(assetName)
	for _, platformKey := range []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64"} {
		if assetName == updateAssetName(edition, version, platformKey) {
			return platformKey, true
		}
	}
	return "", false
}

func updateAssetName(edition, version, platformKey string) string {
	prefix := "clawpanel"
	if strings.EqualFold(strings.TrimSpace(edition), "lite") {
		suffix := strings.ReplaceAll(platformKey, "_", "-")
		if strings.TrimSpace(suffix) == "" {
			return ""
		}
		return fmt.Sprintf("clawpanel-lite-core-v%s-%s.tar.gz", version, suffix)
	}
	name := fmt.Sprintf("%s-v%s-%s", prefix, version, strings.ReplaceAll(platformKey, "_", "-"))
	if strings.HasPrefix(platformKey, "windows_") {
		name += ".exe"
	}
	return name
}

func newHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyFunc()
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func proxyFunc() func(*http.Request) (*url.URL, error) {
	if raw := strings.TrimSpace(os.Getenv("CLAWPANEL_UPDATE_PROXY")); raw != "" {
		parsed, err := url.Parse(raw)
		if err == nil {
			return http.ProxyURL(parsed)
		}
	}
	return http.ProxyFromEnvironment
}

func downloadToFile(rawURL, dest string) error {
	client := newHTTPClient(10 * time.Minute)
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回 HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func verifyExistingFile(path, expectedSHA string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	actualSHA, err := fileSHA256(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(actualSHA, expectedSHA), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func buildLocalDownloadURL(publicBasePath, edition, assetName string) string {
	base := strings.TrimRight(strings.TrimSpace(publicBasePath), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/files/%s", base, strings.TrimSpace(edition), url.PathEscape(filepath.Base(assetName)))
}

func prefixSHA(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 16 {
		return raw[:16]
	}
	return raw
}
