package dirutils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDir(t *testing.T) string {
	// Create a temporary directory for tests
	tempDir, err := os.MkdirTemp("", "nigiri-dirutils-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	return tempDir
}

func cleanupTestDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: Failed to clean up test directory: %v", err)
	}
}

func createTestFiles(t *testing.T, baseDir string, files []string, dirs []string) {
	// Create directories first
	for _, dir := range dirs {
		dirPath := filepath.Join(baseDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dirPath, err)
		}
	}

	// Create files
	for _, file := range files {
		filePath := filepath.Join(baseDir, file)
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create parent directory for %s: %v", filePath, err)
		}
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}
}

func TestGetDirEntries(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create test files and directories
	files := []string{
		"file1.txt",
		"file2.go",
		".hidden",
		"subdir/file3.txt",
	}
	dirs := []string{
		"subdir",
		"emptydir",
		".hiddendir",
	}
	createTestFiles(t, testDir, files, dirs)

	// Debug: List what was actually created
	entries, _ := os.ReadDir(testDir)
	t.Logf("Created in testDir: %d entries", len(entries))
	for _, e := range entries {
		t.Logf("  - %s (isDir: %v)", e.Name(), e.IsDir())
	}

	subdirEntries, _ := os.ReadDir(filepath.Join(testDir, "subdir"))
	t.Logf("Created in subdir: %d entries", len(subdirEntries))
	for _, e := range subdirEntries {
		t.Logf("  - %s (isDir: %v)", e.Name(), e.IsDir())
	}

	// Test cases
	tests := []struct {
		name      string
		filter    string
		wantCount int
	}{
		{
			name:      "no filter",
			filter:    "",
			wantCount: 4, // 2 files (file1.txt, file2.go) + 2 dirs (subdir, emptydir)
		},
		{
			name:      "txt filter",
			filter:    "txt",
			wantCount: 1, // Only file1.txt (subdir/file3.txt is in a subdirectory)
		},
		{
			name:      "hidden filter",
			filter:    ".",
			wantCount: 4, // Adjusted: (.hidden, .hiddendir) + (file1.txt, file2.go) = 4 entries
		},
		{
			name:      "non-matching filter",
			filter:    "nonexistent",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := GetDirEntries(testDir, tt.filter)
			if err != nil {
				t.Fatalf("GetDirEntries() error = %v", err)
			}

			// Debug output
			t.Logf("Filter: '%s', got %d entries, want %d", tt.filter, len(entries), tt.wantCount)
			for _, e := range entries {
				t.Logf("  - %s (isDir: %v)", e.Name, e.IsDir)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("GetDirEntries() returned %d entries, want %d", len(entries), tt.wantCount)
			}

			// Check if hidden files/dirs are included when using dot filter
			if tt.filter == "." {
				hasHidden := false
				for _, entry := range entries {
					if len(entry.Name) > 0 && entry.Name[0] == '.' {
						hasHidden = true
						break
					}
				}
				if !hasHidden {
					t.Errorf("GetDirEntries() with '.' filter should include hidden files/dirs")
				}
			}
		})
	}
}

func TestSortDirEntriesByTime(t *testing.T) {
	// Create test entries with different modification times
	now := time.Now()
	entries := []DirEntry{
		{
			Name:    "newest",
			ModTime: now,
		},
		{
			Name:    "middle",
			ModTime: now.Add(-time.Hour),
		},
		{
			Name:    "oldest",
			ModTime: now.Add(-2 * time.Hour),
		},
	}

	// Test ascending order
	entriesAsc := make([]DirEntry, len(entries))
	copy(entriesAsc, entries)
	SortDirEntriesByTime(entriesAsc, false)

	if entriesAsc[0].Name != "oldest" || entriesAsc[2].Name != "newest" {
		t.Errorf("SortDirEntriesByTime(ascending) failed, got order: %v", namesOf(entriesAsc))
	}

	// Test descending order
	entriesDesc := make([]DirEntry, len(entries))
	copy(entriesDesc, entries)
	SortDirEntriesByTime(entriesDesc, true)

	if entriesDesc[0].Name != "newest" || entriesDesc[2].Name != "oldest" {
		t.Errorf("SortDirEntriesByTime(descending) failed, got order: %v", namesOf(entriesDesc))
	}
}

