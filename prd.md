# PRD: k8s-log-catcher

> Simpan log Kubernetes yang biasa hilang, bisa diakses kapan saja, ringan, dan bisa di-reset sewaktu-waktu.

---

## 1. Masalah

```
$ kubectl logs api-server-7f8b9c-x2k1p
→ Cuma tampil log yang sekarang

$ kubectl logs api-server-7f8b9c-x2k1p --previous
→ Cuma 1 restart kebelakang, itupun kalau pod belum di-schedule ulang

Log 3 hari lalu? Hilang.
Log dari pod yang sudah terminated? Hilang.
Log saat incident jam 2 pagi kemarin? Hilang.
```

**Yang dibutuhkan:**

1. Log dari `kubectl logs` tersimpan otomatis per tanggal
2. Bisa diakses kapan saja lewat dashboard (hari ini, kemarin, minggu lalu)
3. Tidak membebani server — ringan, jalan di background
4. Database bisa di-reset/dikosongkan kapan saja supaya tidak membengkak
5. Support semua jenis service yang jalan di K8s — tidak perlu setting per service
6. Support pod replika — `api-server` punya 5 replicas, semua log-nya bisa dilihat bareng atau per-pod

---

## 2. Solusi: Cara Kerjanya

### Sumber Data: File yang Sama dengan `kubectl logs`

`kubectl logs` membaca dari file yang ditulis kubelet di setiap node:

```
/var/log/containers/<pod>_<namespace>_<container>-<id>.log
```

k8s-log-catcher **baca file yang sama**, lalu simpan ke database lokal sebelum Kubernetes menghapus/rotate file tersebut.

```
Pod stdout/stderr
       │
       ▼
/var/log/containers/*.log    ← kubelet tulis ke sini
       │                     ← kubectl logs baca dari sini
       │
       ▼
k8s-log-catcher (DaemonSet)  ← kita juga baca dari sini
       │
       ▼
BadgerDB (embedded)           ← simpan per tanggal, compressed
       │
       ▼
Dashboard (built-in)          ← browse, search, filter
```

### Kenapa Ringan

| Aspek | Pendekatan |
|-------|-----------|
| Baca log | Baca file yang sudah ada di disk — bukan API call, bukan intercept network |
| Simpan | BadgerDB embedded — pure Go, ~50 MB RAM, built-in compression |
| Dashboard | Embedded di binary yang sama — bukan container terpisah |
| Background | inotify — hanya bereaksi saat ada log baru, bukan polling |

---

## 3. Arsitektur

### 2 Komponen, 1 Binary

```
┌─── Node 1 ──────────────────┐    ┌─── Node 2 ──────────────────┐
│                              │    │                              │
│  Pod A    Pod B    Pod C     │    │  Pod D    Pod E    Pod F     │
│   │        │        │        │    │   │        │        │        │
│   ▼        ▼        ▼        │    │   ▼        ▼        ▼        │
│  /var/log/containers/*.log   │    │  /var/log/containers/*.log   │
│           │                  │    │           │                  │
│           ▼                  │    │           ▼                  │
│  ┌──────────────────┐        │    │  ┌──────────────────┐        │
│  │ k8s-log-catcher  │        │    │  │ k8s-log-catcher  │        │
│  │ --mode=agent     │        │    │  │ --mode=agent     │        │
│  │                  │        │    │  │                  │        │
│  │ • Baca log file  │        │    │  │ • Baca log file  │        │
│  │ • Simpan ke DB   │        │    │  │ • Simpan ke DB   │        │
│  │ • Serve local API│        │    │  │ • Serve local API│        │
│  └──────────────────┘        │    │  └──────────────────┘        │
└──────────────────────────────┘    └──────────────────────────────┘
                    │                           │
                    └─────────┬─────────────────┘
                              ▼
                 ┌──────────────────────┐
                 │ k8s-log-catcher      │
                 │ --mode=aggregator    │
                 │                      │
                 │ • Query ke agents    │
                 │ • Merge hasil        │
                 │ • Serve dashboard    │
                 │ • Reset/cleanup API  │
                 └──────────────────────┘
                              │
                              ▼
                        Web Dashboard
```

