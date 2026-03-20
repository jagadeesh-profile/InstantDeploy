#!/bin/bash
# Deploy monitoring stack (Prometheus, Loki, Grafana) to Kubernetes

set -e

NAMESPACE="monitoring"
K8S_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../infrastructure/k8s" && pwd)"

log_info() { echo -e "\033[0;32m[INFO]\033[0m $1"; }
log_error() { echo -e "\033[0;31m[ERROR]\033[0m $1"; }

log_info "Deploying monitoring stack to namespace: $NAMESPACE"

# Create namespace
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $NAMESPACE
  labels:
    monitoring: "true"
EOF

# Deploy monitoring components
log_info "Deploying Prometheus..."
kubectl apply -f "$K8S_DIR/monitoring/prometheus.yaml"

log_info "Deploying Loki..."
kubectl apply -f "$K8S_DIR/monitoring/loki.yaml"

log_info "Deploying Grafana..."
kubectl apply -f "$K8S_DIR/monitoring/grafana.yaml"

# Wait for deployments
log_info "Waiting for deployments to be ready..."
kubectl rollout status deployment/prometheus -n $NAMESPACE --timeout=5m
kubectl rollout status deployment/loki -n $NAMESPACE --timeout=5m
kubectl rollout status deployment/grafana -n $NAMESPACE --timeout=5m

log_info "✓ Monitoring stack deployed successfully!"
log_info ""
log_info "Access your monitoring tools:"
log_info "- Prometheus: kubectl port-forward -n monitoring svc/prometheus 9090:9090"
log_info "- Grafana:    kubectl port-forward -n monitoring svc/grafana 3000:3000"
log_info "- Loki:       kubectl port-forward -n monitoring svc/loki 3100:3100"
log_info ""
log_info "Grafana login: admin / admin1234 (CHANGE THIS PASSWORD!)"
