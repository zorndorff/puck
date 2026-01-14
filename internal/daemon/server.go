package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/sandwich-labs/puck/internal/config"
	"github.com/sandwich-labs/puck/internal/network"
	"github.com/sandwich-labs/puck/internal/podman"
	"github.com/sandwich-labs/puck/internal/puck"
	"github.com/sandwich-labs/puck/internal/store"
)

// Daemon is the main daemon server
type Daemon struct {
	cfg     *config.Config
	podman  *podman.Client
	store   *store.DB
	manager *puck.Manager
	router  *network.Router

	listener net.Listener
	mu       sync.RWMutex
	running  bool
}

// New creates a new daemon instance
func New() (*Daemon, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	ctx := context.Background()
	pc, err := podman.NewClient(ctx, cfg.PodmanSocket)
	if err != nil {
		return nil, fmt.Errorf("connecting to podman: %w", err)
	}

	db, err := store.Open(cfg.DatabasePath())
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	mgr := puck.NewManager(cfg, pc, db)

	// Create router for HTTP routing
	router := network.NewRouter(cfg.RouterPort, cfg.RouterDomain)
	if cfg.Tailnet != "" {
		router.SetTailnet(cfg.Tailnet)
	}

	return &Daemon{
		cfg:     cfg,
		podman:  pc,
		store:   db,
		manager: mgr,
		router:  router,
	}, nil
}

// Run starts the daemon
func (d *Daemon) Run(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("daemon already running")
	}
	d.running = true
	d.mu.Unlock()

	// Ensure socket directory exists
	socketDir := filepath.Dir(d.cfg.DaemonSocket)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	// Remove existing socket
	os.Remove(d.cfg.DaemonSocket)

	// Create Unix socket listener
	ln, err := net.Listen("unix", d.cfg.DaemonSocket)
	if err != nil {
		return fmt.Errorf("creating socket: %w", err)
	}
	d.listener = ln

	log.Info("Daemon listening", "socket", d.cfg.DaemonSocket)

	// Start HTTP router
	if err := d.router.Start(); err != nil {
		log.Warn("Failed to start HTTP router", "error", err)
		// Continue without router - it's not critical
	} else {
		log.Info("HTTP router started", "port", d.cfg.RouterPort, "domain", d.cfg.RouterDomain)
	}

	// Sync existing pucks to router
	d.syncRoutesToRouter(ctx)

	// Accept connections
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return nil
				default:
					log.Error("Accept error", "error", err)
					continue
				}
			}
			go d.handleConnection(ctx, conn)
		}
	}
}

// Shutdown stops the daemon
func (d *Daemon) Shutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.running = false
	if d.router != nil {
		d.router.Stop()
	}
	if d.listener != nil {
		d.listener.Close()
	}
	if d.store != nil {
		d.store.Close()
	}
}

// syncRoutesToRouter adds routes for all running pucks
func (d *Daemon) syncRoutesToRouter(ctx context.Context) {
	pucks, err := d.manager.List(ctx)
	if err != nil {
		log.Warn("Failed to list pucks for router sync", "error", err)
		return
	}

	for _, p := range pucks {
		if p.Status == store.StatusRunning && p.HostPort > 0 {
			// Route to localhost with the mapped host port
			if err := d.router.AddRoute(p.Name, "127.0.0.1", p.HostPort); err != nil {
				log.Warn("Failed to add route", "puck", p.Name, "error", err)
			}
		}
	}
}

// Request represents a daemon request
type Request struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// Response represents a daemon response
type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func (d *Daemon) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Success: false, Error: err.Error()})
		return
	}

	resp := d.handleRequest(ctx, &req)
	encoder.Encode(resp)
}

func (d *Daemon) handleRequest(ctx context.Context, req *Request) Response {
	switch req.Action {
	case "create":
		return d.handleCreate(ctx, req.Data)
	case "list":
		return d.handleList(ctx)
	case "get":
		return d.handleGet(ctx, req.Data)
	case "start":
		return d.handleStart(ctx, req.Data)
	case "stop":
		return d.handleStop(ctx, req.Data)
	case "destroy":
		return d.handleDestroy(ctx, req.Data)
	case "destroy-all":
		return d.handleDestroyAll(ctx, req.Data)
	case "snapshot-create":
		return d.handleSnapshotCreate(ctx, req.Data)
	case "snapshot-restore":
		return d.handleSnapshotRestore(ctx, req.Data)
	case "snapshot-list":
		return d.handleSnapshotList(ctx, req.Data)
	case "snapshot-delete":
		return d.handleSnapshotDelete(ctx, req.Data)
	case "ping":
		return Response{Success: true}
	default:
		return Response{Success: false, Error: fmt.Sprintf("unknown action: %s", req.Action)}
	}
}

