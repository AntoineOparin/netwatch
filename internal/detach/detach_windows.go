//go:build windows

package detach

import "syscall"

const detachedProcess = 0x00000008 // DETACHED_PROCESS: no console window

func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
