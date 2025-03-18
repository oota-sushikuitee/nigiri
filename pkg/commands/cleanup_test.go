package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestCleanupCommand tests the cleanup command functionality
func TestCleanupCommand(t *testing.T) {
	// Set up a custom test directory for nigiriRoot
	originalNigiriRoot := nigiriRoot
	tempDir, err := os.MkdirTemp("", "nigiri-cleanup-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	defer func() { nigiriRoot = originalNigiriRoot }()
	nigiriRoot = tempDir

	// Create test targets
	setupTestTargets(t, tempDir)

	t.Run("Show disk usage", func(t *testing.T) {
		// Create a buffer to capture command output
		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, nil)

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output
		output := stdout.String()
		if !strings.Contains(output, "Disk usage by target") {
			t.Errorf("Expected disk usage header, got: %s", output)
		}
		if !strings.Contains(output, "test-target-1:") {
			t.Errorf("Expected test target in output, got: %s", output)
		}
		if !strings.Contains(output, "Total disk usage:") {
			t.Errorf("Expected total disk usage in output, got: %s", output)
		}
	})

	t.Run("Cleanup with dry run", func(t *testing.T) {
		// Create a buffer to capture command output
		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, nil, "--dry-run", "test-target-1")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output
		output := stdout.String()
		if !strings.Contains(output, "Dry run: No builds were removed") {
			t.Errorf("Expected dry run message, got: %s", output)
		}

		// Verify no files were actually removed
		builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-1", "*"))
		if len(builds) != 7 { // We created 7 builds in setupTestTargets
			t.Errorf("Expected 7 builds to remain, got %d", len(builds))
		}
	})

	t.Run("Cleanup with max-builds parameter", func(t *testing.T) {
		// Create a buffer to capture command output
		var stdout bytes.Buffer
		// --yes to skip confirmation, keep 3 latest builds
		cmd := setupCleanupTestCommand(&stdout, nil, "--yes", "--max-builds", "3", "test-target-1")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output
		output := stdout.String()
		if !strings.Contains(output, "builds removed successfully") {
			t.Errorf("Expected success message, got: %s", output)
		}

		// Verify only 3 builds remain
		builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-1", "*"))
		if len(builds) != 3 {
			t.Errorf("Expected 3 builds to remain, got %d", len(builds))
		}
	})

	t.Run("Cleanup with max-age parameter", func(t *testing.T) {
		// Reset test targets
		os.RemoveAll(tempDir)
		setupTestTargets(t, tempDir)

		// Create a buffer to capture command output
		var stdout bytes.Buffer
		// --yes to skip confirmation, keep builds newer than 10 days
		cmd := setupCleanupTestCommand(&stdout, nil, "--yes", "--max-age", "10", "test-target-2")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// In our test setup, 4 builds are "newer" (less than 10 days old)
		builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-2", "*"))
		if len(builds) != 4 {
			t.Errorf("Expected 4 builds to remain, got %d", len(builds))
		}
	})

	t.Run("Cleanup with user confirmation - yes", func(t *testing.T) {
		// Reset test targets
		os.RemoveAll(tempDir)
		setupTestTargets(t, tempDir)

		// Mock user input
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		defer func() { os.Stdin = oldStdin }()

		// Create input in a goroutine
		go func() {
			defer w.Close()
			w.Write([]byte("y\n"))
		}()

		// Create a buffer to capture command output
		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, r, "--max-builds", "2", "test-target-1")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output
		output := stdout.String()
		if !strings.Contains(output, "Do you want to continue?") {
			t.Errorf("Expected confirmation prompt, got: %s", output)
		}
		if !strings.Contains(output, "builds removed successfully") {
			t.Errorf("Expected success message, got: %s", output)
		}

		// Verify only 2 builds remain
		builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-1", "*"))
		if len(builds) != 2 {
			t.Errorf("Expected 2 builds to remain, got %d", len(builds))
		}
	})

	t.Run("Cleanup with user confirmation - no", func(t *testing.T) {
		// Reset test targets
		os.RemoveAll(tempDir)
		setupTestTargets(t, tempDir)

		// Mock user input
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		defer func() { os.Stdin = oldStdin }()

		// Create input in a goroutine
		go func() {
			defer w.Close()
			w.Write([]byte("n\n"))
		}()

		// Create a buffer to capture command output
		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, r, "--max-builds", "2", "test-target-1")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output
		output := stdout.String()
		if !strings.Contains(output, "Do you want to continue?") {
			t.Errorf("Expected confirmation prompt, got: %s", output)
		}
		if !strings.Contains(output, "Cleanup cancelled") {
			t.Errorf("Expected cancellation message, got: %s", output)
		}

		// Verify all builds remain
		builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-1", "*"))
		if len(builds) != 7 {
			t.Errorf("Expected 7 builds to remain, got %d", len(builds))
		}
	})

	t.Run("Cleanup all targets", func(t *testing.T) {
		// Reset test targets
		os.RemoveAll(tempDir)
		setupTestTargets(t, tempDir)

		// Create a buffer to capture command output
		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, nil, "--yes", "--all", "--max-builds", "2")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify only 2 builds remain for each target
		target1Builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-1", "*"))
		if len(target1Builds) != 2 {
			t.Errorf("Expected 2 builds to remain for target-1, got %d", len(target1Builds))
		}

		target2Builds, _ := filepath.Glob(filepath.Join(tempDir, "test-target-2", "*"))
		if len(target2Builds) != 2 {
			t.Errorf("Expected 2 builds to remain for target-2, got %d", len(target2Builds))
		}
	})

	t.Run("Cleanup non-existent target", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, nil, "non-existent-target")

		// Execute the command
		err := cmd.Execute()
		if err == nil {
			t.Fatal("Expected error for non-existent target, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})

	t.Run("Cleanup with no builds to remove", func(t *testing.T) {
		// Create a new target with only 1 build
		targetDir := filepath.Join(tempDir, "empty-target")
		os.MkdirAll(targetDir, 0755)
		createTestBuild(t, targetDir, "build-1", time.Now())

		var stdout bytes.Buffer
		// Set max-builds to 3 (more than we have)
		cmd := setupCleanupTestCommand(&stdout, nil, "--max-builds", "3", "empty-target")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output contains appropriate message
		output := stdout.String()
		if !strings.Contains(output, "No builds to remove") {
			t.Errorf("Expected 'no builds to remove' message, got: %s", output)
		}
	})

	t.Run("Cleanup with no targets found", func(t *testing.T) {
		// Remove all targets
		os.RemoveAll(tempDir)
		os.MkdirAll(tempDir, 0755)

		var stdout bytes.Buffer
		cmd := setupCleanupTestCommand(&stdout, nil, "--all")

		// Execute the command
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify output
		output := stdout.String()
		if !strings.Contains(output, "No targets found") {
			t.Errorf("Expected 'no targets found' message, got: %s", output)
		}
	})
}

// setupCleanupTestCommand creates a configured cleanup command for testing with arguments
func setupCleanupTestCommand(out io.Writer, in io.Reader, args ...string) *cobra.Command {
	cmd := newCleanupCommand().cmd
	cmd.SetOut(out)
	if in != nil {
		cmd.SetIn(in)
	}
	cmd.SetArgs(args)
	return cmd
}

// setupTestTargets creates test targets and builds with different dates
func setupTestTargets(t *testing.T, rootDir string) {
	// Test target 1 - create 7 builds with different dates
	target1Dir := filepath.Join(rootDir, "test-target-1")
	os.MkdirAll(target1Dir, 0755)

	now := time.Now()
	// Create builds with different dates - newest first to match SortDirEntriesByTime behavior
	createTestBuild(t, target1Dir, "build-newest", now)
	createTestBuild(t, target1Dir, "build-2", now.AddDate(0, 0, -5))
	createTestBuild(t, target1Dir, "build-3", now.AddDate(0, 0, -10))
	createTestBuild(t, target1Dir, "build-4", now.AddDate(0, 0, -15))
	createTestBuild(t, target1Dir, "build-5", now.AddDate(0, 0, -20))
	createTestBuild(t, target1Dir, "build-6", now.AddDate(0, 0, -25))
	createTestBuild(t, target1Dir, "build-oldest", now.AddDate(0, 0, -30))

	// Test target 2 - create 7 builds with different dates
	target2Dir := filepath.Join(rootDir, "test-target-2")
	os.MkdirAll(target2Dir, 0755)

	createTestBuild(t, target2Dir, "build-newest", now)
	createTestBuild(t, target2Dir, "build-2", now.AddDate(0, 0, -3))
	createTestBuild(t, target2Dir, "build-3", now.AddDate(0, 0, -6))
	createTestBuild(t, target2Dir, "build-4", now.AddDate(0, 0, -9)) // This is the cutoff for 10 days
	createTestBuild(t, target2Dir, "build-5", now.AddDate(0, 0, -12))
	createTestBuild(t, target2Dir, "build-6", now.AddDate(0, 0, -15))
	createTestBuild(t, target2Dir, "build-oldest", now.AddDate(0, 0, -18))
}

// createTestBuild creates a test build directory with some content and sets its modification time
func createTestBuild(t *testing.T, targetDir string, buildName string, modTime time.Time) {
	buildDir := filepath.Join(targetDir, buildName)
	os.MkdirAll(buildDir, 0755)

	// Create a dummy file to give the directory some size
	dummyFile := filepath.Join(buildDir, "dummy.txt")
	dummyContent := fmt.Sprintf("This is a dummy file for build %s", buildName)
	err := os.WriteFile(dummyFile, []byte(dummyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}

	// Set the modification time
	err = os.Chtimes(buildDir, modTime, modTime)
	if err != nil {
		t.Fatalf("Failed to set modification time: %v", err)
	}
}
