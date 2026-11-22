// Package targets resolves the on-disk layout of build targets and their commits.
package targets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oota-sushikuitee/nigiri/internal/dirutils"
	"github.com/oota-sushikuitee/nigiri/pkg/commits"
)

// Target represents a build target with its associated commits
//
// Fields:
//   - Target: The name of the target
type Target struct {
	Target string
}

// ValidateTargetName checks that a user-supplied target name is safe to use
// as a directory name directly under the nigiri root. A target name must be a
// single local path element: names containing path separators, "..", ".", or
// an empty string are rejected so that a crafted name cannot resolve to a
// path outside the nigiri root (e.g. "nigiri remove ../<dir>").
//
// Parameters:
//   - name: The target name to validate
//
// Returns:
//   - error: An error describing why the name is invalid, or nil if it is valid
func ValidateTargetName(name string) error {
	if name == "" {
		return errors.New("target name must not be empty")
	}

	if name == "." || name == ".." {
		return fmt.Errorf("invalid target name %q: must be a single local path element", name)
	}

	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid target name %q: must not contain path separators", name)
	}

	if !filepath.IsLocal(name) {
		return fmt.Errorf("invalid target name %q: must be a single local path element", name)
	}

	return nil
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
	if err := ValidateTargetName(t.Target); err != nil {
		return "", err
	}

	fp := filepath.Join(nigiriRoot, t.Target)
	if !dirutils.Exists(fp) {
		return "", fmt.Errorf("target root does not exist: %s", fp)
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
	if err := ValidateTargetName(t.Target); err != nil {
		return "", err
	}

	fp := filepath.Join(nigiriRoot, t.Target)
	if !dirutils.Exists(fp) {
		if err := os.MkdirAll(fp, 0o755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", fp, err)
		}
	}

	return fp, nil
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

	if err := os.MkdirAll(fp, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", fp, err)
	}

	return fp, nil
}
