package commands

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	cfgmodel "github.com/oota-sushikuitee/nigiri/internal/models/config"
	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
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
	// timeout is the build timeout in minutes (0 = no timeout)
	timeout int
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
	flags.IntVar(&c.timeout, "timeout", 30, "Build timeout in minutes (0 = no timeout)")

	c.cmd = cmd
	return c
}

// getCompletionTargets returns a list of available targets for command completion
func (c *buildCommand) getCompletionTargets(prefix string) []string {
	return getConfiguredTargets(prefix)
}

// resolveCloneDepth determines the clone depth to use. A shallow clone only
// contains the default branch HEAD, so it cannot resolve an arbitrary commit;
// when a commit is requested, fall back to a full clone (depth 0).
func resolveCloneDepth(depth int, commit string) int {
	if commit != "" {
		return 0
	}
	return depth
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
	cm := newConfigManager()
	err := cm.LoadCfgFile()
	if err != nil {
		return logger.CreateErrorf("failed to load configuration: %w", err)
	}

	// Check if target exists in config
	targetCfg, exists := cm.Config.GetTarget(target)
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

	// A commit directory created for a build that then fails must not survive:
	// the next build would see it and report the commit as already built.
	buildSucceeded := false
	if !isExistCommitDir {
		defer func() {
			if buildSucceeded {
				return
			}
			if rmErr := os.RemoveAll(commitDir); rmErr != nil {
				logger.Warnf("Failed to remove commit directory %s after failed build: %v", commitDir, rmErr)
			}
		}()
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
	cloneDepth := resolveCloneDepth(c.depth, c.commit)
	if c.commit != "" && cloneDepth != c.depth {
		c.cmd.Printf("Commit specified; cloning full history to resolve %s\n", c.commit)
	}
	cloneOptions := vcsutils.Options{
		Depth:      cloneDepth,
		Verbose:    c.verbose,
		AuthMethod: authMethod,
	}
	if cloneErr := git.Clone(cloneDir, cloneOptions); cloneErr != nil {
		return logger.CreateErrorf("failed to clone repository: %w", cloneErr)
	}

	// If a specific commit was requested, always check it out so the build
	// never silently uses the default branch HEAD instead
	if c.commit != "" {
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
	cmd, cmdErr := selectBuildCommand(buildCmd, cm.Config.Defaults, runtime.GOOS)
	if cmdErr != nil {
		return cmdErr
	}

	// Build log file path
	buildLogPath := filepath.Join(logDir, "build.log")
	buildLogFile, err := os.Create(buildLogPath)
	if err != nil {
		return logger.CreateErrorf("failed to create build log file: %w", err)
	}
	defer func() {
		if err := buildLogFile.Close(); err != nil {
			logger.Warnf("failed to close build log file: %v", err)
		}
	}()

	// Run the build command
	c.cmd.Printf("Building target '%s' with command: %s\n", target, cmd)
	if c.timeout > 0 {
		c.cmd.Printf("Build timeout: %d minutes\n", c.timeout)
	}
	buildStartTime := time.Now()

	// Create context with timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc
	if c.timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(c.timeout)*time.Minute)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	var stdout io.Writer = buildLogFile
	var stderr io.Writer = buildLogFile
	if c.verbose {
		// If verbose, show output in terminal too
		stdout = io.MultiWriter(os.Stdout, buildLogFile)
		stderr = io.MultiWriter(os.Stderr, buildLogFile)
	}

	var env []string
	if len(targetCfg.Env) > 0 {
		env = append(os.Environ(), targetCfg.Env...)
	}

	buildErr := runBuildCommand(ctx, cmd, stdout, stderr, env, buildWaitDelay)

	// Check if the build was killed due to timeout
	if ctx.Err() == context.DeadlineExceeded {
		buildErr = logger.CreateErrorf("build timed out after %d minutes", c.timeout)
	}
	buildDuration := time.Since(buildStartTime)

	// Record build metadata. Only successful builds are recorded, so the build
	// date always describes the artifacts actually present in the directory.
	if buildErr == nil {
		metadata := fmt.Sprintf("Target: %s\nCommit: %s\nShort hash: %s\n%s %s\nClone duration: %s\nBuild duration: %s\nOS: %s\nArchitecture: %s\n",
			target, headCommit.Hash, headCommit.ShortHash,
			buildDateField, time.Now().Format(time.RFC3339),
			cloneDuration, buildDuration, runtime.GOOS, runtime.GOARCH)
		if writeErr := os.WriteFile(filepath.Join(commitDir, buildInfoFileName), []byte(metadata), 0644); writeErr != nil {
			logger.Warnf("Failed to write build metadata: %v", writeErr)
		}
	}

	// Only a successful build produces artifacts. A failed forced rebuild must
	// leave the previous build's artifacts untouched instead of replacing them
	// with the output of a build that did not run.
	if buildErr == nil {
		// Copy built binary if binary path is specified
		binaryCaptured := false
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
				} else {
					binaryCaptured = true
				}
			}
		}

		// Handle binary_only option or compress source. Dropping the source
		// without a binary would leave nothing runnable behind, so the archive
		// is kept whenever no binary was captured.
		if targetCfg.BinaryOnly && binaryCaptured {
			// If binary_only is set, remove source directory
			if err := os.RemoveAll(cloneDir); err != nil {
				logger.Warnf("Failed to remove source directory: %v", err)
			}
		} else {
			if targetCfg.BinaryOnly {
				logger.Warnf("binary-only is set for target '%s' but no binary was captured; keeping the source archive", target)
			}
			// Compress source directory
			srcTarGzPath := filepath.Join(commitDir, "source.tar.gz")
			if err := compressDirectory(cloneDir, srcTarGzPath); err != nil {
				logger.Warnf("Failed to compress source directory (keeping the source tree): %v", err)
				// A partial archive would fail to extract later; drop it.
				if rmErr := os.Remove(srcTarGzPath); rmErr != nil && !os.IsNotExist(rmErr) {
					logger.Warnf("Failed to remove incomplete archive %s: %v", srcTarGzPath, rmErr)
				}
			} else {
				// If compression successful, remove source directory
				if err := os.RemoveAll(cloneDir); err != nil {
					logger.Warnf("Failed to remove source directory after compression: %v", err)
				}
			}
		}
	}

	// Check if build was successful
	if buildErr != nil {
		return logger.CreateErrorf("build failed: %w\nSee build log at %s", buildErr, buildLogPath)
	}

	buildSucceeded = true
	c.cmd.Printf("Target '%s' built at commit %s\n", target, headCommit.ShortHash)
	c.cmd.Printf("Run with: nigiri run %s %s\n", target, headCommit.ShortHash)
	return nil
}

