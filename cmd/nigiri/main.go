package main

import (
	"fmt"
	"os"

	"github.com/oota-sushikuitee/nigiri/pkg/commands"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
