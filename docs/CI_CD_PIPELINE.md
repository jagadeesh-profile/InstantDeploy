# CI/CD Pipeline Documentation

## Overview

The InstantDeploy CI/CD system provides a fully automated, production-ready pipeline with build, test, staging, and production deployment workflows.

```
┌──────────────┐
│   Code Push  │
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────┐
│ GitHub Actions: Build & Test         │
│ - Backend: Go linting, tests, build  │
│ - Frontend: ESLint, jest, build      │
│ - Security: Gosec, npm audit         │
│ - Docker: Build & push images        │
└──────┬───────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ Code Quality Gate                    │
│ - Line coverage: 80% (backend)       │
│ - Line coverage: 70% (frontend)      │
│ - No security vulnerabilities        │
│ - SonarQube analysis (optional)       │
└──────┬───────────────────────────────┘
       │
       ▼ (if PRs approved)
┌──────────────────────────────────────┐
│ Deploy to Staging                    │
│ - Kubernetes rolling update          │
│ - Run smoke tests                    │
│ - Performance baseline check         │
│ - Manual QA window (8 hours)         │
└──────┬───────────────────────────────┘
       │
       ▼ (if approved + tag)
┌──────────────────────────────────────┐
│ Production Deployment                │
│ - Pre-deployment checks              │
│ - Canary deployment (10% traffic)    │
│ - Full production rollout            │
│ - Post-deployment verification       │
│ - Automatic rollback on failure      │
└──────┬───────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────┐
│ Production Monitoring                │
│ - Health checks (continuous)         │
│ - Performance metrics (Prometheus)   │
│ - Error tracking (Sentry)            │
│ - Log aggregation (Loki)             │
└──────────────────────────────────────┘
```

---

## Workflows

### 1. Backend Build Pipeline (`backend-build.yml`)

**Trigger**: Push to `main`, `develop`, `staging` (backend/ files changed), or PR

**Stages**:

1. **Lint & Format Check**
   - golangci-lint for static analysis
   - gofmt for code formatting
   - Fails if code is not properly formatted

2. **Unit & Integration Tests**
   - PostgreSQL and Redis services
   - Unit tests with coverage (target: 80%)
   - Integration tests with test database
   - Coverage report to Codecov

3. **Security Scan**
   - Gosec: Go security issues
   - Nancy: Dependency vulnerabilities
   - Fails if critical issues found

4. **Build Docker Image**
   - Multi-stage build for optimization
   - Tag with commit SHA and branch name
   - Push to GHCR (GitHub Container Registry)
   - Cache layers for faster rebuilds

5. **Image Vulnerability Scan**
   - Trivy: Container image scanning
   - Checks for CVEs in base image and dependencies
   - SARIF report uploaded to GitHub Security

**Notifications**: Slack on failure

---

### 2. Frontend Build Pipeline (`frontend-build.yml`)

**Trigger**: Push to `main`, `develop`, `staging` (frontend/ files changed), or PR

**Stages**:

1. **Lint & Type Check**
   - ESLint: Code style and best practices
   - TypeScript: Type checking
   - Fails on type errors

2. **Unit Tests**
   - Jest test suite
   - Code coverage report (target: 70%)
   - Upload to Codecov

3. **Build & Optimize**
   - Vite build process
   - Check bundle size (warn if >1MB)
   - Upload build artifacts

4. **Docker Image Build**
   - Nginx-based serving
   - Multi-stage build
   - Environment-aware configuration
   - Push to GHCR

5. **Security Scan**
   - npm audit for dependencies
   - Snyk for vulnerability scanning
   - Flags high-severity issues

**Notifications**: Slack on failure

---

### 3. Staging Deployment (`deploy-staging.yml`)

**Trigger**: Push to `develop` or `staging` branches, or manual dispatch

**Features**:

- Deploy latest images to staging Kubernetes cluster
- Automated rollout status checks (5min timeout)
- Smoke tests to verify deployment
- Integration tests
- Performance baseline check

**Pre-requisites**:
- Kube config available in `KUBE_CONFIG_STAGING` secret
- Staging cluster accessible

**Manual Dispatch Options**:
- Specify custom backend/frontend image versions
- Override deployment configuration

---

### 4. Production Deployment (`deploy-production.yml`)

**Trigger**: Push to `main` branch with semantic version tag (v*.*.*)

