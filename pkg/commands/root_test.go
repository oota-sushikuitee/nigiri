package commands

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	assert.NotNil(t, cmd)
	assert.IsType(t, &cobra.Command{}, cmd.cmd)
}

func TestExecute(t *testing.T) {
	cmd := NewRootCommand()
	err := cmd.Execute()
	assert.NoError(t, err)
}
