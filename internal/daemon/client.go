package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/sandwich-labs/puck/internal/config"
	"github.com/sandwich-labs/puck/internal/sprite"
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
		return fmt.Errorf(resp.Error)
	}
	return nil
}

// Create creates a new sprite
func (c *Client) Create(opts sprite.CreateOptions) (*store.Sprite, error) {
	data, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.send(&Request{Action: "create", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf(resp.Error)
	}

	var s store.Sprite
	if err := json.Unmarshal(resp.Data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns all sprites
func (c *Client) List() ([]*store.Sprite, error) {
	resp, err := c.send(&Request{Action: "list"})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf(resp.Error)
	}

	var sprites []*store.Sprite
	if err := json.Unmarshal(resp.Data, &sprites); err != nil {
		return nil, err
	}
	return sprites, nil
}

// Get retrieves a sprite by name
func (c *Client) Get(name string) (*store.Sprite, error) {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.send(&Request{Action: "get", Data: data})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf(resp.Error)
	}

	var s store.Sprite
	if err := json.Unmarshal(resp.Data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Start starts a sprite
func (c *Client) Start(name string) error {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.send(&Request{Action: "start", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Error)
	}
	return nil
}

// Stop stops a sprite
func (c *Client) Stop(name string) error {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.send(&Request{Action: "stop", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Error)
	}
	return nil
}

// Destroy removes a sprite
func (c *Client) Destroy(name string, force bool) error {
	data, _ := json.Marshal(map[string]interface{}{"name": name, "force": force})
	resp, err := c.send(&Request{Action: "destroy", Data: data})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Error)
	}
	return nil
}
