package commands

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// buildInfoFileName is the metadata file the build command writes into a
	// commit directory.
	buildInfoFileName = "build-info.txt"
	// buildDateField is the metadata field holding the RFC3339 build time.
	buildDateField = "Build date:"
)

// buildTime returns the build timestamp recorded for a commit directory,
// falling back to the supplied value when no usable metadata is present.
// Directory modification times are not authoritative: running a build extracts
// sources into its directory and the target process writes into it, so an old
// build can otherwise look newer than the genuinely latest one.
//
// Parameters:
//   - commitDir: The commit directory holding the build
//   - fallback: The timestamp to use when no build date is recorded
//
// Returns:
//   - time.Time: The recorded build time, or the fallback
func buildTime(commitDir string, fallback time.Time) time.Time {
	data, err := os.ReadFile(filepath.Join(commitDir, buildInfoFileName))
	if err != nil {
		return fallback
	}
	for _, line := range strings.Split(string(data), "\n") {
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
