package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop a running puck",
	Long:  `Stop a running puck.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	name := args[0]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	if err := client.Stop(name); err != nil {
		return err
	}

	fmt.Printf("Stopped puck '%s'\n", name)
	return nil
}
