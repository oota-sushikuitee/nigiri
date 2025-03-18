package fsutils

import (
	"os"
	"path/filepath"
)

// MakeDir creates a directory if it does not already exist
//
// Parameters:
//   - dir: The directory path to create
//
// Returns:
//   - error: Any error encountered during the process
func MakeDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// RemoveAllContents removes all contents from a directory
//
// Parameters:
//   - dir: The directory path to clean
//
// Returns:
//   - error: Any error encountered during the process
func RemoveAllContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveIfExists removes a file or directory if it exists
//
// Parameters:
//   - path: The path to the file or directory to remove
//
// Returns:
//   - error: Any error encountered during the process
func RemoveIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(path)
}
