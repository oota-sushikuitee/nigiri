package commands

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/pkg/logger"
	"github.com/spf13/cobra"
)

//go:embed sample-config.yml
var sampleConfig []byte

type initCommand struct {
	cmd *cobra.Command
}

func newInitCommand() *initCommand {
	c := &initCommand{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize nigiri configuration",
		Long:  `Create a new nigiri configuration file in the ~/.nigiri directory with default settings.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.executeInit()
		},
	}
	c.cmd = cmd

	return c
}

func (c *initCommand) executeInit() error {
	// Create nigiri root directory if it doesn't exist
	if err := os.MkdirAll(nigiriRoot, 0o755); err != nil {
		return logger.CreateErrorf("failed to create nigiri root directory: %w", err)
	}

	// Configuration file path
	configFilePath := filepath.Join(nigiriRoot, ".nigiri.yml")

	// Check if config file already exists
	if _, err := os.Stat(configFilePath); err == nil {
		c.cmd.Printf("Configuration file already exists at %s\n", configFilePath)
		c.cmd.Print("Do you want to overwrite it? (y/n): ")

		var confirm string
		if err := logger.ReadInput(&confirm); err != nil {
			return logger.CreateErrorf("failed to read confirmation: %w", err)
		}

		if confirm != "y" && confirm != "Y" {
			c.cmd.Println("Initialization cancelled.")

			return nil
		}
	}

	// Write the configuration file
	if err := os.WriteFile(configFilePath, sampleConfig, 0o644); err != nil {
		return logger.CreateErrorf("failed to write configuration file: %w", err)
	}

	c.cmd.Printf("Configuration file created at %s\n", configFilePath)
	c.cmd.Println("Edit this file to add your own targets.")
	c.cmd.Println("Run 'nigiri build <target>' to build one, then 'nigiri list' to see your builds.")

	return nil
}
