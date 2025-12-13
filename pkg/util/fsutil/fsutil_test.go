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
