// Package config provides functionality to manage the nigiri CLI configuration files
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/internal/models/config"
	"github.com/spf13/viper"
)

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

// LoadCfgFile loads the configuration file from the configuration directory
func (cm *ConfigManager) LoadCfgFile() error {
	cfgDir := cm.Config.GetCfgDir()
	if cfgDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		cfgDir = filepath.Join(homeDir, ".nigiri")
		cm.Config.SetCfgDir(cfgDir)
	}

	v := viper.New()
	v.SetConfigName(".nigiri")
	v.SetConfigType("yaml")
	v.AddConfigPath(cfgDir)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Create a map to store the intermediate configuration
	var cfg struct {
		Targets  map[string]map[string]interface{} `mapstructure:"targets"`
		Defaults map[string]string                 `mapstructure:"defaults"`
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(cfg.Targets) == 0 {
		return fmt.Errorf("no targets found in configuration file at %s", v.ConfigFileUsed())
	}

	// Convert the map to our config structure
	cm.Config.Targets = make(map[string]config.Target)
	for name, targetCfg := range cfg.Targets {
		target := config.Target{}

		// Handle source/sources field
		if source, ok := targetCfg["source"]; ok {
			target.Sources = source.(string)
		} else if sources, ok := targetCfg["sources"]; ok {
			target.Sources = sources.(string)
		}

		// Handle other fields
		if branch, ok := targetCfg["default-branch"]; ok {
			target.DefaultBranch = branch.(string)
		}
		if binaryOnly, ok := targetCfg["binary-only"]; ok {
			target.BinaryOnly = binaryOnly.(bool)
		}
		if workingDir, ok := targetCfg["working-directory"]; ok {
			target.WorkingDirectory = workingDir.(string)
		}
		if env, ok := targetCfg["env"]; ok {
			if envSlice, isSlice := env.([]interface{}); isSlice {
				for _, e := range envSlice {
					target.Env = append(target.Env, e.(string))
				}
			}
		}

		// Handle build command
		if buildCmd, ok := targetCfg["build-command"].(map[string]interface{}); ok {
			if linux, exists := buildCmd["linux"]; exists {
				target.BuildCommand.Linux = linux.(string)
			}
			if windows, exists := buildCmd["windows"]; exists {
				target.BuildCommand.Windows = windows.(string)
			}
			if darwin, exists := buildCmd["darwin"]; exists {
				target.BuildCommand.Darwin = darwin.(string)
			}
			if binPath, exists := buildCmd["binary-path"]; exists {
				target.BuildCommand.BinaryPathValue = binPath.(string)
			}
		}

		cm.Config.Targets[name] = target
	}

	// Handle defaults
	if cfg.Defaults != nil {
		cm.Config.Defaults = config.BuildCommand{
			Linux:   cfg.Defaults["linux"],
			Windows: cfg.Defaults["windows"],
			Darwin:  cfg.Defaults["darwin"],
		}
	}

	return nil
}

// SaveCfgFile saves the configuration to the configuration file
func (cm *ConfigManager) SaveCfgFile() error {
	cfgDir := cm.Config.GetCfgDir()
	v := viper.New()
	v.SetConfigName(".nigiri")
	v.SetConfigType("yaml")
	v.AddConfigPath(cfgDir)

	// Create target configurations that properly include all fields
	targetConfigs := make(map[string]map[string]interface{})
	for name, target := range cm.Config.Targets {
		targetConfig := map[string]interface{}{
			"source":            target.Sources,
			"default-branch":    target.DefaultBranch,
			"binary-only":       target.BinaryOnly,
			"working-directory": target.WorkingDirectory,
		}

		if len(target.Env) > 0 {
			targetConfig["env"] = target.Env
		}

		buildCommand := map[string]interface{}{
			"linux":   target.BuildCommand.Linux,
			"windows": target.BuildCommand.Windows,
			"darwin":  target.BuildCommand.Darwin,
		}

		if target.BuildCommand.BinaryPathValue != "" {
			buildCommand["binary-path"] = target.BuildCommand.BinaryPathValue
		}

		targetConfig["build-command"] = buildCommand
		targetConfigs[name] = targetConfig
	}

	// Set values in viper
	if err := v.MergeConfigMap(map[string]interface{}{
		"targets": targetConfigs,
		"defaults": map[string]interface{}{
			"linux":   cm.Config.Defaults.Linux,
			"windows": cm.Config.Defaults.Windows,
			"darwin":  cm.Config.Defaults.Darwin,
		},
	}); err != nil {
		return fmt.Errorf("failed to merge config: %w", err)
	}

	// Save to file
	configFile := filepath.Join(cfgDir, ".nigiri.yml")
	return v.WriteConfigAs(configFile)
}

// GetConfig returns the configuration
func (cm *ConfigManager) GetConfig() *config.Config {
	return cm.Config
}
