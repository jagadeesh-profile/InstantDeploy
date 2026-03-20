#!/bin/bash
# Pre-commit Hook Installation
# Installs git hooks for code quality checks

set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
HOOKS_DIR="$REPO_ROOT/.git/hooks"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Create hooks directory
mkdir -p "$HOOKS_DIR"

# Create pre-commit hook
cat > "$HOOKS_DIR/pre-commit" <<'EOF'
#!/bin/bash
# Pre-commit hook for code quality checks

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

STAGED_FILES=$(git diff --cached --name-only)
REPO_ROOT=$(git rev-parse --show-toplevel)

# Check for backend changes
if echo "$STAGED_FILES" | grep -q "^backend/"; then
    log_info "Checking backend code..."
    cd "$REPO_ROOT/backend"
    
    # Format check
    if ! go fmt ./... >/dev/null 2>&1; then
        log_error "Go fmt check failed. Run 'go fmt ./...'"
        exit 1
    fi
    
    # Lint
    if command -v golangci-lint &>/dev/null; then
        if ! golangci-lint run --deadline 60s >/dev/null 2>&1; then
            log_warn "golangci-lint found issues (non-blocking)"
        fi
    fi
    
    log_info "✓ Backend checks passed"
fi

# Check for frontend changes
if echo "$STAGED_FILES" | grep -q "^frontend/"; then
    log_info "Checking frontend code..."
    cd "$REPO_ROOT/frontend"
    
    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        log_error "node_modules not found. Run 'npm install'"
        exit 1
    fi
    
    # Lint
    if npm run lint --if-present >/dev/null 2>&1; then
        log_info "✓ Frontend linting passed"
    else
        log_error "Frontend linting failed"
        exit 1
    fi
fi

# Check for secrets
log_info "Scanning for secrets..."
if git diff --cached | grep -E 'password|token|secret|api.?key' -i; then
    log_error "Potential secrets found in staged files. Please review and remove."
    exit 1
fi

log_info "✓ All pre-commit checks passed"
exit 0
EOF

chmod +x "$HOOKS_DIR/pre-commit"
log_info "✓ Pre-commit hook installed"

# Create commit-msg hook
cat > "$HOOKS_DIR/commit-msg" <<'EOF'
#!/bin/bash
# Commit message validation

COMMIT_MSG_FILE=$1
COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")

# Check commit message format
# Format: type(scope): description
# Example: feat(backend): add user authentication

if ! echo "$COMMIT_MSG" | grep -q "^[a-z]*([a-z]*): "; then
    echo "Error: Commit message must follow format: type(scope): description"
    echo "Example: feat(backend): add user authentication"
    exit 1
fi

# Check commit message length
FIRST_LINE=$(echo "$COMMIT_MSG" | head -1)
if [ ${#FIRST_LINE} -gt 72 ]; then
    echo "Error: First line of commit message must not exceed 72 characters"
    exit 1
fi

exit 0
EOF

chmod +x "$HOOKS_DIR/commit-msg"
log_info "✓ Commit message hook installed"

log_info ""
log_info "All git hooks installed successfully!"
log_info "Hooks location: $HOOKS_DIR"
