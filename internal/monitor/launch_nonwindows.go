//go:build !windows

package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Non-Windows stub: this path is only used in Windows branch logic.
func launchNapCatInUserSession(_, _ string) error {
	return fmt.Errorf("launchNapCatInUserSession is only supported on Windows")
}

// Shared helper for scanning NapCat inner resource directory.
func findNapCatInnerDir(shellDir string) string {
	var found string
	_ = filepath.WalkDir(shellDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "napcat.mjs") {
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
