#!/bin/bash
# Performance and load testing script using k6

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "$SCRIPT_DIR/../tests/performance" && pwd)"

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
TEST_DURATION="${TEST_DURATION:-17m}"  # Total test duration
USERS="${USERS:-100}"  # Max concurrent users
OUTPUT_FILE="${OUTPUT_FILE:-k6-results.json}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    command -v k6 &>/dev/null || {
        log_error "k6 not found. Install from: https://k6.io/docs/getting-started/installation/"
        exit 1
    }
    
    [ -f "$TESTS_DIR/load-test.js" ] || {
        log_error "Load test file not found: $TESTS_DIR/load-test.js"
        exit 1
    }
    
    log_info "✓ Prerequisites met"
}

# Health check
health_check() {
    log_info "Performing health check on $BASE_URL..."
    
    if ! curl -s "$BASE_URL/api/v1/health" > /dev/null; then
        log_error "Backend not responding at $BASE_URL"
        exit 1
    fi
    
    log_info "✓ Backend is healthy"
}

# Run load test
run_load_test() {
    log_info "Starting load test..."
    log_info "Configuration:"
    log_info "  Base URL: $BASE_URL"
    log_info "  Max users: $USERS"
    log_info "  Duration: $TEST_DURATION"
    log_info "  Output: $OUTPUT_FILE"
    log_info ""
    
    cd "$TESTS_DIR"
    
    k6 run \
        --vus "$USERS" \
        --duration "$TEST_DURATION" \
        --out json="$OUTPUT_FILE" \
        --env BASE_URL="$BASE_URL" \
        load-test.js
    
    log_info "✓ Load test completed"
}

# Analyze results
analyze_results() {
    log_info "Analyzing results..."
    
    # Extract key metrics from JSON output
    local results_file="${TESTS_DIR}/${OUTPUT_FILE}"
    
    if [ ! -f "$results_file" ]; then
        log_warn "Results file not found: $results_file"
        return
    fi
    
    # Summary statistics
    log_info ""
    log_info "Test Results Summary:"
    log_info "======================================"
    
    # Parse JSON for metrics
    cat "$results_file" | jq '.metrics[] | select(.type=="Trend") | {metric: .name[5:-5], values: .values} | select(.metric != "body_size") | select(.metric != "blocked") | select(.metric != "connecting")' 2>/dev/null | head -30
    
    log_info ""
    log_info "Detailed results in: $results_file"
}

# Generate HTML report
generate_report() {
    log_info "Generating HTML report..."
    
    local html_file="$(dirname "$OUTPUT_FILE")/report.html"
    
    cat > "$html_file" <<'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>InstantDeploy Performance Test Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@3/dist/chart.min.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        .metric { border: 1px solid #ddd; padding: 10px; margin: 10px 0; }
        .pass { background: #d4edda; }
        .fail { background: #f8d7da; }
        table { width: 100%; border-collapse: collapse; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background: #f0f0f0; }
        .chart-container { position: relative; width: 90%; height: 400px; margin: 20px 0; }
    </style>
</head>
<body>
    <h1>InstantDeploy Performance Test Report</h1>
    <p>Generated: <span id="timestamp"></span></p>
    
    <h2>Key Metrics</h2>
    <table>
        <tr>
            <th>Metric</th>
            <th>Target</th>
            <th>Result</th>
            <th>Status</th>
        </tr>
        <tr>
            <td>95th Percentile Latency</td>
            <td>&lt; 500ms</td>
            <td><span id="p95">Loading...</span></td>
            <td><span id="p95-status">-</span></td>
        </tr>
        <tr>
            <td>Error Rate</td>
            <td>&lt; 10%</td>
            <td><span id="error-rate">Loading...</span></td>
            <td><span id="error-status">-</span></td>
        </tr>
        <tr>
            <td>Throughput</td>
            <td>&gt; 10 req/s</td>
            <td><span id="throughput">Loading...</span></td>
            <td><span id="throughput-status">-</span></td>
        </tr>
    </table>
    
    <div class="chart-container">
        <canvas id="latencyChart"></canvas>
    </div>
    
    <div class="chart-container">
        <canvas id="throughputChart"></canvas>
    </div>
    
    <script>
        document.getElementById('timestamp').textContent = new Date().toLocaleString();
        
        // Placeholder for actual data
        console.log('Load JSON results to populate charts');
    </script>
</body>
</html>
EOF
    
    log_info "✓ Report generated: $html_file"
}

# Main
main() {
    log_info "InstantDeploy Performance Testing"
    log_info "===================================="
    log_info ""
    
    check_prerequisites
    health_check
    run_load_test
    analyze_results
    generate_report
    
    log_info ""
    log_info "✓ Performance testing complete!"
}

main "$@"
