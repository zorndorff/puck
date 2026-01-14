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
	"github.com/sandwich-labs/puck/internal/podman"
	"github.com/sandwich-labs/puck/internal/sprite"
	"github.com/sandwich-labs/puck/internal/store"
)

// Daemon is the main daemon server
type Daemon struct {
	cfg     *config.Config
	podman  *podman.Client
	store   *store.DB
	manager *sprite.Manager

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

	mgr := sprite.NewManager(cfg, pc, db)

	return &Daemon{
		cfg:     cfg,
		podman:  pc,
		store:   db,
		manager: mgr,
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
	if d.listener != nil {
		d.listener.Close()
	}
	if d.store != nil {
		d.store.Close()
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
	case "ping":
		return Response{Success: true}
	default:
		return Response{Success: false, Error: fmt.Sprintf("unknown action: %s", req.Action)}
	}
}

func (d *Daemon) handleCreate(ctx context.Context, data json.RawMessage) Response {
	var opts sprite.CreateOptions
	if err := json.Unmarshal(data, &opts); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	s, err := d.manager.Create(ctx, opts)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	respData, _ := json.Marshal(s)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleList(ctx context.Context) Response {
	sprites, err := d.manager.List(ctx)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	respData, _ := json.Marshal(sprites)
	return Response{Success: true, Data: respData}
}

func (d *Daemon) handleGet(ctx context.Context, data json.RawMessage) Response {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	s, err := d.manager.Get(ctx, params.Name)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	respData, _ := json.Marshal(s)
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

	return Response{Success: true}
}

// Manager returns the sprite manager (for console command which needs direct access)
func (d *Daemon) Manager() *sprite.Manager {
	return d.manager
}