| Komponen | Deploy Sebagai | RAM | CPU |
|----------|---------------|-----|-----|
| Agent | DaemonSet (1 per node) | ~64 MB (limit 128 MB) | < 1% idle |
| Aggregator | Deployment (1 pod) | ~64 MB (limit 256 MB) | < 1% idle |

### Service-Agnostic: Tangkap Semua, Otomatis

Agent baca **semua file** di `/var/log/containers/`. Tidak peduli service-nya apa:

| Jenis Workload | Contoh | Ditangkap? |
|---------------|--------|:---:|
| Deployment (+ replicas) | `api-server` × 5 replicas | ✅ |
| StatefulSet | `postgres-0`, `postgres-1` | ✅ |
| DaemonSet | `node-exporter` | ✅ |
| Job / CronJob | `backup-job`, `hourly-cleanup` | ✅ |
| Standalone Pod | `debug-pod` | ✅ |
| Init Container | `init-migrate` | ✅ |
| Sidecar | `istio-proxy`, `envoy` | ✅ |
| Operator/CRD workload | `kafka-broker-0`, `redis-cluster-1` | ✅ |
| Helm release | Apapun | ✅ |
| Knative / Argo / Tekton | Apapun yang jadi Pod | ✅ |

**Prinsipnya: kalau bisa `kubectl logs`, kita tangkap.**

Tidak perlu setting per-service. Tidak perlu SDK. Tidak perlu sidecar. Tinggal deploy DaemonSet, selesai.

### Replica-Aware: Pod Replika Dikenali

Kubernetes naming convention:
```
Deployment:   api-server-7f8b9c6d4-x2k1p     → workload: api-server
              api-server-7f8b9c6d4-y3m4n     → workload: api-server
              api-server-7f8b9c6d4-z5o6p     → workload: api-server

StatefulSet:  postgres-0                      → workload: postgres
              postgres-1                      → workload: postgres

CronJob:      cleanup-28456123-abc12          → workload: cleanup
```

Agent extract **workload name** dari pod name. Di dashboard bisa:
- **View per-workload**: Lihat gabungan log dari semua replica `api-server`
- **View per-pod**: Lihat log spesifik dari 1 replica
- **Compare**: Side-by-side log dari 2 replica (debug kenapa 1 error tapi lainnya tidak)

---

## 4. Storage: Simpan Per Tanggal, Bisa Di-Reset

### Key Design

```
Key:   <tanggal>:<namespace>:<workload>:<pod>:<container>:<timestamp_nano>:<seq>
Value: compressed(log_line)

Contoh:
  2025-01-15:production:api-server:api-server-7f8b-x2k1p:app:1736942400123456789:0001
```

**Tanggal sebagai prefix pertama** — ini memungkinkan:

1. **Browse per tanggal**: "Tampilkan log tanggal 15 Januari"
2. **Delete per tanggal**: "Hapus semua log sebelum 10 Januari" → cepat, hanya delete key range
3. **Reset total**: "Kosongkan semua" → drop entire database, < 1 detik

### Retention & Reset

```yaml
storage:
  retention: 168h        # Auto-hapus log > 7 hari (default)
  max_disk: 5GB          # Hard limit — hapus terlama saat mendekati limit
```

**3 Cara Mengelola Ukuran Database:**

| Cara | Mekanisme | Kapan |
|------|----------|-------|
| **Auto-retention** | TTL per entry, GC tiap 5 menit | Otomatis, di background |
| **Disk cap** | Monitor disk usage, hapus terlama saat > limit | Otomatis, proteksi disk penuh |
| **Manual reset** | API call atau tombol di dashboard | Kapan saja user mau |

### Reset API

```
# Hapus log per tanggal
DELETE /api/v1/logs?before=2025-01-10

# Hapus log per namespace
DELETE /api/v1/logs?namespace=staging

# Hapus log per workload
DELETE /api/v1/logs?namespace=production&workload=debug-pod

# RESET TOTAL — kosongkan semua log
DELETE /api/v1/logs/all

# Cek disk usage
GET /api/v1/storage
→ { "used": "1.2 GB", "limit": "5 GB", "entries": 2400000, "oldest": "2025-01-08", "newest": "2025-01-15" }
```

### Dashboard: Storage Management

