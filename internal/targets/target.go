package targets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oota-sushikuitee/nigiri/internal/dirutils"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
)

// Target represents a build target with its associated commits
//
// Fields:
//   - Target: The name of the target
//   - Commits: A collection of commits associated with the target
type Target struct {
	Target  string
	Commits commits.Commits
}

// GetTargetRootDir returns the root directory for the specified target
//
// Parameters:
//   - nigiriRoot: The root directory for nigiri
//
// Returns:
//   - string: The target root directory path
//   - error: Any error encountered during the process
func (t *Target) GetTargetRootDir(nigiriRoot string) (string, error) {
	fp := filepath.Join(nigiriRoot, t.Target)
	if !dirutils.Exists(fp) {
		return "", fmt.Errorf("target root does not exist: %s", fp)
	}
	return fp, nil
}

// CreateTargetRootDir creates the root directory for the specified target
//
// Parameters:
//   - nigiriRoot: The root directory for nigiri
//
// Returns:
//   - string: The created target root directory path
//   - error: Any error encountered during the process
func (t *Target) CreateTargetRootDir(nigiriRoot string) (string, error) {
	fp := filepath.Join(nigiriRoot, t.Target)
	if dirutils.Exists(fp) {
		return "", fmt.Errorf("target root already exists: %s", fp)
	}
	if err := os.MkdirAll(fp, 0755); err != nil {
		return "", err
	}
	return fp, nil
}

// CreateTargetRootDirIfNotExist creates the root directory for the specified target if it does not already exist
//
// Parameters:
//   - nigiriRoot: The root directory for nigiri
//
// Returns:
//   - string: The created or existing target root directory path
//   - error: Any error encountered during the process
func (t *Target) CreateTargetRootDirIfNotExist(nigiriRoot string) (string, error) {
	fp := filepath.Join(nigiriRoot, t.Target)
	if !dirutils.Exists(fp) {
		if err := os.MkdirAll(fp, 0755); err != nil {
			return "", err
		}
	}
	return fp, nil
}

// GetTargetHeadDir returns the latest commit directory for the specified target
//
// Parameters:
//   - nigiriRoot: The root directory for nigiri
//
// Returns:
//   - string: The latest commit directory path
//   - error: Any error encountered during the process
func (t *Target) GetTargetHeadDir(nigiriRoot string) (string, error) {
	if len(t.Commits.Commits) == 0 {
		return "", fmt.Errorf("no commits found for target: %s", t.Target)
	}
	// Get the latest commit
	latestCommit := t.Commits.Commits[len(t.Commits.Commits)-1]
	targetRoot, err := t.GetTargetRootDir(nigiriRoot)
	if err != nil {
		return "", err
	}
	// Get the latest commit directory
	return GetTargetCommitDir(targetRoot, latestCommit)
}

// IsExistTargetCommitDir checks if the commit directory for the specified target exists
//
// Parameters:
//   - targetRoot: The root directory for the target
//   - commit: The commit to check
//
// Returns:
//   - bool: True if the commit directory exists, false otherwise
func IsExistTargetCommitDir(targetRoot string, commit commits.Commit) bool {
	fp := filepath.Join(targetRoot, commit.ShortHash)
	return dirutils.Exists(fp)
}

// GetTargetCommitDir returns the commit directory for the specified target and commit
//
// Parameters:
//   - targetRoot: The root directory for the target
//   - commit: The commit to get the directory for
//
// Returns:
//   - string: The commit directory path
//   - error: Any error encountered during the process
func GetTargetCommitDir(targetRoot string, commit commits.Commit) (string, error) {
	if err := commit.Validate(); err != nil {
		return "", err
	}
	fp := filepath.Join(targetRoot, commit.ShortHash)
	if !dirutils.Exists(fp) {
		return "", fmt.Errorf("commit directory does not exist: %s", fp)
	}
	return fp, nil
}

// CreateTargetCommitDir creates the commit directory for the specified target and commit
//
// Parameters:
//   - targetRoot: The root directory for the target
//   - commit: The commit to create the directory for
//
// Returns:
//   - string: The created commit directory path
//   - error: Any error encountered during the process
func CreateTargetCommitDir(targetRoot string, commit commits.Commit) (string, error) {
	if err := commit.Validate(); err != nil {
		return "", err
	}
	fp := filepath.Join(targetRoot, commit.ShortHash)
	if dirutils.Exists(fp) {
		return "", fmt.Errorf("commit directory already exists: %s", fp)
	}
	if err := os.MkdirAll(fp, 0755); err != nil {
		return "", err
	}
	return fp, nil
}
