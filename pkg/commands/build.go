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

const defaultBuildTimeoutMinutes = 30

// buildTimeoutUnit is a variable so tests can exercise --timeout without
// waiting for whole minutes.
var buildTimeoutUnit = time.Minute

type buildCommand struct {
	cmd        *cobra.Command
	commit     string
	depth      int
	verbose    bool
	forceBuild bool
	useToken   bool
	// timeout is in minutes; 0 disables it.
	timeout int
}

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
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
	flags.BoolVarP(&c.forceBuild, "force", "f", false,
		"Force rebuild even if the target has already been built at the specified commit")
	flags.BoolVarP(&c.useToken, "use-token", "t", false,
		"Use GitHub token for authentication (required for private repositories)")
	flags.IntVar(&c.timeout, "timeout", defaultBuildTimeoutMinutes, "Build timeout in minutes (0 = no timeout)")

	c.cmd = cmd

	return c
}

func (c *buildCommand) getCompletionTargets(prefix string) []string {
	return getConfiguredTargets(prefix)
}

// resolveCloneDepth falls back to a full clone (depth 0) when a commit is
// requested: a shallow clone only contains the default branch HEAD, so it
// cannot resolve an arbitrary commit.
func resolveCloneDepth(depth int, commit string) int {
	if commit != "" {
		return 0
	}

	return depth
}

type buildJob struct {
	git       *vcsutils.Git
	target    string
	commitDir string
	commit    commits.Commit
	defaults  cfgmodel.BuildCommand
	cfg       cfgmodel.Target
}

func (c *buildCommand) executeBuild(target string) error {
	targetCfg, defaults, err := loadBuildTarget(target)
	if err != nil {
		return err
	}

	targetRootDir, err := ensureTargetRootDir(target)
	if err != nil {
		return err
	}

	git := &vcsutils.Git{Source: targetCfg.Sources}

	headCommit, err := c.resolveHeadCommit(git, targetCfg)
	if err != nil {
		return err
	}

	isExistCommitDir := targets.IsExistTargetCommitDir(targetRootDir, headCommit)
	if isExistCommitDir && !c.forceBuild {
		c.cmd.Printf("Commit %s has already been built. Use --force to rebuild.\n", headCommit.ShortHash)

		return nil
	}

	commitDir, err := c.prepareCommitDir(targetRootDir, headCommit, isExistCommitDir)
	if err != nil {
		return err
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

	job := &buildJob{
		git: git, target: target, commitDir: commitDir, commit: headCommit, defaults: defaults, cfg: targetCfg,
	}
	if err := c.buildInCommitDir(job); err != nil {
		return err
	}

	buildSucceeded = true

	c.cmd.Printf("Target '%s' built at commit %s\n", target, headCommit.ShortHash)
	c.cmd.Printf("Run with: nigiri run %s %s\n", target, headCommit.ShortHash)

	return nil
}

func loadBuildTarget(target string) (cfgmodel.Target, cfgmodel.BuildCommand, error) {
	cm := newConfigManager()
	if err := cm.LoadCfgFile(); err != nil {
		return cfgmodel.Target{}, cfgmodel.BuildCommand{}, logger.CreateErrorf("failed to load configuration: %w", err)
	}

	targetCfg, exists := cm.Config.GetTarget(target)
	if !exists {
		return cfgmodel.Target{}, cfgmodel.BuildCommand{},
			logger.CreateErrorf("target '%s' not found in configuration", target)
	}

	return targetCfg, cm.Config.Defaults, nil
}

func ensureTargetRootDir(target string) (string, error) {
	fsTarget := targets.Target{Target: target}

	if _, err := fsTarget.CreateTargetRootDirIfNotExist(nigiriRoot); err != nil {
		return "", logger.CreateErrorf("failed to create target directory: %w", err)
	}

	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return "", logger.CreateErrorf("failed to get target directory: %w", err)
	}

	return targetRootDir, nil
}

