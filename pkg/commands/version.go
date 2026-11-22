package commands

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

// Build information, set at link time with -ldflags "-X".
var (
	// Version is the release version, or "dev" for a source build.
	Version = "dev"

	// Commit is the git commit the binary was built from.
	Commit = "none"

	// BuildDate is when the binary was built.
	BuildDate = "unknown"
)

type versionCommand struct {
	cmd *cobra.Command
}

func newVersionCommand() *versionCommand {
	c := &versionCommand{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Print detailed version information about the nigiri CLI.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.executeVersion()
		},
	}
	c.cmd = cmd

	return c
}

func (c *versionCommand) executeVersion() error {
	fmt.Fprintln(c.cmd.OutOrStdout(), "nigiri version information:")
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Version:    %s\n", Version)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Commit:     %s\n", Commit)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Built:      %s\n", BuildDate)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Go version: %s\n", runtime.Version())
	fmt.Fprintf(c.cmd.OutOrStdout(), "  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Root dir:   %s\n", nigiriRoot)
	fmt.Fprintf(c.cmd.OutOrStdout(), "  Current time: %s\n", time.Now().Format(time.RFC3339))

	return nil
}
