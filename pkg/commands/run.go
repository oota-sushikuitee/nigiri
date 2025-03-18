package commands

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/config"
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
			if len(args) < 1 {
				return cmd.Help()
			}

			target := args[0]
			var commitHash string
			var targetArgs []string

			// Parse arguments to separate commit and target args
			if len(args) > 1 {
				// Look for "--" in args to find target arguments explicitly
				dashIndex := -1
				for i, arg := range args {
					if arg == "--" {
						dashIndex = i
						break
					}
				}

				if dashIndex != -1 {
					// "--" was found for explicit separation
					if dashIndex > 1 {
						// There is a commit hash between target and "--"
						commitHash = args[1]
					}
					if dashIndex < len(args)-1 {
						// There are args after "--"
						targetArgs = args[dashIndex+1:]
					}
				} else {
					// "--" not found, but we still need to handle arguments
					// Second argument could be a commit hash or a flag
					secondArg := args[1]

					// If the second argument starts with "-", it's a flag/option for the target
					// Or if it's HEAD/head, treat it as a commit hash
					if strings.HasPrefix(secondArg, "-") {
						// It's a flag, so no commit hash specified
						commitHash = ""       // Use latest commit
						targetArgs = args[1:] // All args after target are for the target program
					} else {
						// Not a flag, so it's a commit hash (or HEAD)
						commitHash = secondArg

						// If there are more arguments, they are passed to the target
						if len(args) > 2 {
							targetArgs = args[2:]
						}
					}
				}
			}

			// Handle HEAD/head alias for the latest commit
			if strings.ToUpper(commitHash) == "HEAD" {
				// HEAD alias is specified, so set empty string to use the latest commit
				commitHash = ""
				cmd.Printf("Using HEAD (latest commit)\n")
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
				// Add HEAD to the list of completions
				completions := c.getCompletionCommits(args[0], toComplete)
				if strings.HasPrefix(strings.ToUpper(toComplete), "HEAD") {
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

// getCompletionTargets returns a list of available targets for command completion
func (c *runCommand) getCompletionTargets(prefix string) []string {
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

// getCompletionCommits returns a list of available commit hashes for the specified target
func (c *runCommand) getCompletionCommits(target, prefix string) []string {
	fsTarget := targets.Target{
		Target:  target,
		Commits: commits.Commits{},
	}
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
			if dir.IsDir() && strings.HasPrefix(dir.Name(), commitHash) {
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
	cm := config.NewConfigManager()
	if err := cm.LoadCfgFile(); err != nil {
		return logger.CreateErrorf("failed to load config: %w", err)
	}
	targetCfg, exists := cm.Config.Targets[target]
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

		// If source archive exists but src directory doesn't, extract it
		if _, err := os.Stat(srcArchive); err == nil {
			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				c.cmd.Printf("Extracting source archive...\n")
				if err := extractTarGz(srcArchive, runDir); err != nil {
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

	// Make sure binary is executable
	if runtime := os.Getenv("GOOS"); runtime != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return logger.CreateErrorf("failed to make binary executable: %w", err)
		}
	}

	// Setup command execution with proper argument handling
	cmd := exec.Command(binaryPath, args...)
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
	return cmd.Run()
}

// extractTarGz extracts a tar.gz file to the specified directory
func extractTarGz(tarGzPath, destDir string) error {
	// Open the tar.gz file
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract each file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reading error: %w", err)
		}

		// Get file path
		filePath := filepath.Join(destDir, header.Name)

		// Create directories if needed
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Make sure parent directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Create file
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy contents
		if _, err := io.Copy(file, tarReader); err != nil {
			file.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		file.Close()

		// Set file permissions
		if err := os.Chmod(filePath, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	return nil
}
