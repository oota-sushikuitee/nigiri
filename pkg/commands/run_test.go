package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
