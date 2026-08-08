package commands

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuildCommand(t *testing.T) {
	cmd := newBuildCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

func TestExecuteBuild(t *testing.T) {
	cmd := newBuildCommand()
	err := cmd.executeBuild("nigiri")
	assert.Error(t, err) // Expecting error due to missing config and other dependencies
}

// TestExecuteBuild_RemovesCommitDirOnFailure checks that a failed build leaves
// no commit directory behind, so the next build is not skipped as already built.
func TestExecuteBuild_RemovesCommitDirOnFailure(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeTestConfig(t, root, filepath.Join(root, "missing-repo"))
	restoreCommandGlobals(t, filepath.Join(root, "nigiri"), cfgPath)

	cmd := newBuildCommand()
	cmd.cmd.SetOut(io.Discard)
	cmd.commit = "abcdef1234567890abcdef1234567890abcdef12"

	err := cmd.executeBuild("sample")
	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(nigiriRoot, "sample", "abcdef1"))
}

func TestResolveCloneDepth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		depth  int
		commit string
		want   int
	}{
		{name: "no commit keeps default shallow depth", depth: 1, commit: "", want: 1},
		{name: "no commit keeps custom depth", depth: 5, commit: "", want: 5},
		{name: "no commit keeps full history depth", depth: 0, commit: "", want: 0},
		{name: "commit with default shallow depth forces full clone", depth: 1, commit: "abc1234", want: 0},
		{name: "commit with custom depth forces full clone", depth: 5, commit: "abc1234", want: 0},
		{name: "commit with full history depth stays full", depth: 0, commit: "abc1234", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveCloneDepth(tt.depth, tt.commit))
		})
	}
}
