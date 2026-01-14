package puck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/sandwich-labs/puck/internal/config"
	"github.com/sandwich-labs/puck/internal/podman"
	"github.com/sandwich-labs/puck/internal/store"
)

// CreateOptions contains options for creating a new puck
type CreateOptions struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Ports []string `json:"ports,omitempty"`
}

// Manager handles puck lifecycle operations
type Manager struct {
	podman *podman.Client
	store  *store.DB
	cfg    *config.Config
}

// NewManager creates a new puck manager
func NewManager(cfg *config.Config, pc *podman.Client, db *store.DB) *Manager {
	return &Manager{
		podman: pc,
		store:  db,
		cfg:    cfg,
	}
}

// BaseHostPort is the starting port for auto-assigned puck ports
const BaseHostPort = 9000

// Create creates a new puck
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*store.Puck, error) {
	// Use default image if not specified
	if opts.Image == "" {
		opts.Image = m.cfg.DefaultImage
	}

	// Find next available host port
	hostPort, err := m.findAvailablePort(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding available port: %w", err)
	}

	// Create puck record
	now := time.Now()
	p := &store.Puck{
		ID:        uuid.New().String(),
		Name:      opts.Name,
		Image:     opts.Image,
		Status:    store.StatusCreating,
		CreatedAt: now,
		UpdatedAt: now,
		VolumeDir: filepath.Join(m.cfg.PucksDir(), opts.Name),
		Ports:     opts.Ports,
		HostPort:  hostPort,
	}

	// Create volume directories
	volumeDirs := []string{"home", "etc", "var"}
	for _, dir := range volumeDirs {
		path := filepath.Join(p.VolumeDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("creating volume directory %s: %w", dir, err)
		}
	}

	// Create container with port mapping for HTTP routing
	volumes := map[string]string{
		filepath.Join(p.VolumeDir, "home"): "/home",
		filepath.Join(p.VolumeDir, "etc"):  "/etc/puck",
		filepath.Join(p.VolumeDir, "var"):  "/var/puck",
	}

	// Add the auto-assigned port mapping (host:container)
	portMappings := append(opts.Ports, fmt.Sprintf("%d:80", hostPort))

	containerID, err := m.podman.CreateContainer(ctx, podman.CreateContainerOptions{
		Name:    opts.Name,
		Image:   opts.Image,
		Volumes: volumes,
		Ports:   portMappings,
		Systemd: true,
		Labels: map[string]string{
			"puck.id": p.ID,
		},
	})
	if err != nil {
		// Clean up volume dir on failure
		os.RemoveAll(p.VolumeDir)
		return nil, fmt.Errorf("creating container: %w", err)
	}

	p.ID = containerID

	// Start the container
	if err := m.podman.StartContainer(ctx, containerID); err != nil {
		m.podman.RemoveContainer(ctx, containerID, true)
		os.RemoveAll(p.VolumeDir)
		return nil, fmt.Errorf("starting container: %w", err)
	}

	// Get container IP
	ip, err := m.podman.GetContainerIP(ctx, containerID)
	if err == nil {
		p.ContainerIP = ip
	}

	p.Status = store.StatusRunning

	// Save to database
	if err := m.store.CreatePuck(ctx, p); err != nil {
		m.podman.RemoveContainer(ctx, containerID, true)
		os.RemoveAll(p.VolumeDir)
		return nil, fmt.Errorf("saving puck: %w", err)
	}

	return p, nil
}

// Get retrieves a puck by name
func (m *Manager) Get(ctx context.Context, name string) (*store.Puck, error) {
	return m.store.GetPuck(ctx, name)
}

// List returns all pucks
func (m *Manager) List(ctx context.Context) ([]*store.Puck, error) {
	pucks, err := m.store.ListPucks(ctx)
	if err != nil {
		return nil, err
	}

	// Update status from Podman for each puck
	for _, p := range pucks {
		running, err := m.podman.IsRunning(ctx, p.ID)
		if err != nil {
			continue // Container might not exist
		}
		if running {
			p.Status = store.StatusRunning
		} else {
			p.Status = store.StatusStopped
		}
	}

	return pucks, nil
}

// Start starts a stopped puck
func (m *Manager) Start(ctx context.Context, name string) error {
	p, err := m.store.GetPuck(ctx, name)
	if err != nil {
		return err
	}

	if err := m.podman.StartContainer(ctx, p.ID); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	// Update IP
	ip, err := m.podman.GetContainerIP(ctx, p.ID)
	if err == nil {
		m.store.UpdatePuckContainerIP(ctx, name, ip)
	}

	return m.store.UpdatePuckStatus(ctx, name, store.StatusRunning)
}

