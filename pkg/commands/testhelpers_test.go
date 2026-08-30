package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeTestConfig writes a minimal configuration file defining a single
// "sample" target pointing at the given source, and returns its path.
func writeTestConfig(t *testing.T, dir, source string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, ".nigiri.yml")
	cfg := "targets:\n" +
		"  sample:\n" +
		"    source: " + source + "\n" +
		"    build-command:\n" +
		"      linux: \"true\"\n" +
		"      darwin: \"true\"\n" +
		"      windows: \"true\"\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o600))
	return cfgPath
}

// testRoot points the package globals at a fresh temporary nigiri root with a
// minimal configuration, so tests never touch the developer's real ~/.nigiri.
// It returns the nigiri root directory.
func testRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, filepath.Join(dir, "missing-repo"))
	root := filepath.Join(dir, "nigiri")
	restoreCommandGlobals(t, root, cfgPath)
	return root
}

// feedStdin replaces standard input with the given content for the duration of
// the test, so prompts driven by fmt.Scanln can be answered.
func feedStdin(t *testing.T, input string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	require.NoError(t, os.WriteFile(path, []byte(input), 0o600))
	file, err := os.Open(path)
	require.NoError(t, err)

	prev := os.Stdin
	t.Cleanup(func() {
		os.Stdin = prev
		if err := file.Close(); err != nil {
			t.Logf("failed to close stdin stub: %v", err)
		}
	})
	os.Stdin = file
}

// restoreCommandGlobals points the package-level nigiri root and config file at
// test locations and restores the previous values when the test ends.
func restoreCommandGlobals(t *testing.T, root, cfgFile string) {
	t.Helper()
	prevRoot, prevCfg := nigiriRoot, cfgFileFlag
	t.Cleanup(func() {
		nigiriRoot, cfgFileFlag = prevRoot, prevCfg
	})
	nigiriRoot = root
	cfgFileFlag = cfgFile
}
