package podman

import (
	"context"

	"github.com/containers/podman/v5/libpod/define"
)

//go:generate mockgen -source=interfaces.go -destination=mock_client.go -package=podman

// ContainerClient defines the interface for container operations.
// This interface allows for easy mocking in tests.
type ContainerClient interface {
	// Container lifecycle
	CreateContainer(ctx context.Context, opts CreateContainerOptions) (string, error)
	StartContainer(ctx context.Context, nameOrID string) error
	StopContainer(ctx context.Context, nameOrID string) error
	RemoveContainer(ctx context.Context, nameOrID string, force bool) error

	// Container inspection
	InspectContainer(ctx context.Context, nameOrID string) (*define.InspectContainerData, error)
	GetContainerIP(ctx context.Context, nameOrID string) (string, error)
	IsRunning(ctx context.Context, nameOrID string) (bool, error)
	ContainerExists(ctx context.Context, nameOrID string) (bool, error)

	// Checkpoint/restore (CRIU)
	Checkpoint(ctx context.Context, nameOrID string, opts CheckpointOptions) error
	Restore(ctx context.Context, opts RestoreOptions) (string, error)

	// Interactive
	Console(ctx context.Context, containerID string, shell string) error
	Exec(ctx context.Context, containerID string, opts ExecOptions) error

	// Utility
	Ping(ctx context.Context) error
	Context() context.Context
	IsMachine() bool
}

// Verify that Client implements ContainerClient interface.
var _ ContainerClient = (*Client)(nil)
