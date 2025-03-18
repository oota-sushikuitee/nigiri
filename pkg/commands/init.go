package commands

import (
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/pkg/logger"
	"github.com/spf13/cobra"
)

// initCommand represents the structure for the init command
type initCommand struct {
	cmd *cobra.Command
}

// newInitCommand creates a new init command instance which helps users
// create their initial nigiri configuration file.
//
// Returns:
//   - *initCommand: A configured init command instance
func newInitCommand() *initCommand {
	c := &initCommand{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize nigiri configuration",
		Long:  `Create a new nigiri configuration file in the ~/.nigiri directory with default settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.executeInit()
		},
	}
	c.cmd = cmd
	return c
}

// executeInit creates a new configuration file with default settings.
//
// Returns:
//   - error: Any error encountered during the initialization process
func (c *initCommand) executeInit() error {
	// Create nigiri root directory if it doesn't exist
	if err := os.MkdirAll(nigiriRoot, 0755); err != nil {
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

	// Create a sample configuration
	sampleConfig := `# Nigiri configuration file
# Define your targets below

targets:
  # Example target
  sample-project:
    source: https://github.com/oota-sushikuitee/nigiri
    default-branch: main
    # The directory within the repository to run the build command (optional)
    working-directory: ""
    # Whether to keep only the binary and remove source code after build (optional)
    binary-only: false
    build-command:
      linux: make build
      windows: make build
      darwin: make build
      # Path to the built binary (relative to working directory or repository root)
      binary-path: bin/nigiri
    env:
      - "GO111MODULE=on"
      - "CGO_ENABLED=0"

  # You can add more targets here
  # another-project:
  #   source: https://github.com/oota-sushikuitee/nigiri
  #   default-branch: master
  #   working-directory: "cmd/app"
  #   binary-only: true
  #   build-command:
  #     linux: make linux
  #     windows: make windows
  #     darwin: make darwin
  #     binary-path: bin/app

# Default settings for all targets
defaults:
  linux: make build
  windows: make build
  darwin: make build
`

	// Write the configuration file
	if err := os.WriteFile(configFilePath, []byte(sampleConfig), 0644); err != nil {
		return logger.CreateErrorf("failed to write configuration file: %w", err)
	}

	c.cmd.Printf("Configuration file created at %s\n", configFilePath)
	c.cmd.Println("Edit this file to add your own targets.")
	c.cmd.Println("Run 'nigiri list' to see your configured targets.")

	return nil
}
