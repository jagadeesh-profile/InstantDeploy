#!/usr/bin/env bash
set -euo pipefail

echo "[InstantDeploy] Setting up frontend dependencies..."
cd "$(dirname "$0")/../frontend"
npm install

echo "[InstantDeploy] Setup complete."