func (c *buildCommand) resolveHeadCommit(git *vcsutils.Git, targetCfg cfgmodel.Target) (commits.Commit, error) {
	headCommit := commits.Commit{Hash: c.commit}

	if c.commit == "" {
		defaultBranch := targetCfg.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		c.cmd.Printf("Getting HEAD of branch '%s' from %s...\n", defaultBranch, targetCfg.Sources)

		if err := git.GetDefaultBranchRemoteHead(defaultBranch); err != nil {
			return commits.Commit{}, logger.CreateErrorf("failed to get HEAD of branch '%s': %w", defaultBranch, err)
		}

		if git.Branch != defaultBranch {
			logger.Warnf("Branch '%s' not found; using the remote default branch '%s'", defaultBranch, git.Branch)
		}

		headCommit.Hash = git.HEAD
	} else {
		c.cmd.Printf("Using specified commit: %s\n", c.commit)
	}

	if err := headCommit.CalculateShortHash(); err != nil {
		return commits.Commit{}, logger.CreateErrorf("failed to calculate short hash: %w", err)
	}

	if err := headCommit.Validate(); err != nil {
		return commits.Commit{}, logger.CreateErrorf("invalid commit: %w", err)
	}

	return headCommit, nil
}

// prepareCommitDir reuses the directory of a forced rebuild, dropping only its
// source tree so the previous artifacts survive a failed rebuild.
func (c *buildCommand) prepareCommitDir(targetRootDir string, headCommit commits.Commit, exists bool) (string, error) {
	if !exists {
		commitDir, err := targets.CreateTargetCommitDir(targetRootDir, headCommit)
		if err != nil {
			return "", logger.CreateErrorf("failed to create commit directory: %w", err)
		}

		return commitDir, nil
	}

	commitDir := filepath.Join(targetRootDir, headCommit.ShortHash)
	c.cmd.Printf("Force rebuilding commit %s\n", headCommit.ShortHash)

	if err := os.RemoveAll(filepath.Join(commitDir, "src")); err != nil {
		return "", logger.CreateErrorf("failed to clean src directory: %w", err)
	}

	return commitDir, nil
}

func (c *buildCommand) buildInCommitDir(job *buildJob) error {
	cwd, err := os.Getwd()
	if err != nil {
		return logger.CreateErrorf("failed to get current working directory: %w", err)
	}
	defer func() {
		if dirErr := os.Chdir(cwd); dirErr != nil {
			logger.Warnf("Failed to change back to original directory: %v", dirErr)
		}
	}()

	if err := os.Chdir(job.commitDir); err != nil {
		return logger.CreateErrorf("failed to change to commit directory: %w", err)
	}

	logDir := filepath.Join(job.commitDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return logger.CreateErrorf("failed to create log directory: %w", err)
	}

	cloneDir, cloneDuration, err := c.cloneSource(job)
	if err != nil {
		return err
	}

	workDir, err := enterWorkDir(cloneDir, job.cfg.WorkingDirectory)
	if err != nil {
		return err
	}

	cmd, err := selectBuildCommand(job.cfg.BuildCommand, job.defaults, runtime.GOOS)
	if err != nil {
		return err
	}

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

	buildDuration, buildErr := c.runBuild(job, cmd, buildLogFile)
	if buildErr != nil {
		return logger.CreateErrorf("build failed: %w\nSee build log at %s", buildErr, buildLogPath)
	}

	// Only successful builds are recorded, so the build date always describes
	// the artifacts actually present in the directory.
	writeBuildMetadata(job, cloneDuration, buildDuration)
	collectArtifacts(job, workDir, cloneDir)

	return nil
}

