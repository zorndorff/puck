package sprite

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

// CreateOptions contains options for creating a new sprite
type CreateOptions struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Ports []string `json:"ports,omitempty"`
}

// Manager handles sprite lifecycle operations
type Manager struct {
	podman *podman.Client
	store  *store.DB
	cfg    *config.Config
}

// NewManager creates a new sprite manager
func NewManager(cfg *config.Config, pc *podman.Client, db *store.DB) *Manager {
	return &Manager{
		podman: pc,
		store:  db,
		cfg:    cfg,
	}
}

// BaseHostPort is the starting port for auto-assigned sprite ports
const BaseHostPort = 9000

// Create creates a new sprite
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*store.Sprite, error) {
	// Use default image if not specified
	if opts.Image == "" {
		opts.Image = m.cfg.DefaultImage
	}

	// Find next available host port
	hostPort, err := m.findAvailablePort(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding available port: %w", err)
	}

	// Create sprite record
	now := time.Now()
	s := &store.Sprite{
		ID:        uuid.New().String(),
		Name:      opts.Name,
		Image:     opts.Image,
		Status:    store.StatusCreating,
		CreatedAt: now,
		UpdatedAt: now,
		VolumeDir: filepath.Join(m.cfg.SpritesDir(), opts.Name),
		Ports:     opts.Ports,
		HostPort:  hostPort,
	}

	// Create volume directories
	volumeDirs := []string{"home", "etc", "var"}
	for _, dir := range volumeDirs {
		path := filepath.Join(s.VolumeDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("creating volume directory %s: %w", dir, err)
		}
	}

	// Create container with port mapping for HTTP routing
	volumes := map[string]string{
		filepath.Join(s.VolumeDir, "home"): "/home",
		filepath.Join(s.VolumeDir, "etc"):  "/etc/puck",
		filepath.Join(s.VolumeDir, "var"):  "/var/puck",
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
			"puck.sprite.id": s.ID,
		},
	})
	if err != nil {
		// Clean up volume dir on failure
		os.RemoveAll(s.VolumeDir)
		return nil, fmt.Errorf("creating container: %w", err)
	}

	s.ID = containerID

	// Start the container
	if err := m.podman.StartContainer(ctx, containerID); err != nil {
		m.podman.RemoveContainer(ctx, containerID, true)
		os.RemoveAll(s.VolumeDir)
		return nil, fmt.Errorf("starting container: %w", err)
	}

	// Get container IP
	ip, err := m.podman.GetContainerIP(ctx, containerID)
	if err == nil {
		s.ContainerIP = ip
	}

	s.Status = store.StatusRunning

	// Save to database
	if err := m.store.CreateSprite(ctx, s); err != nil {
		m.podman.RemoveContainer(ctx, containerID, true)
		os.RemoveAll(s.VolumeDir)
		return nil, fmt.Errorf("saving sprite: %w", err)
	}

	return s, nil
}

// Get retrieves a sprite by name
func (m *Manager) Get(ctx context.Context, name string) (*store.Sprite, error) {
	return m.store.GetSprite(ctx, name)
}

// List returns all sprites
func (m *Manager) List(ctx context.Context) ([]*store.Sprite, error) {
	sprites, err := m.store.ListSprites(ctx)
	if err != nil {
		return nil, err
	}

	// Update status from Podman for each sprite
	for _, s := range sprites {
		running, err := m.podman.IsRunning(ctx, s.ID)
		if err != nil {
			continue // Container might not exist
		}
		if running {
			s.Status = store.StatusRunning
		} else {
			s.Status = store.StatusStopped
		}
	}

	return sprites, nil
}

// Start starts a stopped sprite
func (m *Manager) Start(ctx context.Context, name string) error {
	s, err := m.store.GetSprite(ctx, name)
	if err != nil {
		return err
	}

	if err := m.podman.StartContainer(ctx, s.ID); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	// Update IP
	ip, err := m.podman.GetContainerIP(ctx, s.ID)
	if err == nil {
		m.store.UpdateSpriteContainerIP(ctx, name, ip)
	}

	return m.store.UpdateSpriteStatus(ctx, name, store.StatusRunning)
}

