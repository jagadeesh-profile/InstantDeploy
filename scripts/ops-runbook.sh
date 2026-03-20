#!/bin/bash
# Production Operations Runbook
# Quick reference for common operational tasks

set -e

NAMESPACE="instantdeploy"
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

section() { echo -e "\n${BLUE}═══════════════════════════════════${NC}\n$1\n${BLUE}═══════════════════════════════════${NC}\n"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }
info() { echo -e "${BLUE}ℹ${NC} $1"; }

# Command: ./ops-runbook.sh status
cmd_status() {
    section "System Status"
    
    echo "Deployment Status:"
    kubectl get deployments -n "$NAMESPACE"
    
    echo -e "\nPod Status:"
    kubectl get pods -n "$NAMESPACE"
    
    echo -e "\nService Status:"
    kubectl get svc -n "$NAMESPACE"
    
    echo -e "\nReplica Status:"
    kubectl top pods -n "$NAMESPACE" 2>/dev/null || warn "Metrics server not available"
}

# Command: ./ops-runbook.sh logs [backend|frontend]
cmd_logs() {
    local component="${1:-backend}"
    section "Logs for $component"
    
    kubectl logs "deployment/instantdeploy-$component" -n "$NAMESPACE" --tail=100
}

# Command: ./ops-runbook.sh metrics
cmd_metrics() {
    section "Performance Metrics"
    
    info "Access Grafana at: http://localhost:3000"
    info ""
    info "Key queries:"
    echo "  • Requests/sec: rate(http_request_total[1m])"
    echo "  • Error rate: rate(http_request_total{status=~\"5..\"}[1m])"
    echo "  • P95 Latency: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[1m]))"
    echo "  • Pod memory: container_memory_usage_bytes{pod=~\"instantdeploy-.*\"}"
    echo "  • Pod CPU: container_cpu_usage_seconds_total{pod=~\"instantdeploy-.*\"}"
}

# Command: ./ops-runbook.sh restart [component]
cmd_restart() {
    local component="${1:-all}"
    section "Restarting $component"
    
    if [ "$component" = "all" ]; then
        kubectl rollout restart deployment/instantdeploy-backend -n "$NAMESPACE"
        kubectl rollout restart deployment/instantdeploy-frontend -n "$NAMESPACE"
        success "All services restarted"
    else
        kubectl rollout restart "deployment/instantdeploy-$component" -n "$NAMESPACE"
        success "$component restarted"
    fi
    
    sleep 3
    echo -e "\nWaiting for rollout..."
    kubectl rollout status "deployment/instantdeploy-$component" -n "$NAMESPACE"
}

# Command: ./ops-runbook.sh scale <component> <replicas>
cmd_scale() {
    local component=$1
    local replicas=$2
    
    if [ -z "$component" ] || [ -z "$replicas" ]; then
        error "Usage: ./ops-runbook.sh scale <component> <replicas>"
        echo "Example: ./ops-runbook.sh scale backend 5"
        exit 1
    fi
    
    section "Scaling $component to $replicas replicas"
    
    kubectl scale deployment "instantdeploy-$component" --replicas="$replicas" -n "$NAMESPACE"
    success "Scaled to $replicas replicas"
}

# Command: ./ops-runbook.sh health
cmd_health() {
    section "Health Check"
    
    info "Checking backend health..."
    kubectl get pods -n "$NAMESPACE" -l app=instantdeploy-backend -o wide
    
    echo ""
    BACKEND_POD=$(kubectl get pod -n "$NAMESPACE" -l app=instantdeploy-backend -o jsonpath='{.items[0].metadata.name}')
    if [ -n "$BACKEND_POD" ]; then
        if kubectl exec -it "$BACKEND_POD" -n "$NAMESPACE" -- curl -s http://localhost:8080/api/v1/health; then
            success "Backend health check passed"
        else
            error "Backend health check failed"
        fi
    fi
    
    echo ""
    info "Checking frontend health..."
    kubectl get pods -n "$NAMESPACE" -l app=instantdeploy-frontend -o wide
}

# Command: ./ops-runbook.sh rollback [component]
cmd_rollback() {
    local component="${1:-all}"
    section "Rollback: $component"
    
    warn "Rolling back to previous deployment..."
    
    if [ "$component" = "all" ]; then
        kubectl rollout undo deployment/instantdeploy-backend -n "$NAMESPACE"
        kubectl rollout undo deployment/instantdeploy-frontend -n "$NAMESPACE"
        success "All services rolled back"
    else
        kubectl rollout undo "deployment/instantdeploy-$component" -n "$NAMESPACE"
        success "$component rolled back"
    fi
    
    sleep 3
    echo -e "\nWaiting for rollout..."
    kubectl rollout status "deployment/instantdeploy-$component" -n "$NAMESPACE" || true
}

# Command: ./ops-runbook.sh events
cmd_events() {
    section "Recent Events"
    
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -20
}

# Command: ./ops-runbook.sh describe <resource>
cmd_describe() {
    local resource=$1
    if [ -z "$resource" ]; then
        error "Usage: ./ops-runbook.sh describe <pod|deployment|service>"
        exit 1
    fi
    
    section "Describing $resource"
    kubectl describe "$resource" -n "$NAMESPACE"
}

# Command: ./ops-runbook.sh exec <pod> [command]
cmd_exec() {
    local pod=$1
    local command="${2:-/bin/sh}"
    
    if [ -z "$pod" ]; then
        error "Usage: ./ops-runbook.sh exec <pod> [command]"
        exit 1
    fi
    
    section "Executing in $pod"
    kubectl exec -it "$pod" -n "$NAMESPACE" -- $command
}

# Command: ./ops-runbook.sh history <component>
cmd_history() {
    local component="${1:-backend}"
    section "Rollout History: $component"
    
    kubectl rollout history "deployment/instantdeploy-$component" -n "$NAMESPACE"
}

# Command: ./ops-runbook.sh usage
usage() {
    cat <<EOF
${BLUE}InstantDeploy Operations Runbook${NC}

Usage: ./ops-runbook.sh [command] [args]

Commands:
  status              Show system status (pods, services, deployments)
  logs [component]    View logs for backend or frontend
  metrics             Show how to access performance metrics
  health              Run health checks
  restart [component] Restart backend, frontend, or all
  scale <comp> <n>    Scale component to N replicas
  rollback [comp]     Rollback to previous deployment
  history [comp]      Show deployment history
  events              Show recent cluster events
  describe <resource> Describe resource details
  exec <pod> [cmd]    Execute command in pod
  help                Show this help message

Examples:
  ./ops-runbook.sh status
  ./ops-runbook.sh logs backend
  ./ops-runbook.sh scale backend 5
  ./ops-runbook.sh restart frontend
  ./ops-runbook.sh rollback backend
  ./ops-runbook.sh exec instantdeploy-backend-xxx /bin/sh

EOF
}

# Main dispatcher
main() {
    local cmd="${1:-help}"
    
    case "$cmd" in
        status) cmd_status ;;
        logs) cmd_logs "$2" ;;
        metrics) cmd_metrics ;;
        health) cmd_health ;;
        restart) cmd_restart "$2" ;;
        scale) cmd_scale "$2" "$3" ;;
        rollback) cmd_rollback "$2" ;;
        history) cmd_history "$2" ;;
        events) cmd_events ;;
        describe) cmd_describe "$2" ;;
        exec) cmd_exec "$2" "$3" ;;
        help) usage ;;
        *)
            error "Unknown command: $cmd"
            echo ""
            usage
            exit 1
            ;;
    esac
}

main "$@"
