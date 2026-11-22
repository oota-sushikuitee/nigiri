package commands

import (
	"os"
	"strings"

	"github.com/oota-sushikuitee/nigiri/internal/targets"
)

func getConfiguredTargets(prefix string) []string {
	cm := newConfigManager()
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

func getTargetCommits(target, prefix string) []string {
	fsTarget := targets.Target{Target: target}

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
