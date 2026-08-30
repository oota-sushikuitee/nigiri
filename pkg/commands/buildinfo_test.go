package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTime(t *testing.T) {
	t.Parallel()
	recorded := time.Date(2024, 5, 4, 3, 2, 1, 0, time.UTC)
	fallback := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		contents string
		writeAny bool
		want     time.Time
	}{
		{
			name:     "recorded build date is used",
			contents: "Target: sample\nBuild date: " + recorded.Format(time.RFC3339) + "\nOS: linux\n",
			writeAny: true,
			want:     recorded,
		},
		{
			name:     "unparsable build date falls back",
			contents: "Build date: yesterday\n",
			writeAny: true,
			want:     fallback,
		},
		{
			name:     "missing build date falls back",
			contents: "Target: sample\n",
			writeAny: true,
			want:     fallback,
		},
		{
			name:     "missing metadata file falls back",
			writeAny: false,
			want:     fallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tt.writeAny {
				require.NoError(t, os.WriteFile(filepath.Join(dir, buildInfoFileName), []byte(tt.contents), 0o600))
			}
			assert.True(t, buildTime(dir, fallback).Equal(tt.want))
		})
	}
}
