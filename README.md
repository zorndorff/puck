# Puck

**Persistent development containers that behave like tiny computers, not ephemeral sandboxes.**

![Puck Demo](demos/demo.gif)

Puck creates and manages stateful containers ("pucks") that persist across sessions. Unlike traditional containers designed for stateless workloads, pucks maintain their state, installed packages, and configurations—just like a real machine.

## Why Puck?

Traditional containers are ephemeral by design. Every restart means reinstalling packages, reconfiguring settings, and losing your work-in-progress. As [Fly.io put it](https://fly.io/blog/code-and-let-live/): *"The age of sandboxes is over. The time of the disposable computer has come."*

Puck brings that philosophy to your local machine:

- **Dev is Prod**: Your development environment persists and evolves with your project
- **Stateful by Default**: Pucks act like actual computers with durable storage
- **Checkpoint/Restore**: Freeze and resume puck state instantly with CRIU
- **HTTP Routing**: Access all pucks through a unified localhost endpoint
- **Agent-Friendly**: Perfect for AI coding agents that need persistent environments

## Features

- **Persistent Containers**: Pucks maintain state across restarts with mounted volumes for `/home`, `/etc`, and `/var`
- **Interactive Console**: Drop into any puck with `puck console`
- **HTTP Router**: Access pucks via `localhost:8080/<puck-name>/`
- **Checkpoint/Restore**: Save and restore complete container state including memory and TCP connections
- **Auto-naming**: Memorable adjective-noun names like `fuzzy-penguin` when you don't specify one
- **Rootless by Default**: Runs without elevated privileges using Podman

## Installation

### Prerequisites

- [Podman](https://podman.io/docs/installation) v5.0+
- Linux (macOS support planned via Podman Machine)

### From Source

```bash
# Clone the repository
git clone https://github.com/sandwich-labs/puck.git
cd puck

# Build (requires Go 1.21+)
go build -o puck ./cmd/puck
go build -o puckd ./cmd/puckd

# Or use Task
task build-all

# Install to your PATH
task install
```

## Quick Start

```bash
# Start the daemon (runs in background)
puck daemon start &

# Create your first puck
puck create myapp

# Get a shell inside
puck console myapp

# Install something - it persists!
dnf install -y nodejs
node --version

# Exit and return later
exit

# Your packages are still there
puck console myapp
node --version  # Still installed!
```

## Commands

### Puck Management

| Command | Description |
|---------|-------------|
| `puck create [name]` | Create a new puck |
| `puck list` | List all pucks |
| `puck console <name>` | Open interactive shell |
| `puck start <name>` | Start a stopped puck |
| `puck stop <name>` | Stop a running puck |
| `puck destroy <name>` | Delete a puck permanently |

### Daemon Management

| Command | Description |
|---------|-------------|
| `puck daemon start` | Start the puck daemon |
| `puck daemon status` | Check if daemon is running |

### Command Details

#### `puck create`

Create a new persistent puck container.

```bash
# Auto-generate a fun name
puck create

# Specify a name
puck create myapp

# Use a specific base image
puck create myapp --image ubuntu:22.04

# Map ports
puck create webserver --image nginx --port 80:80
```

**Flags:**
- `-i, --image <image>` - Base image (default: `fedora:latest`)
- `-p, --port <host:container>` - Port mapping

#### `puck console`

![Console Demo](demos/console-demo.gif)

Open an interactive shell session inside a puck.

```bash
puck console myapp

# Use a different shell
puck console myapp --shell /bin/zsh
```

**Flags:**
- `-s, --shell <path>` - Shell to use (default: `/bin/bash`)

#### `puck destroy`

![Lifecycle Demo](demos/lifecycle-demo.gif)

Remove a puck and its associated data.

```bash
# Destroy a specific puck
puck destroy myapp

# Force destroy without confirmation
puck destroy myapp --force

# Destroy all pucks
puck destroy --all
```

**Flags:**
- `-f, --force` - Skip confirmation
- `--all` - Destroy all pucks

## HTTP Routing

Puck includes a built-in HTTP router (powered by Caddy) that provides unified access to all pucks:

```
http://localhost:8080/           → Lists all pucks
http://localhost:8080/myapp/     → Routes to myapp container
http://localhost:8080/webserver/ → Routes to webserver container
```

The router automatically strips the puck name prefix and forwards requests to the container's mapped port.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      puck CLI                           │
│  (create, list, console, start, stop, destroy)          │
└─────────────────────┬───────────────────────────────────┘
                      │ Unix Socket
┌─────────────────────▼───────────────────────────────────┐
│                    puckd Daemon                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │    Puck     │  │    HTTP     │  │     SQLite      │  │
│  │   Manager   │  │   Router    │  │     Store       │  │
│  └──────┬──────┘  └─────────────┘  └─────────────────┘  │
└─────────┼───────────────────────────────────────────────┘
          │ Podman Bindings
┌─────────▼───────────────────────────────────────────────┐
│                    Podman Engine                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │   Puck   │  │   Puck   │  │   Puck   │   ...        │
│  │  (myapp) │  │  (web)   │  │  (api)   │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
```

### Components

- **puck CLI**: User-facing commands that communicate with the daemon
- **puckd Daemon**: Background service managing puck lifecycle and HTTP routing
- **Puck Manager**: Handles container creation, volume mounting, and state management
- **HTTP Router**: Caddy-based reverse proxy for accessing pucks
- **SQLite Store**: Metadata persistence for pucks and snapshots
- **Podman Engine**: Container runtime (rootless by default)

## Configuration

Puck looks for configuration in the following locations:

1. `~/.config/puck/config.yaml`
2. Environment variables with `PUCK_` prefix

### Example Configuration

```yaml
# ~/.config/puck/config.yaml

# Base image for new pucks
default_image: fedora:latest

# HTTP router port
router_port: 8080

# Auto-stop idle pucks after this duration
idle_timeout: 15m

# Data directory for pucks and snapshots
data_dir: ~/.local/share/puck
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PUCK_DATA_DIR` | Data storage location | `~/.local/share/puck` |
| `PUCK_DEFAULT_IMAGE` | Default container image | `fedora:latest` |
| `PUCK_ROUTER_PORT` | HTTP router port | `8080` |

## Data Storage

Puck stores data in `~/.local/share/puck/`:

```
~/.local/share/puck/
├── puck.db              # SQLite database
├── puckd.sock           # Daemon Unix socket
├── pucks/
│   └── myapp/           # Per-puck volumes
│       ├── home/        # Persistent home directory
│       ├── etc/         # System configuration
│       └── var/         # Variable data
└── snapshots/           # CRIU checkpoint archives
```

## Snapshots (Experimental)

Puck supports CRIU-based checkpointing to freeze and restore complete container state:

```bash
# Create a snapshot
puck snapshot create myapp --name before-update

# Restore from snapshot
puck snapshot restore myapp --from before-update

# List snapshots
puck snapshot list myapp
```

> **Note**: Requires CRIU support in your Podman installation. Not available on all platforms.

## Development

```bash
# Install dependencies
go mod download

# Run tests
task test

# Run linter
task lint

# Build all binaries
task build-all

# Development mode (rebuilds on change)
task dev
```

### Generating Demo GIFs

Demo GIFs are created using [VHS](https://github.com/charmbracelet/vhs):

```bash
# Install VHS
go install github.com/charmbracelet/vhs@latest

# Generate all demos
for tape in demos/*.tape; do
  vhs "$tape"
done
```

## Roadmap

- [ ] macOS support via Podman Machine
- [ ] Windows support via WSL2/Podman
- [ ] Tailscale Funnel integration for public URLs
- [ ] Wake-on-request for idle pucks
- [ ] Remote puck synchronization
- [ ] GUI dashboard

## Inspiration

Puck is directly inspired by [Fly.io's Sprites](https://fly.io/blog/code-and-let-live/)—their vision of "disposable cloud computers" that combine instant provisioning with true persistence. We wanted to bring that same philosophy to local development.

### The Problem with Ephemeral Sandboxes

As Fly.io's Thomas Ptacek argues:

> "The state of the art in agent isolation is a read-only sandbox... ephemeral sandboxes are obsolete. Stop killing your sandboxes every time you use them."

The industry has been forcing agents (and developers) into stateless containers designed for horizontal-scaling production workloads. But that's not how development actually works:

> "Claude isn't a pro developer. Claude is a hyper-productive five-year-old savant... If you force an agent to, it'll work around containerization and do work. But you're not helping the agent in any way by doing that. They don't want containers. They don't want 'sandboxes'. They want computers."

### What Makes a Computer?

Fly.io's definition is simple and powerful:

> - A computer doesn't necessarily vanish after a single job is completed, and
> - it has durable storage.
>
> Since current agent sandboxes have neither of these, I can stop the definition right there.

### Dev is Prod, Prod is Dev

The most compelling idea from Sprites is that for many applications—especially personal tools and AI-assisted development—the distinction between development and production environments is artificial:

> "For this app, dev is prod, prod is dev."

Puck embraces this philosophy locally. Your sprites persist, evolve with your projects, and maintain state across sessions—just like a real machine would.

### Credits

- **[Fly.io](https://fly.io)** for the Sprites concept and the ["Code And Let Live"](https://fly.io/blog/code-and-let-live/) manifesto that inspired this project
- The Fly.io team for articulating why ephemeral sandboxes are holding back both human developers and AI agents
- The full Fly.io blog post is preserved in [STARTING.md](STARTING.md)

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read the [contribution guidelines](CONTRIBUTING.md) first.

---

<p align="center">
  Built with care by <a href="https://github.com/sandwich-labs">Sandwich Labs</a>
</p>
