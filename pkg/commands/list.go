package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/config"
	"github.com/spf13/cobra"
)

// listCommand represents the structure for the list command
type listCommand struct {
	cmd *cobra.Command
}

// newListCommand creates a new list command instance which allows users
// to view installed targets and their commits. It can list all targets
// or provide detailed information about the commits for a specific target.
//
// Returns:
//   - *listCommand: A configured list command instance
func newListCommand() *listCommand {
	c := &listCommand{}
	cmd := &cobra.Command{
		Use:   "list [target]",
		Short: "List installed targets and commits",
		Long:  `List all installed targets and their commits, or list commits for a specific target.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.listAllTargets()
			}
			return c.listTargetCommits(args[0])
		},
	}
	c.cmd = cmd
	return c
}

// listAllTargets lists all installed targets and the number of commits for each.
// It reads the nigiri root directory and displays a summary of all available targets.
//
// Returns:
//   - error: Any error encountered while reading the directory or target information
func (c *listCommand) listAllTargets() error {
	// Examine the contents of the .nigiri directory
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		if os.IsNotExist(err) {
			c.cmd.Println("No targets installed.")
			return nil
		}
		return fmt.Errorf("failed to read nigiri root directory: %w", err)
	}

	if len(entries) == 0 {
		c.cmd.Println("No targets installed.")
		return nil
	}

	// Display each target directory
	c.cmd.Println("Installed targets:")
	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[0] != '.' {
			targetName := entry.Name()
			// Count the number of commits
			targetDir := filepath.Join(nigiriRoot, targetName)
			commits, err := os.ReadDir(targetDir)
			if err != nil {
				continue
			}
			commitCount := 0
			for _, commit := range commits {
				if commit.IsDir() {
					commitCount++
				}
			}
			c.cmd.Printf("  %s (%d commits)\n", targetName, commitCount)
		}
	}

	c.cmd.Println("\nUse 'nigiri list <target>' to see commits for a specific target.")
	return nil
}

// commitInfo represents information about a commit, optimized for memory layout
type commitInfo struct {
	modTime time.Time // 24 bytes
	hash    string    // 16 bytes (pointer + length)
}

// listTargetCommits lists all commits for a specified target, sorted by build time.
// It displays configuration information for the target if available, followed by a list
// of commit hashes with their build timestamps.
//
// Parameters:
//   - target: The name of the target whose commits should be listed
//
// Returns:
//   - error: Any error encountered while reading the target directory or commit information
func (c *listCommand) listTargetCommits(target string) error {
	// Create Target instance
	fsTarget := targets.Target{
		Target:  target,
		Commits: commits.Commits{},
	}
	targetDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return err
	}

	// Check if target directory exists
	if _, statErr := os.Stat(targetDir); os.IsNotExist(statErr) {
		return fmt.Errorf("target '%s' is not installed", target)
	}

	// Get commit directories
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("failed to read target directory: %w", err)
	}

	if len(entries) == 0 {
		c.cmd.Printf("No commits found for target '%s'.\n", target)
		return nil
	}

	// Collect commit information and sort by time
	var commits []commitInfo
	for _, entry := range entries {
		if entry.IsDir() {
			commitDir := filepath.Join(targetDir, entry.Name())
			info, err := os.Stat(commitDir)
			if err != nil {
				continue
			}
			commits = append(commits, commitInfo{
				hash:    entry.Name(),
				modTime: info.ModTime(),
			})
		}
	}

	// Sort by build time (newest first)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].modTime.After(commits[j].modTime)
	})

	// Get configuration information
	cm := config.NewConfigManager()
	if err := cm.LoadCfgFile(); err == nil {
		if targetCfg, ok := cm.Config.Targets[target]; ok {
			c.cmd.Printf("Target: %s\n", target)
			c.cmd.Printf("Source: %s\n", targetCfg.Sources)
			c.cmd.Printf("Default branch: %s\n", targetCfg.DefaultBranch)
		}
	}

	c.cmd.Printf("\nCommits for target '%s' (newest first):\n", target)
	for i, commit := range commits {
		c.cmd.Printf("  %d. %s (built on %s)\n", i+1, commit.hash, commit.modTime.Format("2006-01-02 15:04:05"))
	}

	c.cmd.Println("\nUse 'nigiri run " + target + " <commit>' to run a specific commit.")
	return nil
}
