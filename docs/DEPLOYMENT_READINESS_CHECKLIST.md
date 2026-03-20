# Deployment Readiness Checklist

**Status**: Production Ready ✅

---

## Pre-Deployment Verification

### Phase 1: Code & Configuration (Hours -24 to -12)

**Development Team Verification**:

- [ ] All code committed and pushed to develop branch
- [ ] All tests passing locally (`go test ./...`, `npm run test`)
- [ ] Code lint passing (`go fmt ./...`, `npm run lint`)
- [ ] No uncommitted changes (`git status` clean)
- [ ] All branches up-to-date with main
- [ ] Feature branches merged via PRs with 2+ approvals

**Infrastructure Verification**:

- [ ] Run deployment readiness validation:
  ```bash
  ./scripts/validate-deployment-readiness.sh
  ```

- [ ] All required files present:
  ```bash
  ls .github/workflows/backend-build.yml
  ls .github/workflows/deploy-production.yml
  ls infrastructure/k8s/backend/deployment.yaml
  ```

- [ ] Kubernetes cluster accessible:
  ```bash
  kubectl cluster-info
  kubectl get nodes
  ```

**Documentation Verification**:

- [ ] All documentation current
- [ ] PRODUCTION_DEPLOYMENT.md reviewed
- [ ] Team has access to runbooks
- [ ] Troubleshooting guide reviewed

---

### Phase 2: Security & Compliance (Hours -12 to -6)

**Security Checks**:

- [ ] GitHub Secrets configured:
  - [ ] KUBE_CONFIG_PRODUCTION set
  - [ ] SLACK_WEBHOOK set
  - [ ] SNYK_TOKEN set (for scanning)
  - [ ] No test/dummy values

- [ ] Secret rotation recent (< 6 months)

- [ ] No secrets in repository:
  ```bash
  git log --all --full-history -S "password\|token\|secret" -- | grep -q ".*" && echo "Found secrets!" || echo "No secrets found"
  ```

**Compliance Checks**:

- [ ] Code of Conduct reviewed
- [ ] License compliance verified
- [ ] Data privacy requirements met
- [ ] Audit logging enabled

**Security Scan Results**:

- [ ] Backend security scan passed (Gosec)
- [ ] Frontend security scan passed (npm audit, Snyk)
- [ ] Container image scan passed (Trivy)
- [ ] No high-severity vulnerabilities

---

### Phase 3: Testing & Validation (Hours -6 to 0)

**Local Build Tests**:

- [ ] Backend builds locally:
  ```bash
  cd backend && go build ./cmd/server
  ```

- [ ] Frontend builds locally:
  ```bash
  cd frontend && npm run build
  ```

- [ ] Docker images build locally:
  ```bash
  docker build -t test-backend backend/
  docker build -t test-frontend frontend/ -f frontend/Dockerfile.prod
  ```

**Staging Environment Tests**:

- [ ] Staging deployment healthy
- [ ] All services responding to health checks
- [ ] Database connectivity verified
- [ ] Cache (Redis) working
- [ ] Monitoring stack operational

**Performance Baseline**:

- [ ] Load tests run successfully:
  ```bash
  ./scripts/run-performance-suite.sh
  ```

- [ ] Metrics meet targets:
  - [ ] P95 Latency < 500ms
  - [ ] Error rate < 0.1%
  - [ ] Throughput > 50 req/s

**Smoke Tests**:

- [ ] User authentication works
- [ ] Repository search works
- [ ] Deployment creation works
- [ ] WebSocket connections work
- [ ] Logs retrievable

---

### Phase 4: Deployment Preparation (Hours 0)

**Team Preparation**:

