package puck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sandwich-labs/puck/internal/config"
	"github.com/sandwich-labs/puck/internal/podman"
	"github.com/sandwich-labs/puck/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestManager creates a test manager with mock podman and temp database
func setupTestManager(t *testing.T) (*Manager, *podman.MockClient, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "puck-manager-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(dir, "test.db")
	db, err := store.Open(dbPath)
	require.NoError(t, err)

	cfg := &config.Config{
		DefaultImage: "fedora:latest",
		DataDir:      dir,
	}

	mock := podman.NewMockClient()

	// Make CreateContainer return unique IDs
	containerCounter := 0
	mock.CreateContainerFunc = func(ctx context.Context, opts podman.CreateContainerOptions) (string, error) {
		containerCounter++
		return fmt.Sprintf("mock-container-%d", containerCounter), nil
	}

	mgr := NewManager(cfg, mock, db)

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return mgr, mock, cleanup
}

func TestNewManager(t *testing.T) {
	t.Run("creates manager with dependencies", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()

		assert.NotNil(t, mgr)
		assert.NotNil(t, mgr.podman)
		assert.NotNil(t, mgr.store)
		assert.NotNil(t, mgr.cfg)
	})
}

func TestCreate(t *testing.T) {
	t.Run("creates puck successfully", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		p, err := mgr.Create(ctx, CreateOptions{
			Name:  "test-puck",
			Image: "fedora:latest",
		})
		require.NoError(t, err)
		assert.Equal(t, "test-puck", p.Name)
		assert.Equal(t, "fedora:latest", p.Image)
		assert.Equal(t, store.StatusRunning, p.Status)
		assert.Equal(t, BaseHostPort, p.HostPort)

		// Verify container was created and started
		assert.True(t, mock.WasCalled("CreateContainer"))
		assert.True(t, mock.WasCalled("StartContainer"))
	})

	t.Run("uses default image when not specified", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		p, err := mgr.Create(ctx, CreateOptions{
			Name: "default-image-puck",
		})
		require.NoError(t, err)
		assert.Equal(t, "fedora:latest", p.Image)
	})

	t.Run("creates volume directories", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		p, err := mgr.Create(ctx, CreateOptions{
			Name: "volume-puck",
		})
		require.NoError(t, err)

		// Check volume directories were created
		for _, subdir := range []string{"home", "etc", "var"} {
			path := filepath.Join(p.VolumeDir, subdir)
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.True(t, info.IsDir())
		}
	})

	t.Run("cleans up on container creation failure", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		mock.CreateContainerFunc = func(ctx context.Context, opts podman.CreateContainerOptions) (string, error) {
			return "", assert.AnError
		}

		p, err := mgr.Create(ctx, CreateOptions{
			Name: "fail-create-puck",
		})
		assert.Error(t, err)
		assert.Nil(t, p)

		// Volume directory should be cleaned up
		expectedDir := filepath.Join(mgr.cfg.DataDir, "pucks", "fail-create-puck")
		_, statErr := os.Stat(expectedDir)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("cleans up on start failure", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		mock.StartContainerFunc = func(ctx context.Context, nameOrID string) error {
			return assert.AnError
		}

		p, err := mgr.Create(ctx, CreateOptions{
			Name: "fail-start-puck",
		})
		assert.Error(t, err)
		assert.Nil(t, p)

		// Container should be removed
		assert.True(t, mock.WasCalled("RemoveContainer"))
	})

	t.Run("assigns unique port for each puck", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		p1, err := mgr.Create(ctx, CreateOptions{Name: "puck1"})
		require.NoError(t, err)

		p2, err := mgr.Create(ctx, CreateOptions{Name: "puck2"})
		require.NoError(t, err)

		assert.NotEqual(t, p1.HostPort, p2.HostPort)
		assert.Equal(t, BaseHostPort, p1.HostPort)
		assert.Equal(t, BaseHostPort+1, p2.HostPort)
	})
}

func TestGet(t *testing.T) {
	t.Run("retrieves existing puck", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		created, err := mgr.Create(ctx, CreateOptions{Name: "get-puck"})
		require.NoError(t, err)

		retrieved, err := mgr.Get(ctx, "get-puck")
		require.NoError(t, err)
		assert.Equal(t, created.Name, retrieved.Name)
	})

	t.Run("returns error for non-existent puck", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Get(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestList(t *testing.T) {
	t.Run("returns empty list when no pucks", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		pucks, err := mgr.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, pucks)
	})

	t.Run("returns all pucks", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "list-puck1"})
		require.NoError(t, err)
		_, err = mgr.Create(ctx, CreateOptions{Name: "list-puck2"})
		require.NoError(t, err)

		pucks, err := mgr.List(ctx)
		require.NoError(t, err)
		assert.Len(t, pucks, 2)
	})

	t.Run("updates status from podman", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "status-puck"})
		require.NoError(t, err)

		// Container reports stopped
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return false, nil
		}

		pucks, err := mgr.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, store.StatusStopped, pucks[0].Status)
	})
}

