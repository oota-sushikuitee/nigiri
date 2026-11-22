// Package commands implements the nigiri CLI commands.
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/pkg/config"
	"github.com/spf13/cobra"
)

var nigiriRoot = defaultNigiriRoot()

var cfgFileFlag string

// defaultNigiriRoot uses the same home directory resolution as the config
// loader, so both agree across platforms (os.UserHomeDir works on Windows,
// where HOME is usually unset).
func defaultNigiriRoot() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".nigiri"
	}

	return filepath.Join(homeDir, ".nigiri")
}

func newConfigManager() *config.ConfigManager {
	cm := config.NewConfigManager()
	if cfgFileFlag != "" {
		cm.Config.SetCfgFile(cfgFileFlag)
	}

	return cm
}

// ExitCodeError reports the non-zero exit status of a target process that
// nigiri ran, so the CLI can exit with the same status instead of 1.
type ExitCodeError struct {
	Code int
}

// Error implements the error interface.
func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("target exited with status %d", e.Code)
}

// ExitCode returns the status the CLI should exit with.
func (e *ExitCodeError) ExitCode() int {
	return e.Code
}

type rootCommand struct {
	cmd *cobra.Command
}

// NewRootCommand returns the nigiri command tree with every subcommand
// registered; call Execute on it to run the CLI.
func NewRootCommand() *rootCommand {
	c := &rootCommand{}
	rootCmd := &cobra.Command{
		Use:   "nigiri",
		Short: "nigiri is a tool for managing git upstreams and build artifacts",
		Long: `nigiri is a tool for managing upstream VCS repositories and build artifacts.
It allows you to easily build, run, and manage different versions of upstream projects.
`,
		Version: Version,
		// main reports errors itself; a failing command is not a usage error,
		// so the usage text must not be dumped on top of it.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	fs := rootCmd.PersistentFlags()
	fs.StringVarP(&cfgFileFlag, "config", "c", "", "config file (default is $HOME/.nigiri/.nigiri.yml)")

	rootCmd.AddCommand(newInitCommand().cmd)
	rootCmd.AddCommand(newBuildCommand().cmd)
	rootCmd.AddCommand(newRunCommand().cmd)
	rootCmd.AddCommand(newRemoveCommand().cmd)
	rootCmd.AddCommand(newCleanupCommand().cmd)
	rootCmd.AddCommand(newVersionCommand().cmd)
	rootCmd.AddCommand(newListCommand().cmd)

	c.cmd = rootCmd

	return c
}

// Execute parses os.Args and runs the selected subcommand. A target that exits
// non-zero under `nigiri run` is reported as an *ExitCodeError.
func (c *rootCommand) Execute() error {
	return c.cmd.Execute() //nolint:wrapcheck // subcommand errors already carry their context
}
