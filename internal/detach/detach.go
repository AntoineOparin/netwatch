// Package detach runs the hub or agent in the background, so it keeps going
// after the terminal closes. Go can't fork, so the program relaunches itself
// without the -detach flag, in its own session, with output going to a log
// file, and the original process exits.
package detach

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Start relaunches the current program in the background and returns the
// new process's PID. The caller should exit right after.
func Start(logPath string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	logPath, err = filepath.Abs(logPath)
	if err != nil {
		return 0, err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log file: %w", err)
	}
	defer log.Close()

	cmd := exec.Command(exe, withoutDetach(os.Args[1:])...)
	cmd.Stdout, cmd.Stderr = log, log // stdin stays /dev/null
	cmd.SysProcAttr = sysProcAttr()
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start in background: %w", err)
	}
	pid := cmd.Process.Pid
	// Don't wait for it: it outlives this process.
	return pid, cmd.Process.Release()
}

// StopHint tells the user how to stop a detached process.
func StopHint(pid int) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("taskkill /PID %d", pid)
	}
	return fmt.Sprintf("kill %d", pid)
}

// withoutDetach drops the -detach flag (in any spelling the flag package
// accepts) so the background copy doesn't detach again.
func withoutDetach(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if strings.HasPrefix(a, "-") && name == "detach" && (!hasValue || value == "true" || value == "1") {
			continue
		}
		out = append(out, a)
	}
	return out
}
