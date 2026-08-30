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

func TestParseRunArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		target     string
		commitHash string
		targetArgs []string
		configFile string
		help       bool
		wantErr    bool
	}{
		{name: "target only", args: []string{"app"}, target: "app"},
		{name: "target and commit", args: []string{"app", "abcdef1"}, target: "app", commitHash: "abcdef1"},
		{
			name: "commit and target args", args: []string{"app", "abcdef1", "run", "-v"},
			target: "app", commitHash: "abcdef1", targetArgs: []string{"run", "-v"},
		},
		{
			name: "flag right after target is a target arg", args: []string{"app", "-v"},
			target: "app", targetArgs: []string{"-v"},
		},
		{
			name: "dash separator without commit keeps preceding args", args: []string{"app", "-v", "--", "input.txt"},
			target: "app", targetArgs: []string{"-v", "input.txt"},
		},
		{
			name: "dash separator with commit keeps middle args", args: []string{"app", "abcdef1", "-v", "--", "input.txt"},
			target: "app", commitHash: "abcdef1", targetArgs: []string{"-v", "input.txt"},
		},
		{
			name: "dash separator only", args: []string{"app", "--", "-v"},
			target: "app", targetArgs: []string{"-v"},
		},
		{
			name: "config flag before target", args: []string{"--config", "my.yml", "app"},
			target: "app", configFile: "my.yml",
		},
		{
			name: "config shorthand before target", args: []string{"-c", "my.yml", "app", "abcdef1"},
			target: "app", commitHash: "abcdef1", configFile: "my.yml",
		},
		{
			name: "config flag with equals", args: []string{"--config=my.yml", "app"},
			target: "app", configFile: "my.yml",
		},
		{
			name: "config shorthand with equals", args: []string{"-c=my.yml", "app"},
			target: "app", configFile: "my.yml",
		},
		{
			name: "config flag after target belongs to the target", args: []string{"app", "-c", "my.yml"},
			target: "app", targetArgs: []string{"-c", "my.yml"},
		},
		{name: "help flag", args: []string{"--help"}, help: true},
		{name: "no arguments", args: nil},
		{name: "config flag without value", args: []string{"--config"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inv, err := parseRunArgs(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.target, inv.target)
			assert.Equal(t, tt.commitHash, inv.commitHash)
			assert.Equal(t, tt.targetArgs, inv.targetArgs)
			assert.Equal(t, tt.configFile, inv.configFile)
			assert.Equal(t, tt.help, inv.help)
		})
	}
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
