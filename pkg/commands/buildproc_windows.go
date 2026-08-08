//go:build windows

package commands

import (
	"os/exec"
	"time"
)

// configureBuildProcess bounds how long Wait blocks on output pipes. Windows
// has no POSIX process group to signal, so only the wait delay applies here.
//
// Parameters:
//   - cmd: The command to configure (must not be started yet)
//   - waitDelay: The grace period before abandoning blocked output pipes
func configureBuildProcess(cmd *exec.Cmd, waitDelay time.Duration) {
	cmd.WaitDelay = waitDelay
}
