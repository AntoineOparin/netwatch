//go:build !windows

package detach

import "syscall"

// A new session: no controlling terminal, so closing the terminal (SIGHUP)
// doesn't stop it.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
