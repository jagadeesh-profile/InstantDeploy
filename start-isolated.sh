#!/bin/bash



set -e



echo " Starting InstantDeploy in Isolated Environment"

echo "=================================================="



# Colors

GREEN='\033[0;32m'

YELLOW='\033[1;33m'

RED='\033[0;31m'

NC='\033[0m'



# Check Docker Desktop is running

if ! docker info > /dev/null 2>&1; then

    echo -e "${RED} Docker Desktop is not running!${NC}"

    echo "Please start Docker Desktop and try again."

    exit 1

fi



echo -e "${GREEN} Docker Desktop is running${NC}"



# Check if containers already exist

if docker ps -a | grep -q "instantdeploy.*isolated"; then

    echo -e "${YELLOW}  Existing containers found${NC}"

    read -p "Do you want to remove them and start fresh? (y/N) " -n 1 -r

    echo

    if [[ $REPLY =~ ^[Yy]$ ]]; then

        echo "Stopping and removing existing containers..."

        docker-compose -f docker-compose.isolated.yml down -v

    fi

fi



# Build and start all services

echo ""

echo " Building Docker images..."

docker-compose -f docker-compose.isolated.yml build --no-cache



echo ""

echo " Starting all services..."

docker-compose -f docker-compose.isolated.yml up -d



# Wait for services to be healthy

echo ""

echo " Waiting for services to be healthy..."

sleep 10



# Check health

echo ""

echo " Checking service health..."



# PostgreSQL

if docker exec instantdeploy-postgres-isolated pg_isready -U instantdeploy_user > /dev/null 2>&1; then

    echo -e "${GREEN} PostgreSQL is healthy${NC}"

else

    echo -e "${RED} PostgreSQL is not healthy${NC}"

fi



# Redis

if docker exec instantdeploy-redis-isolated redis-cli ping > /dev/null 2>&1; then

    echo -e "${GREEN} Redis is healthy${NC}"

else

    echo -e "${RED} Redis is not healthy${NC}"

fi



# Backend

sleep 5

if curl -f -s http://localhost:8082/api/v1/health > /dev/null 2>&1; then

    echo -e "${GREEN} Backend API is healthy${NC}"

else

    echo -e "${YELLOW}  Backend is starting... (this may take a moment)${NC}"

fi



# Frontend

if curl -f -s http://localhost:3000 > /dev/null 2>&1; then

    echo -e "${GREEN} Frontend is healthy${NC}"

else

    echo -e "${YELLOW}  Frontend is starting...${NC}"

fi



echo ""

echo "=================================================="

echo -e "${GREEN} InstantDeploy is running!${NC}"

echo "=================================================="

echo ""

echo " Access Points:"

echo "   Frontend:   http://localhost:3000"

echo "   Backend:    http://localhost:8082"

echo "   Prometheus: http://localhost:9090"

echo "   Grafana:    http://localhost:3001 (admin/admin)"

echo ""

echo " Useful Commands:"

echo "   View logs:        docker-compose -f docker-compose.isolated.yml logs -f"

echo "   Stop all:         docker-compose -f docker-compose.isolated.yml down"

echo "   Restart:          docker-compose -f docker-compose.isolated.yml restart"

echo "   Shell (backend):  docker exec -it instantdeploy-backend-isolated sh"

echo "   Shell (postgres): docker exec -it instantdeploy-postgres-isolated psql -U instantdeploy_user -d instantdeploy"

echo ""

echo " Test TicTacToe Deployment:"

echo "   ./test-tictactoe.sh"

echo ""
