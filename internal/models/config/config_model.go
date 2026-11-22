// Package config defines the configuration models for the nigiri CLI.
package config

import "strings"

// Config is the parsed nigiri configuration file.
type Config struct {
	Targets  map[string]Target
	Defaults BuildCommand
	cfgDir   string
	cfgFile  string
}

// NewConfig returns an empty Config.
func NewConfig() *Config {
	return &Config{}
}

// Target is the configuration of one build target.
type Target struct {
	BuildCommand BuildCommand
	// DefaultBranch is built when no commit is given.
	DefaultBranch string
	// Sources is the URL of the repository to clone.
	Sources string
	// WorkingDirectory is the repository subdirectory the build command runs in.
	WorkingDirectory string
	// Env holds KEY=VALUE pairs added to the environment of the build and of the built binary.
	Env []string
	// BinaryOnly discards the source tree after a successful build.
	BinaryOnly bool
}

// BuildCommand holds the per-OS build commands of a target and the path of the
// binary they produce.
type BuildCommand struct {
	Linux   string
	Windows string
	Darwin  string
	// BinaryPathValue is relative to the working directory; read it through BinaryPath.
	BinaryPathValue string
}

// BinaryPath returns the configured binary path and whether one is set.
func (bc BuildCommand) BinaryPath() (string, bool) {
	if bc.BinaryPathValue == "" {
		return "", false
	}

	return bc.BinaryPathValue, true
}

// GetTarget looks up a target by name, falling back to the lowercased name so
// the CLI argument and the configured target name do not have to agree on case.
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

// GetCfgDir returns the directory the configuration file is loaded from.
func (c *Config) GetCfgDir() string {
	return c.cfgDir
}

// SetCfgDir sets the directory the configuration file is loaded from.
func (c *Config) SetCfgDir(cfgDir string) {
	c.cfgDir = cfgDir
}

// GetCfgFile returns the explicit configuration file path, or "" when unset.
func (c *Config) GetCfgFile() string {
	return c.cfgFile
}

// SetCfgFile sets an explicit configuration file path, which takes precedence
// over the configuration directory during loading.
func (c *Config) SetCfgFile(cfgFile string) {
	c.cfgFile = cfgFile
}
