package commands

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/config"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
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
	// useToken enables GitHub token authentication
	useToken bool
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
	flags.BoolVarP(&c.useToken, "use-token", "t", false, "Use GitHub token for authentication (required for private repositories)")

	c.cmd = cmd
	return c
}

// getCompletionTargets returns a list of available targets for command completion
func (c *buildCommand) getCompletionTargets(prefix string) []string {
	cm := config.NewConfigManager()
	if err := cm.LoadCfgFile(); err != nil {
		return nil
	}

	var targets []string
	for target := range cm.Config.Targets {
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
	cm := config.NewConfigManager()
	err := cm.LoadCfgFile()
	if err != nil {
		return logger.CreateErrorf("failed to load configuration: %w", err)
	}

	// Check if target exists in config
	targetCfg, exists := cm.Config.Targets[target]
	if !exists {
		return logger.CreateErrorf("target '%s' not found in configuration", target)
	}

	// Create target directory if it doesn't exist
	fsTarget := targets.Target{
		Target:  target,
		Commits: commits.Commits{},
	}

	if _, createErr := fsTarget.CreateTargetRootDirIfNotExist(nigiriRoot); createErr != nil {
		return logger.CreateErrorf("failed to create target directory: %w", createErr)
	}

	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return logger.CreateErrorf("failed to get target directory: %w", err)
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
			return logger.CreateErrorf("failed to get HEAD of branch '%s': %w", defaultBranch, gitErr)
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
		return logger.CreateErrorf("failed to calculate short hash: %w", hashErr)
	}

	if validateErr := headCommit.Validate(); validateErr != nil {
		return logger.CreateErrorf("invalid commit: %w", validateErr)
	}

	// Check if commit has already been built
	isExistCommitDir := targets.IsExistTargetCommitDir(targetRootDir, headCommit)
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
			return logger.CreateErrorf("failed to clean src directory: %w", cleanErr)
		}
	} else {
		// Create a new commit directory
		commitDir, createErr = targets.CreateTargetCommitDir(targetRootDir, headCommit)
		if createErr != nil {
			return logger.CreateErrorf("failed to create commit directory: %w", createErr)
		}
	}

	// Record current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return logger.CreateErrorf("failed to get current working directory: %w", err)
	}
	defer func() {
		if dirErr := os.Chdir(cwd); dirErr != nil {
			logger.Warnf("Failed to change back to original directory: %v", dirErr)
		}
	}()

	// Change to the commit directory
	if chErr := os.Chdir(commitDir); chErr != nil {
		return logger.CreateErrorf("failed to change to commit directory: %w", chErr)
	}

	// Create log directory for build logs
	logDir := filepath.Join(commitDir, "logs")
	if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
		return logger.CreateErrorf("failed to create log directory: %w", mkErr)
	}

	// Clone the repository with specified options
	cloneStartTime := time.Now()
	cloneDir := filepath.Join(commitDir, "src")
	c.cmd.Printf("Cloning repository to %s...\n", cloneDir)
	authMethod := vcsutils.AuthNone
	if c.useToken {
		authMethod = vcsutils.AuthToken
	}
	cloneOptions := vcsutils.Options{
		Depth:      c.depth,
		Verbose:    c.verbose,
		AuthMethod: authMethod,
	}
	if cloneErr := git.Clone(cloneDir, cloneOptions); cloneErr != nil {
		return logger.CreateErrorf("failed to clone repository: %w", cloneErr)
	}

	// If specific commit was requested, check it out
	if c.commit != "" && c.depth != 1 {
		c.cmd.Printf("Checking out commit %s...\n", c.commit)
		if checkoutErr := git.Checkout(cloneDir, c.commit); checkoutErr != nil {
			return logger.CreateErrorf("failed to checkout commit %s: %w", c.commit, checkoutErr)
		}
	}

	cloneDuration := time.Since(cloneStartTime)
	c.cmd.Printf("Repository cloned in %s\n", cloneDuration)

	// Change to the source directory for building
	// If working directory is specified, change to that directory
	workDir := cloneDir
	if targetCfg.WorkingDirectory != "" {
		workDir = filepath.Join(cloneDir, targetCfg.WorkingDirectory)
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			return logger.CreateErrorf("working directory '%s' not found in source", targetCfg.WorkingDirectory)
		}
	}
	if chdirErr := os.Chdir(workDir); chdirErr != nil {
		return logger.CreateErrorf("failed to change to working directory: %w", chdirErr)
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
		return logger.CreateErrorf("unsupported OS: %s", runtime.GOOS)
	}

	if cmd == "" {
		return logger.CreateErrorf("no build command specified for OS: %s", runtime.GOOS)
	}

	// Build log file path
	buildLogPath := filepath.Join(logDir, "build.log")
	buildLogFile, err := os.Create(buildLogPath)
	if err != nil {
		return logger.CreateErrorf("failed to create build log file: %w", err)
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
		execCmd.Stdout = io.MultiWriter(os.Stdout, buildLogFile)
		execCmd.Stderr = io.MultiWriter(os.Stderr, buildLogFile)
	}

	// Set environment variables if specified
	if len(targetCfg.Env) > 0 {
		execCmd.Env = append(os.Environ(), targetCfg.Env...)
	}

	buildErr := execCmd.Run()
	buildDuration := time.Since(buildStartTime)

	// Create a build metadata file
	metadataPath := filepath.Join(commitDir, "build-info.txt")
	metaFile, err := os.Create(metadataPath)
	if err == nil {
		defer metaFile.Close()
		if _, err := metaFile.WriteString(fmt.Sprintf("Target: %s\n", target)); err != nil {
			logger.Warnf("Failed to write target info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("Commit: %s\n", headCommit.Hash)); err != nil {
			logger.Warnf("Failed to write commit info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("Short hash: %s\n", headCommit.ShortHash)); err != nil {
			logger.Warnf("Failed to write short hash info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("Build date: %s\n", time.Now().Format(time.RFC3339))); err != nil {
			logger.Warnf("Failed to write build date info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("Clone duration: %s\n", cloneDuration)); err != nil {
			logger.Warnf("Failed to write clone duration info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("Build duration: %s\n", buildDuration)); err != nil {
			logger.Warnf("Failed to write build duration info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("OS: %s\n", runtime.GOOS)); err != nil {
			logger.Warnf("Failed to write OS info: %v", err)
		}
		if _, err := metaFile.WriteString(fmt.Sprintf("Architecture: %s\n", runtime.GOARCH)); err != nil {
			logger.Warnf("Failed to write architecture info: %v", err)
		}
	}

	// Process source files based on binary_only option or always compress them
	if buildErr == nil {
		// Copy built binary if binary path is specified
		binaryPath, hasBinaryPath := buildCmd.BinaryPath()
		if hasBinaryPath {
			// If binary path is specified, copy it to the commit directory
			sourceFile := filepath.Join(workDir, binaryPath)
			destFile := filepath.Join(commitDir, "bin")

			// Create bin directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
				logger.Warnf("Failed to create bin directory: %v", err)
			} else {
				// Copy the binary
				if copyErr := copyFile(sourceFile, destFile); copyErr != nil {
					logger.Warnf("Failed to copy binary: %v", copyErr)
				}
			}
		}
	}

	// Handle binary_only option or compress source
	if targetCfg.BinaryOnly {
		// If binary_only is set, remove source directory
		if err := os.RemoveAll(cloneDir); err != nil {
			logger.Warnf("Failed to remove source directory: %v", err)
		}
	} else {
		// Compress source directory
		srcTarGzPath := filepath.Join(commitDir, "source.tar.gz")
		if err := compressDirectory(cloneDir, srcTarGzPath); err != nil {
			logger.Warnf("Failed to compress source directory: %v", err)
		} else {
			// If compression successful, remove source directory
			if err := os.RemoveAll(cloneDir); err != nil {
				logger.Warnf("Failed to remove source directory after compression: %v", err)
			}
		}
	}

	// Check if build was successful
	if buildErr != nil {
		return logger.CreateErrorf("build failed: %w\nSee build log at %s", buildErr, buildLogPath)
	}

	c.cmd.Printf("Target '%s' built at commit %s\n", target, headCommit.ShortHash)
	c.cmd.Printf("Run with: nigiri run %s %s\n", target, headCommit.ShortHash)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy file contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Get file permissions
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(dst, info.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// compressDirectory compresses a directory into a tar.gz file
func compressDirectory(srcDir, tarGzPath string) error {
	// Create tar.gz file
	tarGzFile, err := os.Create(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to create tar.gz file: %w", err)
	}
	defer tarGzFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(tarGzFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk through directory and add files to tar
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Set header name relative to source directory
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		header.Name = relPath

		// Skip if it's the root directory
		if relPath == "." {
			return nil
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// Skip directories (they are only headers in tar)
		if info.IsDir() {
			return nil
		}

		// Open and copy file contents to tar
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to write file to tar: %w", err)
		}

		return nil
	})
}
