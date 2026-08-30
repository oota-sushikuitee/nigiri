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
	var exitErr *commands.ExitCodeError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}

	logger.Error(err)
	os.Exit(1)
}
