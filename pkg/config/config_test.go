package config

import (
	"os"
	"path/filepath"
	"testing"

	internalconfig "github.com/oota-sushikuitee/nigiri/internal/models/config"
)

func setupTestConfig(t *testing.T) (string, *ConfigManager) {
	// Create a temporary directory for config test
	tempDir, err := os.MkdirTemp("", "nigiri-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a sample config file
	configContent := `
targets:
  test-target:
    source: https://github.com/oota-sushikuitee/nigiri
    default-branch: main
    build-command:
      linux: make build
      windows: make build
      darwin: make build
      binary-path: bin/nigiri
    env:
      - "GO111MODULE=on"
      - "CGO_ENABLED=0"
  another-target:
    source: https://github.com/Okabe-Junya/.github
    default-branch: main
    build-command:
      linux: make build
      windows: make build
      darwin: make build
defaults:
  linux: make build
  windows: make build
  darwin: make build
`
	configPath := filepath.Join(tempDir, ".nigiri.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
		os.RemoveAll(tempDir)
	}

	// Create a config instance
	cm := NewConfigManager()
	cm.Config.SetCfgDir(tempDir)
	return tempDir, cm
}

func setupInvalidYamlConfig(t *testing.T) (string, *ConfigManager) {
	tempDir, err := os.MkdirTemp("", "nigiri-invalid-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create an invalid YAML config file
	invalidContent := `
targets:
  invalid-yaml:
    - this is not valid yaml
      indentation is wrong
    source: https://example.com
    default-branch: main
  build-command: not-a-map
defaults:
  - also: invalid
`
	configPath := filepath.Join(tempDir, ".nigiri.yml")
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid test config: %v", err)
		os.RemoveAll(tempDir)
	}

	cm := NewConfigManager()
	cm.Config.SetCfgDir(tempDir)
	return tempDir, cm
}

func cleanupTestConfig(tempDir string) {
	os.RemoveAll(tempDir)
}

func TestNewConfigManager(t *testing.T) {
	cm := NewConfigManager()
	if cm == nil {
		t.Error("NewConfigManager() returned nil")
	}
	if cm.Config == nil {
		t.Error("NewConfigManager().Config is nil")
	}
}

func TestConfigManager_GetConfig(t *testing.T) {
	cm := NewConfigManager()
	config := cm.GetConfig()
	if config == nil {
		t.Error("GetConfig() returned nil")
	}
}

func TestConfig_GetSetCfgDir(t *testing.T) {
	// 既存のNewConfigではホームディレクトリが設定されるため、
	// 空のcfgDirでConfigを直接初期化する
	cfg := &internalconfig.Config{}
	testDir := "/test/config/dir"

	// 初期値は空のはず
	if dir := cfg.GetCfgDir(); dir != "" {
		t.Errorf("Initial config dir expected to be empty, got %s", dir)
	}

	// Set and get
	cfg.SetCfgDir(testDir)
	if dir := cfg.GetCfgDir(); dir != testDir {
		t.Errorf("Config dir = %s, want %s", dir, testDir)
	}
}

func TestConfigManager_LoadCfgFile(t *testing.T) {
	tempDir, cm := setupTestConfig(t)
	defer cleanupTestConfig(tempDir)

	err := cm.LoadCfgFile()
	if err != nil {
		t.Fatalf("LoadCfgFile() error = %v", err)
	}

	// Verify loaded configuration
	if len(cm.Config.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(cm.Config.Targets))
	}

	// Check first target
	target1, exists := cm.Config.Targets["test-target"]
	if !exists {
		t.Error("test-target not found in loaded config")
	} else {
		if target1.Sources != "https://github.com/oota-sushikuitee/nigiri" {
			t.Errorf("Target source = %s, want %s", target1.Sources, "https://github.com/oota-sushikuitee/nigiri")
		}
		if target1.DefaultBranch != "main" {
			t.Errorf("Target default branch = %s, want %s", target1.DefaultBranch, "main")
		}
		if target1.BuildCommand.Linux != "make build" {
			t.Errorf("Target Linux build command = %s, want %s", target1.BuildCommand.Linux, "make build")
		}
		if len(target1.Env) != 2 {
			t.Errorf("Expected 2 env variables, got %d", len(target1.Env))
		}
	}

	// Check second target
	target2, exists := cm.Config.Targets["another-target"]
	if !exists {
		t.Error("another-target not found in loaded config")
	} else {
		if target2.Sources != "https://github.com/Okabe-Junya/.github" {
			t.Errorf("Target source = %s, want %s", target2.Sources, "https://github.com/Okabe-Junya/.github")
		}
		if target2.DefaultBranch != "main" {
			t.Errorf("Target default branch = %s, want %s", target2.DefaultBranch, "main")
		}
	}

	// Check defaults
	if cm.Config.Defaults.Linux != "make build" {
		t.Errorf("Default Linux build command = %s, want %s", cm.Config.Defaults.Linux, "make build")
	}
}

func TestConfigManager_LoadCfgFile_NonExistentFile(t *testing.T) {
	cm := NewConfigManager()
	cm.Config.SetCfgDir("/non/existent/directory")
	err := cm.LoadCfgFile()
	if err == nil {
		t.Error("LoadCfgFile() expected error for non-existent file")
	}
}

