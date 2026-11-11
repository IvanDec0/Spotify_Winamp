//go:build linux

package webplayer

import (
	"os/exec"
	"syscall"
)

// helperSysProcAttr puts the helper and every Chromium child in one process
// group, and asks the kernel to terminate it if Cadence dies without cleanup.
func helperSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
}

// signalHelper sends a signal to the helper's whole process group so
// Chromium's renderer and utility processes cannot outlive it.
func signalHelper(cmd *exec.Cmd, force bool) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	return syscall.Kill(-cmd.Process.Pid, signal)
}
