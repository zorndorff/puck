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

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
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
