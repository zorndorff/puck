package podman

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings/containers"
)

// CheckpointOptions contains options for checkpointing a container
type CheckpointOptions struct {
	ExportPath   string // Path to export checkpoint archive
	LeaveRunning bool   // Keep container running after checkpoint
}

// RestoreOptions contains options for restoring a container
type RestoreOptions struct {
	ImportPath string // Path to checkpoint archive
	Name       string // New container name (optional)
}

// Checkpoint creates a CRIU checkpoint of a running container
func (c *Client) Checkpoint(ctx context.Context, nameOrID string, opts CheckpointOptions) error {
	checkpointOpts := new(containers.CheckpointOptions)

	if opts.ExportPath != "" {
		checkpointOpts = checkpointOpts.WithExport(opts.ExportPath)
	}
	if opts.LeaveRunning {
		checkpointOpts = checkpointOpts.WithLeaveRunning(true)
	}

	// Enable TCP connection checkpoint (for network sockets)
	checkpointOpts = checkpointOpts.WithTCPEstablished(true)

	_, err := containers.Checkpoint(c.conn, nameOrID, checkpointOpts)
	if err != nil {
		return fmt.Errorf("checkpointing container: %w", err)
	}

	return nil
}

// Restore restores a container from a CRIU checkpoint
func (c *Client) Restore(ctx context.Context, opts RestoreOptions) (string, error) {
	restoreOpts := new(containers.RestoreOptions)

	if opts.ImportPath != "" {
		restoreOpts = restoreOpts.WithImportArchive(opts.ImportPath)
	}
	if opts.Name != "" {
		restoreOpts = restoreOpts.WithName(opts.Name)
	}

	// Enable TCP connection restore
	restoreOpts = restoreOpts.WithTCPEstablished(true)

	response, err := containers.Restore(c.conn, "", restoreOpts)
	if err != nil {
		return "", fmt.Errorf("restoring container: %w", err)
	}

	return response.Id, nil
}
