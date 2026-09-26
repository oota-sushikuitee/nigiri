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

func TestExtractTarGz_NoEscape(t *testing.T) {
	sym := func(name, target string) *tar.Header {
		return &tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target}
	}
	reg := func(name string) *tar.Header {
		return &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0644, Size: 3}
	}
	hard := func(name, target string) *tar.Header {
		return &tar.Header{Name: name, Typeflag: tar.TypeLink, Linkname: target}
	}

	tests := []struct {
		name    string
		entries []*tar.Header
	}{
		{"parent traversal", []*tar.Header{reg("../escape.txt")}},
		{"absolute name", []*tar.Header{reg("/escape.txt")}},
		{"write through escaping symlink", []*tar.Header{sym("d", ".."), reg("d/escape.txt")}},
		{"write through absolute symlink", []*tar.Header{sym("d", "PARENT"), reg("d/escape.txt")}},
		{"symlink planted via symlinked parent", []*tar.Header{sym("a", "."), sym("a/b", ".."), reg("b/escape.txt")}},
		{"symlink chain", []*tar.Header{sym("l1", "l2"), sym("a", "."), sym("a/l2", ".."), reg("l1/escape.txt")}},
		{"hard link to outside file", []*tar.Header{hard("h", "../secret.txt")}},
		{"hard link through symlink", []*tar.Header{sym("a", "."), sym("a/b", ".."), hard("h", "b/secret.txt")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			secret := filepath.Join(parent, "secret.txt")
			if err := os.WriteFile(secret, []byte("secret"), 0600); err != nil {
				t.Fatal(err)
			}
			dstDir := filepath.Join(parent, "dst")
			if err := os.Mkdir(dstDir, 0755); err != nil {
				t.Fatal(err)
			}

			bodies := map[string]string{}
			for _, h := range tt.entries {
				if h.Linkname == "PARENT" {
					h.Linkname = parent
				}
				if h.Typeflag == tar.TypeReg {
					bodies[h.Name] = "bad"
				}
			}
			archive := filepath.Join(t.TempDir(), "mal.tar.gz")
			writeTarGz(t, archive, tt.entries, bodies)

			_ = extractTarGz(archive, dstDir)

			if _, err := os.Lstat(filepath.Join(parent, "escape.txt")); err == nil {
				t.Errorf("archive wrote outside the extraction root")
			}
			secretInfo, err := os.Stat(secret)
			if err != nil {
				t.Fatal(err)
			}
			_ = filepath.WalkDir(dstDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || !d.Type().IsRegular() {
					return err
				}
				if info, err := os.Lstat(path); err == nil && os.SameFile(info, secretInfo) {
					t.Errorf("%s is a hard link to a file outside the extraction root", path)
				}
				return nil
			})
		})
	}
}
