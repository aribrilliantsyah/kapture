<div align="center">

<img src="build/appicon.png" width="128" alt="Kapture">

# Kapture

**Kubernetes Application & Pod Tracking and Unified Resource Explorer**

Penangkap dan pengelola log Kubernetes mandiri, ultra-ringan, dan persisten. Menyimpan riwayat log container berhari-hari tanpa membebani server dan tanpa ketergantungan stack berat (ELK/Loki), dilengkapi dashboard bawaan yang responsif serta kendali reset instan.

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-CRI--Native-326CE5?style=flat&logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Storage](https://img.shields.io/badge/Engine-BadgerDB%20v4-7952B3?style=flat)](https://github.com/dgraph-io/badger)
[![Binary Size](https://img.shields.io/badge/Binary-~13%20MB-success?style=flat)](#)
[![Zero External DB](https://img.shields.io/badge/Dependencies-Zero-brightgreen?style=flat)](#)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

## Kenapa Kapture

Saat melakukan investigasi insiden di cluster Kubernetes pada jam 2 pagi, hal berikut hampir selalu terjadi:

```bash
$ kubectl logs api-server-7f8b9c-x2k1p
# Hanya memuat beberapa baris terakhir...

$ kubectl logs api-server-7f8b9c-x2k1p --previous
# Error from server (BadRequest): previous terminated container not found...
```

1. **Log pod yang restart atau terminated langsung musnah.**
2. **Log rotate bawaan kubelet** (default 10MB × 5 file) dengan cepat menimpa histori beberapa hari ke belakang.
3. **Solusi umum (ELK Stack atau Loki + Grafana)** membutuhkan setidaknya 2–4 GB RAM per node, konfigurasi rumit, dan membebani resource server secara konstan.

**Kapture** dirancang untuk menyelesaikan dilema ini dengan cara yang sangat efisien:
- **Membaca langsung dari berkas lokal** (`/var/log/containers/*.log`) yang sudah ditulis kubelet menggunakan kernel *inotify*. Tidak ada overhead API call ke K8s API server.
- **Tersimpan per tanggal** di engine embedded lokal (BadgerDB v4) dengan kompresi data hemat ruang.
- **Ringan dan hening di background** — hanya butuh memori ~64 MB dan CPU idle <1%.
- **Kendali penuh ukuran disk** — database dapat dibersihkan per tanggal, per namespace, atau di-reset total kapan saja lewat satu klik di dashboard tanpa perlu restart pod.

---

## Fitur Unggulan

- **100% Service & Workload Agnostic** — Tangkap log dari workload apa saja tanpa konfigurasi per-service: Deployment, StatefulSet, DaemonSet, Job, CronJob, Init Container, Sidecar (Envoy/Istio), hingga Operator CRD kustom.
- **Replica-Aware & Grouping Cerdas** — Secara otomatis mengenali pola penamaan pod Kubernetes. Log dari 5 replica `api-server` dapat dilihat bersamaan (merged & time-aligned) atau diisolasi per individual pod.
- **Penyimpanan Berbasis Tanggal (Date-First Architecture)** — Log diindeks dengan prefix tanggal (`YYYY-MM-DD`). Sangat cepat untuk mencari insiden kemarin, 3 hari lalu, atau 1 minggu yang lalu.
- **Tombol Reset & Purge Seketika** — Khawatir log membengkak? Tersedia auto-retention (default 7 hari), batas disk hard-limit, serta tombol **Reset All Logs** di antarmuka web untuk mengosongkan database seketika.
- **Dashboard Web Bawaan (Embedded Single-Binary)** — Antarmuka web modern dengan tema gelap (dark mode), live-tail real-time, filter autocomplete, dan rincian log stack-trace. Tidak butuh instalasi Node.js atau web server terpisah.
- **Filter Analitik Multidimensi** — Saring data berdasarkan:
  - Rentang Tanggal & Jam
  - Namespace & Workload
  - Pod & Container
  - Tingkat Keparahan (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`)
  - Pencarian Teks Bebas & Regex (`/timeout.*after \d+ms/`)
- **Autentikasi 2FA Universal (Google Authenticator)** — Dilindungi sistem login berstandar industri dengan Time-based One-Time Password (TOTP, RFC 6238). Wizard setup awal menyajikan QR code untuk dipindai langsung via Google Authenticator atau Authy. Setiap sesi dashboard berikutnya wajib diverifikasi dengan kode OTP 6-digit.
- **Ekspor Cepat** — Unduh hasil filter langsung dalam format **JSON** atau **CSV** untuk kebutuhan audit dan laporan tim.
- **Zero External Dependencies** — Berjalan sebagai satu binary murni Go (`~13 MB`). Tidak perlu Elasticsearch, Postgres, Redis, atau Fluentd.

---

## Arsitektur: Bagaimana Kapture Bekerja?

Kapture menggunakan model **Distributed-Local Storage**. Log tidak dikirim bolak-balik via jaringan ke database terpusat, melainkan disimpan secara lokal di node tempat pod berjalan. Komponen Aggregator hanya bertindak sebagai *query router* saat Anda membuka dashboard.

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
│  │  │  • BadgerDB (lokal node)│  │        │  │  • BadgerDB (lokal node)│  │ │
│  │  └────────────┬────────────┘  │        │  └────────────┬────────────┘  │ │
│  └───────────────┼───────────────┘        └───────────────┼───────────────┘ │
│                  │                                        │                 │
│                  └───────────────────┬────────────────────┘                 │
│                                      │ (HTTP Internal)                      │
│                                      ▼                                      │
│                      ┌─────────────────────────────────┐                    │
│                      │  Kapture Aggregator             │                    │
│                      │  (Deployment, 1 Pod)            │                    │
│                      │  • Menemukan agent otomatis     │                    │
│                      │  • Fan-out query & merge sort   │                    │
│                      │  • Menyajikan Web Dashboard     │                    │
│                      └────────────────┬────────────────┘                    │
│                                       │                                     │
└───────────────────────────────────────┼─────────────────────────────────────┘
                                        │ HTTP :19488
                                        ▼
                            Browser / Web Dashboard
```

### 2 Peran dalam 1 Binary:

| Peran | Pola Deployment | Lokasi Berjalan | Fungsi Utama |
|---|---|---|---|
| **`agent`** | **DaemonSet** (1 pod per node) | Di **seluruh node** (Master & Worker) | Memantau `/var/log/containers/*.log`, mem-parse metadata, dan menyimpan batch ke BadgerDB lokal di node tersebut. |
| **`aggregator`** | **Deployment** (1 replica saja) | Di salah satu node (dipilih otomatis oleh scheduler K8s) | Mengarahkan pencarian ke seluruh agent, menggabungkan hasil log secara urut waktu (*merge-sort*), dan melayani dashboard web. |

---

## Deploy ke Kubernetes

Semua konfigurasi (RBAC, DaemonSet Agent, Deployment Aggregator, dan Service) sudah disatukan ke dalam **satu berkas manifest siap pakai**.

### Langkah 1: Siapkan Container Image

Bangun dan unggah image Kapture ke registry container Anda (Docker Hub, GitHub Packages, atau private registry):

```bash
# 1. Build binary container
docker build -t your-registry/kapture:latest .

# 2. Push ke registry
docker push your-registry/kapture:latest
```

> 📖 **Panduan Lengkap Registry:** Kunjungi **[`docs/CONTAINER_REGISTRY_GUIDE.md`](docs/CONTAINER_REGISTRY_GUIDE.md)** untuk petunjuk lengkap langkah demi langkah mengunggah image ke **Docker Hub, GitHub Container Registry (GHCR), Harbor, AWS ECR, GCP GAR**, serta panduan multi-architecture build (`linux/amd64` dan `linux/arm64`).
>
> **Catatan:** Image yang digunakan untuk `agent` dan `aggregator` adalah **image yang sama persis**. Mode kerjanya ditentukan otomatis lewat argumen `--mode=agent` dan `--mode=aggregator`.

### Langkah 2: Sesuaikan Manifest & Terapkan

Manifest Kapture telah **dipisah secara modular** per tanggung jawab komponen agar mudah dikelola dalam GitOps/CI-CD:

| Berkas | Jenis Sumber Daya | Fungsi |
|---|---|---|
| `00-namespace.yaml` | `Namespace` | Ruang isolasi `log-catcher` |
| `01-rbac.yaml` | `ClusterRole`, `Binding` | Izin akses membaca metadata pod/namespace |
| `02-secret.yaml` | `Secret` | Kredensial awal admin |
| `03-agent-daemonset.yaml` | `DaemonSet` | Pengumpul log di setiap node host |
| `04-aggregator-deployment.yaml` | `Deployment` | Dashboard web & router query |
| `05-service.yaml` | `Service` | Endpoint akses dashboard & headless discovery |

Cukup sesuaikan nama image di `03-agent-daemonset.yaml` dan `04-aggregator-deployment.yaml`, lalu terapkan sekaligus dari workstation/master:

```bash
# Terapkan seluruh direktori deploy (otomatis terurut):
kubectl apply -f deploy/
```

Verifikasi pod berjalan:

```bash
kubectl get pods -n log-catcher -o wide
```

Hasilnya akan menampilkan **1 agent di setiap node** dan **1 aggregator pod**:
```text
NAME                                         READY   STATUS    NODE
k8s-log-catcher-agent-4j2x1                  1/1     Running   master-node
k8s-log-catcher-agent-9b8vc                  1/1     Running   worker-node-1
k8s-log-catcher-agent-z7q1a                  1/1     Running   worker-node-2
k8s-log-catcher-aggregator-5d8f9976f-w2k8m   1/1     Running   worker-node-1
```

### Langkah 3: Akses Dashboard & Setup 2FA (Google Authenticator)

Lakukan *port-forwarding* ke service aggregator:

```bash
kubectl port-forward -n log-catcher svc/k8s-log-catcher 19488:19488
```

Buka peramban Anda di: **[`http://localhost:19488`](http://localhost:19488)**

#### 1. Setup Awal (Onboarding Pertama Kali)
Saat pertama kali membuka dashboard, Kapture akan menampilkan dialog **Setup Awal**:
1. **Atur Kredensial Administrator:** Masukkan Username dan Password baru (minimal 6 karakter).
2. **Scan QR Code 2FA:** Pindai kode QR yang muncul menggunakan aplikasi **Google Authenticator**, **Authy**, **Microsoft Authenticator**, atau pengelola kata sandi favorit Anda.
3. **Verifikasi OTP:** Masukkan 6-digit kode verifikasi yang tampil di aplikasi authenticator Anda, lalu klik **"Simpan & Aktifkan 2FA"**.

Setup kredensial dan kunci rahasia 2FA langsung disimpan secara aman ke volume persisten Kapture (`/data/logcatcher/auth.json`), sehingga tidak akan hilang saat pod di-*restart*.

#### 2. Login Seterusnya (Alur 2 Langkah yang Aman)
Setelah setup selesai, setiap kali mengakses dashboard Anda akan melewati alur masuk dua tahap:
1. **Langkah 1 (Kredensial):** Masukkan Username dan Password. Sistem memvalidasi kebenaran akun dan hash password (bcrypt).
2. **Langkah 2 (Verifikasi 2FA):** Setelah password terbukti benar, antarmuka otomatis beralih menampilkan form 2FA untuk memasukkan **6-digit kode OTP** dari aplikasi Google Authenticator.
3. **Sesi Aktif:** Setelah kode 2FA terverifikasi, sesi aktif diterbitkan dan berlaku selama **24 jam**. Anda dapat mengakhiri sesi kapan saja lewat tombol **`🚪 Keluar`** di bar navigasi atas.

> 🔒 **Opsi Intranet Non-Auth:** Jika Kapture di-*deploy* di jaringan lokal tertutup dan Anda ingin menonaktifkan login secara total, set variabel lingkungan `LOG_CATCHER_AUTH_ENABLED=false`.

*(Opsi lain: Buat Ingress atau ubah tipe Service ke `NodePort` / `LoadBalancer` pada `deploy/05-service.yaml` jika ingin diakses langsung dari jaringan kantor).*

---

## Menjalankan dari Sumber & Pengujian Lokal

Anda dapat menguji Kapture secara penuh di komputer lokal tanpa perlu cluster Kubernetes asli.

### Prasyarat
- **Go 1.26+** terpasang
- Sistem operasi Linux atau macOS

### 1. Buat Data Simulasi Container Log

Skrip bawaan akan membuat berkas log berformat CRI containerd yang realistis (replika pod, log error, warn, multiline, cronjob):

```bash
make testdata
# Log tiruan akan dibuat di testdata/containers/
```

### 2. Jalankan Agent di Terminal 1

```bash
make run-agent
```
Agent akan membaca log di `testdata/containers`, menyimpannya ke database lokal di `testdata/db`, dan membuka port API `:19489`.

### 3. Jalankan Aggregator di Terminal 2

```bash
make run-aggregator
```
Aggregator akan aktif di port `:19488`, menghubungkan diri ke agent lokal, dan menyajikan dashboard web.

Buka browser di **`http://localhost:19488`**.

### 4. Menjalankan Unit Test

```bash
make test
```

---

## Manajemen Penyimpanan & Reset Log

Kapture memberikan kontrol penuh agar penyimpanan disk server Anda tidak pernah penuh:

```
┌─ Storage Management ────────────────────────────────────────┐
│  Disk Used:  ████░░░░░░░░░░░░░░░░  154 KB / 5.0 GB (3%)     │
│  Total Logs: 350 baris                                      │
│  Rentang:    2026-09-08 s/d 2026-09-10                      │
│                                                             │
│  [📅 Hapus Sebelum Tanggal: [ 2026-09-09 ] [Hapus]]        │
│  [⚠️ Reset Total Semua Log]                                  │
└─────────────────────────────────────────────────────────────┘
```

1. **Auto-Retention (TTL)** — Setiap baris log memiliki masa aktif (default `168h` / 7 hari). Latar belakang garbage collector berjalan tiap 5 menit untuk memangkas data kedaluwarsa.
2. **Disk Cap Hard-Limit** — Parameter `max_disk` (default `5GB`) membatasi kapasitas maksimal per node. Bila batas hampir tercapai, log terlama otomatis dikorbankan terlebih dahulu.
3. **Reset Manual Seketika** — Klik tombol **Storage** di kanan atas dashboard, tentukan tanggal yang ingin dihapus, atau tekan **Reset All Logs** untuk mengosongkan seluruh database seketika.

---

## Dokumentasi REST API

Kapture menyediakan REST API yang bersih dan mudah diintegrasikan dengan skrip devops atau curl:

| Metode | Endpoint | Deskripsi |
|---|---|---|
| `GET` | `/api/v1/logs` | Ambil baris log dengan filter query parameter |
| `GET` | `/api/v1/dates` | Daftar seluruh tanggal yang memiliki rekaman log |
| `GET` | `/api/v1/namespaces` | Daftar namespace yang tercatat di database |
| `GET` | `/api/v1/workloads` | Daftar workload (hasil pengelompokan replika pod) |
| `GET` | `/api/v1/pods` | Daftar nama pod aktif |
| `GET` | `/api/v1/storage` | Statistik kapasitas disk, baris log, dan sebaran per tanggal |
| `GET` | `/api/v1/health` | Healthcheck status koneksi aggregator ke seluruh agent |
| `DELETE` | `/api/v1/logs?before=YYYY-MM-DD` | Hapus seluruh log sebelum tanggal tertentu |
| `DELETE` | `/api/v1/logs/all` | **Reset Total:** Hapus seluruh data log di semua node |

### Contoh Pemanggilan Curl:

```bash
# Ambil log error dari workload 'api-server' pada hari ini
curl "http://localhost:19488/api/v1/logs?workload=api-server&level=ERROR&limit=50"

# Hapus log yang berumur lebih dari 3 hari lalu
curl -X DELETE "http://localhost:19488/api/v1/logs?before=2026-09-07"

# Reset bersih seluruh database
curl -X DELETE "http://localhost:19488/api/v1/logs/all"
```

---

## Konfigurasi Lingkungan (Environment Variables)

Semua opsi konfigurasi dapat dikontrol lewat berkas `config.yaml` maupun variabel lingkungan:

| Variabel Lingkungan | Nilai Bawaan | Keterangan |
|---|---|---|
| `LOG_CATCHER_MODE` | `agent` | Mode eksekusi: `agent` atau `aggregator` |
| `LOG_CATCHER_LOG_PATH` | `/var/log/containers` | Direktori target file log di node host |
| `LOG_CATCHER_STORAGE_PATH`| `/data/logcatcher` | Direktori database BadgerDB lokal |
| `LOG_CATCHER_STORAGE_RETENTION` | `168h` (7 hari) | Batas retensi log otomatis (`72h`, `168h`, `720h`) |
| `LOG_CATCHER_STORAGE_MAX_DISK` | `5GB` | Batas maksimum ruang disk sebelum rotasi paksa |
| `LOG_CATCHER_AGENT_PORT` | `19489` | Port HTTP internal agent |
| `LOG_CATCHER_DASHBOARD_PORT` | `19488` | Port antarmuka web Aggregator |
| `LOG_CATCHER_PASSWORD` | *(kosong)* | Aktifkan Basic Auth dashboard jika diisi |
| `LOG_CATCHER_DISCOVERY_METHOD` | `kubernetes` | Metode penemuan node agent (`kubernetes` / `static`) |
| `LOG_CATCHER_LOG_LEVEL` | `info` | Tingkat log internal (`debug`, `info`, `warn`, `error`) |

---

## Tumpukan Teknologi

| Komponen | Pustaka / Versi | Alasan Pemilihan |
|---|---|---|
| **Bahasa Utama** | Go **1.26+** | Kompilasi single binary, performa konkurensi goroutine tinggi, ekosistem native K8s |
| **Engine Basis Data** | BadgerDB **v4.9** | Key-Value store embedded murni Go (tanpa CGO), cepat untuk operasi batch write, dilengkapi kompresi Snappy bawaan & TTL |
| **Pendeteksi Berkas** | `fsnotify` **v1.10** | Memanfaatkan *inotify* kernel Linux untuk mendeteksi perubahan log seketika tanpa *polling loop* |
| **Frontend UI** | HTML5, CSS3 kustom, Vanilla JS | Berkas statis di-embed ke dalam binary melalui `go:embed`. Membuka dashboard instan tanpa lag dan tanpa build-step Node yang rumit |
| **Format Kontainer** | CRI Log Specification | Kompatibel penuh dengan runtime containerd dan CRI-O standar Kubernetes modern |

---

## Struktur Direktori

```
k8s-log-catcher/
├── cmd/
│   └── logcatcher/
│       └── main.go                 # Entrypoint aplikasi (mode switch)
├── internal/
│   ├── agent/                      # Logika pengumpul log pada node
│   │   ├── agent.go                # Siklus hidup agent & batch writer
│   │   ├── enricher/               # Ekstraksi metadata pod & deteksi replika
│   │   ├── parser/                 # Parser CRI containerd & deteksi level log
│   │   ├── server/                 # HTTP server lokal agent
│   │   └── tailer/                 # Inotify watcher & pembaca offset berkas
│   ├── aggregator/                 # Logika router & pusat dashboard
│   │   ├── aggregator.go           # Siklus hidup aggregator & HTTP server
│   │   ├── discovery/              # Penemu node K8s / endpoint statis
│   │   ├── fanout/                 # Query paralel ke seluruh node & merge sort
│   │   └── handler/                # REST API endpoints & route statis
│   ├── config/                     # Pengurai konfigurasi YAML & ENV
│   ├── model/                      # Definisi struct & protokol data log
│   └── storage/
│       └── badger/                 # Implementasi database BadgerDB & indeks tanggal
├── web/
│   ├── embed.go                    # Direktif go:embed untuk bundling UI
│   └── static/                     # Aset dashboard web (HTML, CSS, JS, Icon)
├── deploy/
│   ├── 00-namespace.yaml           # Namespace isolasi log-catcher
│   ├── 01-rbac.yaml                # Izin ClusterRole & ServiceAccount
│   ├── 02-secret.yaml              # Kredensial awal admin
│   ├── 03-agent-daemonset.yaml     # Manifest DaemonSet agent per node
│   ├── 04-aggregator-deployment.yaml # Manifest Deployment aggregator
│   └── 05-service.yaml             # Service dashboard & agent discovery
├── build/
│   └── appicon.svg                 # Ikon vektor aplikasi bergaya macOS
├── scripts/
│   └── generate-testdata.sh        # Generator simulasi log untuk pengujian
├── Dockerfile                      # Multi-stage container build (~15 MB)
├── Makefile                        # Otomasi build, test, dan dev runner
├── prd.md                          # Dokumen Product Requirements (PRD)
└── README.md
```

---

## Lisensi

Didistribusikan di bawah lisensi [MIT](LICENSE). Bebas digunakan, dimodifikasi, dan didistribusikan baik untuk keperluan pribadi maupun komersial.

---

## Kredit

Dibuat untuk mempermudah monitoring log pod Kubernetes agar tidak lagi hilang saat dibutuhkan.

Dibangun dengan Go, kecintaan pada sistem yang minimalis, dan semangat otomasi cloud-native. ⭐
