//go:build windows

package handler

func parentOwnership(path string) (uid, gid int, ok bool, err error) {
	return 0, 0, false, nil
}
