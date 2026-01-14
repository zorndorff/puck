package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage puck snapshots",
	Long:  `Create, restore, and manage CRIU checkpoint snapshots of pucks.`,
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create <puck> <name>",
	Short: "Create a snapshot of a puck",
	Long: `Create a CRIU checkpoint snapshot of a running puck.

This captures the complete state of the container including memory, processes,
and network connections. The snapshot can later be restored to bring the
puck back to this exact state.`,
	Args: cobra.ExactArgs(2),
	RunE: runSnapshotCreate,
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore <puck> <name>",
	Short: "Restore a puck from a snapshot",
	Long: `Restore a puck to a previously saved snapshot state.

This replaces the current container with one restored from the checkpoint,
including all memory state, running processes, and network connections.`,
	Args: cobra.ExactArgs(2),
	RunE: runSnapshotRestore,
}

var snapshotListCmd = &cobra.Command{
	Use:     "list <puck>",
	Aliases: []string{"ls"},
	Short:   "List snapshots for a puck",
	Args:    cobra.ExactArgs(1),
	RunE:    runSnapshotList,
}

var snapshotDeleteCmd = &cobra.Command{
	Use:     "delete <puck> <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a snapshot",
	Args:    cobra.ExactArgs(2),
	RunE:    runSnapshotDelete,
}

var (
	snapshotLeaveRunning bool
)

func init() {
	snapshotCreateCmd.Flags().BoolVar(&snapshotLeaveRunning, "leave-running", false, "keep puck running after snapshot")

	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
}

func runSnapshotCreate(cmd *cobra.Command, args []string) error {
	puckName := args[0]
	snapshotName := args[1]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	fmt.Printf("Creating snapshot '%s' of puck '%s'...\n", snapshotName, puckName)

	snapshot, err := client.SnapshotCreate(puckName, snapshotName, snapshotLeaveRunning)
	if err != nil {
		return err
	}

	fmt.Printf("Snapshot created: %s (%s)\n", snapshot.Name, humanize.Bytes(uint64(snapshot.SizeBytes)))
	if !snapshotLeaveRunning {
		fmt.Println("Puck is now checkpointed (stopped). Use 'puck snapshot restore' to restore it.")
	}

	return nil
}

func runSnapshotRestore(cmd *cobra.Command, args []string) error {
	puckName := args[0]
	snapshotName := args[1]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	fmt.Printf("Restoring puck '%s' from snapshot '%s'...\n", puckName, snapshotName)

	if err := client.SnapshotRestore(puckName, snapshotName); err != nil {
		return err
	}

	fmt.Println("Puck restored and running")
	return nil
}

func runSnapshotList(cmd *cobra.Command, args []string) error {
	puckName := args[0]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	snapshots, err := client.SnapshotList(puckName)
	if err != nil {
		return err
	}

	if len(snapshots) == 0 {
		fmt.Printf("No snapshots for puck '%s'\n", puckName)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tCREATED")
	for _, s := range snapshots {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			s.Name,
			humanize.Bytes(uint64(s.SizeBytes)),
			humanize.Time(s.CreatedAt),
		)
	}
	w.Flush()

	return nil
}

func runSnapshotDelete(cmd *cobra.Command, args []string) error {
	puckName := args[0]
	snapshotName := args[1]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	if err := client.SnapshotDelete(puckName, snapshotName); err != nil {
		return err
	}

	fmt.Printf("Deleted snapshot '%s' from puck '%s'\n", snapshotName, puckName)
	return nil
}
