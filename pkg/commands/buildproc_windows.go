//go:build windows

package commands

import (
	"os"
	"os/exec"
	"time"
)

// buildShell returns the shell binary and the arguments that precede the build
// command string on this platform. Windows has no /bin/sh, so the command
// interpreter named by ComSpec is used, falling back to cmd.exe.
//
// Returns:
//   - string: The shell to execute
//   - []string: The arguments to pass before the command string
func buildShell() (string, []string) {
	if comSpec := os.Getenv("ComSpec"); comSpec != "" {
		return comSpec, []string{"/C"}
	}
	return "cmd.exe", []string{"/C"}
}

// configureBuildProcess bounds how long Wait blocks on output pipes. Windows
// has no POSIX process group to signal, so only the wait delay applies here.
//
// Parameters:
//   - cmd: The command to configure (must not be started yet)
//   - waitDelay: The grace period before abandoning blocked output pipes
func configureBuildProcess(cmd *exec.Cmd, waitDelay time.Duration) {
	cmd.WaitDelay = waitDelay
}