func (d *Daemon) handleCreate(ctx context.Context, data json.RawMessage) Response {
	var opts puck.CreateOptions
	if err := json.Unmarshal(data, &opts); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	p, err := d.manager.Create(ctx, opts)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Add route for the new puck using its host port
	if p.HostPort > 0 {
		if err := d.router.AddRoute(p.Name, "127.0.0.1", p.HostPort); err != nil {
			log.Warn("Failed to add route for puck", "name", p.Name, "error", err)
		}
	}

	respData, _ := json.Marshal(p)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleList(ctx context.Context) Response {
	pucks, err := d.manager.List(ctx)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	respData, _ := json.Marshal(pucks)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleGet(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	p, err := d.manager.Get(ctx, params.Name)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	respData, _ := json.Marshal(p)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleStart(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.Start(ctx, params.Name); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Add route for started puck using its host port
	p, err := d.manager.Get(ctx, params.Name)
	if err == nil && p.HostPort > 0 {
		if err := d.router.AddRoute(p.Name, "127.0.0.1", p.HostPort); err != nil {
			log.Warn("Failed to add route for puck", "name", p.Name, "error", err)
		}
	}

	return Response{Success: true}
}

func (d *Daemon) handleStop(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.Stop(ctx, params.Name); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Remove route for stopped puck
	if err := d.router.RemoveRoute(params.Name); err != nil {
		log.Warn("Failed to remove route for puck", "name", params.Name, "error", err)
	}

	return Response{Success: true}
}

func (d *Daemon) handleDestroy(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.Destroy(ctx, params.Name, params.Force); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Remove route for destroyed puck
	if err := d.router.RemoveRoute(params.Name); err != nil {
		log.Warn("Failed to remove route for puck", "name", params.Name, "error", err)
	}

	return Response{Success: true}
}

func (d *Daemon) handleDestroyAll(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		Force bool `json:"force"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	destroyed, err := d.manager.DestroyAll(ctx, params.Force)

	// Remove routes for all destroyed pucks
	for _, name := range destroyed {
		if err := d.router.RemoveRoute(name); err != nil {
			log.Warn("Failed to remove route for puck", "name", name, "error", err)
		}
	}

	if err != nil {
		// Return partial success with list of destroyed pucks
		respData, _ := json.Marshal(destroyed)
		return Response{Success: false, Error: err.Error(), Data: respData}
	}

	respData, _ := json.Marshal(destroyed)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleSnapshotCreate(ctx context.Context, data json.RawMessage) Response {
	var opts puck.SnapshotCreateOptions
	if err := json.Unmarshal(data, &opts); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	snapshot, err := d.manager.CreateSnapshot(ctx, opts)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Remove route if not leaving running (puck is checkpointed)
	if !opts.LeaveRunning {
		if err := d.router.RemoveRoute(opts.PuckName); err != nil {
			log.Warn("Failed to remove route for checkpointed puck", "name", opts.PuckName, "error", err)
		}
	}

	respData, _ := json.Marshal(snapshot)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleSnapshotRestore(ctx context.Context, data json.RawMessage) Response {
	var opts puck.SnapshotRestoreOptions
	if err := json.Unmarshal(data, &opts); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.RestoreSnapshot(ctx, opts); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Re-add route for restored puck
	p, err := d.manager.Get(ctx, opts.PuckName)
	if err == nil && p.HostPort > 0 {
		if err := d.router.AddRoute(p.Name, "127.0.0.1", p.HostPort); err != nil {
			log.Warn("Failed to add route for restored puck", "name", p.Name, "error", err)
		}
	}

	return Response{Success: true}
}

func (d *Daemon) handleSnapshotList(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		PuckName string `json:"puck_name"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	snapshots, err := d.manager.ListSnapshots(ctx, params.PuckName)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	respData, _ := json.Marshal(snapshots)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleSnapshotDelete(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		PuckName     string `json:"puck_name"`
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.DeleteSnapshot(ctx, params.PuckName, params.SnapshotName); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	return Response{Success: true}
}

// Manager returns the puck manager (for console command which needs direct access)
func (d *Daemon) Manager() *puck.Manager {
	return d.manager
}
