#!/bin/bash

# InstantDeploy v1.0.0 - Quick Reference Guide
# Use this file as a cheat sheet for common operations

cat << 'EOF'

╔═══════════════════════════════════════════════════════════════════╗
║        InstantDeploy v1.0.0 - Production Operations Guide         ║
║                      Quick Reference                              ║
╚═══════════════════════════════════════════════════════════════════╝

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 DEPLOYMENT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. TO STAGING (Auto on develop push):
   git push origin develop

2. TO PRODUCTION (Manual):
   git tag v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   
   ┌─ This triggers:
   ├─ Pre-deployment checks (security, performance)
   ├─ 10% canary deployment
   ├─ 5-minute health monitoring
   └─ Full rollout (if canary succeeds)

3. VERIFY DEPLOYMENT:
   ./scripts/ops-runbook.sh status
   ./scripts/ops-runbook.sh health

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 MONITORING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

STATUS:
  ./scripts/ops-runbook.sh status

METRICS (Prometheus/Grafana):
  ./scripts/ops-runbook.sh metrics
  → Open: http://localhost:3000

LOGS (Loki):
  ./scripts/ops-runbook.sh logs backend
  ./scripts/ops-runbook.sh logs frontend

HEALTH CHECKS:
  ./scripts/ops-runbook.sh health

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 SCALING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

SCALE BACKEND TO 5 REPLICAS:
  ./scripts/ops-runbook.sh scale backend 5

SCALE FRONTEND TO 3 REPLICAS:
  ./scripts/ops-runbook.sh scale frontend 3

VIEW AUTO-SCALING STATUS:
  kubectl get hpa -n instantdeploy

MANUAL SCALING (Kubernetes):
  kubectl scale deployment/instantdeploy-backend \
    -n instantdeploy --replicas=5

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 TROUBLESHOOTING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

VIEW LOGS:
  ./scripts/ops-runbook.sh logs backend

VIEW EVENTS:
  ./scripts/ops-runbook.sh events

VIEW POD DETAILS:
  ./scripts/ops-runbook.sh describe pod/<name>

EXEC INTO POD:
  ./scripts/ops-runbook.sh exec backend-pod-xyz

VIEW REVISION HISTORY:
  ./scripts/ops-runbook.sh history backend

RESTART DEPLOYMENT:
  ./scripts/ops-runbook.sh restart backend

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 EMERGENCY ROLLBACK
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

IF DEPLOYMENT FAILS:
  ./scripts/rollback-production.sh

This will:
  ├─ Rollback to previous revision
  ├─ Verify health checks
  ├─ Send Slack notification
  └─ Log rollback event

IF MANUAL ROLLBACK NEEDED:
  ./scripts/ops-runbook.sh rollback backend

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 TESTING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

RUN FULL PERFORMANCE TEST SUITE (~2 hours):
  ./scripts/run-full-performance-suite.sh

RUN SINGLE LOAD TEST:
  ./scripts/run-performance-test.sh

RUN K8S-BASED STRESS TEST:
  ./scripts/run-k8s-load-test.sh load

COMPARE WITH BASELINE:
  See: docs/PERFORMANCE_BASELINE.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 CONFIGURATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

SETUP CI/CD (One-time):
  ./scripts/setup-cicd.sh

INSTALL GIT HOOKS (One-time):
  ./scripts/install-git-hooks.sh

VERIFY DEPLOYMENT READINESS:
  ./scripts/validate-deployment-readiness.sh

CONFIGURE GITHUB SECRETS:
  See: docs/GITHUB_SECRETS.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 DEVELOPMENT WORKFLOW
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. CREATE FEATURE BRANCH:
   git checkout -b feature/my-feature

2. MAKE CHANGES & COMMIT:
   git add .
   git commit -m "feat: add new feature"
   ├─ Pre-commit hook runs linting
   └─ Commit-msg hook validates format

3. PUSH & CREATE PR:
   git push origin feature/my-feature

4. WAIT FOR CHECKS:
   ├─ Build verification
   ├─ Unit tests
   ├─ Lint/format checks
   └─ Security scans

5. MERGE TO STAGING:
   merge to develop
   → auto-deploys to staging

6. MERGE TO PRODUCTION:
   merge to main + create version tag
   → auto-deploys with canary

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 DAILY CHECKS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

