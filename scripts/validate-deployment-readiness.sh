#!/bin/bash
# Deployment Readiness Validation Script
# Validates that all components are ready for production deployment

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
FAIL=0
WARN=0

pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASS++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAIL++))
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARN++))
}

section() {
    echo -e "\n${BLUE}═════════════════════════════════════${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}═════════════════════════════════════${NC}\n"
}

# Check prerequisites
section "1. Prerequisites"

command -v git &>/dev/null && pass "git installed" || fail "git not installed"
command -v kubectl &>/dev/null && pass "kubectl installed" || fail "kubectl not installed"
command -v docker &>/dev/null && pass "docker installed" || warn "docker not installed (needed for local builds)"

# Check repository
section "2. Repository Configuration"

git rev-parse --git-dir > /dev/null 2>&1 && pass "Git repository valid" || fail "Not in git repository"

if git remote -v | grep -q origin; then
    pass "Git remote 'origin' configured"
else
    fail "Git remote 'origin' not configured"
fi

# Check local files
section "3. Required Files"

required_files=(
    ".github/workflows/backend-build.yml"
    ".github/workflows/frontend-build.yml"
    ".github/workflows/deploy-staging.yml"
    ".github/workflows/deploy-production.yml"
    "infrastructure/k8s/backend/deployment.yaml"
    "infrastructure/k8s/frontend/deployment.yaml"
    "backend/Dockerfile"
    "frontend/Dockerfile.prod"
    "scripts/rollback-production.sh"
    "docs/PRODUCTION_DEPLOYMENT.md"
    "docs/CI_CD_PIPELINE.md"
)

for file in "${required_files[@]}"; do
    [ -f "$file" ] && pass "Found: $file" || fail "Missing: $file"
done

# Check secrets
section "4. GitHub Secrets Configuration"

echo "Checking GitHub repository settings for required secrets..."
echo "(Note: This requires GitHub CLI or manual verification)"

echo ""
echo "Required secrets:"
echo "  - KUBE_CONFIG_PRODUCTION (base64 kubeconfig)"
echo "  - SLACK_WEBHOOK (Slack incoming webhook)"
echo "  - SNYK_TOKEN (optional, for security scanning)"

warn "Secrets must be configured manually in GitHub repository settings"

# Check Kubernetes connectivity
section "5. Kubernetes Cluster"

if kubectl cluster-info &>/dev/null; then
    pass "Kubernetes cluster accessible"
    
    NODES=$(kubectl get nodes -o jsonpath='{.items | length}')
    [ "$NODES" -ge 3 ] && pass "Cluster has $NODES nodes (3+ recommended)" || warn "Cluster has only $NODES nodes (3+ recommended)"
    
    # Check namespaces
    if kubectl get namespace instantdeploy &>/dev/null; then
        pass "Namespace 'instantdeploy' exists"
    else
        warn "Namespace 'instantdeploy' not found (will be created on first deploy)"
    fi
else
    fail "Cannot connect to Kubernetes cluster"
fi

# Check image registry
section "6. Container Registry"

if kubectl get secret regcred -n instantdeploy &>/dev/null 2>&1; then
    pass "Container registry credentials configured"
else
    warn "Container registry credentials may not be configured (check kubernetes secrets)"
fi

# Check monitoring
section "7. Monitoring Stack"

if kubectl get deployment prometheus -n instantdeploy &>/dev/null 2>&1; then
    pass "Prometheus deployed"
else
    warn "Prometheus not deployed (will be installed with infrastructure)"
fi

# Check documentation
section "8. Documentation"

docs=(
    "docs/PRODUCTION_DEPLOYMENT.md"
    "docs/CI_CD_PIPELINE.md"
    "docs/CI_CD_QUICKSTART.md"
    "docs/KUBERNETES_DEPLOYMENT.md"
    "docs/MONITORING_SETUP.md"
    "docs/FRONTEND_DEPLOYMENT.md"
    "docs/IOS_DEPLOYMENT.md"
    "docs/TROUBLESHOOTING.md"
)

for doc in "${docs[@]}"; do
    [ -f "$doc" ] && pass "Found: $doc" || fail "Missing: $doc"
done

# Check build configurations
section "9. Build Configurations"

[ -f "backend/go.mod" ] && pass "Backend module defined" || fail "Backend go.mod missing"
[ -f "frontend/package.json" ] && pass "Frontend package defined" || fail "Frontend package.json missing"
[ -f "backend/Dockerfile" ] && pass "Backend Dockerfile exists" || fail "Backend Dockerfile missing"
[ -f "frontend/Dockerfile.prod" ] && pass "Frontend production Dockerfile exists" || fail "Frontend Dockerfile.prod missing"

# Check git hooks
section "10. Git Hooks"

if [ -x ".git/hooks/pre-commit" ]; then
    pass "Pre-commit hook installed"
else
    warn "Pre-commit hook not installed (run: ./scripts/install-git-hooks.sh)"
fi

# Performance test configuration
section "11. Performance Testing"

[ -f "tests/performance/load-test.js" ] && pass "Load test script exists" || warn "Load test script missing"
[ -f "infrastructure/k8s/testing/load-test-jobs.yaml" ] && pass "K8s load test job configured" || warn "K8s load test job missing"

# Summary
section "Readiness Assessment"

TOTAL=$((PASS + FAIL + WARN))

echo "Results: ${GREEN}$PASS passed${NC}, ${RED}$FAIL failed${NC}, ${YELLOW}$WARN warnings${NC} (Total: $TOTAL)"

if [ "$FAIL" -eq 0 ]; then
    echo -e "\n${GREEN}✓ All critical checks passed!${NC}"
    if [ "$WARN" -gt 0 ]; then
        echo -e "${YELLOW}Address $WARN warnings before production deployment${NC}"
    fi
    exit 0
else
    echo -e "\n${RED}✗ $FAIL critical checks failed. Please address before deployment.${NC}"
    exit 1
fi
