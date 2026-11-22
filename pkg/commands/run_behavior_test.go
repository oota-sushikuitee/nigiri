//go:build !windows

package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBuildFile creates path (relative to the sample build directory
// abcdef1) with content and mode, creating parent directories.
func writeBuildFile(t *testing.T, rel, content string, mode os.FileMode) string {
	t.Helper()

	path := filepath.Join(nigiriRoot, "sample", "abcdef1", rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))

	return path
}

func newTestRun(out io.Writer) *runCommand {
	c := newRunCommand()
	c.cmd.SetOut(out)
	c.cmd.SetErr(io.Discard)

	return c
}

func TestExecuteRun_NoBuilds(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	require.NoError(t, os.MkdirAll(filepath.Join(nigiriRoot, "sample"), 0o755))

	err := newTestRun(io.Discard).executeRun("sample", "", nil)
	require.Error(t, err)
	assert.Equal(t, "no builds found for target sample", err.Error())
}

func TestExecuteRun_CommitTooShort(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	require.NoError(t, os.MkdirAll(filepath.Join(nigiriRoot, "sample"), 0o755))

	err := newTestRun(io.Discard).executeRun("sample", "abc", nil)
	require.Error(t, err)
	assert.Equal(t, "commit hash is too short: abc (minimum 7 characters)", err.Error())
}

func TestExecuteRun_TargetMissingFromConfig(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	require.NoError(t, os.MkdirAll(filepath.Join(nigiriRoot, "other", "abcdef1"), 0o755))

	err := newTestRun(io.Discard).executeRun("other", "abcdef1", nil)
	require.Error(t, err)
	assert.Equal(t, "target 'other' not found in configuration", err.Error())
}

func TestExecuteRun_ConfigLoadFailure(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	writeBuildFile(t, "bin", "#!/bin/sh\n", 0o755)
	require.NoError(t, os.Remove(cfgFileFlag))

	err := newTestRun(io.Discard).executeRun("sample", "abcdef1", nil)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "failed to load config: "))
}

func TestExecuteRun_BinaryRunsWithArgsEnvAndDir(t *testing.T) {
	setupSampleTarget(t, "unused", "    env:\n      - NIGIRI_PROBE=probe-value\n"+buildCommandFor("true"))
	bin := writeBuildFile(t, "bin", "#!/bin/sh\necho \"args=$*\"\necho \"env=$NIGIRI_PROBE\"\npwd\n", 0o644)

	var out bytes.Buffer

	require.NoError(t, newTestRun(&out).executeRun("sample", "", []string{"a", "-b"}))

	output := out.String()
	assert.True(t, strings.HasPrefix(output,
		"Using latest commit: abcdef1\nRunning "+bin+" with args: [a -b]\nargs=a -b\nenv=probe-value\n"), output)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(output), filepath.Join("sample", "abcdef1")), output)

	st, err := os.Stat(bin)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), st.Mode().Perm())
}

func TestExecuteRun_BinaryPathFromConfig(t *testing.T) {
	setupSampleTarget(t, "unused", "    working-directory: sub\n"+buildCommandFor("true")+"      binary-path: out/tool\n")
	tool := writeBuildFile(t, "src/sub/out/tool", "#!/bin/sh\necho from-tool\n", 0o755)

	var out bytes.Buffer

	require.NoError(t, newTestRun(&out).executeRun("sample", "abcdef1", nil))
	assert.Equal(t, "Binary not found in commit/bin directory, looking for alternative locations...\n"+
		"Running "+tool+" with args: []\nfrom-tool\n", out.String())
}

func TestExecuteRun_MissingWorkingDirectory(t *testing.T) {
	setupSampleTarget(t, "unused", "    working-directory: nope\n"+buildCommandFor("true"))
	writeBuildFile(t, "src/sample", "#!/bin/sh\n", 0o755)

	err := newTestRun(io.Discard).executeRun("sample", "abcdef1", nil)
	require.Error(t, err)
	assert.Equal(t, "working directory 'nope' not found in source", err.Error())
}

func TestExecuteRun_BinaryCandidates(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{name: "source root", files: []string{"src/sample", "src/bin/sample", "src/build/sample"}, want: "src/sample"},
		{name: "bin directory", files: []string{"src/bin/sample", "src/build/sample"}, want: "src/bin/sample"},
		{name: "build directory", files: []string{"src/build/sample"}, want: "src/build/sample"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSampleTarget(t, "unused", buildCommandFor("true"))

			for _, f := range tt.files {
				writeBuildFile(t, f, "#!/bin/sh\necho ran-"+f+"\n", 0o755)
			}

			var out bytes.Buffer

			require.NoError(t, newTestRun(&out).executeRun("sample", "abcdef1", nil))
			assert.Contains(t, out.String(), "ran-"+tt.want+"\n")
		})
	}
}

func TestExecuteRun_BinaryNotFound(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	require.NoError(t, os.MkdirAll(filepath.Join(nigiriRoot, "sample", "abcdef1", "src"), 0o755))

	err := newTestRun(io.Discard).executeRun("sample", "abcdef1", nil)
	require.Error(t, err)
	assert.Equal(t, "binary not found at "+filepath.Join(nigiriRoot, "sample", "abcdef1", "src", "sample"), err.Error())
}

func TestExecuteRun_CorruptArchiveIsCleanedUp(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	writeBuildFile(t, "source.tar.gz", "not a gzip stream", 0o644)

	var out bytes.Buffer

	err := newTestRun(&out).executeRun("sample", "abcdef1", nil)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "failed to extract source archive: "))
	assert.Contains(t, out.String(), "Extracting source archive...\n")
	assert.NoDirExists(t, filepath.Join(nigiriRoot, "sample", "abcdef1", "src"))
}

func TestExecuteRun_ExistingSrcSkipsExtraction(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	writeBuildFile(t, "source.tar.gz", "not a gzip stream", 0o644)
	writeBuildFile(t, "src/sample", "#!/bin/sh\necho from-src\n", 0o755)

	var out bytes.Buffer

	require.NoError(t, newTestRun(&out).executeRun("sample", "abcdef1", nil))
	assert.NotContains(t, out.String(), "Extracting")
	assert.Contains(t, out.String(), "from-src\n")
}

func TestExecuteRun_StartFailureIsNotExitCode(t *testing.T) {
	setupSampleTarget(t, "unused", buildCommandFor("true"))
	writeBuildFile(t, "bin", "\x00\x01not-an-executable", 0o755)

	err := newTestRun(io.Discard).executeRun("sample", "abcdef1", nil)
	require.Error(t, err)

	var exitErr *ExitCodeError
	assert.NotErrorAs(t, err, &exitErr)
}
