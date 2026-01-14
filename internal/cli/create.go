package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/sandwich-labs/puck/internal/daemon"
	"github.com/sandwich-labs/puck/internal/puck"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new puck",
	Long:  `Create a new persistent container (puck) with the given name.`,
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
		name = generatePuckName()
	}

	client, err := daemon.NewClient()
	if err != nil {
		return err
	}

	// Check daemon is running
	if err := client.Ping(); err != nil {
		return fmt.Errorf("daemon not running: %w\nStart with: puck daemon start", err)
	}

	log.Info("Creating puck", "name", name, "image", createImage)

	p, err := client.Create(puck.CreateOptions{
		Name:  name,
		Image: createImage,
		Ports: createPorts,
	})
	if err != nil {
		return err
	}

	// Show access URLs
	port := viper.GetInt("router_port")
	if port == 0 {
		port = 8080
	}
	tailnet := viper.GetString("tailnet")

	fmt.Printf("Created puck '%s'\n", p.Name)
	fmt.Printf("  Local:  http://localhost:%d/%s\n", port, p.Name)
	if tailnet != "" {
		fmt.Printf("  Remote: https://puck.%s/%s\n", tailnet, p.Name)
	}
	return nil
}

func generatePuckName() string {
	adjectives := []string{"swift", "brave", "calm", "eager", "fair", "glad", "keen", "neat", "wise", "bold"}
	nouns := []string{"fox", "owl", "elk", "bee", "ant", "bat", "cat", "dog", "eel", "jay"}

	t := time.Now().UnixNano()
	adj := adjectives[int(t)%len(adjectives)]
	noun := nouns[int(t/10)%len(nouns)]

	return fmt.Sprintf("%s-%s", adj, noun)
}
