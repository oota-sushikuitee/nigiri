package commands

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

// Version information (embedded at build time)
var (
	// Version is the current version of the nigiri CLI
	Version = "dev"

	// Commit is the git commit hash from which the binary was built
	Commit = "none"

	// BuildDate is the date and time when the binary was built
	BuildDate = "unknown"
)

// versionCommand represents the structure for the version command
type versionCommand struct {
	cmd *cobra.Command
}

// newVersionCommand creates a new version command instance which displays
// detailed version information about the nigiri CLI.
//
// Returns:
//   - *versionCommand: A configured version command instance
func newVersionCommand() *versionCommand {
	c := &versionCommand{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Print detailed version information about the nigiri CLI.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.executeVersion()
		},
	}
	c.cmd = cmd
	return c
}

// executeVersion displays detailed version information about the nigiri CLI,
// including the version number, commit hash, build date, Go version, and system information.
//
// Returns:
//   - error: Any error encountered during the execution of the command
func (c *versionCommand) executeVersion() error {
	fmt.Fprintln(c.cmd.OutOrStdout(), "nigiri version information:")
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Version:    %s\n", Version)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Commit:     %s\n", Commit)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Built:      %s\n", BuildDate)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Go version: %s\n", runtime.Version())
	fmt.Fprintf(c.cmd.OutOrStdout(), "  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	// Current configuration directory information
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Root dir:   %s\n", nigiriRoot)
	// Display current time
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Current time: %s\n", time.Now().Format(time.RFC3339))
	return nil
}
