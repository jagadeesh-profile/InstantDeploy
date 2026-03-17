# deploy-k8s.ps1 - Deploy InstantDeploy to Docker Desktop Kubernetes

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "==> Building backend Docker image..." -ForegroundColor Cyan
docker build -t instantdeploy-backend:local ./backend

Write-Host ""
Write-Host "==> Building frontend Docker image..." -ForegroundColor Cyan
docker build -t instantdeploy-frontend:local ./frontend

Write-Host ""
Write-Host "==> Applying Kubernetes manifests..." -ForegroundColor Cyan
kubectl apply -k ./infrastructure/k8s

Write-Host ""
Write-Host "==> Waiting for datastores to be ready..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=instantdeploy-postgres -n instantdeploy --timeout=120s
kubectl wait --for=condition=ready pod -l app=instantdeploy-redis    -n instantdeploy --timeout=60s

Write-Host ""
Write-Host "==> Waiting for backend to be ready..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=instantdeploy-backend -n instantdeploy --timeout=120s

Write-Host ""
Write-Host "==> Waiting for frontend to be ready..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=instantdeploy-frontend -n instantdeploy --timeout=60s

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  InstantDeploy is live!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Frontend  -> http://localhost:30000" -ForegroundColor White
Write-Host "  Backend   -> http://localhost:30080" -ForegroundColor White
Write-Host ""
Write-Host "  Login: demo / Demo123!" -ForegroundColor White
Write-Host ""
