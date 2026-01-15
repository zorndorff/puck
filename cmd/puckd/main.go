package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/sandwich-labs/puck/internal/daemon"
)

func main() {
	log.Info("Starting puckd daemon")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	d, err := daemon.New()
	if err != nil {
		log.Fatal("Failed to create daemon", "error", err)
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
}
