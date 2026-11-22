// Package dirutils provides directory listing, sizing, and cleanup helpers.
package dirutils

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oota-sushikuitee/nigiri/pkg/logger"
)

// DirEntry represents a directory entry with its name, time, and whether it's a directory
//
// Fields:
//   - ModTime: The modification time of the entry
//   - Name: The name of the entry
//   - IsDir: Whether the entry is a directory
type DirEntry struct {
	ModTime time.Time
	Name    string
	IsDir   bool
}

// SortDirEntriesByTime sorts directory entries by modification time.
func SortDirEntriesByTime(entries []DirEntry, descending bool) {
	if descending {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ModTime.After(entries[j].ModTime)
		})
	} else {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ModTime.Before(entries[j].ModTime)
		})
	}
}

// GetDirEntries returns a list of directory entries.
func GetDirEntries(dir string, filter string) ([]DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, logger.CreateErrorf("failed to read directory: %w", err)
	}

	var result []DirEntry

	for _, entry := range entries {
		// Skip hidden files/directories (starting with .) if filter doesn't explicitly include them
		if strings.HasPrefix(entry.Name(), ".") && !strings.Contains(filter, ".") {
			continue
		}

		// Apply filter if provided
		if filter != "" && !strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(filter)) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, DirEntry{
			Name:    entry.Name(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
		})
	}

	return result, nil
}

// GetDirSize calculates the total size of a directory in bytes.
func GetDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		}

		return nil
	})
	if err != nil {
		return size, logger.CreateErrorf("failed to calculate size of %s: %w", path, err)
	}

	return size, nil
}

// Exists checks if a file or directory exists at the given path
//
// Parameters:
//   - path: The path to check
//
// Returns:
//   - bool: True if the file or directory exists, false otherwise
func Exists(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}