// Stop stops a running puck
func (m *Manager) Stop(ctx context.Context, name string) error {
	p, err := m.store.GetPuck(ctx, name)
	if err != nil {
		return err
	}

	if err := m.podman.StopContainer(ctx, p.ID); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}

	return m.store.UpdatePuckStatus(ctx, name, store.StatusStopped)
}

// Destroy removes a puck and its data
func (m *Manager) Destroy(ctx context.Context, name string, force bool) error {
	p, err := m.store.GetPuck(ctx, name)
	if err != nil {
		return err
	}

	// Stop container first if running and not forcing
	if !force {
		running, _ := m.podman.IsRunning(ctx, p.ID)
		if running {
			if err := m.podman.StopContainer(ctx, p.ID); err != nil {
				return fmt.Errorf("stopping container: %w (use --force to override)", err)
			}
		}
	}

	// Remove container (force if requested)
	if err := m.podman.RemoveContainer(ctx, p.ID, force); err != nil {
		// Try to remove even if container doesn't exist
		if !force {
			return fmt.Errorf("removing container: %w", err)
		}
		// Continue cleanup even if container removal fails with force
	}

	// Remove volume directory
	if p.VolumeDir != "" {
		os.RemoveAll(p.VolumeDir) // Ignore errors - may not exist
	}

	// Remove from database
	if err := m.store.DeletePuck(ctx, name); err != nil {
		return fmt.Errorf("removing from database: %w", err)
	}

	return nil
}

// DestroyAll removes all pucks
func (m *Manager) DestroyAll(ctx context.Context, force bool) ([]string, error) {
	pucks, err := m.store.ListPucks(ctx)
	if err != nil {
		return nil, err
	}

	var destroyed []string
	var errors []string

	for _, p := range pucks {
		if err := m.Destroy(ctx, p.Name, force); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", p.Name, err))
		} else {
			destroyed = append(destroyed, p.Name)
		}
	}

	if len(errors) > 0 {
		return destroyed, fmt.Errorf("failed to destroy some pucks: %v", errors)
	}

	return destroyed, nil
}

// Console opens a shell in a puck
func (m *Manager) Console(ctx context.Context, name string, shell string) error {
	p, err := m.store.GetPuck(ctx, name)
	if err != nil {
		return err
	}

	// Start if not running
	running, err := m.podman.IsRunning(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("checking container status: %w", err)
	}

	if !running {
		if err := m.Start(ctx, name); err != nil {
			return fmt.Errorf("starting puck: %w", err)
		}
	}

	return m.podman.Console(ctx, p.ID, shell)
}

// Exists checks if a puck exists
func (m *Manager) Exists(ctx context.Context, name string) bool {
	_, err := m.store.GetPuck(ctx, name)
	return err == nil
}

// SnapshotCreateOptions contains options for creating a snapshot
type SnapshotCreateOptions struct {
	PuckName     string `json:"puck_name"`
	SnapshotName string `json:"snapshot_name"`
	LeaveRunning bool   `json:"leave_running"`
}

// SnapshotRestoreOptions contains options for restoring a snapshot
type SnapshotRestoreOptions struct {
	PuckName     string `json:"puck_name"`
	SnapshotName string `json:"snapshot_name"`
}