// selectBuildCommand returns the build command to run for goos, falling back to
// the configuration's `defaults` section when the target defines no command of
// its own.
//
// Parameters:
//   - target: The target's own build command configuration
//   - defaults: The configuration-wide default build commands
//   - goos: The operating system to select the command for
//
// Returns:
//   - string: The command to run
//   - error: An error when the OS is unsupported or no command is configured
func selectBuildCommand(target, defaults cfgmodel.BuildCommand, goos string) (string, error) {
	forOS := func(bc cfgmodel.BuildCommand) (string, bool) {
		switch goos {
		case "linux":
			return bc.Linux, true
		case "windows":
			return bc.Windows, true
		case "darwin":
			return bc.Darwin, true
		default:
			return "", false
		}
	}

	cmd, supported := forOS(target)
	if !supported {
		return "", logger.CreateErrorf("unsupported OS: %s", goos)
	}
	if cmd == "" {
		cmd, _ = forOS(defaults)
	}
	if cmd == "" {
		return "", logger.CreateErrorf("no build command specified for OS: %s", goos)
	}
	return cmd, nil
}

// buildWaitDelay bounds how long Wait blocks after the build shell has exited
// or the build context has been cancelled. Without it, grandchildren that
// inherited the output pipes keep the build hanging indefinitely.
const buildWaitDelay = 5 * time.Second

