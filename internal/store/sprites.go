package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Status represents the current state of a sprite
type Status string

const (
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusCheckpointed Status = "checkpointed"
	StatusCreating     Status = "creating"
	StatusError        Status = "error"
)

// Sprite represents a persistent container managed by puck
type Sprite struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	VolumeDir   string    `json:"volume_dir"`
	Ports       []string  `json:"ports,omitempty"`
	HostPort    int       `json:"host_port,omitempty"`    // Auto-assigned port for HTTP routing
	TailscaleIP string    `json:"tailscale_ip,omitempty"`
	FunnelURL   string    `json:"funnel_url,omitempty"`
	ContainerIP string    `json:"container_ip,omitempty"`
}

// Snapshot represents a checkpoint of a sprite's state
type Snapshot struct {
	ID         string    `json:"id"`
	SpriteID   string    `json:"sprite_id"`
	SpriteName string    `json:"sprite_name"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateSprite creates a new sprite in the database
func (db *DB) CreateSprite(ctx context.Context, s *Sprite) error {
	portsJSON, err := json.Marshal(s.Ports)
	if err != nil {
		return fmt.Errorf("marshaling ports: %w", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO sprites (id, name, image, status, volume_dir, ports, host_port, container_ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.Name, s.Image, s.Status, s.VolumeDir, string(portsJSON), s.HostPort, s.ContainerIP, s.CreatedAt, s.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting sprite: %w", err)
	}

	return nil
}

// GetSprite retrieves a sprite by name
func (db *DB) GetSprite(ctx context.Context, name string) (*Sprite, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, image, status, volume_dir, ports, host_port, container_ip, tailscale_ip, funnel_url, created_at, updated_at
		FROM sprites WHERE name = ?
	`, name)

	return scanSprite(row)
}

// GetSpriteByID retrieves a sprite by ID
func (db *DB) GetSpriteByID(ctx context.Context, id string) (*Sprite, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, image, status, volume_dir, ports, host_port, container_ip, tailscale_ip, funnel_url, created_at, updated_at
		FROM sprites WHERE id = ?
	`, id)

	return scanSprite(row)
}

// ListSprites returns all sprites
func (db *DB) ListSprites(ctx context.Context) ([]*Sprite, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, image, status, volume_dir, ports, host_port, container_ip, tailscale_ip, funnel_url, created_at, updated_at
		FROM sprites ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying sprites: %w", err)
	}
	defer rows.Close()

	var sprites []*Sprite
	for rows.Next() {
		s, err := scanSpriteRow(rows)
		if err != nil {
			return nil, err
		}
		sprites = append(sprites, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sprites: %w", err)
	}

	return sprites, nil
}

// UpdateSpriteStatus updates a sprite's status
func (db *DB) UpdateSpriteStatus(ctx context.Context, name string, status Status) error {
	result, err := db.ExecContext(ctx, `
		UPDATE sprites SET status = ?, updated_at = ? WHERE name = ?
	`, status, time.Now(), name)
	if err != nil {
		return fmt.Errorf("updating sprite status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("sprite '%s' not found", name)
	}

	return nil
}

// UpdateSpriteContainerIP updates a sprite's container IP
func (db *DB) UpdateSpriteContainerIP(ctx context.Context, name, ip string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE sprites SET container_ip = ?, updated_at = ? WHERE name = ?
	`, ip, time.Now(), name)
	return err
}

// UpdateSpriteTailscale updates a sprite's Tailscale info
func (db *DB) UpdateSpriteTailscale(ctx context.Context, name, tailscaleIP, funnelURL string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE sprites SET tailscale_ip = ?, funnel_url = ?, updated_at = ? WHERE name = ?
	`, tailscaleIP, funnelURL, time.Now(), name)
	return err
}

// DeleteSprite deletes a sprite by name
func (db *DB) DeleteSprite(ctx context.Context, name string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM sprites WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting sprite: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("sprite '%s' not found", name)
	}

	return nil
}

// scanSprite scans a single row into a Sprite
func scanSprite(row *sql.Row) (*Sprite, error) {
	var s Sprite
	var portsJSON string
	var hostPort sql.NullInt64
	var tailscaleIP, funnelURL, containerIP sql.NullString

	err := row.Scan(
		&s.ID, &s.Name, &s.Image, &s.Status, &s.VolumeDir,
		&portsJSON, &hostPort, &containerIP, &tailscaleIP, &funnelURL,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sprite not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning sprite: %w", err)
	}

	if err := json.Unmarshal([]byte(portsJSON), &s.Ports); err != nil {
		s.Ports = []string{}
	}

	s.HostPort = int(hostPort.Int64)
	s.ContainerIP = containerIP.String
	s.TailscaleIP = tailscaleIP.String
	s.FunnelURL = funnelURL.String

	return &s, nil
}

// scanSpriteRow scans a row from rows.Next() into a Sprite
func scanSpriteRow(rows *sql.Rows) (*Sprite, error) {
	var s Sprite
	var portsJSON string
	var hostPort sql.NullInt64
	var tailscaleIP, funnelURL, containerIP sql.NullString

	err := rows.Scan(
		&s.ID, &s.Name, &s.Image, &s.Status, &s.VolumeDir,
		&portsJSON, &hostPort, &containerIP, &tailscaleIP, &funnelURL,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning sprite row: %w", err)
	}

	if err := json.Unmarshal([]byte(portsJSON), &s.Ports); err != nil {
		s.Ports = []string{}
	}

	s.HostPort = int(hostPort.Int64)
	s.ContainerIP = containerIP.String
	s.TailscaleIP = tailscaleIP.String
	s.FunnelURL = funnelURL.String

	return &s, nil
}
