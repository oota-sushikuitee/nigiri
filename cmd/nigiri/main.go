// Command nigiri builds and runs specific commits of upstream repositories.
package main

import (
	"errors"
	"os"

	"github.com/oota-sushikuitee/nigiri/pkg/commands"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
)

func main() {
	err := commands.NewRootCommand().Execute()
	if err == nil {
		return
	}

	// A target that ran and exited non-zero already reported for itself; pass
	// its status on instead of reporting a nigiri failure.
	if exitErr, ok := errors.AsType[*commands.ExitCodeError](err); ok {
		os.Exit(exitErr.ExitCode())
	}

	logger.Error(err)
	os.Exit(1)
}
