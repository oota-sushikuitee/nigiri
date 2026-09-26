// Package config provides functionality to manage the nigiri CLI configuration files
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/internal/models/config"
	"go.yaml.in/yaml/v3"
)

// fileConfig mirrors the on-disk YAML layout of a nigiri configuration file.
type fileConfig struct {
	Targets  map[string]targetConfig `yaml:"targets"`
	Defaults map[string]string       `yaml:"defaults"`
}

// targetConfig mirrors the on-disk YAML layout of a single target. Source is
// the current key; Sources is kept as a fallback alias for older config files.
type targetConfig struct {
	BuildCommand     buildCommandConfig `yaml:"build-command"`
	Source           string             `yaml:"source"`
	Sources          string             `yaml:"sources,omitempty"`
	DefaultBranch    string             `yaml:"default-branch"`
	WorkingDirectory string             `yaml:"working-directory"`
	Env              []string           `yaml:"env,omitempty"`
	BinaryOnly       bool               `yaml:"binary-only"`
}

type buildCommandConfig struct {
	Linux      string `yaml:"linux"`
	Windows    string `yaml:"windows"`
	Darwin     string `yaml:"darwin"`
	BinaryPath string `yaml:"binary-path,omitempty"`
}

// ConfigManager handles the reading and writing of configuration files
type ConfigManager struct {
	Config *config.Config
}

// NewConfigManager creates a new ConfigManager with default configuration
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

// configFilePath resolves the configuration file to load: the explicit
// --config path when set, otherwise the first of .nigiri.yaml / .nigiri.yml
// found in the configuration directory.
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

// LoadCfgFile loads the configuration file. When an explicit config file path
// has been set (e.g. via the --config flag), that file is loaded directly;
// otherwise the file is discovered in the configuration directory.
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

// SaveCfgFile saves the configuration to the configuration file
func (cm *ConfigManager) SaveCfgFile() error {
	cfgDir := cm.Config.GetCfgDir()

	file := fileConfig{
		Targets: make(map[string]targetConfig, len(cm.Config.Targets)),
		Defaults: map[string]string{
			"linux":   cm.Config.Defaults.Linux,
			"windows": cm.Config.Defaults.Windows,
			"darwin":  cm.Config.Defaults.Darwin,
		},
	}

	for name, target := range cm.Config.Targets {
		file.Targets[name] = targetConfig{
			Source:           target.Sources,
			DefaultBranch:    target.DefaultBranch,
			BinaryOnly:       target.BinaryOnly,
			WorkingDirectory: target.WorkingDirectory,
			Env:              target.Env,
			BuildCommand: buildCommandConfig{
				Linux:      target.BuildCommand.Linux,
				Windows:    target.BuildCommand.Windows,
				Darwin:     target.BuildCommand.Darwin,
				BinaryPath: target.BuildCommand.BinaryPathValue,
			},
		}
	}

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configFile := filepath.Join(cfgDir, ".nigiri.yml")
	if err := os.WriteFile(configFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// GetConfig returns the configuration
func (cm *ConfigManager) GetConfig() *config.Config {
	return cm.Config
}
