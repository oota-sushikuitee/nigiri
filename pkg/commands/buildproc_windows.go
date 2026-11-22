//go:build windows

package commands

import (
	"os"
	"os/exec"
	"time"
)

// buildShell uses the command interpreter named by ComSpec, since Windows has
// no /bin/sh.
func buildShell() (string, []string) {
	if comSpec := os.Getenv("ComSpec"); comSpec != "" {
		return comSpec, []string{"/C"}
	}
	return "cmd.exe", []string{"/C"}
}

// configureBuildProcess only sets the wait delay: Windows has no POSIX process
// group to signal.
func configureBuildProcess(cmd *exec.Cmd, waitDelay time.Duration) {
	cmd.WaitDelay = waitDelay
}