// Stop stops a running sprite
func (m *Manager) Stop(ctx context.Context, name string) error {
	s, err := m.store.GetSprite(ctx, name)
	if err != nil {
		return err
	}

	if err := m.podman.StopContainer(ctx, s.ID); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}

	return m.store.UpdateSpriteStatus(ctx, name, store.StatusStopped)
}

// Destroy removes a sprite and its data
func (m *Manager) Destroy(ctx context.Context, name string, force bool) error {
	s, err := m.store.GetSprite(ctx, name)
	if err != nil {
		return err
	}

	// Stop container first if running and not forcing
	if !force {
		running, _ := m.podman.IsRunning(ctx, s.ID)
		if running {
			if err := m.podman.StopContainer(ctx, s.ID); err != nil {
				return fmt.Errorf("stopping container: %w (use --force to override)", err)
			}
		}
	}

	// Remove container (force if requested)
	if err := m.podman.RemoveContainer(ctx, s.ID, force); err != nil {
		// Try to remove even if container doesn't exist
		if !force {
			return fmt.Errorf("removing container: %w", err)
		}
		// Continue cleanup even if container removal fails with force
	}

	// Remove volume directory
	if s.VolumeDir != "" {
		os.RemoveAll(s.VolumeDir) // Ignore errors - may not exist
	}

	// Remove from database
	if err := m.store.DeleteSprite(ctx, name); err != nil {
		return fmt.Errorf("removing from database: %w", err)
	}

	return nil
}

// DestroyAll removes all sprites
func (m *Manager) DestroyAll(ctx context.Context, force bool) ([]string, error) {
	sprites, err := m.store.ListSprites(ctx)
	if err != nil {
		return nil, err
	}

	var destroyed []string
	var errors []string

	for _, s := range sprites {
		if err := m.Destroy(ctx, s.Name, force); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", s.Name, err))
		} else {
			destroyed = append(destroyed, s.Name)
		}
	}

	if len(errors) > 0 {
		return destroyed, fmt.Errorf("failed to destroy some sprites: %v", errors)
	}

	return destroyed, nil
}

// Console opens a shell in a sprite
func (m *Manager) Console(ctx context.Context, name string, shell string) error {
	s, err := m.store.GetSprite(ctx, name)
	if err != nil {
		return err
	}

	// Start if not running
	running, err := m.podman.IsRunning(ctx, s.ID)
	if err != nil {
		return fmt.Errorf("checking container status: %w", err)
	}

	if !running {
		if err := m.Start(ctx, name); err != nil {
			return fmt.Errorf("starting sprite: %w", err)
		}
	}

	return m.podman.Console(ctx, s.ID, shell)
}

// Exists checks if a sprite exists
func (m *Manager) Exists(ctx context.Context, name string) bool {
	_, err := m.store.GetSprite(ctx, name)
	return err == nil
}

// SnapshotCreateOptions contains options for creating a snapshot
type SnapshotCreateOptions struct {
	SpriteName   string `json:"sprite_name"`
	SnapshotName string `json:"snapshot_name"`
	LeaveRunning bool   `json:"leave_running"`
}

// SnapshotRestoreOptions contains options for restoring a snapshot
type SnapshotRestoreOptions struct {
	SpriteName   string `json:"sprite_name"`
	SnapshotName string `json:"snapshot_name"`
}

