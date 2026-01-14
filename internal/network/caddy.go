package network

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/caddyserver/caddy/v2"

	// Register Caddy modules
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	_ "github.com/caddyserver/caddy/v2/modules/caddytls"
	_ "github.com/caddyserver/caddy/v2/modules/caddytls/standardstek"

	// Register Tailscale integration
	_ "github.com/tailscale/caddy-tailscale"
)

// Router manages HTTP routing for pucks via Caddy
type Router struct {
	mu       sync.RWMutex
	routes   map[string]routeInfo // puck name -> route info
	port     int
	running  bool
	domain   string // e.g., "localhost"
	tailnet  string // tailnet name for Tailscale mode (optional)
}

type routeInfo struct {
	IP   string
	Port int
}

// NewRouter creates a new Caddy-based router
func NewRouter(port int, domain string) *Router {
	if domain == "" {
		domain = "localhost"
	}
	return &Router{
		routes: make(map[string]routeInfo),
		port:   port,
		domain: domain,
	}
}

// SetTailnet enables Tailscale mode with the given tailnet name
func (r *Router) SetTailnet(tailnet string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tailnet = tailnet
}

// Start initializes and starts the Caddy server
func (r *Router) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}

	cfg := r.buildConfig()
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := caddy.Load(cfgJSON, false); err != nil {
		return fmt.Errorf("loading caddy config: %w", err)
	}

	r.running = true
	return nil
}

// Stop shuts down the Caddy server
func (r *Router) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	if err := caddy.Stop(); err != nil {
		return fmt.Errorf("stopping caddy: %w", err)
	}

	r.running = false
	return nil
}

// AddRoute adds or updates a route for a puck
func (r *Router) AddRoute(puckName string, containerIP string, containerPort int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes[puckName] = routeInfo{IP: containerIP, Port: containerPort}

	return r.reload()
}

// RemoveRoute removes a route for a puck
func (r *Router) RemoveRoute(puckName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.routes, puckName)

	return r.reload()
}

// GetRoutes returns all current routes
func (r *Router) GetRoutes() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make(map[string]string)
	for name, info := range r.routes {
		routes[name] = fmt.Sprintf("%s:%d", info.IP, info.Port)
	}
	return routes
}

// reload updates the Caddy config with current routes
func (r *Router) reload() error {
	if !r.running {
		return nil
	}

	cfg := r.buildConfig()
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := caddy.Load(cfgJSON, false); err != nil {
		return fmt.Errorf("reloading caddy config: %w", err)
	}

	return nil
}

// buildConfig creates the Caddy configuration
// Uses path-based routing: /puck-name/* -> puck backend
func (r *Router) buildConfig() map[string]interface{} {
	routes := make([]map[string]interface{}, 0)

	// Add routes for each puck using path-based routing
	for name, info := range r.routes {
		target := fmt.Sprintf("%s:%d", info.IP, info.Port)
		pathPrefix := fmt.Sprintf("/%s", name)

		route := map[string]interface{}{
			"match": []map[string]interface{}{
				{"path": []string{pathPrefix, pathPrefix + "/*"}},
			},
			"handle": []map[string]interface{}{
				{
					"handler": "rewrite",
					"strip_path_prefix": pathPrefix,
				},
				{
					"handler": "reverse_proxy",
					"upstreams": []map[string]interface{}{
						{"dial": target},
					},
				},
			},
		}
		routes = append(routes, route)
	}

	// Add a root route listing available pucks
	puckList := "Available pucks:\n"
	for name := range r.routes {
		puckList += fmt.Sprintf("  /%s\n", name)
	}
	if len(r.routes) == 0 {
		puckList = "No pucks found. Create one with: puck create <name>"
	}

	defaultRoute := map[string]interface{}{
		"handle": []map[string]interface{}{
			{
				"handler":     "static_response",
				"status_code": "200",
				"body":        puckList,
				"headers": map[string][]string{
					"Content-Type": {"text/plain; charset=utf-8"},
				},
			},
		},
	}
	routes = append(routes, defaultRoute)

	// Build server config - only use the configured port
	serverConfig := map[string]interface{}{
		"listen": []string{fmt.Sprintf(":%d", r.port)},
		"routes": routes,
	}

	// If tailnet is configured, add Tailscale listener for HTTPS
	if r.tailnet != "" {
		serverConfig["listen"] = []string{
			fmt.Sprintf(":%d", r.port),
			fmt.Sprintf("tailscale/:%d", 443), // HTTPS on Tailscale
		}
	}

	return map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"puck": serverConfig,
				},
			},
		},
	}
}
