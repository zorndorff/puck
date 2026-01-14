package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sandwich-labs/puck/internal/config"
	"github.com/sandwich-labs/puck/internal/puck"
	"github.com/sandwich-labs/puck/internal/store"
)

// Client communicates with the puckd daemon
type Client struct {
	socketPath string
}

// NewClient creates a new daemon client
func NewClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &Client{socketPath: cfg.DaemonSocket}, nil
}

// NewClientWithSocket creates a client with a specific socket path
func NewClientWithSocket(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) send(req *Request) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w (is puckd running?)", err)
	}
	defer conn.Close()

	// Set deadlines
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &resp, nil
}

// Ping checks if the daemon is running
func (c *Client) Ping() error {
	resp, err := c.send(&Request{Action: "ping"})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// Create creates a new puck
func (c *Client) Create(opts puck.CreateOptions) (*store.Puck, error) {
	data, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.send(&Request{Action: "create", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	var p store.Puck
	if err := json.Unmarshal(resp.Data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns all pucks
func (c *Client) List() ([]*store.Puck, error) {
	resp, err := c.send(&Request{Action: "list"})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	var pucks []*store.Puck
	if err := json.Unmarshal(resp.Data, &pucks); err != nil {
		return nil, err
	}
	return pucks, nil
}

// Get retrieves a puck by name
func (c *Client) Get(name string) (*store.Puck, error) {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.send(&Request{Action: "get", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	var p store.Puck
	if err := json.Unmarshal(resp.Data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Start starts a puck
func (c *Client) Start(name string) error {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.send(&Request{Action: "start", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// Stop stops a puck
func (c *Client) Stop(name string) error {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.send(&Request{Action: "stop", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// Destroy removes a puck
func (c *Client) Destroy(name string, force bool) error {
	data, _ := json.Marshal(map[string]interface{}{"name": name, "force": force})
	resp, err := c.send(&Request{Action: "destroy", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// DestroyAll removes all pucks
func (c *Client) DestroyAll(force bool) ([]string, error) {
	data, _ := json.Marshal(map[string]interface{}{"force": force})
	resp, err := c.send(&Request{Action: "destroy-all", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	var destroyed []string
	if err := json.Unmarshal(resp.Data, &destroyed); err != nil {
		return nil, err
	}
	return destroyed, nil
}

// SnapshotCreate creates a checkpoint snapshot of a puck
func (c *Client) SnapshotCreate(puckName, snapshotName string, leaveRunning bool) (*store.Snapshot, error) {
	data, _ := json.Marshal(puck.SnapshotCreateOptions{
		PuckName:     puckName,
		SnapshotName: snapshotName,
		LeaveRunning: leaveRunning,
	})
	resp, err := c.send(&Request{Action: "snapshot-create", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	var snapshot store.Snapshot
	if err := json.Unmarshal(resp.Data, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// SnapshotRestore restores a puck from a checkpoint snapshot
func (c *Client) SnapshotRestore(puckName, snapshotName string) error {
	data, _ := json.Marshal(puck.SnapshotRestoreOptions{
		PuckName:     puckName,
		SnapshotName: snapshotName,
	})
	resp, err := c.send(&Request{Action: "snapshot-restore", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// SnapshotList returns all snapshots for a puck
func (c *Client) SnapshotList(puckName string) ([]*store.Snapshot, error) {
	data, _ := json.Marshal(map[string]string{"puck_name": puckName})
	resp, err := c.send(&Request{Action: "snapshot-list", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	var snapshots []*store.Snapshot
	if err := json.Unmarshal(resp.Data, &snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

// SnapshotDelete deletes a snapshot
func (c *Client) SnapshotDelete(puckName, snapshotName string) error {
	data, _ := json.Marshal(map[string]string{
		"puck_name":     puckName,
		"snapshot_name": snapshotName,
	})
	resp, err := c.send(&Request{Action: "snapshot-delete", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}
