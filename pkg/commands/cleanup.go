package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oota-sushikuitee/nigiri/internal/dirutils"
	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/spf13/cobra"
)

const (
	defaultMaxAgeDays = 30
	defaultMaxBuilds  = 5
	hoursPerDay       = 24
	bytesPerMB        = 1 << 20
)

func toMB(size int64) float64 {
	return float64(size) / bytesPerMB
}

type cleanupCommand struct {
	cmd         *cobra.Command
	maxAge      int
	maxBuilds   int
	dryRun      bool
	allTargets  bool
	skipConfirm bool
}

func newCleanupCommand() *cleanupCommand {
	c := &cleanupCommand{}
	cmd := &cobra.Command{
		Use:   "cleanup [target]",
		Short: "Clean up old builds",
		Long: `Clean up old builds to manage disk space.
If a target is specified, only that target's builds will be cleaned up.
Without arguments, shows the current disk usage of builds.`,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				if c.allTargets {
					return c.executeCleanupAll()
				}

				return c.showDiskUsage()
			}

			return c.executeCleanup(args[0])
		},
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Offer tab completion for targets if no arguments provided yet
			if len(args) == 0 {
				return c.getCompletionTargets(toComplete), cobra.ShellCompDirectiveNoFileComp
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	flags := cmd.Flags()
	flags.IntVarP(&c.maxAge, "max-age", "a", defaultMaxAgeDays, "Maximum age of builds to keep in days (0 to disable)")
	flags.IntVarP(&c.maxBuilds, "max-builds", "b", defaultMaxBuilds,
		"Maximum number of builds to keep per target (0 to disable)")
	flags.BoolVarP(&c.dryRun, "dry-run", "d", false, "Show what would be removed without actually removing anything")
	flags.BoolVarP(&c.allTargets, "all", "A", false, "Clean up all targets")
	flags.BoolVarP(&c.skipConfirm, "yes", "y", false, "Skip confirmation prompt")

	c.cmd = cmd

	return c
}

func (c *cleanupCommand) getCompletionTargets(prefix string) []string {
	return getInstalledTargets(prefix)
}

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
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
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

			c.cmd.Printf("  %s: %.2f MB (%d builds)\n", entry.Name(), toMB(size), buildCount)
			totalSize += size
		}
	}

	c.cmd.Printf("\nTotal disk usage: %.2f MB\n", toMB(totalSize))
	c.cmd.Println("\nTo clean up old builds, run 'nigiri cleanup <target>' or 'nigiri cleanup --all'")

	return nil
}

func (c *cleanupCommand) executeCleanup(target string) error {
	fsTarget := targets.Target{Target: target}

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

	// Retention must follow when a build was produced, not the directory mtime,
	// which running or extracting a build rewrites.
	for i := range builds {
		buildPath := filepath.Join(targetRootDir, builds[i].Name)
		builds[i].ModTime = buildTime(buildPath, builds[i].ModTime)
	}

	// Sort by build time (newest first)
	dirutils.SortDirEntriesByTime(builds, true)

	// Determine which builds to remove
	var buildsToRemove []dirutils.DirEntry

	// By count
	if c.maxBuilds > 0 && len(builds) > c.maxBuilds {
		buildsToRemove = append(buildsToRemove, builds[c.maxBuilds:]...)
	}

	// By age
	if c.maxAge > 0 {
		maxAgeDuration := time.Duration(c.maxAge) * hoursPerDay * time.Hour
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
	c.cmd.Printf("This will free approximately %.2f MB of disk space.\n", toMB(totalSizeToFree))

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
		removedCount, toMB(totalSizeToFree))

	return nil
}

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
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
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
