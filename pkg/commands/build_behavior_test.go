//go:build !windows

package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/oota-sushikuitee/nigiri/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initRepoWithFiles commits files (path -> content, all executable) to a new
// local repository and returns its path and the commit hash.
func initRepoWithFiles(t *testing.T, files map[string]string) (string, string) {
	t.Helper()

	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	for name, content := range files {
		path := filepath.Join(repoDir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o755))

		_, err = worktree.Add(name)
		require.NoError(t, err)
	}

	hash, err := worktree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)

	return repoDir, hash.String()
}

// setupSampleTarget writes a config whose "sample" target clones the master
// branch of source and has the given extra YAML lines (indented under the
// target), and points the package globals at a fresh nigiri root.
func setupSampleTarget(t *testing.T, source, extra string) {
	t.Helper()
	setupSampleTargetRaw(t, source, "    default-branch: master\n"+extra)
}

func setupSampleTargetRaw(t *testing.T, source, extra string) {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".nigiri.yml")
	cfg := "targets:\n  sample:\n    source: " + source + "\n" + extra
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o600))
	restoreCommandGlobals(t, filepath.Join(dir, "nigiri"), cfgPath)
}

func buildCommandFor(cmd string) string {
	return "    build-command:\n" +
		"      linux: '" + cmd + "'\n" +
		"      darwin: '" + cmd + "'\n" +
		"      windows: '" + cmd + "'\n"
}

// captureLogger redirects the package logger for the duration of the test.
func captureLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	logger.SetOutput(&buf)
	t.Cleanup(func() { logger.SetOutput(os.Stderr) })

	return &buf
}

func newTestBuild(out io.Writer) *buildCommand {
	c := newBuildCommand()
	c.cmd.SetOut(out)
	c.depth = 1
	c.timeout = defaultBuildTimeoutMinutes

	return c
}

func TestExecuteBuild_DefaultBranchHead(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("echo build-output"))
	logs := captureLogger(t)

	var out bytes.Buffer

	require.NoError(t, newTestBuild(&out).executeBuild("sample"))
	assert.Empty(t, logs.String())

	short := head[:7]
	commitDir := filepath.Join(nigiriRoot, "sample", short)
	output := out.String()
	assert.Contains(t, output, "Getting HEAD of branch 'master' from "+repoDir+"...\n")
	assert.Contains(t, output, "Cloning repository to "+filepath.Join(commitDir, "src")+"...\n")
	assert.Contains(t, output, "Repository cloned in ")
	assert.NotContains(t, output, "cloning full history")
	assert.Contains(t, output, "Building target 'sample' with command: echo build-output\n")
	assert.Contains(t, output, "Build timeout: 30 minutes\n")
	assert.True(t, strings.HasSuffix(output,
		"Target 'sample' built at commit "+short+"\nRun with: nigiri run sample "+short+"\n"))

	buildLog, err := os.ReadFile(filepath.Join(commitDir, "logs", "build.log"))
	require.NoError(t, err)
	assert.Equal(t, "build-output\n", string(buildLog))

	info, err := os.ReadFile(filepath.Join(commitDir, buildInfoFileName))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(info),
		"Target: sample\nCommit: "+head+"\nShort hash: "+short+"\n"+buildDateField+" "))
	assert.Contains(t, string(info), "\nClone duration: ")
	assert.Contains(t, string(info), "\nBuild duration: ")
	assert.True(t, strings.HasSuffix(string(info), "\nOS: "+runtime.GOOS+"\nArchitecture: "+runtime.GOARCH+"\n"))

	assert.FileExists(t, filepath.Join(commitDir, "source.tar.gz"))
	assert.NoDirExists(t, filepath.Join(commitDir, "src"))
	assert.NoFileExists(t, filepath.Join(commitDir, "bin"))
}

// TestExecuteBuild_DefaultBranchFallsBackToMain builds a master-only source
// with the implicit "main" branch, so the remote's default branch is used.
func TestExecuteBuild_DefaultBranchFallsBackToMain(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTargetRaw(t, repoDir, buildCommandFor("true"))
	logs := captureLogger(t)

	var out bytes.Buffer

	require.NoError(t, newTestBuild(&out).executeBuild("sample"))
	assert.True(t, strings.HasPrefix(out.String(), "Getting HEAD of branch 'main' from "+repoDir+"...\n"))
	assert.Equal(t, "WARNING: Branch 'main' not found; using the remote default branch 'master'\n", logs.String())
	assert.DirExists(t, filepath.Join(nigiriRoot, "sample", head[:7]))
	assert.NoDirExists(t, filepath.Join(nigiriRoot, "sample", "0000000"))
}

func TestExecuteBuild_NoTimeoutMessageWhenDisabled(t *testing.T) {
	repoDir, _ := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("true"))

	var out bytes.Buffer

	c := newTestBuild(&out)
	c.timeout = 0
	require.NoError(t, c.executeBuild("sample"))
	assert.NotContains(t, out.String(), "Build timeout")
}

