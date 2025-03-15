package commands

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// nigiriRoot is the default path for nigiri's data directory
var nigiriRoot = filepath.Join(os.Getenv("HOME"), ".nigiri")

// rootCommand represents the structure for the root command
type rootCommand struct {
	cmd *cobra.Command
	log *log.Logger
}

// NewRootCommand creates a new root command instance which serves as the base command
// for the nigiri CLI application. It initializes the command structure and adds all subcommands.
//
// Returns:
//   - *rootCommand: A configured root command instance ready to be executed
func NewRootCommand() *rootCommand {
	c := &rootCommand{}
	rootCmd := &cobra.Command{
		Use:   "nigiri",
		Short: "nigiri is a tool for managing git upstreams and build artifacts",
		Long: `nigiri is a tool for managing upstream VCS repositories and build artifacts.
It allows you to easily build, run, and manage different versions of upstream projects.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add global flags
	fs := rootCmd.PersistentFlags()
	fs.StringP("config", "c", "", "config file (default is $HOME/.nigiri/.nigiri.yml)")

	// Add subcommands
	rootCmd.AddCommand(newInitCommand().cmd)
	rootCmd.AddCommand(newBuildCommand().cmd)
	rootCmd.AddCommand(newRunCommand().cmd)
	rootCmd.AddCommand(newRemoveCommand().cmd)
	rootCmd.AddCommand(newCleanupCommand().cmd) // Add cleanup command
	rootCmd.AddCommand(newVersionCommand().cmd)
	rootCmd.AddCommand(newListCommand().cmd)

	c.cmd = rootCmd
	c.log = log.New(log.Writer(), "nigiri: ", log.LstdFlags)
	return c
}

// Execute runs the root command, processing any command line arguments
// and executing the appropriate subcommand.
//
// Returns:
//   - error: Any error encountered during command execution
func (c *rootCommand) Execute() error {
	return c.cmd.Execute()
}
