package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVersionCommand(t *testing.T) {
	cmd := newVersionCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.cmd)
}

func TestExecuteVersion(t *testing.T) {
	cmd := newVersionCommand()
	err := cmd.executeVersion()
	assert.NoError(t, err)
}
