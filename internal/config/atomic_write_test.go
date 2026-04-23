package config

import (
	"path/filepath"
	"testing"
)

func TestFsyncDirectoryWindowsSkipsDirectorySync(t *testing.T) {
	previousGOOS := atomicGOOS
	atomicGOOS = "windows"
	defer func() {
		atomicGOOS = previousGOOS
	}()

	missingDir := filepath.Join(t.TempDir(), "missing")
	if err := fsyncDirectory(missingDir); err != nil {
		t.Fatalf("fsyncDirectory() should be noop on windows, got %v", err)
	}
}

func TestFsyncDirectoryNonWindowsReturnsOpenErrorForMissingDirectory(t *testing.T) {
	previousGOOS := atomicGOOS
	atomicGOOS = "linux"
	defer func() {
		atomicGOOS = previousGOOS
	}()

	missingDir := filepath.Join(t.TempDir(), "missing")
	if err := fsyncDirectory(missingDir); err == nil {
		t.Fatalf("expected fsyncDirectory() to fail for missing directory on non-windows")
	}
}

func TestFsyncDirectoryNonWindowsSucceedsForExistingDirectory(t *testing.T) {
	previousGOOS := atomicGOOS
	atomicGOOS = "linux"
	defer func() {
		atomicGOOS = previousGOOS
	}()

	dir := t.TempDir()
	if err := fsyncDirectory(dir); err != nil {
		t.Fatalf("fsyncDirectory() error = %v", err)
	}
}
