#!/usr/bin/env bash
# ==============================================================================
# Script Build & Push Image Kapture ke GitLab Container Registry
# ==============================================================================

set -e

# Warna Output Terminal
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ── 1. Muat Konfigurasi dari File 'registry.conf' jika ada ──
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONF_FILE="${SCRIPT_DIR}/registry.conf"
ROOT_CONF="${SCRIPT_DIR}/../.registry.conf"

LOADED_FROM_CONF=false
if [ -f "$CONF_FILE" ]; then
    # shellcheck disable=SC1090
    source "$CONF_FILE"
    LOADED_FROM_CONF=true
elif [ -f "$ROOT_CONF" ]; then
    # shellcheck disable=SC1090
    source "$ROOT_CONF"
    LOADED_FROM_CONF=true
fi

# Nilai bawaan jika tidak ditentukan di file config / env
REGISTRY="${REGISTRY:-registry.e-gitlab.prodak.id}"
GROUP="${GROUP:-bpd-diy1/va}"
DEFAULT_IMAGE_NAME="${IMAGE_NAME:-kapture}"
DEFAULT_USERNAME="${REGISTRY_USERNAME:-aribrilliantsyah}"
TOKEN="${REGISTRY_TOKEN:-${GITLAB_TOKEN:-$DOCKER_PASSWORD}}"

echo -e "${CYAN}====================================================${NC}"
echo -e "${CYAN}       Kapture — Container Build & Push Pipeline     ${NC}"
echo -e "${CYAN}====================================================${NC}"

if [ "$LOADED_FROM_CONF" = true ]; then
    echo -e "${GREEN}✓ Konfigurasi dimuat dari file 'registry.conf'${NC}"
else
    echo -e "${YELLOW}ℹ Tip: Buat 'scripts/registry.conf' dari 'scripts/registry.conf.example'${NC}"
    echo -e "${YELLOW}  agar Anda tidak perlu memasukkan username & token setiap kali push.${NC}"
fi

# ── 2. Tentukan Versi Image ──
VERSION="$1"
if [ -z "$VERSION" ]; then
    read -r -p "Masukkan Versi Image (contoh: v1.0.0 atau 1.0.0): " VERSION
fi

if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Versi tidak boleh kosong!${NC}"
    exit 1
fi

# ── 3. Tentukan Nama Image ──
CUSTOM_IMAGE="$2"
if [ -n "$CUSTOM_IMAGE" ]; then
    IMAGE_NAME="$CUSTOM_IMAGE"
else
    IMAGE_NAME="$DEFAULT_IMAGE_NAME"
fi

FULL_IMAGE_TAG="${REGISTRY}/${GROUP}/${IMAGE_NAME}:${VERSION}"
LATEST_IMAGE_TAG="${REGISTRY}/${GROUP}/${IMAGE_NAME}:latest"

echo -e "\n${BLUE}Target Image:${NC}"
echo -e "  • Image Tag  : ${GREEN}${FULL_IMAGE_TAG}${NC}"
echo -e "  • Latest Tag : ${GREEN}${LATEST_IMAGE_TAG}${NC}\n"

# ── 4. Autentikasi Docker Login ──
USERNAME="${REGISTRY_USERNAME:-${DOCKER_USERNAME:-$DEFAULT_USERNAME}}"

if [ -z "$TOKEN" ]; then
    echo -e "${YELLOW}Autentikasi ke ${REGISTRY}...${NC}"
    read -r -s -p "Masukkan Password / Access Token untuk user '${USERNAME}': " TOKEN
    echo ""
fi

if [ -n "$TOKEN" ]; then
    echo -e "${BLUE}Melakukan login ke ${REGISTRY}...${NC}"
    echo "$TOKEN" | docker login "$REGISTRY" -u "$USERNAME" --password-stdin
    echo -e "${GREEN}✓ Login berhasil!${NC}\n"
else
    echo -e "${YELLOW}Token kosong, mencoba menggunakan sesi docker login yang sudah ada...${NC}\n"
fi

# ── 5. Build Docker Image ──
echo -e "${BLUE}Membangun Docker Image (${FULL_IMAGE_TAG})...${NC}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || true)"
docker build --build-arg VERSION="$VERSION" --build-arg COMMIT="$COMMIT" -t "$FULL_IMAGE_TAG" -t "$LATEST_IMAGE_TAG" .
echo -e "${GREEN}✓ Build image selesai!${NC}\n"

# ── 6. Push ke Container Registry ──
echo -e "${BLUE}Mengunggah image ke GitLab Container Registry...${NC}"
docker push "$FULL_IMAGE_TAG"
docker push "$LATEST_IMAGE_TAG"
echo -e "${GREEN}✓ Push ${FULL_IMAGE_TAG} berhasil!${NC}"
echo -e "${GREEN}✓ Push ${LATEST_IMAGE_TAG} berhasil!${NC}\n"

# ── 7. Opsi Otomatis Memperbarui Manifest Kubernetes ──
read -r -p "Apakah Anda ingin memperbarui image tag di folder deploy/ sekarang? (y/N): " UPDATE_MANIFESTS
if [[ "$UPDATE_MANIFESTS" =~ ^[Yy]$ ]]; then
    AGENT_FILE="deploy/03-agent-daemonset.yaml"
    AGG_FILE="deploy/04-aggregator-deployment.yaml"

    if [ -f "$AGENT_FILE" ]; then
        sed -i -E "s|image: .*|image: ${FULL_IMAGE_TAG}|g" "$AGENT_FILE"
        echo -e "${GREEN}✓ Berhasil update ${AGENT_FILE}${NC}"
    fi

    if [ -f "$AGG_FILE" ]; then
        sed -i -E "s|image: .*|image: ${FULL_IMAGE_TAG}|g" "$AGG_FILE"
        echo -e "${GREEN}✓ Berhasil update ${AGG_FILE}${NC}"
    fi

    # Secret imagePullSecrets 'gitlab-auth' dikelola terpisah di cluster,
    # skrip ini tidak membuat atau menimpanya.

    echo -e "\n${CYAN}Terapkan manifest ke Kubernetes dengan perintah:${NC}"
    echo -e "  ${YELLOW}kubectl apply -f deploy/${NC}\n"
fi

echo -e "${GREEN}====================================================${NC}"
echo -e "${GREEN}             SEMUA PROSES SELESAI!                  ${NC}"
echo -e "${GREEN}====================================================${NC}"
