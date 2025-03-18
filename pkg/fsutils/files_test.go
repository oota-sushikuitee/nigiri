package fsutils

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "nigiri-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tempDir
}

func cleanupTestDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("Failed to clean up test dir: %v", err)
	}
}

func TestMakeDir(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Test creating a new directory
	newDir := filepath.Join(testDir, "new-dir")
	if err := MakeDir(newDir); err != nil {
		t.Errorf("MakeDir() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created")
	}

	// Test creating an existing directory (should not error)
	if err := MakeDir(newDir); err != nil {
		t.Errorf("MakeDir() on existing dir error = %v", err)
	}
}

func TestRemoveAllContents(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create some test files and directories
	files := []string{"file1.txt", "file2.txt"}
	dirs := []string{"dir1", "dir2"}

	for _, file := range files {
		path := filepath.Join(testDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	for _, dir := range dirs {
		path := filepath.Join(testDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	// Remove all contents
	if err := RemoveAllContents(testDir); err != nil {
		t.Errorf("RemoveAllContents() error = %v", err)
	}

	// Verify contents were removed
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Directory still contains %d entries", len(entries))
	}

	// Verify the directory itself still exists
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Errorf("Directory itself was removed")
	}
}

func TestRemoveIfExists(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Test with non-existent path
	nonExistentPath := filepath.Join(testDir, "non-existent")
	if err := RemoveIfExists(nonExistentPath); err != nil {
		t.Errorf("RemoveIfExists() on non-existent path error = %v", err)
	}

	// Test with existing file
	filePath := filepath.Join(testDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := RemoveIfExists(filePath); err != nil {
		t.Errorf("RemoveIfExists() on existing file error = %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("File still exists")
	}

	// Test with existing directory
	dirPath := filepath.Join(testDir, "test-dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	if err := RemoveIfExists(dirPath); err != nil {
		t.Errorf("RemoveIfExists() on existing directory error = %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Errorf("Directory still exists")
	}
}