- [ ] On-call team notified
- [ ] Slack channel ready (#instantdeploy-deployments)
- [ ] Team has access to:
  - [ ] GitHub Actions dashboard
  - [ ] Grafana monitoring
  - [ ] Kubernetes cluster
  - [ ] Deployment documentation
  - [ ] Rollback procedures

**Communication Plan**:

- [ ] Status page ready
- [ ] Communication template prepared
- [ ] Notification recipients listed
- [ ] Incident response plan reviewed

**Backups & Recovery**:

- [ ] Database backup recent and verified
- [ ] Previous deployment revision accessible
- [ ] Rollback procedure tested on staging
- [ ] Recovery time objective (RTO) documented: < 30 minutes

**Final Verification**:

- [ ] No changes in production since last verification
- [ ] Kubeconfig valid and current
- [ ] All GitHub secrets accessible
- [ ] Git tag ready: `v1.0.0`

---

## Deployment Execution

### Step 1: Pre-Deployment Health Check (T-10 min)

```bash
# Verify cluster is healthy
kubectl get nodes
kubectl get pods -n instantdeploy

# Check current deployments
kubectl get deployment -n instantdeploy

# Verify monitoring is running
kubectl get pod -n instantdeploy -l app=prometheus
kubectl get pod -n instantdeploy -l app=grafana
```

### Step 2: Create and Push Release Tag (T-5 min)

```bash
# Create semantic version tag
git tag -a v1.0.0 -m "Initial production release"

# Push tag to trigger deployment
git push origin v1.0.0
```

### Step 3: Monitor Deployment (T+0 to T+30 min)

**Watch GitHub Actions**:
- Go to: Actions → deploy-production
- Monitor each stage:
  1. Pre-deployment checks (5 min)
  2. Canary deployment (10 min)
  3. Full rollout (10 min)
  4. Post-deployment tests (5 min)

**Watch Kubernetes**:
```bash
# Terminal 1: Watch deployment progress
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy
kubectl rollout status deployment/instantdeploy-frontend -n instantdeploy

# Terminal 2: Watch pods
kubectl get pods -n instantdeploy -w

# Terminal 3: Watch events
kubectl get events -n instantdeploy -w
```

**Watch Metrics**:
```bash
# Open Grafana dashboard
# Monitor: CPU, memory, request rate, error rate, latency
```

### Step 4: Verification (T+30 to T+60 min)

**Health Checks**:
```bash
# Backend health
kubectl exec -it deployment/instantdeploy-backend -n instantdeploy -- \
  curl -s http://localhost:8080/api/v1/health | jq .

# Frontend health
curl -s https://api.instantdeploy.example.com/api/v1/health | jq .
```

**Smoke Tests**:

- [ ] Can login
- [ ] Can search repositories
- [ ] Can create deployment
- [ ] Can view logs
- [ ] WebSocket updates work

**Performance Verification**:

- [ ] Latency: Check P95 < 500ms in Grafana
- [ ] Throughput: requests/sec > 50
- [ ] Error rate: < 0.1%
- [ ] No pod restarts

**Log Review**:

```bash
# Check for errors in logs
kubectl logs deployment/instantdeploy-backend -n instantdeploy --tail=100 | grep ERROR
kubectl logs deployment/instantdeploy-frontend -n instantdeploy --tail=100 | grep ERROR
```

### Step 5: Announcement (T+60 min)

- [ ] Send Slack announcement: "@channel Deployment successful!"
- [ ] Update status page
- [ ] Notify stakeholders
- [ ] Log deployment in runbook

---

## Post-Deployment Monitoring

### Hour 1

- [ ] Latency stable (no spikes)
- [ ] Error rate normal
- [ ] No pod crashes
- [ ] Database performance normal

### Hours 2-4

- [ ] Continued stability
- [ ] User reports normal
- [ ] No performance degradation

### Day 1

- [ ] 24-hour stability achieved
- [ ] No issues discovered
- [ ] Performance metrics match baseline

---

## Rollback Decision Tree

### If Any Check Fails

**Decision**: ROLLBACK

```bash
./scripts/rollback-production.sh
```

**Conditions triggering rollback**:

1. **Health checks fail**
   - Backend health check timeout
   - Frontend not responding
   - Database connectivity lost

2. **Performance degrades**
   - P95 latency > 1000ms
   - Error rate > 1%
   - Throughput < 20 req/s

3. **Critical errors detected**
   - >5 pod restarts in 5 minutes
   - Out of memory errors
   - Panic/crash in logs

4. **Infrastructure issues**
   - Persistent storage failure
   - Network connectivity lost
   - Kubernetes cluster issues

---

## Sign-Off

| Role | Name | Date | Time |
|------|------|------|------|
| **DevOps Lead** | _______ | _____ | _____ |
| **Backend Lead** | _______ | _____ | _____ |
| **Frontend Lead** | _______ | _____ | _____ |
| **QA Lead** | _______ | _____ | _____ |
| **Security Lead** | _______ | _____ | _____ |

---

## Deployment Record

**Release Version**: v1.0.0  
**Deployment Date**: ________  
**Start Time**: ________  
**Completion Time**: ________  
**Status**: ☐ Successful ☐ Rolled Back ☐ Failed

**Deployment Notes**:
```
_________________________________________________________________
_________________________________________________________________
_________________________________________________________________
```

**Issues Encountered**:
```
_________________________________________________________________
_________________________________________________________________
```

**Performance Baseline**:
- P95 Latency: _______ ms
- Error Rate: _______ %
- Throughput: _______ req/s

---

## References

- [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
- [CI/CD Pipeline Documentation](CI_CD_PIPELINE.md)
- [Kubernetes Operations Guide](KUBERNETES_DEPLOYMENT.md)
- [Troubleshooting Guide](TROUBLESHOOTING.md)
- [Performance Baseline](PERFORMANCE_BASELINE.md)
