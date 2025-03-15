package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oota-sushikuitee/nigiri/pkg/config"
	"github.com/oota-sushikuitee/nigiri/pkg/fsutils"
	"github.com/spf13/cobra"
)

// runCommand represents the structure for the run command
type runCommand struct {
	cmd *cobra.Command
}

// newRunCommand creates a new run command instance which allows users
// to execute previously built targets with optional arguments.
// The command supports specifying a particular commit to run or defaults to the latest.
func newRunCommand() *runCommand {
	c := &runCommand{}
	cmd := &cobra.Command{
		Use:   "run target [commit] [-- args...]",
		Short: "Run a built target",
		Long: `Run a built target with optional arguments.
If commit is not specified, the latest built commit will be used.
Any arguments after -- will be passed to the target.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			target := args[0]
			var commitHash string
			var targetArgs []string

			// Parse arguments to separate commit and target args
			if len(args) > 1 {
				// Look for "--" in args to find target arguments
				dashIndex := -1
				for i, arg := range args[1:] {
					if arg == "--" {
						dashIndex = i + 1
						break
					}
				}

				if dashIndex != -1 {
					// "--" was found
					if dashIndex > 1 {
						// There is a commit hash between target and "--"
						commitHash = args[1]
					}
					if dashIndex < len(args)-1 {
						// There are args after "--"
						targetArgs = args[dashIndex+1:]
					}
				} else if len(args) > 1 {
					// If no "--", then the second arg is commit hash
					commitHash = args[1]
				}
			}

			return c.executeRun(target, commitHash, targetArgs)
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

	c.cmd = cmd
	return c
}

// getCompletionTargets returns a list of available targets for command completion
func (c *runCommand) getCompletionTargets(prefix string) []string {
	cfg := config.NewConfig()
	if err := cfg.LoadCfgFile(); err != nil {
		return nil
	}

	var targets []string
	for target := range cfg.Targets {
		if strings.HasPrefix(target, prefix) {
			targets = append(targets, target)
		}
	}
	return targets
}

// getCompletionCommits returns a list of available commit hashes for the specified target
func (c *runCommand) getCompletionCommits(target, prefix string) []string {
	fsTarget := fsutils.Target{Target: target}
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

// executeRun executes the specified target with the given commit hash and arguments.
// If commitHash is empty, it uses the most recently built version of the target.
// It handles locating the binary, setting up the execution environment, and running the process.
//
// Parameters:
//   - target: The name of the built target to run
//   - commitHash: The specific commit hash to use (can be empty for the latest build)
//   - args: Additional arguments to pass to the target binary when executing
//
// Returns:
//   - error: Any error encountered during the execution process
func (c *runCommand) executeRun(target, commitHash string, args []string) error {
	fsTarget := fsutils.Target{Target: target}
	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return err
	}

	// Use latest commit if none specified
	var runDir string
	if commitHash == "" {
		// Find the most recent commit directory
		dirs, err := os.ReadDir(targetRootDir)
		if err != nil {
			return fmt.Errorf("failed to read target directory: %w", err)
		}

		var latestDir string
		var latestInfo os.FileInfo
		for _, dir := range dirs {
			if dir.IsDir() {
				info, err := os.Stat(filepath.Join(targetRootDir, dir.Name()))
				if err != nil {
					continue
				}
				if latestInfo == nil || info.ModTime().After(latestInfo.ModTime()) {
					latestInfo = info
					latestDir = dir.Name()
				}
			}
		}

		if latestDir == "" {
			return fmt.Errorf("no builds found for target %s", target)
		}

		runDir = filepath.Join(targetRootDir, latestDir)
		c.cmd.Printf("Using latest commit: %s\n", latestDir)
	} else {
		// For specified commit
		if len(commitHash) < 7 {
			return fmt.Errorf("commit hash is too short: %s (minimum 7 characters)", commitHash)
		}

		// Find directory matching the commit hash
		dirs, err := os.ReadDir(targetRootDir)
		if err != nil {
			return fmt.Errorf("failed to read target directory: %w", err)
		}

		var matchingDir string
		for _, dir := range dirs {
			if dir.IsDir() && strings.HasPrefix(dir.Name(), commitHash) {
				matchingDir = dir.Name()
				break
			}
		}

		if matchingDir == "" {
			return fmt.Errorf("no build found for commit %s", commitHash)
		}

		runDir = filepath.Join(targetRootDir, matchingDir)
	}

	// Find executable file
	cfg := config.NewConfig()
	if err := cfg.LoadCfgFile(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Locate source directory
	srcDir := filepath.Join(runDir, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory not found: %s", srcDir)
	}

	// Get binary path from config or use default location
	var binaryPath string
	if binPath, ok := cfg.Targets[target].BuildCommand.BinaryPath(); ok {
		binaryPath = filepath.Join(srcDir, binPath)
	} else {
		// By default, assume binary is in the src directory
		binaryPath = filepath.Join(srcDir, target)

		// If binary not found directly, try common locations
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			// Try bin/ directory
			altPath := filepath.Join(srcDir, "bin", target)
			if _, err := os.Stat(altPath); err == nil {
				binaryPath = altPath
			} else {
				// Try build/ directory
				altPath = filepath.Join(srcDir, "build", target)
				if _, err := os.Stat(altPath); err == nil {
					binaryPath = altPath
				}
			}
		}
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s", binaryPath)
	}

	// Make sure binary is executable
	if runtime := os.Getenv("GOOS"); runtime != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	// Set up environment from the config if specified
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = c.cmd.OutOrStdout()
	cmd.Stderr = c.cmd.ErrOrStderr()
	cmd.Stdin = os.Stdin
	cmd.Dir = srcDir // Set working directory to source directory

	// Add any environment variables from config
	if len(cfg.Targets[target].Env) > 0 {
		cmd.Env = append(os.Environ(), cfg.Targets[target].Env...)
	}

	c.cmd.Printf("Running %s with args: %v\n", binaryPath, args)
	return cmd.Run()
}
