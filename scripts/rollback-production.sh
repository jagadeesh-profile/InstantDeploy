#!/bin/bash
# Production Rollback Script
# Reverts to previous stable deployment

set -e

NAMESPACE="${NAMESPACE:-instantdeploy}"
TIMEOUT="${TIMEOUT:-600}"
SLACK_WEBHOOK="${SLACK_WEBHOOK:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }

# Check prerequisites
check_prerequisites() {
    command -v kubectl &>/dev/null || {
        log_error "kubectl not found"
        exit 1
    }
}

# Get previous revision
get_previous_revision() {
    local deployment=$1
    local revisions=$(kubectl rollout history deployment/"$deployment" -n "$NAMESPACE" | tail -2 | head -1 | awk '{print $1}')
    echo "$revisions"
}

# Rollback deployment
rollback_deployment() {
    local deployment=$1
    local revision=$2
    
    log_info "Rolling back $deployment to revision $revision..."
    
    kubectl rollout undo deployment/"$deployment" --to-revision="$revision" -n "$NAMESPACE"
    
    # Wait for rollback to complete
    kubectl rollout status deployment/"$deployment" -n "$NAMESPACE" --timeout="${TIMEOUT}s"
}

# Send notification
send_notification() {
    local status=$1
    local message=$2
    
    if [ -z "$SLACK_WEBHOOK" ]; then
        return
    fi
    
    curl -X POST "$SLACK_WEBHOOK" \
        -H 'Content-type: application/json' \
        --data "{
            \"text\": \"Production Rollback: $status\",
            \"attachments\": [{
                \"color\": \"$([ \"$status\" = \"Success\" ] && echo 'good' || echo 'danger')\",
                \"text\": \"$message\",
                \"timestamp\": $(date +%s)
            }]
        }" || true
}

# Main rollback
main() {
    log_info "Starting production rollback..."
    
    check_prerequisites
    
    # Rollback backend
    BACKEND_PREV=$(get_previous_revision "instantdeploy-backend")
    if [ -n "$BACKEND_PREV" ]; then
        rollback_deployment "instantdeploy-backend" "$BACKEND_PREV"
        log_info "✓ Backend rolled back to revision $BACKEND_PREV"
    fi
    
    # Rollback frontend
    FRONTEND_PREV=$(get_previous_revision "instantdeploy-frontend")
    if [ -n "$FRONTEND_PREV" ]; then
        rollback_deployment "instantdeploy-frontend" "$FRONTEND_PREV"
        log_info "✓ Frontend rolled back to revision $FRONTEND_PREV"
    fi
    
    # Health checks
    log_info "Running health checks..."
    sleep 10
    
    if kubectl exec -it svc/instantdeploy-backend -n "$NAMESPACE" -- curl -s http://localhost:8080/api/v1/health | jq .status | grep -q '"ok"'; then
        log_info "✓ Backend health check passed"
        send_notification "Success" "Production rollback completed successfully. Backend and Frontend are healthy."
    else
        log_error "Backend health check failed"
        send_notification "Failed" "Production rollback, but health checks failed. Manual intervention required."
        exit 1
    fi
    
    log_info "✓ Production rollback completed successfully"
}

main "$@"
