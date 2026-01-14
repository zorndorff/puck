package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Status represents the current state of a puck
type Status string

const (
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusCheckpointed Status = "checkpointed"
	StatusCreating     Status = "creating"
	StatusError        Status = "error"
)

// Puck represents a persistent container managed by puck
type Puck struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	VolumeDir   string    `json:"volume_dir"`
	Ports       []string  `json:"ports,omitempty"`
	HostPort    int       `json:"host_port,omitempty"` // Auto-assigned port for HTTP routing
	TailscaleIP string    `json:"tailscale_ip,omitempty"`
	FunnelURL   string    `json:"funnel_url,omitempty"`
	ContainerIP string    `json:"container_ip,omitempty"`
}

// Snapshot represents a checkpoint of a puck's state
type Snapshot struct {
	ID        string    `json:"id"`
	PuckID    string    `json:"puck_id"`
	PuckName  string    `json:"puck_name"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// CreatePuck creates a new puck in the database
func (db *DB) CreatePuck(ctx context.Context, p *Puck) error {
	portsJSON, err := json.Marshal(p.Ports)
	if err != nil {
		return fmt.Errorf("marshaling ports: %w", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO pucks (id, name, image, status, volume_dir, ports, host_port, container_ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.Image, p.Status, p.VolumeDir, string(portsJSON), p.HostPort, p.ContainerIP, p.CreatedAt, p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting puck: %w", err)
	}

	return nil
}

// GetPuck retrieves a puck by name
func (db *DB) GetPuck(ctx context.Context, name string) (*Puck, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, image, status, volume_dir, ports, host_port, container_ip, tailscale_ip, funnel_url, created_at, updated_at
		FROM pucks WHERE name = ?
	`, name)

	return scanPuck(row)
}

// GetPuckByID retrieves a puck by ID
func (db *DB) GetPuckByID(ctx context.Context, id string) (*Puck, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, image, status, volume_dir, ports, host_port, container_ip, tailscale_ip, funnel_url, created_at, updated_at
		FROM pucks WHERE id = ?
	`, id)

	return scanPuck(row)
}

// ListPucks returns all pucks
func (db *DB) ListPucks(ctx context.Context) ([]*Puck, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, image, status, volume_dir, ports, host_port, container_ip, tailscale_ip, funnel_url, created_at, updated_at
		FROM pucks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying pucks: %w", err)
	}
	defer rows.Close()

	var pucks []*Puck
	for rows.Next() {
		p, err := scanPuckRow(rows)
		if err != nil {
			return nil, err
		}
		pucks = append(pucks, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pucks: %w", err)
	}

	return pucks, nil
}

// UpdatePuckStatus updates a puck's status
func (db *DB) UpdatePuckStatus(ctx context.Context, name string, status Status) error {
	result, err := db.ExecContext(ctx, `
		UPDATE pucks SET status = ?, updated_at = ? WHERE name = ?
	`, status, time.Now(), name)
	if err != nil {
		return fmt.Errorf("updating puck status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("puck '%s' not found", name)
	}

	return nil
}

// UpdatePuckContainerIP updates a puck's container IP
func (db *DB) UpdatePuckContainerIP(ctx context.Context, name, ip string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE pucks SET container_ip = ?, updated_at = ? WHERE name = ?
	`, ip, time.Now(), name)
	return err
}

// UpdatePuckTailscale updates a puck's Tailscale info
func (db *DB) UpdatePuckTailscale(ctx context.Context, name, tailscaleIP, funnelURL string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE pucks SET tailscale_ip = ?, funnel_url = ?, updated_at = ? WHERE name = ?
	`, tailscaleIP, funnelURL, time.Now(), name)
	return err
}

// DeletePuck deletes a puck by name
func (db *DB) DeletePuck(ctx context.Context, name string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM pucks WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting puck: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("puck '%s' not found", name)
	}

	return nil
}

// scanPuck scans a single row into a Puck
func scanPuck(row *sql.Row) (*Puck, error) {
	var p Puck
	var portsJSON string
	var hostPort sql.NullInt64
	var tailscaleIP, funnelURL, containerIP sql.NullString

	err := row.Scan(
		&p.ID, &p.Name, &p.Image, &p.Status, &p.VolumeDir,
		&portsJSON, &hostPort, &containerIP, &tailscaleIP, &funnelURL,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("puck not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning puck: %w", err)
	}

	if err := json.Unmarshal([]byte(portsJSON), &p.Ports); err != nil {
		p.Ports = []string{}
	}

	p.HostPort = int(hostPort.Int64)
	p.ContainerIP = containerIP.String
	p.TailscaleIP = tailscaleIP.String
	p.FunnelURL = funnelURL.String

	return &p, nil
}

// scanPuckRow scans a row from rows.Next() into a Puck
func scanPuckRow(rows *sql.Rows) (*Puck, error) {
	var p Puck
	var portsJSON string
	var hostPort sql.NullInt64
	var tailscaleIP, funnelURL, containerIP sql.NullString

	err := rows.Scan(
		&p.ID, &p.Name, &p.Image, &p.Status, &p.VolumeDir,
		&portsJSON, &hostPort, &containerIP, &tailscaleIP, &funnelURL,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning puck row: %w", err)
	}

	if err := json.Unmarshal([]byte(portsJSON), &p.Ports); err != nil {
		p.Ports = []string{}
	}

	p.HostPort = int(hostPort.Int64)
	p.ContainerIP = containerIP.String
	p.TailscaleIP = tailscaleIP.String
	p.FunnelURL = funnelURL.String

	return &p, nil
}