// runBuildCommand runs the build command in a shell under ctx. The shell is
// placed in its own process group so cancellation reaches the whole build
// process tree instead of only the shell, and waitDelay bounds the wait for
// output pipes that orphaned descendants may still hold open.
//
// Parameters:
//   - ctx: The context bounding the build (carries the --timeout deadline)
//   - command: The shell command to run
//   - stdout: Writer receiving the command's standard output
//   - stderr: Writer receiving the command's standard error
//   - env: The environment for the command (nil inherits the current one)
//   - waitDelay: The grace period before abandoning blocked output pipes
//
// Returns:
//   - error: Any error encountered while running the build command
func runBuildCommand(ctx context.Context, command string, stdout, stderr io.Writer, env []string, waitDelay time.Duration) error {
	shell, shellArgs := buildShell()
	execCmd := exec.CommandContext(ctx, shell, append(shellArgs, command)...)
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr
	execCmd.Env = env
	configureBuildProcess(execCmd, waitDelay)

	err := execCmd.Run()
	// A wait-delay expiry on an otherwise clean run means the build command
	// itself succeeded and only orphaned descendants still held the output
	// pipes open, which is not a build failure.
	if errors.Is(err, exec.ErrWaitDelay) && ctx.Err() == nil {
		logger.Warnf("Build command left background processes holding its output; continued after %s", waitDelay)
		return nil
	}
	return err
}

// copyFile copies a file from src to dst. A source above the size limit is
// rejected rather than copied truncated, since the result would be reported as
// a successful build and then executed.
func copyFile(src, dst string) error {
	// Get file info up front: the size decides whether the copy may proceed
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	if info.Size() > maxFileSizeForArchive {
		return fmt.Errorf("file %s exceeds the %d byte size limit", src, int64(maxFileSizeForArchive))
	}

	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			logger.Warnf("failed to close source file %s: %v", src, err)
		}
	}()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			logger.Warnf("failed to close destination file %s: %v", dst, err)
		}
	}()

	// Copy file contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(dst, info.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// maxFileSizeForArchive is the maximum file size allowed in archives (1GB)
const maxFileSizeForArchive = 1 << 30

// wrapClose annotates a non-nil Close error with msg, and returns nil otherwise
func wrapClose(msg string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// compressDirectory compresses a directory into a tar.gz file
func compressDirectory(srcDir, tarGzPath string) (err error) {
	// Create tar.gz file
	tarGzFile, createErr := os.Create(tarGzPath)
	if createErr != nil {
		return fmt.Errorf("failed to create tar.gz file: %w", createErr)
	}

	gzipWriter := gzip.NewWriter(tarGzFile)
	tarWriter := tar.NewWriter(gzipWriter)

	// Closing is where the tar trailer and the gzip footer are flushed, so a
	// short write only surfaces there. The caller deletes the source tree when
	// this reports success, so those errors must reach it.
	defer func() {
		err = errors.Join(err,
			wrapClose("failed to finalize tar archive", tarWriter.Close()),
			wrapClose("failed to finalize gzip stream", gzipWriter.Close()),
			wrapClose("failed to close tar.gz file", tarGzFile.Close()),
		)
	}()

	// Walk through directory and add files to tar
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Resolve the link target for symlinks so it is recorded in the header.
		// filepath.Walk uses Lstat, so info describes the link itself.
		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, linkTarget)
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

		// Reject oversized files rather than truncating them: the header
		// declares the full size, so a short body corrupts the archive.
		if info.Mode().IsRegular() && info.Size() > maxFileSizeForArchive {
			return fmt.Errorf("file %s exceeds the %d byte archive size limit", path, int64(maxFileSizeForArchive))
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// Only regular files carry content; directories and symlinks are
		// represented by their header alone. Writing content for a symlink
		// would follow the link and corrupt the archive.
		if !info.Mode().IsRegular() {
			return nil
		}

		// Add file to tar using helper function to avoid defer accumulation in Walk
		if err := addFileToTar(tarWriter, path, info.Size()); err != nil {
			return err
		}

		return nil
	})
}

// addFileToTar adds a single file to the tar archive with proper resource cleanup.
// Files above the archive size limit are rejected rather than truncated, because
// a body shorter than the declared header size desynchronizes the archive.
//
// Parameters:
//   - tarWriter: The archive to write the file body to
//   - path: The file to add
//   - size: The size declared in the already-written tar header
//
// Returns:
//   - error: Any error encountered while adding the file
func addFileToTar(tarWriter *tar.Writer, path string, size int64) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warnf("failed to close file %s: %v", path, err)
		}
	}()

	written, err := io.Copy(tarWriter, file)
	if err != nil {
		return fmt.Errorf("failed to write file to tar: %w", err)
	}
	if written != size {
		return fmt.Errorf("file %s changed while archiving: wrote %d of %d bytes", path, written, size)
	}

	return nil
}
