#!/bin/bash
# InstantDeploy Kubernetes Production Deployment Script
# This script handles full deployment to a Kubernetes cluster

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
K8S_DIR="$SCRIPT_DIR/infrastructure/k8s"

# Configuration
DOCKER_REGISTRY="${DOCKER_REGISTRY:-docker.io}"
DOCKER_USERNAME="${DOCKER_USERNAME:-}"
DOCKER_PASSWORD="${DOCKER_PASSWORD:-}"
REGISTRY_URL="${REGISTRY_URL:-}"
DOMAIN="${DOMAIN:-instantdeploy.example.com}"
NAMESPACE="instantdeploy"
CLUSTER_CONTEXT="${CLUSTER_CONTEXT:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    command -v kubectl &>/dev/null || { log_error "kubectl not found"; exit 1; }
    command -v docker &>/dev/null || { log_error "docker not found"; exit 1; }
    command -v kustomize &>/dev/null || { log_error "kustomize not found"; exit 1; }
    
    log_info "✓ All tools present"
}

# Build and push docker images
build_images() {
    log_info "Building Docker images..."
    
    cd "$PROJECT_DIR"
    
    # Build backend
    docker build -f backend/Dockerfile -t instantdeploy-backend:latest backend/
    log_info "✓ Backend image built"
    
    # Build frontend
    docker build -f frontend/Dockerfile -t instantdeploy-frontend:latest frontend/
    log_info "✓ Frontend image built"
    
    if [ -n "$REGISTRY_URL" ]; then
        log_info "Pushing images to registry: $REGISTRY_URL"
        docker tag instantdeploy-backend:latest "$REGISTRY_URL/instantdeploy-backend:latest"
        docker tag instantdeploy-frontend:latest "$REGISTRY_URL/instantdeploy-frontend:latest"
        
        docker push "$REGISTRY_URL/instantdeploy-backend:latest"
        docker push "$REGISTRY_URL/instantdeploy-frontend:latest"
        
        log_info "✓ Images pushed"
    fi
}

# Set kubectl context
set_context() {
    if [ -n "$CLUSTER_CONTEXT" ]; then
        log_info "Setting kubectl context to: $CLUSTER_CONTEXT"
        kubectl config use-context "$CLUSTER_CONTEXT"
    fi
    
    CURRENT_CONTEXT=$(kubectl config current-context)
    log_info "Using cluster context: $CURRENT_CONTEXT"
}

# Create namespace
create_namespace() {
    log_info "Creating namespace: $NAMESPACE"
    kubectl apply -f <(cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $NAMESPACE
  labels:
    app.kubernetes.io/name: instantdeploy
EOF
)
    log_info "✓ Namespace created/exists"
}

# Create secrets
create_secrets() {
    log_info "Creating secrets..."
    
    # PostgreSQL secret
    kubectl create secret generic instantdeploy-postgres-secret \
        -n "$NAMESPACE" \
        --from-literal=POSTGRES_USER=postgres \
        --from-literal=POSTGRES_PASSWORD=postgres \
        --from-literal=POSTGRES_DB=instantdeploy \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Backend secret
    kubectl create secret generic instantdeploy-backend-secret \
        -n "$NAMESPACE" \
        --from-literal=JWT_SECRET=$(openssl rand -base64 32) \
        --from-literal=GITHUB_TOKEN="${GITHUB_TOKEN:-}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Image pull secret if using private registry
    if [ -n "$REGISTRY_URL" ] && [ -n "$DOCKER_USERNAME" ] && [ -n "$DOCKER_PASSWORD" ]; then
        kubectl create secret docker-registry instantdeploy-registry \
            -n "$NAMESPACE" \
            --docker-server="$REGISTRY_URL" \
            --docker-username="$DOCKER_USERNAME" \
            --docker-password="$DOCKER_PASSWORD" \
            --dry-run=client -o yaml | kubectl apply -f -
    fi
    
    log_info "✓ Secrets created"
}

# Deploy using kustomize
deploy_kustomize() {
    log_info "Deploying with kustomize..."
    
    cd "$K8S_DIR"
    
    if [ -n "$REGISTRY_URL" ]; then
        kustomize edit set image \
            instantdeploy-backend="$REGISTRY_URL/instantdeploy-backend:latest" \
            instantdeploy-frontend="$REGISTRY_URL/instantdeploy-frontend:latest"
    fi
    
    kustomize build . | kubectl apply -f -
    
    log_info "✓ Deployment applied"
}

# Wait for rollout
wait_rollout() {
    log_info "Waiting for deployments to be ready..."
    
    kubectl rollout status deployment/instantdeploy-backend -n "$NAMESPACE" --timeout=5m
    kubectl rollout status deployment/instantdeploy-frontend -n "$NAMESPACE" --timeout=5m
    kubectl rollout status deployment/instantdeploy-postgres -n "$NAMESPACE" --timeout=5m
    
    log_info "✓ All deployments ready"
}

# Verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    BACKEND_READY=$(kubectl get deployment instantdeploy-backend -n "$NAMESPACE" -o=jsonpath='{.status.readyReplicas}')
    FRONTEND_READY=$(kubectl get deployment instantdeploy-frontend -n "$NAMESPACE" -o=jsonpath='{.status.readyReplicas}')
    POSTGRES_READY=$(kubectl get deployment instantdeploy-postgres -n "$NAMESPACE" -o=jsonpath='{.status.readyReplicas}')
    
    if [ "$BACKEND_READY" -gt 0 ] && [ "$FRONTEND_READY" -gt 0 ] && [ "$POSTGRES_READY" -gt 0 ]; then
        log_info "✓ Deployment verified successfully"
        
        # Show access info
        log_info ""
        log_info "Deployment Summary:"
        log_info "- Backend: $BACKEND_READY replicas ready"
        log_info "- Frontend: $FRONTEND_READY replicas ready"
        log_info "- Database: $POSTGRES_READY replicas ready"
        log_info ""
        log_info "Access your deployment:"
        BACKEND_SERVICE=$(kubectl get service instantdeploy-backend -n "$NAMESPACE" -o=jsonpath='{.status.loadBalancer.ingress[0].hostname}:{.spec.ports[0].port}' 2>/dev/null || echo "kubectl port-forward --address=0.0.0.0 svc/instantdeploy-backend 8080:8080")
        log_info "- Backend API: http://$BACKEND_SERVICE"
        log_info "- Frontend: http://$DOMAIN (requires ingress)"
        return 0
    else
        log_error "Deployment verification failed"
        log_error "Backend: $BACKEND_READY/1, Frontend: $FRONTEND_READY/1, Database: $POSTGRES_READY/1"
        return 1
    fi
}

# Main
main() {
    log_info "InstantDeploy Kubernetes Deployment"
    log_info "======================================"
    
    check_prerequisites
    set_context
    create_namespace
    create_secrets
    build_images
    deploy_kustomize
    wait_rollout
    verify_deployment
    
    log_info "✓ Deployment complete!"
}

main "$@"
