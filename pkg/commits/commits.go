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

// Validate checks if the commit has valid hash and short hash values
//
// Returns:
//   - error: Any error encountered during validation
func (c *Commit) Validate() error {
	if c.Hash == "" {
		return fmt.Errorf("hash is empty")
	}
	if len(c.Hash) < 7 {
		return fmt.Errorf("hash is too short: %s", c.Hash)
	}
	if c.ShortHash == "" {
		return fmt.Errorf("short hash is empty")
	}
	if len(c.ShortHash) < 7 {
		return fmt.Errorf("short hash is too short: %s", c.ShortHash)
	}
	return nil
}

// CalculateShortHash calculates the short hash from the full hash
//
// Returns:
//   - error: Any error encountered during the calculation
func (c *Commit) CalculateShortHash() error {
	if len(c.Hash) < 7 {
		return fmt.Errorf("hash is too short: %s", c.Hash)
	}
	c.ShortHash = c.Hash[:7]
	return nil
}
