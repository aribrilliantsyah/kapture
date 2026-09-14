# Panduan Internal: Deploy Kapture di Cluster Kubernetes

Dokumen internal untuk tim. Isinya urutan pasti dari cluster kosong sampai dashboard bisa dibuka, memakai folder manifest `kapture/` yang sudah dipaketkan (`00-namespace.yaml` sampai `05-service.yaml`).

Ringkasnya hanya dua langkah:

```bash
# 1. Namespace + secret registry (sekali saja per cluster)
kubectl apply -f kapture/00-namespace.yaml
kubectl create secret docker-registry gitlab-auth -n kapture \
  --docker-server=registry.e-gitlab.prodak.id \
  --docker-username='<username-gitlab>' \
  --docker-password='<personal-access-token>'

# 2. Terapkan seluruh paket
kubectl apply -f kapture/
```

Sisanya di bawah ini adalah detail, penyesuaian node, dan hal-hal yang wajib diketahui sebelum dipakai di produksi.

---

## 1. Prasyarat

| Kebutuhan | Keterangan |
|---|---|
| Akses `kubectl` | Dengan hak membuat Namespace, ClusterRole, dan ClusterRoleBinding |
| Image sudah ter-push | `registry.e-gitlab.prodak.id/bpd-diy1/va/kapture:<versi>` (lihat [`CONTAINER_REGISTRY_GUIDE.md`](CONTAINER_REGISTRY_GUIDE.md) atau `make docker-push VERSION=v1.0.5`) |
| Kredensial registry | Username GitLab + **Personal Access Token** dengan scope `read_registry` |
| Node target aggregator | Satu node tetap untuk menyimpan akun dashboard (lihat bagian 4) |

Isi folder paket:

| Berkas | Sumber Daya | Fungsi |
|---|---|---|
| `00-namespace.yaml` | Namespace | Ruang isolasi `kapture` |
| `01-rbac.yaml` | ServiceAccount, ClusterRole, ClusterRoleBinding | Izin baca `pods`, `namespaces`, `replicasets`, `jobs` untuk pengelompokan log per workload |
| `02-secret.yaml` | Secret `kapture-auth` | Lihat catatan penting di bagian 6 |
| `03-agent-daemonset.yaml` | DaemonSet | Agent di setiap node, membaca `/var/log/containers` |
| `04-aggregator-deployment.yaml` | Deployment | Dashboard dan router query |
| `05-service.yaml` | Service `kapture` + headless `kapture-agents` | Akses dashboard dan penemuan agent |

---

## 2. Langkah 1: Namespace dan Secret Registry

Secret registry **harus dibuat lebih dulu**, karena kedua manifest memakai `imagePullSecrets: gitlab-auth`. Namespace harus ada sebelum secret dibuat di dalamnya.

```bash
kubectl apply -f kapture/00-namespace.yaml

kubectl create secret docker-registry gitlab-auth \
  --namespace kapture \
  --docker-server=registry.e-gitlab.prodak.id \
  --docker-username='<username-gitlab>' \
  --docker-password='<personal-access-token>'
```

> **Jangan pakai password akun GitLab.** Buat Personal Access Token (Settings → Access Tokens) dengan scope `read_registry` saja, atau Deploy Token khusus untuk project ini.

Verifikasi:

```bash
kubectl get secret gitlab-auth -n kapture
kubectl get secret gitlab-auth -n kapture -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d
```

Memperbarui token yang kedaluwarsa (ganti isi secret tanpa menghapus deployment):

```bash
kubectl create secret docker-registry gitlab-auth -n kapture \
  --docker-server=registry.e-gitlab.prodak.id \
  --docker-username='<username-gitlab>' \
  --docker-password='<token-baru>' \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deployment/kapture-aggregator -n kapture
kubectl rollout restart daemonset/kapture-agent -n kapture
```

