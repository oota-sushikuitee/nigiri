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

// Target is a named build target whose builds live in a directory of the same
// name under the nigiri root.
type Target struct {
	Target string
}

// ValidateTargetName rejects names that are not a single local path element
// (empty, ".", "..", or containing a path separator), so that a crafted name
// cannot resolve outside the nigiri root (e.g. "nigiri remove ../<dir>").
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

// GetTargetRootDir returns the target's directory under nigiriRoot, or an
// error if the name is invalid or the directory does not exist.
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

// CreateTargetRootDirIfNotExist returns the target's directory under
// nigiriRoot, creating it first when it is missing.
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

// IsExistTargetCommitDir reports whether targetRoot already holds a build
// directory for commit.
func IsExistTargetCommitDir(targetRoot string, commit commits.Commit) bool {
	fp := filepath.Join(targetRoot, commit.ShortHash)

	return dirutils.Exists(fp)
}

// CreateTargetCommitDir creates the build directory for commit under
// targetRoot, named after its short hash, and returns its path. It fails if
// the commit is invalid or the directory already exists.
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
