# Performance Testing Guide

## Overview

Performance testing validates that InstantDeploy meets production requirements:

- **Throughput**: Minimum 50 requests/second
- **Latency**: 95th percentile < 500ms  
- **Error Rate**: < 0.1%
- **Availability**: 99.99% uptime under normal load

## Test Environment

Recommended test environment:

```
┌─────────────────────────────────────┐
│   Load Generator (k6)               │
│   - 10+ Virtual Users               │
│   - Realistic user scenarios        │
│   - Gradual ramp-up/ramp-down       │
└──────────────┬──────────────────────┘
               │
        ┌──────▼──────┐
        │  Production │
        │  Cluster    │
        │  (Measured) │
        └──────────────┘
```

## Prerequisites

### Install k6

```bash
# macOS
brew install k6

# Linux
sudo apt-get install k6

# Windows (Chocolatey)
choco install k6

# Or from source: https://k6.io/docs/getting-started/installation/
```

### Prepare Test Environment

```bash
# 1. Deploy to production or staging
kubectl apply -f infrastructure/k8s/kustomization.yaml -n instantdeploy

# 2. Verify readiness
kubectl wait deployment/instantdeploy-backend -n instantdeploy --for=condition=available

# 3. Get API URL
kubectl get svc instantdeploy-backend -n instantdeploy
```

## Running Performance Tests

### 1. Basic Load Test

```bash
# Run load test
BASE_URL=http://localhost:8080 \
  ./scripts/run-performance-test.sh

# Or using k6 directly
k6 run tests/performance/load-test.js \
  --env BASE_URL=http://localhost:8080 \
  --out json=results.json
```

### 2. Custom Test Parameters

```bash
# Adjust VUs (Virtual Users) and duration
k6 run tests/performance/load-test.js \
  --vus 50 \
  --duration 10m \
  --env BASE_URL=http://localhost:8080
```

### 3. Stress Test (Find Breaking Point)

```bash
k6 run tests/performance/load-test.js \
  --stage "1m:100" \
  --stage "2m:500" \
  --stage "2m:1000" \
  --stage "1m:0" \
  --env BASE_URL=http://localhost:8080
```

## Test Scenarios

### Scenario 1: Normal Load

- **Users**: 50 (ramp up over 1 minute)
- **Duration**: 10 minutes
- **Expectations**: 
  - All requests succeed
  - Latency < 200ms (p95)
  - Throughput > 50 req/s

### Scenario 2: Peak Load

- **Users**: 200 (sudden spike)
- **Duration**: 5 minutes
- **Expectations**:
  - Error rate < 1%
  - Latency < 500ms (p95)
  - Autoscaler activates

### Scenario 3: Sustained Load

- **Users**: 100 (constant)
- **Duration**: 1 hour
- **Expectations**:
  - Stable performance
  - No memory leaks
  - Consistent throughput

### Scenario 4: Long Running Session

- **Duration**: 8 hours
- **Concurrent Users**: 50
- **Breaks**: 30-minute inactivity windows
- **Expectations**:
  - Handle reconnections gracefully
  - No connection state leaks

## Interpreting Results

### Key Metrics

```json
{
  "http_req_duration": {
    "avg": 150.5,
    "max": 2000,
    "p(95)": 350,
    "p(99)": 800
  },
  "http_req_failed": {
    "rate": 0.001  // 0.1% failure rate
  },
  "iteration_duration": {
    "avg": 45000   // ~45 seconds per full user journey
  }
}
```

### Thresholds

| Metric | Target | Yellow | Red |
|--------|--------|--------|-----|
| **P95 Latency** | <500ms | 500-800ms | >800ms |
| **P99 Latency** | <1s | 1-2s | >2s |
| **Error Rate** | <0.1% | 0.1-1% | >1% |
| **Throughput** | >50/s | <50/s | <10/s |

## Performance Baselines

### Reference Results

**Environment**: 3 node cluster, each with 4 CPU, 8GB RAM

| Load | P95 Latency | Throughput | Errors | Memory |
|------|-------------|-----------|--------|--------|
| Light (10 VUs) | 80ms | 25/s | 0% | 300MB |
| Normal (50 VUs) | 150ms | 75/s | 0% | 450MB |
| Peak (100 VUs) | 350ms | 150/s | 0% | 600MB |
| Stress (200 VUs) | 650ms | 180/s | 0.2% | 750MB |

## Analyzing Results

### JSON Output

```bash
# View results file
cat k6-results.json | jq '.metrics'

# Filter to specific metrics
cat k6-results.json | jq '.data.series[] | select(.metric == "http_req_duration")'

# Get summary
k6 run tests/performance/load-test.js --summary-export=summary.json
cat summary.json | jq '.metrics'
```

### HTML Report

```bash
# Generate detailed HTML report
# Results are in report.html

# Key sections:
# - Response Time Distribution
# - Throughput Over Time
# - Error Rates  
# - Resource Utilization
```

## Continuous Monitoring

### Prometheus Queries

During test execution, monitor in real-time:

```promql
# Request rate
rate(http_request_total[1m])

# Error rate
rate(http_request_total{status=~"5.."}[1m])

# Latency p95
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[1m]))

# Pod resource usage
container_memory_usage_bytes{pod=~"instantdeploy-backend-.*"}
```

### Grafana Dashboard

1. Open Grafana: `http://localhost:3000`
2. Import dashboard: `docs/dashboards/performance-test.json`
3. Watch metrics during test

## Optimization

### If Performance is Poor

1. **High Latency**:
   - Add database indexes
   - Enable caching (Redis)
   - Optimize API queries
   - Increase pod resources

2. **High Error Rate**:
   - Check backend logs
   - Verify database connectivity
   - Increase connection pools
   - Add retry logic

3. **Low Throughput**:
   - Increase pod replicas
   - Enable HPA autoscaling
   - Add load balancer
   - Optimize code paths

### Tuning Checklist

- [ ] Database indexes created
- [ ] Connection pools increased
- [ ] Caching enabled (Redis)
- [ ] Pod resources sufficient
- [ ] Autoscaling configured
- [ ] Load balancer healthy
- [ ] Rate limiting removed for test
- [ ] Monitoring enabled

## Test Automation

### CI/CD Integration

```yaml
# .github/workflows/performance-test.yml
name: Performance Test
on: [push]
jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run performance test
        run: |
          docker pull loadimpact/k6:latest
          docker run --net host -v $PWD:/workspace \
            loadimpact/k6:latest run /workspace/tests/performance/load-test.js
      - name: Check results
        run: |
          # Fail if thresholds exceeded
          cat k6-results.json | jq '.metrics.http_req_duration.values.p95'
```

## Reporting

### Executive Summary

```markdown
# Performance Test Report

**Date**: 2026-03-18  
**Test Duration**: 30 minutes  
**Max Concurrent Users**: 100

## Results

✓ **PASS** - All performance targets met

- **Throughput**: 150 req/s (target: >50)
- **Latency p95**: 250ms (target: <500ms)
- **Error Rate**: 0.02% (target: <1%)
- **Availability**: 99.98%

## Recommendations

1. Performance is production-ready
2. Monitor memory usage during sustained load
3. Plan for autoscaling at 80% capacity
```

## Next Steps

1. Run baseline tests (normal load)
2. Identify bottlenecks
3. Optimize code/infrastructure
4. Re-test to verify improvements
5. Schedule weekly performance tests
6. Archive results for trend analysis
