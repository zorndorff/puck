package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
	"github.com/sandwich-labs/puck/internal/systemd"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Daemon management commands",
	Long:  `Commands for managing the puck daemon.`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the puck daemon",
	Long:  `Start the puck daemon in the foreground.`,
	RunE:  runDaemonStart,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon status",
	Long:  `Check if the puck daemon is running.`,
	RunE:  runDaemonStatus,
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install puckd as a systemd user service",
	Long: `Install the puckd daemon as a systemd user service.

This will:
- Copy the puckd binary to ~/.local/bin/
- Create a systemd user service file
- Enable the service to start on login

Use --now to also start the service immediately.`,
	RunE: runDaemonInstall,
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the puckd systemd service",
	Long: `Uninstall the puckd systemd user service.

This will:
- Stop the service if running
- Disable the service
- Remove the service file

Use --remove-binary to also remove the puckd binary.`,
	RunE: runDaemonUninstall,
}

var (
	installNow       bool
	uninstallBinary  bool
)

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)

	daemonInstallCmd.Flags().BoolVar(&installNow, "now", false, "Start the service immediately after installation")
	daemonUninstallCmd.Flags().BoolVar(&uninstallBinary, "remove-binary", false, "Also remove the puckd binary")
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	log.Info("Starting puck daemon")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	d, err := daemon.New()
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	go func() {
		if err := d.Run(ctx); err != nil {
			log.Error("Daemon error", "error", err)
			cancel()
		}
	}()

	<-sigCh
	log.Info("Shutting down daemon")
	d.Shutdown()

	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	// Check if running via systemd
	if systemd.IsInstalled() {
		if systemd.IsRunning() {
			fmt.Println("Daemon is running (systemd user service)")
		} else {
			fmt.Println("Daemon is installed but not running")
			fmt.Println("Start with: systemctl --user start puckd")
		}
		return nil
	}

	// Fall back to direct ping check
	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		fmt.Println("Daemon is not running")
		return nil
	}

	fmt.Println("Daemon is running")
	return nil
}

func runDaemonInstall(cmd *cobra.Command, args []string) error {
	// Find the puckd binary - look next to current executable first
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}

	// Look for puckd in the same directory as puck
	puckdPath := execPath[:len(execPath)-len("puck")] + "puckd"
	if _, err := os.Stat(puckdPath); os.IsNotExist(err) {
		// Try looking in current directory
		puckdPath = "puckd"
		if _, err := os.Stat(puckdPath); os.IsNotExist(err) {
			return fmt.Errorf("puckd binary not found - build it first with 'go build ./cmd/puckd'")
		}
	}

	// Check if already installed
	if systemd.IsInstalled() {
		fmt.Println("puckd is already installed as a systemd service")
		fmt.Println("Uninstall first with: puck daemon uninstall")
		return nil
	}

	fmt.Printf("Installing puckd from %s...\n", puckdPath)

	if err := systemd.Install(puckdPath); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	servicePath, _ := systemd.ServiceFilePath()
	binaryPath, _ := systemd.BinaryInstallPath()

	fmt.Println("Installation complete!")
	fmt.Printf("  Binary: %s\n", binaryPath)
	fmt.Printf("  Service: %s\n", servicePath)

	if installNow {
		fmt.Println("Starting service...")
		if err := systemd.Start(); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}
		fmt.Println("Service started successfully")
	} else {
		fmt.Println("\nTo start the daemon:")
		fmt.Println("  systemctl --user start puckd")
		fmt.Println("\nTo view logs:")
		fmt.Println("  journalctl --user -u puckd -f")
	}

	return nil
}

func runDaemonUninstall(cmd *cobra.Command, args []string) error {
	if !systemd.IsInstalled() {
		fmt.Println("puckd is not installed as a systemd service")
		return nil
	}

	fmt.Println("Uninstalling puckd service...")

	if err := systemd.Uninstall(uninstallBinary); err != nil {
		return fmt.Errorf("uninstallation failed: %w", err)
	}

	fmt.Println("Service uninstalled successfully")
	if uninstallBinary {
		fmt.Println("Binary removed")
	} else {
		binaryPath, _ := systemd.BinaryInstallPath()
		fmt.Printf("Binary still available at: %s\n", binaryPath)
	}

	return nil
}
