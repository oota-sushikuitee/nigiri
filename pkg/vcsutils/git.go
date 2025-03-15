package vcsutils

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
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

// Clone clones the repository to the specified directory
//
// Parameters:
//   - cloneDir: The directory to clone the repository into
//   - opts: Additional options for cloning
//
// Returns:
//   - error: Any error encountered during the cloning process
func (g *Git) Clone(cloneDir string, opts ...Options) error {
	// Default options
	depth := 1
	verbose := false

	// Apply options if provided
	if len(opts) > 0 {
		if opts[0].Depth > 0 {
			depth = opts[0].Depth
		}
		verbose = opts[0].Verbose
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:               g.Source,
		ShallowSubmodules: depth == 1,
		Depth:             depth,
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

// GetDefaultBranchRemoteHead retrieves the HEAD commit hash of the default branch from the remote repository
//
// Parameters:
//   - defaultBranch: The name of the default branch
//
// Returns:
//   - error: Any error encountered during the process
func (g *Git) GetDefaultBranchRemoteHead(defaultBranch string) error {
	remote := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{g.Source},
	})

	refs, err := remote.List(&git.ListOptions{})
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
