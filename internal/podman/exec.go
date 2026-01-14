package podman

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

// ExecOptions contains options for executing a command in a container
type ExecOptions struct {
	Cmd         []string
	Interactive bool
	TTY         bool
	WorkDir     string
	Env         []string
	User        string
}

// Exec executes a command in a container using podman CLI
// This is simpler and more reliable than the bindings for interactive use
func (c *Client) Exec(ctx context.Context, containerID string, opts ExecOptions) error {
	args := []string{"exec"}

	if opts.Interactive {
		args = append(args, "-i")
	}
	if opts.TTY {
		args = append(args, "-t")
	}
	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}
	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}

	args = append(args, containerID)
	args = append(args, opts.Cmd...)

	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up TTY if needed
	if opts.TTY {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setctty: true,
			Setsid:  true,
		}
	}

	return cmd.Run()
}

// Console opens an interactive shell in a container
func (c *Client) Console(ctx context.Context, containerID string, shell string) error {
	if shell == "" {
		shell = "/bin/bash"
	}

	return c.Exec(ctx, containerID, ExecOptions{
		Cmd:         []string{shell},
		Interactive: true,
		TTY:         true,
	})
}
