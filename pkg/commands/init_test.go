package commands

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInitCommand(t *testing.T) {
	cmd := newInitCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

// initTestGlobals points the package globals at a fresh nigiri root and returns
// the path the generated configuration file will have.
func initTestGlobals(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "nigiri")
	cfgPath := filepath.Join(root, ".nigiri.yml")
	restoreCommandGlobals(t, root, cfgPath)
	return cfgPath
}

// TestExecuteInit_WritesLoadableConfig checks that the generated sample config
// parses back through the loader, so the template and the parser cannot drift.
func TestExecuteInit_WritesLoadableConfig(t *testing.T) {
	cfgPath := initTestGlobals(t)

	cmd := newInitCommand()
	cmd.cmd.SetOut(io.Discard)
	require.NoError(t, cmd.executeInit())
	require.FileExists(t, cfgPath)

	cm := newConfigManager()
	require.NoError(t, cm.LoadCfgFile())

	target, ok := cm.Config.GetTarget("sample-project")
	require.True(t, ok)
	assert.Equal(t, "https://github.com/oota-sushikuitee/nigiri", target.Sources)
	assert.Equal(t, "main", target.DefaultBranch)
	assert.Equal(t, "make build", target.BuildCommand.Linux)
	assert.Equal(t, "make build", target.BuildCommand.Darwin)
	assert.Equal(t, "make build", target.BuildCommand.Windows)
	assert.Equal(t, []string{"GO111MODULE=on", "CGO_ENABLED=0"}, target.Env)
	assert.False(t, target.BinaryOnly)

	binaryPath, hasBinaryPath := target.BuildCommand.BinaryPath()
	require.True(t, hasBinaryPath)
	assert.Equal(t, "bin/nigiri", binaryPath)

	// The generated defaults must reach the loader as build commands.
	assert.Equal(t, "make build", cm.Config.Defaults.Linux)
}

func TestExecuteInit_OverwritePrompt(t *testing.T) {
	tests := []struct {
		name         string
		confirm      string
		wantOverwrit bool
	}{
		{name: "declining keeps the existing config", confirm: "n\n"},
		{name: "confirming replaces the config", confirm: "y\n", wantOverwrit: true},
	}

	const existing = "targets:\n  mine:\n    source: https://example.com/repo\n"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := initTestGlobals(t)
			require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
			require.NoError(t, os.WriteFile(cfgPath, []byte(existing), 0o600))
			feedStdin(t, tt.confirm)

			cmd := newInitCommand()
			cmd.cmd.SetOut(io.Discard)
			require.NoError(t, cmd.executeInit())

			content, err := os.ReadFile(cfgPath)
			require.NoError(t, err)
			if tt.wantOverwrit {
				assert.Contains(t, string(content), "sample-project")
			} else {
				assert.Equal(t, existing, string(content))
			}
		})
	}
}
