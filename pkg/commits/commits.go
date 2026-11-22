// Package commits models git commits and validates their hashes.
package commits

import "fmt"

// Commit identifies a git commit by its full and short hash.
type Commit struct {
	Hash      string
	ShortHash string
}

const (
	// ShortHashLength is the number of leading hash characters used as the
	// on-disk directory name of a build.
	ShortHashLength = 7
	// maxHashLength is the length of a full SHA-1 commit hash.
	maxHashLength = 40
)

// validateHashString rejects anything but hexadecimal characters (path
// separators, "..", spaces) because the value is used verbatim as a filesystem
// path element, and a crafted commit argument must not escape the nigiri root.
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

// Validate reports an error unless both Hash and ShortHash are hexadecimal
// strings of ShortHashLength to 40 characters.
func (c *Commit) Validate() error {
	if err := validateHashString("hash", c.Hash); err != nil {
		return err
	}

	return validateHashString("short hash", c.ShortHash)
}

// CalculateShortHash sets ShortHash to the first ShortHashLength characters of
// Hash, after checking that Hash is a valid commit hash.
func (c *Commit) CalculateShortHash() error {
	if err := validateHashString("hash", c.Hash); err != nil {
		return err
	}

	c.ShortHash = c.Hash[:ShortHashLength]

	return nil
}