**Flow**:

1. **Pre-Deployment Checks**
   - Verify image existence in registry
   - Scan images for vulnerabilities
   - Validate Kubernetes manifests
   - Health check staging environment

2. **Canary Deployment** (optional)
   - Deploy to 10% of traffic (1 replica)
   - Monitor for 5 minutes
   - Automatic rollback on high errors
   - Manual approval gate for full rollout

3. **Production Deployment**
   - Create backup of current state
   - Update backend and frontend deployments
   - Wait for rollout (10min timeout)
   - Smoke tests and verification
   - Automatic rollback on failure

4. **Post-Deployment**
   - Create GitHub release
   - Tag deployment in container registry
   - Send success notification

**Manual Dispatch Options**:
- Specify exact image versions
- Enable/disable canary deployment

**Safety Features**:
- Canary deployment validates changes
- Automatic rollback on health check failure
- Manual production environment approval
- Deployment audit trail

---

## Code Quality Gate (`code-quality-gate.yml`)

**Trigger**: All pull requests to main/develop/staging

**Requirements**:

| Component | Requirement | Tool |
|-----------|-------------|------|
| **Backend Coverage** | ≥80% | Go test + coverage |
| **Frontend Coverage** | ≥70% | Jest + coverage |
| **Backend Linting** | No errors | golangci-lint |
| **Frontend Linting** | No errors | ESLint |
| **Type Safety** | No errors | TypeScript, Go vet |
| **Security Scan** | No critical CVEs | Gosec, Snyk |
| **Dependencies** | No high-severity vulns | go mod verify, npm audit |

**Failure Modes**:
- Coverage threshold not met → PR blocks
- Security issues detected → PR blocks
- Linting errors → PR blocks

**Auto-comment**: Quality check result added as PR comment

---

## Rollback Procedure

### Automatic Rollback

Triggered automatically when:
- Health check fails post-deployment
- Critical errors detected (>5 in canary)
- Manual rollback initiated

### Manual Rollback

```bash
# Rollback to previous revision
./scripts/rollback-production.sh

# Or specific revision
kubectl rollout undo deployment/instantdeploy-backend -n instantdeploy --to-revision=3
kubectl rollout undo deployment/instantdeploy-frontend -n instantdeploy --to-revision=3

# Monitor rollout
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy

# Get rollout history
kubectl rollout history deployment/instantdeploy-backend -n instantdeploy
```

---

## Configuration

### GitHub Secrets

Required secrets in GitHub repository:

```yaml
# Kubernetes
KUBE_CONFIG_STAGING: base64-encoded kubeconfig for staging
KUBE_CONFIG_PRODUCTION: base64-encoded kubeconfig for production

# Container Registry
GITHUB_TOKEN: (auto-provided)

# Slack
SLACK_WEBHOOK: https://hooks.slack.com/services/xxx

# Security Scanning
SNYK_TOKEN: Snyk API token
SONAR_TOKEN: SonarQube API token

# Optional Monitoring
SENTRY_DSN: Sentry error tracking
PAGERDUTY_KEY: PagerDuty incident management
```

### Environment Variables

**Build**:
```bash
VITE_API_URL=http://localhost:8080          # Frontend API endpoint
VITE_WS_URL=ws://localhost:8080             # WebSocket endpoint
GO_ENV=production                            # Go environment
```

**Deployment**:
```bash
NAMESPACE=instantdeploy                      # Kubernetes namespace
TIMEOUT=600                                  # Deployment timeout (seconds)
PERFORMANCE_THRESHOLD=95                     # Min p95 latency (ms)
ERROR_RATE_THRESHOLD=0.1                     # Max error rate (%)
```

---

## Monitoring CI/CD

### GitHub Actions Dashboard

Access at: `https://github.com/{owner}/{repo}/actions`

Shows:
- All workflow runs
- Build/test/deploy status
- Artifact downloads
- Duration and logs

### Slack Notifications

Notifications for:
- ✅ Build success
- ❌ Build failure
- 🚀 Deployment initiated
- ✓ Deployment completed
- ⚠️ Manual approval needed

---

## Best Practices

### 1. Semantic Versioning

Tag production releases with semantic versions:
```bash
git tag -a v1.2.3 -m "Release version 1.2.3"
git push origin v1.2.3
```

Triggers production deployment automatically.

### 2. Pull Request Process

