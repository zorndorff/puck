package daemon

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientWithSocket(t *testing.T) {
	t.Run("creates client with specified socket path", func(t *testing.T) {
		client := NewClientWithSocket("/custom/path.sock")
		assert.Equal(t, "/custom/path.sock", client.socketPath)
	})
}

// mockServer creates a mock Unix socket server for testing
func setupMockServer(t *testing.T, handler func(net.Conn)) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "puck-test-*")
	require.NoError(t, err)

	socketPath := filepath.Join(dir, "test.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	// Accept connections in goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Server closed
			}
			handler(conn)
		}
	}()

	cleanup := func() {
		listener.Close()
		os.RemoveAll(dir)
	}

	return socketPath, cleanup
}

func TestPing(t *testing.T) {
	t.Run("returns nil on success response", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			// Read request
			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			// Send success response
			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		err := client.Ping()
		assert.NoError(t, err)
	})

	t.Run("returns error on failure response", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: false, Error: "test error"})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		err := client.Ping()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
	})

	t.Run("returns error when daemon not running", func(t *testing.T) {
		client := NewClientWithSocket("/nonexistent/socket.sock")
		err := client.Ping()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connecting to daemon")
		assert.Contains(t, err.Error(), "is puckd running")
	})
}

func TestList(t *testing.T) {
	t.Run("returns puck list from response", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			// Verify action
			assert.Equal(t, "list", req.Action)

			// Send response with pucks
			pucksJSON, _ := json.Marshal([]map[string]interface{}{
				{"name": "puck1", "status": "running"},
				{"name": "puck2", "status": "stopped"},
			})
			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true, Data: pucksJSON})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		pucks, err := client.List()
		require.NoError(t, err)
		assert.Len(t, pucks, 2)
	})

	t.Run("returns error on failure response", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: false, Error: "database error"})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		pucks, err := client.List()
		assert.Error(t, err)
		assert.Nil(t, pucks)
		assert.Contains(t, err.Error(), "database error")
	})
}

func TestStart(t *testing.T) {
	t.Run("sends correct action and name", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			// Verify request
			assert.Equal(t, "start", req.Action)

			var params map[string]string
			json.Unmarshal(req.Data, &params)
			assert.Equal(t, "my-puck", params["name"])

			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		err := client.Start("my-puck")
		assert.NoError(t, err)
	})
}

func TestStop(t *testing.T) {
	t.Run("sends correct action and name", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			assert.Equal(t, "stop", req.Action)

			var params map[string]string
			json.Unmarshal(req.Data, &params)
			assert.Equal(t, "my-puck", params["name"])

			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		err := client.Stop("my-puck")
		assert.NoError(t, err)
	})
}

func TestDestroy(t *testing.T) {
	t.Run("sends correct action with force flag", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			assert.Equal(t, "destroy", req.Action)

			var params map[string]interface{}
			json.Unmarshal(req.Data, &params)
			assert.Equal(t, "my-puck", params["name"])
			assert.Equal(t, true, params["force"])

			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		err := client.Destroy("my-puck", true)
		assert.NoError(t, err)
	})
}

func TestDestroyAll(t *testing.T) {
	t.Run("returns list of destroyed pucks", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			assert.Equal(t, "destroy-all", req.Action)

			destroyedJSON, _ := json.Marshal([]string{"puck1", "puck2"})
			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true, Data: destroyedJSON})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		destroyed, err := client.DestroyAll(true)
		require.NoError(t, err)
		assert.Equal(t, []string{"puck1", "puck2"}, destroyed)
	})
}

func TestSnapshotCreate(t *testing.T) {
	t.Run("sends correct parameters", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			assert.Equal(t, "snapshot-create", req.Action)

			var params map[string]interface{}
			json.Unmarshal(req.Data, &params)
			assert.Equal(t, "my-puck", params["puck_name"])
			assert.Equal(t, "snap1", params["snapshot_name"])
			assert.Equal(t, true, params["leave_running"])

			snapshotJSON, _ := json.Marshal(map[string]interface{}{
				"id":        "snap-id",
				"name":      "snap1",
				"puck_name": "my-puck",
			})
			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true, Data: snapshotJSON})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		snapshot, err := client.SnapshotCreate("my-puck", "snap1", true)
		require.NoError(t, err)
		assert.Equal(t, "snap1", snapshot.Name)
	})
}

func TestSnapshotList(t *testing.T) {
	t.Run("returns snapshots from response", func(t *testing.T) {
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			var req Request
			decoder.Decode(&req)

			assert.Equal(t, "snapshot-list", req.Action)

			snapshotsJSON, _ := json.Marshal([]map[string]interface{}{
				{"name": "snap1"},
				{"name": "snap2"},
			})
			encoder := json.NewEncoder(conn)
			encoder.Encode(Response{Success: true, Data: snapshotsJSON})
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		snapshots, err := client.SnapshotList("my-puck")
		require.NoError(t, err)
		assert.Len(t, snapshots, 2)
	})
}

func TestSendTimeout(t *testing.T) {
	t.Run("connection timeout when server doesn't respond", func(t *testing.T) {
		// Create a server that never responds
		socketPath, cleanup := setupMockServer(t, func(conn net.Conn) {
			// Read request but never respond
			io.ReadAll(conn)
			time.Sleep(40 * time.Second) // Longer than client timeout
		})
		defer cleanup()

		client := NewClientWithSocket(socketPath)
		// This should timeout since server never responds
		err := client.Ping()
		assert.Error(t, err)
	})
}
