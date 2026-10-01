<div align="center">

<img src="build/appicon.png" width="128" alt="Kapture">

# Kapture

**Kubernetes Application & Pod Tracking and Unified Resource Explorer**

Autonomous, ultra-lightweight, and persistent Kubernetes container log manager. Retains days of container logs locally without burdening cluster resources or requiring heavy stacks (ELK/Loki), featuring a responsive embedded web dashboard and instant reset controls.

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-CRI--Native-326CE5?style=flat&logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Storage](https://img.shields.io/badge/Engine-BadgerDB%20v4-7952B3?style=flat)](https://github.com/dgraph-io/badger)
[![Binary Size](https://img.shields.io/badge/Binary-~13%20MB-success?style=flat)](#)
[![Zero External DB](https://img.shields.io/badge/Dependencies-Zero-brightgreen?style=flat)](#)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

<p align="center">
  <b>English</b> •
  <a href="README.id.md">Bahasa Indonesia</a>
</p>

</div>

---

## Quick Start

### Helm
For a quick evaluation or production deployment, install Kapture from the GitHub OCI registry into a dedicated namespace:

```bash
helm install kapture oci://ghcr.io/aribrilliantsyah/charts/kapture \
  --version 0.0.1 \
  --namespace kapture \
  --create-namespace
```

Forward the service to your local machine:

```bash
kubectl port-forward --namespace kapture svc/kapture 19488:19488
```

Open **[`http://localhost:19488`](http://localhost:19488)** in your browser, create the first administrator, and follow the 2FA setup wizard.

> 💡 **Important:** The default chart values are suitable for evaluation and development. Before using Kapture in production, configure log retention (`agent.storage.retention`), disk capacity limit (`agent.storage.maxDisk`), timezone (`global.timezone`), and enable persistent storage (PVC) for the aggregator on multi-node clusters.

---

### Other Installation Options

#### Docker
To run an isolated aggregator container for exploration or testing:

```bash
mkdir -p data
docker run -d --name kapture \
  -p 19488:19488 \
  -v "$(pwd)/data:/data/kapture" \
  -e KAPTURE_TIMEZONE=Asia/Jakarta \
  ghcr.io/aribrilliantsyah/kapture:v0.0.1 --mode=aggregator
```

#### Kubernetes Manifest
A standalone manifest intended for evaluation, ready to apply directly from GitHub:

```bash
kubectl apply -f https://raw.githubusercontent.com/aribrilliantsyah/kapture/main/deploy/install.yaml
kubectl port-forward --namespace kapture svc/kapture 19488:19488
```

#### Build from Source
Building Kapture from source requires Go 1.22+ and Make:

```bash
git clone https://github.com/aribrilliantsyah/kapture.git
cd kapture
make build
./bin/kapture --mode=aggregator
```

---

## Why Kapture

During a 2:00 AM production incident investigation in a Kubernetes cluster, this almost always happens:

```bash
$ kubectl logs api-server-7f8b9c-x2k1p
# Only loads the last few lines...

$ kubectl logs api-server-7f8b9c-x2k1p --previous
# Error from server (BadRequest): previous terminated container not found...
```

1. **Restarted or terminated pod logs vanish immediately.**
2. **Kubelet's default log rotation** (typically 10MB × 5 files) rapidly overwrites history from earlier in the week.
3. **Common solutions (ELK Stack or Loki + Grafana)** require 2–4 GB RAM per node, intricate configurations, and constantly strain cluster compute resources.

**Kapture** was engineered to solve this dilemma with minimal footprint:
- **Direct local file tailing** (`/var/log/containers/*.log`) already written by kubelet via kernel *inotify*. Zero API call overhead to the Kubernetes API server.
- **Date-partitioned storage** in an embedded local engine (BadgerDB v4) with Snappy compression.
- **Lightweight & silent in the background** — consumes only ~64 MB memory and <1% idle CPU.
- **Complete disk control** — databases can be purged by date, by namespace, or reset completely with a single click in the dashboard without pod restarts.

---

## Key Features

- **100% Service & Workload Agnostic** — Captures logs from any workload without per-service configuration: Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, Init Containers, Sidecars (Envoy/Istio), and custom CRD operators.
- **Replica-Aware & Smart Grouping** — Intelligently parses Kubernetes pod naming conventions. View logs across 5 `api-server` replicas simultaneously (merged & time-aligned) or isolate individual pods.
- **Date-First Storage Architecture** — Indexed by date prefix (`YYYY-MM-DD`). Instant query performance whether inspecting incidents from yesterday, 3 days ago, or last week.
- **Instant Purge & Reset Controls** — Configurable auto-retention (default: retain until disk cap), hard disk limits, and a one-click **Reset All Logs** action in the web UI.
- **Embedded Web Dashboard (Single-Binary)** — Modern dark-mode interface with live-tail streaming, autocomplete filters, and stack-trace expansion. No Node.js or separate web server needed.
- **Multi-Dimensional Analytics Filtering** — Filter records across:
  - Date & Time Ranges
  - Namespaces & Workloads
  - Pods & Containers
  - Severity Levels (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`)
  - Full-Text & Regular Expressions (`/timeout.*after \d+ms/`)
- **Universal 2FA Authentication (TOTP)** — Enterprise-grade security with Time-based One-Time Passwords (RFC 6238). First-run wizard provides a QR code for Google Authenticator, Authy, or password managers. Subsequent sessions require 6-digit OTP verification.
- **Multi-User Management (Admin & Operator)** — Multi-account support with two roles: **Admin** (full access + user management) and **Operator** (read-only administration, full dashboard access). Admins can self-recover passwords or 2FA via security questions.
- **Backup & Restore** — Download logs across all nodes as a single `.tar.gz` archive (optionally bounded by date ranges), then restore to the same or a different Kapture instance (e.g. locally on your laptop for offline debugging).
- **ANSI Color Terminal Rendering** — Logs containing ANSI escape codes (e.g. Spring Boot) are rendered in rich colors rather than broken characters; `ERROR` and `WARN` severity detection remains accurate.
- **Quick Export** — Download filtered queries instantly in **JSON** or **CSV** formats for auditing and incident reports.
- **WebSocket Live Tail, Histogram & Compare View** — Real-time log streaming (agent → aggregator → browser), clickable volume histogram bars to zoom into time windows, and side-by-side replica comparisons with synchronized timestamps.
- **Workload-Centric History** — Agents resolve pod `ownerReferences` (Pod → ReplicaSet → Deployment, Job → CronJob) via the Kubernetes API. Rollouts produce new pod names, but logs stay categorized under the parent Deployment.
- **Local Timezone Support** — With `KAPTURE_TIMEZONE=Asia/Jakarta`, logs partition by local calendar days and dashboard timestamps display in local time regardless of browser timezone.
- **Recap Analytics Dashboard** — 14-day volume histograms per level, top error sources, busiest workloads, and latest errors without full log scans.
- **Search Query Syntax** — `timeout database` (AND), `error OR warning`, `error -healthcheck`, `"connection refused"`, `/failed.*\d+ retries/`, and field filters (`ns:`, `workload:`, `pod:`, `c:`, `level:`).
- **Zero External Dependencies** — Single static Go binary (~13 MB). No Elasticsearch, Postgres, Redis, or Fluentd required.

---

## Architecture: How Kapture Works

Kapture utilizes a **Distributed-Local Storage** architecture. Logs are never shipped continuously over the network to a central database; they remain stored locally on the worker node where the container runs. The Aggregator component functions solely as a *query router* when you interact with the dashboard.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             KUBERNETES CLUSTER                              │
│                                                                             │
│  ┌─ Worker Node 1 ───────────────┐        ┌─ Worker Node 2 ───────────────┐ │
│  │                               │        │                               │ │
│  │  Pod A (app)   Pod B (db)     │        │  Pod C (app)   Pod D (worker) │ │
│  │     │              │          │        │     │              │          │ │
│  │     ▼              ▼          │        │     ▼              ▼          │ │
│  │  /var/log/containers/*.log    │        │  /var/log/containers/*.log    │ │
│  │           │ (inotify)         │        │           │ (inotify)         │ │
│  │           ▼                   │        │           ▼                   │ │
│  │  ┌─────────────────────────┐  │        │  ┌─────────────────────────┐  │ │
│  │  │  Kapture Agent          │  │        │  │  Kapture Agent          │  │ │
│  │  │  (DaemonSet Pod)        │  │        │  │  (DaemonSet Pod)        │  │ │
│  │  │  • Tailer & CRI Parser  │  │        │  │  • Tailer & CRI Parser  │  │ │
│  │  │  • BadgerDB (node-local)│  │        │  │  • BadgerDB (node-local)│  │ │
│  │  └────────────┬────────────┘  │        │  └────────────┬────────────┘  │ │
│  └───────────────┼───────────────┘        └───────────────┼───────────────┘ │
│                  │                                        │                 │
│                  └───────────────────┬────────────────────┘                 │
│                                      │ (Internal HTTP)                      │
│                                      ▼                                      │
│                      ┌─────────────────────────────────┐                    │
│                      │  Kapture Aggregator             │                    │
│                      │  (Deployment, 1 Pod)            │                    │
│                      │  • Auto-discovers agents        │                    │
│                      │  • Fan-out query & merge-sort   │                    │
│                      │  • Serves Web Dashboard & API   │                    │
│                      └────────────────┬────────────────┘                    │
│                                       │                                     │
└───────────────────────────────────────┼─────────────────────────────────────┘
                                        │ HTTP :19488
                                        ▼
                            Browser / Web Dashboard
```

### 2 Roles in 1 Binary:

| Role | Deployment Pattern | Location | Primary Responsibility |
|---|---|---|---|
| **`agent`** | **DaemonSet** (1 pod per node) | **Every node** (Control Plane & Workers) | Watches `/var/log/containers/*.log`, parses metadata, and writes batches to the node-local BadgerDB. |
| **`aggregator`** | **Deployment** (1 replica) | Any worker node (scheduled by K8s) | Fans queries out to all agents, merge-sorts results chronologically, and serves the web dashboard & API. |

---

## Deploy to Kubernetes

Kapture offers flexible deployment methods tailored to your infrastructure needs:

1. **Helm Chart (OCI / GHCR)** — Recommended for production and simplified configuration management (`values.yaml`).
2. **Standalone Manifest** (`deploy/install.yaml`) — Instant evaluation via `kubectl apply -f https://raw.githubusercontent.com/...`.
3. **Modular Manifests** (`deploy/*.yaml`) — For GitOps pipelines (ArgoCD/Flux) and manual customization.

### Method 1: Using Helm (Recommended)

Install directly from the GitHub Container Registry without cloning the repository:

```bash
# Public OCI install
helm install kapture oci://ghcr.io/aribrilliantsyah/charts/kapture \
  --version 0.0.1 \
  --namespace kapture \
  --create-namespace
```

Or install using the local chart in this repository:

```bash
# Install from local charts/kapture directory
helm install kapture ./charts/kapture \
  --namespace kapture \
  --create-namespace \
  --set global.timezone="Asia/Jakarta"
```

---

### Method 2: Using the Standalone Manifest

```bash
kubectl apply -f https://raw.githubusercontent.com/aribrilliantsyah/kapture/main/deploy/install.yaml
```

---

### Method 3: Using Modular Manifests (deploy/)

If you need granular customization across individual resource files:

#### 1. Prepare Container Image (If Using a Private Registry)

Build and push the Kapture image to your container registry (Docker Hub, GitHub Packages, or private registry):

```bash
# 1. Build binary container
docker build -t your-registry/kapture:latest .

# 2. Push to registry
docker push your-registry/kapture:latest
```

> 📖 **Registry Guide:** See **[`docs/CONTAINER_REGISTRY_GUIDE.md`](docs/CONTAINER_REGISTRY_GUIDE.md)** for detailed instructions on pushing to **Docker Hub, GitHub Container Registry (GHCR), Harbor, AWS ECR, and GCP GAR**, including multi-arch builds (`linux/amd64` and `linux/arm64`).
>
> **Note:** The `agent` and `aggregator` share the **exact same container image**. Execution mode is determined by `--mode=agent` and `--mode=aggregator` arguments.

#### 2. Apply Manifests

The manifests in `deploy/` are modularly organized:

| File | Resource Type | Description |
|---|---|---|
| `00-namespace.yaml` | `Namespace` | `kapture` namespace isolation |
| `01-rbac.yaml` | `ClusterRole`, `Binding` | Permissions to read pod and namespace metadata |
| `02-secret.yaml` | `Secret` | Initial admin credentials |
| `03-agent-daemonset.yaml` | `DaemonSet` | Node-level log collectors |
| `04-aggregator-deployment.yaml` | `Deployment` | Web dashboard & query router |
| `05-service.yaml` | `Service` | Dashboard access endpoint & headless discovery |

Adjust the image names in `03-agent-daemonset.yaml` and `04-aggregator-deployment.yaml`, then apply:

```bash
kubectl apply -f deploy/
```

Verify running pods:

```bash
kubectl get pods -n kapture -o wide
```

Output should show **1 agent per node** and **1 aggregator pod**:
```text
NAME                                  READY   STATUS    NODE
kapture-agent-4j2x1                   1/1     Running   master-node
kapture-agent-9b8vc                   1/1     Running   worker-node-1
kapture-agent-z7q1a                   1/1     Running   worker-node-2
kapture-aggregator-5d8f9976f-w2k8m    1/1     Running   worker-node-1
```

---

## Dashboard Access & 2FA Setup

Port-forward the aggregator service to your workstation:

```bash
kubectl port-forward -n kapture svc/kapture 19488:19488
```

Open your browser at: **[`http://localhost:19488`](http://localhost:19488)**

Common port-forwarding patterns:

```bash
# Custom local port (e.g. 8080): open http://localhost:8080
kubectl port-forward -n kapture svc/kapture 8080:19488

# Run in background; stop with: kill %1 (or pkill -f "port-forward -n kapture")
kubectl port-forward -n kapture svc/kapture 19488:19488 >/dev/null 2>&1 &

# Bind to all interfaces for remote access across a private LAN
kubectl port-forward -n kapture --address 0.0.0.0 svc/kapture 19488:19488

# Target specific kubeconfig context
kubectl --context prod-cluster port-forward -n kapture svc/kapture 19488:19488

# Directly debug an agent (unauthenticated agent API on port 19489)
kubectl port-forward -n kapture pod/<agent-pod-name> 19489:19489
curl http://localhost:19489/healthz
```

#### 1. Initial Setup (First-Time Onboarding)
Upon accessing the dashboard for the first time, Kapture presents a 3-step setup wizard:
1. **Administrator Account:** Set your initial Username and Password. Passwords must be at least 8 characters and include lowercase, uppercase, digit, and symbol characters (e.g. `Qawsed#1477`).
2. **Recovery Question:** Choose a security question and provide an answer (case-insensitive) for password or 2FA reset recovery.
3. **Scan 2FA QR Code:** Scan the QR code using **Google Authenticator**, **Authy**, or your preferred authenticator app, then submit the 6-digit confirmation code.

Credentials, 2FA secrets, and hashed recovery answers (bcrypt) are persisted in the aggregator's volume (`/data/kapture/auth.json`).

#### 2. Ongoing Authentication
1. **Credentials:** Username and Password (bcrypt).
2. **2FA Code:** 6-digit code from your authenticator app.
3. **Active Sessions:** HMAC-signed `HttpOnly` session cookie valid for **30 days** (auto-renewed, survives aggregator restarts).
4. **Rate Limiting:** After 5 failed attempts within 10 minutes, the client IP and username are locked for 5 minutes. For automated scripts, `POST /api/v1/auth/login` accepts `{username, password, code}` and returns a Bearer token.

#### 3. Users & Roles
| Role | Permissions |
|---|---|
| **Admin** | Full access + **Users** management (add, edit roles, reset passwords, reset 2FA, delete accounts) |
| **Operator** | All dashboard & analytics features (excluding user administration) |

#### 4. Account Recovery
- **Admin Self-Recovery:** Click *Forgot your password or lost your phone?* on the login page. Answering the recovery question requires an additional factor (2FA code to reset password, or password to reset 2FA).
- **Operator Recovery:** Handled by an Admin through the **Users** menu.
- **Full Lockout Recovery:** Remove `auth.json` on the aggregator storage volume to trigger the initial setup wizard again (collected logs remain intact).

> 🔒 **Intranet Mode:** To run completely without authentication in closed private networks, set `KAPTURE_AUTH_ENABLED=false`.

---

## Running from Source & Local Development

You can test Kapture locally without a real Kubernetes cluster.

### Prerequisites
- **Go 1.26+**
- Linux or macOS

### 1. Generate Simulated Container Logs

Generate realistic CRI-formatted container logs (multi-replica pods, error/warn entries, multi-line traces):

```bash
make testdata
# Mock logs created in testdata/containers/
```

### 2. Run Agent in Terminal 1

```bash
make run-agent
```
The agent reads logs from `testdata/containers`, stores them in a local BadgerDB at `testdata/db`, and opens API port `:19489`.

### 3. Run Aggregator in Terminal 2

```bash
make run-aggregator
```
The aggregator starts on port `:19488`, connects to the local agent, and serves the web UI. Open **`http://localhost:19488`**.

### 4. Run Unit Tests

```bash
make test
```

---

## Storage Management & Log Retention

Kapture gives you full control over node disk usage:

```
┌─ Storage Management ────────────────────────────────────────┐
│  Disk Used:  ████░░░░░░░░░░░░░░░░  154 KB / 5.0 GB (3%)     │
│  Total Logs: 350 lines                                      │
│  Date Range: 2026-09-08 to 2026-09-10                       │
│                                                             │
│  [📅 Purge Before Date: [ 2026-09-09 ] [Delete]]            │
│  [⚠️ Reset All Logs]                                        │
└─────────────────────────────────────────────────────────────┘
```

1. **Retention (Optional)** — Default `0`: older dates are never dropped automatically. When `KAPTURE_STORAGE_RETENTION` is set (e.g. `30d`), entire calendar days past the cutoff are pruned every 5 minutes.
2. **Hard Disk Cap** — Parameter `max_disk` (default `5GB`) enforces a maximum local BadgerDB size per node. When approaching limits, the oldest calendar days are dropped automatically.
3. **Instant Manual Purge** — From the **Storage** view, purge logs before a chosen date or click **Reset All Logs** to empty databases immediately.
4. **Backup & Restore** — Under the *Backup and restore* card:
   - **Download** streams a compressed `kapture-backup-<timestamp>.tar.gz` containing node-level BadgerDB backups and `manifest.json`.
   - **Choose backup file** uploads the archive and appends log entries without duplicates. Missing nodes are routed to available agents, making it easy to restore production backups onto a local laptop instance (`make run-agent` + `make run-aggregator`).

---

## REST API Documentation

Kapture provides an intuitive REST API for scripting, curl commands, and automation:

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/logs` | Chronological logs (default: newest first). Params: `from`, `to` (RFC3339), `date`, `namespace`, `workload`, `workload_type`, `pod`, `container`, `level`, `search`, `regex`, `exclude_ns`, `limit`, `sort=asc\|desc`, `cursor` |
| `GET` | `/api/v1/logs/export?format=csv\|json` | Export query results (up to `max_results`) |
| `GET` | `/api/v1/stats/volume` | Volume histogram by level + top error workloads |
| `GET` | `/api/v1/stats/recap?from=YYYY-MM-DD&namespace=X` | Daily recap rollups per workload: line counts, bytes, pods observed |
| `WS` | `/api/v1/tail` | WebSocket live tail streaming |
| `GET` | `/api/v1/catalog` | Catalog of all containers observed (namespace, workload, pod, node, first/last seen) |
| `GET` | `/api/v1/dates` | List of dates with recorded logs |
| `GET` | `/api/v1/namespaces`, `/workloads`, `/pods` | List recorded namespaces, workloads, or pods |
| `GET` | `/api/v1/nodes` | Status of all discovered agents |
| `GET` | `/api/v1/storage` | Storage capacity, line counts, and node/date breakdowns |
| `GET` | `/api/v1/health` | Version + agent status (requires authentication) |
| `GET` | `/healthz` | Public liveness probe endpoint |
| `DELETE` | `/api/v1/logs?date=YYYY-MM-DD` | Delete logs for a specific calendar date |
| `DELETE` | `/api/v1/logs?before=YYYY-MM-DD` | Delete all logs prior to a date |
| `DELETE` | `/api/v1/logs?namespace=X[&workload=Y]` | Delete logs for a namespace / workload |
| `DELETE` | `/api/v1/logs/all` | **Reset All:** Wipe all log databases across all nodes |
| `GET` | `/api/v1/storage/backup?from=YYYY-MM-DD&to=YYYY-MM-DD` | Download `.tar.gz` backup archive across all nodes |
| `POST` | `/api/v1/storage/restore` | Restore: upload `.tar.gz` backup file |
| `GET` `POST` | `/api/v1/users` | *(Admin)* List / create user `{username, display_name, role, password}` |
| `PATCH` `DELETE` | `/api/v1/users/{id}` | *(Admin)* Update `{username, display_name, role}` / delete user |
| `POST` | `/api/v1/users/{id}/password`, `/api/v1/users/{id}/2fa/reset` | *(Admin)* Reset temporary password / reset 2FA |
| `PUT` | `/api/v1/users/{id}/recovery` | *(Admin)* Update another admin's recovery question |
| `GET` `PATCH` | `/api/v1/profile` | Current user profile (display name, username) |
| `POST` | `/api/v1/profile/password`, `/api/v1/profile/2fa` | Change password / display 2FA enrollment QR code |
| `PUT` | `/api/v1/profile/recovery` | *(Admin)* Update recovery question `{question, answer, password}` |

### Curl Examples:

```bash
# 1. Forward access to aggregator
kubectl port-forward -n kapture svc/kapture 19488:19488

# 2. Authenticate to obtain a session token
TOKEN=$(curl -s -X POST http://localhost:19488/api/v1/auth/login \
  -d '{"username":"admin","password":"<password>","code":"123456"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

# Fetch error logs for workload 'api-server' today
curl -H "Authorization: Bearer $TOKEN" "http://localhost:19488/api/v1/logs?workload=api-server&level=ERROR&limit=50"

# Purge logs older than 3 days
curl -H "Authorization: Bearer $TOKEN" -X DELETE "http://localhost:19488/api/v1/logs?before=2026-09-07"

# Cleanly wipe all logs across all nodes
curl -H "Authorization: Bearer $TOKEN" -X DELETE "http://localhost:19488/api/v1/logs/all"

# Backup all nodes to a local file
curl -H "Authorization: Bearer $TOKEN" -o kapture-backup.tar.gz "http://localhost:19488/api/v1/storage/backup?from=2026-09-01"
```

---

## Troubleshooting

| Symptom | Cause & Solution |
|---|---|
| Dashboard: *No agents discovered* | Aggregator cannot discover agent pods. Check `kubectl get pods -n kapture -l role=agent` and headless service `kapture-agents`. |
| Dashboard: *No logs collected yet* | Agents are running but not capturing logs. Check agent logs: `cannot open log file (is its symlink target mounted?)`. Nodes using Docker container runtimes require `/var/lib/docker/containers` mounted. |
| Aggregator keeps restarting | Outdated manifest using `/api/v1/health` (requires login) for liveness probes. Upgrade to `/healthz`. |
| Upgrading from legacy schema | Key schema changed to chronological sort. On startup, agents wipe outdated schemas and re-tail existing node log files. |

---

## Environment Variables

Settings are read from environment variables (`KAPTURE_*`, with legacy `LOG_CATCHER_*` aliases).

| Variable | Default | Description |
|---|---|---|
| `KAPTURE_MODE` | `agent` | Execution mode: `agent` or `aggregator` |
| `KAPTURE_LOG_PATH` | `/var/log/containers` | Container log directory on the host node |
| `KAPTURE_STORAGE_PATH`| `/data/kapture` | Local BadgerDB database directory |
| `KAPTURE_TIMEZONE` | `TZ` or `UTC` | IANA timezone (e.g. `Asia/Jakarta`) for date partitioning & UI timestamps |
| `KAPTURE_STORAGE_RETENTION` | `0` (indefinite) | Auto-retention duration (e.g. `30d`, `168h`). `0` disables auto-purge |
| `KAPTURE_STORAGE_MAX_DISK` | `5GB` | Max disk space threshold per node before emergency oldest-day pruning |
| `KAPTURE_AGENT_PORT` | `19489` | Internal HTTP API port for agent |
| `KAPTURE_DASHBOARD_PORT` | `19488` | Web dashboard & REST API port for aggregator |
| `KAPTURE_AUTH_ENABLED` | `true` | Enables login + 2FA (`false` disables auth) |
| `KAPTURE_DISCOVERY_METHOD` | `kubernetes` | Agent discovery method (`kubernetes` / `static`) |
| `KAPTURE_LOG_LEVEL` | `info` | Internal log level (`debug`, `info`, `warn`, `error`) |

---

## Tech Stack

| Component | Library / Version | Rationale |
|---|---|---|
| **Language** | Go **1.26+** | Single static binary, low memory footprint, native Kubernetes ecosystem |
| **Storage Engine** | BadgerDB **v4.9** | Pure-Go embedded key-value store (no CGO), fast batch writes, Snappy compression |
| **File Watcher** | `fsnotify` **v1.10** + 1s poll | *inotify* for file creation; lightweight 1s poll handles symlinks and kubelet log rotations |
| **Frontend UI** | HTML5, Modern CSS, Vanilla JS | Embedded into binary via `go:embed`. Instant dashboard loads with zero Node build pipeline |
| **Container Spec** | CRI Log Specification | Fully compatible with standard Kubernetes containerd and CRI-O runtimes |

---

## Directory Structure

```
kapture/
├── cmd/
│   └── kapture/
│       └── main.go                 # Application entrypoint (mode switch)
├── internal/
│   ├── agent/                      # Node log collector logic
│   │   ├── agent.go                # Agent lifecycle & batch writer
│   │   ├── enricher/               # Pod metadata & replica detection
│   │   ├── parser/                 # CRI containerd log parser & level detection
│   │   ├── server/                 # Node-local HTTP server
│   │   └── tailer/                 # Inotify watcher & file offset tracker
│   ├── aggregator/                 # Router & central dashboard logic
│   │   ├── aggregator.go           # Aggregator lifecycle & HTTP server
│   │   ├── discovery/              # K8s node / static endpoint discovery
│   │   ├── fanout/                 # Parallel fan-out queries & merge-sort
│   │   └── handler/                # REST API endpoints & static routes
│   ├── config/                     # Configuration & ENV parser
│   ├── model/                      # Structs & protocol definitions
│   └── storage/
│       └── badger/                 # BadgerDB implementation & date indexing
├── web/
│   ├── embed.go                    # go:embed directives for UI assets
│   └── static/                     # Web dashboard assets (HTML, CSS, JS, icons)
├── charts/
│   └── kapture/                    # Official Helm Chart (Chart.yaml, values.yaml, templates)
├── deploy/
│   ├── install.yaml                # Universal standalone manifest (Quick Install)
│   ├── 00-namespace.yaml           # Isolated kapture namespace
│   ├── 01-rbac.yaml                # ClusterRole & ServiceAccount permissions
│   ├── 02-secret.yaml              # Default admin credentials
│   ├── 03-agent-daemonset.yaml     # Per-node agent DaemonSet
│   ├── 04-aggregator-deployment.yaml # Aggregator Deployment
│   └── 05-service.yaml             # Dashboard & agent discovery services
├── build/
│   └── appicon.svg                 # Application vector icon
├── scripts/
│   ├── release-ghcr.sh             # Docker & Helm GHCR release pipeline
│   └── generate-testdata.sh        # Test log data generator
├── Dockerfile                      # Multi-stage container build (~15 MB)
├── Makefile                        # Build, test, and dev runner automation
├── prd.md                          # Product Requirements Document
├── README.id.md                    # Indonesian documentation
└── README.md                       # English documentation (default)
```

---

## License

Distributed under the [MIT](LICENSE) License. Free to use, modify, and distribute for both personal and commercial purposes.

---

## Credits

**Author:** Ari Ardiansyah — [github.com/aribrilliantsyah](https://github.com/aribrilliantsyah) · [ariardiansyah.study@gmail.com](mailto:ariardiansyah.study@gmail.com)

Built to ensure Kubernetes container logs are never lost when you need them most, regardless of programming languages or frameworks.

JetBrains Mono Nerd Font (SIL OFL) and Lucide icons (ISC).

Crafted with Go, a deep passion for minimalist engineering, and cloud-native automation. ⭐
