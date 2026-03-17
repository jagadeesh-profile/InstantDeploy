# InstantDeploy — Product Requirements Document (PRD)

**Version**: 1.0
**Date**: 2026-03-16
**Status**: Draft
**Target Platform**: Docker Desktop + Kubernetes (local-first, cloud-ready)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [Goals & Non-Goals](#3-goals--non-goals)
4. [Target Users](#4-target-users)
5. [Architecture Overview](#5-architecture-overview)
6. [Architecture Issues & Remediation Plan](#6-architecture-issues--remediation-plan)
7. [Functional Requirements](#7-functional-requirements)
8. [Non-Functional Requirements](#8-non-functional-requirements)
9. [Docker Desktop Integration](#9-docker-desktop-integration)
10. [Kubernetes Integration](#10-kubernetes-integration)
11. [Security Requirements](#11-security-requirements)
12. [Data Model](#12-data-model)
13. [API Specification](#13-api-specification)
14. [Frontend Requirements](#14-frontend-requirements)
15. [Monitoring & Observability](#15-monitoring--observability)
16. [Deployment Strategy](#16-deployment-strategy)
17. [Release Milestones](#17-release-milestones)
18. [Open Questions](#18-open-questions)

---

## 1. Executive Summary

**InstantDeploy** is a developer-facing platform that lets users deploy any public or private GitHub repository to a local containerized environment in seconds — no manual Dockerfile writing, no CLI commands, no configuration required. The user pastes a repo URL, selects a branch, and the system automatically detects the runtime, generates an optimized Dockerfile, builds the image, runs it, and streams real-time logs to the browser.

**Initial target runtime**: Docker Desktop (local) with Kubernetes via Docker Desktop's built-in K8s cluster. This enables zero-infrastructure-cost development and testing before scaling to cloud Kubernetes.

---

## 2. Problem Statement

| Pain Point | Current State | Impact |
|---|---|---|
| Manual Dockerization | Developers write Dockerfiles from scratch for every project | 30–60 min per project |
| No local K8s preview | Kubernetes testing requires cloud infra or complex local setup | High friction for dev iteration |
| Build feedback loops | Shell scripts, CI logs hidden behind external services | Slow debugging |
| Onboarding overhead | New devs need to understand container runtimes before running anything | Days of setup time |
| Port conflict management | Multiple local services conflict on ports | Runtime errors, developer frustration |

InstantDeploy eliminates all five pain points with a single-page UI.

---

## 3. Goals & Non-Goals

### Goals
- **G1**: Deploy any GitHub repo to Docker Desktop in < 90 seconds end-to-end
- **G2**: Automatically detect runtime (Node, Go, Python, Java, Rust, PHP, Ruby, .NET, Static)
- **G3**: Stream build logs in real-time via WebSocket
- **G4**: Support Docker Desktop's built-in Kubernetes for pod-based deployments
- **G5**: Provide deployment history, status tracking, and one-click teardown
- **G6**: Full user authentication with role-based access (developer / admin)
- **G7**: Expose Prometheus metrics and Grafana dashboards for local observability

### Non-Goals
- Cloud Kubernetes (EKS, GKE, AKS) — out of scope for v1.0
- CI/CD pipeline integration (GitHub Actions, etc.) — v2.0
- Multi-tenant SaaS — v2.0
- Private registry support (ECR, GCR) — v2.0
- Rolling deployments / canary releases — v2.0

---

## 4. Target Users

### Primary: Individual Developer (Local Use)
- Wants to test a GitHub project quickly without reading docs
- Uses Docker Desktop daily
- Comfortable with CLI but prefers UI for deployment management

### Secondary: Team Lead / DevOps (Local Demo)
- Demonstrates deploys to non-technical stakeholders
- Needs stable local environment with reproducible builds
- Wants monitoring baked in

### Tertiary: Student / OSS Contributor
- Evaluating a project quickly
- No cloud account, limited resources
- Needs zero-config onboarding

---

## 5. Architecture Overview

```
┌────────────────────────────────────────────────────────────────────┐
│                        USER BROWSER                                 │
│  React 18 + TypeScript + Tailwind + Zustand + Axios + WS           │
└────────────────────────┬───────────────────────────────────────────┘
                         │ REST + WebSocket (ws://)
                         ▼
┌────────────────────────────────────────────────────────────────────┐
│                     FRONTEND CONTAINER                              │
│  Nginx (port 80/3000)                                              │
│  • Serves SPA bundle                                               │
│  • Proxies /api → backend                                          │
│  • Proxies /ws → backend                                           │
└────────────────────────┬───────────────────────────────────────────┘
                         │
                         ▼
┌────────────────────────────────────────────────────────────────────┐
│                      BACKEND (Go 1.22)   port 8080                 │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐  │
│  │  Chi Router  │  │  JWT Auth    │  │  WebSocket Hub          │  │
│  │  + Middleware│  │  (HS256)     │  │  (Gorilla WS)           │  │
│  └──────────────┘  └──────────────┘  └─────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    RUNTIME MANAGER                           │   │
│  │  Smart Detector → Dockerfile Generator → Build Fixer        │   │
│  │  Worker Pool (3) → Git Clone → Docker Build → Docker Run    │   │
│  │  Event Publisher → WebSocket Broadcast                      │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐  │
│  │  Postgres    │  │  Redis       │  │  Prometheus Metrics     │  │
│  │  Store       │  │  Queue       │  │  /metrics endpoint      │  │
│  └──────────────┘  └──────────────┘  └─────────────────────────┘  │
└────────────┬──────────────────────────────────────────────────────-┘
             │ docker.sock (Unix socket)
             ▼
┌────────────────────────────────────────────────────────────────────┐
│            DOCKER DESKTOP RUNTIME                                   │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Docker Engine (dockerd)                                      │  │
│  │  • Build images from generated Dockerfiles                   │  │
│  │  • Run containers with auto port assignment                  │  │
│  │  • Container lifecycle management                            │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Kubernetes (Docker Desktop built-in)                         │  │
│  │  • Namespace: instantdeploy                                   │  │
│  │  • Deployments for backend, frontend, postgres, redis        │  │
│  │  • NodePort services for external access                     │  │
│  │  • PVCs for postgres data                                    │  │
│  │  • ConfigMaps + Secrets                                      │  │
│  └──────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────┘
             │
             ▼
┌────────────────────────────────────────────────────────────────────┐
│                   DATASTORES & OBSERVABILITY                        │
│  PostgreSQL 16 (port 5432) │ Redis 7 (port 6379)                   │
│  Prometheus (port 9090)    │ Grafana (port 3001)                    │
└────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Technology | Responsibility |
|---|---|---|
| Frontend | React 18, Vite, Tailwind, Zustand | UI, state, WebSocket client |
| Backend | Go 1.22, Chi, pgx, Redis | API, build orchestration, auth |
| Runtime Manager | Go, Docker CLI/SDK | Smart detect, build, run |
| PostgreSQL | postgres:16-alpine | Deployments, users, logs |
| Redis | redis:7-alpine | Durable build queue, caching |
| Nginx (frontend) | nginx:1.27-alpine | SPA serving, API reverse proxy |
| Prometheus | prom/prometheus | Metrics scraping |
| Grafana | grafana/grafana | Dashboards, alerting |

---

## 6. Architecture Issues & Remediation Plan

### CRITICAL — Must fix before any shared use

#### C1: No User Isolation on Deployments
- **Issue**: The `deployments` table has no `user_id` column. All users can see all deployments.
- **Fix**: Add `user_id TEXT NOT NULL` to deployments schema. Add FK to users. Filter all queries by JWT-extracted user_id.
- **Files**: `internal/database/deployment_store.go`, `internal/api/handlers.go`

#### C2: WebSocket Subscription Not Validated Against JWT
- **Issue**: User subscribes to events using a `user_id` query param — never validated against the JWT.
- **Fix**: Extract `user_id` from JWT claims in `internal/websocket/handler.go`. Reject mismatched subscriptions.
- **Files**: `internal/websocket/handler.go`

#### C3: No Input Validation on Repository URL
- **Issue**: Repo URL passed directly to `git clone` without validation.
- **Risk**: Command injection, SSRF via file:// or git:// URLs.
- **Fix**: Validate URL via regex `^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`. Reject all other schemes.
- **Files**: `internal/api/handlers.go` (CreateDeployment), `internal/runtime/manager.go`

#### C4: Weak Default JWT Secret
- **Issue**: Default JWT secret is `"instantdeploy-dev-secret"` — committed to source.
- **Fix**: Generate 256-bit random secret at first run if not set. Fail fast if `ENV=production` and secret matches default.
- **Files**: `pkg/utils/config.go`

#### C5: Hardcoded Demo User in Binary
- **Issue**: `Demo123!` password and demo user setup is hardcoded in `handlers.go`.
- **Fix**: Remove entirely. Use database seeding script controlled by `SEED_DEMO_USER=true` env var (development only).
- **Files**: `internal/api/handlers.go`

#### C6: Missing Password Reset Code Server Validation
- **Issue**: Password reset does not validate the reset code server-side.
- **Fix**: Store reset codes in `users.verification_code` with expiry. Validate code + expiry before allowing password change.
- **Files**: `internal/api/handlers.go`, `internal/database/user_store.go`

---

### HIGH — Fix before team use

#### H1: Docker Socket Mounted Without Restrictions
- **Issue**: Backend container runs with full Docker socket access (`/var/run/docker.sock`). Any compromise = full host access.
- **Fix (v1)**: Run backend container with a read-only rootfs except `/tmp/builds`. Use Docker GID group (999), never root.
- **Fix (v2)**: Replace socket mount with Docker-in-Docker sidecar or rootless Docker.
- **Files**: `Dockerfile.backend`, `docker-compose.isolated.yml`, `infrastructure/k8s/backend/deployment.yaml`

#### H2: No CORS Origin Restriction
- **Issue**: CORS likely allows `*`. Any site can make credentialed requests.
- **Fix**: Set `ALLOWED_ORIGINS` env var. In Chi middleware, validate `Origin` header against allowlist.
- **Files**: `pkg/utils/http.go`, `internal/api/router.go`

#### H3: No Rate Limiting on Auth Endpoints
- **Issue**: Login endpoint can be brute-forced. DB-level `failed_attempts` is not enforced consistently.
- **Fix**: Add sliding-window rate limiter (10 attempts / 10 min per IP). Use Redis for rate limit state.
- **Files**: `internal/api/router.go`, new middleware file

#### H4: Secrets in Docker Compose Plaintext
- **Issue**: `JWT_SECRET`, `DATABASE_URL`, Redis URL all hardcoded in `docker-compose.isolated.yml`.
- **Fix**: Move all secrets to `.env` file (gitignored). Reference via `env_file:` in compose. Document in `.env.example`.
- **Files**: `docker-compose.isolated.yml`

#### H5: No Graceful Shutdown
- **Issue**: `main.go` has no SIGTERM handler. In-flight builds are killed abruptly on pod restart.
- **Fix**: Register `os.Signal` listener. Drain worker queue, wait for active builds to finish (up to 30s), then shutdown.
- **Files**: `cmd/server/main.go`

#### H6: No Structured Logging
- **Issue**: `log.Printf` scattered throughout. No log levels, correlation IDs, or JSON output.
- **Fix**: Integrate `log/slog` (stdlib, Go 1.21+). Add request ID middleware. Log all builds, auth events, errors as structured JSON.
- **All backend files**

---

### MEDIUM — Fix in v1.1

#### M1: No Audit Trail
- **Issue**: No log of who deployed what, when, with what result.
- **Fix**: Emit audit log entries to a dedicated `audit_logs` table on create/delete deployment, login, password change.

#### M2: Nginx Missing Security Headers
- **Issue**: No `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`.
- **Fix**: Add standard security headers block to `nginx.conf`.
- **Files**: `frontend/nginx.conf`

#### M3: No Database Connection Pool Limits
- **Issue**: `pgxpool.New()` uses defaults; can exhaust PostgreSQL connections under load.
- **Fix**: Set `MaxConns: 20`, `MinConns: 2`, `MaxConnIdleTime: 5m`.
- **Files**: `internal/database/postgres.go`

#### M4: No Orphaned Container Cleanup on Startup
- **Issue**: If the backend crashes during a build, Docker containers keep running with no record.
- **Fix**: On startup, query Docker for containers with label `instantdeploy=true` and reconcile against DB.
- **Files**: `internal/runtime/manager.go`

#### M5: WebSocket Uses `ws://` Not `wss://`
- **Issue**: Frontend hardcodes `ws://` protocol.
- **Fix**: Detect `window.location.protocol` and use `wss://` when on HTTPS.
- **Files**: `frontend/src/services/websocket.ts`

#### M6: No Request ID Tracing
- **Issue**: Middleware generates RequestID but doesn't propagate it to logs.
- **Fix**: Pass request ID via context; include in all log entries.
- **Files**: `internal/api/router.go`

#### M7: Race Condition in Cleanup Loop
- **Issue**: `cleanupInactiveLoop` and explicit delete can run concurrently on the same deployment.
- **Fix**: Use database-level `SELECT FOR UPDATE` or a soft-delete pattern with a `deleted_at` column.
- **Files**: `internal/runtime/manager.go`

---

### LOW — Backlog

| ID | Issue | Fix |
|---|---|---|
| L1 | No environment variable validation on startup | Add `validateConfig()` in `pkg/utils/config.go` |
| L2 | No separate K8s liveness vs readiness probes | Add `/health/ready` and `/health/live` |
| L3 | Frontend GitHub token could leak if stored client-side | Always proxy GitHub API through backend |
| L4 | No API versioning migration strategy | Document in CONTRIBUTING.md |
| L5 | Build errors may expose sensitive paths/keys | Sanitize `docker build` output before broadcasting |

---

## 7. Functional Requirements

### 7.1 Authentication

| ID | Requirement | Priority |
|---|---|---|
| FR-AUTH-1 | Users register with username, email, password | P0 |
| FR-AUTH-2 | Email verification via 6-digit code | P0 |
| FR-AUTH-3 | Login returns signed JWT (HS256, 120min default) | P0 |
| FR-AUTH-4 | Password reset via email verification code (server-validated, 15min TTL) | P0 |
| FR-AUTH-5 | Account lockout after 5 failed login attempts (exponential backoff) | P1 |
| FR-AUTH-6 | Role-based access: `developer` and `admin` | P1 |
| FR-AUTH-7 | JWT refresh endpoint | P2 |

### 7.2 Deployment Management

| ID | Requirement | Priority |
|---|---|---|
| FR-DEP-1 | Create deployment by providing GitHub URL + branch | P0 |
| FR-DEP-2 | Auto-detect project runtime (Node, Go, Python, Java, Rust, PHP, Ruby, .NET, Static) | P0 |
| FR-DEP-3 | Auto-generate optimized Dockerfile based on detected runtime | P0 |
| FR-DEP-4 | Build and run container via Docker Desktop socket | P0 |
| FR-DEP-5 | Stream real-time build logs via WebSocket | P0 |
| FR-DEP-6 | List all deployments for the authenticated user | P0 |
| FR-DEP-7 | Delete deployment and stop/remove associated container | P0 |
| FR-DEP-8 | View historical build logs per deployment | P0 |
| FR-DEP-9 | Auto-assign available host port (avoid conflicts) | P0 |
| FR-DEP-10 | Deployment status lifecycle: queued → cloning → building → starting → running / failed | P0 |
| FR-DEP-11 | Deploy to Kubernetes pod (Docker Desktop K8s) as alternative to bare Docker container | P1 |
| FR-DEP-12 | Retry failed builds up to N times (configurable) | P1 |
| FR-DEP-13 | Auto-cleanup idle deployments after TTL (default 20 min, configurable) | P1 |

### 7.3 Repository Discovery

| ID | Requirement | Priority |
|---|---|---|
| FR-REPO-1 | Search GitHub repositories by keyword via backend proxy | P1 |
| FR-REPO-2 | Display repo name, description, stars, language | P1 |
| FR-REPO-3 | GitHub PAT support for private repos | P2 |

### 7.4 Runtime & Observability

| ID | Requirement | Priority |
|---|---|---|
| FR-OBS-1 | Expose `/metrics` endpoint for Prometheus scraping | P0 |
| FR-OBS-2 | Track: HTTP request count, latency histogram, deployment created/failed counters | P0 |
| FR-OBS-3 | Grafana dashboard pre-provisioned with deployment overview panel | P1 |
| FR-OBS-4 | Health check endpoint returning JSON status of all dependencies | P0 |
| FR-OBS-5 | Runtime stats endpoint (queue depth, active workers, total deployments) | P1 |

---

## 8. Non-Functional Requirements

### 8.1 Performance

| ID | Requirement | Target |
|---|---|---|
| NFR-PERF-1 | API response time (P99) | < 200ms for non-build operations |
| NFR-PERF-2 | Build start latency (from submit to clone start) | < 2 seconds |
| NFR-PERF-3 | WebSocket log delivery latency | < 500ms from build output to browser |
| NFR-PERF-4 | Concurrent builds | 3 (configurable up to 10) |
| NFR-PERF-5 | Max queue depth | 256 (Redis-backed for overflow) |

### 8.2 Reliability

| ID | Requirement | Target |
|---|---|---|
| NFR-REL-1 | Backend availability | 99.5% (local dev target) |
| NFR-REL-2 | Build queue persistence | Survive backend restart (Redis AOF) |
| NFR-REL-3 | Graceful shutdown | Finish active builds, drain queue within 30s |
| NFR-REL-4 | Database reconnect | Auto-retry with exponential backoff |

### 8.3 Security

See Section 11 for full security requirements.

### 8.4 Scalability (future)

| ID | Requirement | Notes |
|---|---|---|
| NFR-SCALE-1 | Stateless backend | JWT-based auth; no server-side session |
| NFR-SCALE-2 | Horizontal build workers | Multiple backend pods can share Redis queue |
| NFR-SCALE-3 | Database pooling | pgxpool, max 20 connections per instance |

### 8.5 Compatibility

| ID | Requirement |
|---|---|
| NFR-COMPAT-1 | Docker Desktop 4.x+ on Windows 11, macOS 13+ |
| NFR-COMPAT-2 | Docker Desktop Kubernetes 1.28+ |
| NFR-COMPAT-3 | Chrome 120+, Firefox 121+, Safari 17+, Edge 120+ |
| NFR-COMPAT-4 | Go 1.22+, Node.js 20 LTS |

---

## 9. Docker Desktop Integration

### 9.1 Requirements

Docker Desktop is the **primary container runtime** for InstantDeploy v1.0.

| ID | Requirement |
|---|---|
| DD-1 | Backend must mount `/var/run/docker.sock` to communicate with Docker Desktop Engine |
| DD-2 | Docker GID must match between host and container (use GID 999 or detect dynamically) |
| DD-3 | Built images must be tagged with `instantdeploy/<deployment-id>:latest` |
| DD-4 | Running containers must be labeled `instantdeploy=true`, `deployment_id=<id>` for reconciliation |
| DD-5 | Host port assignment must check for conflicts before binding (range: 8100–8999) |
| DD-6 | All `docker build` and `docker run` commands must have enforced timeouts |
| DD-7 | On backend startup, reconcile Docker containers with deployment database state |
| DD-8 | Backend must NOT run as root; use `appuser` with Docker group membership |

### 9.2 Docker Compose Setup (Development)

The `docker-compose.isolated.yml` file defines the complete local stack:

```yaml
# Service dependency order:
# postgres → redis → backend → frontend
# prometheus and grafana are independent

# Required volumes:
# - postgres_data     (persistent DB)
# - redis_data        (AOF persistence)
# - grafana_data      (dashboards/config)
# - /tmp/builds       (transient build workspace, bind mount)
# - /var/run/docker.sock (Docker Desktop socket)
```

**Startup Command:**
```bash
# Windows (PowerShell)
.\start-isolated.ps1

# Mac/Linux
./start-isolated.sh
```

**Port Map (Local):**
| Service | Host Port | Container Port |
|---|---|---|
| Backend | 8082 | 8080 |
| Frontend | 3000 | 80 |
| PostgreSQL | 5432 | 5432 |
| Redis | 6379 | 6379 |
| Prometheus | 9090 | 9090 |
| Grafana | 3001 | 3000 |
| Deployed apps | 8100–8999 | varies |

### 9.3 Image Build Strategy

```
GitHub Repo
    │
    ▼ git clone --depth 1
/tmp/builds/<deployment_id>/
    │
    ▼ SmartDetector.Detect()
Runtime Type (e.g., "nodejs-18")
    │
    ▼ DockerfileGenerator.Generate()
Dockerfile.instantdeploy
    │
    ▼ BuildFixer.Fix()
Cleaned Dockerfile + build context
    │
    ▼ docker build -t instantdeploy/<id>:latest
    │    --label instantdeploy=true
    │    --label deployment_id=<id>
    │    -f Dockerfile.instantdeploy .
    │
    ▼ docker run -d
         -p <host_port>:<app_port>
         --name instantdeploy-<id>
         --label instantdeploy=true
         instantdeploy/<id>:latest
```

---

## 10. Kubernetes Integration

### 10.1 Target: Docker Desktop Built-in Kubernetes

Docker Desktop ships with a single-node Kubernetes cluster (`docker-desktop` context). This is the v1.0 target — no external cluster required.

**Prerequisites:**
1. Docker Desktop → Settings → Kubernetes → Enable Kubernetes ✓
2. `kubectl config use-context docker-desktop`
3. Run `deploy-k8s.ps1` (Windows) or `deploy-k8s.sh` (Mac/Linux)

### 10.2 Namespace & RBAC

```yaml
# Namespace: instantdeploy
# All resources scoped to this namespace

# ServiceAccount: instantdeploy-backend
# ClusterRole: instantdeploy-deployer
#   - get/list/watch/create/delete pods, deployments, services
#   - get/list nodes
# ClusterRoleBinding: instantdeploy-backend → instantdeploy-deployer
```

**Requirement**: The backend ServiceAccount must have permissions to create/delete Pods and Services in the `instantdeploy` namespace when deploying user projects to K8s (FR-DEP-11).

### 10.3 Core Manifests

```
infrastructure/k8s/
├── kustomization.yaml           # Root kustomize entry
├── backend/
│   ├── namespace.yaml           # instantdeploy namespace
│   ├── serviceaccount.yaml      # NEW: backend SA
│   ├── rbac.yaml                # NEW: Role + RoleBinding
│   ├── configmap.yaml           # App config (port, workers, etc.)
│   ├── secret.yaml              # JWT secret, DB credentials
│   ├── deployment.yaml          # Backend pod spec
│   └── service.yaml             # ClusterIP :8080
├── frontend/
│   ├── deployment.yaml          # Frontend pod spec
│   ├── service.yaml             # NodePort :30000
│   └── configmap.yaml           # nginx.conf
└── datastores/
    ├── postgres.yaml             # StatefulSet + Service
    ├── postgres-secret.yaml      # DB password
    ├── postgres-pvc.yaml         # 10Gi PVC
    └── redis.yaml                # Deployment + Service
```

### 10.4 Backend Deployment Spec (Key Fields)

```yaml
spec:
  containers:
  - name: backend
    image: instantdeploy-backend:local
    # Docker socket for container builds
    volumeMounts:
    - name: docker-socket
      mountPath: /var/run/docker.sock
    - name: build-workspace
      mountPath: /tmp/builds
    # Health probes (SEPARATE endpoints)
    readinessProbe:
      httpGet:
        path: /api/v1/health/ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 10
    livenessProbe:
      httpGet:
        path: /api/v1/health/live
        port: 8080
      initialDelaySeconds: 15
      periodSeconds: 20
    # Resource limits
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"    # Increased from 256Mi for build tasks
        cpu: "1000m"       # Increased from 500m for build tasks
  volumes:
  - name: docker-socket
    hostPath:
      path: /var/run/docker.sock
      type: Socket
  - name: build-workspace
    emptyDir: {}
  # Security context
  securityContext:
    runAsNonRoot: true
    runAsUser: 1001
    fsGroup: 999           # Docker GID
```

### 10.5 Accessing Services

| Service | Access Method | URL |
|---|---|---|
| Frontend | NodePort 30000 | `http://localhost:30000` |
| Backend (debug) | `kubectl port-forward svc/backend 8080:8080` | `http://localhost:8080` |
| Grafana | `kubectl port-forward svc/grafana 3001:3000` | `http://localhost:3001` |
| Prometheus | `kubectl port-forward svc/prometheus 9090:9090` | `http://localhost:9090` |

### 10.6 K8s Deployment Mode for User Apps (FR-DEP-11)

When a user selects "Deploy to Kubernetes" instead of "Deploy to Docker":

1. Backend builds the Docker image normally via Docker socket
2. Backend calls Kubernetes API (using in-cluster service account) to create:
   - A `Deployment` in namespace `instantdeploy` with the built image
   - A `Service` (NodePort, auto-assigned) for external access
3. Backend monitors pod phase via K8s watch API
4. Access URL format: `http://localhost:<node_port>`
5. On delete: remove Deployment + Service via K8s API

---

## 11. Security Requirements

### 11.1 Authentication & Authorization

| ID | Requirement |
|---|---|
| SEC-AUTH-1 | JWT secret MUST be ≥ 32 random bytes. Fail fast if default is used in production |
| SEC-AUTH-2 | Passwords stored as bcrypt hash (cost ≥ 12) |
| SEC-AUTH-3 | Email verification required before first login |
| SEC-AUTH-4 | Password reset codes: stored server-side, 15-minute TTL, single-use |
| SEC-AUTH-5 | Account lockout: 5 failed attempts → 15-minute lockout (stored in Redis) |
| SEC-AUTH-6 | All JWT claims validated: `exp`, `iat`, `sub` (user_id) |
| SEC-AUTH-7 | WebSocket connections validated against JWT claims (user_id match) |

### 11.2 Input Validation

| ID | Requirement |
|---|---|
| SEC-INPUT-1 | Repository URLs must match `^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\.git)?$` |
| SEC-INPUT-2 | Branch names must match `^[a-zA-Z0-9._/-]{1,100}$` |
| SEC-INPUT-3 | All user inputs sanitized before storage (prevent XSS via log output) |
| SEC-INPUT-4 | Max request body: 1MB |
| SEC-INPUT-5 | Max build log line length: 4096 characters |

### 11.3 Network Security

| ID | Requirement |
|---|---|
| SEC-NET-1 | CORS: restrict `Access-Control-Allow-Origin` to configured frontend origin only |
| SEC-NET-2 | WebSocket: use `wss://` when served over HTTPS |
| SEC-NET-3 | Rate limiting: 10 auth requests / 10 min / IP; 100 API requests / min / user |
| SEC-NET-4 | Nginx security headers: CSP, X-Frame-Options DENY, X-Content-Type-Options nosniff, HSTS |

### 11.4 Secrets Management

| ID | Requirement |
|---|---|
| SEC-SEC-1 | No secrets in source code or Docker images |
| SEC-SEC-2 | Secrets loaded from environment variables or `.env` (gitignored) |
| SEC-SEC-3 | K8s Secrets used for JWT secret, DB credentials |
| SEC-SEC-4 | `.env.example` provides template with placeholder values only |

### 11.5 Container Security

| ID | Requirement |
|---|---|
| SEC-CONT-1 | Backend container runs as non-root (`appuser`, UID 1001) |
| SEC-CONT-2 | Backend container in Docker group (GID 999) for socket access |
| SEC-CONT-3 | No `--privileged` flag on any container |
| SEC-CONT-4 | All built images tagged with deployment ID; auto-pruned after TTL |
| SEC-CONT-5 | Build workspace (`/tmp/builds`) cleaned after each deployment build |

---

## 12. Data Model

### 12.1 Users Table

```sql
CREATE TABLE users (
  username          TEXT PRIMARY KEY,
  email             TEXT NOT NULL UNIQUE,
  password_hash     BYTEA NOT NULL,
  role              TEXT NOT NULL DEFAULT 'developer',
  verified          BOOLEAN NOT NULL DEFAULT FALSE,
  verification_code TEXT,
  verification_code_expires_at TIMESTAMP,          -- NEW: code TTL
  failed_attempts   INTEGER NOT NULL DEFAULT 0,
  locked_until      TIMESTAMP,
  created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 12.2 Deployments Table

```sql
CREATE TABLE deployments (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users(username),  -- NEW: user isolation
  repository      TEXT NOT NULL,
  branch          TEXT NOT NULL DEFAULT 'main',
  status          TEXT NOT NULL,
  target          TEXT NOT NULL DEFAULT 'docker',            -- NEW: 'docker' | 'kubernetes'
  url             TEXT,
  local_url       TEXT,
  repo_url        TEXT,
  image           TEXT,
  container       TEXT,
  error           TEXT,
  deleted_at      TIMESTAMP,                                 -- NEW: soft delete
  created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deployments_user_id ON deployments(user_id);
CREATE INDEX idx_deployments_status ON deployments(status);
```

### 12.3 Deployment Logs Table

```sql
CREATE TABLE deployment_logs (
  id              BIGSERIAL PRIMARY KEY,
  deployment_id   TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
  time            TIMESTAMP NOT NULL,
  level           TEXT NOT NULL CHECK (level IN ('info', 'warn', 'error')),
  message         TEXT NOT NULL
);

CREATE INDEX idx_deployment_logs_deployment_id ON deployment_logs(deployment_id);
CREATE INDEX idx_deployment_logs_time ON deployment_logs(deployment_id, time);
```

### 12.4 Audit Logs Table (NEW)

```sql
CREATE TABLE audit_logs (
  id          BIGSERIAL PRIMARY KEY,
  user_id     TEXT NOT NULL,
  action      TEXT NOT NULL,   -- 'login', 'logout', 'deploy.create', 'deploy.delete', 'password.reset'
  resource_id TEXT,
  ip_address  TEXT,
  user_agent  TEXT,
  metadata    JSONB,
  created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id, created_at DESC);
```

---

## 13. API Specification

### 13.1 Base URL
- **Development**: `http://localhost:8082/api/v1`
- **Docker Desktop K8s**: `http://localhost:30000/api/v1`

### 13.2 Authentication Endpoints (Public)

| Method | Path | Body | Response |
|---|---|---|---|
| `POST` | `/auth/signup` | `{username, email, password}` | `201 {message}` |
| `POST` | `/auth/login` | `{email, password}` | `200 {token, user}` |
| `POST` | `/auth/verify` | `{email, code}` | `200 {message}` |
| `POST` | `/auth/forgot-password` | `{email}` | `200 {message}` |
| `POST` | `/auth/reset-password` | `{email, code, new_password}` | `200 {message}` |

### 13.3 Deployment Endpoints (Authenticated)

| Method | Path | Body / Params | Response |
|---|---|---|---|
| `GET` | `/deployments` | — | `200 [{deployment}]` |
| `POST` | `/deployments` | `{repository, branch, target?}` | `201 {deployment}` |
| `GET` | `/deployments/{id}` | — | `200 {deployment}` |
| `DELETE` | `/deployments/{id}` | — | `204` |
| `GET` | `/deployments/{id}/logs` | `?since=<timestamp>` | `200 [{log}]` |

### 13.4 Repository Endpoints (Authenticated)

| Method | Path | Params | Response |
|---|---|---|---|
| `GET` | `/repositories` | `?q=<query>` | `200 [{repo}]` |

### 13.5 System Endpoints

| Method | Path | Auth | Response |
|---|---|---|---|
| `GET` | `/health` | None | `200 {status, dependencies}` |
| `GET` | `/health/ready` | None | `200` / `503` |
| `GET` | `/health/live` | None | `200` |
| `GET` | `/metrics` | None | `200 (Prometheus text)` |
| `GET` | `/runtime/stats` | JWT | `200 {queue_depth, active_workers, total}` |

### 13.6 WebSocket

**Endpoint**: `ws://localhost:8082/ws?token=<jwt>`

> Changed from `?user_id=` to `?token=` — server extracts user_id from JWT claims.

**Event Types:**
```json
// Deployment status change
{
  "type": "deployment_status",
  "deployment_id": "abc123",
  "user_id": "alice",
  "status": "building",
  "url": null,
  "timestamp": "2026-03-16T10:00:00Z"
}

// Real-time log line
{
  "type": "deployment_log",
  "deployment_id": "abc123",
  "user_id": "alice",
  "level": "info",
  "message": "Step 3/7: RUN npm install",
  "timestamp": "2026-03-16T10:00:05Z"
}
```

---

## 14. Frontend Requirements

### 14.1 Pages & Routes

| Route | Page | Auth Required |
|---|---|---|
| `/` | Redirect to `/login` or `/dashboard` | — |
| `/login` | Login / Sign-up page | No |
| `/dashboard` | Main deployment dashboard | Yes |
| `/deployments/:id` | Deployment detail + live logs | Yes |

### 14.2 Dashboard Features

- [ ] List all user deployments (cards with status badge, URL link, created time)
- [ ] Create deployment form (repo URL, branch, target: Docker / Kubernetes)
- [ ] Real-time status updates via WebSocket (no polling)
- [ ] Live log viewer with auto-scroll (collapsible per deployment)
- [ ] One-click delete with confirmation
- [ ] Repository search with GitHub integration
- [ ] Runtime stats panel (queue depth, active builds)
- [ ] Toast notifications for status transitions

### 14.3 State Management

```
authStore (Zustand + localStorage)
  ├── user: { username, email, role }
  ├── token: string
  └── isAuthenticated: boolean

deploymentStore (Zustand)
  ├── deployments: Deployment[]
  ├── activeDeploymentId: string | null
  ├── logs: Record<deploymentId, LogEntry[]>
  └── wsConnected: boolean
```

### 14.4 WebSocket Client Requirements

- Connect on mount with `?token=<jwt>` query param
- Reconnect automatically (5 attempts, 3s → 6s → 12s exponential backoff)
- Use `wss://` when page is served over HTTPS
- Dispatch events to Zustand store on receipt
- Show disconnected banner if WS fails after all retries

---

## 15. Monitoring & Observability

### 15.1 Prometheus Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `http_requests_total` | Counter | method, path, status | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | method, path | Request latency |
| `deployments_total` | Counter | status (created/completed/failed) | Deployment outcomes |
| `build_queue_depth` | Gauge | — | Current queue size |
| `active_build_workers` | Gauge | — | Workers currently building |
| `deployment_duration_seconds` | Histogram | runtime_type | Time from queue to running |

### 15.2 Health Check Response

```json
GET /health
{
  "status": "ok",
  "version": "1.0.0",
  "dependencies": {
    "postgres": { "status": "ok", "latency_ms": 2 },
    "redis":    { "status": "ok", "latency_ms": 1 },
    "docker":   { "status": "ok", "version": "24.0.7" }
  }
}
```

### 15.3 Grafana Dashboards

Pre-provisioned dashboards:
1. **Deployment Overview** — total deployments, success rate, queue depth, worker utilization
2. **API Performance** — request rate, P50/P95/P99 latency, error rate by endpoint
3. **Build Analytics** — deployment duration by runtime type, failure reasons

---

## 16. Deployment Strategy

### 16.1 Local Docker Desktop (Development)

```bash
# Clone repo
git clone https://github.com/yourorg/instantdeploy.git

# Copy environment config
cp .env.example .env
# Edit .env: set JWT_SECRET, DATABASE_URL, etc.

# Start full stack
docker compose -f docker-compose.isolated.yml up --build -d

# Verify
curl http://localhost:8082/health
open http://localhost:3000
```

### 16.2 Docker Desktop Kubernetes (Staging/Demo)

```bash
# Ensure Docker Desktop K8s is enabled
kubectl config use-context docker-desktop

# Build images (locally, no registry needed)
docker build -t instantdeploy-backend:local -f Dockerfile.backend .
docker build -t instantdeploy-frontend:local -f frontend/Dockerfile frontend/

# Deploy
kubectl apply -k infrastructure/k8s/

# Verify
kubectl get pods -n instantdeploy
kubectl get svc -n instantdeploy

# Access frontend
open http://localhost:30000
```

### 16.3 Environment Variables Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `REDIS_ADDR` | Yes | `localhost:6379` | Redis address |
| `JWT_SECRET` | Yes | — | Min 32 bytes, random |
| `JWT_EXPIRY_MINUTES` | No | `120` | Token expiry |
| `PORT` | No | `8080` | Backend HTTP port |
| `BUILD_WORKERS` | No | `3` | Concurrent build workers |
| `BUILD_TIMEOUT` | No | `900` | Build timeout (seconds) |
| `MAX_BUILD_RETRIES` | No | `2` | Max retry attempts |
| `DOCKER_HOST` | No | `unix:///var/run/docker.sock` | Docker socket path |
| `ALLOWED_ORIGINS` | No | `http://localhost:3000` | CORS whitelist (comma-separated) |
| `PROMETHEUS_ENABLED` | No | `true` | Enable /metrics endpoint |
| `GITHUB_TOKEN` | No | — | GitHub PAT for repo search |
| `ENV` | No | `development` | `development` or `production` |
| `SEED_DEMO_USER` | No | `false` | Create demo user on startup (dev only) |

---

## 17. Release Milestones

### v1.0 — MVP (Local Docker + K8s)
**Goal**: Fully functional local deployment tool

- [ ] Fix all CRITICAL issues (C1–C6)
- [ ] Fix all HIGH issues (H1–H6)
- [ ] Add user isolation to deployments
- [ ] Kubernetes mode for user app deployments (FR-DEP-11)
- [ ] Separate K8s liveness/readiness probes
- [ ] RBAC for backend service account
- [ ] Full Grafana dashboard set
- [ ] `.env.example` with all vars documented
- [ ] README with Docker Desktop + K8s quick-start

### v1.1 — Hardened
**Goal**: Team-safe, production-grade local deployment

- [ ] Fix all MEDIUM issues (M1–M7)
- [ ] Audit log table + API
- [ ] Nginx security headers
- [ ] Request ID tracing end-to-end
- [ ] Private GitHub repo support (PAT)
- [ ] JWT refresh endpoint

### v2.0 — Cloud-Ready
**Goal**: Deploy to EKS / GKE / AKS

- [ ] Helm chart for Kubernetes deployment
- [ ] External secret management (Vault / AWS SSM)
- [ ] Multi-tenant isolation
- [ ] CI/CD pipeline integration (GitHub Actions trigger)
- [ ] Role-based UI (admin panel)
- [ ] Cloud image registry (ECR / GCR) integration

---

## 18. Open Questions

| # | Question | Owner | Target Decision |
|---|---|---|---|
| OQ-1 | Should K8s deployments use `NodePort` or `Ingress` for user app access on Docker Desktop? | Backend lead | v1.0 |
| OQ-2 | What is the max image size allowed per deployment? (disk space management) | Platform | v1.0 |
| OQ-3 | Should we support monorepos with multiple services? | Product | v1.1 |
| OQ-4 | Email delivery for verification codes — which provider? (SMTP, SendGrid, SES) | Platform | v1.0 |
| OQ-5 | Should build workspace use `/tmp/builds` or a named Docker volume? | Backend lead | v1.0 |
| OQ-6 | Is Loki log aggregation in scope for v1.0 or v1.1? | Platform | v1.0 |
| OQ-7 | Should deployed user app containers be isolated in a separate Docker network? | Security | v1.0 |

---

*Document maintained by the InstantDeploy team. Last updated: 2026-03-16.*
