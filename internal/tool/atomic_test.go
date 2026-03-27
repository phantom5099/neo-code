package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteReplacesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "important.txt")

	if err := os.WriteFile(targetFile, []byte("original data"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	finalContent := []byte("final safe data")
	if err := AtomicWrite(targetFile, finalContent); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	updatedContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(updatedContent) != string(finalContent) {
		t.Fatalf("expected %q, got %q", string(finalContent), string(updatedContent))
	}
}

func TestAtomicWriteCreatesParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	deepFile := filepath.Join(tmpDir, "a", "b", "c", "test.txt")

	if err := AtomicWrite(deepFile, []byte("hello")); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	if _, err := os.Stat(deepFile); err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
}
