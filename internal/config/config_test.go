package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	t.Run("has default image", func(t *testing.T) {
		assert.Equal(t, "fedora:latest", cfg.DefaultImage)
	})

	t.Run("has default idle timeout", func(t *testing.T) {
		assert.Equal(t, 15, cfg.IdleTimeout)
	})

	t.Run("has default router port", func(t *testing.T) {
		assert.Equal(t, 8080, cfg.RouterPort)
	})

	t.Run("has default router domain", func(t *testing.T) {
		assert.Equal(t, "localhost", cfg.RouterDomain)
	})

	t.Run("tailnet is disabled by default", func(t *testing.T) {
		assert.Empty(t, cfg.Tailnet)
	})

	t.Run("data dir is not empty", func(t *testing.T) {
		assert.NotEmpty(t, cfg.DataDir)
	})

	t.Run("daemon socket is not empty", func(t *testing.T) {
		assert.NotEmpty(t, cfg.DaemonSocket)
	})
}

func TestLoad(t *testing.T) {
	// Reset viper between tests
	cleanup := func() {
		viper.Reset()
		os.Unsetenv("PUCK_DATA_DIR")
		os.Unsetenv("PUCK_DEFAULT_IMAGE")
		os.Unsetenv("PUCK_ROUTER_PORT")
	}

	t.Run("returns default config without overrides", func(t *testing.T) {
		cleanup()
		defer cleanup()

		// Use a temp directory for data_dir
		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		viper.Set("data_dir", dir)

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "fedora:latest", cfg.DefaultImage)
		assert.Equal(t, 8080, cfg.RouterPort)
	})

	t.Run("applies environment overrides", func(t *testing.T) {
		cleanup()
		defer cleanup()

		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		os.Setenv("PUCK_DATA_DIR", dir)
		os.Setenv("PUCK_DEFAULT_IMAGE", "ubuntu:22.04")
		os.Setenv("PUCK_ROUTER_PORT", "9090")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, dir, cfg.DataDir)
		assert.Equal(t, "ubuntu:22.04", cfg.DefaultImage)
		assert.Equal(t, 9090, cfg.RouterPort)
	})

	t.Run("creates data directory if not exists", func(t *testing.T) {
		cleanup()
		defer cleanup()

		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Create a path that doesn't exist yet
		newDir := filepath.Join(dir, "new", "nested", "path")
		viper.Set("data_dir", newDir)

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, newDir, cfg.DataDir)

		// Verify directory was created
		info, err := os.Stat(newDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("applies viper overrides", func(t *testing.T) {
		cleanup()
		defer cleanup()

		dir, err := os.MkdirTemp("", "puck-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		viper.Set("data_dir", dir)
		viper.Set("idle_timeout", 30)
		viper.Set("router_domain", "myapp.local")
		viper.Set("tailnet", "my-tailnet")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, 30, cfg.IdleTimeout)
		assert.Equal(t, "myapp.local", cfg.RouterDomain)
		assert.Equal(t, "my-tailnet", cfg.Tailnet)
	})
}

func TestDefaultDataDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		oldValue := os.Getenv("XDG_DATA_HOME")
		defer os.Setenv("XDG_DATA_HOME", oldValue)

		os.Setenv("XDG_DATA_HOME", "/custom/data")
		dir := defaultDataDir()
		assert.Equal(t, "/custom/data/puck", dir)
	})

	t.Run("falls back to ~/.local/share when XDG_DATA_HOME not set", func(t *testing.T) {
		oldValue := os.Getenv("XDG_DATA_HOME")
		defer os.Setenv("XDG_DATA_HOME", oldValue)

		os.Unsetenv("XDG_DATA_HOME")
		dir := defaultDataDir()
		home, _ := os.UserHomeDir()
		assert.Equal(t, filepath.Join(home, ".local", "share", "puck"), dir)
	})
}

func TestPucksDir(t *testing.T) {
	cfg := &Config{DataDir: "/test/data"}
	assert.Equal(t, "/test/data/pucks", cfg.PucksDir())
}

func TestSnapshotsDir(t *testing.T) {
	cfg := &Config{DataDir: "/test/data"}
	assert.Equal(t, "/test/data/snapshots", cfg.SnapshotsDir())
}

func TestDatabasePath(t *testing.T) {
	cfg := &Config{DataDir: "/test/data"}
	assert.Equal(t, "/test/data/puck.db", cfg.DatabasePath())
}

func TestDefaultDaemonSocket(t *testing.T) {
	socket := defaultDaemonSocket()
	assert.Contains(t, socket, "puckd.sock")
}