```
┌─────────────────────────────────────────────────────────────┐
│  ⚙️ Storage Management                                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Disk Usage:  ████████░░░░░░░░░░░░  1.2 GB / 5 GB (24%)   │
│  Total Logs:  2,400,000 entries                             │
│  Oldest Log:  2025-01-08                                    │
│  Newest Log:  2025-01-15                                    │
│  Retention:   7 days (auto)                                 │
│                                                             │
│  ┌─ Per-Date Breakdown ────────────────────────────────────┐│
│  │  📅 2025-01-15 (today)    342,000 entries   180 MB  [🗑]││
│  │  📅 2025-01-14            356,000 entries   190 MB  [🗑]││
│  │  📅 2025-01-13            310,000 entries   165 MB  [🗑]││
│  │  📅 2025-01-12            298,000 entries   155 MB  [🗑]││
│  │  📅 2025-01-11            345,000 entries   180 MB  [🗑]││
│  │  📅 2025-01-10            389,000 entries   195 MB  [🗑]││
│  │  📅 2025-01-09            360,000 entries   185 MB  [🗑]││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  [🗑 Delete Before Date: [2025-01-10] [Delete]]             │
│  [⚠️ Reset All Logs] ← konfirmasi 2x sebelum eksekusi      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. Dashboard: Browse, Search, Filter

### Layout Utama

```
┌─────────────────────────────────────────────────────────────────────┐
│  k8s-log-catcher                    [📊 Stats] [⚙️ Storage] [🔧]   │
├────────────────┬────────────────────────────────────────────────────┤
│                │                                                    │
│  Namespaces    │  📅 Date: [2025-01-15 ▾]  Time: [All Day ▾]       │
│                │  🔍 Search: [__________________] [Regex ☐]         │
│  ● all         │                                                    │
│  ○ production  │  Workload: [All ▾]  Pod: [All ▾]  Level: [All ▾]  │
│  ○ database    │                                                    │
│  ○ kube-system │  ┌─ Logs ──────────────────────────────────────┐  │
│                │  │                                              │  │
│  Workloads     │  │ 10:30:01 api-server [pod-x2k1p] INFO  ...   │  │
│                │  │ 10:30:01 api-server [pod-y3m4n] INFO  ...   │  │
│  ▼ api-server  │  │ 10:30:02 worker     [pod-3d4e]  WARN  ...   │  │
│    ├ pod-x2k1p │  │ 10:30:03 postgres   [postgres-0] INFO ...   │  │
│    ├ pod-y3m4n │  │ 10:30:03 api-server [pod-x2k1p] ERROR ...   │  │
│    └ pod-z5o6p │  │ 10:30:04 api-server [pod-x2k1p] ERROR       │  │
│  ▶ worker (3)  │  │          Stack: goroutine 1 [running]:       │  │
│  ▶ postgres(3) │  │                 main.handler(...)            │  │
│  ▶ redis (1)   │  │ 10:30:05 worker     [pod-8f9g]  INFO  ...   │  │
│                │  │                                       ▼ tail │  │
│  ── Actions ── │  └──────────────────────────────────────────────┘  │
│  [All Replicas]│                                                    │
│  [Compare]     │  Showing 1,234 logs  [⬇ Export] [📋 Copy]         │
│  [Live Tail]   │  [◀ Prev Date] [Next Date ▶]                      │
└────────────────┴────────────────────────────────────────────────────┘
```

### Filter Lengkap

| Filter | Tipe | Contoh |
|--------|------|--------|
| **Tanggal** | Date picker | `2025-01-15`, range `01-10 s/d 01-15` |
| **Waktu** | Time range | `10:00 - 11:00`, atau All Day |
| **Namespace** | Multi-select | `production`, `database` |
| **Workload** | Dropdown + autocomplete | `api-server`, `postgres` |
| **Pod** | Dropdown (nested di bawah workload) | `api-server-7f8b-x2k1p` |
| **Container** | Dropdown | `app`, `sidecar`, `init-migrate` |
| **Level** | Toggle buttons | `DEBUG` `INFO` `WARN` `ERROR` `FATAL` |
| **Node** | Dropdown | `node-1`, `node-2` |
| **Workload Type** | Toggle | `Deployment` `StatefulSet` `Job` `CronJob` |
| **Full-text Search** | Search box | `"connection refused"` |
| **Regex** | Toggle regex mode | `error.*timeout.*\d+ms` |
| **Exclude** | Negative filter | `-kube-system`, `-healthcheck` |

### Search Syntax

```
# Exact phrase
"connection refused"

