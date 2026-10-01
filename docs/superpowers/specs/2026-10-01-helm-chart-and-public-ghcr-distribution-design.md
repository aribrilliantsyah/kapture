# Specification: Kapture Helm Chart & Automated Public Release via GHCR

- **Date:** 2026-10-01
- **Status:** Draft (Pending User Review)
- **Author:** Kapture Core Team
- **Topic:** Helm Packaging, Zero-Friction Installation, Automated GHCR Release Pipeline

---

## 1. Context and Goals

### 1.1 Context
Currently, deploying Kapture to Kubernetes requires:
1. Manually building the container image.
2. Pushing the image to a private GitLab container registry.
3. Manually setting up a Kubernetes `imagePullSecrets` (`gitlab-auth`) on the target cluster.
4. Applying raw manifests from `deploy/*.yaml`.

This process is slow, requires manual credential coordination, and prevents standard single-command installation workflows.

### 1.2 Goals
- **Zero-Friction Installation:** Allow users to install Kapture with a single Helm command and immediately access the dashboard via `kubectl port-forward svc/kapture 19488:19488`.
- **Public & Free Container Registry:** Publish multi-arch container images (`linux/amd64`, `linux/arm64`) to GitHub Container Registry (`ghcr.io/aribrilliantsyah/kapture`) completely free of charge with no download rate limits.
- **OCI Helm Chart Distribution:** Publish the packaged Helm chart directly to GHCR (`oci://ghcr.io/aribrilliantsyah/charts/kapture`) on each tag release.
- **Automated Release & Changelog:** Trigger full build, automated structured changelog generation (Conventional Commits), GitHub Release creation, container push, and Helm OCI push on every git tag (`v*.*.*`).
- **Clean Separation:** Keep existing raw manifests in `deploy/` untouched; place all Helm chart files inside `charts/kapture/`.

---

## 2. Helm Chart Design (`charts/kapture`)

### 2.1 File Hierarchy
```
charts/kapture/
├── Chart.yaml
├── values.yaml
├── .helmignore
└── templates/
    ├── _helpers.tpl
    ├── serviceaccount.yaml
    ├── rbac.yaml
    ├── secret.yaml
    ├── daemonset-agent.yaml
    ├── deployment-aggregator.yaml
    ├── service.yaml
    └── NOTES.txt
```

### 2.2 Chart Metadata (`Chart.yaml`)
- `apiVersion: v2`
- `name: kapture`
- `description: Ultra-lightweight persistent Kubernetes container log catcher and explorer`
- `type: application`
- `version: 1.0.5` (dynamically synced to git tag during release)
- `appVersion: "1.0.5"` (dynamically synced to git tag during release)
- `home: https://github.com/aribrilliantsyah/kapture`
- `sources: [https://github.com/aribrilliantsyah/kapture]`
- `keywords: [kubernetes, logging, logs, badgerdb, observability, tail]`

### 2.3 Configuration Schema (`values.yaml`)
```yaml
global:
  # Local timezone for daily log partitioning and dashboard display
  timezone: "Asia/Jakarta"

image:
  repository: ghcr.io/aribrilliantsyah/kapture
  pullPolicy: IfNotPresent
  # Overrides image tag whose default is the chart appVersion
  tag: ""

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  name: ""
  annotations: {}

rbac:
  create: true

auth:
  enabled: true
  # Initial dashboard credentials for setup wizard
  username: "admin"
  password: "admin123"
  # Optional: use existing secret instead of creating one
  existingSecret: ""

agent:
  enabled: true
  port: 19489
  retention: "0"       # 0 = retention managed by disk cap, or e.g. "30d", "168h"
  maxDisk: "5GB"
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 128Mi
  hostPaths:
    varlog: /var/log
    data: /var/lib/kapture
  tolerations:
    - operator: Exists

aggregator:
  replicas: 1
  port: 19488
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 500m
      memory: 256Mi
  persistence:
    # HostPath on node where aggregator runs to persist auth.json and 2FA credentials
    hostPath: /var/lib/kapture-aggregator
  nodeSelector: {}
  tolerations: []
  affinity: {}

service:
  type: ClusterIP
  port: 19488
  annotations: {}
```

### 2.4 Templates Breakdown
1. **`_helpers.tpl`**:
   - `kapture.name`, `kapture.fullname`, `kapture.chart`, `kapture.labels`, `kapture.selectorLabels`.
   - Dedicated selector labels for agent (`role: agent`) and aggregator (`role: aggregator`) matching Kapture's discovery contract (`KAPTURE_DISCOVERY_LABEL_SELECTOR`).
2. **`serviceaccount.yaml`**:
   - Standard ServiceAccount referenced by both Agent and Aggregator.
3. **`rbac.yaml`**:
   - `ClusterRole` granting read access to `pods`, `nodes`, `namespaces`, and controller owners (`apps/deployments`, `apps/daemonsets`, `apps/statefulsets`, `batch/jobs`, `batch/cronjobs`) needed for workload resolution and agent discovery.
   - `ClusterRoleBinding` linking the ServiceAccount.
4. **`secret.yaml`**:
   - Generates the `kapture-auth` secret containing `username` and `password` if `auth.existingSecret` is not provided.
5. **`daemonset-agent.yaml`**:
   - Deploys on every node (tolerating all taints).
   - Injects `KAPTURE_NODE_NAME` via `spec.nodeName` fieldRef.
   - Mounts `/var/log` (read-only) and `/var/lib/kapture` (BadgerDB).
   - Injects port, timezone, retention, and maxDisk configurations.