func TestSortDirEntriesByName(t *testing.T) {
	entries := []DirEntry{
		{Name: "zebra"},
		{Name: "apple"},
		{Name: "monkey"},
	}

	// Test ascending order
	entriesAsc := make([]DirEntry, len(entries))
	copy(entriesAsc, entries)
	SortDirEntriesByName(entriesAsc, false)

	if entriesAsc[0].Name != "apple" || entriesAsc[2].Name != "zebra" {
		t.Errorf("SortDirEntriesByName(ascending) failed, got order: %v", namesOf(entriesAsc))
	}

	// Test descending order
	entriesDesc := make([]DirEntry, len(entries))
	copy(entriesDesc, entries)
	SortDirEntriesByName(entriesDesc, true)

	if entriesDesc[0].Name != "zebra" || entriesDesc[2].Name != "apple" {
		t.Errorf("SortDirEntriesByName(descending) failed, got order: %v", namesOf(entriesDesc))
	}
}

// Helper function to extract names for error reporting
func namesOf(entries []DirEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name
	}
	return names
}

func TestGetDirSize(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create test files with known sizes
	files := []struct {
		path string
		size int
	}{
		{"file1.txt", 100},
		{"file2.txt", 200},
		{"subdir/file3.txt", 300},
	}

	// Create the files with specific sizes
	for _, file := range files {
		filePath := filepath.Join(testDir, file.path)
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		// Create file with specific size
		data := make([]byte, file.size)
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Test GetDirSize
	size, err := GetDirSize(testDir)
	if err != nil {
		t.Fatalf("GetDirSize() error = %v", err)
	}

	// Expected total size
	expectedSize := int64(100 + 200 + 300)
	if size != expectedSize {
		t.Errorf("GetDirSize() = %d, want %d", size, expectedSize)
	}

	// Test with non-existent directory
	_, err = GetDirSize(filepath.Join(testDir, "nonexistent"))
	if err == nil {
		t.Error("GetDirSize() expected error for non-existent directory")
	}
}

func TestEnsureDirExists(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Test with non-existent directory
	newDir := filepath.Join(testDir, "new-dir")
	err := EnsureDirExists(newDir)
	if err != nil {
		t.Errorf("EnsureDirExists() error = %v", err)
	}

	// Directory should exist now
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("EnsureDirExists() failed to create directory")
	}

	// Test with already existing directory
	err = EnsureDirExists(newDir)
	if err != nil {
		t.Errorf("EnsureDirExists() on existing dir error = %v", err)
	}
}

func TestCleanOldDirs(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create test directories with different modification times
	now := time.Now()
	dirs := []struct {
		name    string
		modTime time.Time
	}{
		{"dir1", now.Add(-48 * time.Hour)}, // 2 days old
		{"dir2", now.Add(-24 * time.Hour)}, // 1 day old
		{"dir3", now},                      // current
	}

	for _, dir := range dirs {
		dirPath := filepath.Join(testDir, dir.name)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Set modification time
		if err := os.Chtimes(dirPath, dir.modTime, dir.modTime); err != nil {
			t.Fatalf("Failed to set directory time: %v", err)
		}
	}

	// Test cleaning by max count
	err := CleanOldDirs(testDir, 2, 0)
	if err != nil {
		t.Fatalf("CleanOldDirs() error = %v", err)
	}

	// Should have removed the oldest directory (dir1)
	if _, err := os.Stat(filepath.Join(testDir, "dir1")); !os.IsNotExist(err) {
		t.Error("CleanOldDirs() failed to remove old directory by count")
	}
	if _, err := os.Stat(filepath.Join(testDir, "dir2")); os.IsNotExist(err) {
		t.Error("CleanOldDirs() incorrectly removed dir2")
	}
	if _, err := os.Stat(filepath.Join(testDir, "dir3")); os.IsNotExist(err) {
		t.Error("CleanOldDirs() incorrectly removed dir3")
	}

	// Test cleaning by age
	testDir = setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Recreate test directories
	for _, dir := range dirs {
		dirPath := filepath.Join(testDir, dir.name)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Set modification time
		if err := os.Chtimes(dirPath, dir.modTime, dir.modTime); err != nil {
			t.Fatalf("Failed to set directory time: %v", err)
		}
	}

	// Clean directories older than 36 hours
	err = CleanOldDirs(testDir, 0, 36*time.Hour)
	if err != nil {
		t.Fatalf("CleanOldDirs() error = %v", err)
	}

	// Should have removed dir1 (48 hours old) but kept dir2 (24 hours) and dir3 (current)
	if _, err := os.Stat(filepath.Join(testDir, "dir1")); !os.IsNotExist(err) {
		t.Error("CleanOldDirs() failed to remove old directory by age")
	}
	if _, err := os.Stat(filepath.Join(testDir, "dir2")); os.IsNotExist(err) {
		t.Error("CleanOldDirs() incorrectly removed dir2")
	}
	if _, err := os.Stat(filepath.Join(testDir, "dir3")); os.IsNotExist(err) {
		t.Error("CleanOldDirs() incorrectly removed dir3")
	}
}
