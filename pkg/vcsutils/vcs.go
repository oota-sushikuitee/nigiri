package vcsutils

// VCS is the set of repository operations implemented by Git.
type VCS interface {
	Clone(cloneDir string, opts Options) error
	GetDefaultBranchRemoteHead(defaultBranch string) error
}
