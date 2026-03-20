#!/bin/bash
# CI/CD Setup Script
# Configures GitHub Actions workflows and local development hooks

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_section() { echo -e "\n${BLUE}===== $1 =====${NC}\n"; }

# Check prerequisites
check_prerequisites() {
    log_section "Checking Prerequisites"
    
    if ! command -v git &>/dev/null; then
        log_error "git not found"
        exit 1
    fi
    
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_error "Not in a git repository"
        exit 1
    fi
    
    log_info "✓ Git repository found"
}

# Install git hooks
install_git_hooks() {
    log_section "Installing Git Hooks"
    
    if [ -x "$REPO_ROOT/scripts/install-git-hooks.sh" ]; then
        bash "$REPO_ROOT/scripts/install-git-hooks.sh"
        log_info "✓ Git hooks installed"
    else
        log_warn "Git hooks script not executable"
    fi
}

# Configure GitHub secrets
configure_github_secrets() {
    log_section "GitHub Secrets Configuration"
    
    cat <<EOF
GitHub Secrets must be configured manually:

1. Go to: https://github.com/$(git config --get remote.origin.url | sed 's/.*://;s/.git$//')/settings/secrets/actions

2. Add the following secrets:

   Required:
   - KUBE_CONFIG_STAGING: (base64 kubeconfig for staging)
   - KUBE_CONFIG_PRODUCTION: (base64 kubeconfig for production)
   - SLACK_WEBHOOK: (Slack incoming webhook URL)

   Optional:
   - SNYK_TOKEN: (Snyk security token)
   - SONAR_TOKEN: (SonarQube token)
   - CODECOV_TOKEN: (Codecov token)

See docs/GITHUB_SECRETS.md for detailed instructions.
EOF
    
    read -p "Have you configured the secrets? (y/n) " -n 1 -r
    echo
    if [ "$REPLY" != "y" ]; then
        log_warn "Secrets not configured - workflows may fail"
    fi
}

# Verify GitHub workflows
verify_workflows() {
    log_section "Verifying GitHub Workflows"
    
    local workflows=(
        ".github/workflows/backend-build.yml"
        ".github/workflows/frontend-build.yml"
        ".github/workflows/deploy-staging.yml"
        ".github/workflows/deploy-production.yml"
        ".github/workflows/code-quality-gate.yml"
    )
    
    for workflow in "${workflows[@]}"; do
        if [ -f "$REPO_ROOT/$workflow" ]; then
            log_info "✓ Found: $workflow"
        else
            log_error "Missing: $workflow"
            exit 1
        fi
    done
}

# Test local build
test_local_build() {
    log_section "Testing Local Builds"
    
    read -p "Test backend build? (y/n) " -n 1 -r
    echo
    if [ "$REPLY" = "y" ]; then
        cd "$REPO_ROOT/backend"
        go build ./cmd/server
        log_info "✓ Backend build successful"
    fi
    
    read -p "Test frontend build? (y/n) " -n 1 -r
    echo
    if [ "$REPLY" = "y" ]; then
        cd "$REPO_ROOT/frontend"
        npm install
        npm run build
        log_info "✓ Frontend build successful"
    fi
}

# Show deployment commands
show_deployment_commands() {
    log_section "Deployment Commands"
    
    cat <<'EOF'
Manual Deployment (via GitHub Actions):

1. Deploy to Staging:
   - Push to 'develop' or 'staging' branch
   - Or: Actions → Deploy to Staging → Run workflow

2. Deploy to Production:
   - Create semantic version tag: git tag v1.2.3
   - Push tag: git push origin v1.2.3
   - Or: Actions → Deploy to Production → Run workflow → Specify versions

3. Rollback Production:
   ./scripts/rollback-production.sh

4. View Deployment History:
   kubectl rollout history deployment/instantdeploy-backend -n instantdeploy
   kubectl rollout history deployment/instantdeploy-frontend -n instantdeploy
EOF
}

# Main setup flow
main() {
    cat <<'EOF'
╔════════════════════════════════════════╗
║ InstantDeploy CI/CD Setup              ║
╚════════════════════════════════════════╝
EOF
    
    check_prerequisites
    install_git_hooks
    verify_workflows
    configure_github_secrets
    test_local_build
    show_deployment_commands
    
    log_section "Setup Complete"
    log_info "CI/CD pipeline is ready for use!"
    log_info ""
    log_info "Next steps:"
    log_info "1. Review docs/CI_CD_PIPELINE.md"
    log_info "2. Configure GitHub secrets"
    log_info "3. Create semantic version tag to trigger production deployment"
    log_info ""
}

main "$@"
