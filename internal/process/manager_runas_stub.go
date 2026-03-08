//go:build !unix

package process

import "os/exec"

func (m *Manager) applyOpenClawRunAs(_ *exec.Cmd) error {
	return nil
}