Alternatif: `scripts/build-and-push.sh` (di repo) otomatis membuat secret ini setelah push image, memakai kredensial dari `scripts/registry.conf`.

---

## 3. Langkah 2: Terapkan Paket

```bash
kubectl apply -f kapture/
```

`kubectl` membaca berkas sesuai urutan nama, jadi Namespace lebih dulu, lalu RBAC, Secret, DaemonSet, Deployment, dan Service.

Verifikasi:

```bash
kubectl get pods -n kapture -o wide
```

Hasil yang diharapkan: **satu agent per node** dan **satu aggregator**.

```text
NAME                                  READY   STATUS    NODE
kapture-agent-4j2x1                   1/1     Running   master-node
kapture-agent-9b8vc                   1/1     Running   worker-node-1
kapture-agent-z7q1a                   1/1     Running   worker-node-2
kapture-aggregator-5d8f9976f-w2k8m    1/1     Running   worker-node-1
```

Cek aggregator sudah menemukan seluruh agent:

```bash
kubectl logs -n kapture deploy/kapture-aggregator | grep -i discovery
```

Buka dashboard:

```bash
kubectl port-forward -n kapture svc/kapture 19488:19488
# lalu buka http://localhost:19488
```

---

## 4. Mengunci Aggregator di Node Tertentu

**Ini wajib dipahami.** Aggregator menyimpan akun dashboard, kunci 2FA, dan pertanyaan pemulihan di `auth.json`, lewat `hostPath: /var/lib/kapture-aggregator` di node tempat pod berjalan.

Artinya: **kalau pod aggregator pindah node, ia tidak menemukan `auth.json` dan kembali menampilkan wizard setup awal.** Akun lama tidak hilang, tetapi tertinggal di disk node yang lama. Karena itu aggregator sebaiknya dikunci di satu node tetap.

Lihat nama node:

```bash
kubectl get nodes
```

Tambahkan `nodeSelector` di `kapture/04-aggregator-deployment.yaml`, sejajar dengan `serviceAccountName` (di dalam `spec.template.spec`):

```yaml
    spec:
      serviceAccountName: kapture
      nodeSelector:
        kubernetes.io/hostname: worker-node-1     # ganti dengan node pilihan
```

Lalu terapkan ulang:

```bash
kubectl apply -f kapture/04-aggregator-deployment.yaml
kubectl rollout status deployment/kapture-aggregator -n kapture
```

Pilihan lain sesuai kebutuhan:

```yaml
      # a. Pakai label sendiri, supaya node bisa diganti tanpa mengubah manifest:
      #    kubectl label node worker-node-1 kapture-role=aggregator
      nodeSelector:
        kapture-role: aggregator

      # b. Kalau node tujuan adalah control-plane, tambahkan toleration:
      tolerations:
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
```

Cara memindahkan aggregator ke node lain tanpa kehilangan akun:

```bash
# 1. Backup dulu dari dashboard (menu Storage -> Download backup), atau salin berkasnya:
kubectl cp kapture/<nama-pod-aggregator>:/data/kapture/auth.json ./auth.json

# 2. Ubah nodeSelector ke node baru, lalu apply

# 3. Salin auth.json ke node baru (lewat pod baru):
kubectl cp ./auth.json kapture/<nama-pod-aggregator-baru>:/data/kapture/auth.json
kubectl rollout restart deployment/kapture-aggregator -n kapture
```

Kalau `auth.json` tidak ikut dipindah, jalankan setup awal lagi dan buat ulang akun.

> Agent tidak perlu dikunci. DaemonSet memang harus jalan di semua node, dan setiap agent menyimpan log node-nya sendiri di `/var/lib/kapture`.

---

## 5. Menyesuaikan Konfigurasi Internal

Semua konfigurasi lewat environment variable di manifest.

**Versi image** (kedua berkas, harus sama):