func TestExecuteBuild_SpecifiedCommit(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("true"))

	var out bytes.Buffer

	c := newTestBuild(&out)
	c.commit = head
	require.NoError(t, c.executeBuild("sample"))

	output := out.String()
	assert.Contains(t, output, "Using specified commit: "+head+"\n")
	assert.Contains(t, output, "Commit specified; cloning full history to resolve "+head+"\n")
	assert.Contains(t, output, "Checking out commit "+head+"...\n")
	assert.NotContains(t, output, "Getting HEAD")
}

func TestExecuteBuild_InvalidCommit(t *testing.T) {
	setupSampleTarget(t, filepath.Join(t.TempDir(), "missing"), buildCommandFor("true"))

	c := newTestBuild(io.Discard)
	c.commit = "not-a-hash"
	err := c.executeBuild("sample")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to calculate short hash: ")
}

func TestExecuteBuild_DefaultBranchLookupFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	setupSampleTargetRaw(t, missing, "    default-branch: develop\n"+buildCommandFor("true"))

	var out bytes.Buffer

	err := newTestBuild(&out).executeBuild("sample")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get HEAD of branch 'develop': ")
	assert.Contains(t, out.String(), "Getting HEAD of branch 'develop' from "+missing+"...\n")
}

func TestExecuteBuild_SkipsAlreadyBuiltAndForceRebuilds(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("true"))

	short := head[:7]

	require.NoError(t, newTestBuild(io.Discard).executeBuild("sample"))

	var skipped bytes.Buffer

	require.NoError(t, newTestBuild(&skipped).executeBuild("sample"))
	assert.True(t, strings.HasSuffix(skipped.String(),
		"Commit "+short+" has already been built. Use --force to rebuild.\n"))
	assert.NotContains(t, skipped.String(), "Cloning")

	commitDir := filepath.Join(nigiriRoot, "sample", short)
	require.NoError(t, os.MkdirAll(filepath.Join(commitDir, "src", "stale"), 0o755))

	var forced bytes.Buffer

	c := newTestBuild(&forced)
	c.forceBuild = true
	require.NoError(t, c.executeBuild("sample"))
	assert.Contains(t, forced.String(), "Force rebuilding commit "+short+"\n")
	assert.NoDirExists(t, filepath.Join(commitDir, "src"))
	assert.FileExists(t, filepath.Join(commitDir, "source.tar.gz"))
}

// TestExecuteBuild_FailedForceRebuildKeepsPreviousBuild pins that a failing
// forced rebuild neither removes the existing commit directory nor replaces
// the previous artifacts.
func TestExecuteBuild_FailedForceRebuildKeepsPreviousBuild(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("true"))

	require.NoError(t, newTestBuild(io.Discard).executeBuild("sample"))

	commitDir := filepath.Join(nigiriRoot, "sample", head[:7])
	infoBefore, err := os.ReadFile(filepath.Join(commitDir, buildInfoFileName))
	require.NoError(t, err)

	cfg := "targets:\n  sample:\n    source: " + repoDir + "\n    default-branch: master\n" +
		buildCommandFor("echo boom; exit 4")
	require.NoError(t, os.WriteFile(cfgFileFlag, []byte(cfg), 0o600))

	c := newTestBuild(io.Discard)
	c.forceBuild = true
	err = c.executeBuild("sample")
	require.Error(t, err)

	buildLogPath := filepath.Join(commitDir, "logs", "build.log")

	assert.True(t, strings.HasPrefix(err.Error(), "build failed: "))
	assert.True(t, strings.HasSuffix(err.Error(), "\nSee build log at "+buildLogPath))
	assert.DirExists(t, commitDir)
	assert.FileExists(t, filepath.Join(commitDir, "source.tar.gz"))

	infoAfter, err := os.ReadFile(filepath.Join(commitDir, buildInfoFileName))
	require.NoError(t, err)
	assert.Equal(t, string(infoBefore), string(infoAfter))

	buildLog, err := os.ReadFile(buildLogPath)
	require.NoError(t, err)
	assert.Equal(t, "boom\n", string(buildLog))
}

func TestExecuteBuild_WorkingDirectoryAndEnv(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sub/sample": "#!/bin/sh\n"})
	probe := t.TempDir()
	cmd := "printf %s \"$NIGIRI_PROBE\" > " + probe + "/env; pwd > " + probe + "/pwd"
	setupSampleTarget(t, repoDir, "    working-directory: sub\n    env:\n      - NIGIRI_PROBE=probe-value\n"+
		buildCommandFor(cmd))

	cwd, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, newTestBuild(io.Discard).executeBuild("sample"))

	env, err := os.ReadFile(filepath.Join(probe, "env"))
	require.NoError(t, err)
	assert.Equal(t, "probe-value", string(env))

	pwd, err := os.ReadFile(filepath.Join(probe, "pwd"))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(string(pwd)),
		filepath.Join("nigiri", "sample", head[:7], "src", "sub")), "build ran in %s", pwd)

	after, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, cwd, after)
}

