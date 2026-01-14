// Package testutil provides shared test utilities for puck tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sandwich-labs/puck/internal/store"
)

// TempDB creates a temporary SQLite database for testing.
// Returns the database and a cleanup function.
func TempDB(t *testing.T) (*store.DB, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "puck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open test db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return db, cleanup
}

// TempDir creates a temporary directory for testing.
// Returns the path and a cleanup function.
func TempDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "puck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// MustMkdir creates a directory or fails the test.
func MustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", path, err)
	}
}

// MustWriteFile writes content to a file or fails the test.
func MustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// MustReadFile reads a file or fails the test.
func MustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return content
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
