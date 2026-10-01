# Panduan Troubleshooting: Mengatasi Agent Kapture yang Gagal Berjalan

Dokumen ini berisi langkah-langkah diagnosis dan solusi tuntas ketika Pod `kapture-agent` di worker node mengalami status **`CrashLoopBackOff`**, **`Error`**, atau gagal melewati pemeriksaan probe (**`connection refused`**).

---

## 1. Langkah Pertama: Identifikasi Penyebab Pasti

Jangan menebak-nebak penyebab kegagalan. Jalankan dua perintah berikut dari master node untuk melihat log error sebelum kontainer mati:

### A. Periksa Log Kontainer Terakhir
```bash
# Ganti <nama-pod-agent> dengan nama pod yang bermasalah (mis. kapture-agent-kbzk8)
kubectl logs -n kapture <nama-pod-agent> --previous
```
*(Flag `--previous` membaca log sesaat sebelum kontainer di-restart oleh Kubernetes).*

### B. Periksa Alasan Penghentian (Last State)
```bash
kubectl describe pod -n kapture <nama-pod-agent> | grep -A 5 "Last State:"
```

Hasil dari dua perintah di atas akan mengarahkan Anda ke salah satu penyebab di bawah ini:

---

## 2. Daftar Masalah dan Solusinya

### Masalah A: Kehabisan Memori / OOMKilled (Exit Code 137)

#### Gejala:
- Output `describe pod` menampilkan:
  ```text
  Last State:     Terminated
    Reason:       OOMKilled
    Exit Code:    137
  ```
- Event pod menampilkan probe gagal (`connection refused`) karena kontainer dimatikan paksa oleh kernel Linux sebelum sempat melayani HTTP.

#### Penyebab:
Database BadgerDB lokal memerlukan alokasi memori untuk *memtable* dan *block cache* saat inisialisasi. Jika node memiliki banyak pod atau berkas log yang sedang diindeks, batas default **128Mi** bisa terlampaui.

#### Solusi:
Tingkatkan batas memori (`limits.memory`) agent menjadi **`256Mi`** atau **`512Mi`**.

**Jika menggunakan Helm:**
```bash
helm upgrade kapture oci://ghcr.io/aribrilliantsyah/charts/kapture \
  --namespace kapture \
  --reuse-values \
  --set agent.resources.limits.memory=256Mi \
  --set agent.resources.requests.memory=128Mi
```

**Jika menggunakan manifest manual (`deploy/03-agent-daemonset.yaml`):**
Ubah bagian `resources`:
```yaml
resources:
  requests:
    cpu: 50m
    memory: 128Mi
  limits:
    cpu: 200m
    memory: 256Mi
```
Lalu terapkan ulang:
```bash
kubectl apply -f deploy/03-agent-daemonset.yaml
```

---

### Masalah B: Kunci Database Tertinggal (BadgerDB Lock)

#### Gejala:
Log kontainer menampilkan pesan:
```text
failed to create agent: open badger: Cannot acquire directory lock on "/data/kapture": Another process is using this Badger database
```

#### Penyebab:
Pod agent sebelumnya dihentikan secara paksa (`SIGKILL`, node restart mendadak, atau crash) sehingga berkas kunci database (`LOCK`) di host node belum sempat dilepas secara bersih.

#### Solusi:
Hapus berkas `LOCK` secara manual di worker node yang bersangkutan:

1. SSH ke **worker node** tempat pod agent berjalan:
   ```bash
   ssh user@worker-node
   ```
2. Hapus berkas `LOCK`:
   ```bash
   sudo rm -f /var/lib/kapture/LOCK
   ```
3. Kembali ke master node dan restart pod agent tersebut:
   ```bash
   kubectl delete pod -n kapture <nama-pod-agent>
   ```

---

### Masalah C: Izin Akses Direktori HostPath (`Permission Denied`)

#### Gejala:
Log kontainer menampilkan:
```text
failed to create agent: open badger: open /data/kapture/MANIFEST: permission denied
```
atau
```text
mkdir /data/kapture: permission denied
```

#### Penyebab:
Direktori `/var/lib/kapture` di host node dibuat dengan kepemilikan atau izin yang tidak dapat dibaca/ditulis oleh proses kontainer.

#### Solusi:
1. Masuk ke worker node yang bersangkutan.
2. Perbaiki izin direktori penyimpanan:
   ```bash
   sudo chmod 755 /var/lib/kapture
   ```
3. Jika menggunakan SELinux (RHEL/CentOS/Rocky Linux), pastikan direktori diizinkan untuk container volume:
   ```bash
   sudo chcon -Rt svirt_sandbox_file_t /var/lib/kapture
   ```

---

### Masalah D: Symlink Log Docker Runtime Tidak Terbaca

#### Gejala:
Agent berhasil berjalan tetapi log dashboard kosong, atau log kontainer menampilkan:
```text
cannot open log file (is its symlink target mounted?): /var/log/pods/...: no such file or directory
```

#### Penyebab:
Di cluster yang menggunakan Docker Engine / `cri-dockerd`, berkas log di `/var/log/containers/*.log` adalah *symlink* yang mengarah ke `/var/lib/docker/containers/`. Jika direktori target tidak di-mount, agent tidak bisa membaca fisik berkasnya.

#### Solusi:
Aktifkan kompatibilitas runtime Docker:

**Jika menggunakan Helm:**
```bash
helm upgrade kapture oci://ghcr.io/aribrilliantsyah/charts/kapture \
  --namespace kapture \
  --reuse-values \
  --set agent.dockerRuntimeCompat=true
```

**Jika menggunakan manifest manual:**
Buka `deploy/03-agent-daemonset.yaml`, lalu hilangkan tanda komentar (`#`) pada volume dan volumeMount `dockercontainers`:
```yaml
volumeMounts:
  - name: dockercontainers
    mountPath: /var/lib/docker/containers
    readOnly: true
volumes:
  - name: dockercontainers
    hostPath:
      path: /var/lib/docker/containers
      type: Directory
```

---

### Masalah E: Zona Waktu Tidak Valid

#### Gejala:
Log kontainer menampilkan:
```text
invalid KAPTURE_TIMEZONE, use an IANA name like Asia/Jakarta
```

#### Penyebab:
Variabel `KAPTURE_TIMEZONE` diisi dengan format yang salah (misalnya singkatan seperti `WIB` bukan format resmi IANA).

#### Solusi:
Gunakan nama zona waktu IANA resmi seperti `Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, atau `UTC`.

---

## 3. Checklist Verifikasi Setelah Perbaikan

Setelah menerapkan salah satu solusi di atas, verifikasi kesehatan pod agent:

```bash
# 1. Pastikan STATUS sudah "Running" dan READY bernilai 1/1
kubectl get pods -n kapture -o wide

# 2. Cek apakah probe sudah merespons dengan benar
kubectl exec -n kapture <nama-pod-agent> -- wget -qO- http://localhost:19489/healthz
# Respons yang benar: {"node":"<nama-node>","status":"ok","version":"..."}
```
