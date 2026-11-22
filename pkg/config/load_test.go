package config

import (
	"os"
	"path/filepath"
	"testing"

	internalconfig "github.com/oota-sushikuitee/nigiri/internal/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sourceURL = "https://github.com/oota-sushikuitee/nigiri"

func makeBuild(cmd string) internalconfig.BuildCommand {
	return internalconfig.BuildCommand{Linux: cmd, Windows: cmd, Darwin: cmd}
}

// TestLoadCfgFile_ParsedConfig pins the in-memory result of representative
// config files, so a change of YAML decoder cannot silently alter how existing
// files are read.
func TestLoadCfgFile_ParsedConfig(t *testing.T) {
	t.Parallel()

	sample, err := os.ReadFile(filepath.Join("..", "commands", "sample-config.yml"))
	require.NoError(t, err)

	tests := []struct {
		name         string
		content      string
		wantTargets  map[string]internalconfig.Target
		wantDefaults internalconfig.BuildCommand
		wantErr      bool
	}{
		{
			name:    "embedded sample config",
			content: string(sample),
			wantTargets: map[string]internalconfig.Target{
				"sample-project": {
					Sources:       sourceURL,
					DefaultBranch: "main",
					BuildCommand: internalconfig.BuildCommand{
						Linux: "make build", Windows: "make build", Darwin: "make build", BinaryPathValue: "bin/nigiri",
					},
					Env: []string{"GO111MODULE=on", "CGO_ENABLED=0"},
				},
			},
			wantDefaults: makeBuild("make build"),
		},
		{
			name: "every field, quoted and unquoted scalars",
			content: `targets:
  app:
    source: "` + sourceURL + `"
    default-branch: 'release/1.0'
    working-directory: cmd/app
    binary-only: true
    env: [A=1, "B=two words", 'C=x: y']
    build-command:
      linux: "make build # not a comment"
      windows: 'go build -o bin\app.exe .'
      darwin: make build
      binary-path: bin/app
defaults:
  linux: "make"
  windows: make
  darwin: 'make'
`,
			wantTargets: map[string]internalconfig.Target{
				"app": {
					Sources:          sourceURL,
					DefaultBranch:    "release/1.0",
					WorkingDirectory: "cmd/app",
					BinaryOnly:       true,
					Env:              []string{"A=1", "B=two words", "C=x: y"},
					BuildCommand: internalconfig.BuildCommand{
						Linux:           "make build # not a comment",
						Windows:         `go build -o bin\app.exe .`,
						Darwin:          "make build",
						BinaryPathValue: "bin/app",
					},
				},
			},
			wantDefaults: makeBuild("make"),
		},
		{
			name: "non-string scalars decode into string fields as written",
			content: `targets:
  app:
    source: ` + sourceURL + `
    default-branch: 1.10
    working-directory: 0755
    env: [true, 42]
    build-command:
      linux: yes
`,
			wantTargets: map[string]internalconfig.Target{
				"app": {
					Sources:          sourceURL,
					DefaultBranch:    "1.10",
					WorkingDirectory: "0755",
					Env:              []string{"true", "42"},
					BuildCommand:     internalconfig.BuildCommand{Linux: "yes"},
				},
			},
		},
		{
			name: "YAML 1.1 boolean words",
			content: `targets:
  on-target:
    source: ` + sourceURL + `
    binary-only: yes
  off-target:
    source: ` + sourceURL + `
    binary-only: off
`,
			wantTargets: map[string]internalconfig.Target{
				"on-target":  {Sources: sourceURL, BinaryOnly: true},
				"off-target": {Sources: sourceURL},
			},
		},
		{
			name: "keys are case-sensitive and target names keep their case",
			content: `targets:
  MyTool:
    Source: https://example.com/ignored
    sources: ` + sourceURL + `
    Default-Branch: ignored
    BUILD-COMMAND:
      linux: ignored
Defaults:
  linux: ignored
`,
			wantTargets: map[string]internalconfig.Target{
				"MyTool": {Sources: sourceURL},
			},
		},
		{
			name: "source wins over the sources alias",
			content: `targets:
  app:
    sources: https://example.com/alias
    source: ` + sourceURL + `
`,
			wantTargets: map[string]internalconfig.Target{"app": {Sources: sourceURL}},
		},
		{
			name: "unknown keys are ignored",
			content: `version: 2
targets:
  app:
    source: ` + sourceURL + `
    timeout: 10
    build-command:
      linux: make
      freebsd: gmake
defaults:
  freebsd: gmake
  linux: make
`,
			wantTargets: map[string]internalconfig.Target{
				"app": {Sources: sourceURL, BuildCommand: internalconfig.BuildCommand{Linux: "make"}},
			},
			wantDefaults: internalconfig.BuildCommand{Linux: "make"},
		},
		{
			name: "empty and null values",
			content: `targets:
  app:
    source: ` + sourceURL + `
    default-branch:
    working-directory: ~
    env: []
defaults:
`,
			wantTargets: map[string]internalconfig.Target{"app": {Sources: sourceURL, Env: []string{}}},
		},
		{
			name: "anchors, aliases and merge keys",
			content: `common: &common
  linux: make
  darwin: make
targets:
  app:
    source: &src ` + sourceURL + `
    build-command:
      <<: *common
      darwin: gmake
  copy:
    source: *src
    build-command: *common
`,
			wantTargets: map[string]internalconfig.Target{
				"app":  {Sources: sourceURL, BuildCommand: internalconfig.BuildCommand{Linux: "make", Darwin: "gmake"}},
				"copy": {Sources: sourceURL, BuildCommand: internalconfig.BuildCommand{Linux: "make", Darwin: "make"}},
			},
		},
		{
			name: "only the first document is read",
			content: `targets:
  first:
    source: ` + sourceURL + `
---
targets:
  second:
    source: ` + sourceURL + `
`,
			wantTargets: map[string]internalconfig.Target{"first": {Sources: sourceURL}},
		},
		{
			name: "duplicate keys are rejected",
			content: `targets:
  app:
    source: ` + sourceURL + `
    source: https://example.com/duplicate
`,
			wantErr: true,
		},
		{
			name: "quoted boolean is rejected",
			content: `targets:
  app:
    source: ` + sourceURL + `
    binary-only: "true"
`,
			wantErr: true,
		},
		{
			name: "list where a string is expected is rejected",
			content: `targets:
  app:
    source: [` + sourceURL + `]
`,
			wantErr: true,
		},
		{
			name:    "tab indentation is rejected",
			content: "targets:\n\tapp:\n\t\tsource: " + sourceURL + "\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.yml")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			cm := NewConfigManager()
			cm.Config.SetCfgFile(path)

			err := cm.LoadCfgFile()
			if tt.wantErr {
				require.Error(t, err)
				t.Logf("error: %v", err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantTargets, cm.Config.Targets)
			assert.Equal(t, tt.wantDefaults, cm.Config.Defaults)
		})
	}
}
