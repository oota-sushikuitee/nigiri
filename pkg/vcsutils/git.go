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

// Git represents a git repository with its source URL and HEAD commit hash
//
// Fields:
//   - Source: The source repository URL
//   - HEAD: The HEAD commit hash
type Git struct {
	Source string
	HEAD   string
}

// AuthMethod represents the authentication method
type AuthMethod string

const (
	// AuthToken uses a GitHub token for authentication
	AuthToken AuthMethod = "token"
	// AuthSSH uses SSH keys for authentication
	AuthSSH AuthMethod = "ssh"
	// AuthNone uses no authentication (for public repositories)
	AuthNone AuthMethod = "none"
)

// Options represents git operation options
type Options struct {
	// AuthMethod specifies the authentication method to use
	AuthMethod AuthMethod
	// Token is the GitHub token to use for authentication
	Token string
	// Depth specifies the clone depth (0 for full history)
	Depth int
	// Verbose enables verbose output
	Verbose bool
	// UnshallowIfNeeded specifies whether to unshallow if needed
	UnshallowIfNeeded bool
}

// getGitHubToken tries to get a GitHub token from various sources
func getGitHubToken() (string, error) {
	// First check environment variable
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}

	// Then try gh cli
	cmd := exec.CommandContext(context.Background(), "gh", "auth", "token")
	output, err := cmd.Output()
	if err == nil {
		token = strings.TrimSpace(string(output))
		if token != "" {
			return token, nil
		}
	}

	// Could add more methods here (like reading from ~/.netrc or other sources)
	return "", fmt.Errorf("no GitHub token found, set GITHUB_TOKEN environment variable or login with 'gh auth login'")
}

// normalizeCloneDepth maps a requested clone depth to the value passed to go-git.
// 0 means full history (go-git treats 0 as no depth limit); negative values are
// coerced to a full clone as well.
func normalizeCloneDepth(depth int) int {
	if depth < 0 {
		return 0
	}
	return depth
}

// Clone clones the repository to the specified directory
//
// Parameters:
//   - cloneDir: The directory to clone the repository into
//   - opts: Additional options for cloning (Depth 0 means full history)
//
// Returns:
//   - error: Any error encountered during the cloning process
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
			Username: "x-access-token", // This is what GitHub expects for token auth
			Password: token,
		}
	}

	// Add progress reporting if verbose
	if verbose {
		cloneOpts.Progress = os.Stdout
	}

	// Create destination directory if it doesn't exist
	if _, err := os.Stat(cloneDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cloneDir, 0755); err != nil {
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
				Username: "x-access-token",
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

// isAuthRequiredError reports whether err indicates that the remote requires
// authentication (or that the provided credentials were rejected). It is used
// to decide whether an anonymous operation should be retried with a token.
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

// GetDefaultBranchRemoteHead retrieves the HEAD commit hash of the default branch from the remote repository
//
// Parameters:
//   - defaultBranch: The name of the default branch
//
// Returns:
//   - error: Any error encountered during the process
func (g *Git) GetDefaultBranchRemoteHead(defaultBranch string) error {
	// When dealing with potentially private repos, it's better to use go-git's
	// authentication mechanisms rather than the RemoteConfig directly

	// First try without authentication
	remote := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{g.Source},
	})
	refs, err := remote.List(&git.ListOptions{})

	// If we failed, try with token (might be a private repo)
	if err != nil && isAuthRequiredError(err) {
		token, tokenErr := getGitHubToken()
		if tokenErr == nil {
			auth := &githttp.BasicAuth{
				Username: "x-access-token",
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
	branchRefName := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", defaultBranch))
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

// Checkout checkouts the specified commit or branch in the repository
//
// Parameters:
//   - repoDir: The directory containing the repository
//   - ref: The reference (commit hash or branch name) to checkout
//
// Returns:
//   - error: Any error encountered during the checkout process
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
