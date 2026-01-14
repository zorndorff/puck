package store

import (
	"context"
	"database/sql"
	"fmt"
)

// CreateSnapshot creates a new snapshot in the database
func (db *DB) CreateSnapshot(ctx context.Context, s *Snapshot) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO snapshots (id, sprite_id, sprite_name, name, path, size_bytes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.SpriteID, s.SpriteName, s.Name, s.Path, s.SizeBytes, s.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting snapshot: %w", err)
	}

	return nil
}

// GetSnapshot retrieves a snapshot by sprite ID and name
func (db *DB) GetSnapshot(ctx context.Context, spriteID, name string) (*Snapshot, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, sprite_id, sprite_name, name, path, size_bytes, created_at
		FROM snapshots WHERE sprite_id = ? AND name = ?
	`, spriteID, name)

	var s Snapshot
	err := row.Scan(&s.ID, &s.SpriteID, &s.SpriteName, &s.Name, &s.Path, &s.SizeBytes, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot '%s' not found for sprite", name)
	}
	if err != nil {
		return nil, fmt.Errorf("scanning snapshot: %w", err)
	}

	return &s, nil
}

// ListSnapshots returns all snapshots for a sprite
func (db *DB) ListSnapshots(ctx context.Context, spriteID string) ([]*Snapshot, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, sprite_id, sprite_name, name, path, size_bytes, created_at
		FROM snapshots WHERE sprite_id = ? ORDER BY created_at DESC
	`, spriteID)
	if err != nil {
		return nil, fmt.Errorf("querying snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.SpriteID, &s.SpriteName, &s.Name, &s.Path, &s.SizeBytes, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning snapshot row: %w", err)
		}
		snapshots = append(snapshots, &s)
	}

	return snapshots, rows.Err()
}

// DeleteSnapshot deletes a snapshot by ID
func (db *DB) DeleteSnapshot(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM snapshots WHERE id = ?`, id)
	return err
}

// DeleteSnapshotsBySprite deletes all snapshots for a sprite
func (db *DB) DeleteSnapshotsBySprite(ctx context.Context, spriteID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM snapshots WHERE sprite_id = ?`, spriteID)
	return err
}