func TestStart(t *testing.T) {
	t.Run("starts stopped puck", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "start-puck"})
		require.NoError(t, err)

		mock.Reset() // Clear creation calls

		err = mgr.Start(ctx, "start-puck")
		require.NoError(t, err)

		assert.True(t, mock.WasCalled("StartContainer"))
	})

	t.Run("returns error for non-existent puck", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		err := mgr.Start(ctx, "non-existent")
		assert.Error(t, err)
	})

	t.Run("updates status to running", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "start-status-puck"})
		require.NoError(t, err)

		err = mgr.Start(ctx, "start-status-puck")
		require.NoError(t, err)

		p, err := mgr.Get(ctx, "start-status-puck")
		require.NoError(t, err)
		assert.Equal(t, store.StatusRunning, p.Status)
	})
}

func TestStop(t *testing.T) {
	t.Run("stops running puck", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "stop-puck"})
		require.NoError(t, err)

		mock.Reset()

		err = mgr.Stop(ctx, "stop-puck")
		require.NoError(t, err)

		assert.True(t, mock.WasCalled("StopContainer"))
	})

	t.Run("updates status to stopped", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "stop-status-puck"})
		require.NoError(t, err)

		err = mgr.Stop(ctx, "stop-status-puck")
		require.NoError(t, err)

		p, err := mgr.Get(ctx, "stop-status-puck")
		require.NoError(t, err)
		assert.Equal(t, store.StatusStopped, p.Status)
	})
}

func TestDestroy(t *testing.T) {
	t.Run("destroys puck and cleans up", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		p, err := mgr.Create(ctx, CreateOptions{Name: "destroy-puck"})
		require.NoError(t, err)

		volumeDir := p.VolumeDir
		mock.Reset()
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return false, nil
		}

		err = mgr.Destroy(ctx, "destroy-puck", false)
		require.NoError(t, err)

		// Verify container was removed
		assert.True(t, mock.WasCalled("RemoveContainer"))

		// Verify volume directory was removed
		_, statErr := os.Stat(volumeDir)
		assert.True(t, os.IsNotExist(statErr))

		// Verify puck is gone from database
		_, err = mgr.Get(ctx, "destroy-puck")
		assert.Error(t, err)
	})

	t.Run("stops running container before removing", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "destroy-running-puck"})
		require.NoError(t, err)

		mock.Reset()
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return true, nil
		}

		err = mgr.Destroy(ctx, "destroy-running-puck", false)
		require.NoError(t, err)

		assert.True(t, mock.WasCalled("StopContainer"))
	})

	t.Run("force destroys without stopping", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "force-destroy-puck"})
		require.NoError(t, err)

		mock.Reset()
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return true, nil
		}

		err = mgr.Destroy(ctx, "force-destroy-puck", true)
		require.NoError(t, err)

		// With force=true, StopContainer should NOT be called
		assert.False(t, mock.WasCalled("StopContainer"))
		assert.True(t, mock.WasCalled("RemoveContainer"))
	})
}

func TestDestroyAll(t *testing.T) {
	t.Run("destroys all pucks", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "all-puck1"})
		require.NoError(t, err)
		_, err = mgr.Create(ctx, CreateOptions{Name: "all-puck2"})
		require.NoError(t, err)

		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return false, nil
		}

		destroyed, err := mgr.DestroyAll(ctx, true)
		require.NoError(t, err)
		assert.Len(t, destroyed, 2)
		assert.Contains(t, destroyed, "all-puck1")
		assert.Contains(t, destroyed, "all-puck2")

		// Verify all pucks are gone
		pucks, err := mgr.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, pucks)
	})

	t.Run("returns empty slice when no pucks", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		destroyed, err := mgr.DestroyAll(ctx, true)
		require.NoError(t, err)
		assert.Empty(t, destroyed)
	})
}

func TestExists(t *testing.T) {
	t.Run("returns true for existing puck", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "exists-puck"})
		require.NoError(t, err)

		assert.True(t, mgr.Exists(ctx, "exists-puck"))
	})

	t.Run("returns false for non-existent puck", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		assert.False(t, mgr.Exists(ctx, "non-existent"))
	})
}

func TestFindAvailablePort(t *testing.T) {
	t.Run("returns base port when no pucks", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		port, err := mgr.findAvailablePort(ctx)
		require.NoError(t, err)
		assert.Equal(t, BaseHostPort, port)
	})

	t.Run("skips used ports", func(t *testing.T) {
		mgr, _, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		// Create pucks to use ports
		_, err := mgr.Create(ctx, CreateOptions{Name: "port-puck1"})
		require.NoError(t, err)
		_, err = mgr.Create(ctx, CreateOptions{Name: "port-puck2"})
		require.NoError(t, err)

		// Next port should be BaseHostPort+2
		port, err := mgr.findAvailablePort(ctx)
		require.NoError(t, err)
		assert.Equal(t, BaseHostPort+2, port)
	})

	t.Run("reuses freed ports", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		// Create and destroy to free first port
		_, err := mgr.Create(ctx, CreateOptions{Name: "temp-puck"})
		require.NoError(t, err)

		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return false, nil
		}
		err = mgr.Destroy(ctx, "temp-puck", true)
		require.NoError(t, err)

		// Should reuse base port
		port, err := mgr.findAvailablePort(ctx)
		require.NoError(t, err)
		assert.Equal(t, BaseHostPort, port)
	})
}