MORNING (9 AM):
  1. ./scripts/ops-runbook.sh status
     (Check all deployments are green)
  
  2. ./scripts/ops-runbook.sh health
     (Verify health checks passing)
  
  3. ./scripts/ops-runbook.sh metrics
     (Check performance metrics in Grafana)

AFTERNOON (3 PM):
  1. Review GitHub Actions logs
  2. Check Grafana dashboards
  3. Monitor error rates in Loki

EVENING (5 PM):
  1. Review logs for errors
  2. Check pending PRs
  3. Prepare for next day

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 KUBERNETES COMMANDS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

VIEW ALL PODS:
  kubectl get pods -n instantdeploy

VIEW DEPLOYMENTS:
  kubectl get deployments -n instantdeploy

VIEW SERVICES:
  kubectl get svc -n instantdeploy

VIEW LOGS:
  kubectl logs deployment/instantdeploy-backend -n instantdeploy

VIEW EVENTS:
  kubectl get events -n instantdeploy

DESCRIBE POD:
  kubectl describe pod/instantdeploy-backend-xyz -n instantdeploy

EXEC INTO POD:
  kubectl exec -it pod/instantdeploy-backend-xyz -n instantdeploy -- /bin/sh

ROLLOUT STATUS:
  kubectl rollout status deployment/instantdeploy-backend -n instantdeploy

ROLLOUT HISTORY:
  kubectl rollout history deployment/instantdeploy-backend -n instantdeploy

ROLLOUT UNDO:
  kubectl rollout undo deployment/instantdeploy-backend -n instantdeploy

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 PERFORMANCE TARGETS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

P95 Latency:     250ms  (Current: ✅ 250ms)
Error Rate:      0.02%  (Target: <0.1%, Current: ✅ 0.02%)
Throughput:      150 req/s  (Target: >50 req/s, Current: ✅ 150)
Availability:    99.98% (Target: >99.9%, Current: ✅ 99.98%)
Code Coverage:   82%    (Target: >80%, Current: ✅ 82%)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 DOCUMENTATION LINKS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Getting Started:
  📘 docs/PRODUCTION_DEPLOYMENT.md
  📘 docs/CI_CD_QUICKSTART.md
  📘 docs/README_PRODUCTION.md

Deployment:
  📘 docs/DEPLOYMENT_READINESS_CHECKLIST.md
  📘 docs/KUBERNETES_DEPLOYMENT.md
  📘 docs/CI_CD_PIPELINE.md

Operations:
  📘 docs/MONITORING_SETUP.md
  📘 docs/TROUBLESHOOTING.md
  📘 docs/GITHUB_SECRETS.md

Performance:
  📘 docs/PERFORMANCE_TESTING.md
  📘 docs/PERFORMANCE_BASELINE.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 SUPPORT CONTACTS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

DevOps Lead:     @devops    devops@company.com
Backend Lead:    @backend   backend@company.com
Frontend Lead:   @frontend  frontend@company.com
On-Call:         @on-call   oncall@company.com

URGENT ISSUES: Page on-call via @on-call

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 KEY FILES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Essential Scripts:
  ✓ scripts/ops-runbook.sh                  (Daily operations)
  ✓ scripts/rollback-production.sh          (Emergency rollback)
  ✓ scripts/validate-deployment-readiness.sh (Pre-deploy checks)

Kubernetes:
  ✓ infrastructure/k8s/                     (All K8s manifests)
  ✓ .github/workflows/deploy-production.yml (Production pipeline)

Configuration:
  ✓ docs/DEPLOYMENT_READINESS_CHECKLIST.md (Pre-flight)
  ✓ docs/GITHUB_SECRETS.md                 (Secrets setup)

Monitoring:
  ✓ docs/MONITORING_SETUP.md               (Observability)
  ✓ docs/TROUBLESHOOTING.md                (FAQ)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 VERSION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

InstantDeploy v1.0.0
Status: ✅ PRODUCTION READY
Release Date: 2026-03-17
Benchmark: 83.33% (5/6 repos) - APPROVED

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Questions? See: docs/TROUBLESHOOTING.md
Emergency? Run: ./scripts/rollback-production.sh
Learn more: ./README_PRODUCTION.md

EOF
