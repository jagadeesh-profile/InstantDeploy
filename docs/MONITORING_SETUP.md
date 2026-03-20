# Production Monitoring & Observability Setup

## Overview

This guide covers setting up comprehensive monitoring, logging, and alerting for InstantDeploy in production using:

- **Prometheus**: Metrics collection and time-series database
- **Loki**: Centralized log aggregation
- **Grafana**: Visualization and dashboarding
- **AlertManager**: Alert routing and notifications

## Quick Start

```bash
# Deploy monitoring stack
chmod +x scripts/deploy-monitoring.sh
./scripts/deploy-monitoring.sh

# Access monitoring tools
kubectl port-forward -n monitoring svc/prometheus 9090:9090
kubectl port-forward -n monitoring svc/grafana 3000:3000
kubectl port-forward -n monitoring svc/loki 3100:3100
```

## Architecture

```
┌──────────────────────────────────────────┐
│       Application Pods (Backend)         │
│    (emit metrics & logs)                 │
└──────────────┬───────────────────────────┘
               │
       ┌───────┴────────┐
       │                │
   ┌───▼────┐      ┌────▼─────┐
   │Promtail│      │Prometheus│
   │(logs)  │      │(metrics)  │
   └───┬────┘      └─────┬─────┘
       │                 │
   ┌───▼─────────────────▼───┐
   │ ┌──────┐   ┌──────────┐ │
   │ │ Loki │   │Prometheus│ │
   │ │      │   │  Server  │ │
   │ └──┬───┘   └────┬─────┘ │
   │    │            │        │
   │    └────┬───────┘        │
   │         │ (datasources) │
   │      ┌──▼────────┐      │
   │      │  Grafana  │      │
   │      │(dashboards)     │
   │      └───────────┘      │
   └────────────────────────┘
```

## Components

### Prometheus

Prometheus scrapes metrics from:
- Kubernetes API server
- Kubelet nodes
- Application pods (via annotations)
- Service monitors

**Configuration**: `infrastructure/k8s/monitoring/prometheus.yaml`

**Key metrics scraped**:
- `http_request_total` - Total requests
- `http_request_duration_seconds` - Request latency
- `http_requests_errors_total` - Error count
- Container resource usage (memory, CPU)
- Pod restart counts

**Retention**: 30 days

### Loki

Centralizes logs from all pods using Promtail DaemonSet.

**Configuration**: `infrastructure/k8s/monitoring/loki.yaml`

**Log labels**:
- `namespace` - Kubernetes namespace
- `pod` - Pod name
- `container` - Container name
- `app` - Application label
- `job` - Job/service name

**Storage**: Local filesystem (can be configured for S3, GCS, etc.)

### Grafana

Provides visualization with pre-built dashboards.

**Configuration**: `infrastructure/k8s/monitoring/grafana.yaml`

**Default credentials**:
- Username: `admin`
- Password: `admin1234` (CHANGE THIS!)

**Pre-configured dashboards**:
- InstantDeploy Overview
- Backend Performance
- Pod Resource Usage
- Error Rate & Logs

**Datasources**:
- Prometheus (for metrics)
- Loki (for logs)

## Instrumentation

### Backend Application Metrics

Add these endpoints to expose Prometheus metrics:

```go
// In backend/cmd/server/main.go
import "github.com/prometheus/client_golang/prometheus/promhttp"

// Register metrics handler
router.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)
```

Then annotate the Pod for scraping:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Key Metrics to Track

```
# Request metrics
http_request_total{method="GET",status="200",endpoint="/api/v1/health"}
http_request_duration_seconds{le="+Inf",endpoint="/api/v1/deployments"}

# Deployment metrics
deployment_build_duration_seconds_total
deployment_success_total
deployment_failure_total

# Resource metrics
container_memory_usage_bytes
container_cpu_usage_seconds_total
```

## Alerting

### Alert Rules

Located in `infrastructure/k8s/monitoring/prometheus.yaml` alerting rules section:

**Critical Alerts**:
- Backend service down (`BackendDown`)
- Pod crash looping (`PodCrashLooping`)
- High error rate (>5%) sustained for 5 minutes

**Warning Alerts**:
- High latency (p95 > 1s)
- High resource usage (>80% capacity)
- Deployment replica mismatch

### Configure AlertManager

