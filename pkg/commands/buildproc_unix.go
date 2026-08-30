//go:build !windows

package commands

import (
	"os/exec"
	"syscall"
	"time"
)

// buildShell returns the shell binary and the arguments that precede the build
// command string on this platform.
//
// Returns:
//   - string: The shell to execute
//   - []string: The arguments to pass before the command string
func buildShell() (string, []string) {
	return "/bin/sh", []string{"-c"}
}

// configureBuildProcess puts the build shell in its own process group and kills
// that whole group on cancellation, so a --timeout also stops the compilers and
// child processes the shell spawned instead of orphaning them.
//
// Parameters:
//   - cmd: The command to configure (must not be started yet)
//   - waitDelay: The grace period before abandoning blocked output pipes
func configureBuildProcess(cmd *exec.Cmd, waitDelay time.Duration) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// A negative pid signals the whole process group created by Setpgid.
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = waitDelay
}
