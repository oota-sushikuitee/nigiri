package vcsutils

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
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
	cmd := exec.Command("gh", "auth", "token")
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

// Clone clones the repository to the specified directory
//
// Parameters:
//   - cloneDir: The directory to clone the repository into
//   - opts: Additional options for cloning
//
// Returns:
//   - error: Any error encountered during the cloning process
func (g *Git) Clone(cloneDir string, opts Options) error {
	// Default options
	depth := 1
	verbose := false
	authMethod := AuthNone

	// Apply provided options
	if opts.Depth > 0 {
		depth = opts.Depth
	}
	verbose = opts.Verbose
	if opts.AuthMethod != "" {
		authMethod = opts.AuthMethod
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:               g.Source,
		ShallowSubmodules: depth == 1,
		Depth:             depth,
	}

	// Handle authentication
	if authMethod == AuthToken || (authMethod == AuthNone && isGitHubURL(g.Source) && isPrivateRepo(g.Source)) {
		token := opts.Token
		if token == "" {
			var err error
			token, err = getGitHubToken()
			if err != nil {
				return err
			}
		}

		cloneOpts.Auth = &http.BasicAuth{
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

// isGitHubURL checks if the URL is a GitHub URL
func isGitHubURL(repoURL string) bool {
	return strings.Contains(repoURL, "github.com")
}

// isPrivateRepo attempts to determine if a repository is private
// This is a heuristic and may not be 100% accurate
func isPrivateRepo(repoURL string) bool {
	// Try to parse the URL
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return true // Assume private if we can't parse the URL
	}

	// Check if it's using SSH protocol
	if parsedURL.Scheme == "git" || strings.HasPrefix(repoURL, "git@") {
		return true // SSH URLs are often used for private repos
	}

	// For GitHub, we could try to query the API to check if the repo is private
	// But for now, we'll assume that we need auth for all GitHub repos
	if isGitHubURL(repoURL) {
		return true
	}

	return false
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
	if err != nil && strings.Contains(err.Error(), "authentication") {
		token, tokenErr := getGitHubToken()
		if tokenErr == nil {
			auth := &http.BasicAuth{
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
		// If not a branch, try as commit hash
		err = w.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(ref),
		})
		if err != nil {
			return fmt.Errorf("failed to checkout reference '%s': %w", ref, err)
		}
	}

	return nil
}
