package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/config"
	"github.com/oota-sushikuitee/nigiri/pkg/fsutils"
	"github.com/oota-sushikuitee/nigiri/pkg/vcsutils"
	"github.com/spf13/cobra"
)

// buildCommand represents the structure for the build command
type buildCommand struct {
	// cmd is the cobra command instance
	cmd *cobra.Command
	// commit specifies a particular commit to build
	commit string
	// depth is the git clone depth
	depth int
	// verbose enables verbose output
	verbose bool
	// forceBuild forces rebuilding even if already built
	forceBuild bool
}

// newBuildCommand creates a new build command instance which is responsible for
// building targets according to their configurations in the nigiri config file.
// It handles the process of cloning repositories and executing build commands.
//
// Returns:
//   - *buildCommand: A configured build command instance
func newBuildCommand() *buildCommand {
	c := &buildCommand{}
	cmd := &cobra.Command{
		Use:   "build target [commit]",
		Short: "Build a target",
		Long: `Build a target from a source repository.
If commit is not specified, the latest commit on the default branch will be built.
If the target has already been built at the specified commit, the build will be skipped unless --force is specified.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			target := args[0]

			// Optional commit hash argument
			if len(args) > 1 {
				c.commit = args[1]
			}

			return c.executeBuild(target)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Offer tab completion for targets if no arguments provided yet
			if len(args) == 0 {
				return c.getCompletionTargets(toComplete), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	// Add flags
	flags := cmd.Flags()
	flags.BoolVarP(&c.verbose, "verbose", "v", false, "Enable verbose output")
	flags.IntVarP(&c.depth, "depth", "d", 1, "Git clone depth (use 0 for full history)")
	flags.BoolVarP(&c.forceBuild, "force", "f", false, "Force rebuild even if the target has already been built at the specified commit")

	c.cmd = cmd
	return c
}

// getCompletionTargets returns a list of available targets for command completion
func (c *buildCommand) getCompletionTargets(prefix string) []string {
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

// executeBuild handles the build process for the specified target.
// It loads configuration, clones the repository at the default branch's HEAD,
// and executes the appropriate OS-specific build command.
//
// Parameters:
//   - target: The name of the target to build as specified in the config file
//
// Returns:
//   - error: Any error encountered during the build process
func (c *buildCommand) executeBuild(target string) error {
	// Load configuration
	cfg := config.NewConfig()
	err := cfg.LoadCfgFile()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Check if target exists in config
	targetCfg, exists := cfg.Targets[target]
	if !exists {
		return fmt.Errorf("target '%s' not found in configuration", target)
	}

	// Create target directory if it doesn't exist
	fsTarget := fsutils.Target{Target: target}
	if _, createErr := fsTarget.CreateTargetRootDirIfNotExist(nigiriRoot); createErr != nil {
		return fmt.Errorf("failed to create target directory: %w", createErr)
	}

	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return fmt.Errorf("failed to get target directory: %w", err)
	}

	// Initialize git utility
	git := vcsutils.Git{
		Source: targetCfg.Sources,
	}

	// Determine the commit to build
	var headCommit commits.Commit
	if c.commit == "" {
		// Get the HEAD of the default branch
		defaultBranch := targetCfg.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main" // Default to 'main' if not specified
		}

		c.cmd.Printf("Getting HEAD of branch '%s' from %s...\n", defaultBranch, targetCfg.Sources)
		if gitErr := git.GetDefaultBranchRemoteHead(defaultBranch); gitErr != nil {
			return fmt.Errorf("failed to get HEAD of branch '%s': %w", defaultBranch, gitErr)
		}

		headCommit = commits.Commit{
			Hash: git.HEAD,
		}
	} else {
		// Use the specified commit
		c.cmd.Printf("Using specified commit: %s\n", c.commit)
		headCommit = commits.Commit{
			Hash: c.commit,
		}
	}

	if hashErr := headCommit.CalculateShortHash(); hashErr != nil {
		return fmt.Errorf("failed to calculate short hash: %w", hashErr)
	}

	if validateErr := headCommit.Validate(); validateErr != nil {
		return fmt.Errorf("invalid commit: %w", validateErr)
	}

	// Check if commit has already been built
	isExistCommitDir := fsutils.IsExistTargetCommitDir(targetRootDir, headCommit)
	if isExistCommitDir && !c.forceBuild {
		c.cmd.Printf("Commit %s has already been built. Use --force to rebuild.\n", headCommit.ShortHash)
		return nil
	}

	// Create commit directory
	var commitDir string
	var createErr error
	if isExistCommitDir {
		// If force rebuild, use the existing directory
		commitDir = filepath.Join(targetRootDir, headCommit.ShortHash)
		c.cmd.Printf("Force rebuilding commit %s\n", headCommit.ShortHash)

		// Clean up the src directory
		srcDir := filepath.Join(commitDir, "src")
		if cleanErr := os.RemoveAll(srcDir); cleanErr != nil {
			return fmt.Errorf("failed to clean src directory: %w", cleanErr)
		}
	} else {
		// Create a new commit directory
		commitDir, createErr = fsutils.CreateTargetCommitDir(targetRootDir, headCommit)
		if createErr != nil {
			return fmt.Errorf("failed to create commit directory: %w", createErr)
		}
	}

	// Record current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	defer func() {
		if dirErr := os.Chdir(cwd); dirErr != nil {
			c.cmd.Printf("Warning: Failed to change back to original directory: %v\n", dirErr)
		}
	}()

	// Change to the commit directory
	if chErr := os.Chdir(commitDir); chErr != nil {
		return fmt.Errorf("failed to change to commit directory: %w", chErr)
	}

	// Create log directory for build logs
	logDir := filepath.Join(commitDir, "logs")
	if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
		return fmt.Errorf("failed to create log directory: %w", mkErr)
	}

	// Clone the repository with specified options
	cloneStartTime := time.Now()
	cloneDir := filepath.Join(commitDir, "src")
	c.cmd.Printf("Cloning repository to %s...\n", cloneDir)

	cloneOptions := vcsutils.Options{
		Depth:   c.depth,
		Verbose: c.verbose,
	}

	if cloneErr := git.Clone(cloneDir, cloneOptions); cloneErr != nil {
		return fmt.Errorf("failed to clone repository: %w", cloneErr)
	}

	// If specific commit was requested, check it out
	if c.commit != "" && c.depth != 1 {
		c.cmd.Printf("Checking out commit %s...\n", c.commit)
		if checkoutErr := git.Checkout(cloneDir, c.commit); checkoutErr != nil {
			return fmt.Errorf("failed to checkout commit %s: %w", c.commit, checkoutErr)
		}
	}

	cloneDuration := time.Since(cloneStartTime)
	c.cmd.Printf("Repository cloned in %s\n", cloneDuration)

	// Change to the source directory for building
	if chdirErr := os.Chdir(cloneDir); chdirErr != nil {
		return fmt.Errorf("failed to change to source directory: %w", chdirErr)
	}

	// Select the appropriate build command based on the OS
	buildCmd := targetCfg.BuildCommand
	var cmd string
	switch os := runtime.GOOS; os {
	case "linux":
		cmd = buildCmd.Linux
	case "windows":
		cmd = buildCmd.Windows
	case "darwin":
		cmd = buildCmd.Darwin
	default:
		return fmt.Errorf("unsupported OS: %s", os)
	}

	if cmd == "" {
		return fmt.Errorf("no build command specified for OS: %s", runtime.GOOS)
	}

	// Build log file path
	buildLogPath := filepath.Join(logDir, "build.log")
	buildLogFile, err := os.Create(buildLogPath)
	if err != nil {
		return fmt.Errorf("failed to create build log file: %w", err)
	}
	defer buildLogFile.Close()

	// Run the build command
	c.cmd.Printf("Building target '%s' with command: %s\n", target, cmd)
	buildStartTime := time.Now()

	execCmd := exec.Command("/bin/sh", "-c", cmd)
	execCmd.Stdout = buildLogFile
	execCmd.Stderr = buildLogFile

	if c.verbose {
		// If verbose, show output in terminal too
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
	}

	if cmdErr := execCmd.Run(); cmdErr != nil {
		return fmt.Errorf("build failed: %w\nSee build log at %s", cmdErr, buildLogPath)
	}

	buildDuration := time.Since(buildStartTime)
	c.cmd.Printf("Build completed successfully in %s\n", buildDuration)

	// Create a build metadata file
	metadataPath := filepath.Join(commitDir, "build-info.txt")
	metaFile, err := os.Create(metadataPath)
	if err == nil {
		defer metaFile.Close()
		fmt.Fprintf(metaFile, "Target: %s\n", target)
		fmt.Fprintf(metaFile, "Commit: %s\n", headCommit.Hash)
		fmt.Fprintf(metaFile, "Short hash: %s\n", headCommit.ShortHash)
		fmt.Fprintf(metaFile, "Build date: %s\n", time.Now().Format(time.RFC3339))
		fmt.Fprintf(metaFile, "Clone duration: %s\n", cloneDuration)
		fmt.Fprintf(metaFile, "Build duration: %s\n", buildDuration)
		fmt.Fprintf(metaFile, "OS: %s\n", runtime.GOOS)
		fmt.Fprintf(metaFile, "Architecture: %s\n", runtime.GOARCH)
	}

	c.cmd.Printf("Target '%s' built at commit %s\n", target, headCommit.ShortHash)
	c.cmd.Printf("Run with: nigiri run %s %s\n", target, headCommit.ShortHash)

	return nil
}
