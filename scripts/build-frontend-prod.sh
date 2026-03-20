#!/bin/bash
# Frontend production build and deployment script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$PROJECT_DIR/frontend"

# Configuration
API_URL="${API_URL:-https://api.instantdeploy.example.com}"
WS_URL="${WS_URL:-wss://api.instantdeploy.example.com}"
REGISTRY="${REGISTRY:-docker.io}"
IMAGE_NAME="${IMAGE_NAME:-instantdeploy-frontend}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
DOCKER_PUSH="${DOCKER_PUSH:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Show configuration
log_info "Frontend Build Configuration"
log_info "=============================="
log_info "API URL: $API_URL"
log_info "WebSocket URL: $WS_URL"
log_info "Image: $REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
log_info ""

# Build frontend assets
build_frontend() {
    log_info "Building frontend assets..."
    cd "$FRONTEND_DIR"
    
    npm ci
    
    # Build with environment variables
    VITE_API_URL="$API_URL" \
    VITE_WS_URL="$WS_URL" \
    VITE_ENV=production \
    npm run build
    
    log_info "✓ Frontend build complete"
}

# Build Docker image
build_docker() {
    log_info "Building Docker image: $REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
    
    cd "$PROJECT_DIR"
    
    docker build \
        -f frontend/Dockerfile.prod \
        -t "$REGISTRY/$IMAGE_NAME:$IMAGE_TAG" \
        -t "$REGISTRY/$IMAGE_NAME:latest" \
        --build-arg VITE_API_URL="$API_URL" \
        --build-arg VITE_WS_URL="$WS_URL" \
        .
    
    log_info "✓ Docker image built"
}

# Push to registry
push_docker() {
    if [ "$DOCKER_PUSH" != "true" ]; then
        log_info "Skipping Docker push (set DOCKER_PUSH=true to enable)"
        return
    fi
    
    log_info "Pushing Docker image to registry..."
    docker push "$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
    docker push "$REGISTRY/$IMAGE_NAME:latest"
    log_info "✓ Docker image pushed"
}

# Validate build
validate() {
    log_info "Validating build..."
    
    if [ ! -d "$FRONTEND_DIR/dist" ]; then
        log_error "Distribution directory not found: $FRONTEND_DIR/dist"
        return 1
    fi
    
    if [ ! -f "$FRONTEND_DIR/dist/index.html" ]; then
        log_error "index.html not found in distribution"
        return 1
    fi
    
    # Check bundle size
    BUNDLE_SIZE=$(du -sh "$FRONTEND_DIR/dist" | cut -f1)
    log_info "✓ Build artifacts: $BUNDLE_SIZE"
    
    # Count files
    FILE_COUNT=$(find "$FRONTEND_DIR/dist" -type f | wc -l)
    log_info "✓ Distribution files: $FILE_COUNT"
}

# Generate deployment manifest
generate_manifest() {
    log_info "Generating deployment manifest..."
    
    cat > "$PROJECT_DIR/frontend-deployment.yaml" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: instantdeploy-frontend
  namespace: instantdeploy
spec:
  replicas: 2
  selector:
    matchLabels:
      app: instantdeploy-frontend
  template:
    metadata:
      labels:
        app: instantdeploy-frontend
    spec:
      containers:
        - name: frontend
          image: $REGISTRY/$IMAGE_NAME:$IMAGE_TAG
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 80
              name: http
            - containerPort: 8080
              name: metrics
          env:
            - name: VITE_API_URL
              value: "$API_URL"
            - name: VITE_WS_URL
              value: "$WS_URL"
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 15
            periodSeconds: 20
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 250m
              memory: 256Mi
EOF
    
    log_info "✓ Manifest generated: frontend-deployment.yaml"
}

# Main
main() {
    build_frontend
    validate
    build_docker
    push_docker
    generate_manifest
    
    log_info ""
    log_info "✓ Frontend production build complete!"
    log_info ""
    log_info "Next steps:"
    log_info "1. Deploy to Kubernetes: kubectl apply -f frontend-deployment.yaml"
    log_info "2. Or update existing deployment: kubectl set image deployment/instantdeploy-frontend frontend=$REGISTRY/$IMAGE_NAME:$IMAGE_TAG -n instantdeploy"
}

main "$@"
