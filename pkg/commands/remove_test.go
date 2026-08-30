package commands

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemoveCommand(t *testing.T) {
	cmd := newRemoveCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

// newTestRemoveCommand returns a remove command writing to io.Discard.
func newTestRemoveCommand() *removeCommand {
	cmd := newRemoveCommand()
	cmd.cmd.SetOut(io.Discard)
	return cmd
}

// makeBuildDirs creates the given build directories for the "sample" target and
// returns the target root.
func makeBuildDirs(t *testing.T, shortHashes ...string) string {
	t.Helper()
	targetRoot := filepath.Join(nigiriRoot, "sample")
	for _, hash := range shortHashes {
		require.NoError(t, os.MkdirAll(filepath.Join(targetRoot, hash), 0o755))
	}
	return targetRoot
}

func TestExecuteRemove_UnknownTarget(t *testing.T) {
	testRoot(t)

	err := newTestRemoveCommand().executeRemove("sample")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target 'sample' not found")
}

// TestExecuteRemove_DeclinedKeepsTarget checks that answering anything but "y"
// leaves the target directory in place.
func TestExecuteRemove_DeclinedKeepsTarget(t *testing.T) {
	testRoot(t)
	targetRoot := makeBuildDirs(t, "abcdef1")
	feedStdin(t, "n\n")

	require.NoError(t, newTestRemoveCommand().executeRemove("sample"))
	assert.DirExists(t, targetRoot)
}

func TestExecuteRemove_ConfirmedRemovesTarget(t *testing.T) {
	testRoot(t)
	targetRoot := makeBuildDirs(t, "abcdef1")
	feedStdin(t, "y\n")

	require.NoError(t, newTestRemoveCommand().executeRemove("sample"))
	assert.NoDirExists(t, targetRoot)
}

func TestExecuteRemoveCommit(t *testing.T) {
	tests := []struct {
		name       string
		builds     []string
		commitHash string
		confirm    string
		wantErr    string
		wantKept   []string
		wantGone   []string
	}{
		{
			name:       "hash shorter than a short hash is rejected",
			builds:     []string{"abcdef1"},
			commitHash: "abcde",
			wantErr:    "commit hash is too short",
			wantKept:   []string{"abcdef1"},
		},
		{
			name:       "unknown commit is reported",
			builds:     []string{"abcdef1"},
			commitHash: "9999999",
			wantErr:    "no builds found for commit",
			wantKept:   []string{"abcdef1"},
		},
		{
			name:       "ambiguous prefix removes nothing",
			builds:     []string{"abcdef1", "abcdef12"},
			commitHash: "abcdef1",
			wantErr:    "please provide a more specific commit hash",
			wantKept:   []string{"abcdef1", "abcdef12"},
		},
		{
			name:       "declined confirmation keeps the build",
			builds:     []string{"abcdef1", "1234567"},
			commitHash: "abcdef1",
			confirm:    "n\n",
			wantKept:   []string{"abcdef1", "1234567"},
		},
		{
			name:       "confirmed removal deletes only the matching build",
			builds:     []string{"abcdef1", "1234567"},
			commitHash: "abcdef1",
			confirm:    "y\n",
			wantKept:   []string{"1234567"},
			wantGone:   []string{"abcdef1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRoot(t)
			targetRoot := makeBuildDirs(t, tt.builds...)
			if tt.confirm != "" {
				feedStdin(t, tt.confirm)
			}

			err := newTestRemoveCommand().executeRemoveCommit("sample", tt.commitHash)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			for _, kept := range tt.wantKept {
				assert.DirExists(t, filepath.Join(targetRoot, kept))
			}
			for _, gone := range tt.wantGone {
				assert.NoDirExists(t, filepath.Join(targetRoot, gone))
			}
		})
	}
}

// TestExecuteRemoveAll_SkipsDotEntries checks that removing every target leaves
// hidden entries such as the configuration file and its directory alone.
func TestExecuteRemoveAll_SkipsDotEntries(t *testing.T) {
	root := testRoot(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "alpha", "abcdef1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "beta", "1234567"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".state"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".nigiri.yml"), []byte("targets:\n"), 0o600))
	feedStdin(t, "y\n")

	require.NoError(t, newTestRemoveCommand().executeRemoveAll())

	assert.NoDirExists(t, filepath.Join(root, "alpha"))
	assert.NoDirExists(t, filepath.Join(root, "beta"))
	assert.DirExists(t, filepath.Join(root, ".state"))
	assert.FileExists(t, filepath.Join(root, ".nigiri.yml"))
}

func TestExecuteRemoveAll_DeclinedKeepsTargets(t *testing.T) {
	root := testRoot(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "alpha", "abcdef1"), 0o755))
	feedStdin(t, "n\n")

	require.NoError(t, newTestRemoveCommand().executeRemoveAll())
	assert.DirExists(t, filepath.Join(root, "alpha"))
}