func (c *buildCommand) cloneSource(job *buildJob) (string, time.Duration, error) {
	cloneStartTime := time.Now()
	cloneDir := filepath.Join(job.commitDir, "src")
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
	if err := job.git.Clone(cloneDir, cloneOptions); err != nil {
		return "", 0, logger.CreateErrorf("failed to clone repository: %w", err)
	}

	// A requested commit is always checked out so the build never silently
	// uses the default branch HEAD instead.
	if c.commit != "" {
		c.cmd.Printf("Checking out commit %s...\n", c.commit)

		if err := job.git.Checkout(cloneDir, c.commit); err != nil {
			return "", 0, logger.CreateErrorf("failed to checkout commit %s: %w", c.commit, err)
		}
	}

	cloneDuration := time.Since(cloneStartTime)
	c.cmd.Printf("Repository cloned in %s\n", cloneDuration)

	return cloneDir, cloneDuration, nil
}

func enterWorkDir(cloneDir, workingDirectory string) (string, error) {
	workDir := cloneDir
	if workingDirectory != "" {
		workDir = filepath.Join(cloneDir, workingDirectory)
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			return "", logger.CreateErrorf("working directory '%s' not found in source", workingDirectory)
		}
	}

	if err := os.Chdir(workDir); err != nil {
		return "", logger.CreateErrorf("failed to change to working directory: %w", err)
	}

	return workDir, nil
}

func (c *buildCommand) runBuild(job *buildJob, cmd string, buildLog io.Writer) (time.Duration, error) {
	c.cmd.Printf("Building target '%s' with command: %s\n", job.target, cmd)

	if c.timeout > 0 {
		c.cmd.Printf("Build timeout: %d minutes\n", c.timeout)
	}

	buildStartTime := time.Now()

	ctx, cancel := c.buildContext()
	defer cancel()

	stdout, stderr := buildLog, buildLog
	if c.verbose {
		stdout = io.MultiWriter(os.Stdout, buildLog)
		stderr = io.MultiWriter(os.Stderr, buildLog)
	}

	var env []string
	if len(job.cfg.Env) > 0 {
		env = append(os.Environ(), job.cfg.Env...)
	}

	buildErr := runBuildCommand(ctx, cmd, stdout, stderr, env, buildWaitDelay)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		buildErr = logger.CreateErrorf("build timed out after %d minutes", c.timeout)
	}

	return time.Since(buildStartTime), buildErr
}

func (c *buildCommand) buildContext() (context.Context, context.CancelFunc) {
	if c.timeout > 0 {
		return context.WithTimeout(context.Background(), time.Duration(c.timeout)*buildTimeoutUnit)
	}

	return context.Background(), func() {}
}

func writeBuildMetadata(job *buildJob, cloneDuration, buildDuration time.Duration) {
	metadata := fmt.Sprintf("Target: %s\nCommit: %s\nShort hash: %s\n%s %s\n"+
		"Clone duration: %s\nBuild duration: %s\nOS: %s\nArchitecture: %s\n",
		job.target, job.commit.Hash, job.commit.ShortHash,
		buildDateField, time.Now().Format(time.RFC3339),
		cloneDuration, buildDuration, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(job.commitDir, buildInfoFileName), []byte(metadata), 0o644); err != nil {
		logger.Warnf("Failed to write build metadata: %v", err)
	}
}

// collectArtifacts keeps the source archive whenever no binary was captured:
// dropping the source without a binary would leave nothing runnable behind.
func collectArtifacts(job *buildJob, workDir, cloneDir string) {
	binaryCaptured := captureBinary(job.cfg.BuildCommand, workDir, job.commitDir)

	if job.cfg.BinaryOnly && binaryCaptured {
		if err := os.RemoveAll(cloneDir); err != nil {
			logger.Warnf("Failed to remove source directory: %v", err)
		}

		return
	}

	if job.cfg.BinaryOnly {
		logger.Warnf("binary-only is set for target '%s' but no binary was captured; keeping the source archive",
			job.target)
	}

	archiveSource(cloneDir, job.commitDir)
}

