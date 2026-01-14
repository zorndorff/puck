package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

// Config holds all configuration for puck
type Config struct {
	DataDir       string `mapstructure:"data_dir"`
	PodmanSocket  string `mapstructure:"podman_socket"`
	DefaultImage  string `mapstructure:"default_image"`
	IdleTimeout   int    `mapstructure:"idle_timeout"` // minutes
	DaemonSocket  string `mapstructure:"daemon_socket"`
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		DataDir:      defaultDataDir(),
		PodmanSocket: defaultPodmanSocket(),
		DefaultImage: "fedora:latest",
		IdleTimeout:  15,
		DaemonSocket: defaultDaemonSocket(),
	}
}

// Load loads configuration from viper and applies defaults
func Load() (*Config, error) {
	cfg := Default()

	// Override with viper values if set
	if v := viper.GetString("data_dir"); v != "" {
		cfg.DataDir = v
	}
	if v := viper.GetString("podman_socket"); v != "" {
		cfg.PodmanSocket = v
	}
	if v := viper.GetString("default_image"); v != "" {
		cfg.DefaultImage = v
	}
	if v := viper.GetInt("idle_timeout"); v > 0 {
		cfg.IdleTimeout = v
	}
	if v := viper.GetString("daemon_socket"); v != "" {
		cfg.DaemonSocket = v
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	return cfg, nil
}

func defaultDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "puck")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "puck")
}

func defaultPodmanSocket() string {
	switch runtime.GOOS {
	case "linux":
		// Check rootless first
		uid := os.Getuid()
		if uid != 0 {
			sock := fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
			if _, err := os.Stat(sock); err == nil {
				return "unix://" + sock
			}
		}
		// Fall back to rootful
		return "unix:///run/podman/podman.sock"

	case "darwin", "windows":
		// Use Podman Machine
		home, _ := os.UserHomeDir()
		return fmt.Sprintf("unix://%s/.local/share/containers/podman/machine/podman.sock", home)

	default:
		return ""
	}
}

func defaultDaemonSocket() string {
	dataDir := defaultDataDir()
	return filepath.Join(dataDir, "puckd.sock")
}

// SpritesDir returns the directory for sprite data
func (c *Config) SpritesDir() string {
	return filepath.Join(c.DataDir, "sprites")
}

// SnapshotsDir returns the directory for snapshots
func (c *Config) SnapshotsDir() string {
	return filepath.Join(c.DataDir, "snapshots")
}

// DatabasePath returns the path to the SQLite database
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "puck.db")
}
