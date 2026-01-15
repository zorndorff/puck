package main

import (
	"os"

	"github.com/sandwich-labs/puck/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
