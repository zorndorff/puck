package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestSnapshot creates a test snapshot for testing
func createTestSnapshot(puckID, puckName, snapshotName string) *Snapshot {
	return &Snapshot{
		ID:        "snapshot-id-" + puckName + "-" + snapshotName, // Include puckName to ensure uniqueness
		PuckID:    puckID,
		PuckName:  puckName,
		Name:      snapshotName,
		Path:      "/tmp/snapshots/" + puckName + "/" + snapshotName + ".tar.gz",
		SizeBytes: 1024 * 1024, // 1MB
		CreatedAt: time.Now(),
	}
}

func TestCreateSnapshot(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// First create a puck for the snapshot
	puck := createTestPuck("snapshot-test-puck")
	err := db.CreatePuck(ctx, puck)
	require.NoError(t, err)

	t.Run("creates snapshot successfully", func(t *testing.T) {
		snapshot := createTestSnapshot(puck.ID, puck.Name, "test-snapshot")
		err := db.CreateSnapshot(ctx, snapshot)
		require.NoError(t, err)

		// Verify it was created
		retrieved, err := db.GetSnapshot(ctx, puck.ID, "test-snapshot")
		require.NoError(t, err)
		assert.Equal(t, snapshot.Name, retrieved.Name)
		assert.Equal(t, snapshot.Path, retrieved.Path)
		assert.Equal(t, snapshot.SizeBytes, retrieved.SizeBytes)
	})

	t.Run("fails on duplicate snapshot name for same puck", func(t *testing.T) {
		snapshot1 := createTestSnapshot(puck.ID, puck.Name, "dup-snapshot")
		err := db.CreateSnapshot(ctx, snapshot1)
		require.NoError(t, err)

		snapshot2 := createTestSnapshot(puck.ID, puck.Name, "dup-snapshot")
		snapshot2.ID = "different-id"
		err = db.CreateSnapshot(ctx, snapshot2)
		assert.Error(t, err, "should fail with duplicate snapshot name for same puck")
	})

	t.Run("allows same snapshot name for different pucks", func(t *testing.T) {
		// Create another puck
		puck2 := createTestPuck("another-puck")
		err := db.CreatePuck(ctx, puck2)
		require.NoError(t, err)

		snapshot1 := createTestSnapshot(puck.ID, puck.Name, "shared-name")
		err = db.CreateSnapshot(ctx, snapshot1)
		require.NoError(t, err)

		snapshot2 := createTestSnapshot(puck2.ID, puck2.Name, "shared-name")
		err = db.CreateSnapshot(ctx, snapshot2)
		require.NoError(t, err, "should allow same name for different pucks")
	})
}

