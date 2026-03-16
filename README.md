# InstantDeploy

Deploy any GitHub repository to a local Docker container in seconds.

## Architecture

```
Browser / iOS
     │
     ▼
React frontend  (Vite · Tailwind · Zustand)   port 5173
     │  REST + WebSocket
     ▼
Go backend                                     port 8080
  ├── chi router + JWT auth
  ├── Runtime manager
  │     ├── SmartDetector   – detects language/framework
  │     ├── BuildFixer      – removes problematic plugins
  │     └── DockerfileGenerator – generates optimised Dockerfiles
  ├── WebSocket hub  – real-time build log streaming
  └── Prometheus metrics
     │
     ├── PostgreSQL  (deployments, logs, users)
     ├── Redis       (durable build queue)
     └── Docker daemon (container builds & runs)
```

## Quick start

```bash
docker compose -f infrastructure/docker-compose.yml up --build
```

| Service    | URL                        |
|------------|----------------------------|
| Frontend   | http://localhost:5173       |
| Backend    | http://localhost:8080       |
| Prometheus | http://localhost:9090       |
| Grafana    | http://localhost:3000 (admin/admin) |

## Demo credentials

| Username | Password  |
|----------|-----------|
| `demo`   | `Demo123!` |

## Local development (without Docker)

```bash
# Start dependencies only
docker compose -f infrastructure/docker-compose.yml up postgres redis -d

# Backend
cd backend
cp .env.example .env   # edit DATABASE_URL / REDIS_ADDR to localhost
go run ./cmd/server

# Frontend (separate terminal)
cd frontend
npm install
npm run dev
```

## Supported project types (auto-detected)

| Language   | Frameworks detected                                    |
|------------|--------------------------------------------------------|
| Java       | Spring Boot (Gradle & Maven), generic Gradle/Maven     |
| Node.js    | Next.js, Nuxt, Vite, CRA, Express, NestJS, Fastify     |
| Python     | Django, FastAPI, Flask, Streamlit, generic             |
| Go         | Gin, Fiber, Echo, generic                              |
| Rust       | Actix, Rocket, Warp, generic                           |
| PHP        | Laravel, Symfony, generic                              |
| Ruby       | Rails, Sinatra, generic                                |
| .NET       | ASP.NET Core                                           |
| Static     | HTML/CSS/JS (served via Nginx)                         |

## Password requirements

Passwords must be 8+ characters with at least one uppercase, one lowercase,
one digit, and one special character (e.g. `Demo123!`).

## Environment variables

See `backend/.env.example` for all available configuration.
