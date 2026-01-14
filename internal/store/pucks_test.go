package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "puck-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return db, cleanup
}

// createTestPuck creates a test puck for testing
func createTestPuck(name string) *Puck {
	now := time.Now()
	return &Puck{
		ID:        "test-id-" + name,
		Name:      name,
		Image:     "fedora:latest",
		Status:    StatusRunning,
		VolumeDir: "/tmp/puck/test/" + name,
		Ports:     []string{"8080:80"},
		HostPort:  9000,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCreatePuck(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("creates puck successfully", func(t *testing.T) {
		puck := createTestPuck("test-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		// Verify puck was created
		retrieved, err := db.GetPuck(ctx, "test-puck")
		require.NoError(t, err)
		assert.Equal(t, puck.Name, retrieved.Name)
		assert.Equal(t, puck.Image, retrieved.Image)
		assert.Equal(t, puck.Status, retrieved.Status)
		assert.Equal(t, puck.VolumeDir, retrieved.VolumeDir)
		assert.Equal(t, puck.Ports, retrieved.Ports)
		assert.Equal(t, puck.HostPort, retrieved.HostPort)
	})

	t.Run("fails on duplicate name", func(t *testing.T) {
		puck1 := createTestPuck("duplicate-puck")
		err := db.CreatePuck(ctx, puck1)
		require.NoError(t, err)

		puck2 := createTestPuck("duplicate-puck")
		puck2.ID = "different-id" // Different ID but same name
		err = db.CreatePuck(ctx, puck2)
		assert.Error(t, err, "should fail with duplicate name")
	})

	t.Run("handles empty ports array", func(t *testing.T) {
		puck := createTestPuck("no-ports-puck")
		puck.Ports = []string{}
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "no-ports-puck")
		require.NoError(t, err)
		assert.Equal(t, []string{}, retrieved.Ports)
	})

	t.Run("handles nil ports array", func(t *testing.T) {
		puck := createTestPuck("nil-ports-puck")
		puck.Ports = nil
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "nil-ports-puck")
		require.NoError(t, err)
		// nil is stored as JSON "null" and retrieved as empty slice
		assert.Empty(t, retrieved.Ports)
	})
}

func TestGetPuck(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("retrieves existing puck", func(t *testing.T) {
		puck := createTestPuck("get-test-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "get-test-puck")
		require.NoError(t, err)
		assert.Equal(t, puck.Name, retrieved.Name)
	})

	t.Run("returns error for non-existent puck", func(t *testing.T) {
		_, err := db.GetPuck(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGetPuckByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("retrieves existing puck by ID", func(t *testing.T) {
		puck := createTestPuck("get-by-id-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		retrieved, err := db.GetPuckByID(ctx, puck.ID)
		require.NoError(t, err)
		assert.Equal(t, puck.Name, retrieved.Name)
		assert.Equal(t, puck.ID, retrieved.ID)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.GetPuckByID(ctx, "non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestListPucks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("returns empty list when no pucks", func(t *testing.T) {
		pucks, err := db.ListPucks(ctx)
		require.NoError(t, err)
		assert.Empty(t, pucks)
	})

	t.Run("returns all pucks ordered by created_at DESC", func(t *testing.T) {
		// Create pucks with different timestamps
		puck1 := createTestPuck("first-puck")
		puck1.CreatedAt = time.Now().Add(-2 * time.Hour)
		err := db.CreatePuck(ctx, puck1)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		puck2 := createTestPuck("second-puck")
		puck2.CreatedAt = time.Now().Add(-1 * time.Hour)
		err = db.CreatePuck(ctx, puck2)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		puck3 := createTestPuck("third-puck")
		puck3.CreatedAt = time.Now()
		err = db.CreatePuck(ctx, puck3)
		require.NoError(t, err)

		pucks, err := db.ListPucks(ctx)
		require.NoError(t, err)
		require.Len(t, pucks, 3)

		// Should be ordered by created_at DESC (newest first)
		assert.Equal(t, "third-puck", pucks[0].Name)
		assert.Equal(t, "second-puck", pucks[1].Name)
		assert.Equal(t, "first-puck", pucks[2].Name)
	})
}

func TestUpdatePuckStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("updates status successfully", func(t *testing.T) {
		puck := createTestPuck("status-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		err = db.UpdatePuckStatus(ctx, "status-puck", StatusStopped)
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "status-puck")
		require.NoError(t, err)
		assert.Equal(t, StatusStopped, retrieved.Status)
	})

	t.Run("updates timestamp", func(t *testing.T) {
		puck := createTestPuck("timestamp-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		err = db.UpdatePuckStatus(ctx, "timestamp-puck", StatusRunning)
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "timestamp-puck")
		require.NoError(t, err)
		assert.True(t, retrieved.UpdatedAt.After(puck.UpdatedAt))
	})

	t.Run("returns error for non-existent puck", func(t *testing.T) {
		err := db.UpdatePuckStatus(ctx, "non-existent", StatusRunning)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUpdatePuckContainerIP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("updates container IP", func(t *testing.T) {
		puck := createTestPuck("ip-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		err = db.UpdatePuckContainerIP(ctx, "ip-puck", "10.88.0.5")
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "ip-puck")
		require.NoError(t, err)
		assert.Equal(t, "10.88.0.5", retrieved.ContainerIP)
	})
}

func TestUpdatePuckTailscale(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("updates tailscale info", func(t *testing.T) {
		puck := createTestPuck("tailscale-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		err = db.UpdatePuckTailscale(ctx, "tailscale-puck", "100.64.1.1", "https://puck.example.ts.net")
		require.NoError(t, err)

		retrieved, err := db.GetPuck(ctx, "tailscale-puck")
		require.NoError(t, err)
		assert.Equal(t, "100.64.1.1", retrieved.TailscaleIP)
		assert.Equal(t, "https://puck.example.ts.net", retrieved.FunnelURL)
	})
}

func TestDeletePuck(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("deletes existing puck", func(t *testing.T) {
		puck := createTestPuck("delete-puck")
		err := db.CreatePuck(ctx, puck)
		require.NoError(t, err)

		err = db.DeletePuck(ctx, "delete-puck")
		require.NoError(t, err)

		// Verify it's gone
		_, err = db.GetPuck(ctx, "delete-puck")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for non-existent puck", func(t *testing.T) {
		err := db.DeletePuck(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestStatus(t *testing.T) {
	t.Run("status constants are correct", func(t *testing.T) {
		assert.Equal(t, Status("running"), StatusRunning)
		assert.Equal(t, Status("stopped"), StatusStopped)
		assert.Equal(t, Status("checkpointed"), StatusCheckpointed)
		assert.Equal(t, Status("creating"), StatusCreating)
		assert.Equal(t, Status("error"), StatusError)
	})
}
