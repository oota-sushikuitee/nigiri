package vcsutils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNormalizeCloneDepth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		depth int
		want  int
	}{
		{name: "zero means full history", depth: 0, want: 0},
		{name: "negative is coerced to full history", depth: -3, want: 0},
		{name: "shallow depth is preserved", depth: 1, want: 1},
		{name: "custom depth is preserved", depth: 10, want: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeCloneDepth(tt.depth); got != tt.want {
				t.Errorf("normalizeCloneDepth(%d) = %d, want %d", tt.depth, got, tt.want)
			}
		})
	}
}

// initTestRepo creates a local repository with two commits and returns the
// repository directory and the two commit hashes
func initTestRepo(t *testing.T) (repoDir, first, second string) {
	t.Helper()
	repoDir = t.TempDir()
	r, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("failed to init repository: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	sig := &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}
	commit := func(content string) string {
		if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		if _, err := w.Add("file.txt"); err != nil {
			t.Fatalf("failed to add file: %v", err)
		}
		hash, err := w.Commit(content, &git.CommitOptions{Author: sig})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
		return hash.String()
	}
	first = commit("first")
	second = commit("second")
	return repoDir, first, second
}

func TestCheckout(t *testing.T) {
	repoDir, first, second := initTestRepo(t)
	g := &Git{}

	tests := []struct {
		name    string
		ref     string
		wantErr bool
		content string
	}{
		{name: "full commit hash", ref: first, content: "first"},
		{name: "short commit hash", ref: second[:7], content: "second"},
		{name: "branch name", ref: "master", content: "second"},
		{name: "unknown reference returns error", ref: "0000000000000000000000000000000000000000", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.Checkout(repoDir, tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Checkout(%q) expected error, got nil", tt.ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("Checkout(%q) failed: %v", tt.ref, err)
			}
			content, err := os.ReadFile(filepath.Join(repoDir, "file.txt"))
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}
			if string(content) != tt.content {
				t.Errorf("file content = %q, want %q", content, tt.content)
			}
		})
	}
}

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
