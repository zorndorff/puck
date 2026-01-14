package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
)

var destroyCmd = &cobra.Command{
	Use:     "destroy [name]",
	Aliases: []string{"rm", "remove"},
	Short:   "Destroy a sprite",
	Long:    `Destroy a sprite and remove all its data.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDestroy,
}

var destroyForce bool

func init() {
	destroyCmd.Flags().BoolVarP(&destroyForce, "force", "f", false, "force removal even if running")
}

func runDestroy(cmd *cobra.Command, args []string) error {
	name := args[0]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	if err := client.Destroy(name, destroyForce); err != nil {
		return err
	}

	fmt.Printf("Destroyed sprite '%s'\n", name)
	return nil
}
