package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/config"
	"github.com/sandwich-labs/puck/internal/podman"
	"github.com/sandwich-labs/puck/internal/sprite"
	"github.com/sandwich-labs/puck/internal/store"
)

var consoleCmd = &cobra.Command{
	Use:   "console [name]",
	Short: "Open a shell in a sprite",
	Long:  `Connect to a sprite and open an interactive shell.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConsole,
}

var consoleShell string

func init() {
	consoleCmd.Flags().StringVarP(&consoleShell, "shell", "s", "/bin/bash", "shell to use")
}

func runConsole(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Console needs direct access to podman for interactive exec
	// so we bypass the daemon for this command
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pc, err := podman.NewClient(ctx, cfg.PodmanSocket)
	if err != nil {
		return fmt.Errorf("connecting to podman: %w", err)
	}

	db, err := store.Open(cfg.DatabasePath())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	mgr := sprite.NewManager(cfg, pc, db)

	return mgr.Console(ctx, name, consoleShell)
}
