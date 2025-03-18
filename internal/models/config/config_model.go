// Package config defines the configuration models for the nigiri CLI
package config

// Config represents the configuration for the nigiri CLI
//
// Fields:
//   - cfgDir: The directory where the configuration file is located
//   - Targets: A map of target names to their configurations
//   - Defaults: The default build command configuration
type Config struct {
	Targets  map[string]Target `mapstructure:"targets"`
	Defaults BuildCommand      `mapstructure:"defaults"`
	cfgDir   string
}

// Target represents the configuration for a specific target
//
// Fields:
//   - BuildCommand: The build command configuration
//   - Env: Environment variables to set when running the target
//   - Sources: The source repository URL
//   - DefaultBranch: The default branch of the repository
//   - WorkingDirectory: The directory within the repository to run the build command
//   - BinaryOnly: Whether to keep only the binary and remove source code after build
type Target struct {
	BuildCommand     BuildCommand `yaml:"build_command"`
	DefaultBranch    string       `yaml:"default_branch"`
	Sources          string       `yaml:"sources"`
	WorkingDirectory string       `yaml:"working_directory"`
	Env              []string     `yaml:"env"`
	BinaryOnly       bool         `yaml:"binary_only"`
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
	BinaryPathValue string `mapstructure:"binary-path"`
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
