package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
)

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a stopped sprite",
	Long:  `Start a sprite that was previously stopped.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	name := args[0]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	if err := client.Start(name); err != nil {
		return err
	}

	fmt.Printf("Started sprite '%s'\n", name)
	return nil
}
