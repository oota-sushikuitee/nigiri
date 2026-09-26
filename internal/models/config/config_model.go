// Package config defines the configuration models for the nigiri CLI
package config

import "strings"

// Config represents the configuration for the nigiri CLI
//
// Fields:
//   - cfgDir: The directory where the configuration file is located
//   - Targets: A map of target names to their configurations
//   - Defaults: The default build command configuration
type Config struct {
	Targets  map[string]Target
	Defaults BuildCommand
	cfgDir   string
	cfgFile  string
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
	BuildCommand     BuildCommand
	DefaultBranch    string
	Sources          string
	WorkingDirectory string
	Env              []string
	BinaryOnly       bool
}

// BuildCommand represents the build command configuration for a target
//
// Fields:
//   - Linux: The build command for Linux
//   - Windows: The build command for Windows
//   - Darwin: The build command for macOS
//   - BinaryPath: The path to the built binary
type BuildCommand struct {
	Linux           string
	Windows         string
	Darwin          string
	BinaryPathValue string
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

// GetTarget looks up a target by name, falling back to a case-insensitive
// match so the CLI argument and the configured target name do not have to
// agree on case exactly.
//
// Parameters:
//   - name: The target name as supplied by the user
//
// Returns:
//   - Target: The matching target configuration
//   - bool: True if a target with that name exists
func (c *Config) GetTarget(name string) (Target, bool) {
	if target, ok := c.Targets[name]; ok {
		return target, true
	}
	if lowered := strings.ToLower(name); lowered != name {
		if target, ok := c.Targets[lowered]; ok {
			return target, true
		}
	}
	return Target{}, false
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

// GetCfgFile returns the explicit configuration file path, if any
//
// Returns:
//   - string: The configuration file path (empty when unset)
func (c *Config) GetCfgFile() string {
	return c.cfgFile
}

// SetCfgFile sets an explicit configuration file path. When set, it takes
// precedence over the configuration directory during loading.
//
// Parameters:
//   - cfgFile: The path to the configuration file to load
func (c *Config) SetCfgFile(cfgFile string) {
	c.cfgFile = cfgFile
}

// NewConfig creates a new Config instance
//
// Returns:
//   - *Config: A new Config instance
func NewConfig() *Config {
	return &Config{}
}
