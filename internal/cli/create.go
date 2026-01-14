package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/sandwich-labs/puck/internal/daemon"
	"github.com/sandwich-labs/puck/internal/sprite"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new sprite",
	Long:  `Create a new persistent container (sprite) with the given name.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCreate,
}

var (
	createImage string
	createPorts []string
)

func init() {
	createCmd.Flags().StringVarP(&createImage, "image", "i", "fedora:latest", "base image to use")
	createCmd.Flags().StringSliceVarP(&createPorts, "port", "p", nil, "ports to expose (e.g., 8080:80)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		name = generateSpriteName()
	}

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	// Check daemon is running
	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	log.Info("Creating sprite", "name", name, "image", createImage)

	s, err := client.Create(sprite.CreateOptions{
		Name:  name,
		Image: createImage,
		Ports: createPorts,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created sprite '%s' (accessible at http://%s.localhost)\n", s.Name, s.Name)
	return nil
}

func generateSpriteName() string {
	adjectives := []string{"swift", "brave", "calm", "eager", "fair", "glad", "keen", "neat", "wise", "bold"}
	nouns := []string{"fox", "owl", "elk", "bee", "ant", "bat", "cat", "dog", "eel", "jay"}

	t := time.Now().UnixNano()
	adj := adjectives[int(t)%len(adjectives)]
	noun := nouns[int(t/10)%len(nouns)]

	return fmt.Sprintf("%s-%s", adj, noun)
}
