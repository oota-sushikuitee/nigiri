package vcsutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClone(t *testing.T) {
	testDir := t.TempDir()

	// Note that this is a public repository
	testCloneRepo := "https://github.com/Okabe-Junya/.github"

	g := Git{
		Source: testCloneRepo,
	}

	opts := Options{
		Depth:      1,
		Verbose:    false,
		AuthMethod: AuthNone,
	}

	err := g.Clone(testDir, opts)
	if err != nil {
		t.Errorf("Failed to clone repository: %v", err)
	}

	// Check if .github directory exists in the test directory
	if _, err := os.Stat(filepath.Join(testDir, ".github")); os.IsNotExist(err) {
		t.Errorf("Failed to clone repository: %v", err)
	}
}

func TestGetRemoteHead(t *testing.T) {
	testDir := t.TempDir()

	// Note that this is a public repository
	testCloneRepo := "https://github.com/Okabe-Junya/.github"

	g := Git{
		Source: testCloneRepo,
	}

	// 新しいOptionsの構造体を使用
	opts := Options{
		Depth:      1,
		Verbose:    false,
		AuthMethod: AuthNone,
	}

	err := g.Clone(testDir, opts)
	if err != nil {
		t.Errorf("Failed to clone repository: %v", err)
	}

	head1 := g.HEAD

	err = g.GetDefaultBranchRemoteHead("main")
	if err != nil {
		t.Errorf("Failed to get remote HEAD: %v", err)
	}

	head2 := g.HEAD

	if head1 != head2 {
		t.Errorf("HEAD does not match: %v != %v", head1, head2)
	}
}
