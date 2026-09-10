# Panduan Build & Push Container Image Kapture

Dokumen ini menjelaskan langkah-langkah membangun (*build*) image kontainer **Kapture** dan mengunggahnya (*push*) ke berbagai Container Registry publik maupun privat (Docker Hub, GitHub Container Registry/GHCR, Harbor, AWS ECR, GCP GAR, dll).

---

## 1. Konsep Dasar

Kapture mengadopsi arsitektur **Single Container Image**. Anda **tidak perlu** membuat dua image terpisah untuk Agent dan Aggregator.

```
                  ┌───────────────────────────────┐
                  │   Dockerfile (Go 1.26 Multi)  │
                  └───────────────┬───────────────┘
                                  │ docker build
                                  ▼
                  ┌───────────────────────────────┐
                  │       1 Image Kontainer       │
                  │   your-registry/kapture:v1.0  │
                  └───────────────┬───────────────┘
                                  │
         ┌────────────────────────┴────────────────────────┐
         │                                                 │
         ▼                                                 ▼
   Mode: Agent                                      Mode: Aggregator
   Argumen: --mode=agent                            Argumen: --mode=aggregator
   Deploy: DaemonSet (Tiap Node)                    Deploy: Deployment (1 Pod)
```

---

## 2. Prasyarat

Pastikan salah satu perkakas kontainer berikut telah terpasang di komputer/mesin CI Anda:
- **Docker Engine** (20.10+) atau **Docker Desktop**
- Atau **Podman** / **nerdctl**
- Akun dan akses ke Container Registry tujuan

---

## 3. Langkah Cepat (Quick Start Otomatis dengan Script)

Tersedia skrip otomatis yang sudah terintegrasi dengan GitLab Container Registry Anda:

```bash
# 1. Salin file konfigurasi kredensial (hanya sekali di awal)
cp scripts/registry.conf.example scripts/registry.conf

# 2. Buka scripts/registry.conf dan masukkan token GitLab Anda:
#    REGISTRY_TOKEN="glpat-xxxxxxxxxxxxxxxxxxxx"

# 3. Jalankan skrip build & push (otomatis login, build, tag, & push):
./scripts/build-and-push.sh v1.0.0
# atau: make docker-push VERSION=v1.0.0
```

> 🔒 Berkas `scripts/registry.conf` sudah didaftarkan di `.gitignore` sehingga token Anda tidak akan pernah ter-*commit* ke Git repository.

---

## 4. Langkah Manual (CLI Standar)

```bash
# 1. Masuk ke direktori project
cd /path/to/k8s-log-catcher

# 2. Build image lokal
docker build -t kapture:latest .

# 3. Beri tag sesuai registry tujuan
docker tag kapture:latest <NAMA_REGISTRY>/<USERNAME_ATAU_PROJECT>/kapture:latest

# 4. Push ke registry
docker push <NAMA_REGISTRY>/<USERNAME_ATAU_PROJECT>/kapture:latest
```

---

## 5. Panduan Berbagai Container Registry

### A. Docker Hub (`docker.io`)

1. **Login ke Docker Hub**:
   ```bash
   docker login -u <DOCKERHUB_USERNAME>
   # Masukkan password atau Personal Access Token (PAT) Anda
   ```

2. **Build dan Beri Tag**:
   ```bash
   docker build -t <DOCKERHUB_USERNAME>/kapture:latest .
   docker tag <DOCKERHUB_USERNAME>/kapture:latest <DOCKERHUB_USERNAME>/kapture:v1.0.0
   ```

3. **Push Image**:
   ```bash
   docker push <DOCKERHUB_USERNAME>/kapture:latest
   docker push <DOCKERHUB_USERNAME>/kapture:v1.0.0
   ```

---

### B. GitHub Container Registry (`ghcr.io`)

GitHub Packages / GHCR sangat direkomendasikan jika kode Anda disimpan di GitHub.

1. **Buat GitHub Personal Access Token (classic)**:
   - Buka: GitHub Settings → Developer Settings → Personal Access Tokens → Tokens (classic).
   - Centang hak akses: `write:packages`, `read:packages`, `delete:packages`.
   - Simpan token tersebut.

2. **Login ke GHCR**:
   ```bash
   echo $CR_PAT | docker login ghcr.io -u <GITHUB_USERNAME> --password-stdin
   ```

3. **Build & Tag**:
   ```bash
   # Catatan: Nama username harus huruf kecil semua (lowercase)
   docker build -t ghcr.io/<github_username>/kapture:latest .
   docker tag ghcr.io/<github_username>/kapture:latest ghcr.io/<github_username>/kapture:v1.0.0
   ```

4. **Push ke GHCR**:
   ```bash
   docker push ghcr.io/<github_username>/kapture:latest
   docker push ghcr.io/<github_username>/kapture:v1.0.0
   ```

5. *(Opsional)* Jadikan paket publik di GitHub agar cluster Kubernetes dapat mengunduhnya tanpa *imagePullSecrets*:
   - Buka repo GitHub Anda → Tab **Packages** → Pilih `kapture`.
   - Masuk ke **Package settings** → Ubah visibilitas ke **Public**.

