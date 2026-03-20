#!/bin/bash
# Kubernetes Load Testing Runner
# Executes k6 load tests in the Kubernetes cluster

set -e

NAMESPACE="${NAMESPACE:-instantdeploy}"
TEST_TYPE="${1:-load}"  # load, stress, or performance
RESULTS_DIR="${RESULTS_DIR:-./test-results}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_debug() { echo -e "${BLUE}[DEBUG]${NC} $1"; }

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    command -v kubectl &>/dev/null || {
        log_error "kubectl not found"
        exit 1
    }
    
    # Check cluster connection
    kubectl cluster-info &>/dev/null || {
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    }
    
    # Check namespace exists
    kubectl get namespace "$NAMESPACE" &>/dev/null || {
        log_error "Namespace $NAMESPACE not found"
        exit 1
    }
    
    log_info "✓ Prerequisites met"
}

# Create results directory
setup_results() {
    mkdir -p "$RESULTS_DIR"
    log_info "Results directory: $RESULTS_DIR"
}

# Deploy load test ConfigMap if not exists
deploy_configmap() {
    log_info "Ensuring ConfigMap is deployed..."
    
    if ! kubectl get configmap load-test-config -n "$NAMESPACE" &>/dev/null; then
        log_info "Deploying load test configuration..."
        kubectl apply -f infrastructure/k8s/testing/load-test-jobs.yaml -n "$NAMESPACE"
    else
        log_info "✓ ConfigMap already exists"
    fi
}

# Run load test
run_load_test() {
    log_info "Starting load test..."
    log_info "Configuration:"
    log_info "  Type: load"
    log_info "  Users: 50"
    log_info "  Duration: 14m"
    log_info "  Namespace: $NAMESPACE"
    
    # Create job
    kubectl create -f - -n "$NAMESPACE" <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: load-test-$(date +%s)
  namespace: $NAMESPACE
spec:
  backoffLimit: 2
  template:
    spec:
      serviceAccountName: default
      containers:
      - name: k6
        image: loadimpact/k6:latest
        command:
        - k6
        - run
        - --vus
        - "50"
        - --duration
        - "14m"
        - --out
        - json=/tmp/results.json
        - /config/load-test.js
        volumeMounts:
        - name: config
          mountPath: /config
        - name: results
          mountPath: /tmp
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
      volumes:
      - name: config
        configMap:
          name: load-test-config
      - name: results
        emptyDir: {}
      restartPolicy: Never
EOF
    
    local job_name=$(kubectl get job -n "$NAMESPACE" -o jsonpath='{.items[-1].metadata.name}')
    log_info "Job created: $job_name"
    
    # Wait for job to complete
    wait_for_job "$job_name"
    
    # Extract results
    extract_results "$job_name"
}

# Run stress test
run_stress_test() {
    log_info "Starting stress test..."
    log_info "Configuration:"
    log_info "  Type: stress"
    log_info "  Max Users: 200"
    log_info "  Ramp: 2m:100 → 5m:200 → 2m:100 → 1m:0"
    log_info "  Namespace: $NAMESPACE"
    
    kubectl create -f - -n "$NAMESPACE" <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: stress-test-$(date +%s)
  namespace: $NAMESPACE
spec:
  backoffLimit: 2
  template:
    spec:
      serviceAccountName: default
      containers:
      - name: k6-stress
        image: loadimpact/k6:latest
        command:
        - k6
        - run
        - --vus
        - "100"
        - --duration
        - "10m"
        - --stage
        - "2m:100"
        - --stage
        - "5m:200"
        - --stage
        - "2m:100"
        - --stage
        - "1m:0"
        - --out
        - json=/tmp/stress-results.json
        - /config/load-test.js
        volumeMounts:
        - name: config
          mountPath: /config
        - name: results
          mountPath: /tmp
        resources:
          requests:
            cpu: 1000m
            memory: 1Gi
          limits:
            cpu: 2000m
            memory: 2Gi
      volumes:
      - name: config
        configMap:
          name: load-test-config
      - name: results
        emptyDir: {}
      restartPolicy: Never
EOF
    
    local job_name=$(kubectl get job -n "$NAMESPACE" -o jsonpath='{.items[-1].metadata.name}')
    log_info "Job created: $job_name"
    
    wait_for_job "$job_name"
    extract_results "$job_name"
}

