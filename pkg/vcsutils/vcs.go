package vcsutils

// VCS defines the interface for version control system operations
type VCS interface {
	// Clone clones the repository to the specified directory
	Clone(cloneDir string, opts Options) error
	// GetDefaultBranchRemoteHead retrieves the HEAD commit hash of the default branch
	GetDefaultBranchRemoteHead(defaultBranch string) error
}
