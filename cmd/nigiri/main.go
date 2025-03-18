package main

import (
	"os"

	"github.com/oota-sushikuitee/nigiri/pkg/commands"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}
