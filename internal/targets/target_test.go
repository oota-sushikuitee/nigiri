package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oota-sushikuitee/nigiri/pkg/commits"
)

const tmpNigiriRoot = "/tmp/nigiri"

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

func TestValidateTargetName(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "valid simple name", target: "nigiri", wantErr: false},
		{name: "valid name with hyphen", target: "my-target", wantErr: false},
		{name: "valid name with dot suffix", target: "target.v2", wantErr: false},
		{name: "empty name", target: "", wantErr: true},
		{name: "dot", target: ".", wantErr: true},
		{name: "dot dot", target: "..", wantErr: true},
		{name: "parent traversal", target: "../x", wantErr: true},
		{name: "nested traversal", target: "a/../../b", wantErr: true},
		{name: "subdirectory", target: "a/b", wantErr: true},
		{name: "absolute path", target: "/etc", wantErr: true},
		{name: "backslash separator", target: `a\b`, wantErr: true},
		{name: "backslash traversal", target: `..\x`, wantErr: true},
		{name: "traversal with trailing separator", target: "../", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTargetName(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTargetName(%q) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestTargetRootDirRejectsInvalidNames(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create a sibling directory of the nigiri root that a traversal name
	// could resolve to
	sibling := filepath.Join(filepath.Dir(testDir), "outside")
	if err := os.MkdirAll(sibling, 0755); err != nil {
		t.Fatalf("Failed to create sibling dir: %v", err)
	}
	defer os.RemoveAll(sibling)

	traversal := "../" + filepath.Base(sibling)
	invalidNames := []string{traversal, "..", "a/b", ""}

	for _, name := range invalidNames {
		tgt := Target{Target: name}
		if _, err := tgt.GetTargetRootDir(testDir); err == nil {
			t.Errorf("GetTargetRootDir(%q) should fail for invalid target name", name)
		}
		if _, err := tgt.CreateTargetRootDir(testDir); err == nil {
			t.Errorf("CreateTargetRootDir(%q) should fail for invalid target name", name)
		}
		if _, err := tgt.CreateTargetRootDirIfNotExist(testDir); err == nil {
			t.Errorf("CreateTargetRootDirIfNotExist(%q) should fail for invalid target name", name)
		}
	}

	// Ensure nothing was created outside the nigiri root
	if _, err := os.Stat(filepath.Join(testDir, "..", "b")); err == nil {
		t.Errorf("directory was created outside the nigiri root")
	}
}

func TestGetTargetRootDir(t *testing.T) {
	t.Run("Target root does not exist", func(t *testing.T) {
		tgt := Target{
			Target: "test",
		}
		_, err := tgt.GetTargetRootDir(tmpNigiriRoot)
		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})

	t.Run("Target root exists", func(t *testing.T) {
		tgt := Target{
			Target: "test",
		}
		testDir := filepath.Join(tmpNigiriRoot, "test")
		os.MkdirAll(tmpNigiriRoot, 0755)
		os.MkdirAll(testDir, 0755)
		defer os.RemoveAll(filepath.Join(tmpNigiriRoot, "test"))

		_, err := tgt.GetTargetRootDir(tmpNigiriRoot)
		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}
	})
}

func TestGetTargetHeadDir(t *testing.T) {
	t.Run("No commits found", func(t *testing.T) {
		tgt := Target{
			Target:  "test",
			Commits: commits.Commits{},
		}
		_, err := tgt.GetTargetHeadDir(tmpNigiriRoot)
		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})

	t.Run("Commits found", func(t *testing.T) {
		tgt := Target{
			Target: "test",
			Commits: commits.Commits{
				Commits: []commits.Commit{
					{
						Hash:      "commit1",
						ShortHash: "commit1",
					},
					{
						Hash:      "commit2",
						ShortHash: "commit2",
					},
				},
			}}
		testDir := filepath.Join(tmpNigiriRoot, "test", "commit2")
		os.MkdirAll(testDir, 0755)
		defer os.RemoveAll(filepath.Join(tmpNigiriRoot, "test"))

		_, err := tgt.GetTargetHeadDir(tmpNigiriRoot)
		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}
	})
}

func TestTarget_CreateTargetRootDirIfNotExist(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	target := Target{Target: "test-target"}
	_, err := target.CreateTargetRootDirIfNotExist(testDir)
	if err != nil {
		t.Errorf("CreateTargetRootDirIfNotExist() error = %v", err)
	}

	// Verify that the directory was created
	targetDir := filepath.Join(testDir, "test-target")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("Target directory was not created")
	}

	// Call again to verify it doesn't error when the directory already exists
	_, err = target.CreateTargetRootDirIfNotExist(testDir)
	if err != nil {
		t.Errorf("CreateTargetRootDirIfNotExist() on existing dir error = %v", err)
	}
}

