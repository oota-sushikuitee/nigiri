package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewListCommand(t *testing.T) {
	cmd := newListCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

// writeCommitDir creates a build directory for target recording built as its
// build date.
func writeCommitDir(t *testing.T, target, shortHash string, built time.Time) {
	t.Helper()
	commitDir := filepath.Join(nigiriRoot, target, shortHash)
	require.NoError(t, os.MkdirAll(commitDir, 0o755))
	metadata := "Target: " + target + "\n" + buildDateField + " " + built.Format(time.RFC3339) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(commitDir, buildInfoFileName), []byte(metadata), 0o600))
}

// runList runs the list command for the given arguments and returns its output.
func runList(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newListCommand()
	cmd.cmd.SetOut(&out)

	var err error
	if len(args) == 0 {
		err = cmd.listAllTargets()
	} else {
		err = cmd.listTargetCommits(args[0])
	}
	return out.String(), err
}

func TestListAllTargets(t *testing.T) {
	root := testRoot(t)
	writeCommitDir(t, "alpha", "abcdef1", time.Now())
	writeCommitDir(t, "alpha", "1234567", time.Now())
	writeCommitDir(t, "beta", "89abcde", time.Now())
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o755))

	out, err := runList(t)
	require.NoError(t, err)
	assert.Contains(t, out, "alpha (2 commits)")
	assert.Contains(t, out, "beta (1 commits)")
	// Hidden entries hold nigiri's own state, not targets.
	assert.NotContains(t, out, ".hidden")
}

func TestListAllTargets_NoRoot(t *testing.T) {
	testRoot(t)

	out, err := runList(t)
	require.NoError(t, err)
	assert.Contains(t, out, "No targets installed.")
}

func TestListTargetCommits_NotInstalled(t *testing.T) {
	testRoot(t)

	_, err := runList(t, "sample")
	require.Error(t, err)
	// The missing target root is reported before the command's own check.
	assert.Contains(t, err.Error(), "sample")
}

// TestListTargetCommits_NewestFirst checks that commits are listed by build
// date, newest first.
func TestListTargetCommits_NewestFirst(t *testing.T) {
	testRoot(t)
	now := time.Now()
	writeCommitDir(t, "sample", "0000001", now.Add(-2*time.Hour))
	writeCommitDir(t, "sample", "0000002", now)
	writeCommitDir(t, "sample", "0000003", now.Add(-time.Hour))

	out, err := runList(t, "sample")
	require.NoError(t, err)
	assert.Less(t, strings.Index(out, "0000002"), strings.Index(out, "0000003"))
	assert.Less(t, strings.Index(out, "0000003"), strings.Index(out, "0000001"))
}
