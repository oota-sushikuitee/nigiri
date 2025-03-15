package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oota-sushikuitee/nigiri/internal/dirutils"
	"github.com/oota-sushikuitee/nigiri/pkg/fsutils"
	"github.com/spf13/cobra"
)

// cleanupCommand represents the structure for the cleanup command
type cleanupCommand struct {
	cmd         *cobra.Command
	maxAge      int
	maxBuilds   int
	dryRun      bool
	allTargets  bool
	skipConfirm bool
}

// newCleanupCommand creates a new cleanup command instance which helps users
// manage disk space by cleaning up old builds.
//
// Returns:
//   - *cleanupCommand: A configured cleanup command instance
func newCleanupCommand() *cleanupCommand {
	c := &cleanupCommand{}
	cmd := &cobra.Command{
		Use:   "cleanup [target]",
		Short: "Clean up old builds",
		Long: `Clean up old builds to manage disk space.
If a target is specified, only that target's builds will be cleaned up.
Without arguments, shows the current disk usage of builds.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if c.allTargets {
					return c.executeCleanupAll()
				}
				return c.showDiskUsage()
			}
			return c.executeCleanup(args[0])
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Offer tab completion for targets if no arguments provided yet
			if len(args) == 0 {
				return c.getCompletionTargets(toComplete), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	flags := cmd.Flags()
	flags.IntVarP(&c.maxAge, "max-age", "a", 30, "Maximum age of builds to keep in days (0 to disable)")
	flags.IntVarP(&c.maxBuilds, "max-builds", "b", 5, "Maximum number of builds to keep per target (0 to disable)")
	flags.BoolVarP(&c.dryRun, "dry-run", "d", false, "Show what would be removed without actually removing anything")
	flags.BoolVarP(&c.allTargets, "all", "A", false, "Clean up all targets")
	flags.BoolVarP(&c.skipConfirm, "yes", "y", false, "Skip confirmation prompt")

	c.cmd = cmd
	return c
}

// getCompletionTargets returns a list of available targets for command completion
func (c *cleanupCommand) getCompletionTargets(prefix string) []string {
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		return nil
	}

	var targets []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			if prefix == "" || strings.HasPrefix(entry.Name(), prefix) {
				targets = append(targets, entry.Name())
			}
		}
	}
	return targets
}

// showDiskUsage displays disk usage information for all targets
//
// Returns:
//   - error: Any error encountered while gathering disk usage information
func (c *cleanupCommand) showDiskUsage() error {
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		if os.IsNotExist(err) {
			c.cmd.Println("No builds found.")
			return nil
		}
		return fmt.Errorf("failed to read nigiri root directory: %w", err)
	}

	c.cmd.Println("Disk usage by target:")
	totalSize := int64(0)

	for _, entry := range entries {
		if entry.IsDir() && !filepath.HasPrefix(entry.Name(), ".") {
			targetDir := filepath.Join(nigiriRoot, entry.Name())
			size, err := dirutils.GetDirSize(targetDir)
			if err != nil {
				c.cmd.Printf("  %s: Failed to calculate size\n", entry.Name())
				continue
			}

			// Count builds
			buildDirs, err := os.ReadDir(targetDir)
			buildCount := 0
			if err == nil {
				for _, buildDir := range buildDirs {
					if buildDir.IsDir() {
						buildCount++
					}
				}
			}

			c.cmd.Printf("  %s: %.2f MB (%d builds)\n", entry.Name(), float64(size)/(1024*1024), buildCount)
			totalSize += size
		}
	}

	c.cmd.Printf("\nTotal disk usage: %.2f MB\n", float64(totalSize)/(1024*1024))
	c.cmd.Println("\nTo clean up old builds, run 'nigiri cleanup <target>' or 'nigiri cleanup --all'")

	return nil
}

// executeCleanup handles the cleanup of old builds for a specific target
//
// Parameters:
//   - target: The name of the target to clean up
//
// Returns:
//   - error: Any error encountered during the cleanup process
func (c *cleanupCommand) executeCleanup(target string) error {
	fsTarget := fsutils.Target{Target: target}
	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return fmt.Errorf("target '%s' not found", target)
	}

	// Get all builds for this target
	entries, err := dirutils.GetDirEntries(targetRootDir, "")
	if err != nil {
		return fmt.Errorf("failed to read target directory: %w", err)
	}

	// Filter to include only directories
	var builds []dirutils.DirEntry
	for _, entry := range entries {
		if entry.IsDir {
			builds = append(builds, entry)
		}
	}

	if len(builds) == 0 {
		c.cmd.Printf("No builds found for target '%s'.\n", target)
		return nil
	}

	// Sort by modification time (newest first)
	dirutils.SortDirEntriesByTime(builds, true)

	// Determine which builds to remove
	var buildsToRemove []dirutils.DirEntry

	// By count
	if c.maxBuilds > 0 && len(builds) > c.maxBuilds {
		buildsToRemove = append(buildsToRemove, builds[c.maxBuilds:]...)
	}

	// By age
	if c.maxAge > 0 {
		maxAgeDuration := time.Duration(c.maxAge) * 24 * time.Hour
		now := time.Now()

		for _, build := range builds {
			// Skip builds already marked for removal
			alreadyMarked := false
			for _, markedBuild := range buildsToRemove {
				if build.Name == markedBuild.Name {
					alreadyMarked = true
					break
				}
			}

			if !alreadyMarked && now.Sub(build.ModTime) > maxAgeDuration {
				buildsToRemove = append(buildsToRemove, build)
			}
		}
	}

	if len(buildsToRemove) == 0 {
		c.cmd.Printf("No builds to remove for target '%s'.\n", target)
		return nil
	}

	// Calculate total space to be freed
	var totalSizeToFree int64
	for _, build := range buildsToRemove {
		buildPath := filepath.Join(targetRootDir, build.Name)
		size, err := dirutils.GetDirSize(buildPath)
		if err == nil {
			totalSizeToFree += size
		}
	}

	// Show what will be removed
	c.cmd.Printf("Found %d builds to remove for target '%s'.\n", len(buildsToRemove), target)
	c.cmd.Printf("This will free approximately %.2f MB of disk space.\n", float64(totalSizeToFree)/(1024*1024))

	for _, build := range buildsToRemove {
		c.cmd.Printf("  %s (built on %s)\n", build.Name, build.ModTime.Format("2006-01-02 15:04:05"))
	}

	if c.dryRun {
		c.cmd.Println("\nDry run: No builds were removed.")
		return nil
	}

	// Confirm before removing
	if !c.skipConfirm {
		c.cmd.Print("\nDo you want to continue? (y/n): ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if confirm != "y" && confirm != "Y" {
			c.cmd.Println("Cleanup cancelled.")
			return nil
		}
	}

	// Remove the builds
	removedCount := 0
	for _, build := range buildsToRemove {
		buildPath := filepath.Join(targetRootDir, build.Name)
		if err := os.RemoveAll(buildPath); err != nil {
			c.cmd.Printf("Warning: Failed to remove build '%s': %v\n", build.Name, err)
			continue
		}
		removedCount++
	}

	c.cmd.Printf("%d builds removed successfully, freeing %.2f MB of disk space.\n",
		removedCount, float64(totalSizeToFree)/(1024*1024))
	return nil
}

// executeCleanupAll handles the cleanup of old builds for all targets
//
// Returns:
//   - error: Any error encountered during the cleanup process
func (c *cleanupCommand) executeCleanupAll() error {
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		if os.IsNotExist(err) {
			c.cmd.Println("No targets found.")
			return nil
		}
		return fmt.Errorf("failed to read nigiri root directory: %w", err)
	}

	var targets []string
	for _, entry := range entries {
		if entry.IsDir() && !filepath.HasPrefix(entry.Name(), ".") {
			targets = append(targets, entry.Name())
		}
	}

	if len(targets) == 0 {
		c.cmd.Println("No targets found.")
		return nil
	}

	c.cmd.Printf("Cleaning up builds for %d targets...\n", len(targets))

	// If not skipping confirmation and not in dry run mode, confirm once for all targets
	if !c.skipConfirm && !c.dryRun {
		c.cmd.Print("This will clean up old builds for all targets. Continue? (y/n): ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if confirm != "y" && confirm != "Y" {
			c.cmd.Println("Cleanup cancelled.")
			return nil
		}

		// Set skipConfirm to true to avoid asking again for each target
		c.skipConfirm = true
	}

	for _, target := range targets {
		c.cmd.Printf("\nProcessing target '%s':\n", target)
		if err := c.executeCleanup(target); err != nil {
			c.cmd.Printf("Warning: Error cleaning up target '%s': %v\n", target, err)
		}
	}

	return nil
}
