package systemd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const serviceName = "puckd.service"
const binaryName = "puckd"

// serviceTemplate is the systemd user service file content
const serviceTemplate = `[Unit]
Description=Puck Daemon - Container Management Service
Documentation=https://github.com/sandwich-labs/puck
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5s
Environment="PUCK_DATA_DIR=%s"

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=puckd

[Install]
WantedBy=default.target
`

// UserServiceDir returns the systemd user service directory
func UserServiceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// ServiceFilePath returns the full path to the service file
func ServiceFilePath() (string, error) {
	dir, err := UserServiceDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serviceName), nil
}

// BinaryInstallPath returns the path where puckd should be installed
func BinaryInstallPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin", binaryName), nil
}

// DataDir returns the default puck data directory
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "puck"), nil
}

// IsInstalled checks if the systemd service is installed
func IsInstalled() bool {
	path, err := ServiceFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// IsRunning checks if the systemd service is running
func IsRunning() bool {
	cmd := exec.Command("systemctl", "--user", "is-active", "--quiet", serviceName)
	return cmd.Run() == nil
}

// IsEnabled checks if the systemd service is enabled
func IsEnabled() bool {
	cmd := exec.Command("systemctl", "--user", "is-enabled", "--quiet", serviceName)
	return cmd.Run() == nil
}

// Install installs the puckd binary and systemd service
func Install(sourceBinaryPath string) error {
	// Get paths
	destBinaryPath, err := BinaryInstallPath()
	if err != nil {
		return err
	}

	serviceDir, err := UserServiceDir()
	if err != nil {
		return err
	}

	servicePath, err := ServiceFilePath()
	if err != nil {
		return err
	}

	dataDir, err := DataDir()
	if err != nil {
		return err
	}

	// Create directories
	if err := os.MkdirAll(filepath.Dir(destBinaryPath), 0755); err != nil {
		return fmt.Errorf("creating binary directory: %w", err)
	}

	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("creating service directory: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	// Copy binary
	if err := copyFile(sourceBinaryPath, destBinaryPath); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}

	// Make binary executable
	if err := os.Chmod(destBinaryPath, 0755); err != nil {
		return fmt.Errorf("setting binary permissions: %w", err)
	}

	// Generate and write service file
	serviceContent := fmt.Sprintf(serviceTemplate, destBinaryPath, dataDir)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("writing service file: %w", err)
	}

	// Reload systemd
	if err := DaemonReload(); err != nil {
		return fmt.Errorf("reloading systemd: %w", err)
	}

	// Enable service
	if err := Enable(); err != nil {
		return fmt.Errorf("enabling service: %w", err)
	}

	return nil
}

// Uninstall removes the systemd service and optionally the binary
func Uninstall(removeBinary bool) error {
	// Stop service if running
	if IsRunning() {
		if err := Stop(); err != nil {
			// Continue even if stop fails
			fmt.Fprintf(os.Stderr, "Warning: failed to stop service: %v\n", err)
		}
	}

	// Disable service if enabled
	if IsEnabled() {
		if err := Disable(); err != nil {
			// Continue even if disable fails
			fmt.Fprintf(os.Stderr, "Warning: failed to disable service: %v\n", err)
		}
	}

	// Remove service file
	servicePath, err := ServiceFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing service file: %w", err)
	}

	// Reload systemd
	if err := DaemonReload(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to reload systemd: %v\n", err)
	}

	// Optionally remove binary
	if removeBinary {
		binaryPath, err := BinaryInstallPath()
		if err != nil {
			return err
		}
		if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing binary: %w", err)
		}
	}

	return nil
}

// DaemonReload runs systemctl --user daemon-reload
func DaemonReload() error {
	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Enable enables the systemd service
func Enable() error {
	cmd := exec.Command("systemctl", "--user", "enable", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Disable disables the systemd service
func Disable() error {
	cmd := exec.Command("systemctl", "--user", "disable", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Start starts the systemd service
func Start() error {
	cmd := exec.Command("systemctl", "--user", "start", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Stop stops the systemd service
func Stop() error {
	cmd := exec.Command("systemctl", "--user", "stop", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Status returns the status output from systemctl
func Status() (string, error) {
	cmd := exec.Command("systemctl", "--user", "status", serviceName)
	output, _ := cmd.CombinedOutput()
	// Don't check error - systemctl status returns non-zero for stopped services
	return string(output), nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