func TestGetSnapshot(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create puck and snapshot
	puck := createTestPuck("get-snapshot-puck")
	err := db.CreatePuck(ctx, puck)
	require.NoError(t, err)

	snapshot := createTestSnapshot(puck.ID, puck.Name, "get-snapshot")
	err = db.CreateSnapshot(ctx, snapshot)
	require.NoError(t, err)

	t.Run("retrieves existing snapshot", func(t *testing.T) {
		retrieved, err := db.GetSnapshot(ctx, puck.ID, "get-snapshot")
		require.NoError(t, err)
		assert.Equal(t, snapshot.ID, retrieved.ID)
		assert.Equal(t, snapshot.Name, retrieved.Name)
	})

	t.Run("returns error for non-existent snapshot", func(t *testing.T) {
		_, err := db.GetSnapshot(ctx, puck.ID, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for wrong puck ID", func(t *testing.T) {
		_, err := db.GetSnapshot(ctx, "wrong-puck-id", "get-snapshot")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestListSnapshots(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create puck
	puck := createTestPuck("list-snapshots-puck")
	err := db.CreatePuck(ctx, puck)
	require.NoError(t, err)

	t.Run("returns empty list when no snapshots", func(t *testing.T) {
		snapshots, err := db.ListSnapshots(ctx, puck.ID)
		require.NoError(t, err)
		assert.Empty(t, snapshots)
	})

	t.Run("returns all snapshots for puck ordered by created_at DESC", func(t *testing.T) {
		// Create snapshots
		s1 := createTestSnapshot(puck.ID, puck.Name, "first")
		s1.CreatedAt = time.Now().Add(-2 * time.Hour)
		err := db.CreateSnapshot(ctx, s1)
		require.NoError(t, err)

		s2 := createTestSnapshot(puck.ID, puck.Name, "second")
		s2.CreatedAt = time.Now().Add(-1 * time.Hour)
		err = db.CreateSnapshot(ctx, s2)
		require.NoError(t, err)

		s3 := createTestSnapshot(puck.ID, puck.Name, "third")
		s3.CreatedAt = time.Now()
		err = db.CreateSnapshot(ctx, s3)
		require.NoError(t, err)

		snapshots, err := db.ListSnapshots(ctx, puck.ID)
		require.NoError(t, err)
		require.Len(t, snapshots, 3)

		// Should be ordered by created_at DESC (newest first)
		assert.Equal(t, "third", snapshots[0].Name)
		assert.Equal(t, "second", snapshots[1].Name)
		assert.Equal(t, "first", snapshots[2].Name)
	})

	t.Run("only returns snapshots for specified puck", func(t *testing.T) {
		// Create another puck with snapshots
		puck2 := createTestPuck("other-puck-list")
		err := db.CreatePuck(ctx, puck2)
		require.NoError(t, err)

		otherSnapshot := createTestSnapshot(puck2.ID, puck2.Name, "other-snapshot")
		err = db.CreateSnapshot(ctx, otherSnapshot)
		require.NoError(t, err)

		// List snapshots for original puck
		snapshots, err := db.ListSnapshots(ctx, puck.ID)
		require.NoError(t, err)

		// Should not include other puck's snapshots
		for _, s := range snapshots {
			assert.Equal(t, puck.ID, s.PuckID)
		}
	})
}

func TestDeleteSnapshot(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create puck and snapshot
	puck := createTestPuck("delete-snapshot-puck")
	err := db.CreatePuck(ctx, puck)
	require.NoError(t, err)

	snapshot := createTestSnapshot(puck.ID, puck.Name, "delete-me")
	err = db.CreateSnapshot(ctx, snapshot)
	require.NoError(t, err)

	t.Run("deletes snapshot successfully", func(t *testing.T) {
		err := db.DeleteSnapshot(ctx, snapshot.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = db.GetSnapshot(ctx, puck.ID, "delete-me")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("no error for non-existent snapshot ID", func(t *testing.T) {
		// SQLite DELETE doesn't return error for non-existent rows
		err := db.DeleteSnapshot(ctx, "non-existent-id")
		assert.NoError(t, err)
	})
}

func TestDeleteSnapshotsByPuck(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create puck with multiple snapshots
	puck := createTestPuck("delete-all-snapshots-puck")
	err := db.CreatePuck(ctx, puck)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		snapshot := createTestSnapshot(puck.ID, puck.Name, "snap"+string(rune('0'+i)))
		err := db.CreateSnapshot(ctx, snapshot)
		require.NoError(t, err)
	}

	t.Run("deletes all snapshots for puck", func(t *testing.T) {
		// Verify snapshots exist
		snapshots, err := db.ListSnapshots(ctx, puck.ID)
		require.NoError(t, err)
		require.Len(t, snapshots, 3)

		// Delete all snapshots
		err = db.DeleteSnapshotsByPuck(ctx, puck.ID)
		require.NoError(t, err)

		// Verify they're gone
		snapshots, err = db.ListSnapshots(ctx, puck.ID)
		require.NoError(t, err)
		assert.Empty(t, snapshots)
	})
}

func TestCascadeDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create puck with snapshots
	puck := createTestPuck("cascade-puck")
	err := db.CreatePuck(ctx, puck)
	require.NoError(t, err)

	snapshot := createTestSnapshot(puck.ID, puck.Name, "cascade-snapshot")
	err = db.CreateSnapshot(ctx, snapshot)
	require.NoError(t, err)

	t.Run("deleting puck cascades to snapshots", func(t *testing.T) {
		// Verify snapshot exists
		retrieved, err := db.GetSnapshot(ctx, puck.ID, "cascade-snapshot")
		require.NoError(t, err)
		assert.NotNil(t, retrieved)

		// Delete the puck
		err = db.DeletePuck(ctx, puck.Name)
		require.NoError(t, err)

		// Verify snapshot is also gone (foreign key cascade)
		snapshots, err := db.ListSnapshots(ctx, puck.ID)
		require.NoError(t, err)
		assert.Empty(t, snapshots)
	})
}