1. **Branch Naming**: `feature/description`, `fix/description`, `docs/description`
2. **Code Review**: Minimum 2 approvals
3. **Quality Gate**: All checks must pass
4. **Testing**: All tests must pass (unit, integration, e2e)
5. **Deployment**: Merge to develop for staging, to main for production

### 3. Commit Messages

Format: `type(scope): description`

```bash
feat(auth): add JWT validation
fix(backend): handle null pointer in deployment handler
docs(readme): update installation instructions
refactor(frontend): consolidate API calls
test(backend): increase test coverage to 85%
```

### 4. Deployment Windows

**Staging**: Anytime (continuous deployment)
**Production**: 
- Main branch: During business hours
- Hotfixes: Anytime (via manual dispatch)

### 5. Monitoring Post-Deployment

After deployment, monitor:
- **Metrics**: CPU, memory, latency (Prometheus)
- **Logs**: Errors, warnings (Loki/Grafana)
- **Errors**: Application exceptions (Sentry)
- **Health**: Endpoint status (continuous checks)

---

## Troubleshooting

### Build Failure

**Problem**: Backend test failure

**Solution**:
```bash
# Run locally to reproduce
cd backend
go test -v ./...

# Check for race conditions
go test -race ./...

# Format code
go fmt ./...
```

### Docker Push Failure

**Problem**: Image push fails to registry

**Solution**:
```bash
# Check authentication
docker login ghcr.io

# Verify image built
docker image ls | grep backend

# Check registry limits
# GitHub Container Registry has 1GB limit per image
```

### Deployment Timeout

**Problem**: Rollout status timeout

**Solution**:
```bash
# Check pod status
kubectl get pods -n instantdeploy

# View pod logs for errors
kubectl logs -l app=instantdeploy-backend -n instantdeploy

# Increase timeout in workflow
# Edit deploy-production.yml timeout value
```

### Automatic Rollback Triggered

**Problem**: Deployment rolled back automatically

**Solution**:
```bash
# Check what triggered rollback
kubectl rollout history deployment/instantdeploy-backend -n instantdeploy

# View current revision
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy

# Get rollback reason from logs
kubectl logs deployment/instantdeploy-backend -n instantdeploy --tail=50

# Manual rollback if needed
./scripts/rollback-production.sh
```

---

## Advanced Configuration

### Custom Build Arguments

Edit `.github/workflows/backend-build.yml`:

```yaml
build-args: |
  VERSION=${{ github.sha }}
  BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  ENVIRONMENT=production
```

### Conditional Deployments

Deploy only on specific conditions:

```yaml
if: |
  github.ref == 'refs/heads/main' &&
  github.event_name == 'push'
```

### Parallel Jobs

Workflows run stages in parallel when possible:
- Backend build and Frontend build run simultaneously
- Tests run before Docker build
- Security scans run in parallel with tests

---

## Continuous Improvement

### Performance Optimization

1. **Cache Docker layers**: Already configured
2. **Parallel tests**: Run multiple test suites
3. **Conditional builds**: Skip backend build if only frontend changes
4. **Artifact caching**: Cache npm packages between runs

### Cost Reduction

1. **Self-hosted runners**: Use for long-running tests
2. **Scheduled workflows**: Run heavy tests off-peak
3. **Artifact retention**: Delete old artifacts automatically
4. **Efficient caching**: Reduce download times

### Reliability

1. **Retry logic**: Automatically retry failed jobs
2. **Timeout settings**: Reasonable defaults (5-10 minutes)
3. **Backup procedures**: Regular backup tests
4. **Monitoring**: Alert on workflow failures

---

## Release Checklist

Before production release:

- [ ] Code review completed (2+ approvals)
- [ ] All tests passing
- [ ] Security scan clean
- [ ] Performance benchmarks met
- [ ] Documentation updated
- [ ] Changelog updated
- [ ] Version number determined (semantic versioning)
- [ ] Tag created and pushed
- [ ] Deployment approved
- [ ] Post-deployment monitoring enabled
- [ ] Release notes published

---

## Support

For CI/CD issues:
1. Check GitHub Actions logs: `https://github.com/{owner}/{repo}/actions`
2. Review workflow files: `.github/workflows/*.yml`
3. Check secret configuration: Repository Settings → Secrets
4. Contact DevOps team for cluster access issues