func TestTarget_CreateTargetRootDir(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	target := Target{Target: "test-target"}
	// First call should succeed
	_, err := target.CreateTargetRootDir(testDir)
	if err != nil {
		t.Errorf("CreateTargetRootDir() first call error = %v", err)
	}

	// Second call should fail since directory already exists
	_, err = target.CreateTargetRootDir(testDir)
	if err == nil {
		t.Errorf("CreateTargetRootDir() second call should return error but didn't")
	}
}

func TestTarget_GetTargetRootDir_Detailed(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	target := Target{Target: "test-target"}
	// Should fail if directory doesn't exist
	_, err := target.GetTargetRootDir(testDir)
	if err == nil {
		t.Errorf("GetTargetRootDir() should fail when dir doesn't exist")
	}

	// Create the directory
	targetDir := filepath.Join(testDir, "test-target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create test target dir: %v", err)
	}

	// Now it should succeed
	gotDir, err := target.GetTargetRootDir(testDir)
	if err != nil {
		t.Errorf("GetTargetRootDir() error = %v", err)
	}
	if gotDir != targetDir {
		t.Errorf("GetTargetRootDir() = %v, want %v", gotDir, targetDir)
	}
}

func TestIsExistTargetCommitDir(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create a fake commit directory
	commitDir := filepath.Join(testDir, "abcdef1")
	if err := os.MkdirAll(commitDir, 0755); err != nil {
		t.Fatalf("Failed to create test commit dir: %v", err)
	}

	// Test with existing commit
	commit := commits.Commit{ShortHash: "abcdef1"}
	if !IsExistTargetCommitDir(testDir, commit) {
		t.Errorf("IsExistTargetCommitDir() = false, want true for existing dir")
	}

	// Test with non-existing commit
	nonExistingCommit := commits.Commit{ShortHash: "123456"}
	if IsExistTargetCommitDir(testDir, nonExistingCommit) {
		t.Errorf("IsExistTargetCommitDir() = true, want false for non-existing dir")
	}
}

func TestCreateAndGetTargetCommitDir(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	commit := commits.Commit{
		Hash:      "1234567890abcdef1234567890abcdef12345678",
		ShortHash: "1234567",
	}

	// First, CreateTargetCommitDir should succeed
	createdDir, err := CreateTargetCommitDir(testDir, commit)
	if err != nil {
		t.Errorf("CreateTargetCommitDir() error = %v", err)
		return
	}
	expectedDir := filepath.Join(testDir, commit.ShortHash)
	if createdDir != expectedDir {
		t.Errorf("CreateTargetCommitDir() = %v, want %v", createdDir, expectedDir)
	}

	// Creating again should fail
	_, err = CreateTargetCommitDir(testDir, commit)
	if err == nil {
		t.Errorf("CreateTargetCommitDir() second call should return error")
	}

	// GetTargetCommitDir should succeed
	gotDir, err := GetTargetCommitDir(testDir, commit)
	if err != nil {
		t.Errorf("GetTargetCommitDir() error = %v", err)
	}
	if gotDir != expectedDir {
		t.Errorf("GetTargetCommitDir() = %v, want %v", gotDir, expectedDir)
	}

	// Test with invalid commit
	invalidCommit := commits.Commit{
		Hash:      "",
		ShortHash: "",
	}
	_, err = GetTargetCommitDir(testDir, invalidCommit)
	if err == nil {
		t.Errorf("GetTargetCommitDir() should fail with invalid commit")
	}
}

func TestTarget_GetTargetHeadDir_Detailed(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	target := Target{
		Target: "test-target",
		Commits: commits.Commits{
			Commits: []commits.Commit{
				{
					Hash:      "1234567890abcdef1234567890abcdef12345678",
					ShortHash: "1234567",
				},
				{
					Hash:      "abcdef1234567890abcdef1234567890abcdef12",
					ShortHash: "abcdef1",
				},
			},
		},
	}

	// Target dir should be created before getting head
	targetDir := filepath.Join(testDir, "test-target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}

	// Create commit directories
	commitDir1 := filepath.Join(targetDir, "1234567")
	commitDir2 := filepath.Join(targetDir, "abcdef1")
	if err := os.MkdirAll(commitDir1, 0755); err != nil {
		t.Fatalf("Failed to create commit dir 1: %v", err)
	}
	if err := os.MkdirAll(commitDir2, 0755); err != nil {
		t.Fatalf("Failed to create commit dir 2: %v", err)
	}

	// GetTargetHeadDir should return the latest commit (second one)
	headDir, err := target.GetTargetHeadDir(testDir)
	if err != nil {
		t.Errorf("GetTargetHeadDir() error = %v", err)
	}
	if headDir != commitDir2 {
		t.Errorf("GetTargetHeadDir() = %v, want %v", headDir, commitDir2)
	}

	// Test with no commits
	emptyTarget := Target{Target: "empty-target"}
	if err := os.MkdirAll(filepath.Join(testDir, "empty-target"), 0755); err != nil {
		t.Fatalf("Failed to create empty target dir: %v", err)
	}
	_, err = emptyTarget.GetTargetHeadDir(testDir)
	if err == nil {
		t.Errorf("GetTargetHeadDir() should fail with no commits")
	}
}
