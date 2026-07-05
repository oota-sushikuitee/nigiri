package commands

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// writeTarGz builds a tar.gz archive at path from the provided entries. A nil
// or empty body is written for entries whose type carries no content.
func writeTarGz(t *testing.T, path string, entries []*tar.Header, bodies map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, h := range entries {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatalf("write header %s: %v", h.Name, err)
		}
		if body, ok := bodies[h.Name]; ok {
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatalf("write body %s: %v", h.Name, err)
			}
		}
	}
}

func TestCompressExtract_RoundTripPreservesSymlink(t *testing.T) {
	srcDir := t.TempDir()

	// A regular file and a symlink pointing to it (within the archive root).
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Symlink("file.txt", filepath.Join(srcDir, "link.txt")); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	archive := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := compressDirectory(srcDir, archive); err != nil {
		t.Fatalf("compressDirectory: %v", err)
	}

	dstDir := t.TempDir()
	if err := extractTarGz(archive, dstDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// The extracted link must still be a symlink pointing at the original target.
	info, err := os.Lstat(filepath.Join(dstDir, "link.txt"))
	if err != nil {
		t.Fatalf("lstat extracted link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("extracted link.txt is not a symlink (mode %v)", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(dstDir, "link.txt"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "file.txt" {
		t.Errorf("symlink target = %q, want %q", target, "file.txt")
	}

	// The regular file must round-trip its content.
	content, err := os.ReadFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("file content = %q, want %q", content, "hello")
	}
}

func TestExtractTarGz_MaliciousEntries(t *testing.T) {
	tests := []struct {
		name    string
		header  *tar.Header
		body    string
		wantErr bool
	}{
		{
			name:    "parent traversal in name",
			header:  &tar.Header{Name: "../escape.txt", Typeflag: tar.TypeReg, Mode: 0644, Size: 3},
			body:    "bad",
			wantErr: true,
		},
		{
			name:    "deep parent traversal in name",
			header:  &tar.Header{Name: "../../../etc/evil.txt", Typeflag: tar.TypeReg, Mode: 0644, Size: 3},
			body:    "bad",
			wantErr: true,
		},
		{
			name:    "absolute path is contained under destDir",
			header:  &tar.Header{Name: "/abs.txt", Typeflag: tar.TypeReg, Mode: 0644, Size: 3},
			body:    "ok!",
			wantErr: false,
		},
		{
			name:    "symlink escaping root via relative target",
			header:  &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../../../../etc/passwd"},
			wantErr: true,
		},
		{
			name:    "symlink escaping root via absolute target",
			header:  &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
			wantErr: true,
		},
		{
			name:    "symlink within root is allowed",
			header:  &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "safe.txt"},
			wantErr: false,
		},
		{
			name:    "hard link escaping root",
			header:  &tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "../../../etc/passwd"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "mal.tar.gz")
			bodies := map[string]string{}
			if tt.body != "" {
				bodies[tt.header.Name] = tt.body
			}
			writeTarGz(t, archive, []*tar.Header{tt.header}, bodies)

			dstDir := t.TempDir()
			err := extractTarGz(archive, dstDir)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestExtractTarGz_PrefixSiblingNotEscaped guards against the separator-unsafe
// prefix check: a destination like ".../root" must not be considered to contain
// a sibling like ".../root-evil".
func TestIsWithinDir_PrefixSibling(t *testing.T) {
	root := filepath.Join("tmp", "root")
	sibling := filepath.Join("tmp", "root-evil", "x")
	if isWithinDir(root, sibling) {
		t.Errorf("isWithinDir(%q, %q) = true, want false", root, sibling)
	}
	if !isWithinDir(root, filepath.Join(root, "sub", "file")) {
		t.Errorf("isWithinDir did not contain a genuine child path")
	}
}
