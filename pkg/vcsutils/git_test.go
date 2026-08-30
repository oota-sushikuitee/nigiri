package vcsutils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestIsAuthRequiredError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "authentication required sentinel", err: transport.ErrAuthenticationRequired, want: true},
		{name: "authorization failed sentinel", err: transport.ErrAuthorizationFailed, want: true},
		{name: "wrapped authentication required", err: fmt.Errorf("clone failed: %w", transport.ErrAuthenticationRequired), want: true},
		{name: "message contains authentication", err: errors.New("remote: authentication required"), want: true},
		{name: "unrelated error", err: errors.New("repository not found"), want: false},
		{name: "network error", err: errors.New("dial tcp: connection refused"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAuthRequiredError(tt.err); got != tt.want {
				t.Errorf("isAuthRequiredError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

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

// TestClone clones the local repository built by initTestRepo, so the test
// exercises the clone path without depending on the network or on a
// third-party repository keeping its branches.
func TestClone(t *testing.T) {
	repoDir, _, second := initTestRepo(t)
	destDir := filepath.Join(t.TempDir(), "clone")

	g := Git{Source: repoDir}
	if err := g.Clone(destDir, Options{Depth: 1, AuthMethod: AuthNone}); err != nil {
		t.Fatalf("failed to clone repository: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "file.txt")); err != nil {
		t.Errorf("cloned worktree is missing file.txt: %v", err)
	}
	if g.HEAD != second {
		t.Errorf("HEAD after clone = %q, want %q", g.HEAD, second)
	}
}

func TestGetDefaultBranchRemoteHead(t *testing.T) {
	repoDir, _, second := initTestRepo(t)
	destDir := filepath.Join(t.TempDir(), "clone")

	g := Git{Source: repoDir}
	if err := g.Clone(destDir, Options{Depth: 1, AuthMethod: AuthNone}); err != nil {
		t.Fatalf("failed to clone repository: %v", err)
	}
	cloned := g.HEAD

	if err := g.GetDefaultBranchRemoteHead("master"); err != nil {
		t.Fatalf("failed to get remote HEAD: %v", err)
	}
	if g.HEAD != cloned {
		t.Errorf("remote HEAD = %q, want the cloned HEAD %q", g.HEAD, cloned)
	}
	if g.HEAD != second {
		t.Errorf("remote HEAD = %q, want %q", g.HEAD, second)
	}
}
