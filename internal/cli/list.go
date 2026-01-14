package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all pucks",
	Long:    `List all pucks managed by puck.`,
	RunE:    runList,
}

func runList(cmd *cobra.Command, args []string) error {
	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	pucks, err := client.List()
	if err != nil {
		return err
	}

	if len(pucks) == 0 {
		fmt.Println("No pucks found. Create one with: puck create <name>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tIMAGE\tCREATED")

	for _, p := range pucks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			p.Name,
			p.Status,
			p.Image,
			p.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	return w.Flush()
}