// CreateSnapshot creates a checkpoint snapshot of a puck
func (m *Manager) CreateSnapshot(ctx context.Context, opts SnapshotCreateOptions) (*store.Snapshot, error) {
	p, err := m.store.GetPuck(ctx, opts.PuckName)
	if err != nil {
		return nil, err
	}

	// Container must be running to checkpoint
	running, err := m.podman.IsRunning(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("checking container status: %w", err)
	}
	if !running {
		return nil, fmt.Errorf("puck must be running to create snapshot")
	}

	// Create snapshots directory
	snapshotDir := filepath.Join(m.cfg.SnapshotsDir(), opts.PuckName)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	// Create checkpoint archive
	exportPath := filepath.Join(snapshotDir, opts.SnapshotName+".tar.gz")
	if err := m.podman.Checkpoint(ctx, p.ID, podman.CheckpointOptions{
		ExportPath:   exportPath,
		LeaveRunning: opts.LeaveRunning,
	}); err != nil {
		return nil, fmt.Errorf("checkpointing container: %w", err)
	}

	// Get file size
	info, err := os.Stat(exportPath)
	if err != nil {
		return nil, fmt.Errorf("getting snapshot size: %w", err)
	}

	// Update puck status if not leaving running
	if !opts.LeaveRunning {
		m.store.UpdatePuckStatus(ctx, opts.PuckName, store.StatusCheckpointed)
	}

	// Create snapshot record
	now := time.Now()
	snapshot := &store.Snapshot{
		ID:        uuid.New().String(),
		PuckID:    p.ID,
		PuckName:  p.Name,
		Name:      opts.SnapshotName,
		Path:      exportPath,
		SizeBytes: info.Size(),
		CreatedAt: now,
	}

	if err := m.store.CreateSnapshot(ctx, snapshot); err != nil {
		// Clean up the checkpoint file on failure
		os.Remove(exportPath)
		return nil, fmt.Errorf("saving snapshot: %w", err)
	}

	return snapshot, nil
}

// RestoreSnapshot restores a puck from a checkpoint snapshot
func (m *Manager) RestoreSnapshot(ctx context.Context, opts SnapshotRestoreOptions) error {
	p, err := m.store.GetPuck(ctx, opts.PuckName)
	if err != nil {
		return err
	}

	snapshot, err := m.store.GetSnapshot(ctx, p.ID, opts.SnapshotName)
	if err != nil {
		return err
	}

	// Check if snapshot file exists
	if _, err := os.Stat(snapshot.Path); os.IsNotExist(err) {
		return fmt.Errorf("snapshot file not found: %s", snapshot.Path)
	}

	// Stop existing container if running
	running, _ := m.podman.IsRunning(ctx, p.ID)
	if running {
		if err := m.podman.StopContainer(ctx, p.ID); err != nil {
			return fmt.Errorf("stopping container: %w", err)
		}
	}

	// Remove existing container
	if err := m.podman.RemoveContainer(ctx, p.ID, true); err != nil {
		// Container might not exist, continue anyway
	}

	// Restore from checkpoint
	newContainerID, err := m.podman.Restore(ctx, podman.RestoreOptions{
		ImportPath: snapshot.Path,
		Name:       opts.PuckName,
	})
	if err != nil {
		return fmt.Errorf("restoring checkpoint: %w", err)
	}

	// Update puck with new container ID and status
	if _, err := m.store.ExecContext(ctx, `
		UPDATE pucks SET id = ?, status = ?, updated_at = ? WHERE name = ?
	`, newContainerID, store.StatusRunning, time.Now(), opts.PuckName); err != nil {
		return fmt.Errorf("updating puck: %w", err)
	}

	// Update container IP
	ip, err := m.podman.GetContainerIP(ctx, newContainerID)
	if err == nil {
		m.store.UpdatePuckContainerIP(ctx, opts.PuckName, ip)
	}

	return nil
}

// ListSnapshots returns all snapshots for a puck
func (m *Manager) ListSnapshots(ctx context.Context, puckName string) ([]*store.Snapshot, error) {
	p, err := m.store.GetPuck(ctx, puckName)
	if err != nil {
		return nil, err
	}

	return m.store.ListSnapshots(ctx, p.ID)
}

// DeleteSnapshot deletes a snapshot
func (m *Manager) DeleteSnapshot(ctx context.Context, puckName, snapshotName string) error {
	p, err := m.store.GetPuck(ctx, puckName)
	if err != nil {
		return err
	}

	snapshot, err := m.store.GetSnapshot(ctx, p.ID, snapshotName)
	if err != nil {
		return err
	}

	// Remove snapshot file
	if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing snapshot file: %w", err)
	}

	// Remove from database
	return m.store.DeleteSnapshot(ctx, snapshot.ID)
}

// findAvailablePort finds the next available host port for puck routing
func (m *Manager) findAvailablePort(ctx context.Context) (int, error) {
	pucks, err := m.store.ListPucks(ctx)
	if err != nil {
		return 0, err
	}

	// Find the highest used port
	usedPorts := make(map[int]bool)
	for _, p := range pucks {
		if p.HostPort > 0 {
			usedPorts[p.HostPort] = true
		}
	}

	// Find first available port starting from BaseHostPort
	for port := BaseHostPort; port < BaseHostPort+1000; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", BaseHostPort, BaseHostPort+1000)
}
