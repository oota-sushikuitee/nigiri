//go:build !windows

package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initSourceRepo creates a local git repository holding an executable script
// named after the "sample" target, and returns the repository path and the
// commit hash of its single commit.
func initSourceRepo(t *testing.T, script string) (repoDir, head string) {
	t.Helper()
	repoDir = t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "sample"), []byte(script), 0o755))
	_, err = worktree.Add("sample")
	require.NoError(t, err)

	hash, err := worktree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)
	return repoDir, hash.String()
}

// TestBuildRunRoundTrip_ExtractsSourceIntoSrcDir builds a target whose only
// artifact is the source archive and then runs it. The archive stores paths
// relative to the source tree, so it has to be extracted into the commit
// directory's src subdirectory; extracting it one level up scattered the
// repository over the build artifacts and made every such run fail.
func TestBuildRunRoundTrip_ExtractsSourceIntoSrcDir(t *testing.T) {
	repoDir, head := initSourceRepo(t, "#!/bin/sh\necho hello-from-target\n")

	root := t.TempDir()
	cfgPath := writeTestConfig(t, root, repoDir)
	restoreCommandGlobals(t, filepath.Join(root, "nigiri"), cfgPath)

	build := newBuildCommand()
	build.cmd.SetOut(io.Discard)
	build.commit = head
	require.NoError(t, build.executeBuild("sample"))

	commitDir := filepath.Join(nigiriRoot, "sample", head[:7])
	require.FileExists(t, filepath.Join(commitDir, "source.tar.gz"))
	require.NoDirExists(t, filepath.Join(commitDir, "src"))

	var out bytes.Buffer
	run := newRunCommand()
	run.cmd.SetOut(&out)
	require.NoError(t, run.executeRun("sample", "", nil))

	assert.FileExists(t, filepath.Join(commitDir, "src", "sample"))
	assert.NoFileExists(t, filepath.Join(commitDir, "sample"))
	assert.Contains(t, out.String(), "hello-from-target")
}

// writeFakeBuild creates a build directory whose binary prints label and whose
// metadata records the given build date.
func writeFakeBuild(t *testing.T, shortHash string, built time.Time, label string) string {
	t.Helper()
	commitDir := filepath.Join(nigiriRoot, "sample", shortHash)
	require.NoError(t, os.MkdirAll(commitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(commitDir, "bin"), []byte("#!/bin/sh\necho "+label+"\n"), 0o755))
	metadata := "Target: sample\n" + buildDateField + " " + built.Format(time.RFC3339) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(commitDir, buildInfoFileName), []byte(metadata), 0o600))
	return commitDir
}

// TestExecuteRun_LatestUsesRecordedBuildDate checks that the newest build is
// resolved from the recorded build date rather than the directory mtime, which
// running a build rewrites.
func TestExecuteRun_LatestUsesRecordedBuildDate(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeTestConfig(t, root, filepath.Join(root, "missing-repo"))
	restoreCommandGlobals(t, filepath.Join(root, "nigiri"), cfgPath)

	now := time.Now()
	newest := writeFakeBuild(t, "aaaaaaa", now, "newest-build")
	older := writeFakeBuild(t, "bbbbbbb", now.Add(-2*time.Hour), "older-build")

	// The older build was run most recently, so its directory has the newest
	// mtime; that must not make it the "latest" build.
	require.NoError(t, os.Chtimes(newest, now.Add(-2*time.Hour), now.Add(-2*time.Hour)))
	require.NoError(t, os.Chtimes(older, now, now))

	var out bytes.Buffer
	run := newRunCommand()
	run.cmd.SetOut(&out)
	require.NoError(t, run.executeRun("sample", "", nil))

	assert.Contains(t, out.String(), "newest-build")
	assert.NotContains(t, out.String(), "older-build")
}

// TestExecuteRun_PropagatesTargetExitCode checks that a target exiting non-zero
// is reported as an ExitCodeError carrying its status, so the CLI can exit with
// the same code instead of a blanket 1.
func TestExecuteRun_PropagatesTargetExitCode(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeTestConfig(t, root, filepath.Join(root, "missing-repo"))
	restoreCommandGlobals(t, filepath.Join(root, "nigiri"), cfgPath)

	commitDir := filepath.Join(nigiriRoot, "sample", "abcdef1")
	require.NoError(t, os.MkdirAll(commitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(commitDir, "bin"), []byte("#!/bin/sh\nexit 3\n"), 0o755))

	run := newRunCommand()
	run.cmd.SetOut(io.Discard)
	run.cmd.SetErr(io.Discard)

	err := run.executeRun("sample", "abcdef1", nil)
	var exitErr *ExitCodeError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 3, exitErr.ExitCode())
}
