//go:build !linux

package webplayer

import (
	"os"
	"os/exec"
	"syscall"
)

// helperSysProcAttr is a no-op on non-Linux platforms. The web player runtime
// is only published for linux/amd64 today, so this exists to keep the package
// buildable and vettable everywhere.
func helperSysProcAttr() *syscall.SysProcAttr {
	return nil
}

func signalHelper(cmd *exec.Cmd, force bool) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if force {
		return cmd.Process.Kill()
	}
	return cmd.Process.Signal(os.Interrupt)
}
