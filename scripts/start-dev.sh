#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "[InstantDeploy] Starting full stack with Docker Compose..."
docker compose -f "$ROOT_DIR/infrastructure/docker-compose.yml" up --build
