package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRemoveCommand(t *testing.T) {
	cmd := newRemoveCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

func TestExecuteRemove(t *testing.T) {
	cmd := newRemoveCommand()
	err := cmd.executeRemove("nigiri")
	assert.Error(t, err) // Expecting error due to missing target directory
}
