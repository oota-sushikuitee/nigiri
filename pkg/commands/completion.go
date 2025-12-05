package commands

import (
	"os"
	"strings"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
	"github.com/oota-sushikuitee/nigiri/pkg/config"
)

// getConfiguredTargets returns a list of target names from the configuration file
// that match the given prefix. This is used for shell completion.
//
// Parameters:
//   - prefix: The prefix to filter targets by
//
// Returns:
//   - []string: A list of matching target names
func getConfiguredTargets(prefix string) []string {
	cm := config.NewConfigManager()
	if err := cm.LoadCfgFile(); err != nil {
		return nil
	}

	var targetList []string
	for target := range cm.Config.Targets {
		if strings.HasPrefix(target, prefix) {
			targetList = append(targetList, target)
		}
	}
	return targetList
}

// getInstalledTargets returns a list of installed target directories
// that match the given prefix. This is used for shell completion.
//
// Parameters:
//   - prefix: The prefix to filter targets by
//
// Returns:
//   - []string: A list of matching target directory names
func getInstalledTargets(prefix string) []string {
	entries, err := os.ReadDir(nigiriRoot)
	if err != nil {
		return nil
	}

	var targetList []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			if prefix == "" || strings.HasPrefix(entry.Name(), prefix) {
				targetList = append(targetList, entry.Name())
			}
		}
	}
	return targetList
}

// getTargetCommits returns a list of commit hashes for the specified target
// that match the given prefix. This is used for shell completion.
//
// Parameters:
//   - target: The target name to get commits for
//   - prefix: The prefix to filter commits by
//
// Returns:
//   - []string: A list of matching commit hashes
func getTargetCommits(target, prefix string) []string {
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

	var commitList []string
	for _, dir := range dirs {
		if dir.IsDir() && strings.HasPrefix(dir.Name(), prefix) {
			commitList = append(commitList, dir.Name())
		}
	}
	return commitList
}
