# InstantDeploy

InstantDeploy is a mobile runtime platform with:

- Go backend API for auth, deployments, repository search, and metrics
- React dashboard for creating and viewing deployments
- SwiftUI iOS client skeleton
- Docker-based infrastructure with PostgreSQL, Redis, Prometheus, Grafana, and Loki

## Project Structure

```
InstantDeploy/
├── backend/
├── frontend/
├── ios/
├── infrastructure/
├── scripts/
└── README.md
```

## Quick Start

### 1) Run with Docker

From repository root:

```bash
docker compose -f infrastructure/docker-compose.yml up --build
```

### 2) Access Services

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin / admin)
- Loki: http://localhost:3100

## Backend API Endpoints

- `GET /api/v1/health`
- `GET /api/v1/runtime/stats` (Bearer token required)
- `POST /api/v1/auth/signup`
- `POST /api/v1/auth/login`
- `GET /api/v1/deployments` (Bearer token required)
- `POST /api/v1/deployments` (Bearer token required)
- `GET /api/v1/repositories?query=...` (Bearer token required)
- `GET /metrics`

### Demo Login

Pre-seeded demo account:

```json
{
	"username": "demo",
	"password": "demo123"
}
```

### Signup + Custom URL Deployment Demo

1. Open frontend: `http://localhost:5173`
2. In Authentication panel:
	 - Select `Sign Up`
	 - Enter username and password (minimum 6 chars)
	 - Confirm password and submit
3. App will auto-login after signup.
4. In New Deployment panel:
	 - Enter repository (example: `octocat/Hello-World`)
	 - Enter branch (example: `main`)
	 - Enter deployment URL (example: `https://myapp.example.com`)
	 - Click `Deploy Now`
5. New deployment appears in the Deployments list with your provided URL.

### API Demo (curl)

Signup:

```bash
curl -X POST http://localhost:8081/api/v1/auth/signup \
	-H "Content-Type: application/json" \
	-d '{"username":"alice","password":"alice123"}'
```

Login:

```bash
curl -X POST http://localhost:8081/api/v1/auth/login \
	-H "Content-Type: application/json" \
	-d '{"username":"alice","password":"alice123"}'
```

Create deployment with custom URL:

```bash
curl -X POST http://localhost:8081/api/v1/deployments \
	-H "Content-Type: application/json" \
	-H "Authorization: Bearer <TOKEN>" \
	-d '{"repository":"octocat/Hello-World","branch":"main","url":"https://myapp.example.com"}'
```

## Scripts

- `scripts/setup.sh`: install frontend dependencies
- `scripts/start-dev.sh`: start the full stack with Docker Compose
- `scripts/start-local-dev.ps1`: start local backend + frontend on Windows (with Redis/Postgres dependencies)
- `scripts/stop-local-dev.ps1`: stop local backend + frontend started by the PowerShell script
- `scripts/local-dev.ps1`: single Windows entrypoint for `start`, `stop`, `status`, and `logs`
- `scripts/deploy-all.ps1`: deploy full stack to local Docker, Kubernetes, or both
- `scripts/seed-db.sql`: seed PostgreSQL with sample deployment

### One-Command Deploy (Windows)

From repository root:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\deploy-all.ps1 -Target all
```

Kubernetes only:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\deploy-all.ps1 -Target k8s
```

Local Docker only:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\deploy-all.ps1 -Target local
```

### Windows Local Dev

From repository root:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-local-dev.ps1
```

Stop local backend/frontend:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\stop-local-dev.ps1
```

Or use the single wrapper command:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\local-dev.ps1 -Action start
powershell -ExecutionPolicy Bypass -File .\scripts\local-dev.ps1 -Action status
powershell -ExecutionPolicy Bypass -File .\scripts\local-dev.ps1 -Action logs -Service backend -Tail 60
powershell -ExecutionPolicy Bypass -File .\scripts\local-dev.ps1 -Action stop
```

## Notes

- Backend runtime supports real project detection, build-file fixups, and generated Dockerfiles across supported stacks.
- Set `GITHUB_TOKEN` in backend environment to avoid GitHub API rate limits.