func captureBinary(buildCmd cfgmodel.BuildCommand, workDir, commitDir string) bool {
	binaryPath, hasBinaryPath := buildCmd.BinaryPath()
	if !hasBinaryPath {
		return false
	}

	sourceFile := filepath.Join(workDir, binaryPath)
	destFile := filepath.Join(commitDir, "bin")

	if err := os.MkdirAll(filepath.Dir(destFile), 0o755); err != nil {
		logger.Warnf("Failed to create bin directory: %v", err)

		return false
	}

	if err := copyFile(sourceFile, destFile); err != nil {
		logger.Warnf("Failed to copy binary: %v", err)

		return false
	}

	return true
}

func archiveSource(cloneDir, commitDir string) {
	srcTarGzPath := filepath.Join(commitDir, "source.tar.gz")
	if err := compressDirectory(cloneDir, srcTarGzPath); err != nil {
		logger.Warnf("Failed to compress source directory (keeping the source tree): %v", err)
		// A partial archive would fail to extract later; drop it.
		if rmErr := os.Remove(srcTarGzPath); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.Warnf("Failed to remove incomplete archive %s: %v", srcTarGzPath, rmErr)
		}

		return
	}

	if err := os.RemoveAll(cloneDir); err != nil {
		logger.Warnf("Failed to remove source directory after compression: %v", err)
	}
}

// selectBuildCommand falls back to the configuration's `defaults` section when
// the target defines no command of its own for goos.
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

// runBuildCommand places the shell in its own process group so cancellation
// reaches the whole build process tree instead of only the shell, and
// waitDelay bounds the wait for output pipes that orphaned descendants may
// still hold open. A nil env inherits the current environment.
func runBuildCommand(
	ctx context.Context, command string, stdout, stderr io.Writer, env []string, waitDelay time.Duration,
) error {
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

	return err //nolint:wrapcheck // the caller reports it as "build failed"
}

// copyFile rejects a source above the size limit rather than copying it
// truncated, since the result would be reported as a successful build and then
// executed.
func copyFile(src, dst string) error {
	// Get file info up front: the size decides whether the copy may proceed
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if info.Size() > maxFileSizeForArchive {
		return fmt.Errorf("file %s exceeds the %d byte size limit", src, int64(maxFileSizeForArchive))
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			logger.Warnf("failed to close source file %s: %v", src, err)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			logger.Warnf("failed to close destination file %s: %v", dst, err)
		}
	}()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if err := os.Chmod(dst, info.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

const maxFileSizeForArchive = 1 << 30

func wrapClose(msg string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", msg, err)
}

func compressDirectory(srcDir, tarGzPath string) (err error) {
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

	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return addEntryToTar(tarWriter, srcDir, path, info)
	})
	if walkErr != nil {
		return fmt.Errorf("failed to archive %s: %w", srcDir, walkErr)
	}

	return nil
}

func addEntryToTar(tarWriter *tar.Writer, srcDir, path string, info os.FileInfo) error {
	// filepath.Walk uses Lstat, so info describes a symlink itself and its
	// target has to be read separately to be recorded in the header.
	var linkTarget string

	if info.Mode()&os.ModeSymlink != 0 {
		var err error

		linkTarget, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed to read symlink: %w", err)
		}
	}

	header, err := tar.FileInfoHeader(info, linkTarget)
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}

	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	header.Name = relPath

	if relPath == "." {
		return nil
	}

	// Reject oversized files rather than truncating them: the header
	// declares the full size, so a short body corrupts the archive.
	if info.Mode().IsRegular() && info.Size() > maxFileSizeForArchive {
		return fmt.Errorf("file %s exceeds the %d byte archive size limit", path, int64(maxFileSizeForArchive))
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Only regular files carry content; directories and symlinks are
	// represented by their header alone. Writing content for a symlink
	// would follow the link and corrupt the archive.
	if !info.Mode().IsRegular() {
		return nil
	}

	return addFileToTar(tarWriter, path, info.Size())
}

// addFileToTar is a separate function so the deferred Close runs per file
// instead of accumulating across the Walk. A body that differs from size, the
// length already declared in the tar header, would desynchronize the archive.
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
