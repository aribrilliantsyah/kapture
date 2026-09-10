#!/usr/bin/env bash
# ==============================================================================
# Script Build & Push Image Kapture ke GitLab Container Registry
# Registry: registry.e-gitlab.prodak.id/bpd-diy1/va/kapture
# ==============================================================================

set -e

# ── Konfigurasi Registry ──
REGISTRY="registry.e-gitlab.prodak.id"
GROUP="bpd-diy1/va"
DEFAULT_IMAGE_NAME="kapture"
DEFAULT_USERNAME="aribrilliantsyah"

# Warna Output Terminal
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}====================================================${NC}"
echo -e "${CYAN}       Kapture — Container Build & Push Pipeline     ${NC}"
echo -e "${CYAN}====================================================${NC}"

# 1. Tentukan Versi Image
VERSION="$1"
if [ -z "$VERSION" ]; then
    read -r -p "Masukkan Versi Image (contoh: v1.0.0 atau 1.0.0): " VERSION
fi

if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Versi tidak boleh kosong!${NC}"
    exit 1
fi

# 2. Tentukan Nama Image (default: kapture)
IMAGE_NAME="$2"
if [ -z "$IMAGE_NAME" ]; then
    IMAGE_NAME="$DEFAULT_IMAGE_NAME"
fi

FULL_IMAGE_TAG="${REGISTRY}/${GROUP}/${IMAGE_NAME}:${VERSION}"
LATEST_IMAGE_TAG="${REGISTRY}/${GROUP}/${IMAGE_NAME}:latest"

echo -e "\n${BLUE}Target Image:${NC}"
echo -e "  • Image Tag  : ${GREEN}${FULL_IMAGE_TAG}${NC}"
echo -e "  • Latest Tag : ${GREEN}${LATEST_IMAGE_TAG}${NC}\n"

# 3. Autentikasi Docker Login ke GitLab Registry
USERNAME="${DOCKER_USERNAME:-$DEFAULT_USERNAME}"
PASSWORD="${DOCKER_PASSWORD:-$GITLAB_TOKEN}"

if [ -z "$PASSWORD" ]; then
    echo -e "${YELLOW}Autentikasi ke ${REGISTRY}...${NC}"
    read -r -s -p "Masukkan Password / Personal Access Token untuk user '${USERNAME}': " PASSWORD
    echo ""
fi

if [ -n "$PASSWORD" ]; then
    echo -e "${BLUE}Melakukan login ke ${REGISTRY}...${NC}"
    echo "$PASSWORD" | docker login "$REGISTRY" -u "$USERNAME" --password-stdin
    echo -e "${GREEN}✓ Login berhasil!${NC}\n"
else
    echo -e "${YELLOW}Password kosong, mencoba menggunakan sesi docker login yang sudah ada...${NC}\n"
fi

# 4. Build Docker Image
echo -e "${BLUE}Membangun Docker Image (${FULL_IMAGE_TAG})...${NC}"
docker build -t "$FULL_IMAGE_TAG" -t "$LATEST_IMAGE_TAG" .
echo -e "${GREEN}✓ Build image selesai!${NC}\n"

# 5. Push ke Container Registry
echo -e "${BLUE}Mengunggah image ke GitLab Container Registry...${NC}"
docker push "$FULL_IMAGE_TAG"
docker push "$LATEST_IMAGE_TAG"
echo -e "${GREEN}✓ Push ${FULL_IMAGE_TAG} berhasil!${NC}"
echo -e "${GREEN}✓ Push ${LATEST_IMAGE_TAG} berhasil!${NC}\n"

# 6. Opsi Otomatis Memperbarui Manifest Kubernetes
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

    echo -e "\n${CYAN}Terapkan manifest ke Kubernetes dengan perintah:${NC}"
    echo -e "  ${YELLOW}kubectl apply -f deploy/${NC}\n"
fi

echo -e "${GREEN}====================================================${NC}"
echo -e "${GREEN}             SEMUA PROSES SELESAI!                  ${NC}"
echo -e "${GREEN}====================================================${NC}"