# AND (default)
timeout database

# OR
error OR warning

# NOT
error -healthcheck

# Field-specific
namespace:production level:error workload:api-server
date:2025-01-15 pod:postgres-0

# Regex
/failed.*after \d+ retries/
```

### Fitur Dashboard Lainnya

**Live Tail** — WebSocket, real-time log masuk, bisa subscribe per workload (semua replicas)

**Log Detail** — klik log entry → expand detail (timestamp, namespace, workload, pod, container, node, level, full message + stack trace)

**Replica Compare** — split-pane view 2 replicas side-by-side, sync scroll, time-aligned

**Export** — download log sebagai CSV/JSON, filtered

**Statistics** — log volume per jam, error rate, top error workloads

---

## 6. API

### REST API

```
# ── Query ──
GET  /api/v1/logs                  Query logs (semua filter via query params)
GET  /api/v1/logs/export           Export sebagai CSV/JSON
WS   /api/v1/tail                  Live tail via WebSocket

# ── Browse ──
GET  /api/v1/dates                 List tanggal yang punya log
GET  /api/v1/namespaces            List namespaces
GET  /api/v1/workloads             List workloads (grouped replicas)
GET  /api/v1/workloads/:name       Detail workload + list replica pods
GET  /api/v1/pods                  List pods
GET  /api/v1/nodes                 List nodes (agents)

# ── Stats ──
GET  /api/v1/stats                 Overview statistics
GET  /api/v1/stats/volume          Log volume per time bucket

# ── Storage Management ──
GET    /api/v1/storage             Disk usage, entry count, date range
DELETE /api/v1/logs?before=<date>  Hapus log sebelum tanggal tertentu
DELETE /api/v1/logs?namespace=X    Hapus log per namespace
DELETE /api/v1/logs?workload=X     Hapus log per workload
DELETE /api/v1/logs/all            RESET — kosongkan semua log

