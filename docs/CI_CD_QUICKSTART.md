# CI/CD Quick Start

## One-Time Setup

### 1. Install Git Hooks

```bash
./scripts/install-git-hooks.sh
```

This installs pre-commit hooks that:
- Format Go code (`go fmt`)
- Lint Go code (`golangci-lint`)
- Lint frontend code (`npm run lint`)
- Scan for secrets
- Validate commit message format

### 2. Configure GitHub Secrets

See [GITHUB_SECRETS.md](GITHUB_SECRETS.md) for detailed instructions.

Required secrets:
- `KUBE_CONFIG_STAGING`
- `KUBE_CONFIG_PRODUCTION`
- `SLACK_WEBHOOK`

### 3. Enable Workflows

GitHub Actions workflows are automatically enabled. No additional setup needed.

---

## Daily Workflow

### 1. Create Feature Branch

```bash
git checkout -b feature/my-feature
```

### 2. Make Changes

```bash
# Backend changes
cd backend
go test ./...
go fmt ./...

# Frontend changes
cd frontend
npm run lint
npm run test
```

### 3. Commit Changes

```bash
git add .
git commit -m "feat(backend): implement new feature"

# Message format: type(scope): description
# Types: feat, fix, refactor, test, docs, chore
```

Pre-commit hooks run automatically:
- ✅ Code formatting
- ✅ Linting
- ✅ Secret scanning
- ✅ Commit message validation

### 4. Push and Create PR

```bash
git push origin feature/my-feature
```

Then create pull request on GitHub.

### 5. PR Checks Run Automatically

GitHub Actions runs:
- ✅ Build & test (backend)
- ✅ Build & test (frontend)
- ✅ Code quality gate
- ✅ Security scanning

**All must pass before merge**

### 6. Code Review

- Minimum 2 approvals required
- Address feedback
- Push updates (tests run again)

### 7. Merge to Develop

```bash
# After approval, merge PR
# This automatically deploys to staging
```

Staging deployment runs:
- ✅ Build Docker images
- ✅ Deploy to staging cluster
- ✅ Smoke tests
- ✅ Performance validation

### 8. Test in Staging

Manual QA window: 8 hours

Access at: `http://staging.instantdeploy.example.com`

### 9. Release to Production

When ready to release:

```bash
# Create version tag
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3

# Or manually trigger:
# GitHub → Actions → Deploy to Production → Run workflow
```

Production deployment runs:
- ✅ Pre-deployment checks
- ✅ Canary deployment (10% traffic)
- ✅ Monitor for 5 minutes
- ✅ Full production rollout
- ✅ Post-deployment verification

---

## Triggering Deployments Manually

### Deploy Staging

**Via GitHub UI**:
1. Go to: Actions → Deploy to Staging
2. Click: "Run workflow"
3. Specify image tags (optional)

**Via CLI**:
```bash
# Push to 'develop' branch (automatic)
git checkout develop
git pull origin develop
git push origin develop
```

### Deploy Production

**Via GitHub UI**:
1. Go to: Actions → Deploy to Production
2. Click: "Run workflow"
3. Specify backend image tag (required)
4. Specify frontend image tag (required)
5. Check "Enable canary" (recommended)

**Via CLI**:
```bash
# Create semantic version tag (automatic)
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

---

## Monitoring Deployment

### GitHub Actions

View in: Actions → [Workflow Name]

Shows:
- ✅ Build status
- ✅ Test results
- ✅ Deployment logs
- ✅ Artifacts
- ✅ Duration

### Kubernetes

```bash
# Watch deployment status
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy

# View pods
kubectl get pods -n instantdeploy

# View logs
kubectl logs -l app=instantdeploy-backend -n instantdeploy --tail=50
```

### Metrics & Logs

**Prometheus** (metrics):
```
http://grafana.example.com:3000
Dashboards → InstantDeploy Overview
```

**Grafana** (logs & metrics):
```
Query examples:
- Requests/sec: rate(http_request_total[1m])
- Error rate: rate(http_errors_total[1m])
- Latency p95: histogram_quantile(0.95, http_request_duration)
```

---

## Rollback Procedures

### Automatic Rollback

Triggered when:
- Health check fails
- Errors exceed threshold (>5 in canary)
- Manual abort initiated

### Manual Rollback

```bash
# Rollback to previous revision
./scripts/rollback-production.sh

# Or specific revision
kubectl rollout undo deployment/instantdeploy-backend -n instantdeploy --to-revision=3

# Monitor rollback
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy
```

---

## Troubleshooting

### Build Failure

1. **Check logs**: Actions → [Workflow] → Logs
2. **Fix locally**:
   ```bash
   cd backend && go test ./...
   cd frontend && npm run lint
   ```
3. **Commit fix and push**

### Deployment Failure

1. **Check kubeconfig**:
   ```bash
   kubectl cluster-info
   kubectl get nodes
   ```
2. **Check pods**:
   ```bash
   kubectl get pods -n instantdeploy
   kubectl describe pod <name> -n instantdeploy
   ```
3. **View logs**:
   ```bash
   kubectl logs deployment/instantdeploy-backend -n instantdeploy
   ```

### Stalled Deployment

1. Check GitHub Actions UI for status
2. If stuck >10 minutes, cancel workflow
3. Check pod status:
   ```bash
   kubectl get deployment -n instantdeploy
   kubectl describe deployment instantdeploy-backend -n instantdeploy
   ```

---

## Performance Tips

1. **Cache Dependencies**
   - Go: Automatically cached
   - npm: Automatically cached

2. **Parallel Jobs**
   - Backend and frontend build in parallel
   - Tests run simultaneously

3. **Early Feedback**
   - Linting runs first (fast)
   - Tests run second
   - Docker build last

4. **Reuse Artifacts**
   - Docker layers cached
   - Build outputs shared between jobs

---

## Security Best Practices

1. **Commit Hooks**
   ```bash
   # Pre-commit hook scans for secrets
   git hook run pre-commit
   ```

2. **Secret Management**
   - ✅ Never commit secrets
   - ✅ Use GitHub Secrets
   - ✅ Rotate quarterly

3. **Code Review**
   - ✅ Always use PRs (never direct push)
   - ✅ Require approvals
   - ✅ Review CI logs

4. **Signed Commits** (optional)
   ```bash
   # Configure GPG signing
   git config user.signingkey YOUR_GPG_KEY
   git commit -S -m "message"
   ```

---

## References

- [CI/CD Pipeline Documentation](CI_CD_PIPELINE.md)
- [GitHub Secrets Setup](GITHUB_SECRETS.md)
- [Kubernetes Deployment](KUBERNETES_DEPLOYMENT.md)
- [Production Deployment](PRODUCTION_DEPLOYMENT.md)

---

## Getting Help

1. **Check logs**: GitHub Actions or kubectl
2. **Review docs**: See reference links above
3. **Common issues**: See TROUBLESHOOTING.md
4. **Contact**: DevOps team or maintainer
