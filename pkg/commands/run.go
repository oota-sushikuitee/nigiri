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
	"strings"
	"time"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
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
		Use:   "run target [commit] [args...]",
		Short: "Run a built target",
		Long: `Run a built target with optional arguments.
If commit is not specified, the latest built commit will be used.
You can use HEAD (or head) to explicitly specify the latest commit.
Arguments will be properly passed to the target command:

Examples:
  # Run the latest build of a target
  nigiri run <target>

  # Run a specific commit
  nigiri run <target> <commit>

  # Run with HEAD (latest commit) explicitly
  nigiri run <target> HEAD

  # Run and pass arguments to the target
  nigiri run <target> <commit> arg1 arg2

  # Run with arguments including flags
  nigiri run <target> HEAD -v --flag=value

  # Explicitly separate nigiri arguments from target arguments
  nigiri run <target> <commit> -- -v --flag=value
`,
		DisableFlagParsing: true, // Let us handle the flags manually
		RunE: func(cmd *cobra.Command, args []string) error {
			inv, err := parseRunArgs(args)
			if err != nil {
				return err
			}
			if inv.help || inv.target == "" {
				return cmd.Help()
			}
			// Flag parsing is disabled for this command, so the global --config
			// flag has to be applied by hand.
			if inv.configFile != "" {
				cfgFileFlag = inv.configFile
			}

			// Handle HEAD/head alias for the latest commit
			if strings.EqualFold(inv.commitHash, "HEAD") {
				// HEAD alias is specified, so set empty string to use the latest commit
				inv.commitHash = ""
				cmd.Printf("Using HEAD (latest commit)\n")
			}

			return c.executeRun(inv.target, inv.commitHash, inv.targetArgs)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Offer tab completion for targets if no arguments provided yet
			if len(args) == 0 {
				return c.getCompletionTargets(toComplete), cobra.ShellCompDirectiveNoFileComp
			}

			// If we already have a target, offer commit hash completions
			if len(args) == 1 {
				// Add HEAD to the list of completions
				completions := c.getCompletionCommits(args[0], toComplete)
				// Offer "HEAD" when the user-typed prefix is a prefix of it.
				//nolint:gocritic // arg order is intentional: match a typed prefix against "HEAD"
				if strings.HasPrefix("HEAD", strings.ToUpper(toComplete)) {
					completions = append([]string{"HEAD"}, completions...)
				}
				return completions, cobra.ShellCompDirectiveNoFileComp
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	c.cmd = cmd
	return c
}

// runInvocation is the result of parsing a raw `nigiri run` argument list.
type runInvocation struct {
	// target is the built target to run
	target string
	// commitHash is the requested commit, empty for the latest build
	commitHash string
	// configFile is the path given to the global --config flag, if any
	configFile string
	// targetArgs are the arguments passed through to the target program
	targetArgs []string
	// help reports whether nigiri's own help was requested
	help bool
}

// parseRunArgs splits a raw `nigiri run` argument list into nigiri's own
// arguments and the target's. Flag parsing is disabled for this command so that
// target flags pass through untouched, which means nigiri's global flags have to
// be recognised here; they are only accepted before the target name, so anything
// after it still belongs to the target. Everything that is neither the target
// nor a commit hash is forwarded, including arguments preceding a "--".
//
// Parameters:
//   - args: The raw argument list as received from cobra
//
// Returns:
//   - runInvocation: The parsed invocation
//   - error: An error when a nigiri flag is missing its value
func parseRunArgs(args []string) (runInvocation, error) {
	var inv runInvocation

	// Arguments after the first "--" always belong to the target.
	before := args
	var afterDash []string
	for i, arg := range args {
		if arg == "--" {
			before, afterDash = args[:i], args[i+1:]
			break
		}
	}

	i := 0
consume:
	for i < len(before) {
		arg := before[i]
		switch {
		case arg == "--help" || arg == "-h":
			inv.help = true
			i++
		case arg == "--config" || arg == "-c":
			if i+1 >= len(before) {
				return inv, logger.CreateErrorf("flag needs an argument: %s", arg)
			}
			inv.configFile = before[i+1]
			i += 2
		case strings.HasPrefix(arg, "--config="):
			inv.configFile = strings.TrimPrefix(arg, "--config=")
			i++
		case strings.HasPrefix(arg, "-c="):
			inv.configFile = strings.TrimPrefix(arg, "-c=")
			i++
		default:
			// The first argument that is none of nigiri's own flags is the
			// target name; everything from there on is positional.
			break consume
		}
	}

	rest := before[i:]
	if len(rest) > 0 {
		inv.target = rest[0]
		rest = rest[1:]
	}
	// A commit hash is a positional argument; a flag right after the target is
	// already an argument for the target program.
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		inv.commitHash = rest[0]
		rest = rest[1:]
	}

	inv.targetArgs = append(append([]string{}, rest...), afterDash...)
	if len(inv.targetArgs) == 0 {
		inv.targetArgs = nil
	}
	return inv, nil
}