# ── Health ──
GET  /api/v1/health                Health check
```

### Query Params `/api/v1/logs`

```
?date=2025-01-15                  Tanggal spesifik
&from=2025-01-15T10:00:00Z        Dari waktu
&to=2025-01-15T11:00:00Z          Sampai waktu
&namespace=production             Namespace
&workload=api-server              Workload (semua replicas)
&pod=api-server-7f8b-x2k1p       Pod spesifik
&container=app                    Container
&level=error,warn                 Log level
&search=timeout                   Full-text search
&regex=failed.*\d+ms              Regex search
&exclude_ns=kube-system           Exclude namespace
&limit=100                        Max results
&cursor=eyJ0cyI6...               Pagination cursor
&sort=desc                        Sort order
```

---

## 7. Agent: Detail Cara Kerja

### Collection Pipeline

```
/var/log/containers/*.log
         │
    fsnotify (inotify)         ← React saat file baru muncul / file berubah
         │
    Tail Reader                ← Baca dari last offset, resume setelah restart
         │
    CRI Parser                 ← Parse: "2025-01-15T10:30:00Z stdout F message"
         │
    Enricher                   ← Extract dari filename: namespace, pod, container
         │                     ← Extract workload name dari pod name
         │                     ← Detect log level dari content
         │
    Batch Writer               ← Kumpulkan 100 lines atau 100ms, tulis 1 batch
         │
    BadgerDB                   ← Compressed, TTL-based, per-date prefix
```

### Resource Controls

```yaml
collector:
  batch_size: 100           # Lines per batch write
  batch_interval: 100ms     # Flush interval
  max_goroutines: 50        # Max concurrent file tailers
  rate_limit: 5000          # Max lines/sec per node
  buffer_size: 4096         # Read buffer per file
```

### Offset Tracking

Setiap file yang di-tail, offset terakhir disimpan di BadgerDB:
```
_offset:/var/log/containers/api-server-7f8b_production_app-a1b2.log → 485932
```

Saat agent restart → resume dari offset → **tidak ada log yang hilang**.

---

## 8. Internal Communication: Agent ↔ Aggregator

### gRPC (Agent ↔ Aggregator)

```protobuf
service LogAgent {
  rpc QueryLogs(QueryRequest) returns (stream LogEntry);
  rpc TailLogs(TailRequest) returns (stream LogEntry);
  rpc DeleteLogs(DeleteRequest) returns (DeleteResponse);
  rpc StorageInfo(Empty) returns (StorageResponse);
  rpc Health(Empty) returns (HealthResponse);
}

message LogEntry {
  int64  timestamp = 1;
  string namespace = 2;
  string workload = 3;       // extracted workload name
  string workload_type = 4;  // deployment, statefulset, job, etc.
  string pod = 5;
  string container = 6;
  string node = 7;
  string stream = 8;         // stdout/stderr
  string level = 9;
  string message = 10;
  string date = 11;          // YYYY-MM-DD
}

message DeleteRequest {
  string before_date = 1;    // hapus sebelum tanggal ini
  string namespace = 2;      // opsional: hapus per namespace
  string workload = 3;       // opsional: hapus per workload
  bool   all = 4;            // true = reset total
}

message StorageResponse {
  int64  used_bytes = 1;
  int64  max_bytes = 2;
  int64  entry_count = 3;
  string oldest_date = 4;
  string newest_date = 5;
  repeated DateBreakdown dates = 6;
}

message DateBreakdown {
  string date = 1;
  int64  entry_count = 2;
  int64  size_bytes = 3;
}
```

### Aggregator Discovery

Aggregator menemukan agents via:
1. Kubernetes API — list pods dengan label `app=k8s-log-catcher,role=agent`
2. Atau static endpoint list (untuk non-K8s testing)

### Query Flow

```
User → Dashboard → Aggregator → fan-out gRPC ke semua agents
                              → merge sort by timestamp
                              → stream ke browser

Reset → Dashboard → Aggregator → fan-out DeleteLogs ke semua agents
                               → return total deleted count
```

---

## 9. Konfigurasi

### Config File

```yaml
mode: agent  # agent | aggregator

agent:
  log_path: /var/log/containers

  storage:
    path: /data/logcatcher
    retention: 168h          # 7 hari, auto-hapus
    max_disk: 5GB            # Hard limit
    compression: snappy      # snappy | zstd | none
    gc_interval: 5m

  collector:
    batch_size: 100
    batch_interval: 100ms
    max_goroutines: 50
    rate_limit: 5000

  exclude:
    namespaces: []           # default: tangkap semua
    labels:
      logging: "false"       # pod dengan label ini di-skip

  api:
    port: 19489
    grpc_port: 19490

aggregator:
  dashboard:
    port: 19488
    auth:
      enabled: true          # Default aktif dengan wizard setup 2FA (Google Authenticator)
      file_path: /data/logcatcher/auth.json # Lokasi penyimpanan persisten kredensial & TOTP secret

  discovery:
    method: kubernetes
    label_selector: "app=k8s-log-catcher,role=agent"

  query:
    timeout: 30s
    max_results: 10000

log_level: info
```

### Environment Variables

```bash
LOG_CATCHER_MODE=agent
LOG_CATCHER_STORAGE_RETENTION=168h
LOG_CATCHER_STORAGE_MAX_DISK=5GB
LOG_CATCHER_DASHBOARD_PORT=19488
LOG_CATCHER_PASSWORD=secret
```

---

## 10. Deployment

### Quick Start

```bash
kubectl apply -f deploy/
```

### Manifests

**Namespace + RBAC:**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: log-catcher
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: k8s-log-catcher
  namespace: log-catcher
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-log-catcher
rules:
  - apiGroups: [""]
    resources: ["pods", "namespaces"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-log-catcher
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k8s-log-catcher
subjects:
  - kind: ServiceAccount
    name: k8s-log-catcher
    namespace: log-catcher
```

**DaemonSet (Agent):**
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: k8s-log-catcher-agent
  namespace: log-catcher
spec:
  selector:
    matchLabels:
      app: k8s-log-catcher
      role: agent
  template:
    metadata:
      labels:
        app: k8s-log-catcher
        role: agent
    spec:
      serviceAccountName: k8s-log-catcher
      tolerations:
        - operator: Exists
      containers:
        - name: agent
          image: ghcr.io/user/k8s-log-catcher:latest
          args: ["--mode=agent"]
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          ports:
            - containerPort: 19489
            - containerPort: 19490
          volumeMounts:
            - name: varlog
              mountPath: /var/log
              readOnly: true
            - name: data
              mountPath: /data/logcatcher
          livenessProbe:
            httpGet:
              path: /healthz
              port: 19489
      volumes:
        - name: varlog
          hostPath:
            path: /var/log
        - name: data
          hostPath:
            path: /var/lib/k8s-log-catcher
            type: DirectoryOrCreate
```

**Deployment (Aggregator) + Service:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-log-catcher-aggregator
  namespace: log-catcher
spec:
  replicas: 1
  selector:
    matchLabels:
      app: k8s-log-catcher
      role: aggregator
  template:
    metadata:
      labels:
        app: k8s-log-catcher
        role: aggregator
    spec:
      serviceAccountName: k8s-log-catcher
      containers:
        - name: aggregator
          image: ghcr.io/user/k8s-log-catcher:latest
          args: ["--mode=aggregator"]
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
          ports:
            - containerPort: 19488
---
apiVersion: v1
kind: Service
metadata:
  name: k8s-log-catcher
  namespace: log-catcher
spec:
  type: ClusterIP
  selector:
    app: k8s-log-catcher
    role: aggregator
  ports:
    - port: 19488
      targetPort: 19488
```

---

## 11. Tech Stack

| Komponen | Teknologi | Alasan |
|----------|-----------|--------|
| Language | Go 1.23+ | Single binary, goroutine ringan, K8s ecosystem |
| Storage | BadgerDB v4 | Pure Go, write-optimized, compression, TTL built-in |
| Agent ↔ Aggregator | gRPC | Streaming, binary protocol, efficient |
| Dashboard | Go `embed` + Preact (3KB) | Embedded di binary, instant load |
| File watching | fsnotify | inotify wrapper, event-driven |
| WebSocket | nhooyr/websocket | Live tail |
| K8s client | client-go | Agent discovery |
| Config | viper | YAML + env vars |
| CSS | Pico CSS (~10KB) | Classless, responsive, dark mode |
| Charts | uPlot (~35KB) | Ringan, performant |

---

## 12. Project Structure

```
k8s-log-catcher/
├── cmd/
│   └── logcatcher/
│       └── main.go
├── internal/
│   ├── agent/
│   │   ├── agent.go                # Agent lifecycle
│   │   ├── tailer/
│   │   │   ├── tailer.go           # File tailer manager
│   │   │   ├── reader.go           # Single file reader + offset
│   │   │   └── watcher.go          # fsnotify watcher
│   │   ├── parser/
│   │   │   ├── cri.go              # CRI log parser
│   │   │   ├── multiline.go        # Stack trace detection
│   │   │   └── level.go            # Log level extraction
│   │   ├── enricher/
│   │   │   ├── metadata.go         # Namespace/pod/container dari filename
│   │   │   └── workload.go         # Workload name + type extraction
│   │   └── server/
│   │       ├── grpc.go             # gRPC server
│   │       └── http.go             # Health + local API
│   ├── aggregator/
│   │   ├── aggregator.go           # Aggregator lifecycle
│   │   ├── discovery/
│   │   │   ├── kubernetes.go       # K8s agent discovery
│   │   │   └── static.go           # Static endpoints
│   │   ├── fanout/
│   │   │   ├── query.go            # Fan-out query
│   │   │   ├── merge.go            # K-way merge sort
│   │   │   ├── tail.go             # Fan-out live tail
│   │   │   └── delete.go           # Fan-out delete/reset
│   │   └── handler/
│   │       ├── logs.go             # Query API
│   │       ├── storage.go          # Storage management + reset
│   │       ├── stats.go            # Statistics
│   │       ├── ws.go               # WebSocket live tail
│   │       └── export.go           # CSV/JSON export
│   ├── storage/
│   │   ├── store.go                # Interface
│   │   └── badger/
│   │       ├── badger.go           # BadgerDB implementation
│   │       ├── keys.go             # Key encoding (date-prefix)
│   │       ├── gc.go               # GC + retention
│   │       └── reset.go            # Delete/reset operations
│   ├── model/
│   │   └── log.go                  # LogEntry, shared types
│   └── config/
│       └── config.go
├── web/
│   ├── embed.go
│   └── static/
│       ├── index.html
│       ├── app.js
│       ├── components/
│       │   ├── log-viewer.js       # Virtual scroll log list
│       │   ├── filter-bar.js       # All filters
│       │   ├── sidebar.js          # Namespace → Workload → Pod tree
│       │   ├── live-tail.js        # WebSocket live tail
│       │   ├── storage-mgmt.js     # Storage & reset panel
│       │   ├── stats-panel.js      # Charts
│       │   ├── log-detail.js       # Detail modal
│       │   └── compare-view.js     # Replica compare
│       └── styles/
│           └── main.css
├── proto/
│   └── logcatcher.proto
├── deploy/
│   ├── 00-namespace.yaml
│   ├── 01-rbac.yaml
│   ├── 02-secret.yaml
│   ├── 03-agent-daemonset.yaml
│   ├── 04-aggregator-deployment.yaml
│   └── 05-service.yaml
├── Dockerfile
├── Makefile
├── go.mod
├── config.example.yaml
├── prd.md
└── README.md
```

---

## 13. Development Phases

### Phase 1: Core Agent (Minggu 1-2)

- [ ] Go module setup
- [ ] Config loading
- [ ] CRI log parser
- [ ] fsnotify file watcher
- [ ] File tailer + offset tracking
- [ ] Metadata enricher (namespace/pod/container from filename)
- [ ] Workload name extractor (strip hash suffixes)
- [ ] BadgerDB storage (date-prefix keys, compression, TTL)
- [ ] Retention GC + disk cap
- [ ] Health endpoint

**Deliverable:** Agent yang collect dan persist log dari semua pod di node.

### Phase 2: Aggregator & API (Minggu 3-4)

- [ ] gRPC proto + codegen
- [ ] Agent gRPC server
- [ ] Aggregator discovery
- [ ] Fan-out query + merge sort
- [ ] REST API (logs, namespaces, workloads, pods, dates)
- [ ] Storage management API (disk usage, delete, reset)
- [ ] WebSocket live tail
- [ ] Pagination (cursor-based)

**Deliverable:** Query log dari semua nodes via API, reset via API.

### Phase 3: Dashboard (Minggu 5-6)

- [ ] HTML shell + responsive layout
- [ ] Sidebar (namespace → workload → pod hierarchy)
- [ ] Log viewer (virtual scroll)
- [ ] All filters (date, namespace, workload, pod, level, search, regex)
- [ ] Live tail (WebSocket)
- [ ] Log detail modal
- [ ] Storage management page (usage, per-date breakdown, reset button)
- [ ] Dark mode
- [ ] Export CSV/JSON
- [ ] Embed via `go:embed`

**Deliverable:** Full dashboard, browse per tanggal, reset dari UI.

### Phase 4: Polish (Minggu 7-8)

- [ ] Replica compare view
- [ ] Statistics page (volume chart, error rate, top workloads)
- [ ] Multiline log support (stack traces)
- [ ] Basic auth
- [ ] Dockerfile
- [ ] K8s manifests
- [ ] README + docs
- [ ] CI/CD

**Deliverable:** Production-ready v1.0.

---

## 14. Risiko & Mitigasi

| Risiko | Mitigasi |
|--------|----------|
| Pod spam log → disk penuh | Rate limiter + disk cap + auto-GC + manual reset |
| Agent restart → log hilang | Offset tracking, resume dari last position |
| Database membengkak | Auto-retention + disk cap + manual reset kapan saja |
| Query lambat (banyak node) | Timeout 30s, result cap 10K, streaming response |
| Workload name extraction salah | Regex fallback + optional K8s API owner reference lookup |
| Pod pindah node → log terpecah | Aggregator merge dari semua nodes otomatis |

---

## 15. Success Criteria

| Metrik | Target |
|--------|--------|
| Log tersimpan setelah pod restart/delete | ✅ 100% (selama agent running) |
| Browse log per tanggal | ✅ Bisa pilih tanggal di dashboard |
| Reset database | ✅ < 5 detik untuk kosongkan semua |
| Agent memory | < 128 MB per node |
| Agent CPU idle | < 1% |
| Dashboard load | < 1 detik |
| Single binary size | < 30 MB |
| Deploy | `kubectl apply -f deploy/` — selesai |
| Service-agnostic | ✅ Semua workload K8s otomatis ditangkap |
| Replica support | ✅ View merged / per-pod / compare |
