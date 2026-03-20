#!/usr/bin/env bash
# deploy-k8s.sh - Deploy InstantDeploy to Docker Desktop Kubernetes
set -e

echo ""
echo "==> Building backend Docker image..."
docker build -t instantdeploy-backend:local ./backend

echo ""
echo "==> Building frontend Docker image..."
docker build -t instantdeploy-frontend:local ./frontend

echo ""
echo "==> Applying Kubernetes manifests..."
kubectl apply -k ./infrastructure/k8s

echo ""
echo "==> Waiting for datastores to be ready..."
kubectl wait --for=condition=ready pod -l app=instantdeploy-postgres -n instantdeploy --timeout=120s
kubectl wait --for=condition=ready pod -l app=instantdeploy-redis    -n instantdeploy --timeout=60s

echo ""
echo "==> Waiting for backend to be ready..."
kubectl wait --for=condition=ready pod -l app=instantdeploy-backend -n instantdeploy --timeout=120s

echo ""
echo "==> Waiting for frontend to be ready..."
kubectl wait --for=condition=ready pod -l app=instantdeploy-frontend -n instantdeploy --timeout=60s

echo ""
echo "========================================"
echo "  InstantDeploy is live!"
echo "========================================"
echo ""
echo "  Frontend  -> http://localhost:30000"
echo "  Backend   -> http://localhost:30080"
echo "  Domain    -> http://chatslm.com (after hosts file + ingress controller setup)"
echo ""
echo "  Create an account from the login page (Sign Up)"
echo ""
echo "  Hosts file entry:"
echo "    127.0.0.1 chatslm.com"
echo ""
