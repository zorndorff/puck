package podman

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// CreateContainerOptions contains options for creating a container
type CreateContainerOptions struct {
	Name    string
	Image   string
	Volumes map[string]string // host:container
	Ports   []string          // "8080:80" format
	Labels  map[string]string
	Systemd bool
}

// CreateContainer creates a new container
func (c *Client) CreateContainer(ctx context.Context, opts CreateContainerOptions) (string, error) {
	// Ensure image is available
	if err := c.ensureImage(ctx, opts.Image); err != nil {
		return "", fmt.Errorf("ensuring image: %w", err)
	}

	// Create spec
	spec := specgen.NewSpecGenerator(opts.Image, false)
	spec.Name = opts.Name
	terminal := true
	spec.Terminal = &terminal
	spec.Stdin = &terminal

	// Enable systemd if requested
	if opts.Systemd {
		spec.Systemd = "always"
	}

	// Add puck management labels
	spec.Labels = map[string]string{
		"managed-by":  "puck",
		"puck.sprite": opts.Name,
	}
	for k, v := range opts.Labels {
		spec.Labels[k] = v
	}

	// Configure mounts for persistence
	for hostPath, containerPath := range opts.Volumes {
		spec.Mounts = append(spec.Mounts, specs.Mount{
			Type:        "bind",
			Source:      hostPath,
			Destination: containerPath,
			Options:     []string{"rw"},
		})
	}

	// Configure port mappings
	for _, portSpec := range opts.Ports {
		pm, err := parsePortMapping(portSpec)
		if err != nil {
			continue // Skip invalid port specs
		}
		spec.PortMappings = append(spec.PortMappings, pm)
	}

	// Create the container
	response, err := containers.CreateWithSpec(c.conn, spec, nil)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	return response.ID, nil
}

// ensureImage pulls the image if not present locally
func (c *Client) ensureImage(ctx context.Context, imageName string) error {
	// Check if image exists
	exists, err := images.Exists(c.conn, imageName, nil)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Pull the image
	_, err = images.Pull(c.conn, imageName, nil)
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", imageName, err)
	}

	return nil
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, nameOrID string) error {
	return containers.Start(c.conn, nameOrID, nil)
}

// StopContainer stops a container
func (c *Client) StopContainer(ctx context.Context, nameOrID string) error {
	timeout := uint(10)
	opts := new(containers.StopOptions).WithTimeout(timeout)
	return containers.Stop(c.conn, nameOrID, opts)
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, nameOrID string, force bool) error {
	opts := new(containers.RemoveOptions).WithForce(force).WithVolumes(true)
	_, err := containers.Remove(c.conn, nameOrID, opts)
	return err
}

// InspectContainer returns container details
func (c *Client) InspectContainer(ctx context.Context, nameOrID string) (*define.InspectContainerData, error) {
	data, err := containers.Inspect(c.conn, nameOrID, nil)
	if err != nil {
		return nil, fmt.Errorf("inspecting container: %w", err)
	}
	return data, nil
}

// GetContainerIP returns the container's IP address
func (c *Client) GetContainerIP(ctx context.Context, nameOrID string) (string, error) {
	data, err := c.InspectContainer(ctx, nameOrID)
	if err != nil {
		return "", err
	}

	// Try to get IP from network settings
	for _, net := range data.NetworkSettings.Networks {
		if net.IPAddress != "" {
			return net.IPAddress, nil
		}
	}

	return "", fmt.Errorf("no IP address found for container")
}

// IsRunning checks if a container is running
func (c *Client) IsRunning(ctx context.Context, nameOrID string) (bool, error) {
	data, err := c.InspectContainer(ctx, nameOrID)
	if err != nil {
		return false, err
	}
	return data.State.Running, nil
}

// ContainerExists checks if a container exists
func (c *Client) ContainerExists(ctx context.Context, nameOrID string) (bool, error) {
	exists, err := containers.Exists(c.conn, nameOrID, nil)
	return exists, err
}

// parsePortMapping parses a port spec like "8080:80" into a nettypes.PortMapping
func parsePortMapping(portSpec string) (nettypes.PortMapping, error) {
	parts := strings.Split(portSpec, ":")
	if len(parts) != 2 {
		return nettypes.PortMapping{}, fmt.Errorf("invalid port spec: %s", portSpec)
	}

	hostPort, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return nettypes.PortMapping{}, fmt.Errorf("invalid host port: %s", parts[0])
	}

	containerPort, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return nettypes.PortMapping{}, fmt.Errorf("invalid container port: %s", parts[1])
	}

	return nettypes.PortMapping{
		HostPort:      uint16(hostPort),
		ContainerPort: uint16(containerPort),
		Protocol:      "tcp",
	}, nil
}
