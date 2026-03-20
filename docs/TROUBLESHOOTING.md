# Troubleshooting Guide

Comprehensive troubleshooting for InstantDeploy production deployment.

## Quick Diagnostics

```bash
# System health check
kubectl get nodes
kubectl get pods -n instantdeploy
kubectl get services -n instantdeploy
kubectl get ingress -n instantdeploy

# Event log
kubectl get events -n instantdeploy --sort-by='.lastTimestamp'
```

## Common Issues & Solutions

### 1. Pod Won't Start

**Symptoms**: Pod stuck in Pending, CreateContainerConfigError, or CrashLoopBackOff

```bash
# Diagnose
kubectl describe pod <pod-name> -n instantdeploy

# Common causes:
# - Image pull errors: Check registry credentials
# - Resource requests: Increase cluster resources
# - Mount errors: Check PVC and storage
# - Config errors: Validate secrets and configmaps
```

**Solutions**:

```bash
# Fix image pull errors
kubectl create secret docker-registry instantdeploy-registry \
  -n instantdeploy \
  --docker-server=<registry> \
  --docker-username=<user> \
  --docker-password=<pass>

# Check cluster resources
kubectl top nodes
kubectl describe nodes

# Check PVC status
kubectl get pvc -n instantdeploy
kubectl describe pvc <pvc-name> -n instantdeploy
```

### 2. Service Not Accessible

**Symptoms**: Cannot reach service via DNS or IP

```bash
# Diagnose
kubectl get svc -n instantdeploy
kubectl get endpoints -n instantdeploy
kubectl describe svc <service-name> -n instantdeploy

# Test DNS
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup instantdeploy-backend.instantdeploy.svc.cluster.local
```

**Solutions**:

```bash
# Verify endpoint exists
kubectl get endpoints instantdeploy-backend -n instantdeploy

# Check pod selector matches labels
kubectl get pods -n instantdeploy -L app

# Test from another pod
kubectl exec -it <pod> -n instantdeploy -- \
  curl http://instantdeploy-backend:8080/health
```

### 3. Database Connection Fails

**Symptoms**: Backend logs show "connection refused" or "FATAL: password authentication failed"

```bash
# Check postgres pod
kubectl get pods -n instantdeploy -l app=instantdeploy-postgres
kubectl logs -n instantdeploy deployment/instantdeploy-postgres

# Test connection
kubectl exec -it <postgres-pod> -n instantdeploy -- \
  psql -U postgres -d instantdeploy -c "SELECT version();"
```

**Solutions**:

```bash
# Verify secret exists  
kubectl get secret instantdeploy-postgres-secret -n instantdeploy
kubectl get secret instantdeploy-postgres-secret -n instantdeploy -o yaml

# Recreate secret with correct credentials
kubectl delete secret instantdeploy-postgres-secret -n instantdeploy
kubernetes create secret generic instantdeploy-postgres-secret \
  -n instantdeploy \
  --from-literal=POSTGRES_PASSWORD=<new-password>

# Restart backend to reconnect
kubectl rollout restart deployment/instantdeploy-backend -n instantdeploy
```

### 4. High Memory Usage

**Symptoms**: Pods killed due to OOM, performance degradation

```bash
# Check memory usage
kubectl top pods -n instantdeploy
kubectl top nodes

# Describe pod for memory limits
kubectl describe pod <pod-name> -n instantdeploy | grep -A 5 "Limits\|Requests"
```

**Solutions**:

```bash
# Increase memory limit
kubectl set resources deployment instantdeploy-backend \
  -n instantdeploy \
  --limits=memory=2Gi \
  --requests=memory=512Mi

# Check for memory leaks in application logs
kubectl logs -n instantdeploy deployment/instantdeploy-backend | grep -i "memory\|gc"

# Examine goroutine count (Go apps)
kubectl port-forward -n instantdeploy svc/instantdeploy-backend 8080:8080
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

### 5. Certificate Issues

**Symptoms**: SSL handshake failures, "certificate is invalid"

```bash
# Check certificate status
kubectl get certificate -n instantdeploy
kubectl describe certificate instantdeploy-cert -n instantdeploy

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager
```

**Solutions**:

```bash
# Delete and regenerate certificate
kubectl delete certificate instantdeploy-cert -n instantdeploy
kubectl apply -f infrastructure/k8s/ingress.yaml

# Force renewal
kubectl annotate certificate instantdeploy-cert \
  -n instantdeploy \
  cert-manager.io/issue-temporary-certificate=true \
  --overwrite

# Wait for renewal
kubectl wait certificate instantdeploy-cert -n instantdeploy --for=condition=Ready
```

### 6. Deployment Stuck Rolling Out

**Symptoms**: Deployment shows "x replicas available" but pods not all running

```bash
# Check rollout status
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy

# Check recent events
kubectl describe deployment instantdeploy-backend -n instantdeploy | tail -30
```

**Solutions**:

```bash
# Force rollout completion
kubectl rollout history deployment/instantdeploy-backend -n instantdeploy
kubectl rollout undo deployment/instantdeploy-backend -n instantdeploy

# Or restart deployment
kubectl rollout restart deployment/instantdeploy-backend -n instantdeploy

# Increase timeout for readiness probe
kubectl patch deployment instantdeploy-backend -n instantdeploy --type merge -p '{"spec":{"progressDeadlineSeconds":600}}'
```

### 7. WebSocket Connection Fails

**Symptoms**: WebSocket "connection refused" or "failed to upgrade protocol"

```bash
# Test WebSocket connection
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  wget -O - http://instantdeploy-backend:8080/ws 2>&1 | head -20

