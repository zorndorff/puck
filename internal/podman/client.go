package podman

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/containers/podman/v5/pkg/bindings"
)

// Client wraps the Podman connection
type Client struct {
	conn context.Context
}

// NewClient creates a new Podman client
func NewClient(ctx context.Context, socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = detectSocket()
	}

	conn, err := bindings.NewConnection(ctx, socketPath)
	if err != nil {
		return nil, fmt.Errorf("connecting to podman at %s: %w", socketPath, err)
	}

	return &Client{conn: conn}, nil
}

// Context returns the connection context for use with podman bindings
func (c *Client) Context() context.Context {
	return c.conn
}

// IsMachine returns true if running on Mac/Windows (using Podman Machine)
func (c *Client) IsMachine() bool {
	return runtime.GOOS != "linux"
}

// detectSocket attempts to find the Podman socket
func detectSocket() string {
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
		if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
			return "unix:///run/podman/podman.sock"
		}
		return ""

	case "darwin":
		home, _ := os.UserHomeDir()
		// Try common Podman Machine locations
		paths := []string{
			home + "/.local/share/containers/podman/machine/podman.sock",
			home + "/.local/share/containers/podman/machine/qemu/podman.sock",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return "unix://" + p
			}
		}
		return ""

	default:
		return ""
	}
}

// Ping tests the connection to Podman
func (c *Client) Ping(ctx context.Context) error {
	// Use the system info endpoint to verify connection
	_, err := bindings.GetClient(c.conn)
	return err
}