# Wait for job to complete
wait_for_job() {
    local job_name=$1
    local timeout=1800  # 30 minutes
    local elapsed=0
    local interval=10
    
    log_info "Waiting for job to complete..."
    
    while [ $elapsed -lt $timeout ]; do
        local status=$(kubectl get job "$job_name" -n "$NAMESPACE" -o jsonpath='{.status.conditions[].type}')
        
        if [ "$status" == "Complete" ]; then
            log_info "✓ Job completed successfully"
            return 0
        elif [ "$status" == "Failed" ]; then
            log_error "Job failed"
            kubectl logs -n "$NAMESPACE" -l job-name="$job_name" --tail=50
            return 1
        fi
        
        # Show progress
        local active=$(kubectl get job "$job_name" -n "$NAMESPACE" -o jsonpath='{.status.active}')
        if [ -n "$active" ]; then
            log_debug "Job running ($elapsed/$timeout seconds)"
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    log_error "Job timeout after ${timeout}s"
    return 1
}

# Extract results from pod
extract_results() {
    local job_name=$1
    local pod_name=$(kubectl get pod -n "$NAMESPACE" -l "job-name=$job_name" -o jsonpath='{.items[0].metadata.name}')
    
    if [ -z "$pod_name" ]; then
        log_warn "Could not find pod for job $job_name"
        return
    fi
    
    log_info "Extracting results from pod: $pod_name"
    
    # Copy results from pod
    kubectl cp "$NAMESPACE/$pod_name:/tmp/results.json" "$RESULTS_DIR/$job_name.json" 2>/dev/null || \
    kubectl cp "$NAMESPACE/$pod_name:/tmp/stress-results.json" "$RESULTS_DIR/$job_name.json" 2>/dev/null || \
    log_warn "Could not extract results file"
    
    # Show pod logs
    log_info "Last 30 lines of pod logs:"
    kubectl logs -n "$NAMESPACE" "$pod_name" --tail=30 || true
    
    if [ -f "$RESULTS_DIR/$job_name.json" ]; then
        log_info "✓ Results saved: $RESULTS_DIR/$job_name.json"
        
        # Parse key metrics
        log_info "Key Metrics:"
        cat "$RESULTS_DIR/$job_name.json" | jq '.metrics[] | select(.type=="Trend") | {metric: .name, p95: .values.p95, max: .values.max}' 2>/dev/null | head -20 || true
    fi
}

# Monitor test in real-time
monitor_test() {
    log_info "Monitoring test metrics..."
    log_info "Prometheus queries:"
    log_info "  Requests/sec: rate(http_request_total[1m])"
    log_info "  Error rate: rate(http_request_total{status=~\"5..\"}[1m])"
    log_info "  P95 Latency: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[1m]))"
    log_info "  Pod Memory: container_memory_usage_bytes{pod=~\"instantdeploy-backend-.*\"}"
    log_info ""
    log_info "Access Grafana at: http://localhost:3000"
}

# Show usage
usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
  load              Run standard load test (50 users, 14 minutes)
  stress            Run stress test (up to 200 users)
  monitor           Monitor test metrics in Prometheus/Grafana
  list              List recent test jobs
  clean             Delete completed test jobs

Environment Variables:
  NAMESPACE         Kubernetes namespace (default: instantdeploy)
  RESULTS_DIR       Directory to save results (default: ./test-results)

Examples:
  $0 load
  $0 stress
  NAMESPACE=staging $0 load
  RESULTS_DIR=/tmp/results $0 stress
EOF
}

# List recent tests
list_tests() {
    log_info "Recent test jobs:"
    kubectl get job -n "$NAMESPACE" -o wide | grep -E "load-test|stress-test" || \
    log_warn "No test jobs found"
}

# Clean up
cleanup_tests() {
    log_info "Cleaning up old test jobs..."
    
    kubectl delete job -n "$NAMESPACE" \
        -l "app in (load-test, stress-test)" \
        --ignore-not-found=true
    
    log_info "✓ Cleanup complete"
}

# Main
main() {
    case "${TEST_TYPE:-help}" in
        load)
            check_prerequisites
            setup_results
            deploy_configmap
            run_load_test
            monitor_test
            ;;
        stress)
            check_prerequisites
            setup_results
            deploy_configmap
            run_stress_test
            monitor_test
            ;;
        monitor)
            monitor_test
            ;;
        list)
            check_prerequisites
            list_tests
            ;;
        clean)
            check_prerequisites
            cleanup_tests
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

main "$@"
