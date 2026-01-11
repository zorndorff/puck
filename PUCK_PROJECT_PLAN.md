# Project Puck: Homelab Sprites Operator

## 1. Executive Summary
**Puck** is a Kubernetes-native system designed to bring the "Sprite" concept—ephemeral-feeling but persistent, auto-sleeping Linux environments—to homelab infrastructure. It allows users to instantly summon, use, and ignore development environments without resource waste or data loss.

## 2. Domain Driven Design (DDD)

### 2.1 Ubiquitous Language
*   **Sprite:** The core aggregate. A persistent, isolated development environment. It appears to the user as a single computer.
*   **Hibernation:** The state where a Sprite consumes zero compute resources (CPU/RAM) but retains all storage and state.
*   **Wake:** The transition from Hibernation to Active state.
*   **Session:** An active user interaction (CLI connection) with a Sprite.
*   **Keep-Alive:** A signal that prevents the Sprite from hibernating.

### 2.2 Bounded Contexts
*   **Orchestration Context (The Operator):** Responsible for the lifecycle of the underlying Kubernetes resources (StatefulSets, PVCs, Services). It translates "Sprite" intents into K8s reality.
*   **Interaction Context (The CLI):** Responsible for the user experience. It handles the "magic" of waking up a Sprite upon connection and presenting a clean shell.

## 3. System Architecture

### 3.1 Components
1.  **Puck Operator:** A Kubernetes Controller watching `Sprite` Custom Resources.
2.  **Puck CLI:** A Go-based command-line tool for users to interact with the system.
3.  **Kubernetes Cluster (k3s):** The runtime environment.

### 3.2 The `Sprite` Custom Resource Definition (CRD)
The `Sprite` CR is the "Source of Truth".

```yaml
apiVersion: puck.sandwich.labs/v1alpha1
kind: Sprite
metadata:
  name: dev-workspace
spec:
  # The base OS image (e.g., ubuntu:24.04, alpine)
  image: "ubuntu:24.04"
  # Storage request for persistence (e.g., "10Gi")
  storageSize: "10Gi"
  # Time in minutes to wait after last activity before hibernating
  idleTimeoutMinutes: 30
  # List of SSH public keys (optional, for direct SSH access)
  authorizedKeys: []
status:
  phase: "Active" | "Hibernating" | "Starting" | "Terminating"
  lastActiveTime: "2026-01-11T12:00:00Z"
  activeSessions: 0
  podName: "dev-workspace-0"
```

## 4. Functional Requirements

### 4.1 Lifecycle Management
*   **Create:** User defines a Sprite. Operator creates a `PersistentVolumeClaim` (PVC) and a `StatefulSet`.
*   **Hibernate (Scale to Zero):** If `activeSessions == 0` AND `time.Now() > lastActiveTime + idleTimeoutMinutes`, the Operator scales the StatefulSet replicas to 0.
*   **Wake:** When a user requests a connection, the system must scale the StatefulSet replicas to 1 and wait for readiness.
*   **Delete:** User deletes the Sprite. Operator cleans up the StatefulSet but **retains the PVC by default** (optional "hard delete" flag).

### 4.2 Persistence
*   Data must survive Hibernation and Pod recreation.
*   The `/home/sprite` directory (or equivalent) must be mounted to the PVC.

### 4.3 Connectivity
*   Primary access via `puck connect <name>`.
*   This command wraps `kubectl exec`.
*   It must handle the "Wake" flow automatically (blocking until the Pod is ready).

## 5. Implementation Plan

### Phase 1: Foundation (Scaffolding)
*   Initialize Go module.
*   Set up Kubebuilder/Controller-Runtime project structure.
*   Define `Sprite` API (Go structs).
*   Generate CRD manifests.

### Phase 2: The Operator (Core Logic)
*   Implement the **Reconciler Loop**:
    *   Ensure PVC exists.
    *   Ensure StatefulSet exists (replicas determined by state).
    *   Handle status updates (Ready/Pending).
*   Implement basic "Wake" logic (manual flag initially).

### Phase 3: The CLI (`puck`)
*   Implement `puck create`: Generates CR YAML and applies it.
*   Implement `puck list`: Shows Sprites and their status.
*   Implement `puck connect`:
    *   Checks status.
    *   If hibernating, patch CR to wake.
    *   Wait for Pod Running.
    *   Exec into shell.

### Phase 4: Automation (The "Magic")
*   Implement `idleTimeout` logic in the Operator.
*   **Challenge:** How to detect activity?
    *   *Solution A (Simple):* CLI updates `lastActiveTime` on connect/disconnect.
    *   *Solution B (Robust):* Sidecar container in the Pod monitoring TTYs or process list.
    *   *Decision:* Start with Solution A (CLI driven heartbeats).

## 6. Technical Stack
*   **Language:** Go (1.23+)
*   **K8s Framework:** Controller-Runtime
*   **CLI Framework:** Cobra + Bubble Tea (for pretty spinners/lists)
*   **Runtime:** k3s (Containerd)
