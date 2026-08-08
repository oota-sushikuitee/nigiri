package commands

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunCommand(t *testing.T) {
	cmd := newRunCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

func TestExecuteRun(t *testing.T) {
	cmd := newRunCommand()
	err := cmd.executeRun("nigiri", "", nil)
	assert.Error(t, err) // Expecting error due to missing config and other dependencies
}

func TestCommitDirMatches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		dirName    string
		commitHash string
		want       bool
	}{
		{name: "full hash matches short directory name", dirName: "abcdef1", commitHash: "abcdef1234567890abcdef1234567890abcdef12", want: true},
		{name: "short hash matches directory name", dirName: "abcdef1", commitHash: "abcdef1", want: true},
		{name: "longer directory name matches shorter query", dirName: "abcdef1234", commitHash: "abcdef1", want: true},
		{name: "different hash does not match", dirName: "abcdef1", commitHash: "1234567890abcdef1234567890abcdef12345678", want: false},
		{name: "full hash differing after the short prefix does not match", dirName: "abcdef1", commitHash: "abcdef0234567890abcdef1234567890abcdef12", want: false},
		{name: "empty directory name does not match", dirName: "", commitHash: "abcdef1", want: false},
		{name: "empty hash does not match", dirName: "abcdef1", commitHash: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, commitDirMatches(tt.dirName, tt.commitHash))
		})
	}
}

// TestExecuteRun_ResolvesFullCommitHash checks that a full-length hash resolves
// to the build directory named after its short hash.
func TestExecuteRun_ResolvesFullCommitHash(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeTestConfig(t, root, filepath.Join(root, "missing-repo"))

	restoreCommandGlobals(t, filepath.Join(root, "nigiri"), cfgPath)
	require.NoError(t, os.MkdirAll(filepath.Join(nigiriRoot, "sample", "abcdef1"), 0o755))

	cmd := newRunCommand()
	cmd.cmd.SetOut(io.Discard)

	err := cmd.executeRun("sample", "abcdef1234567890abcdef1234567890abcdef12", nil)
	require.Error(t, err)
	// The build directory is found; only its (absent) contents make this fail.
	assert.NotContains(t, err.Error(), "no build found for commit")
	assert.Contains(t, err.Error(), "source directory not found")
}

func TestExecuteRun_UnknownCommitIsReported(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeTestConfig(t, root, filepath.Join(root, "missing-repo"))

	restoreCommandGlobals(t, filepath.Join(root, "nigiri"), cfgPath)
	require.NoError(t, os.MkdirAll(filepath.Join(nigiriRoot, "sample", "abcdef1"), 0o755))

	cmd := newRunCommand()
	cmd.cmd.SetOut(io.Discard)

	err := cmd.executeRun("sample", "9999999999999999999999999999999999999999", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no build found for commit")
}