# Check backend logs for WebSocket errors
kubectl logs -n instantdeploy deployment/instantdeploy-backend | grep -i "websocket\|upgrade"
```

**Solutions**:

```bash
# Verify Upgrade header in nginx config
kubectl get configmap -n instantdeploy instantdeploy-frontend-config

# Check ingress proxy settings
kubectl get ingress -n instantdeploy -o yaml | grep -A 5 "websocket\|upgrade"

# Restart backend and frontend
kubectl rollout restart deployment/instantdeploy-backend -n instantdeploy
kubectl rollout restart deployment/instantdeploy-frontend -n instantdeploy
```

### 8. Monitoring Data Missing

**Symptoms**: Prometheus shows "no data", Grafana dashboards blank

```bash
# Check Prometheus scrape targets
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Visit http://localhost:9090/targets

# Check for scrape errors
kubectl logs -n monitoring deployment/prometheus | grep "error\|scrape"

# Check Promtail logs
kubectl logs -n monitoring ds/promtail | head -50
```

**Solutions**:

```bash
# Check service discovery
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Visit http://localhost:9090/service-discovery

# Verify metrics endpoint
kubectl exec -it <pod> -n instantdeploy -- \
  curl http://localhost:8080/metrics | head -20

# Restart Prometheus to reconnect
kubectl rollout restart deployment/prometheus -n monitoring
```

## Performance Issues

### Slow Requests

```bash
# Check latency
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Query: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Check backend resource usage
kubectl top pods -n instantdeploy
kubectl describe pod <backend-pod> -n instantdeploy | grep -A 5 "cpu\|memory"

# Examine slow query logs
kubectl logs -n instantdeploy deployment/instantdeploy-backend | grep "duration\|latency"
```

**Solutions**:

```bash
# Enable query profiling (Go)
kubectl port-forward -n instantdeploy svc/instantdeploy-backend 6060:6060
go tool pprof http://localhost:6060/debug/pprof/profile

# Add database indexes
kubectl exec -it <postgres-pod> -n instantdeploy -- \
  psql -U postgres -d instantdeploy -c "CREATE INDEX idx_deployment_status ON deployments(status);"

# Scale up backend
kubectl scale deployment instantdeploy-backend --replicas=5 -n instantdeploy
```

### High CPU Usage

```bash
# Monitor CPU
watch 'kubectl top pods -n instantdeploy'

# Check goroutines (Go apps)
kubectl port-forward -n instantdeploy svc/instantdeploy-backend 8080:8080
curl http://localhost:8080/debug/pprof/allocs?debug=1 | less
```

**Solutions**:

```bash
# Profile CPU
go tool pprof http://localhost:8080/debug/pprof/cpu?seconds=30

# Check for tight loops or goroutine leaks in logs
kubectl logs -n instantdeploy deployment/instantdeploy-backend | grep -i "goroutine\|lock"

# Reduce scrape intervals
kubectl edit cm prometheus-config -n monitoring
# Change: scrape_interval: 30s (from 15s)

kubectl rollout restart deployment/prometheus -n monitoring
```

## Network Issues

### DNS Resolution Fails

```bash
# Test CoreDNS
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup kubernetes.default

# Check CoreDNS pod
kubectl get pods -n kube-system -l k8s-app=kube-dns
```

### Network Connectivity Issues

```bash
# Test pod-to-pod connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  wget -O - http://instantdeploy-backend:8080/health

# Check network policies
kubectl get networkpolicies -n instantdeploy
kubectl describe networkpolicies -n instantdeploy
```

**Solutions**:

```bash
# If network policy blocks traffic, temporarily disable
kubectl delete networkpolicies --all -n instantdeploy

# Or update policy
kubectl patch networkpolicies instantdeploy-network-policy -n instantdeploy \
  -p '{"spec":{"podSelector": null}}' 2>/dev/null || true
```

## Data Issues

### Database Corruption

```bash
# Check database integrity
kubectl exec <postgres-pod> -n instantdeploy -- \
  pg_dump -d instantdeploy --verbose 2>&1 | grep -i "error"

# Check logs
kubectl logs -n instantdeploy deployment/instantdeploy-postgres | grep -i "error"
```

**Solutions**:

```bash
# Vacuum and reindex (maintenance)
kubectl exec -it <postgres-pod> -n instantdeploy -- \
  psql -U postgres -d instantdeploy -c "VACUUM ANALYZE; REINDEX DATABASE instantdeploy;"

# Restore from backup if severe
# See OPERATIONS.md for backup/restore procedures
```

### Data Loss Prevention

```bash
# Enable automated backups
kubectl create cronjob instantdeploy-backup --image=postgres:16-alpine --schedule="0 2 * * *" -- \
  sh -c 'pg_dump -h postgres -U postgres instantdeploy > /backups/backup-$(date +%Y%m%d-%H%M%S).sql'

# Verify backups
kubectl logs cronjob/instantdeploy-backup -n instantdeploy
```

## Getting Help

1. **Check logs**: All three components
2. **Describe resources**: Pods, deployments, services
3. **Check events**: Recent Kubernetes events
4. **Review metrics**: Memory, CPU, disk usage
5. **Test connectivity**: Service discovery, networking

### Debug Bundle

```bash
# Create comprehensive debug bundle
mkdir debug-bundle
kubectl cluster-info dump --output-directory=debug-bundle/cluster-info
kubectl describe all -n instantdeploy > debug-bundle/resources.txt
kubectl logs -n instantdeploy $(kubectl get pods -n instantdeploy -o name) --all-containers=true > debug-bundle/logs.txt

# Share with support
tar czf debug-bundle.tar.gz debug-bundle/
```

See [PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md) for deployment overview.
