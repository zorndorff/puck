package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// isDuplicateColumnError checks if the error is a "duplicate column" error
func isDuplicateColumnError(err error) bool {
	return strings.Contains(err.Error(), "duplicate column")
}

// isTableExistsError checks if the error is a "table already exists" error
func isTableExistsError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "already another table")
}

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
	path string
}

// Open opens the SQLite database at the given path
func Open(path string) (*DB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	// Open database with WAL mode and foreign keys
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	db := &DB{conn: conn, path: path}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	migrations := []string{
		// Create pucks table (renamed from sprites)
		`CREATE TABLE IF NOT EXISTS pucks (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			image TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'stopped',
			volume_dir TEXT NOT NULL,
			ports TEXT DEFAULT '[]',
			host_port INTEGER DEFAULT 0,
			container_ip TEXT DEFAULT '',
			tailscale_ip TEXT DEFAULT '',
			funnel_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Migration: rename sprites table to pucks if sprites exists
		`ALTER TABLE sprites RENAME TO pucks`,
		// Migration: add host_port column if not exists
		`ALTER TABLE pucks ADD COLUMN host_port INTEGER DEFAULT 0`,
		// Create snapshots table with puck references
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			puck_id TEXT NOT NULL,
			puck_name TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			size_bytes INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (puck_id) REFERENCES pucks(id) ON DELETE CASCADE,
			UNIQUE(puck_id, name)
		)`,
		// Migration: rename sprite columns in snapshots if they exist
		`ALTER TABLE snapshots RENAME COLUMN sprite_id TO puck_id`,
		`ALTER TABLE snapshots RENAME COLUMN sprite_name TO puck_name`,
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_pucks_name ON pucks(name)`,
		`CREATE INDEX IF NOT EXISTS idx_pucks_status ON pucks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_puck ON snapshots(puck_id)`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			// Ignore expected errors from migrations
			if !isDuplicateColumnError(err) && !isTableExistsError(err) &&
				!strings.Contains(err.Error(), "no such table") &&
				!strings.Contains(err.Error(), "no such column") {
				return fmt.Errorf("executing migration: %w", err)
			}
		}
	}

	return nil
}

// ExecContext executes a query with context
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// QueryContext executes a query and returns rows
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a query and returns a single row
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}