// getCompletionTargets returns a list of available targets for command completion
func (c *runCommand) getCompletionTargets(prefix string) []string {
	return getConfiguredTargets(prefix)
}

// getCompletionCommits returns a list of available commit hashes for the specified target
func (c *runCommand) getCompletionCommits(target, prefix string) []string {
	return getTargetCommits(target, prefix)
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
	fsTarget := targets.Target{
		Target:  target,
		Commits: commits.Commits{},
	}
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
			return logger.CreateErrorf("failed to read target directory: %w", err)
		}

		var latestDir string
		var latestBuilt time.Time
		for _, dir := range dirs {
			if dir.IsDir() {
				commitDir := filepath.Join(targetRootDir, dir.Name())
				info, err := os.Stat(commitDir)
				if err != nil {
					continue
				}
				built := buildTime(commitDir, info.ModTime())
				if latestDir == "" || built.After(latestBuilt) {
					latestBuilt = built
					latestDir = dir.Name()
				}
			}
		}

		if latestDir == "" {
			return logger.CreateErrorf("no builds found for target %s", target)
		}

		runDir = filepath.Join(targetRootDir, latestDir)
		c.cmd.Printf("Using latest commit: %s\n", latestDir)
	} else {
		// For specified commit
		if len(commitHash) < 7 {
			return logger.CreateErrorf("commit hash is too short: %s (minimum 7 characters)", commitHash)
		}

		// Find directory matching the commit hash
		dirs, err := os.ReadDir(targetRootDir)
		if err != nil {
			return logger.CreateErrorf("failed to read target directory: %w", err)
		}

		var matchingDir string
		for _, dir := range dirs {
			if dir.IsDir() && commitDirMatches(dir.Name(), commitHash) {
				matchingDir = dir.Name()
				break
			}
		}

		if matchingDir == "" {
			return logger.CreateErrorf("no build found for commit %s", commitHash)
		}

		runDir = filepath.Join(targetRootDir, matchingDir)
	}

	// Get configuration for working directory setting
	cm := newConfigManager()
	if err := cm.LoadCfgFile(); err != nil {
		return logger.CreateErrorf("failed to load config: %w", err)
	}
	targetCfg, exists := cm.Config.GetTarget(target)
	if !exists {
		return logger.CreateErrorf("target '%s' not found in configuration", target)
	}

	// Look for the binary in the commit directory first
	binaryPath := filepath.Join(runDir, "bin")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		c.cmd.Printf("Binary not found in commit/bin directory, looking for alternative locations...\n")

		// Check for compressed source
		srcArchive := filepath.Join(runDir, "source.tar.gz")
		srcDir := filepath.Join(runDir, "src")

		// If source archive exists but src directory doesn't, extract it.
		// Archive entries are stored relative to the source tree root, so the
		// archive must be extracted into src rather than into the commit
		// directory, where it would scatter the repository over the build
		// artifacts.
		if _, err := os.Stat(srcArchive); err == nil {
			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				c.cmd.Printf("Extracting source archive...\n")
				if err := os.MkdirAll(srcDir, 0755); err != nil {
					return logger.CreateErrorf("failed to create source directory: %w", err)
				}
				if err := extractTarGz(srcArchive, srcDir); err != nil {
					// A half-extracted tree would make the next run skip extraction.
					if rmErr := os.RemoveAll(srcDir); rmErr != nil {
						logger.Warnf("Failed to remove incomplete source directory %s: %v", srcDir, rmErr)
					}
					return logger.CreateErrorf("failed to extract source archive: %w", err)
				}
			}
		}

		// At this point, we should have a src directory (either it was there or we extracted it)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return logger.CreateErrorf("source directory not found: %s", srcDir)
		}

		// Apply working directory if specified
		workDir := srcDir
		if targetCfg.WorkingDirectory != "" {
			workDir = filepath.Join(srcDir, targetCfg.WorkingDirectory)
			if _, err := os.Stat(workDir); os.IsNotExist(err) {
				return logger.CreateErrorf("working directory '%s' not found in source", targetCfg.WorkingDirectory)
			}
		}

		// Get binary path from config
		if binPath, ok := targetCfg.BuildCommand.BinaryPath(); ok {
			binaryPath = filepath.Join(workDir, binPath)
		} else {
			// Try common locations for the binary
			binaryPath = filepath.Join(workDir, target)
			// If binary not found directly, try common locations
			if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
				// Try bin/ directory
				altPath := filepath.Join(workDir, "bin", target)
				if _, err := os.Stat(altPath); err == nil {
					binaryPath = altPath
				} else {
					// Try build/ directory
					altPath = filepath.Join(workDir, "build", target)
					if _, err := os.Stat(altPath); err == nil {
						binaryPath = altPath
					}
				}
			}
		}
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return logger.CreateErrorf("binary not found at %s", binaryPath)
	}

	// Make sure binary is executable (not needed on Windows)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return logger.CreateErrorf("failed to make binary executable: %w", err)
		}
	}

	// Setup command execution with proper argument handling
	cmd := exec.CommandContext(context.Background(), binaryPath, args...)
	cmd.Stdout = c.cmd.OutOrStdout()
	cmd.Stderr = c.cmd.ErrOrStderr()
	cmd.Stdin = os.Stdin

	// Set working directory to binary's directory
	cmd.Dir = filepath.Dir(binaryPath)

	// Add any environment variables from config
	if len(targetCfg.Env) > 0 {
		cmd.Env = append(os.Environ(), targetCfg.Env...)
	}

	c.cmd.Printf("Running %s with args: %v\n", binaryPath, args)
	if err := cmd.Run(); err != nil {
		// The target ran and failed on its own terms: report its status so the
		// caller can distinguish it from nigiri failing to start the target.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
			return &ExitCodeError{Code: exitErr.ExitCode()}
		}
		return err
	}
	return nil
}

