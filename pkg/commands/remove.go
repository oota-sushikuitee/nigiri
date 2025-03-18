package commands

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
	"github.com/spf13/cobra"
)

// removeCommand represents the structure for the remove command
type removeCommand struct {
	cmd *cobra.Command
	all bool
}

// newRemoveCommand creates a new remove command instance which allows users
// to remove a specified target from the nigiri root directory.
//
// Returns:
//   - *removeCommand: A configured remove command instance
func newRemoveCommand() *removeCommand {
	c := &removeCommand{}
	cmd := &cobra.Command{
		Use:   "remove target [commit]",
		Short: "Remove a target or specific commit build",
		Long: `Remove a target or a specific commit build of a target.
If commit is specified, only that commit build is removed.
If --all flag is provided, all targets will be removed.
If no commit is specified, the entire target and all its builds will be removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if c.all {
				// If --all flag is provided, remove all targets
				if len(args) > 0 {
					return logger.CreateErrorf("cannot specify a target with --all flag")
				}
				return c.executeRemoveAll()
			}

			if len(args) == 0 {
				return cmd.Help()
			}

			target := args[0]

			if len(args) > 1 {
				// If commit is specified, remove only that commit
				commitHash := args[1]
				return c.executeRemoveCommit(target, commitHash)
			}

			// Otherwise, remove the entire target
			return c.executeRemove(target)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Offer tab completion for targets if no arguments provided yet
			if len(args) == 0 {
				return c.getCompletionTargets(toComplete), cobra.ShellCompDirectiveNoFileComp
			}

			// If we already have a target, offer commit hash completions
			if len(args) == 1 {
				return c.getCompletionCommits(args[0], toComplete), cobra.ShellCompDirectiveNoFileComp
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&c.all, "all", false, "Remove all targets")

	c.cmd = cmd
	return c
}

// getCompletionTargets returns a list of available targets for command completion
func (c *removeCommand) getCompletionTargets(prefix string) []string {
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		return nil
	}

	var targets []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && strings.HasPrefix(entry.Name(), prefix) {
			targets = append(targets, entry.Name())
		}
	}
	return targets
}

// getCompletionCommits returns a list of available commit hashes for the specified target
func (c *removeCommand) getCompletionCommits(target, prefix string) []string {
	fsTarget := targets.Target{Target: target}
	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return nil
	}

	dirs, err := os.ReadDir(targetRootDir)
	if err != nil {
		return nil
	}

	var commits []string
	for _, dir := range dirs {
		if dir.IsDir() && strings.HasPrefix(dir.Name(), prefix) {
			commits = append(commits, dir.Name())
		}
	}
	return commits
}

// executeRemove handles the removal of the specified target from the nigiri root directory.
// It deletes the target's root directory and all its contents.
//
// Parameters:
//   - target: The name of the target to remove
//
// Returns:
//   - error: Any error encountered during the removal process
func (c *removeCommand) executeRemove(target string) error {
	t := targets.Target{Target: target}
	targetRootDir, err := t.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return logger.CreateErrorf("target '%s' not found", target)
	}

	// Ask for confirmation before removing the entire target
	c.cmd.Printf("This will remove the target '%s' and all its builds. Continue? (y/n): ", target)
	var confirm string
	if err := logger.ReadInput(&confirm); err != nil {
		return logger.CreateErrorf("failed to read confirmation: %w", err)
	}

	if strings.ToLower(confirm) != "y" {
		c.cmd.Println("Operation cancelled.")
		return nil
	}

	if err := os.RemoveAll(targetRootDir); err != nil {
		return logger.CreateErrorf("failed to remove target '%s': %w", target, err)
	}

	c.cmd.Printf("Target '%s' removed successfully.\n", target)
	return nil
}

// executeRemoveCommit handles the removal of a specific commit build for a target.
//
// Parameters:
//   - target: The name of the target
//   - commitHash: The commit hash to remove
//
// Returns:
//   - error: Any error encountered during the removal process
func (c *removeCommand) executeRemoveCommit(target, commitHash string) error {
	t := targets.Target{Target: target}
	targetRootDir, err := t.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return logger.CreateErrorf("target '%s' not found", target)
	}

	// Check if commit hash is valid
	if len(commitHash) < 7 {
		return logger.CreateErrorf("commit hash is too short: %s (minimum 7 characters)", commitHash)
	}

	// Find directories that match the commit hash prefix
	dirs, err := os.ReadDir(targetRootDir)
	if err != nil {
		return logger.CreateErrorf("failed to read target directory: %w", err)
	}

	var matchingDirs []string
	for _, dir := range dirs {
		if dir.IsDir() && strings.HasPrefix(dir.Name(), commitHash) {
			matchingDirs = append(matchingDirs, dir.Name())
		}
	}

	if len(matchingDirs) == 0 {
		return logger.CreateErrorf("no builds found for commit %s", commitHash)
	}

	if len(matchingDirs) > 1 {
		c.cmd.Println("Multiple commits match the provided hash:")
		for i, dir := range matchingDirs {
			c.cmd.Printf("%d. %s\n", i+1, dir)
		}
		return logger.CreateErrorf("please provide a more specific commit hash")
	}

	// Found exactly one matching commit
	fullCommitHash := matchingDirs[0]
	commitDir := filepath.Join(targetRootDir, fullCommitHash)

	// Ask for confirmation
	c.cmd.Printf("Remove build for commit %s? (y/n): ", fullCommitHash)
	var confirm string
	if err := logger.ReadInput(&confirm); err != nil {
		return logger.CreateErrorf("failed to read confirmation: %w", err)
	}

	if strings.ToLower(confirm) != "y" {
		c.cmd.Println("Operation cancelled.")
		return nil
	}

	if err := os.RemoveAll(commitDir); err != nil {
		return logger.CreateErrorf("failed to remove commit build: %w", err)
	}

	c.cmd.Printf("Build for commit %s of target '%s' removed successfully.\n", fullCommitHash, target)
	return nil
}

// executeRemoveAll handles the removal of all targets from the nigiri root directory.
//
// Returns:
//   - error: Any error encountered during the removal process
func (c *removeCommand) executeRemoveAll() error {
	// Ask for confirmation before removing all targets
	c.cmd.Print("This will remove ALL targets and ALL builds. This cannot be undone. Continue? (y/n): ")
	var confirm string
	if err := logger.ReadInput(&confirm); err != nil {
		return logger.CreateErrorf("failed to read confirmation: %w", err)
	}

	if strings.ToLower(confirm) != "y" {
		c.cmd.Println("Operation cancelled.")
		return nil
	}

	// List all directories in nigiri root
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		if os.IsNotExist(err) {
			c.cmd.Println("No targets to remove.")
			return nil
		}
		return logger.CreateErrorf("failed to read nigiri root directory: %w", err)
	}

	removedCount := 0
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			targetPath := filepath.Join(nigiriRoot, entry.Name())
			if err := os.RemoveAll(targetPath); err != nil {
				c.cmd.Printf("Warning: Failed to remove target '%s': %v\n", entry.Name(), err)
				continue
			}
			removedCount++
		}
	}

	c.cmd.Printf("%d targets removed successfully.\n", removedCount)
	return nil
}
