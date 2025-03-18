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
//   - SizeInKB: The size of the entry in kilobytes
//   - Permission: The file permissions
//   - IsDir: Whether the entry is a directory
type DirEntry struct {
	ModTime    time.Time
	Name       string
	SizeInKB   int64
	Permission os.FileMode
	IsDir      bool
}

// SortDirEntriesByTime sorts directory entries by modification time
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

// SortDirEntriesByName sorts directory entries by name
func SortDirEntriesByName(entries []DirEntry, descending bool) {
	if descending {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name > entries[j].Name
		})
	} else {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name < entries[j].Name
		})
	}
}

// GetDirEntries returns a list of directory entries
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
			Name:       entry.Name(),
			ModTime:    info.ModTime(),
			IsDir:      entry.IsDir(),
			SizeInKB:   info.Size() / 1024,
			Permission: info.Mode().Perm(),
		})
	}

	return result, nil
}

// GetDirSize calculates the total size of a directory in bytes
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
	return size, err
}

// EnsureDirExists ensures that the specified directory exists
func EnsureDirExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// CleanOldDirs removes old directories based on a maximum count or age
func CleanOldDirs(parentDir string, maxDirs int, maxAge time.Duration) error {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return logger.CreateErrorf("failed to read directory: %w", err)
	}

	var dirs []DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			dirs = append(dirs, DirEntry{
				Name:    entry.Name(),
				ModTime: info.ModTime(),
				IsDir:   true,
			})
		}
	}

	// Sort by modification time (oldest first)
	SortDirEntriesByTime(dirs, false)

	// Remove old directories by count
	if maxDirs > 0 && len(dirs) > maxDirs {
		for i := 0; i < len(dirs)-maxDirs; i++ {
			dirToRemove := filepath.Join(parentDir, dirs[i].Name)
			if err := os.RemoveAll(dirToRemove); err != nil {
				return logger.CreateErrorf("failed to remove directory %s: %w", dirToRemove, err)
			}
		}
	}

	// Remove old directories by age
	if maxAge > 0 {
		now := time.Now()
		for _, dir := range dirs {
			if now.Sub(dir.ModTime) > maxAge {
				dirToRemove := filepath.Join(parentDir, dir.Name)
				if err := os.RemoveAll(dirToRemove); err != nil {
					return logger.CreateErrorf("failed to remove directory %s: %w", dirToRemove, err)
				}
			}
		}
	}

	return nil
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