---

### C. Self-Hosted Registry (Harbor / Nexus / GitLab)

Jika kantor atau server Anda menggunakan private registry lokal (misal: Harbor atau GitLab):

1. **Login ke Private Registry**:
   ```bash
   docker login registry.perusahaan.com -u <USERNAME>
   ```

2. **Build & Tag**:
   ```bash
   docker build -t registry.perusahaan.com/devops/kapture:v1.0.0 .
   ```

3. **Push**:
   ```bash
   docker push registry.perusahaan.com/devops/kapture:v1.0.0
   ```

---

### D. AWS Elastic Container Registry (ECR)

1. **Autentikasi Docker ke Amazon ECR**:
   ```bash
   aws ecr get-login-password --region <AWS_REGION> | \
     docker login --username AWS --password-stdin <ACCOUNT_ID>.dkr.ecr.<AWS_REGION>.amazonaws.com
   ```

2. **Buat Repository (jika belum ada)**:
   ```bash
   aws ecr create-repository --repository-name kapture --region <AWS_REGION>
   ```

3. **Build, Tag & Push**:
   ```bash
   docker build -t kapture:latest .
   docker tag kapture:latest <ACCOUNT_ID>.dkr.ecr.<AWS_REGION>.amazonaws.com/kapture:v1.0.0
   docker push <ACCOUNT_ID>.dkr.ecr.<AWS_REGION>.amazonaws.com/kapture:v1.0.0
   ```

---

### E. Google Artifact Registry (GAR)

1. **Konfigurasi Autentikasi**:
   ```bash
   gcloud auth configure-docker <REGION>-docker.pkg.dev
   ```

2. **Build & Push**:
   ```bash
   docker build -t <REGION>-docker.pkg.dev/<PROJECT_ID>/<REPO_NAME>/kapture:v1.0.0 .
   docker push <REGION>-docker.pkg.dev/<PROJECT_ID>/<REPO_NAME>/kapture:v1.0.0
   ```

---

## 5. Membangun Multi-Architecture Image (`amd64` & `arm64`)

Jika cluster Anda memiliki campuran arsitektur (misalnya: node x86_64 dan AWS Graviton ARM64 / Raspberry Pi / Apple Silicon), gunakan **Docker Buildx** agar satu image bisa berjalan di arsitektur apa pun tanpa error `exec format error`.

```bash
# 1. Siapkan builder instance buildx
docker buildx create --name kapture-builder --use
docker buildx inspect --bootstrap

# 2. Build dan langsung push multi-arch image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t <REGISTRY_URL>/<PROJECT>/kapture:latest \
  -t <REGISTRY_URL>/<PROJECT>/kapture:v1.0.0 \
  --push .
```

---

## 6. Konfigurasi Kubernetes dengan Private Registry

Jika image diunggah ke registry privat, pod Kubernetes memerlukan **ImagePullSecrets** untuk mengunduh image tersebut.

### Langkah 1: Buat Secret Kredensial Registry

Jalankan perintah berikut di cluster Kubernetes:

```bash
kubectl create secret docker-registry kapture-regcred \
  --namespace=log-catcher \
  --docker-server=<NAMA_REGISTRY> \
  --docker-username=<USERNAME> \
  --docker-password=<PASSWORD_ATAU_TOKEN> \
  --docker-email=<EMAIL>
```

### Langkah 2: Tambahkan ke Manifest (`deploy/03-agent-daemonset.yaml` & `deploy/04-aggregator-deployment.yaml`)

Tambahkan blok `imagePullSecrets` pada bagian `spec.template.spec` di DaemonSet dan Deployment:

```yaml
spec:
  template:
    spec:
      serviceAccountName: k8s-log-catcher
      imagePullSecrets:
        - name: kapture-regcred    # <-- Tambahkan ini
      containers:
        - name: agent
          image: registry.perusahaan.com/devops/kapture:v1.0.0
```

---

## 7. Pemecahan Masalah (Troubleshooting)

| Gejala Masalah | Penyebab Umum | Solusi |
|---|---|---|
| `ImagePullBackOff` / `ErrImagePull` | Nama image atau tag salah ketik, atau image bersifat privat tanpa `imagePullSecrets` | Periksa nama image di Docker Hub/GHCR. Jika privat, buat `imagePullSecrets` sesuai Bab 6. |
| `exec format error` pada pod | Arsitektur image tidak cocok dengan CPU node (misal image arm64 dijalankan di host x86_64) | Gunakan panduan **Multi-Arch Build** pada Bab 5 dengan flag `--platform linux/amd64,linux/arm64`. |
| `denied: requested access to the resource is denied` | Token expired atau hak akses *write* tidak ada saat `docker push` | Lakukan login ulang `docker login` dengan token yang memiliki izin *write*. |
| Ukuran Image Terlalu Besar | Tidak menggunakan multi-stage build | `Dockerfile` Kapture sudah menggunakan multi-stage build Alpine murni sehingga ukuran image final hanya **~15 MB**. |
