package commands

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	buildInfoFileName = "build-info.txt"
	buildDateField    = "Build date:"
)

// buildTime prefers the recorded build date over the directory modification
// time: running a build extracts sources into its directory and the target
// process writes into it, so an old build could otherwise look newer than the
// genuinely latest one.
func buildTime(commitDir string, fallback time.Time) time.Time {
	data, err := os.ReadFile(filepath.Join(commitDir, buildInfoFileName))
	if err != nil {
		return fallback
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		value, found := strings.CutPrefix(strings.TrimSpace(line), buildDateField)
		if !found {
			continue
		}

		recorded, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
		if err != nil {
			return fallback
		}

		return recorded
	}

	return fallback
}
