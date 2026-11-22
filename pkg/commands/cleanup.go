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

	builds, err := listBuildsNewestFirst(targetRootDir)
	if err != nil {
		return err
	}

	if len(builds) == 0 {
		c.cmd.Printf("No builds found for target '%s'.\n", target)

		return nil
	}

	buildsToRemove := c.selectBuildsToRemove(builds)
	if len(buildsToRemove) == 0 {
		c.cmd.Printf("No builds to remove for target '%s'.\n", target)

		return nil
	}

	totalSizeToFree := totalBuildSize(targetRootDir, buildsToRemove)
	c.printRemovalPlan(target, buildsToRemove, totalSizeToFree)

	if c.dryRun {
		c.cmd.Println("\nDry run: No builds were removed.")

		return nil
	}

	if !c.skipConfirm {
		c.cmd.Print("\nDo you want to continue? (y/n): ")

		confirmed, err := readYes()
		if err != nil {
			return err
		}

		if !confirmed {
			c.cmd.Println("Cleanup cancelled.")

			return nil
		}
	}

	removedCount := c.removeBuilds(targetRootDir, buildsToRemove)

	c.cmd.Printf("%d builds removed successfully, freeing %.2f MB of disk space.\n",
		removedCount, toMB(totalSizeToFree))

	return nil
}

// listBuildsNewestFirst orders builds by recorded build time rather than the
// directory mtime, which running or extracting a build rewrites.
func listBuildsNewestFirst(targetRootDir string) ([]dirutils.DirEntry, error) {
	entries, err := dirutils.GetDirEntries(targetRootDir, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read target directory: %w", err)
	}

	var builds []dirutils.DirEntry

	for _, entry := range entries {
		if entry.IsDir {
			entry.ModTime = buildTime(filepath.Join(targetRootDir, entry.Name), entry.ModTime)
			builds = append(builds, entry)
		}
	}

	dirutils.SortDirEntriesByTime(builds, true)

	return builds, nil
}

// selectBuildsToRemove expects builds newest first; builds beyond --max-builds
// come first in the result, followed by the remaining ones older than --max-age.
func (c *cleanupCommand) selectBuildsToRemove(builds []dirutils.DirEntry) []dirutils.DirEntry {
	var buildsToRemove []dirutils.DirEntry

	if c.maxBuilds > 0 && len(builds) > c.maxBuilds {
		buildsToRemove = append(buildsToRemove, builds[c.maxBuilds:]...)
	}

	if c.maxAge <= 0 {
		return buildsToRemove
	}

	maxAgeDuration := time.Duration(c.maxAge) * hoursPerDay * time.Hour
	now := time.Now()

	for _, build := range builds {
		if !containsBuild(buildsToRemove, build.Name) && now.Sub(build.ModTime) > maxAgeDuration {
			buildsToRemove = append(buildsToRemove, build)
		}
	}

	return buildsToRemove
}

func (c *cleanupCommand) printRemovalPlan(target string, builds []dirutils.DirEntry, totalSize int64) {
	c.cmd.Printf("Found %d builds to remove for target '%s'.\n", len(builds), target)
	c.cmd.Printf("This will free approximately %.2f MB of disk space.\n", toMB(totalSize))

	for _, build := range builds {
		c.cmd.Printf("  %s (built on %s)\n", build.Name, build.ModTime.Format("2006-01-02 15:04:05"))
	}
}

func containsBuild(builds []dirutils.DirEntry, name string) bool {
	for _, build := range builds {
		if build.Name == name {
			return true
		}
	}

	return false
}

func totalBuildSize(targetRootDir string, builds []dirutils.DirEntry) int64 {
	var total int64

	for _, build := range builds {
		size, err := dirutils.GetDirSize(filepath.Join(targetRootDir, build.Name))
		if err == nil {
			total += size
		}
	}

	return total
}

func (c *cleanupCommand) removeBuilds(targetRootDir string, builds []dirutils.DirEntry) int {
	removedCount := 0

	for _, build := range builds {
		buildPath := filepath.Join(targetRootDir, build.Name)
		if err := os.RemoveAll(buildPath); err != nil {
			c.cmd.Printf("Warning: Failed to remove build '%s': %v\n", build.Name, err)

			continue
		}

		removedCount++
	}

	return removedCount
}

// readYes reads one line from stdin and reports whether it is "y" or "Y".
func readYes() (bool, error) {
	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	return confirm == "y" || confirm == "Y", nil
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

	// Confirm once for all targets instead of once per target.
	if !c.skipConfirm && !c.dryRun {
		c.cmd.Print("This will clean up old builds for all targets. Continue? (y/n): ")

		confirmed, err := readYes()
		if err != nil {
			return err
		}

		if !confirmed {
			c.cmd.Println("Cleanup cancelled.")

			return nil
		}

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
