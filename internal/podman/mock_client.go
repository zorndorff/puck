package podman

import (
	"context"

	"github.com/containers/podman/v5/libpod/define"
)

// MockClient is a mock implementation of ContainerClient for testing.
type MockClient struct {
	// Function hooks for each method - set these to customize behavior
	CreateContainerFunc   func(ctx context.Context, opts CreateContainerOptions) (string, error)
	StartContainerFunc    func(ctx context.Context, nameOrID string) error
	StopContainerFunc     func(ctx context.Context, nameOrID string) error
	RemoveContainerFunc   func(ctx context.Context, nameOrID string, force bool) error
	InspectContainerFunc  func(ctx context.Context, nameOrID string) (*define.InspectContainerData, error)
	GetContainerIPFunc    func(ctx context.Context, nameOrID string) (string, error)
	IsRunningFunc         func(ctx context.Context, nameOrID string) (bool, error)
	ContainerExistsFunc   func(ctx context.Context, nameOrID string) (bool, error)
	CheckpointFunc        func(ctx context.Context, nameOrID string, opts CheckpointOptions) error
	RestoreFunc           func(ctx context.Context, opts RestoreOptions) (string, error)
	ConsoleFunc           func(ctx context.Context, containerID string, shell string) error
	ExecFunc              func(ctx context.Context, containerID string, opts ExecOptions) error
	PingFunc              func(ctx context.Context) error

	// Track calls for verification
	Calls []MockCall
}

// MockCall records a method call for verification.
type MockCall struct {
	Method string
	Args   []interface{}
}

// NewMockClient creates a new MockClient with sensible defaults.
func NewMockClient() *MockClient {
	return &MockClient{
		Calls: make([]MockCall, 0),
		// Default implementations return nil/empty values
		CreateContainerFunc:  func(ctx context.Context, opts CreateContainerOptions) (string, error) { return "mock-container-id", nil },
		StartContainerFunc:   func(ctx context.Context, nameOrID string) error { return nil },
		StopContainerFunc:    func(ctx context.Context, nameOrID string) error { return nil },
		RemoveContainerFunc:  func(ctx context.Context, nameOrID string, force bool) error { return nil },
		InspectContainerFunc: func(ctx context.Context, nameOrID string) (*define.InspectContainerData, error) { return &define.InspectContainerData{}, nil },
		GetContainerIPFunc:   func(ctx context.Context, nameOrID string) (string, error) { return "10.88.0.2", nil },
		IsRunningFunc:        func(ctx context.Context, nameOrID string) (bool, error) { return true, nil },
		ContainerExistsFunc:  func(ctx context.Context, nameOrID string) (bool, error) { return true, nil },
		CheckpointFunc:       func(ctx context.Context, nameOrID string, opts CheckpointOptions) error { return nil },
		RestoreFunc:          func(ctx context.Context, opts RestoreOptions) (string, error) { return "restored-container-id", nil },
		ConsoleFunc:          func(ctx context.Context, containerID string, shell string) error { return nil },
		ExecFunc:             func(ctx context.Context, containerID string, opts ExecOptions) error { return nil },
		PingFunc:             func(ctx context.Context) error { return nil },
	}
}

func (m *MockClient) recordCall(method string, args ...interface{}) {
	m.Calls = append(m.Calls, MockCall{Method: method, Args: args})
}

func (m *MockClient) CreateContainer(ctx context.Context, opts CreateContainerOptions) (string, error) {
	m.recordCall("CreateContainer", opts)
	return m.CreateContainerFunc(ctx, opts)
}

func (m *MockClient) StartContainer(ctx context.Context, nameOrID string) error {
	m.recordCall("StartContainer", nameOrID)
	return m.StartContainerFunc(ctx, nameOrID)
}

func (m *MockClient) StopContainer(ctx context.Context, nameOrID string) error {
	m.recordCall("StopContainer", nameOrID)
	return m.StopContainerFunc(ctx, nameOrID)
}

func (m *MockClient) RemoveContainer(ctx context.Context, nameOrID string, force bool) error {
	m.recordCall("RemoveContainer", nameOrID, force)
	return m.RemoveContainerFunc(ctx, nameOrID, force)
}

func (m *MockClient) InspectContainer(ctx context.Context, nameOrID string) (*define.InspectContainerData, error) {
	m.recordCall("InspectContainer", nameOrID)
	return m.InspectContainerFunc(ctx, nameOrID)
}

func (m *MockClient) GetContainerIP(ctx context.Context, nameOrID string) (string, error) {
	m.recordCall("GetContainerIP", nameOrID)
	return m.GetContainerIPFunc(ctx, nameOrID)
}

func (m *MockClient) IsRunning(ctx context.Context, nameOrID string) (bool, error) {
	m.recordCall("IsRunning", nameOrID)
	return m.IsRunningFunc(ctx, nameOrID)
}

func (m *MockClient) ContainerExists(ctx context.Context, nameOrID string) (bool, error) {
	m.recordCall("ContainerExists", nameOrID)
	return m.ContainerExistsFunc(ctx, nameOrID)
}

func (m *MockClient) Checkpoint(ctx context.Context, nameOrID string, opts CheckpointOptions) error {
	m.recordCall("Checkpoint", nameOrID, opts)
	return m.CheckpointFunc(ctx, nameOrID, opts)
}

func (m *MockClient) Restore(ctx context.Context, opts RestoreOptions) (string, error) {
	m.recordCall("Restore", opts)
	return m.RestoreFunc(ctx, opts)
}

func (m *MockClient) Console(ctx context.Context, containerID string, shell string) error {
	m.recordCall("Console", containerID, shell)
	return m.ConsoleFunc(ctx, containerID, shell)
}

func (m *MockClient) Exec(ctx context.Context, containerID string, opts ExecOptions) error {
	m.recordCall("Exec", containerID, opts)
	return m.ExecFunc(ctx, containerID, opts)
}

func (m *MockClient) Ping(ctx context.Context) error {
	m.recordCall("Ping")
	return m.PingFunc(ctx)
}

func (m *MockClient) Context() context.Context {
	return context.Background()
}

func (m *MockClient) IsMachine() bool {
	return false
}

// CallCount returns the number of times a method was called.
func (m *MockClient) CallCount(method string) int {
	count := 0
	for _, call := range m.Calls {
		if call.Method == method {
			count++
		}
	}
	return count
}

// WasCalled returns true if the method was called at least once.
func (m *MockClient) WasCalled(method string) bool {
	return m.CallCount(method) > 0
}

// Reset clears all recorded calls.
func (m *MockClient) Reset() {
	m.Calls = make([]MockCall, 0)
}

// Verify MockClient implements ContainerClient.
var _ ContainerClient = (*MockClient)(nil)