// CreateSnapshot creates a checkpoint snapshot of a sprite
func (m *Manager) CreateSnapshot(ctx context.Context, opts SnapshotCreateOptions) (*store.Snapshot, error) {
	s, err := m.store.GetSprite(ctx, opts.SpriteName)
	if err != nil {
		return nil, err
	}

	// Container must be running to checkpoint
	running, err := m.podman.IsRunning(ctx, s.ID)
	if err != nil {
		return nil, fmt.Errorf("checking container status: %w", err)
	}
	if !running {
		return nil, fmt.Errorf("sprite must be running to create snapshot")
	}

	// Create snapshots directory
	snapshotDir := filepath.Join(m.cfg.SnapshotsDir(), opts.SpriteName)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	// Create checkpoint archive
	exportPath := filepath.Join(snapshotDir, opts.SnapshotName+".tar.gz")
	if err := m.podman.Checkpoint(ctx, s.ID, podman.CheckpointOptions{
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

	// Update sprite status if not leaving running
	if !opts.LeaveRunning {
		m.store.UpdateSpriteStatus(ctx, opts.SpriteName, store.StatusCheckpointed)
	}

	// Create snapshot record
	now := time.Now()
	snapshot := &store.Snapshot{
		ID:         uuid.New().String(),
		SpriteID:   s.ID,
		SpriteName: s.Name,
		Name:       opts.SnapshotName,
		Path:       exportPath,
		SizeBytes:  info.Size(),
		CreatedAt:  now,
	}

	if err := m.store.CreateSnapshot(ctx, snapshot); err != nil {
		// Clean up the checkpoint file on failure
		os.Remove(exportPath)
		return nil, fmt.Errorf("saving snapshot: %w", err)
	}

	return snapshot, nil
}

// RestoreSnapshot restores a sprite from a checkpoint snapshot
func (m *Manager) RestoreSnapshot(ctx context.Context, opts SnapshotRestoreOptions) error {
	s, err := m.store.GetSprite(ctx, opts.SpriteName)
	if err != nil {
		return err
	}

	snapshot, err := m.store.GetSnapshot(ctx, s.ID, opts.SnapshotName)
	if err != nil {
		return err
	}

	// Check if snapshot file exists
	if _, err := os.Stat(snapshot.Path); os.IsNotExist(err) {
		return fmt.Errorf("snapshot file not found: %s", snapshot.Path)
	}

	// Stop existing container if running
	running, _ := m.podman.IsRunning(ctx, s.ID)
	if running {
		if err := m.podman.StopContainer(ctx, s.ID); err != nil {
			return fmt.Errorf("stopping container: %w", err)
		}
	}

	// Remove existing container
	if err := m.podman.RemoveContainer(ctx, s.ID, true); err != nil {
		// Container might not exist, continue anyway
	}

	// Restore from checkpoint
	newContainerID, err := m.podman.Restore(ctx, podman.RestoreOptions{
		ImportPath: snapshot.Path,
		Name:       opts.SpriteName,
	})
	if err != nil {
		return fmt.Errorf("restoring checkpoint: %w", err)
	}

	// Update sprite with new container ID and status
	if _, err := m.store.ExecContext(ctx, `
		UPDATE sprites SET id = ?, status = ?, updated_at = ? WHERE name = ?
	`, newContainerID, store.StatusRunning, time.Now(), opts.SpriteName); err != nil {
		return fmt.Errorf("updating sprite: %w", err)
	}

	// Update container IP
	ip, err := m.podman.GetContainerIP(ctx, newContainerID)
	if err == nil {
		m.store.UpdateSpriteContainerIP(ctx, opts.SpriteName, ip)
	}

	return nil
}

// ListSnapshots returns all snapshots for a sprite
func (m *Manager) ListSnapshots(ctx context.Context, spriteName string) ([]*store.Snapshot, error) {
	s, err := m.store.GetSprite(ctx, spriteName)
	if err != nil {
		return nil, err
	}

	return m.store.ListSnapshots(ctx, s.ID)
}

// DeleteSnapshot deletes a snapshot
func (m *Manager) DeleteSnapshot(ctx context.Context, spriteName, snapshotName string) error {
	s, err := m.store.GetSprite(ctx, spriteName)
	if err != nil {
		return err
	}

	snapshot, err := m.store.GetSnapshot(ctx, s.ID, snapshotName)
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

// findAvailablePort finds the next available host port for sprite routing
func (m *Manager) findAvailablePort(ctx context.Context) (int, error) {
	sprites, err := m.store.ListSprites(ctx)
	if err != nil {
		return 0, err
	}

	// Find the highest used port
	usedPorts := make(map[int]bool)
	for _, s := range sprites {
		if s.HostPort > 0 {
			usedPorts[s.HostPort] = true
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