// commitDirMatches reports whether the stored build directory dirName refers to
// the commit the user asked for. Build directories are named after the 7-char
// short hash, so a query can be either a prefix of that name or a longer hash
// (typically the full 40-char one) that the name is a prefix of.
//
// Parameters:
//   - dirName: The name of a build directory under the target root
//   - commitHash: The commit hash supplied by the user
//
// Returns:
//   - bool: True if the directory holds the build for the queried commit
func commitDirMatches(dirName, commitHash string) bool {
	if dirName == "" || commitHash == "" {
		return false
	}
	return strings.HasPrefix(dirName, commitHash) || strings.HasPrefix(commitHash, dirName)
}

// maxFileSizeForExtract is the maximum file size allowed when extracting archives (1GB)
const maxFileSizeForExtract = 1 << 30

// extractTarGz extracts a tar.gz file to the specified directory
func extractTarGz(tarGzPath, destDir string) error {
	// Open the tar.gz file
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warnf("failed to close archive file: %v", err)
		}
	}()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			logger.Warnf("failed to close gzip reader: %v", err)
		}
	}()

	// Every filesystem operation goes through root, which rejects any path
	// (including one reached via a symlink extracted earlier) that resolves
	// outside destDir.
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return fmt.Errorf("failed to open extraction root: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			logger.Warnf("failed to close extraction root: %v", err)
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reading error: %w", err)
		}

		// Joining with "." turns absolute names into root-relative ones.
		name := filepath.Join(".", header.Name)
		if header.Typeflag != tar.TypeDir {
			if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", header.Name, err)
			}
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeSymlink:
			if err := root.Symlink(header.Linkname, name); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
		case tar.TypeLink:
			if err := root.Link(filepath.Join(".", header.Linkname), name); err != nil {
				return fmt.Errorf("failed to create hard link: %w", err)
			}
		default:
			if err := extractFileFromTar(tarReader, root, name, header.Mode, header.Size); err != nil {
				return err
			}
		}
	}

	return nil
}

// extractFileFromTar extracts a single file from the tar reader with proper
// resource cleanup. Entries above the size limit are rejected rather than
// written truncated, since the result would be reported as a valid extraction.
//
// Parameters:
//   - tarReader: The archive positioned at the entry to extract
//   - root: The extraction root the destination is resolved in
//   - filePath: The destination path relative to root
//   - mode: The file mode recorded in the tar header
//   - size: The entry size recorded in the tar header
//
// Returns:
//   - error: Any error encountered while extracting the entry
func extractFileFromTar(tarReader *tar.Reader, root *os.Root, filePath string, mode, size int64) error {
	if size > maxFileSizeForExtract {
		return fmt.Errorf("archive entry %s exceeds the %d byte extraction size limit", filePath, int64(maxFileSizeForExtract))
	}

	file, err := root.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warnf("failed to close file %s: %v", filePath, err)
		}
	}()

	written, err := io.Copy(file, tarReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if written != size {
		return fmt.Errorf("archive entry %s is truncated: wrote %d of %d bytes", filePath, written, size)
	}

	// Set file permissions
	if err := file.Chmod(os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}