// Test loading an invalid YAML file
func TestConfigManager_LoadCfgFile_InvalidYaml(t *testing.T) {
	tempDir, cm := setupInvalidYamlConfig(t)
	defer cleanupTestConfig(tempDir)

	err := cm.LoadCfgFile()
	if err == nil {
		t.Error("LoadCfgFile() should return error for invalid YAML")
	}
}

// Test loading a config file with empty config directory
func TestConfigManager_LoadCfgFile_EmptyCfgDir(t *testing.T) {
	cm := NewConfigManager()
	cm.Config.SetCfgDir("")
	err := cm.LoadCfgFile()

	// The function should handle empty cfgDir by using home directory
	if err == nil {
		// If there's a valid config in the default location, this might not error
		// So we just check that cfgDir was set to something
		if cm.Config.GetCfgDir() == "" {
			t.Error("LoadCfgFile() didn't set cfgDir when it was empty")
		}
	}
}

// Test loading a file with empty targets
func TestConfigManager_LoadCfgFile_NoTargets(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nigiri-empty-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	emptyConfig := `defaults:
  linux: make build
  windows: make build
  darwin: make build`

	configPath := filepath.Join(tempDir, ".nigiri.yml")
	if err := os.WriteFile(configPath, []byte(emptyConfig), 0644); err != nil {
		t.Fatalf("Failed to write empty test config: %v", err)
	}

	cm := NewConfigManager()
	cm.Config.SetCfgDir(tempDir)
	err = cm.LoadCfgFile()
	if err == nil {
		t.Error("LoadCfgFile() should return error when no targets are defined")
	}
}

func TestBuildCommand_BinaryPath(t *testing.T) {
	tests := []struct {
		name        string
		buildCmd    internalconfig.BuildCommand
		wantPath    string
		wantHasPath bool
	}{
		{
			name:        "with binary path",
			buildCmd:    internalconfig.BuildCommand{BinaryPathValue: "bin/app"},
			wantPath:    "bin/app",
			wantHasPath: true,
		},
		{
			name:        "without binary path",
			buildCmd:    internalconfig.BuildCommand{},
			wantPath:    "",
			wantHasPath: false,
		},
		{
			name:        "with empty binary path",
			buildCmd:    internalconfig.BuildCommand{BinaryPathValue: ""},
			wantPath:    "",
			wantHasPath: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotHasPath := tt.buildCmd.BinaryPath()
			if gotPath != tt.wantPath {
				t.Errorf("BinaryPath() path = %v, want %v", gotPath, tt.wantPath)
			}
			if gotHasPath != tt.wantHasPath {
				t.Errorf("BinaryPath() hasPath = %v, want %v", gotHasPath, tt.wantHasPath)
			}
		})
	}
}

func TestConfigManager_SaveCfgFile(t *testing.T) {
	tempDir, cm := setupTestConfig(t)
	defer cleanupTestConfig(tempDir)

	// Load the existing config
	if err := cm.LoadCfgFile(); err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Modify the config
	cm.Config.Targets["new-target"] = internalconfig.Target{
		Sources:       "https://github.com/Okabe-Junya/dotfiles",
		DefaultBranch: "main",
		BuildCommand: internalconfig.BuildCommand{
			Linux:           "make build",
			Windows:         "make build",
			Darwin:          "make build",
			BinaryPathValue: "/usr/local/bin/test",
		},
		Env:              []string{"TEST_ENV=value"},
		WorkingDirectory: "/tmp",
		BinaryOnly:       true,
	}

	// Save the modified config
	err := cm.SaveCfgFile()
	if err != nil {
		t.Fatalf("SaveCfgFile() error = %v", err)
	}

	// Create a new config instance and load the saved file
	newCm := NewConfigManager()
	newCm.Config.SetCfgDir(tempDir)
	if err := newCm.LoadCfgFile(); err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	// Verify the new config has the added target
	newTarget, exists := newCm.Config.Targets["new-target"]
	if !exists {
		t.Error("new-target not found in saved config")
	} else {
		if newTarget.Sources != "https://github.com/Okabe-Junya/dotfiles" {
			t.Errorf("Saved target source = %s, want %s", newTarget.Sources, "https://github.com/Okabe-Junya/dotfiles")
		}
		if !newTarget.BinaryOnly {
			t.Error("Saved target binary-only flag was not persisted")
		}
		if newTarget.WorkingDirectory != "/tmp" {
			t.Errorf("Saved target working directory = %s, want %s", newTarget.WorkingDirectory, "/tmp")
		}
		path, hasPath := newTarget.BuildCommand.BinaryPath()
		if !hasPath {
			t.Error("Saved target binary path was not persisted")
		} else if path != "/usr/local/bin/test" {
			t.Errorf("Saved target binary path = %s, want %s", path, "/usr/local/bin/test")
		}
	}

	// Verify original targets still exist
	if _, exists := newCm.Config.Targets["test-target"]; !exists {
		t.Error("test-target not found in saved config")
	}
	if _, exists := newCm.Config.Targets["another-target"]; !exists {
		t.Error("another-target not found in saved config")
	}
}

// Test saving to a directory with insufficient permissions
func TestConfigManager_SaveCfgFile_PermissionDenied(t *testing.T) {
	// Skip on Windows where permissions work differently
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	// Only run if not running as root
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	cm := NewConfigManager()
	cm.Config.SetCfgDir("/root/.nigiri") // A directory normal users can't write to

	// Should fail to save
	err := cm.SaveCfgFile()
	if err == nil {
		t.Error("SaveCfgFile() should fail when writing to a protected directory")
	}
}
