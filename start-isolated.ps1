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





================================================================================

PART 7: TROUBLESHOOTING GUIDE

================================================================================



COMMON ISSUES & SOLUTIONS

--------------------------



ISSUE: "Cannot connect to Docker daemon"

SOLUTION:

- Ensure Docker Desktop is running

- On Windows: Settings  General  "Expose daemon on tcp://localhost:2375"

- Restart Docker Desktop



ISSUE: "Permission denied on /var/run/docker.sock"

SOLUTION:

- On Windows: This should work automatically

- On Linux: Add user to docker group:

    sudo usermod -aG docker $USER

    newgrp docker



ISSUE: "Ports already in use"

SOLUTION:

- Stop conflicting services:

    docker-compose -f docker-compose.isolated.yml down

- Or change ports in docker-compose.isolated.yml:

    ports:

      - "8083:8082"  # Change 8083 to any free port



ISSUE: "Build context path contains backslashes"

SOLUTION:

- This is solved by running everything in Linux containers

- All paths inside containers use forward slashes

- No Windows paths (C:\Users\...) are used



ISSUE: "Git clone fails inside container"

SOLUTION:

- Check internet connectivity

- Try SSH keys if needed:

    docker exec -it instantdeploy-backend-isolated sh

    ssh-keygen -t ed25519

    cat ~/.ssh/id_ed25519.pub

    # Add to GitHub



ISSUE: "Out of disk space"

SOLUTION:

- Clean up Docker:

    docker system prune -a -f --volumes

- Increase Docker Desktop disk limit:

    Settings  Resources  Disk image size





VERIFY EVERYTHING IS ISOLATED

-------------------------------



# Check where builds happen (should be /tmp/builds inside container)

docker exec instantdeploy-backend-isolated ls -la /tmp/builds



# Check no Windows paths

docker exec instantdeploy-backend-isolated env | grep -i "c:"

# Should return nothing



# Check Docker can create containers

docker exec instantdeploy-backend-isolated docker ps



# Check Git works

docker exec instantdeploy-backend-isolated git --version





ACCESSING LOGS

--------------



# All services

docker-compose -f docker-compose.isolated.yml logs -f



# Specific service

docker-compose -f docker-compose.isolated.yml logs -f backend



# Database

docker exec -it instantdeploy-postgres-isolated \

  psql -U instantdeploy_user -d instantdeploy \

  -c "SELECT id, status, repository FROM deployments ORDER BY created_at DESC LIMIT 10;"



# Build queue

docker exec -it instantdeploy-redis-isolated \

  redis-cli ZCARD instantdeploy:build_queue





RESETTING EVERYTHING

--------------------



# Complete reset (deletes all data)

docker-compose -f docker-compose.isolated.yml down -v



# Remove all InstantDeploy images

docker images | grep instantdeploy | awk '{print $3}' | xargs docker rmi -f



# Clean Docker

docker system prune -a -f --volumes



# Start fresh

./start-isolated.sh





================================================================================

PART 8: QUICK START GUIDE

================================================================================



FOR WINDOWS USERS (PowerShell):

--------------------------------



1. Open PowerShell as Administrator



2. Navigate to project:

   cd C:\path\to\instantdeploy



3. Start environment:

   .\start-isolated.ps1



4. Wait for " InstantDeploy is running!"



5. Access:

   - Frontend: http://localhost:3000

   - Backend:  http://localhost:8082



6. Test deployment:

   .\test-tictactoe.ps1





FOR MAC/LINUX USERS (Bash):

----------------------------



1. Open Terminal



2. Navigate to project:

   cd /path/to/instantdeploy



3. Make scripts executable:

   chmod +x start-isolated.sh test-tictactoe.sh stop-isolated.sh



4. Start environment:

   ./start-isolated.sh



5. Wait for " InstantDeploy is running!"



6. Access:

   - Frontend: http://localhost:3000

   - Backend:  http://localhost:8082



7. Test deployment:

   ./test-tictactoe.sh





DAILY WORKFLOW:

---------------



# Start environment

./start-isolated.sh



# Make code changes

# (Edit files in your IDE as normal)



# Rebuild backend if Go code changed

docker-compose -f docker-compose.isolated.yml build backend

docker-compose -f docker-compose.isolated.yml up -d backend



# Rebuild frontend if React code changed

docker-compose -f docker-compose.isolated.yml build frontend

docker-compose -f docker-compose.isolated.yml up -d frontend



# View logs

docker-compose -f docker-compose.isolated.yml logs -f backend



# Stop environment

./stop-isolated.sh





ADVANTAGES OF THIS APPROACH:

-----------------------------



 No Windows path issues (all paths are Linux)

 Reproducible on any machine with Docker Desktop

 Easy to reset (just rebuild containers)

 Isolated from host system

 Same environment as production

 Easy to share (just share docker-compose file)

 Works on Windows, Mac, and Linux identically

 No need to install Go, Node, PostgreSQL on host

 Clean development environment





DISADVANTAGES:

--------------



 Slightly slower builds (running in VM)

 Need to rebuild container to see Go code changes

 Uses more disk space (Docker images)

 Requires Docker Desktop to be running





================================================================================

END OF ISOLATED ENVIRONMENT SETUP

================================================================================



SUMMARY:



 Complete Docker-in-Docker setup

 All services run in Linux containers

 No Windows path issues

 Fully isolated virtual environment

 One-command startup (./start-isolated.sh or .\start-isolated.ps1)

 Easy to test deployments

 Easy to reset and start fresh

 Production-ready configuration



All paths inside containers are Linux paths (/tmp/builds, /app, etc.)

No C://Users/jagad or Windows-style paths!



Copy all files to your project and run:

  Windows: .\start-isolated.ps1

  Mac/Linux: ./start-isolated.sh



Then test TicTacToe:

  Windows: .\test-tictactoe.ps1

  Mac/Linux: ./test-tictactoe.sh