func TestExecuteBuild_MissingWorkingDirectory(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, "    working-directory: nope\n"+buildCommandFor("true"))

	err := newTestBuild(io.Discard).executeBuild("sample")
	require.Error(t, err)
	assert.Equal(t, "working directory 'nope' not found in source", err.Error())
	assert.NoDirExists(t, filepath.Join(nigiriRoot, "sample", head[:7]))
}

func TestExecuteBuild_NoCommandForOS(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, "")

	err := newTestBuild(io.Discard).executeBuild("sample")
	require.Error(t, err)
	assert.Equal(t, "no build command specified for OS: "+runtime.GOOS, err.Error())
	assert.NoDirExists(t, filepath.Join(nigiriRoot, "sample", head[:7]))
}

func TestExecuteBuild_BinaryOnlyKeepsOnlyBinary(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\necho hi\n"})
	setupSampleTarget(t, repoDir, "    binary-only: true\n"+buildCommandFor("true")+"      binary-path: sample\n")

	require.NoError(t, newTestBuild(io.Discard).executeBuild("sample"))

	commitDir := filepath.Join(nigiriRoot, "sample", head[:7])
	bin, err := os.ReadFile(filepath.Join(commitDir, "bin"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho hi\n", string(bin))

	st, err := os.Stat(filepath.Join(commitDir, "bin"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), st.Mode().Perm())
	assert.NoDirExists(t, filepath.Join(commitDir, "src"))
	assert.NoFileExists(t, filepath.Join(commitDir, "source.tar.gz"))
}

func TestExecuteBuild_BinaryPathWithoutBinaryOnlyAlsoArchives(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("true")+"      binary-path: sample\n")

	require.NoError(t, newTestBuild(io.Discard).executeBuild("sample"))

	commitDir := filepath.Join(nigiriRoot, "sample", head[:7])
	assert.FileExists(t, filepath.Join(commitDir, "bin"))
	assert.FileExists(t, filepath.Join(commitDir, "source.tar.gz"))
	assert.NoDirExists(t, filepath.Join(commitDir, "src"))
}

func TestExecuteBuild_BinaryOnlyWithoutBinaryKeepsArchive(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, "    binary-only: true\n"+buildCommandFor("true")+"      binary-path: missing\n")
	logs := captureLogger(t)

	require.NoError(t, newTestBuild(io.Discard).executeBuild("sample"))

	commitDir := filepath.Join(nigiriRoot, "sample", head[:7])
	assert.NoFileExists(t, filepath.Join(commitDir, "bin"))
	assert.FileExists(t, filepath.Join(commitDir, "source.tar.gz"))
	assert.Contains(t, logs.String(), "WARNING: Failed to copy binary: failed to get file info: ")
	assert.Contains(t, logs.String(),
		"WARNING: binary-only is set for target 'sample' but no binary was captured; keeping the source archive\n")
}

func TestExecuteBuild_VerboseMirrorsBuildOutput(t *testing.T) {
	repoDir, _ := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("echo to-stdout; echo to-stderr >&2"))

	stdout := swapStdFile(t, &os.Stdout)
	stderr := swapStdFile(t, &os.Stderr)

	c := newTestBuild(io.Discard)
	c.verbose = true
	require.NoError(t, c.executeBuild("sample"))

	assert.Contains(t, readSwapped(t, stdout), "to-stdout\n")
	assert.Contains(t, readSwapped(t, stderr), "to-stderr\n")
}

func TestExecuteBuild_Timeout(t *testing.T) {
	repoDir, head := initRepoWithFiles(t, map[string]string{"sample": "#!/bin/sh\n"})
	setupSampleTarget(t, repoDir, buildCommandFor("sleep 5"))

	prev := buildTimeoutUnit
	buildTimeoutUnit = 50 * time.Millisecond

	t.Cleanup(func() { buildTimeoutUnit = prev })

	c := newTestBuild(io.Discard)
	c.timeout = 1
	err := c.executeBuild("sample")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "build failed: build timed out after 1 minutes\nSee build log at "))
	assert.NoDirExists(t, filepath.Join(nigiriRoot, "sample", head[:7]))
}

func swapStdFile(t *testing.T, target **os.File) *os.File {
	t.Helper()

	f, err := os.Create(filepath.Join(t.TempDir(), "std"))
	require.NoError(t, err)

	prev := *target
	*target = f

	t.Cleanup(func() {
		*target = prev

		if err := f.Close(); err != nil {
			t.Logf("failed to close stub: %v", err)
		}
	})

	return f
}

func readSwapped(t *testing.T, f *os.File) string {
	t.Helper()

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)

	return string(data)
}