```bash
# Cara cepat tanpa mengedit berkas:
kubectl set image -n kapture deployment/kapture-aggregator aggregator=registry.e-gitlab.prodak.id/bpd-diy1/va/kapture:v1.0.6
kubectl set image -n kapture daemonset/kapture-agent agent=registry.e-gitlab.prodak.id/bpd-diy1/va/kapture:v1.0.6
```

Sebaiknya tetap ubah juga di `kapture/03-*.yaml` dan `kapture/04-*.yaml`, supaya `kubectl apply` berikutnya tidak mengembalikan versi lama.

**Pengaturan yang sering diubah:**

| Variabel | Lokasi | Catatan |
|---|---|---|
| `KAPTURE_TIMEZONE` | agent + aggregator | **Wajib sama di keduanya.** Menentukan batas tanggal penyimpanan dan jam di dashboard |
| `KAPTURE_STORAGE_RETENTION` | agent | `0` = tidak pernah dihapus otomatis. Isi `30d` bila ingin log lebih dari 30 hari dibuang |
| `KAPTURE_STORAGE_MAX_DISK` | agent | Batas disk per node, default `5GB`. Hari terlama dibuang saat penuh |
| `KAPTURE_AUTH_ENABLED` | aggregator | `false` hanya untuk jaringan internal tertutup. Dashboard jadi tanpa login |

Setelah mengubah, terapkan lalu tunggu rollout:

```bash
kubectl apply -f kapture/
kubectl rollout status daemonset/kapture-agent -n kapture
kubectl rollout status deployment/kapture-aggregator -n kapture
```

**Batas memori.** Aggregator dibatasi `256Mi` dan agent `128Mi`. Kalau nanti melakukan *restore* backup berukuran besar ke agent di cluster, naikkan limit memori agent ke `256Mi` supaya tidak kena OOMKilled.

---

## 6. Catatan Penting Sebelum Produksi

**a. Secret `kapture-auth` tidak dipakai untuk login.** `02-secret.yaml` berisi `admin` / `admin123`, dan `KAPTURE_USERNAME` serta `KAPTURE_PASSWORD` di manifest aggregator **tidak dibaca** oleh sistem akun. Akun administrator yang sebenarnya dibuat lewat **wizard setup** saat dashboard pertama kali dibuka (username, password, pertanyaan pemulihan, dan QR 2FA), lalu disimpan di `auth.json`. Jadi:

- Jangan menganggap `admin123` sebagai password dashboard.
- Segera buka dashboard setelah deploy dan selesaikan setup, supaya tidak ada orang lain yang mendahului membuat akun administrator.
- Password wajib minimal 8 karakter dengan huruf kecil, huruf besar, angka, dan simbol (contoh `Qawsed#1477`).

**b. Jangan menulis `imagePullSecrets` dua kali.** Pada konfigurasi yang beredar di tim, `imagePullSecrets` sempat muncul dua kali di dalam `spec.template.spec` (satu sebelum `containers`, satu lagi setelahnya). Dengan validasi ketat, yang menjadi bawaan `kubectl` versi baru, apply ditolak:

```text
error converting YAML to JSON: yaml: unmarshal errors:
  line NN: key "imagePullSecrets" already set in map
```

Kalau validasinya longgar (`--validate=false`, atau alat lain), duplikat itu diterima diam-diam dan **nilai terakhir yang dipakai**. Ini justru berbahaya: entri pertama terlihat ada di berkas tetapi tidak berpengaruh. Cukup tulis satu kali, sebelum `containers`. Versi di repo ini sudah benar.

**c. Akses dari luar.** `05-service.yaml` memakai `ClusterIP`, jadi hanya bisa diakses lewat `port-forward`. Untuk akses tetap dari jaringan kantor, ubah ke `NodePort`/`LoadBalancer` atau buat Ingress. Bila memakai Ingress dan ingin memakai fitur *restore* backup, naikkan batas ukuran body:

```yaml
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
```

