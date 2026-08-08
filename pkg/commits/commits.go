package commits

import "fmt"

// Commit represents a git commit with its hash and short hash
//
// Fields:
//   - Hash: The full commit hash
//   - ShortHash: The short version of the commit hash

type Commit struct {
	Hash      string
	ShortHash string
}

// Commits represents a collection of git commits
//
// Fields:
//   - Commits: A slice of Commit structs

type Commits struct {
	Commits []Commit
}

const (
	// ShortHashLength is the number of leading hash characters used as the
	// on-disk directory name of a build.
	ShortHashLength = 7
	// maxHashLength is the length of a full SHA-1 commit hash.
	maxHashLength = 40
)

// validateHashString checks that a commit identifier is a plausible git object
// name. The value is used verbatim as a filesystem path element, so anything
// but hexadecimal characters (path separators, "..", spaces) is rejected to
// prevent a crafted commit argument from escaping the nigiri root.
//
// Parameters:
//   - kind: The field name used in error messages
//   - value: The commit identifier to validate
//
// Returns:
//   - error: An error describing why the identifier is invalid, or nil if it is valid
func validateHashString(kind, value string) error {
	if value == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if len(value) < ShortHashLength {
		return fmt.Errorf("%s is too short: %s", kind, value)
	}
	if len(value) > maxHashLength {
		return fmt.Errorf("%s is too long: %s", kind, value)
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return fmt.Errorf("invalid %s %q: must contain hexadecimal characters only", kind, value)
	}
	return nil
}

// Validate checks if the commit has valid hash and short hash values
//
// Returns:
//   - error: Any error encountered during validation
func (c *Commit) Validate() error {
	if err := validateHashString("hash", c.Hash); err != nil {
		return err
	}
	return validateHashString("short hash", c.ShortHash)
}

// CalculateShortHash calculates the short hash from the full hash
//
// Returns:
//   - error: Any error encountered during the calculation
func (c *Commit) CalculateShortHash() error {
	if err := validateHashString("hash", c.Hash); err != nil {
		return err
	}
	c.ShortHash = c.Hash[:ShortHashLength]
	return nil
}
