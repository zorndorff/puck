package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Run("creates router with specified port", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		assert.Equal(t, 8080, router.port)
		assert.Equal(t, "localhost", router.domain)
		assert.NotNil(t, router.routes)
		assert.False(t, router.running)
	})

	t.Run("uses localhost as default domain", func(t *testing.T) {
		router := NewRouter(9000, "")
		assert.Equal(t, "localhost", router.domain)
	})

	t.Run("initializes empty routes map", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		assert.Empty(t, router.routes)
	})
}

func TestSetTailnet(t *testing.T) {
	t.Run("sets tailnet name", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		router.SetTailnet("my-tailnet")
		assert.Equal(t, "my-tailnet", router.tailnet)
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		router.SetTailnet("first")
		router.SetTailnet("second")
		assert.Equal(t, "second", router.tailnet)
	})
}

func TestGetRoutes(t *testing.T) {
	t.Run("returns empty map when no routes", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		routes := router.GetRoutes()
		assert.Empty(t, routes)
	})

	t.Run("returns all routes as IP:port strings", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		router.routes["puck1"] = routeInfo{IP: "10.88.0.2", Port: 80}
		router.routes["puck2"] = routeInfo{IP: "10.88.0.3", Port: 8080}

		routes := router.GetRoutes()
		assert.Len(t, routes, 2)
		assert.Equal(t, "10.88.0.2:80", routes["puck1"])
		assert.Equal(t, "10.88.0.3:8080", routes["puck2"])
	})
}

// TestBuildConfig tests the config generation logic
// Note: We can't easily test Start/Stop/AddRoute/RemoveRoute because they
// depend on the actual Caddy server being available. These are tested
// by verifying the config structure instead.
func TestBuildConfig(t *testing.T) {
	t.Run("generates valid config structure", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		config := router.buildConfig()

		// Verify top-level structure
		apps, ok := config["apps"].(map[string]interface{})
		require.True(t, ok, "should have apps key")

		http, ok := apps["http"].(map[string]interface{})
		require.True(t, ok, "should have http app")

		servers, ok := http["servers"].(map[string]interface{})
		require.True(t, ok, "should have servers")

		puckServer, ok := servers["puck"].(map[string]interface{})
		require.True(t, ok, "should have puck server")

		// Verify listen address
		listen, ok := puckServer["listen"].([]string)
		require.True(t, ok, "should have listen array")
		assert.Contains(t, listen, ":8080")
	})

	t.Run("includes route for each puck", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		router.routes["web-app"] = routeInfo{IP: "10.88.0.2", Port: 80}

		config := router.buildConfig()

		apps := config["apps"].(map[string]interface{})
		http := apps["http"].(map[string]interface{})
		servers := http["servers"].(map[string]interface{})
		puckServer := servers["puck"].(map[string]interface{})
		routes := puckServer["routes"].([]map[string]interface{})

		// Should have at least 2 routes: the puck route and default route
		assert.GreaterOrEqual(t, len(routes), 2)

		// Find the puck route
		var foundPuckRoute bool
		for _, route := range routes {
			if match, ok := route["match"].([]map[string]interface{}); ok {
				for _, m := range match {
					if paths, ok := m["path"].([]string); ok {
						for _, p := range paths {
							if p == "/web-app" || p == "/web-app/*" {
								foundPuckRoute = true
								break
							}
						}
					}
				}
			}
		}
		assert.True(t, foundPuckRoute, "should have route for web-app")
	})

	t.Run("includes default root route", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		config := router.buildConfig()

		apps := config["apps"].(map[string]interface{})
		http := apps["http"].(map[string]interface{})
		servers := http["servers"].(map[string]interface{})
		puckServer := servers["puck"].(map[string]interface{})
		routes := puckServer["routes"].([]map[string]interface{})

		// Last route should be the default handler
		lastRoute := routes[len(routes)-1]
		handlers, ok := lastRoute["handle"].([]map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, handlers)

		// Should have static_response handler
		var hasStaticResponse bool
		for _, h := range handlers {
			if h["handler"] == "static_response" {
				hasStaticResponse = true
			}
		}
		assert.True(t, hasStaticResponse, "should have static_response handler for default route")
	})

	t.Run("default route shows no pucks message when empty", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		config := router.buildConfig()

		apps := config["apps"].(map[string]interface{})
		http := apps["http"].(map[string]interface{})
		servers := http["servers"].(map[string]interface{})
		puckServer := servers["puck"].(map[string]interface{})
		routes := puckServer["routes"].([]map[string]interface{})

		// Find static_response body
		lastRoute := routes[len(routes)-1]
		handlers := lastRoute["handle"].([]map[string]interface{})

		var body string
		for _, h := range handlers {
			if h["handler"] == "static_response" {
				body = h["body"].(string)
			}
		}
		assert.Contains(t, body, "No pucks found")
	})

	t.Run("adds tailscale listener when tailnet set", func(t *testing.T) {
		router := NewRouter(8080, "localhost")
		router.SetTailnet("my-tailnet")
		config := router.buildConfig()

		apps := config["apps"].(map[string]interface{})
		http := apps["http"].(map[string]interface{})
		servers := http["servers"].(map[string]interface{})
		puckServer := servers["puck"].(map[string]interface{})
		listen := puckServer["listen"].([]string)

		// Should have both regular and tailscale listeners
		assert.Len(t, listen, 2)
		assert.Contains(t, listen, ":8080")
		assert.Contains(t, listen, "tailscale/:443")
	})
}

func TestRouteInfo(t *testing.T) {
	t.Run("stores IP and port", func(t *testing.T) {
		info := routeInfo{IP: "192.168.1.1", Port: 8000}
		assert.Equal(t, "192.168.1.1", info.IP)
		assert.Equal(t, 8000, info.Port)
	})
}

// Note: Start, Stop, AddRoute, and RemoveRoute methods require
// the actual Caddy server to be running. They are better tested
// in integration tests with the real Caddy binary.
// The buildConfig tests above verify the configuration logic.
