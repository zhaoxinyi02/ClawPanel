package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var versionTokenRe = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

var (
	runtimePathsByHome sync.Map // map[string][]string
)

// BuildExecEnv builds a resilient HOME/PATH execution environment so service mode
// can still discover tools installed under nvm/fnm/npm-global locations.
func BuildExecEnv() []string {
	home := runtimeHomeDir()
	path := BuildAugmentedPath(os.Getenv("PATH"))
	env := os.Environ()
	env = append(env, "HOME="+home, "PATH="+path)
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+home)
	}
	return env
}

// BuildAugmentedPath appends common runtime bin paths (nvm/fnm/etc.) to PATH.
func BuildAugmentedPath(currentPath string) string {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	var parts []string
	if currentPath != "" {
		parts = append(parts, strings.Split(currentPath, sep)...)
	}
	for _, home := range candidateHomes() {
		parts = append(parts, runtimeExtraBinPaths(home)...)
	}
	return strings.Join(dedupeNonEmpty(parts), sep)
}

// DetectOpenClawBinaryPath returns an absolute path to openclaw executable if found.
func DetectOpenClawBinaryPath() string {
	if p, err := exec.LookPath("openclaw"); err == nil && p != "" {
		return p
	}

	exeName := "openclaw"
	if runtime.GOOS == "windows" {
		exeName = "openclaw.cmd"
	}

	var candidates []string
	home := runtimeHomeDir()
	for _, h := range candidateHomes() {
		for _, p := range runtimeExtraBinPaths(h) {
			candidates = append(candidates, filepath.Join(p, exeName))
		}
	}

	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd"),
			`C:\ClawPanel\npm-global\openclaw.cmd`,
			`C:\ClawPanel\npm-global\node_modules\.bin\openclaw.cmd`,
			`C:\Program Files\nodejs\openclaw.cmd`,
		)
	} else {
		candidates = append(candidates,
			"/usr/local/bin/openclaw",
			"/usr/bin/openclaw",
			"/snap/bin/openclaw",
		)
	}

	if appDir := getNpmGlobalOpenClawDir(); appDir != "" {
		// .../lib/node_modules/openclaw -> .../bin/openclaw
		base := filepath.Dir(filepath.Dir(filepath.Dir(appDir)))
		candidates = append(candidates, filepath.Join(base, "bin", exeName))
	}

	for _, c := range dedupeNonEmpty(candidates) {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func runtimeHomeDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		if runtime.GOOS == "darwin" {
			home = "/var/root"
		} else if runtime.GOOS == "windows" {
			home = os.Getenv("USERPROFILE")
			if home == "" {
				home = `C:\Users\Administrator`
			}
		} else {
			home = "/root"
		}
	}
	return home
}

func candidateHomes() []string {
	homes := []string{runtimeHomeDir()}
	if runtime.GOOS == "windows" {
		homes = append(homes, getWindowsUserHomes()...)
		return dedupeNonEmpty(homes)
	}
	if runtimeHomeDir() != "/root" {
		homes = append(homes, "/root")
	}
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				homes = append(homes, filepath.Join("/home", e.Name()))
			}
		}
	}
	return dedupeNonEmpty(homes)
}

func runtimeExtraBinPaths(home string) []string {
	if cached, ok := runtimePathsByHome.Load(home); ok {
		if paths, ok := cached.([]string); ok {
			return paths
		}
	}
	paths := computeRuntimeExtraBinPaths(home)
	runtimePathsByHome.Store(home, paths)
	return paths
}

func computeRuntimeExtraBinPaths(home string) []string {
	return computeRuntimeExtraBinPathsForOS(runtime.GOOS, home)

}

func computeRuntimeExtraBinPathsForOS(goos, home string) []string {
	if goos == "windows" {
		paths := []string{
			filepath.Join(home, "AppData", "Roaming", "npm"),
			filepath.Join(home, "AppData", "Roaming", "nvm"),
			filepath.Join(home, "AppData", "Local", "Microsoft", "WindowsApps"),
			filepath.Join(home, "AppData", "Local", "Programs", "Python"),
			filepath.Join(home, "AppData", "Local", "pyenv", "pyenv-win", "bin"),
			filepath.Join(home, "AppData", "Local", "pyenv", "pyenv-win", "shims"),
			filepath.Join(home, "scoop", "shims"),
			filepath.Join(home, ".local", "bin"),
			`C:\Program Files\nodejs`,
			`C:\Program Files\Git\cmd`,
			`C:\Program Files\Git\bin`,
			`C:\Program Files\Git\mingw64\bin`,
			`C:\Program Files (x86)\Git\cmd`,
			`C:\Program Files (x86)\Git\bin`,
			`C:\Program Files (x86)\Git\mingw64\bin`,
			`C:\ProgramData\chocolatey\bin`,
			`C:\ClawPanel\npm-global`,
			`C:\ClawPanel\npm-global\node_modules\.bin`,
		}
		paths = append(paths, globVersionedDirs(filepath.Join(home, "AppData", "Local", "Programs", "Python", "Python*"))...)
		paths = append(paths, globVersionedDirs(filepath.Join(home, "AppData", "Local", "Programs", "Python", "Python*", "Scripts"))...)
		paths = append(paths, globVersionedDirs(filepath.Join(home, "AppData", "Roaming", "nvm", "v*"))...)
		return dedupeNonEmpty(paths)
	}

	paths := []string{
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
		"/snap/bin",
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".npm-global", "bin"),
		filepath.Join(home, ".volta", "bin"),
		filepath.Join(home, ".asdf", "shims"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".local", "share", "fnm", "current", "bin"),
		filepath.Join(home, ".fnm", "current", "bin"),
	}

	paths = append(paths, globVersionedDirs(filepath.Join(home, ".nvm", "versions", "node", "*", "bin"))...)
	paths = append(paths, globVersionedDirs(filepath.Join(home, ".local", "share", "fnm", "node-versions", "*", "installation", "bin"))...)
	paths = append(paths, globVersionedDirs(filepath.Join(home, ".fnm", "node-versions", "*", "installation", "bin"))...)

	if uid := os.Geteuid(); uid >= 0 {
		paths = append(paths, globVersionedDirs(filepath.Join("/run", "user", fmt.Sprintf("%d", uid), "fnm_multishells", "*", "bin"))...)
	}
	return dedupeNonEmpty(paths)
}

func globVersionedDirs(pattern string) []string {
	items, err := filepath.Glob(pattern)
	if err != nil || len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		ai, aj := extractVersionTuple(items[i]), extractVersionTuple(items[j])
		if ai[0] != aj[0] {
			return ai[0] > aj[0]
		}
		if ai[1] != aj[1] {
			return ai[1] > aj[1]
		}
		if ai[2] != aj[2] {
			return ai[2] > aj[2]
		}
		return items[i] > items[j]
	})
	return items
}

func extractVersionTuple(path string) [3]int {
	parts := strings.Split(path, string(os.PathSeparator))
	for _, p := range parts {
		m := versionTokenRe.FindStringSubmatch(p)
		if len(m) == 0 {
			continue
		}
		var t [3]int
		for i := 1; i <= 3; i++ {
			if i < len(m) && m[i] != "" {
				if v, err := strconv.Atoi(m[i]); err == nil {
					t[i-1] = v
				}
			}
		}
		return t
	}
	return [3]int{0, 0, 0}
}

func dedupeNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
