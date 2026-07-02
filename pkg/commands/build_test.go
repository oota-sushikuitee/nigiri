package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
