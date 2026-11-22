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

	cfgmodel "github.com/oota-sushikuitee/nigiri/internal/models/config"
	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
	"github.com/spf13/cobra"
)

type runCommand struct {
	cmd *cobra.Command
}

const runLongHelp = `Run a built target with optional arguments.
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
`

func newRunCommand() *runCommand {
	c := &runCommand{}
	cmd := &cobra.Command{
		Use:                "run target [commit] [args...]",
		Short:              "Run a built target",
		Long:               runLongHelp,
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
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

type runInvocation struct {
	target string
	// commitHash is empty for the latest build.
	commitHash string
	configFile string
	targetArgs []string
	help       bool
}

// parseRunArgs exists because flag parsing is disabled for `nigiri run` so that
// target flags pass through untouched, which means nigiri's global flags have to
// be recognised here; they are only accepted before the target name, so anything
// after it still belongs to the target. Everything that is neither the target
// nor a commit hash is forwarded, including arguments preceding a "--".
func parseRunArgs(args []string) (runInvocation, error) {
	var inv runInvocation

	before, afterDash := splitAtDoubleDash(args)

	i, err := consumeRunFlags(before, &inv)
	if err != nil {
		return inv, err
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

// splitAtDoubleDash separates the arguments after the first "--", which
// always belong to the target.
func splitAtDoubleDash(args []string) ([]string, []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}

	return args, nil
}

// consumeRunFlags records nigiri's own leading flags in inv and returns the
// index of the first argument that is not one of them: the target name.
func consumeRunFlags(args []string, inv *runInvocation) (int, error) {
	i := 0

	for i < len(args) {
		arg := args[i]

		switch {
		case arg == "--help" || arg == "-h":
			inv.help = true
			i++
		case arg == "--config" || arg == "-c":
			if i+1 >= len(args) {
				return i, logger.CreateErrorf("flag needs an argument: %s", arg)
			}

			inv.configFile = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--config="):
			inv.configFile = strings.TrimPrefix(arg, "--config=")
			i++
		case strings.HasPrefix(arg, "-c="):
			inv.configFile = strings.TrimPrefix(arg, "-c=")
			i++
		default:
			return i, nil
		}
	}

	return i, nil
}

func (c *runCommand) getCompletionTargets(prefix string) []string {
	return getConfiguredTargets(prefix)
}

func (c *runCommand) getCompletionCommits(target, prefix string) []string {
	return getTargetCommits(target, prefix)
}

func (c *runCommand) executeRun(target, commitHash string, args []string) error {
	fsTarget := targets.Target{Target: target}

	targetRootDir, err := fsTarget.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return err
	}

	runDir, err := c.resolveRunDir(targetRootDir, target, commitHash)
	if err != nil {
		return err
	}

	cm := newConfigManager()
	if err := cm.LoadCfgFile(); err != nil {
		return logger.CreateErrorf("failed to load config: %w", err)
	}

	targetCfg, exists := cm.Config.GetTarget(target)
	if !exists {
		return logger.CreateErrorf("target '%s' not found in configuration", target)
	}

	binaryPath, err := c.locateBinary(runDir, target, targetCfg)
	if err != nil {
		return err
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return logger.CreateErrorf("binary not found at %s", binaryPath)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0o755); err != nil {
			return logger.CreateErrorf("failed to make binary executable: %w", err)
		}
	}

	return c.runBinary(binaryPath, args, targetCfg.Env)
}

// resolveRunDir picks the newest build by recorded build date when commitHash
// is empty, and otherwise the build directory matching commitHash.
func (c *runCommand) resolveRunDir(targetRootDir, target, commitHash string) (string, error) {
	if commitHash == "" {
		latestDir, err := latestBuildDir(targetRootDir)
		if err != nil {
			return "", err
		}

		if latestDir == "" {
			return "", logger.CreateErrorf("no builds found for target %s", target)
		}

		c.cmd.Printf("Using latest commit: %s\n", latestDir)

		return filepath.Join(targetRootDir, latestDir), nil
	}

	if len(commitHash) < commits.ShortHashLength {
		return "", logger.CreateErrorf("commit hash is too short: %s (minimum %d characters)",
			commitHash, commits.ShortHashLength)
	}

	matchingDirs, err := matchingBuildDirs(targetRootDir, commitHash)
	if err != nil {
		return "", err
	}

	if len(matchingDirs) == 0 {
		return "", logger.CreateErrorf("no build found for commit %s", commitHash)
	}

	return filepath.Join(targetRootDir, matchingDirs[0]), nil
}

func latestBuildDir(targetRootDir string) (string, error) {
	dirs, err := os.ReadDir(targetRootDir)
	if err != nil {
		return "", logger.CreateErrorf("failed to read target directory: %w", err)
	}

	var (
		latestDir   string
		latestBuilt time.Time
	)

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

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

	return latestDir, nil
}

func matchingBuildDirs(targetRootDir, commitHash string) ([]string, error) {
	dirs, err := os.ReadDir(targetRootDir)
	if err != nil {
		return nil, logger.CreateErrorf("failed to read target directory: %w", err)
	}

	var matching []string

	for _, dir := range dirs {
		if dir.IsDir() && commitDirMatches(dir.Name(), commitHash) {
			matching = append(matching, dir.Name())
		}
	}

	return matching, nil
}

// locateBinary prefers the binary captured at build time and otherwise looks
// in the source tree, extracting the source archive first if needed.
func (c *runCommand) locateBinary(runDir, target string, targetCfg cfgmodel.Target) (string, error) {
	binaryPath := filepath.Join(runDir, "bin")
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		return binaryPath, nil
	}

	c.cmd.Printf("Binary not found in commit/bin directory, looking for alternative locations...\n")

	srcDir, err := c.ensureSourceTree(runDir)
	if err != nil {
		return "", err
	}

	workDir := srcDir
	if targetCfg.WorkingDirectory != "" {
		workDir = filepath.Join(srcDir, targetCfg.WorkingDirectory)
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			return "", logger.CreateErrorf("working directory '%s' not found in source", targetCfg.WorkingDirectory)
		}
	}

	if binPath, ok := targetCfg.BuildCommand.BinaryPath(); ok {
		return filepath.Join(workDir, binPath), nil
	}

	return binaryCandidate(workDir, target), nil
}

