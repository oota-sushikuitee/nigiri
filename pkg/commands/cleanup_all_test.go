package commands

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCleanupAll_Confirmation(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		stdin         string
		wantOutput    []string
		wantRemaining int
	}{
		{
			name:  "confirmed once for all targets",
			args:  []string{"--all", "--max-builds", "2"},
			stdin: "y\n",
			wantOutput: []string{
				"Cleaning up builds for 2 targets...\n", "Continue? (y/n): ", "\nProcessing target 'test-target-1':\n",
			},
			wantRemaining: 2,
		},
		{
			name:          "declined",
			args:          []string{"--all", "--max-builds", "2"},
			stdin:         "n\n",
			wantOutput:    []string{"Continue? (y/n): Cleanup cancelled.\n"},
			wantRemaining: 7,
		},
		{
			name:          "dry run does not prompt",
			args:          []string{"--all", "--dry-run", "--max-builds", "2"},
			wantOutput:    []string{"Dry run: No builds were removed."},
			wantRemaining: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			restoreCommandGlobals(t, root, "")
			setupTestTargets(t, root)

			if tt.stdin != "" {
				feedStdin(t, tt.stdin)
			}

			var out bytes.Buffer

			require.NoError(t, setupCleanupTestCommand(&out, nil, tt.args...).Execute())

			for _, want := range tt.wantOutput {
				assert.Contains(t, out.String(), want)
			}

			if tt.stdin == "" {
				assert.NotContains(t, out.String(), "Continue? (y/n)")
			}

			builds, err := filepath.Glob(filepath.Join(root, "test-target-1", "*"))
			require.NoError(t, err)
			assert.Len(t, builds, tt.wantRemaining)
		})
	}
}
