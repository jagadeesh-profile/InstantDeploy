# PowerShell version for Windows users

Write-Host " Starting InstantDeploy in Isolated Environment" -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Green

# Check Docker Desktop
try {
   docker info | Out-Null
   Write-Host " Docker Desktop is running" -ForegroundColor Green
} catch {
   Write-Host " Docker Desktop is not running!" -ForegroundColor Red
   Write-Host "Please start Docker Desktop and try again."
   exit 1
}

# Check existing containers
$existing = docker ps -a | Select-String "instantdeploy.*isolated"
if ($existing) {
   Write-Host "  Existing containers found" -ForegroundColor Yellow
   $response = Read-Host "Remove them and start fresh? (y/N)"
   if ($response -eq "y") {
      Write-Host "Stopping and removing existing containers..."
      docker-compose -f docker-compose.isolated.yml down -v
   }
}

# Build images
Write-Host ""
Write-Host " Building Docker images..." -ForegroundColor Cyan
docker-compose -f docker-compose.isolated.yml build --no-cache

# Start services
Write-Host ""
Write-Host " Starting all services..." -ForegroundColor Cyan
docker-compose -f docker-compose.isolated.yml up -d

# Wait for health
Write-Host ""
Write-Host " Waiting for services..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# Check health
Write-Host ""
Write-Host " Checking service health..." -ForegroundColor Cyan

# PostgreSQL
try {
   docker exec instantdeploy-postgres-isolated pg_isready -U instantdeploy_user | Out-Null
   Write-Host " PostgreSQL is healthy" -ForegroundColor Green
} catch {
   Write-Host " PostgreSQL is not healthy" -ForegroundColor Red
}

# Redis
try {
   docker exec instantdeploy-redis-isolated redis-cli ping | Out-Null
   Write-Host " Redis is healthy" -ForegroundColor Green
} catch {
   Write-Host " Redis is not healthy" -ForegroundColor Red
}

# Backend
Start-Sleep -Seconds 5
try {
   $response = Invoke-WebRequest -Uri "http://localhost:8082/api/v1/health" -UseBasicParsing
   Write-Host " Backend API is healthy" -ForegroundColor Green
} catch {
   Write-Host "  Backend is starting..." -ForegroundColor Yellow
}

# Frontend
try {
   $response = Invoke-WebRequest -Uri "http://localhost:3000" -UseBasicParsing
   Write-Host " Frontend is healthy" -ForegroundColor Green
} catch {
   Write-Host "  Frontend is starting..." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "==================================================" -ForegroundColor Green
Write-Host " InstantDeploy is running!" -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Green
Write-Host ""
Write-Host " Access Points:"
Write-Host "   Frontend:   http://localhost:3000"
Write-Host "   Backend:    http://localhost:8082"
Write-Host "   Prometheus: http://localhost:9090"
Write-Host "   Grafana:    http://localhost:3001"
Write-Host ""
Write-Host " Useful Commands:"
Write-Host "   View logs:  docker-compose -f docker-compose.isolated.yml logs -f"
Write-Host "   Stop all:   docker-compose -f docker-compose.isolated.yml down"
Write-Host "   Shell:      docker exec -it instantdeploy-backend-isolated sh"
Write-Host ""