// ensureSourceTree extracts the source archive into src rather than into the
// commit directory: archive entries are stored relative to the source tree
// root, so extracting one level up would scatter the repository over the build
// artifacts.
func (c *runCommand) ensureSourceTree(runDir string) (string, error) {
	srcArchive := filepath.Join(runDir, "source.tar.gz")
	srcDir := filepath.Join(runDir, "src")

	if _, err := os.Stat(srcArchive); err == nil {
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			if err := c.extractSource(srcArchive, srcDir); err != nil {
				return "", err
			}
		}
	}

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return "", logger.CreateErrorf("source directory not found: %s", srcDir)
	}

	return srcDir, nil
}

func (c *runCommand) extractSource(srcArchive, srcDir string) error {
	c.cmd.Printf("Extracting source archive...\n")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return logger.CreateErrorf("failed to create source directory: %w", err)
	}

	if err := extractTarGz(srcArchive, srcDir); err != nil {
		// A half-extracted tree would make the next run skip extraction.
		if rmErr := os.RemoveAll(srcDir); rmErr != nil {
			logger.Warnf("Failed to remove incomplete source directory %s: %v", srcDir, rmErr)
		}

		return logger.CreateErrorf("failed to extract source archive: %w", err)
	}

	return nil
}

// binaryCandidate returns the first existing of <target>, bin/<target>, and
// build/<target> under workDir, or <target> when none exists.
func binaryCandidate(workDir, target string) string {
	binaryPath := filepath.Join(workDir, target)
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		return binaryPath
	}

	for _, dir := range []string{"bin", "build"} {
		altPath := filepath.Join(workDir, dir, target)
		if _, err := os.Stat(altPath); err == nil {
			return altPath
		}
	}

	return binaryPath
}

func (c *runCommand) runBinary(binaryPath string, args, env []string) error {
	cmd := exec.CommandContext(context.Background(), binaryPath, args...)
	cmd.Stdout = c.cmd.OutOrStdout()
	cmd.Stderr = c.cmd.ErrOrStderr()
	cmd.Stdin = os.Stdin
	cmd.Dir = filepath.Dir(binaryPath)

	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	c.cmd.Printf("Running %s with args: %v\n", binaryPath, args)

	if err := cmd.Run(); err != nil {
		// The target ran and failed on its own terms: report its status so the
		// caller can distinguish it from nigiri failing to start the target.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
			return &ExitCodeError{Code: exitErr.ExitCode()}
		}

		return err //nolint:wrapcheck // exec errors already name the binary
	}

	return nil
}

// commitDirMatches matches in both directions because build directories are
// named after the 7-char short hash, so a query can be either a prefix of that
// name or a longer hash (typically the full 40-char one) that the name is a
// prefix of.
func commitDirMatches(dirName, commitHash string) bool {
	if dirName == "" || commitHash == "" {
		return false
	}

	return strings.HasPrefix(dirName, commitHash) || strings.HasPrefix(commitHash, dirName)
}

const (
	maxFileSizeForExtract = 1 << 30
	extractDirPerm        = 0o755
)

func extractTarGz(tarGzPath, destDir string) error {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warnf("failed to close archive file: %v", err)
		}
	}()

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

		if err := extractTarEntry(tarReader, root, header); err != nil {
			return err
		}
	}

	return nil
}

func extractTarEntry(tarReader *tar.Reader, root *os.Root, header *tar.Header) error {
	// Joining with "." turns absolute names into root-relative ones.
	name := filepath.Join(".", header.Name) //nolint:gosec // G305: root confines every path to destDir
	if header.Typeflag != tar.TypeDir {
		if err := root.MkdirAll(filepath.Dir(name), extractDirPerm); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", header.Name, err)
		}
	}

	switch header.Typeflag {
	case tar.TypeDir:
		if err := root.MkdirAll(name, extractDirPerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	case tar.TypeSymlink:
		if err := root.Symlink(header.Linkname, name); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	case tar.TypeLink:
		//nolint:gosec // G305: root confines every path to destDir
		if err := root.Link(filepath.Join(".", header.Linkname), name); err != nil {
			return fmt.Errorf("failed to create hard link: %w", err)
		}
	default:
		return extractFileFromTar(tarReader, root, name, header.FileInfo().Mode().Perm(), header.Size)
	}

	return nil
}

// extractFileFromTar is a separate function so the deferred Close runs per
// entry. Entries above the size limit are rejected rather than written
// truncated, since the result would be reported as a valid extraction.
func extractFileFromTar(
	tarReader *tar.Reader, root *os.Root, filePath string, mode os.FileMode, size int64,
) error {
	if size > maxFileSizeForExtract {
		return fmt.Errorf("archive entry %s exceeds the %d byte extraction size limit",
			filePath, int64(maxFileSizeForExtract))
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

	if err := file.Chmod(mode); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}
