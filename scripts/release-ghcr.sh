#!/usr/bin/env bash
# ==============================================================================
# Script Rilis Kapture ke GitHub Container Registry (GHCR) & Helm OCI
# ==============================================================================

set -e

# Warna Output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

GH_REGISTRY="ghcr.io"
GH_USER="${GH_USER:-${GITHUB_USER:-aribrilliantsyah}}"
IMAGE_NAME="kapture"

echo -e "${CYAN}====================================================${NC}"
echo -e "${CYAN}     Kapture — Release Pipeline (GHCR & Helm OCI)   ${NC}"
echo -e "${CYAN}====================================================${NC}"

# ── 1. Input Versi ──
INPUT_VERSION="$1"
if [ -z "$INPUT_VERSION" ]; then
    read -r -p "Masukkan versi rilis (contoh: 0.0.1 atau v0.0.1): " INPUT_VERSION
fi

if [ -z "$INPUT_VERSION" ]; then
    echo -e "${RED}Error: Versi tidak boleh kosong!${NC}"
    exit 1
fi

# Normalisasi versi:
# APP_VERSION (mis. v0.0.1) untuk binary, docker tag, dan appVersion
# HELM_VERSION (mis. 0.0.1, SemVer murni tanpa 'v') untuk Chart.yaml version
if [[ "$INPUT_VERSION" =~ ^v(.*) ]]; then
    HELM_VERSION="${BASH_REMATCH[1]}"
    APP_VERSION="$INPUT_VERSION"
else
    HELM_VERSION="$INPUT_VERSION"
    APP_VERSION="v$INPUT_VERSION"
fi

FULL_IMAGE_TAG="${GH_REGISTRY}/${GH_USER}/${IMAGE_NAME}:${APP_VERSION}"
LATEST_IMAGE_TAG="${GH_REGISTRY}/${GH_USER}/${IMAGE_NAME}:latest"
CHART_DIR="charts/kapture"

echo -e "\n${BLUE}Target Rilis:${NC}"
echo -e "  • User / Org   : ${CYAN}${GH_USER}${NC}"
echo -e "  • App Version  : ${GREEN}${APP_VERSION}${NC} (muncul di dashboard & docker tag)"
echo -e "  • Helm Version : ${GREEN}${HELM_VERSION}${NC} (SemVer chart)"
echo -e "  • Docker Image : ${GREEN}${FULL_IMAGE_TAG}${NC}"
echo -e "  • Helm OCI     : ${GREEN}oci://${GH_REGISTRY}/${GH_USER}/charts${NC}\n"

# ── 2. Autentikasi GHCR (Docker & Helm) ──
TOKEN="${GH_TOKEN:-${GITHUB_TOKEN}}"

if [ -z "$TOKEN" ]; then
    echo -e "${YELLOW}ℹ GH_TOKEN / GITHUB_TOKEN belum di-set.${NC}"
    echo -e "${YELLOW}  Jika Anda belum login ke ghcr.io, masukkan GitHub PAT (Personal Access Token).${NC}"
    read -r -s -p "Masukkan GitHub Token (kosongkan jika sudah login via docker): " TOKEN
    echo ""
fi

if [ -n "$TOKEN" ]; then
    echo -e "${BLUE}Melakukan login ke ${GH_REGISTRY}...${NC}"
    echo "$TOKEN" | docker login "$GH_REGISTRY" -u "$GH_USER" --password-stdin
    echo "$TOKEN" | helm registry login "$GH_REGISTRY" -u "$GH_USER" --password-stdin
    echo -e "${GREEN}✓ Login GHCR berhasil!${NC}\n"
else
    echo -e "${YELLOW}Mencoba menggunakan sesi login docker & helm yang sudah aktif...${NC}\n"
fi

# ── 3. Build & Tag Docker Image ──
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
echo -e "${BLUE}Membangun Docker Image (${FULL_IMAGE_TAG})...${NC}"
docker build \
    --build-arg VERSION="$APP_VERSION" \
    --build-arg COMMIT="$COMMIT" \
    -t "$FULL_IMAGE_TAG" \
    -t "$LATEST_IMAGE_TAG" .

echo -e "${GREEN}✓ Docker build selesai!${NC}\n"

# ── 4. Push Docker Image ke GHCR ──
echo -e "${BLUE}Mengunggah Docker image ke GHCR...${NC}"
docker push "$FULL_IMAGE_TAG"
docker push "$LATEST_IMAGE_TAG"
echo -e "${GREEN}✓ Docker image berhasil di-push!${NC}\n"

# ── 5. Sinkronisasi Versi di Helm Chart & Manifest Standalone ──
echo -e "${BLUE}Menyinkronkan versi di Helm Chart & deploy/install.yaml...${NC}"

# Update Chart.yaml
sed -i -E "s/^version: .*/version: ${HELM_VERSION}/" "${CHART_DIR}/Chart.yaml"
sed -i -E "s/^appVersion: .*/appVersion: \"${APP_VERSION}\"/" "${CHART_DIR}/Chart.yaml"

# Update deploy/install.yaml
if [ -f "deploy/install.yaml" ]; then
    sed -i -E "s|image: ghcr.io/${GH_USER}/kapture:[^ ]+|image: ghcr.io/${GH_USER}/kapture:${APP_VERSION}|g" deploy/install.yaml
fi

# Validasi Chart
helm lint "${CHART_DIR}"

# ── 6. Package & Push Helm Chart ke GHCR OCI ──
PKG_TMP="$(mktemp -d)"
trap 'rm -rf "$PKG_TMP"' EXIT

echo -e "${BLUE}Mem-package Helm chart...${NC}"
helm package "${CHART_DIR}" -d "$PKG_TMP"

echo -e "${BLUE}Mengunggah Helm chart ke GHCR OCI...${NC}"
helm push "${PKG_TMP}/kapture-${HELM_VERSION}.tgz" "oci://${GH_REGISTRY}/${GH_USER}/charts"
echo -e "${GREEN}✓ Helm chart berhasil di-push ke GHCR OCI!${NC}\n"

echo -e "${GREEN}====================================================${NC}"
echo -e "${GREEN}  🎉 Rilis ${APP_VERSION} Berhasil Diunggah ke GHCR!${NC}"
echo -e "${GREEN}====================================================${NC}"
echo -e "${CYAN}Perintah Instalasi Publik:${NC}"
echo -e "  ${YELLOW}# Helm:${NC}"
echo -e "  helm install kapture oci://${GH_REGISTRY}/${GH_USER}/charts/kapture \\"
echo -e "    --version ${HELM_VERSION} \\"
echo -e "    --namespace kapture \\"
echo -e "    --create-namespace"
echo -e ""
echo -e "  ${YELLOW}# Manifest Standalone:${NC}"
echo -e "  kubectl apply -f https://raw.githubusercontent.com/${GH_USER}/kapture/main/deploy/install.yaml"
echo -e ""
echo -e "${YELLOW}Catatan Penting:${NC}"
echo -e "Pastikan kedua package di GitHub (Container & Helm Chart) diatur ke ${GREEN}Public${NC} agar"
echo -e "dapat di-install di server tanpa perlu membuat secret kredensial apa pun."
