package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
)

var destroyCmd = &cobra.Command{
	Use:     "destroy [name]",
	Aliases: []string{"rm", "remove"},
	Short:   "Destroy a puck",
	Long:    `Destroy a puck and remove all its data. Use --all to destroy all pucks.`,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runDestroy,
}

var (
	destroyForce bool
	destroyAll   bool
)

func init() {
	destroyCmd.Flags().BoolVarP(&destroyForce, "force", "f", false, "force removal even if running")
	destroyCmd.Flags().BoolVar(&destroyAll, "all", false, "destroy all pucks")
}

func runDestroy(cmd *cobra.Command, args []string) error {
	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	if destroyAll {
		destroyed, err := client.DestroyAll(destroyForce)
		if err != nil {
			return err
		}
		if len(destroyed) == 0 {
			fmt.Println("No pucks to destroy")
		} else {
			for _, name := range destroyed {
				fmt.Printf("Destroyed puck '%s'\n", name)
			}
		}
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("puck name required (or use --all)")
	}

	name := args[0]
	if err := client.Destroy(name, destroyForce); err != nil {
		return err
	}

	fmt.Printf("Destroyed puck '%s'\n", name)
	return nil
}
