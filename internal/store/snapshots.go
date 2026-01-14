package store

import (
	"context"
	"database/sql"
	"fmt"
)

// CreateSnapshot creates a new snapshot in the database
func (db *DB) CreateSnapshot(ctx context.Context, s *Snapshot) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO snapshots (id, puck_id, puck_name, name, path, size_bytes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.PuckID, s.PuckName, s.Name, s.Path, s.SizeBytes, s.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting snapshot: %w", err)
	}

	return nil
}

// GetSnapshot retrieves a snapshot by puck ID and name
func (db *DB) GetSnapshot(ctx context.Context, puckID, name string) (*Snapshot, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, puck_id, puck_name, name, path, size_bytes, created_at
		FROM snapshots WHERE puck_id = ? AND name = ?
	`, puckID, name)

	var s Snapshot
	err := row.Scan(&s.ID, &s.PuckID, &s.PuckName, &s.Name, &s.Path, &s.SizeBytes, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot '%s' not found for puck", name)
	}
	if err != nil {
		return nil, fmt.Errorf("scanning snapshot: %w", err)
	}

	return &s, nil
}

// ListSnapshots returns all snapshots for a puck
func (db *DB) ListSnapshots(ctx context.Context, puckID string) ([]*Snapshot, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, puck_id, puck_name, name, path, size_bytes, created_at
		FROM snapshots WHERE puck_id = ? ORDER BY created_at DESC
	`, puckID)
	if err != nil {
		return nil, fmt.Errorf("querying snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.PuckID, &s.PuckName, &s.Name, &s.Path, &s.SizeBytes, &s.CreatedAt); err != nil {
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

// DeleteSnapshotsByPuck deletes all snapshots for a puck
func (db *DB) DeleteSnapshotsByPuck(ctx context.Context, puckID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM snapshots WHERE puck_id = ?`, puckID)
	return err
}
