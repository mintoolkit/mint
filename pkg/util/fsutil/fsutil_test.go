package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirExcludesWholeDirectoryWhenPatternHasDoubleStar(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	excludedDir := filepath.Join(src, "var", "lib", "postgresql", "data")
	if err := os.MkdirAll(excludedDir, 0o755); err != nil {
		t.Fatalf("failed to create excluded dir: %v", err)
	}

	excludedFile := filepath.Join(excludedDir, "file.txt")
	if err := os.WriteFile(excludedFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create excluded file: %v", err)
	}

	keepFile := filepath.Join(src, "keep.txt")
	if err := os.WriteFile(keepFile, []byte("y"), 0o644); err != nil {
		t.Fatalf("failed to create keep file: %v", err)
	}

	pattern := excludedDir + "/**"
	clone := false
	copyRelPath := true
	skipErrors := true

	if err, errs := CopyDir(clone, src, dst, copyRelPath, skipErrors, []string{pattern}, nil, nil); err != nil {
		t.Fatalf("CopyDir returned error: %v", err)
	} else if len(errs) > 0 {
		t.Fatalf("CopyDir returned copy errors: %v", errs)
	}

	excludedTarget := filepath.Join(dst, "var", "lib", "postgresql", "data")
	if _, err := os.Stat(excludedTarget); err == nil {
		t.Fatalf("expected excluded directory %q to be skipped", excludedTarget)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected error checking excluded directory: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "keep.txt")); err != nil {
		t.Fatalf("expected kept file to be copied, got error: %v", err)
	}
}

func TestCopyRegularFile(t *testing.T) {
	tt := []struct {
		name        string
		content     string
		clone       bool
		makeDir     bool
		expectError bool
	}{
		{
			name:        "copy file with content",
			content:     "hello world",
			clone:       false,
			makeDir:     true,
			expectError: false,
		},
		{
			name:        "copy empty file",
			content:     "",
			clone:       false,
			makeDir:     true,
			expectError: false,
		},
		{
			name:        "copy without makeDir to existing dir",
			content:     "test content",
			clone:       false,
			makeDir:     false,
			expectError: false,
		},
	}

	for _, test := range tt {
		// Create temp directory for test
		tmpDir, err := os.MkdirTemp("", "fsutil_test")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create source file
		srcPath := filepath.Join(tmpDir, "src", "testfile.txt")
		if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
			t.Fatalf("failed to create src dir: %v", err)
		}
		if err := os.WriteFile(srcPath, []byte(test.content), 0644); err != nil {
			t.Fatalf("failed to create src file: %v", err)
		}

		// Set up destination path
		var dstPath string
		if test.makeDir {
			dstPath = filepath.Join(tmpDir, "dst", "testfile.txt")
		} else {
			// Create dst dir first for non-makeDir tests
			dstDir := filepath.Join(tmpDir, "dst")
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				t.Fatalf("failed to create dst dir: %v", err)
			}
			dstPath = filepath.Join(dstDir, "testfile.txt")
		}

		// Copy file
		err = CopyRegularFile(test.clone, srcPath, dstPath, test.makeDir)

		if test.expectError {
			if err == nil {
				t.Errorf("test %q: expected error but got none", test.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("test %q: unexpected error: %v", test.name, err)
			continue
		}

		// Verify destination file exists and has correct content
		dstContent, err := os.ReadFile(dstPath)
		if err != nil {
			t.Errorf("test %q: failed to read dst file: %v", test.name, err)
			continue
		}

		if string(dstContent) != test.content {
			t.Errorf("test %q: content mismatch, got %q, want %q", test.name, string(dstContent), test.content)
		}
	}
}

func TestCopyRegularFilePreservesSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fsutil_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file with known content
	content := "This is test content with known size"
	srcPath := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("failed to stat src file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dst.txt")
	if err := CopyRegularFile(false, srcPath, dstPath, true); err != nil {
		t.Fatalf("CopyRegularFile failed: %v", err)
	}

	// Verify sizes match
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat dst file: %v", err)
	}

	if dstInfo.Size() != srcInfo.Size() {
		t.Errorf("size mismatch: src=%d, dst=%d", srcInfo.Size(), dstInfo.Size())
	}
}

func TestCopyRegularFileMissingSource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fsutil_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")

	err = CopyRegularFile(false, srcPath, dstPath, true)
	if err == nil {
		t.Error("expected error for missing source file, got none")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fsutil_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	content := "test content for CopyFile"
	srcPath := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dst.txt")
	if err := CopyFile(false, srcPath, dstPath, true); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read dst file: %v", err)
	}

	if string(dstContent) != content {
		t.Errorf("content mismatch: got %q, want %q", string(dstContent), content)
	}
}

func TestCopyFileWithSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fsutil_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	content := "symlink test content"
	srcPath := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	// Create symlink to source
	symlinkPath := filepath.Join(tmpDir, "src_link.txt")
	if err := os.Symlink(srcPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Copy via symlink - CopyFile should handle this
	dstPath := filepath.Join(tmpDir, "dst.txt")
	if err := CopyFile(false, symlinkPath, dstPath, true); err != nil {
		t.Fatalf("CopyFile via symlink failed: %v", err)
	}

	// Verify content was copied (not the symlink itself)
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read dst file: %v", err)
	}

	if string(dstContent) != content {
		t.Errorf("content mismatch: got %q, want %q", string(dstContent), content)
	}
}