Send notifications to Slack, PagerDuty, etc.:

```yaml
global:
  resolve_timeout: 5m

route:
  receiver: 'default'
  group_by: ['alertname', 'cluster']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  routes:
    - match:
        severity: critical
      receiver: 'critical'
      continue: true

receivers:
  - name: 'default'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#alerts'
  
  - name: 'critical'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/CRITICAL/WEBHOOK'
        channel: '#critical-alerts'
    pagerduty_configs:
      - service_key: 'YOUR-PAGERDUTY-KEY'
```

## Dashboard Queries

### Request Rate (per minute)
```
rate(http_request_total[1m])
```

### Error Rate (percentage)
```
100 * rate(http_requests_errors_total[5m]) / rate(http_request_total[5m])
```

### Latency (95th percentile)
```
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

### Pod CPU Usage
```
sum(rate(container_cpu_usage_seconds_total[5m])) by (pod, namespace)
```

### Pod Memory Usage
```
container_memory_usage_bytes
```

### Deployment Replicas Ready
```
kube_deployment_status_replicas_available / kube_deployment_spec_replicas
```

### Deployment Generation Mismatch
```
kube_deployment_status_observed_generation != kube_deployment_metadata_generation
```

## Logging Queries (LogQL)

### Get all logs from InstantDeploy namespace
```
{namespace="instantdeploy"}
```

### Errors only
```
{namespace="instantdeploy"} | "error"
```

### By pod
```
{namespace="instantdeploy", pod="instantdeploy-backend-xxx"}
```

### JSON parsing
```
{namespace="instantdeploy"} | json level="error"
```

### Error rate
```
sum(rate({namespace="instantdeploy"} |= "error" [5m]))
```

## Performance Tuning

### Prometheus Storage

Modify retention and storage in `prometheus.yaml`:

```yaml
args:
  - '--storage.tsdb.retention.time=30d'    # Keep 30 days of data
  - '--storage.tsdb.max-block-duration=2h'  # 2h blocks
```

### Reduce Scrape Load

```yaml
scrape_configs:
  - job_name: 'kubernetes-pods'
    scrape_interval: 30s    # Increase from 15s
    scrape_timeout: 10s
    metric_relabel_configs:
      # Drop unnecessary metrics
      - source_labels: [__name__]
        regex: 'container_network.*'
        action: drop
```

### Loki Performance

```yaml
ingester:
  max_chunk_age: 1h          # Flush chunks after 1 hour
  chunk_idle_period: 3m      # Force flush after 3m idle
```

## Troubleshooting

### Prometheus scrape targets failing

```bash
# Check Prometheus web UI
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Visit http://localhost:9090/targets

# Check logs
kubectl logs -n monitoring deployment/prometheus
```

### Grafana cannot connect to Prometheus

```bash
# Verify service DNS
kubectl exec -it -n monitoring deployment/grafana -- \
  nslookup prometheus.monitoring.svc.cluster.local

# Check firewall/network policies
kubectl get networkpolicies -n monitoring
```

### Loki ingestion rate limit hit

Increase in `loki-config.yml`:

```yaml
limits_config:
  ingestion_rate_mb: 10  # Increase from default
  ingestion_burst_size_mb: 20
```

### Missing logs

```bash
# Check Promtail DaemonSet
kubectl get ds -n monitoring promtail

# Check Promtail logs
kubectl logs -n monitoring ds/promtail -f

# Verify mount points
kubectl describe ds/promtail -n monitoring
```

## Backup & Restore

### Backup Grafana Dashboards

```bash
# Export all dashboards as JSON
kubectl exec -it -n monitoring deployment/grafana -- \
  curl -s http://localhost:3000/api/dashboards/db \
  -H"Authorization: Bearer YOUR_API_TOKEN" > dashboards_backup.json
```

### Backup Prometheus Data

```bash
# Create snapshot
kubectl exec -it -n monitoring deployment/prometheus -- \
  curl -XPOST http://localhost:9090/api/v1/admin/tsdb/snapshot
```

## Next Steps

1. **Customize dashboards** for your specific metrics
2. **Set up AlertManager** for notifications
3. **Configure long-term storage** (S3, GCS, etc.) for Prometheus/Loki
4. **Integrate with on-call platform** (PagerDuty, Opsgenie)
5. **Create runbooks** for common alerts
