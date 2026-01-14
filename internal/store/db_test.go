package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("creates database file", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// Verify database file was created
		_, err = os.Stat(dbPath)
		assert.NoError(t, err)
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Nested path that doesn't exist
		dbPath := filepath.Join(dir, "nested", "dirs", "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// Verify nested directories were created
		_, err = os.Stat(filepath.Dir(dbPath))
		assert.NoError(t, err)
	})

	t.Run("migrations are idempotent", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")

		// Open database first time
		db1, err := Open(dbPath)
		require.NoError(t, err)
		db1.Close()

		// Open database second time - migrations should run without error
		db2, err := Open(dbPath)
		require.NoError(t, err)
		defer db2.Close()
	})

	t.Run("creates required tables", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// Check pucks table exists
		var pucksCount int
		err = db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='pucks'").Scan(&pucksCount)
		require.NoError(t, err)
		assert.Equal(t, 1, pucksCount)

		// Check snapshots table exists
		var snapshotsCount int
		err = db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='snapshots'").Scan(&snapshotsCount)
		require.NoError(t, err)
		assert.Equal(t, 1, snapshotsCount)
	})

	t.Run("creates indexes", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// Check indexes exist
		var indexCount int
		err = db.conn.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='index' AND name IN ('idx_pucks_name', 'idx_pucks_status', 'idx_snapshots_puck')
		`).Scan(&indexCount)
		require.NoError(t, err)
		assert.Equal(t, 3, indexCount)
	})

	t.Run("enables foreign keys", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)
		defer db.Close()

		var fkEnabled int
		err = db.conn.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
		require.NoError(t, err)
		assert.Equal(t, 1, fkEnabled, "foreign keys should be enabled")
	})

	t.Run("enables WAL mode", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)
		defer db.Close()

		var journalMode string
		err = db.conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
		require.NoError(t, err)
		assert.Equal(t, "wal", journalMode, "journal mode should be WAL")
	})
}

func TestClose(t *testing.T) {
	t.Run("closes without error", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		dbPath := filepath.Join(dir, "test.db")
		db, err := Open(dbPath)
		require.NoError(t, err)

		err = db.Close()
		assert.NoError(t, err)
	})
}

func TestIsDuplicateColumnError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "duplicate column error",
			err:      fmt.Errorf("duplicate column name: foo"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDuplicateColumnError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTableExistsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "already exists error",
			err:      fmt.Errorf("table pucks already exists"),
			expected: true,
		},
		{
			name:     "another table error",
			err:      fmt.Errorf("there is already another table named pucks"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTableExistsError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

