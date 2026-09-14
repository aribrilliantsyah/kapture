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
- **Manajemen Pengguna (Admin & Operasional)** — Banyak akun dengan dua peran: **Admin** (termasuk kelola pengguna) dan **Operasional** (semua fitur kecuali kelola pengguna). Admin bisa memulihkan sendiri password atau 2FA lewat pertanyaan keamanan; akun operasional di-reset oleh admin. Menu **Profile** untuk ubah nama, password, dan menampilkan ulang QR 2FA saat ganti HP.
- **Backup & Restore** — Unduh log semua node sebagai satu berkas `.tar.gz` (bisa dibatasi rentang tanggal), lalu restore ke Kapture yang sama atau ke Kapture lain, misalnya di laptop untuk investigasi offline.
- **Warna Terminal di Log** — Log yang berisi kode warna ANSI (mis. Spring Boot) ditampilkan berwarna, bukan karakter aneh; level `ERROR`/`WARN` tetap terdeteksi.
- **Ekspor Cepat** — Unduh hasil filter langsung dalam format **JSON** atau **CSV** untuk kebutuhan audit dan laporan tim.
- **Live Tail via WebSocket, Histogram & Compare** — Log baru didorong real-time (agent → aggregator → browser, tetap satu image), klik batang histogram volume untuk zoom ke rentang waktu itu, dan bandingkan 2 replica berdampingan dengan scroll yang tersinkron berdasarkan timestamp.
- **Riwayat per Workload, bukan per Pod** — Agent mengikuti `ownerReferences` (Pod → ReplicaSet → Deployment, Job → CronJob) lewat Kubernetes API. Setiap rollout memberi nama pod baru, tetapi lognya tetap dikelompokkan di bawah Deployment yang sama; tab **Pods** menampilkan semua generasi pod beserta ReplicaSet-nya.
- **Waktu Lokal (mis. WIB)** — Dengan `KAPTURE_TIMEZONE=Asia/Jakarta`, log dikelompokkan per tanggal WIB dan semua jam di dashboard ditampilkan dalam WIB, apa pun zona waktu browser, sehingga cocok dengan jam yang dicetak aplikasi. Waktu diambil dari timestamp container runtime, jadi zona waktu aplikasi (mis. JVM) tidak berpengaruh. Mengganti zona waktu memindahkan log yang sudah tersimpan ke tanggal lokalnya sekali saat agent start.
- **Dashboard Rekap** — Volume harian 14 hari per level, sumber error teratas, workload tersibuk, dan error terbaru. Rekap harian disimpan sebagai indeks sehingga tidak perlu memindai log.
- **Sintaks Pencarian** — `timeout database` (AND), `error OR warning`, `error -healthcheck`, `"connection refused"`, `/failed.*\d+ retries/`, serta filter field `ns:` `workload:` `pod:` `c:` `level:`.
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
| `00-namespace.yaml` | `Namespace` | Ruang isolasi `kapture` |
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

> 🏢 **Panduan Internal:** Untuk langkah lengkap di cluster internal (membuat secret `gitlab-auth` lebih dulu, mengunci aggregator di node tertentu agar akun dashboard tidak hilang, upgrade versi, backup, dan troubleshooting), lihat **[`docs/INTERNAL_K8S_USAGE.md`](docs/INTERNAL_K8S_USAGE.md)**.

Verifikasi pod berjalan:

```bash
kubectl get pods -n kapture -o wide
```

Hasilnya akan menampilkan **1 agent di setiap node** dan **1 aggregator pod**:
```text
NAME                                  READY   STATUS    NODE
kapture-agent-4j2x1                   1/1     Running   master-node
kapture-agent-9b8vc                   1/1     Running   worker-node-1
kapture-agent-z7q1a                   1/1     Running   worker-node-2
kapture-aggregator-5d8f9976f-w2k8m    1/1     Running   worker-node-1
```

### Langkah 3: Akses Dashboard & Setup 2FA (Google Authenticator)

Lakukan *port-forwarding* ke service aggregator:

```bash
kubectl port-forward -n kapture svc/kapture 19488:19488
```

