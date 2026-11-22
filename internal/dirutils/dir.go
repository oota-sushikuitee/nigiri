// Package dirutils provides directory listing and sizing helpers.
package dirutils

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oota-sushikuitee/nigiri/pkg/logger"
)

// DirEntry is the subset of a directory entry's metadata that callers sort and
// filter on.
type DirEntry struct {
	ModTime time.Time
	Name    string
	IsDir   bool
}

// SortDirEntriesByTime sorts entries in place by ModTime, newest first when
// descending is true.
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

// GetDirEntries returns the entries of dir whose names contain filter,
// compared case-insensitively; an empty filter matches everything. Hidden
// entries are skipped unless filter contains a dot. Entries whose metadata
// cannot be read are omitted.
func GetDirEntries(dir string, filter string) ([]DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, logger.CreateErrorf("failed to read directory: %w", err)
	}

	var result []DirEntry

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") && !strings.Contains(filter, ".") {
			continue
		}

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

// GetDirSize returns the total size in bytes of the regular files under path.
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

// Exists reports whether path exists. Stat errors other than "not exist", such
// as a permission error, count as existing.
func Exists(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}
