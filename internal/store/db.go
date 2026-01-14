package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

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
		`CREATE TABLE IF NOT EXISTS sprites (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			image TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'stopped',
			volume_dir TEXT NOT NULL,
			ports TEXT DEFAULT '[]',
			container_ip TEXT DEFAULT '',
			tailscale_ip TEXT DEFAULT '',
			funnel_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			sprite_id TEXT NOT NULL,
			sprite_name TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			size_bytes INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (sprite_id) REFERENCES sprites(id) ON DELETE CASCADE,
			UNIQUE(sprite_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sprites_name ON sprites(name)`,
		`CREATE INDEX IF NOT EXISTS idx_sprites_status ON sprites(status)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_sprite ON snapshots(sprite_id)`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
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
