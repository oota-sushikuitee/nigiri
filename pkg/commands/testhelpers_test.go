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