6. **`deployment-aggregator.yaml`**:
   - Injects discovery configuration: `kubernetes` discovery method, current release namespace, and label selector.
   - Injects auth credentials from Secret.
   - Mounts `/var/lib/kapture-aggregator` for persistent `auth.json` sessions and TOTP state.
7. **`service.yaml`**:
   - Exposes Aggregator dashboard on port `19488` via `ClusterIP`.
   - Exposes headless `kapture-agents` service for internal DNS-based agent discovery on port `19489`.
8. **`NOTES.txt`**:
   - Clear post-install message with zero cognitive load:
     ```
     ========================================================================
     🎉 Kapture is successfully installed!
     ========================================================================

     1. Connect to the Kapture Dashboard:
        $ kubectl port-forward svc/{{ include "kapture.fullname" . }} {{ .Values.service.port }}:{{ .Values.service.port }}

     2. Open your browser:
        http://localhost:{{ .Values.service.port }}

     3. Initial Credentials:
        Username : {{ .Values.auth.username }}
        Password : {{ .Values.auth.password }}

     To inspect running components:
        $ kubectl get pods -l {{ include "kapture.labels" . | nindent 4 }}
     ========================================================================
     ```

---

## 3. GitHub Actions Release Workflow (`.github/workflows/release.yaml`)

### 3.1 Trigger
- Triggered on tag push matching `v*.*.*` (e.g. `v1.0.6`, `v1.1.0`).

### 3.2 Pipeline Steps
1. **Checkout Code:** Full git history fetch for changelog generator (`fetch-depth: 0`).
2. **Setup QEMU & Docker Buildx:** For cross-platform binary and container builds (`linux/amd64`, `linux/arm64`).
3. **Login to GHCR:** Using `GITHUB_TOKEN` (`ghcr.io`).
4. **Extract Release Metadata:**
   - Parse tag name (e.g. `v1.1.0` -> version `1.1.0`).
   - Extract commit hash.
5. **Generate Structured Changelog:**
   - Automatically categorize commits since the previous tag into:
     - 🚀 **Features** (`feat:`)
     - 🐛 **Bug Fixes** (`fix:`)
     - 🧰 **Refactoring & Maintenance** (`refactor:`, `chore:`, `perf:`)
     - 📝 **Documentation** (`docs:`)
6. **Build & Push Multi-Arch Docker Image:**
   - Build using existing `Dockerfile` with `--build-arg VERSION=${VERSION} --build-arg COMMIT=${COMMIT}`.
   - Push to `ghcr.io/aribrilliantsyah/kapture:${TAG}` and `ghcr.io/aribrilliantsyah/kapture:latest`.
7. **Package & Push Helm Chart to OCI:**
   - Run `helm lint charts/kapture`.
   - Update `version` and `appVersion` in `Chart.yaml` to match release tag.
   - Package chart: `helm package charts/kapture --destination .dist`.
   - Push to OCI registry: `helm push .dist/kapture-*.tgz oci://ghcr.io/aribrilliantsyah/charts`.
8. **Create GitHub Release:**
   - Create official GitHub Release with tag name, attached Helm package artifact, and the generated structured changelog.

---

## 4. Documentation Reorganization & Dedicated Helm Guide

### 4.1 Root Directory Clean-up
To keep the repository root clean, all standalone markdown files except `README.md` (`prd.md`, `MYSTANDARD.md`, `CLAUDE.md`) will be moved into `docs/` (or symlinked if needed for CLI tooling). The root will only display `README.md`.

### 4.2 Dedicated Helm Guide (`docs/HELM_GUIDE.md`)
A comprehensive, user-friendly markdown guide will be created in `docs/HELM_GUIDE.md` covering:
- **Instant Quickstart:** Single command installation via OCI or local chart + port-forward instructions.
- **How to Release/Push to Helm:**
  - **Automated (Recommended):** How to trigger tag releases (`git tag v1.x.x && git push github v1.x.x`) to let GitHub Actions handle build, changelog, image push, and Helm OCI push.
  - **Manual Push:** Step-by-step CLI instructions using `helm package` and `helm push ... oci://ghcr.io/...` with `echo $GITHUB_TOKEN | helm registry login ghcr.io`.
- **Values Configuration Table:** Explaining every key parameter in `values.yaml` (timezone, auth, agent retention, resource limits, hostPaths).
- **Uninstall / Upgrade Commands:** Clean instructions for upgrading and uninstalling the Helm release.

---

## 5. Testing & Verification Plan

### 5.1 Local Chart Validation
- Run `helm lint charts/kapture` to verify syntax and standard compliance.
- Run `helm template test-release charts/kapture --debug` to verify all rendered manifests (DaemonSet, Deployment, Services, RBAC, Secret) match the validated manifests in `deploy/`.
- Test parameter overrides (e.g. `--set auth.password=customPass --set global.timezone=UTC`).

### 5.2 Workflow Syntax Verification
- Validate `.github/workflows/release.yaml` against GitHub Actions schema.
- Confirm correct token permissions (`contents: write`, `packages: write`).

### 5.3 Documentation & File Verification
- Verify root contains only `README.md` among markdown files.
- Verify `docs/HELM_GUIDE.md` contains accurate push, release, and installation instructions.

### 5.4 End-to-End User Experience Verification
- User runs:
  ```bash
  helm install kapture ./charts/kapture
  kubectl port-forward svc/kapture 19488:19488
  ```
- Browser opens `http://localhost:19488` with no `ImagePullBackOff` and zero manual configuration required.
