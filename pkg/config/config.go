package config

import (
	"github.com/spf13/viper"
)

// Config represents the configuration for the nigiri CLI
//
// Fields:
//   - cfgDir: The directory where the configuration file is located
//   - Targets: A map of target names to their configurations
//   - Defaults: The default build command configuration

type Config struct {
	cfgDir   string
	Targets  map[string]Target `mapstructure:"targets"`
	Defaults BuildCommand      `mapstructure:"defaults"`
}

// Target represents the configuration for a specific target
//
// Fields:
//   - Target: The name of the target
//   - Sources: The source repository URL
//   - DefaultBranch: The default branch of the repository
//   - BuildCommand: The build command configuration
//   - Env: Environment variables to set when running the target
//   - Commits: A list of commit hashes

type Target struct {
	Target        string
	Sources       string       `mapstructure:"source"`
	DefaultBranch string       `mapstructure:"default-branch"`
	BuildCommand  BuildCommand `mapstructure:"build-command"`
	Env           []string     `mapstructure:"env"`
	Commits       []string
}

// BuildCommand represents the build command configuration for a target
//
// Fields:
//   - Linux: The build command for Linux
//   - Windows: The build command for Windows
//   - Darwin: The build command for macOS
//   - BinaryPath: The path to the built binary
type BuildCommand struct {
	Linux           string `mapstructure:"linux"`
	Windows         string `mapstructure:"windows"`
	Darwin          string `mapstructure:"darwin"`
	BinaryPathValue string `mapstructure:"binary-path"` // The binary path field
}

// BinaryPath returns the configured binary path if set, otherwise false
//
// Returns:
//   - string: The binary path
//   - bool: True if the binary path is set, false otherwise
func (bc BuildCommand) BinaryPath() (string, bool) {
	if bc.BinaryPathValue == "" {
		return "", false
	}
	return bc.BinaryPathValue, true
}

// GetCfgDir returns the configuration directory
//
// Returns:
//   - string: The configuration directory
func (c *Config) GetCfgDir() string {
	return c.cfgDir
}

// SetCfgDir sets the configuration directory
//
// Parameters:
//   - cfgDir: The directory to set as the configuration directory
func (c *Config) SetCfgDir(cfgDir string) {
	c.cfgDir = cfgDir
}

// NewConfig creates a new Config instance
//
// Returns:
//   - *Config: A new Config instance
func NewConfig() *Config {
	return &Config{}
}

// LoadCfgFile loads the configuration file from the configuration directory
//
// Returns:
//   - error: Any error encountered during the loading process
func (c *Config) LoadCfgFile() error {
	v := viper.New()
	v.SetConfigName(".nigiri")
	v.SetConfigType("yaml")
	v.AddConfigPath(c.cfgDir)
	// TODO: fix this hardcoded path
	v.AddConfigPath("example")
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(&c); err != nil {
		return err
	}
	return nil
}

// SaveCfgFile saves the configuration to the configuration file
//
// Returns:
//   - error: Any error encountered during the saving process
func (c *Config) SaveCfgFile() error {
	viper.Set("targets", c.Targets)
	viper.Set("defaults", c.Defaults)
	if err := viper.WriteConfig(); err != nil {
		return err
	}
	return nil
}
