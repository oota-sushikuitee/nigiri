// Package vcsutils clones and checks out source repositories.
package vcsutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Git is a remote git repository. HEAD holds the commit hash resolved by the
// last successful Clone or GetDefaultBranchRemoteHead.
type Git struct {
	Source string
	HEAD   string
}

const tokenAuthUsername = "x-access-token"

// AuthMethod selects how Clone authenticates to the remote.
type AuthMethod string

const (
	// AuthToken authenticates over HTTPS with a GitHub token.
	AuthToken AuthMethod = "token"
	// AuthSSH is reserved for SSH key authentication; Clone does not
	// implement it and clones anonymously.
	AuthSSH AuthMethod = "ssh"
	// AuthNone clones anonymously, retrying with a GitHub token if the
	// remote requires authentication and a token is available.
	AuthNone AuthMethod = "none"
)

// Options configures Clone.
type Options struct {
	// AuthMethod defaults to AuthNone when empty.
	AuthMethod AuthMethod
	// Token is used with AuthToken. When empty, GITHUB_TOKEN and then
	// `gh auth token` are tried.
	Token string
	// Depth limits the clone history; 0 or a negative value clones the full
	// history.
	Depth int
	// Verbose writes clone progress to stdout.
	Verbose bool
	// UnshallowIfNeeded is currently ignored.
	UnshallowIfNeeded bool
}

func getGitHubToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}

	cmd := exec.CommandContext(context.Background(), "gh", "auth", "token")

	output, err := cmd.Output()
	if err == nil {
		token = strings.TrimSpace(string(output))
		if token != "" {
			return token, nil
		}
	}

	// Could add more methods here (like reading from ~/.netrc or other sources)
	return "", errors.New("no GitHub token found, set GITHUB_TOKEN environment variable or login with 'gh auth login'")
}

// normalizeCloneDepth coerces negative depths to 0, which go-git treats as no
// depth limit.
func normalizeCloneDepth(depth int) int {
	if depth < 0 {
		return 0
	}

	return depth
}

// Clone clones Source into cloneDir, creating the directory if needed, and
// records the cloned HEAD commit in g.HEAD.
func (g *Git) Clone(cloneDir string, opts Options) error {
	// Default options
	depth := normalizeCloneDepth(opts.Depth)
	verbose := opts.Verbose
	authMethod := AuthNone

	// Apply provided options
	if opts.AuthMethod != "" {
		authMethod = opts.AuthMethod
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:               g.Source,
		ShallowSubmodules: depth == 1,
		Depth:             depth,
	}

	// For explicit token authentication, attach credentials up front.
	// Anonymous clones (AuthNone) are attempted without credentials first and
	// only retried with a token if the server requires authentication; this
	// keeps token-less clones of public repositories working.
	if authMethod == AuthToken {
		token := opts.Token
		if token == "" {
			var err error

			token, err = getGitHubToken()
			if err != nil {
				return err
			}
		}

		cloneOpts.Auth = &githttp.BasicAuth{
			Username: tokenAuthUsername,
			Password: token,
		}
	}

	// Add progress reporting if verbose
	if verbose {
		cloneOpts.Progress = os.Stdout
	}

	// Create destination directory if it doesn't exist
	if _, err := os.Stat(cloneDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cloneDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", cloneDir, err)
		}
	}

	// Perform clone
	r, err := git.PlainClone(cloneDir, false, cloneOpts)

	// If an anonymous clone failed because the server requires authentication,
	// retry with a token when one is available (e.g. private repositories).
	if err != nil && authMethod == AuthNone && cloneOpts.Auth == nil && isAuthRequiredError(err) {
		if token, tokenErr := getGitHubToken(); tokenErr == nil {
			cloneOpts.Auth = &githttp.BasicAuth{
				Username: tokenAuthUsername,
				Password: token,
			}
			// A failed clone may leave a partially initialized directory;
			// clear it so the retry starts from a clean state.
			_ = os.RemoveAll(cloneDir)
			r, err = git.PlainClone(cloneDir, false, cloneOpts)
		}
	}

	if err != nil {
		// Handle specific errors more gracefully
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("destination path already exists and is not empty: %s", cloneDir)
		}

		return fmt.Errorf("git clone failed: %w", err)
	}

	// Get HEAD reference
	ref, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	g.HEAD = ref.Hash().String()

	return nil
}

// isAuthRequiredError also matches on the message text because not every
// transport wraps the go-git sentinel errors.
func isAuthRequiredError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, transport.ErrAuthenticationRequired) ||
		errors.Is(err, transport.ErrAuthorizationFailed) {
		return true
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "authentication required") ||
		strings.Contains(msg, "authorization failed") ||
		strings.Contains(msg, "authentication")
}

// GetDefaultBranchRemoteHead stores in g.HEAD the commit that defaultBranch
// points to on the remote, without cloning. If the branch is not found, the
// remote's HEAD is used instead. Listing is retried with a GitHub token when
// the remote requires authentication.
func (g *Git) GetDefaultBranchRemoteHead(defaultBranch string) error {
	remote := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{g.Source},
	})
	refs, err := remote.List(&git.ListOptions{})

	// If we failed, try with token (might be a private repo)
	if err != nil && isAuthRequiredError(err) {
		token, tokenErr := getGitHubToken()
		if tokenErr == nil {
			auth := &githttp.BasicAuth{
				Username: tokenAuthUsername,
				Password: token,
			}
			refs, err = remote.List(&git.ListOptions{Auth: auth})
		}
	}

	if err != nil {
		if strings.Contains(err.Error(), "authentication") {
			return fmt.Errorf("authentication failed: %w", err)
		}

		return fmt.Errorf("failed to list remote references: %w", err)
	}

	// Try finding the exact match first
	for _, ref := range refs {
		if ref.Name().IsBranch() && ref.Name().Short() == defaultBranch {
			g.HEAD = ref.Hash().String()

			return nil
		}
	}

	// If not found, try with refs/heads/ prefix
	branchRefName := plumbing.ReferenceName("refs/heads/" + defaultBranch)
	for _, ref := range refs {
		if ref.Name() == branchRefName {
			g.HEAD = ref.Hash().String()

			return nil
		}
	}

	// Also try HEAD resolution for default branch
	for _, ref := range refs {
		if ref.Name().String() == "HEAD" {
			g.HEAD = ref.Hash().String()

			return nil
		}
	}

	return fmt.Errorf("branch '%s' not found in remote repository", defaultBranch)
}

// Checkout checks out ref in the repository at repoDir. ref is tried as a
// local branch name first, then resolved as a revision (full or short commit
// hash, or tag).
func (g *Git) Checkout(repoDir string, ref string) error {
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Try checkout as branch first
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(ref),
	})
	if err != nil {
		// If not a branch, resolve the revision (full/short commit hash or tag)
		hash, resolveErr := r.ResolveRevision(plumbing.Revision(ref))
		if resolveErr != nil {
			return fmt.Errorf("failed to resolve reference '%s': %w", ref, resolveErr)
		}

		if err := w.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
			return fmt.Errorf("failed to checkout reference '%s': %w", ref, err)
		}
	}

	return nil
}
