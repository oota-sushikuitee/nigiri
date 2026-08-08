//go:build !windows

package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipedWriter returns a writer that is not an *os.File, which makes os/exec
// pipe the command output through a copy goroutine (as verbose builds do).
func pipedWriter() io.Writer {
	return io.MultiWriter(io.Discard)
}

func TestRunBuildCommand_Success(t *testing.T) {
	err := runBuildCommand(context.Background(), "exit 0", pipedWriter(), pipedWriter(), nil, time.Second)
	assert.NoError(t, err)
}

func TestRunBuildCommand_ReportsFailure(t *testing.T) {
	err := runBuildCommand(context.Background(), "exit 3", pipedWriter(), pipedWriter(), nil, time.Second)
	assert.Error(t, err)
}

// TestRunBuildCommand_TimeoutKillsProcessGroup checks that cancelling the build
// context stops the processes the build shell spawned, not just the shell.
func TestRunBuildCommand_TimeoutKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := runBuildCommand(ctx, "sleep 60 & echo $! > "+pidFile+"; wait", pipedWriter(), pipedWriter(), nil, time.Second)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, ctx.Err())
	assert.Less(t, elapsed, 30*time.Second)

	pid := readChildPID(t, pidFile)
	assert.Eventually(t, func() bool {
		return syscall.Kill(pid, syscall.Signal(0)) != nil
	}, 10*time.Second, 100*time.Millisecond, "grandchild process %d survived the build timeout", pid)
}

// TestRunBuildCommand_WaitDelayBoundsOrphanedOutput checks that a background
// process holding the inherited output pipe cannot make the build hang.
func TestRunBuildCommand_WaitDelayBoundsOrphanedOutput(t *testing.T) {
	start := time.Now()
	err := runBuildCommand(context.Background(), "sleep 60 &", pipedWriter(), pipedWriter(), nil, 500*time.Millisecond)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 30*time.Second)
}

// readChildPID reads the pid the build shell recorded for its background child.
func readChildPID(t *testing.T, pidFile string) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil && pid > 0
	}, 10*time.Second, 100*time.Millisecond, "build shell never recorded its child pid")
	return pid
}
