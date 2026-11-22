// Package config loads the nigiri configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/internal/models/config"
	"go.yaml.in/yaml/v3"
)

type fileConfig struct {
	Targets  map[string]targetConfig `yaml:"targets"`
	Defaults map[string]string       `yaml:"defaults"`
}

// targetConfig keeps Sources as a fallback alias of Source for older config
// files.
type targetConfig struct {
	BuildCommand     buildCommandConfig `yaml:"build-command"`
	Source           string             `yaml:"source"`
	Sources          string             `yaml:"sources"`
	DefaultBranch    string             `yaml:"default-branch"`
	WorkingDirectory string             `yaml:"working-directory"`
	Env              []string           `yaml:"env"`
	BinaryOnly       bool               `yaml:"binary-only"`
}

type buildCommandConfig struct {
	Linux      string `yaml:"linux"`
	Windows    string `yaml:"windows"`
	Darwin     string `yaml:"darwin"`
	BinaryPath string `yaml:"binary-path"`
}

// ConfigManager reads the configuration file into Config.
type ConfigManager struct {
	Config *config.Config
}

// NewConfigManager returns a ConfigManager with an empty Config whose
// configuration directory is ~/.nigiri (or the current directory when the
// home directory cannot be determined).
func NewConfigManager() *ConfigManager {
	cfg := config.NewConfig()

	homeDir, err := os.UserHomeDir()
	if err == nil {
		cfg.SetCfgDir(filepath.Join(homeDir, ".nigiri"))
	} else {
		cfg.SetCfgDir(".")
	}

	return &ConfigManager{
		Config: cfg,
	}
}

// LoadCfgFile parses the configuration file into Config. A path set with
// Config.SetCfgFile is loaded directly; otherwise .nigiri.yaml, then
// .nigiri.yml, is looked up in the configuration directory. A file that
// defines no targets is an error.
func (cm *ConfigManager) LoadCfgFile() error {
	path, err := cm.configFilePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var file fileConfig
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(file.Targets) == 0 {
		return fmt.Errorf("no targets found in configuration file at %s", path)
	}

	cm.Config.Targets = make(map[string]config.Target, len(file.Targets))
	for name, t := range file.Targets {
		source := t.Source
		if source == "" {
			source = t.Sources
		}

		cm.Config.Targets[name] = config.Target{
			Sources:          source,
			DefaultBranch:    t.DefaultBranch,
			BinaryOnly:       t.BinaryOnly,
			WorkingDirectory: t.WorkingDirectory,
			Env:              t.Env,
			BuildCommand: config.BuildCommand{
				Linux:           t.BuildCommand.Linux,
				Windows:         t.BuildCommand.Windows,
				Darwin:          t.BuildCommand.Darwin,
				BinaryPathValue: t.BuildCommand.BinaryPath,
			},
		}
	}

	cm.Config.Defaults = config.BuildCommand{
		Linux:   file.Defaults["linux"],
		Windows: file.Defaults["windows"],
		Darwin:  file.Defaults["darwin"],
	}

	return nil
}

func (cm *ConfigManager) configFilePath() (string, error) {
	if cfgFile := cm.Config.GetCfgFile(); cfgFile != "" {
		return cfgFile, nil
	}

	cfgDir := cm.Config.GetCfgDir()
	if cfgDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}

		cfgDir = filepath.Join(homeDir, ".nigiri")
		cm.Config.SetCfgDir(cfgDir)
	}

	for _, name := range []string{".nigiri.yaml", ".nigiri.yml"} {
		candidate := filepath.Join(cfgDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to read config file: no .nigiri.yaml or .nigiri.yml found in %s", cfgDir)
}