**d. Node dengan runtime Docker.** Bila node memakai `cri-dockerd`, symlink log menunjuk ke `/var/lib/docker/containers`. Aktifkan mount yang sudah disiapkan (dalam bentuk komentar) di `03-agent-daemonset.yaml`, bagian `volumeMounts` dan `volumes`.

---

## 7. Operasi Harian

```bash
# Status ringkas
kubectl get pods -n kapture -o wide

# Log komponen Kapture sendiri
kubectl logs -n kapture deploy/kapture-aggregator --tail=100
kubectl logs -n kapture -l role=agent --tail=50 --prefix

# Restart setelah ganti konfigurasi
kubectl rollout restart deployment/kapture-aggregator -n kapture
kubectl rollout restart daemonset/kapture-agent -n kapture

# Kembali ke versi image sebelumnya
kubectl rollout undo deployment/kapture-aggregator -n kapture

# Akses API satu agent langsung (tanpa login), untuk pemeriksaan
kubectl port-forward -n kapture pod/<nama-pod-agent> 19489:19489
curl http://localhost:19489/healthz
curl 'http://localhost:19489/api/v1/storage'
```

**Backup rutin** lewat dashboard (menu **Storage** → *Download backup*), atau lewat API:

```bash
kubectl port-forward -n kapture svc/kapture 19488:19488 &
TOKEN=$(curl -s -X POST http://localhost:19488/api/v1/auth/login \
  -d '{"username":"<admin>","password":"<password>","code":"<6-digit-2FA>"}' \
  | sed -E 's/.*"token":"([^"]+)".*/\1/')
curl -H "Authorization: Bearer $TOKEN" -o kapture-backup.tar.gz \
  "http://localhost:19488/api/v1/storage/backup"
```

Berkas hasilnya bisa di-restore ke Kapture lain, misalnya yang dijalankan di laptop untuk investigasi offline.

---

## 8. Troubleshooting

| Gejala | Penyebab | Tindakan |
|---|---|---|
| `ImagePullBackOff` | Secret `gitlab-auth` belum ada, salah namespace, atau token kedaluwarsa | Buat ulang secret (bagian 2), lalu `kubectl rollout restart` |
| `ErrImagePull` + `manifest unknown` | Tag image belum ada di registry | Cek `docker pull <image>:<tag>` dari laptop, atau push ulang |
| Wizard setup muncul lagi padahal sudah pernah setup | Pod aggregator pindah node, `auth.json` tertinggal di node lama | Kunci node (bagian 4), lalu salin `auth.json` |
| Dashboard: *No agents discovered* | Service headless atau label agent tidak cocok | `kubectl get pods -n kapture -l app=kapture,role=agent` dan `kubectl get endpoints -n kapture kapture-agents` |
| Dashboard: *No logs collected yet* | Symlink log tidak ter-mount (khas runtime Docker) | Cek log agent: `cannot open log file (is its symlink target mounted?)`, aktifkan mount `/var/lib/docker/containers` |
| Pod aggregator restart terus | Liveness probe memakai endpoint yang butuh login | Pastikan probe memakai `/healthz`, bukan `/api/v1/health` |
| Agent `OOMKilled` | Limit memori terlalu kecil untuk beban node (mis. saat restore) | Naikkan `resources.limits.memory` di `03-agent-daemonset.yaml` |
| Jam log tidak cocok dengan aplikasi | `KAPTURE_TIMEZONE` berbeda antara agent dan aggregator | Samakan nilainya, lalu restart keduanya |

---

## 9. Menghapus Kapture

```bash
kubectl delete -f kapture/
# ClusterRole dan ClusterRoleBinding ikut terhapus karena ada di 01-rbac.yaml
```

Data log **tidak** ikut terhapus karena tersimpan di disk node. Bersihkan manual bila perlu:

```bash
# di setiap node
sudo rm -rf /var/lib/kapture
# di node aggregator (berisi akun dan 2FA)
sudo rm -rf /var/lib/kapture-aggregator
```
