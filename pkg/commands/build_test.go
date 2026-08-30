package commands

import (
	"io"
	"path/filepath"
	"testing"

	cfgmodel "github.com/oota-sushikuitee/nigiri/internal/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuildCommand(t *testing.T) {
	cmd := newBuildCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

func TestExecuteBuild_UnknownTarget(t *testing.T) {
	testRoot(t)

	cmd := newBuildCommand()
	cmd.cmd.SetOut(io.Discard)

	err := cmd.executeBuild("not-configured")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target 'not-configured' not found in configuration")
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

func TestSelectBuildCommand(t *testing.T) {
	t.Parallel()
	target := cfgmodel.BuildCommand{Linux: "make linux", Darwin: "make darwin"}
	defaults := cfgmodel.BuildCommand{Linux: "make default-linux", Windows: "make default-windows", Darwin: "make default-darwin"}

	tests := []struct {
		name     string
		target   cfgmodel.BuildCommand
		defaults cfgmodel.BuildCommand
		goos     string
		want     string
		wantErr  string
	}{
		{name: "target command wins", target: target, defaults: defaults, goos: "linux", want: "make linux"},
		{name: "falls back to defaults", target: target, defaults: defaults, goos: "windows", want: "make default-windows"},
		{name: "no defaults configured", target: target, goos: "windows", wantErr: "no build command specified"},
		{name: "empty target and defaults", goos: "darwin", wantErr: "no build command specified"},
		{name: "unsupported os", target: target, defaults: defaults, goos: "plan9", wantErr: "unsupported OS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := selectBuildCommand(tt.target, tt.defaults, tt.goos)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
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
