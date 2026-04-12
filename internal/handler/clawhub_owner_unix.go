//go:build !windows

package handler

import (
	"os"
	"syscall"
)

func parentOwnership(path string) (uid, gid int, ok bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, false, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false, nil
	}
	return int(stat.Uid), int(stat.Gid), true, nil
}