Buka peramban Anda di: **[`http://localhost:19488`](http://localhost:19488)**

Variasi yang sering dipakai:

```bash
# Port lokal lain (mis. 19488 sudah terpakai): buka http://localhost:8080
kubectl port-forward -n kapture svc/kapture 8080:19488

# Jalan di background, hentikan dengan: kill %1 (atau pkill -f "port-forward -n kapture")
kubectl port-forward -n kapture svc/kapture 19488:19488 >/dev/null 2>&1 &

# Bisa diakses dari komputer lain di jaringan yang sama (bind ke semua interface)
kubectl port-forward -n kapture --address 0.0.0.0 svc/kapture 19488:19488

# Pakai kubeconfig / context tertentu
kubectl --context prod-cluster port-forward -n kapture svc/kapture 19488:19488

# Debug satu agent secara langsung (API agent tanpa login, port 19489)
kubectl get pods -n kapture -l role=agent -o wide        # pilih pod di node yang dicari
kubectl port-forward -n kapture pod/<nama-pod-agent> 19489:19489
curl http://localhost:19489/healthz
```

> Koneksi `port-forward` putus bila pod aggregator restart. Jalankan ulang perintahnya. Live tail di dashboard tersambung kembali otomatis setelah forward aktif lagi.

#### 1. Setup Awal (Onboarding Pertama Kali)
Saat pertama kali membuka dashboard, Kapture akan menampilkan halaman **Setup Awal** (3 langkah):
1. **Akun Administrator:** Masukkan Username dan Password baru. Password wajib minimal 8 karakter dan memuat huruf kecil, huruf besar, angka, dan simbol (contoh `Qawsed#1477`). Aturan ini berlaku untuk semua password: setup, pengguna baru, reset, dan ganti password.
2. **Pertanyaan Pemulihan:** Pilih satu pertanyaan keamanan dan isi jawabannya (tidak peka huruf besar/kecil). Dipakai untuk memulihkan password atau 2FA yang hilang.
3. **Scan QR Code 2FA:** Pindai kode QR dengan **Google Authenticator**, **Authy**, **Microsoft Authenticator**, atau pengelola kata sandi favorit Anda, lalu masukkan 6-digit kodenya.

Akun, kunci 2FA, dan jawaban pemulihan (di-hash bcrypt) disimpan di volume persisten Kapture (`/data/kapture/auth.json`), sehingga tidak hilang saat pod di-*restart*. Berkas lama (satu admin) otomatis dimigrasikan; pengguna cukup login ulang sekali.

#### 2. Login Seterusnya (Alur Bertahap)
1. **Kredensial:** Username dan Password (bcrypt).
2. **Kode 2FA:** 6-digit kode dari aplikasi authenticator.
3. **Langkah tambahan bila perlu:** akun baru atau yang password-nya di-reset admin wajib **mengganti password**; akun yang 2FA-nya di-reset wajib **scan QR baru**.
4. **Sesi Aktif:** cookie sesi `HttpOnly` yang ditandatangani (HMAC). Berlaku **30 hari**, diperpanjang otomatis, **tetap valid walau aggregator restart**. Ganti password atau reset oleh admin mengakhiri semua sesi akun itu.

Setiap langkah login berlaku 5 menit. Jika kedaluwarsa, halaman kembali ke form password dengan pesan penjelasan. Setelah 5 kali gagal, klien (dan akun) dikunci 5 menit; kode TOTP yang sudah dipakai tidak bisa dipakai ulang. Untuk skrip, `POST /api/v1/auth/login` dengan `{username, password, code}` mengembalikan token untuk header `Authorization: Bearer <token>`.

#### 3. Pengguna & Peran
| Peran | Hak akses |
|---|---|
| **Admin** | Semua fitur + menu **Users** (tambah, ubah peran, reset password, reset 2FA, hapus pengguna) |
| **Operasional** | Semua fitur kecuali menu Users |

- Admin menambah pengguna dengan **password sementara**. Saat login pertama, pengguna memilih password sendiri lalu memasang 2FA.
- Admin terakhir tidak bisa dihapus atau diturunkan perannya.
- Admin tanpa pertanyaan pemulihan ditandai **Set recovery question** di menu Users. Klik tanda itu (atau menu ⋮) untuk mengatur pertanyaan pemulihan admin lain; untuk akun sendiri diminta password saat ini.
- Menu **Profile** (semua pengguna): ubah nama tampilan dan username, ganti password, **Show QR code** (butuh password) untuk memindahkan 2FA ke HP baru, serta atur pertanyaan pemulihan (admin).

#### 4. Lupa Password / HP Hilang
- **Admin:** klik *Forgot your password or lost your phone?* di halaman login, jawab pertanyaan pemulihan, lalu:
  - *Lupa password* → konfirmasi dengan kode 2FA, lalu buat password baru.
  - *HP hilang* → konfirmasi dengan password, lalu scan QR 2FA baru.

  Jawaban saja tidak pernah cukup untuk mengganti keduanya, sehingga pertanyaan keamanan yang tertebak tidak bisa dipakai mengambil alih akun.
- **Operasional:** minta admin melakukan reset lewat menu **Users**.
- **Semua admin terkunci:** hapus `auth.json` di volume aggregator lalu jalankan setup ulang (log tidak terpengaruh).

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

1. **Retensi (opsional)** — Default `0`: log dari tanggal-tanggal sebelumnya **tidak dihapus otomatis**. Jika `KAPTURE_STORAGE_RETENTION` diisi (mis. `30d`), hari yang seluruhnya lewat batas dibuang oleh GC tiap 5 menit.
2. **Disk Cap Hard-Limit** — Parameter `max_disk` (default `5GB`) membatasi kapasitas maksimal per node. Bila batas hampir tercapai, log terlama otomatis dikorbankan terlebih dahulu.
3. **Reset Manual Seketika** — Buka menu **Storage**, tentukan tanggal yang ingin dihapus, atau tekan **Reset All Logs** untuk mengosongkan seluruh database seketika.
4. **Backup & Restore** — Di menu **Storage**, kartu *Backup and restore*:
   - **Download** mengunduh satu berkas `kapture-backup-<waktu>.tar.gz` berisi log setiap node (opsional dibatasi tanggal awal/akhir). Isi arsip: `nodes/<node>/NNNNNN.badger` (format backup BadgerDB) dan `manifest.json`.
   - **Choose backup file** mengunggah berkas tersebut dan menambahkan lognya. Tidak ada yang dihapus, dan baris yang sudah ada tidak terduplikasi (restore aman diulang). Log tiap node masuk ke agent dengan nama node yang sama, atau ke agent pertama bila node itu tidak ada, misalnya saat me-restore backup produksi ke Kapture di laptop (`make run-agent` + `make run-aggregator`).
   - Tanggal log mengikuti `KAPTURE_TIMEZONE` Kapture tujuan, retensi tujuan juga berlaku. Posisi baca berkas (offset) tidak ikut di-backup.
   - Unggahan besar lewat Ingress mungkin perlu menaikkan batas ukuran body, mis. `nginx.ingress.kubernetes.io/proxy-body-size: "0"`.

---

## Dokumentasi REST API

Kapture menyediakan REST API yang bersih dan mudah diintegrasikan dengan skrip devops atau curl:

| Metode | Endpoint | Deskripsi |
|---|---|---|
| `GET` | `/api/v1/logs` | Log urut waktu (default terbaru dulu). Param: `from`, `to` (RFC3339), `date`, `namespace`, `workload`, `workload_type`, `pod`, `container`, `level` (koma), `search`, `regex`, `exclude_ns`, `limit`, `sort=asc\|desc`, `cursor` |
| `GET` | `/api/v1/logs/export?format=csv\|json` | Ekspor hasil filter (maks `max_results`) |
| `GET` | `/api/v1/stats/volume` | Histogram volume per level + top workload error. Param filter sama, plus `buckets` |
| `GET` | `/api/v1/stats/recap?from=YYYY-MM-DD&namespace=X` | Rekap harian per workload: jumlah baris per level, byte, pod yang terlihat |
| `WS` | `/api/v1/tail` | Live tail WebSocket, parameter filter sama dengan `/api/v1/logs` |
| `GET` | `/api/v1/catalog` | Semua container yang pernah punya log, termasuk pod lama (namespace, workload, pod, node, first/last seen) |
| `GET` | `/api/v1/dates` | Daftar tanggal yang memiliki log |
| `GET` | `/api/v1/namespaces`, `/workloads`, `/pods` | Daftar namespace / workload / pod |
| `GET` | `/api/v1/nodes` | Status tiap agent |
| `GET` | `/api/v1/storage` | Kapasitas disk, jumlah baris, sebaran per tanggal dan per node |
| `GET` | `/api/v1/health` | Versi + status agent (butuh login) |
| `GET` | `/healthz` | Liveness probe publik |
| `DELETE` | `/api/v1/logs?date=YYYY-MM-DD` | Hapus log satu tanggal |
| `DELETE` | `/api/v1/logs?before=YYYY-MM-DD` | Hapus seluruh log sebelum tanggal tertentu |
| `DELETE` | `/api/v1/logs?namespace=X[&workload=Y]` | Hapus log per namespace / workload |
| `DELETE` | `/api/v1/logs/all` | **Reset Total:** Hapus seluruh data log di semua node |
| `GET` | `/api/v1/storage/backup?from=YYYY-MM-DD&to=YYYY-MM-DD` | Unduh backup `.tar.gz` semua node (tanggal opsional) |
| `POST` | `/api/v1/storage/restore` | Restore: body = berkas `.tar.gz` dari endpoint backup |
| `GET` `POST` | `/api/v1/users` | *(Admin)* Daftar / tambah pengguna `{username, display_name, role, password}` |
| `PATCH` `DELETE` | `/api/v1/users/{id}` | *(Admin)* Ubah `{username, display_name, role}` / hapus pengguna |
| `POST` | `/api/v1/users/{id}/password`, `/api/v1/users/{id}/2fa/reset` | *(Admin)* Reset password sementara / reset 2FA |
| `PUT` | `/api/v1/users/{id}/recovery` | *(Admin)* Pertanyaan pemulihan admin lain `{question, answer}` |
| `GET` `PATCH` | `/api/v1/profile` | Profil sendiri (nama tampilan, username) |
| `POST` | `/api/v1/profile/password`, `/api/v1/profile/2fa` | Ganti password `{current, password}` / tampilkan QR 2FA `{password}` |
| `PUT` | `/api/v1/profile/recovery` | *(Admin)* Pertanyaan pemulihan `{question, answer, password}` |

Pagination: kirim `next_cursor` dari respons sebelumnya sebagai `cursor`. Jika `partial: true`, batas scan per request tercapai dan cursor melanjutkan pencarian.

### Contoh Pemanggilan Curl:

```bash
# 1. Buka akses ke aggregator (terminal terpisah, atau tambahkan & di akhir)
kubectl port-forward -n kapture svc/kapture 19488:19488

# 2. Login sekali untuk mendapat token (kode = 6 digit dari authenticator)
TOKEN=$(curl -s -X POST http://localhost:19488/api/v1/auth/login \
  -d '{"username":"admin","password":"<password>","code":"123456"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

# Ambil log error dari workload 'api-server' pada hari ini
curl -H "Authorization: Bearer $TOKEN" "http://localhost:19488/api/v1/logs?workload=api-server&level=ERROR&limit=50"

# Hapus log yang berumur lebih dari 3 hari lalu
curl -H "Authorization: Bearer $TOKEN" -X DELETE "http://localhost:19488/api/v1/logs?before=2026-09-07"

# Reset bersih seluruh database
curl -H "Authorization: Bearer $TOKEN" -X DELETE "http://localhost:19488/api/v1/logs/all"

# Backup semua node ke berkas lokal, lalu restore ke Kapture lain (mis. di laptop)
curl -H "Authorization: Bearer $TOKEN" -o kapture-backup.tar.gz "http://localhost:19488/api/v1/storage/backup?from=2026-09-01"
curl -H "Authorization: Bearer $TOKEN" -X POST --data-binary @kapture-backup.tar.gz "http://localhost:19488/api/v1/storage/restore"
```

Token berlaku 30 hari. Jika `KAPTURE_AUTH_ENABLED=false`, header `Authorization` tidak diperlukan.

---

## Troubleshooting: Log Tidak Muncul

| Gejala | Penyebab & Solusi |
|---|---|
| Dashboard: *No agents discovered* | Aggregator tidak menemukan agent. Cek `kubectl get pods -n kapture -l role=agent` dan service headless `kapture-agents`. |
| Dashboard: *No logs collected yet* | Agent jalan tapi belum menyimpan log. Cek log agent: `cannot open log file (is its symlink target mounted?)` berarti target symlink tidak ter-mount. Node dengan runtime Docker butuh mount `/var/lib/docker/containers` (lihat komentar di `deploy/03-agent-daemonset.yaml`). |
| Aggregator restart terus | Versi lama memakai liveness probe `/api/v1/health` yang butuh login (401). Gunakan manifest terbaru (`/healthz`). |
| Upgrade dari versi lama | Layout key database berubah (urut waktu). Saat start, agent menghapus data lama beserta offset lalu membaca ulang file log yang masih ada di node. |

---

## Konfigurasi Lingkungan (Environment Variables)

Konfigurasi dibaca dari variabel lingkungan (`KAPTURE_*`, dengan `LOG_CATCHER_*` sebagai nama lama). `config.example.yaml` hanya dokumentasi, tidak dibaca oleh binary.

| Variabel Lingkungan | Nilai Bawaan | Keterangan |
|---|---|---|
| `LOG_CATCHER_MODE` | `agent` | Mode eksekusi: `agent` atau `aggregator` |
| `LOG_CATCHER_LOG_PATH` | `/var/log/containers` | Direktori target file log di node host |
| `LOG_CATCHER_STORAGE_PATH`| `/data/kapture` | Direktori database BadgerDB lokal |
| `KAPTURE_TIMEZONE` | `TZ` atau `UTC` | Zona waktu IANA, mis. `Asia/Jakarta`. Menentukan batas tanggal (penyimpanan, rekap, hapus per tanggal) dan jam di dashboard. Samakan di agent dan aggregator |
| `LOG_CATCHER_STORAGE_RETENTION` | `0` (simpan terus) | `0` = tidak dihapus otomatis (hanya `max_disk` atau hapus manual). Bisa juga `30d`, `168h` |
| `LOG_CATCHER_STORAGE_MAX_DISK` | `5GB` | Batas maksimum ruang disk sebelum rotasi paksa |
| `LOG_CATCHER_AGENT_PORT` | `19489` | Port HTTP internal agent |
| `LOG_CATCHER_DASHBOARD_PORT` | `19488` | Port antarmuka web Aggregator |
| `LOG_CATCHER_AUTH_ENABLED` | `true` | Login + 2FA di dashboard (`false` untuk intranet tertutup) |
| `LOG_CATCHER_DISCOVERY_METHOD` | `kubernetes` | Metode penemuan node agent (`kubernetes` / `static`) |
| `LOG_CATCHER_LOG_LEVEL` | `info` | Tingkat log internal (`debug`, `info`, `warn`, `error`) |

---

## Tumpukan Teknologi

| Komponen | Pustaka / Versi | Alasan Pemilihan |
|---|---|---|
| **Bahasa Utama** | Go **1.26+** | Kompilasi single binary, performa konkurensi goroutine tinggi, ekosistem native K8s |
| **Engine Basis Data** | BadgerDB **v4.9** | Key-Value store embedded murni Go (tanpa CGO), cepat untuk operasi batch write, dilengkapi kompresi Snappy bawaan & TTL |
| **Pendeteksi Berkas** | `fsnotify` **v1.10** + poll 1 detik | *inotify* untuk file baru/terhapus. `/var/log/containers/*.log` adalah symlink ke `/var/log/pods`, dan inotify tidak melaporkan tulisan ke target symlink, jadi isi file dibaca lewat poll ringan (1 `fstat` per file per detik) yang juga mengikuti rotasi kubelet |
| **Frontend UI** | HTML5, CSS3 kustom, Vanilla JS | Berkas statis di-embed ke dalam binary melalui `go:embed`. Membuka dashboard instan tanpa lag dan tanpa build-step Node yang rumit |
| **Format Kontainer** | CRI Log Specification | Kompatibel penuh dengan runtime containerd dan CRI-O standar Kubernetes modern |

---

## Struktur Direktori

```
k8s-log-catcher/
├── cmd/
│   └── kapture/
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
│   ├── 00-namespace.yaml           # Namespace isolasi kapture
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

**Penulis:** Ari Ardiansyah — [github.com/aribrilliantsyah](https://github.com/aribrilliantsyah) · [ariardiansyah.study@gmail.com](mailto:ariardiansyah.study@gmail.com)

Dibuat untuk mempermudah monitoring log pod Kubernetes agar tidak lagi hilang saat dibutuhkan, tanpa peduli framework atau bahasa yang dipakai aplikasinya.

Sebagian perancangan dan penulisan kode dibantu model **Claude** (Anthropic) dan **Gemini** (Google); setiap usulan tetap ditinjau, diuji, dan disesuaikan secara manual.

Font JetBrains Mono Nerd Font (SIL OFL) dan ikon Lucide (ISC). Halaman **About** di dashboard memuat ringkasan yang sama.

Dibangun dengan Go, kecintaan pada sistem yang minimalis, dan semangat otomasi cloud-native. ⭐
