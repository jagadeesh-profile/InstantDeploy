#!/bin/bash



echo " Stopping InstantDeploy Isolated Environment"



docker-compose -f docker-compose.isolated.yml down



echo " All services stopped"

echo ""

echo "To remove volumes (wipe all data):"

echo "  docker-compose -f docker-compose.isolated.yml down -v"
