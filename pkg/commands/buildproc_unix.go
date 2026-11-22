//go:build !windows

package commands

import (
	"os/exec"
	"syscall"
	"time"
)

func buildShell() (string, []string) {
	return "/bin/sh", []string{"-c"}
}

// configureBuildProcess kills the shell's whole process group on cancellation,
// so a --timeout also stops the compilers and child processes the shell
// spawned instead of orphaning them.
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
