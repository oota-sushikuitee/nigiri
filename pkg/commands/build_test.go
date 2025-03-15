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
