//go:build unix

package process

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func (m *Manager) applyOpenClawRunAs(cmd *exec.Cmd) error {
	if cmd == nil || os.Geteuid() != 0 {
		return nil
	}

	uid, gid, username, home, source, err := m.resolveOpenClawRunUser()
	if err != nil {
		return err
	}
	if uid == 0 {
		return nil
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}
	if username != "" {
		cmd.Env = upsertEnv(cmd.Env, "USER", username)
		cmd.Env = upsertEnv(cmd.Env, "LOGNAME", username)
	}
	if home != "" {
		cmd.Env = upsertEnv(cmd.Env, "HOME", home)
	}

	displayUser := formatRunUser(username, uid)
	if source != "" {
		log.Printf("[ProcessMgr] OpenClaw 将以用户 %s(uid=%d gid=%d) 运行 (%s)", displayUser, uid, gid, source)
	} else {
		log.Printf("[ProcessMgr] OpenClaw 将以用户 %s(uid=%d gid=%d) 运行", displayUser, uid, gid)
	}
	return nil
}

func (m *Manager) resolveOpenClawRunUser() (uid uint32, gid uint32, username, home, source string, err error) {
	// Explicit override from env takes highest priority.
	if raw := strings.TrimSpace(os.Getenv("OPENCLAW_RUN_AS_USER")); raw != "" {
		uid, gid, username, home, err = lookupUserIdentity(raw)
		if err != nil {
			return 0, 0, "", "", "", fmt.Errorf("OPENCLAW_RUN_AS_USER 无效: %w", err)
		}
		return uid, gid, username, home, "env:OPENCLAW_RUN_AS_USER", nil
	}

	// Default to owner of OpenClaw config directory.
	if m != nil && m.cfg != nil {
		ocDir := strings.TrimSpace(m.cfg.OpenClawDir)
		if ocDir != "" {
			if info, statErr := os.Stat(ocDir); statErr == nil {
				if st, ok := info.Sys().(*syscall.Stat_t); ok && st != nil && st.Uid != 0 {
					username, home = lookupUserMetaByID(st.Uid)
					if home == "" {
						home = filepath.Dir(ocDir)
					}
					return st.Uid, st.Gid, username, home, "owner:"+ocDir, nil
				}
			}
		}
	}

	// Secondary fallback: if launched via sudo, follow the interactive user.
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" && sudoUser != "root" {
		uid, gid, username, home, err = lookupUserIdentity(sudoUser)
		if err == nil && uid != 0 {
			return uid, gid, username, home, "env:SUDO_USER", nil
		}
	}

	return 0, 0, "", "", "", nil
}

func lookupUserIdentity(raw string) (uid uint32, gid uint32, username, home string, err error) {
	var u *user.User
	if _, parseErr := strconv.ParseUint(raw, 10, 32); parseErr == nil {
		u, err = user.LookupId(raw)
	} else {
		u, err = user.Lookup(raw)
	}
	if err != nil {
		return 0, 0, "", "", err
	}

	parsedUID, err := strconv.ParseUint(strings.TrimSpace(u.Uid), 10, 32)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("解析 uid 失败: %w", err)
	}
	parsedGID, err := strconv.ParseUint(strings.TrimSpace(u.Gid), 10, 32)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("解析 gid 失败: %w", err)
	}

	return uint32(parsedUID), uint32(parsedGID), strings.TrimSpace(u.Username), strings.TrimSpace(u.HomeDir), nil
}

func lookupUserMetaByID(uid uint32) (username, home string) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(u.Username), strings.TrimSpace(u.HomeDir)
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	updated := false
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			if !updated {
				out = append(out, prefix+value)
				updated = true
			}
			continue
		}
		out = append(out, item)
	}
	if !updated {
		out = append(out, prefix+value)
	}
	return out
}

func formatRunUser(username string, uid uint32) string {
	if strings.TrimSpace(username) != "" {
		return strings.TrimSpace(username)
	}
	return strconv.FormatUint(uint64(uid), 10)
}
