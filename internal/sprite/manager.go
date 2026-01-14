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

// Create creates a new sprite
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*store.Sprite, error) {
	// Use default image if not specified
	if opts.Image == "" {
		opts.Image = m.cfg.DefaultImage
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
	}

	// Create volume directories
	volumeDirs := []string{"home", "etc", "var"}
	for _, dir := range volumeDirs {
		path := filepath.Join(s.VolumeDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("creating volume directory %s: %w", dir, err)
		}
	}

	// Create container
	volumes := map[string]string{
		filepath.Join(s.VolumeDir, "home"): "/home",
		filepath.Join(s.VolumeDir, "etc"):  "/etc/puck",
		filepath.Join(s.VolumeDir, "var"):  "/var/puck",
	}

	containerID, err := m.podman.CreateContainer(ctx, podman.CreateContainerOptions{
		Name:    opts.Name,
		Image:   opts.Image,
		Volumes: volumes,
		Ports:   opts.Ports,
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

	// Remove container
	if err := m.podman.RemoveContainer(ctx, s.ID, force); err != nil {
		if !force {
			return fmt.Errorf("removing container: %w", err)
		}
		// Continue cleanup even if container removal fails with force
	}

	// Remove volume directory
	if err := os.RemoveAll(s.VolumeDir); err != nil {
		return fmt.Errorf("removing volume directory: %w", err)
	}

	// Remove from database
	if err := m.store.DeleteSprite(ctx, name); err != nil {
		return fmt.Errorf("removing from database: %w", err)
	}

	return nil
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
