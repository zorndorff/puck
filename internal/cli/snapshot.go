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
	Short: "Manage sprite snapshots",
	Long:  `Create, restore, and manage CRIU checkpoint snapshots of sprites.`,
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create <sprite> <name>",
	Short: "Create a snapshot of a sprite",
	Long: `Create a CRIU checkpoint snapshot of a running sprite.

This captures the complete state of the container including memory, processes,
and network connections. The snapshot can later be restored to bring the
sprite back to this exact state.`,
	Args: cobra.ExactArgs(2),
	RunE: runSnapshotCreate,
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore <sprite> <name>",
	Short: "Restore a sprite from a snapshot",
	Long: `Restore a sprite to a previously saved snapshot state.

This replaces the current container with one restored from the checkpoint,
including all memory state, running processes, and network connections.`,
	Args: cobra.ExactArgs(2),
	RunE: runSnapshotRestore,
}

var snapshotListCmd = &cobra.Command{
	Use:     "list <sprite>",
	Aliases: []string{"ls"},
	Short:   "List snapshots for a sprite",
	Args:    cobra.ExactArgs(1),
	RunE:    runSnapshotList,
}

var snapshotDeleteCmd = &cobra.Command{
	Use:     "delete <sprite> <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a snapshot",
	Args:    cobra.ExactArgs(2),
	RunE:    runSnapshotDelete,
}

var (
	snapshotLeaveRunning bool
)

func init() {
	snapshotCreateCmd.Flags().BoolVar(&snapshotLeaveRunning, "leave-running", false, "keep sprite running after snapshot")

	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
}

func runSnapshotCreate(cmd *cobra.Command, args []string) error {
	spriteName := args[0]
	snapshotName := args[1]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	fmt.Printf("Creating snapshot '%s' of sprite '%s'...\n", snapshotName, spriteName)

	snapshot, err := client.SnapshotCreate(spriteName, snapshotName, snapshotLeaveRunning)
	if err != nil {
		return err
	}

	fmt.Printf("Snapshot created: %s (%s)\n", snapshot.Name, humanize.Bytes(uint64(snapshot.SizeBytes)))
	if !snapshotLeaveRunning {
		fmt.Println("Sprite is now checkpointed (stopped). Use 'puck snapshot restore' to restore it.")
	}

	return nil
}

func runSnapshotRestore(cmd *cobra.Command, args []string) error {
	spriteName := args[0]
	snapshotName := args[1]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	fmt.Printf("Restoring sprite '%s' from snapshot '%s'...\n", spriteName, snapshotName)

	if err := client.SnapshotRestore(spriteName, snapshotName); err != nil {
		return err
	}

	fmt.Println("Sprite restored and running")
	return nil
}

func runSnapshotList(cmd *cobra.Command, args []string) error {
	spriteName := args[0]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	snapshots, err := client.SnapshotList(spriteName)
	if err != nil {
		return err
	}

	if len(snapshots) == 0 {
		fmt.Printf("No snapshots for sprite '%s'\n", spriteName)
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
	spriteName := args[0]
	snapshotName := args[1]

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	if err := client.SnapshotDelete(spriteName, snapshotName); err != nil {
		return err
	}

	fmt.Printf("Deleted snapshot '%s' from sprite '%s'\n", snapshotName, spriteName)
	return nil
}