func TestConsole(t *testing.T) {
	t.Run("opens console on running container", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "console-puck"})
		require.NoError(t, err)

		mock.Reset()
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return true, nil
		}

		err = mgr.Console(ctx, "console-puck", "/bin/bash")
		require.NoError(t, err)

		assert.True(t, mock.WasCalled("Console"))
	})

	t.Run("starts stopped container before console", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "stopped-console-puck"})
		require.NoError(t, err)

		mock.Reset()
		callCount := 0
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			callCount++
			// First call returns false (not running), subsequent calls return true
			return callCount > 1, nil
		}

		err = mgr.Console(ctx, "stopped-console-puck", "/bin/bash")
		require.NoError(t, err)

		assert.True(t, mock.WasCalled("StartContainer"))
		assert.True(t, mock.WasCalled("Console"))
	})
}

func TestCreateSnapshot(t *testing.T) {
	t.Run("creates snapshot of running puck", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		// Create checkpoint file when Checkpoint is called
		mock.CheckpointFunc = func(ctx context.Context, nameOrID string, opts podman.CheckpointOptions) error {
			// Create the snapshot file
			if err := os.MkdirAll(filepath.Dir(opts.ExportPath), 0755); err != nil {
				return err
			}
			return os.WriteFile(opts.ExportPath, []byte("checkpoint-data"), 0644)
		}

		_, err := mgr.Create(ctx, CreateOptions{Name: "snapshot-puck"})
		require.NoError(t, err)

		mock.Reset()
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return true, nil
		}

		snapshot, err := mgr.CreateSnapshot(ctx, SnapshotCreateOptions{
			PuckName:     "snapshot-puck",
			SnapshotName: "test-snap",
			LeaveRunning: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "test-snap", snapshot.Name)
		assert.NotEmpty(t, snapshot.Path)
		assert.True(t, mock.WasCalled("Checkpoint"))
	})

	t.Run("fails when puck not running", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		_, err := mgr.Create(ctx, CreateOptions{Name: "stopped-snap-puck"})
		require.NoError(t, err)

		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return false, nil
		}

		_, err = mgr.CreateSnapshot(ctx, SnapshotCreateOptions{
			PuckName:     "stopped-snap-puck",
			SnapshotName: "test-snap",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be running")
	})
}

func TestListSnapshots(t *testing.T) {
	t.Run("returns snapshots for puck", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		mock.CheckpointFunc = func(ctx context.Context, nameOrID string, opts podman.CheckpointOptions) error {
			if err := os.MkdirAll(filepath.Dir(opts.ExportPath), 0755); err != nil {
				return err
			}
			return os.WriteFile(opts.ExportPath, []byte("data"), 0644)
		}
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return true, nil
		}

		_, err := mgr.Create(ctx, CreateOptions{Name: "list-snap-puck"})
		require.NoError(t, err)

		_, err = mgr.CreateSnapshot(ctx, SnapshotCreateOptions{
			PuckName:     "list-snap-puck",
			SnapshotName: "snap1",
			LeaveRunning: true,
		})
		require.NoError(t, err)

		snapshots, err := mgr.ListSnapshots(ctx, "list-snap-puck")
		require.NoError(t, err)
		assert.Len(t, snapshots, 1)
		assert.Equal(t, "snap1", snapshots[0].Name)
	})
}

func TestDeleteSnapshot(t *testing.T) {
	t.Run("deletes snapshot and file", func(t *testing.T) {
		mgr, mock, cleanup := setupTestManager(t)
		defer cleanup()
		ctx := context.Background()

		var snapshotPath string
		mock.CheckpointFunc = func(ctx context.Context, nameOrID string, opts podman.CheckpointOptions) error {
			snapshotPath = opts.ExportPath
			if err := os.MkdirAll(filepath.Dir(opts.ExportPath), 0755); err != nil {
				return err
			}
			return os.WriteFile(opts.ExportPath, []byte("data"), 0644)
		}
		mock.IsRunningFunc = func(ctx context.Context, nameOrID string) (bool, error) {
			return true, nil
		}

		_, err := mgr.Create(ctx, CreateOptions{Name: "delete-snap-puck"})
		require.NoError(t, err)

		_, err = mgr.CreateSnapshot(ctx, SnapshotCreateOptions{
			PuckName:     "delete-snap-puck",
			SnapshotName: "to-delete",
			LeaveRunning: true,
		})
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(snapshotPath)
		require.NoError(t, err)

		err = mgr.DeleteSnapshot(ctx, "delete-snap-puck", "to-delete")
		require.NoError(t, err)

		// Verify file is gone
		_, err = os.Stat(snapshotPath)
		assert.True(t, os.IsNotExist(err))

		// Verify snapshot is gone from database
		snapshots, err := mgr.ListSnapshots(ctx, "delete-snap-puck")
		require.NoError(t, err)
		assert.Empty(t, snapshots)
	})
}
