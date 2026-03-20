# GitHub Secrets Configuration Guide

## Overview

This guide explains how to configure secrets for the CI/CD pipeline.

## Required Secrets

### 1. Kubernetes Configuration

**Secret Name**: `KUBE_CONFIG_STAGING` and `KUBE_CONFIG_PRODUCTION`

**Purpose**: kubeconfig file for cluster access

**Steps**:
```bash
# 1. Get kubeconfig from cluster
kubectl config view --raw

# 2. Base64 encode
cat ~/.kube/config | base64

# 3. Add to GitHub Secrets:
# - Go to: Settings → Secrets and variables → Actions
# - Click "New repository secret"
# - Name: KUBE_CONFIG_STAGING
# - Value: (paste base64 content)
# - Repeat for KUBE_CONFIG_PRODUCTION
```

### 2. Slack Webhook

**Secret Name**: `SLACK_WEBHOOK`

**Purpose**: Send deployment notifications to Slack

**Steps**:
```bash
# 1. Create Slack app
# - Go to: https://api.slack.com/apps
# - Click "Create New App" → "From scratch"
# - Name: InstantDeploy
# - Workspace: [Select your workspace]

# 2. Enable Incoming Webhooks
# - Left sidebar: "Incoming Webhooks"
# - Click "Add New Webhook to Workspace"
# - Select channel: #deployments
# - Authorize

# 3. Copy webhook URL
# - Format: https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX

# 4. Add to GitHub Secrets
# - Settings → Secrets → New repository secret
# - Name: SLACK_WEBHOOK
# - Value: (paste webhook URL)
```

**Test Webhook**:
```bash
curl -X POST -H 'Content-type: application/json' \
  --data '{"text":"Test From GitHub CI/CD"}' \
  YOUR_WEBHOOK_URL
```

### 3. Container Registry Authentication

**Note**: GitHub Actions automatically provides `GITHUB_TOKEN` for GitHub Container Registry (GHCR).

**For Docker Hub** (optional):

```bash
# 1. Create Docker Hub token
# - Go to: https://hub.docker.com/settings/security
# - Click "New Access Token"
# - Name: github-actions
# - Permissions: Read & Write

# 2. Add to GitHub Secrets
# - Name: DOCKER_USERNAME
# - Value: (your Docker Hub username)

# - Name: DOCKER_TOKEN
# - Value: (paste token)
```

### 4. Security Scanning

**Snyk Token**:
```bash
# 1. Get token from Snyk
# - Go to: https://app.snyk.io/account/settings
# - API Token section
# - Copy token

# 2. Add to GitHub Secrets
# - Name: SNYK_TOKEN
# - Value: (paste token)
```

**SonarQube Token** (optional):
```bash
# 1. Generate token in SonarQube
# - Go to: Administration → Security → Users
# - Generate token

# 2. Add to GitHub Secrets
# - Name: SONAR_TOKEN
# - Value: (paste token)
```

### 5. Code Coverage

**Codecov Token**:
```bash
# 1. Sign up at: https://codecov.io
# - Connect: GitHub
# - Select repository

# 2. Get repository token
# - Settings → Repository Token

# 3. Add to GitHub Secrets
# - Name: CODECOV_TOKEN
# - Value: (paste token)
```

## Optional Secrets

### Error Tracking (Sentry)

```bash
# 1. Create Sentry project
# - Go to: https://sentry.io
# - Create new project
# - Platform: Go / React

# 2. Get DSN
# - Project Settings → Client Keys (DSN)

# 3. Add to GitHub Secrets
# - Name: SENTRY_DSN
# - Value: (paste DSN)
```

### Incident Management (PagerDuty)

```bash
# 1. Create PagerDuty integration key
# - Services → [Service] → Integrations → New Integration
# - Integration Type: Events API v2

# 2. Copy integration key

# 3. Add to GitHub Secrets
# - Name: PAGERDUTY_KEY
# - Value: (paste key)
```

## Environment Variables

Environment-specific configuration:

```yaml
Staging:
  VITE_API_URL: http://staging-api.instantdeploy.example.com
  VITE_WS_URL: ws://staging-api.instantdeploy.example.com
  LOG_LEVEL: debug

Production:
  VITE_API_URL: https://api.instantdeploy.example.com
  VITE_WS_URL: wss://api.instantdeploy.example.com
  LOG_LEVEL: error
```

## Verification

### List All Secrets

```bash
# Go to: Settings → Secrets and variables → Actions
# Shows all configured secrets (values hidden)
```

### Test Secret Access

Add test workflow:
```yaml
name: Test Secrets
on: workflow_dispatch

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Check secrets
        run: |
          [ -n "${{ secrets.KUBE_CONFIG_STAGING }}" ] && echo "✓ KUBE_CONFIG_STAGING set"
          [ -n "${{ secrets.SLACK_WEBHOOK }}" ] && echo "✓ SLACK_WEBHOOK set"
          [ -n "${{ secrets.SNYK_TOKEN }}" ] && echo "✓ SNYK_TOKEN set"
```

## Rotation Schedule

| Secret | Rotation Frequency |
|--------|-------------------|
| Kubeconfig | Every 90 days |
| Slack Webhook | When team changes |
| API Tokens | Every 6 months |
| Container Registry | When key compromised |
| SSH Keys | Every 12 months |

## Best Practices

1. **Use distinct secrets for each environment** (staging vs production)
2. **Rotate secrets regularly** (quarterly)
3. **Grant minimal permissions** (e.g., read-only for registry pull)
4. **Audit secret access** (check GitHub logs)
5. **Never commit secrets to repository**
6. **Use GitHub's secret scanning** to prevent leaks
7. **Enable required status checks** before merge
8. **Document who has access** to sensitive secrets

## Troubleshooting

### Secret Not Found

**Error**: `Error: Undefined secret`

**Solution**:
```bash
# Verify secret name spellingis exact (case-sensitive)
# Check: Settings → Secrets → Actions
# Re-create if needed
```

### Permission Denied

**Error**: `Error: Permission denied`

**Solution**:
```bash
# Check kubeconfig permissions
# Verify token expiration
# Verify RBAC roles
kubectl auth can-i create deployments --as=system:serviceaccount:instantdeploy:github-actions
```

### Webhook Not Sending

**Error**: Slack notification not received

**Solution**:
```bash
# Test webhook manually
curl -X POST -H 'Content-type: application/json' \
  -d '{"text":"Test"}' \
  ${{ secrets.SLACK_WEBHOOK }}

# Check workflow logs for errors
```

## Security Considerations

1. **Least Privilege**: Grant minimum permissions needed
2. **Expiration**: Set token expiration dates
3. **Monitoring**: Monitor secret usage and access
4. **Audit Trail**: Log all deployments triggered
5. **Incident Response**: Have revocation plan ready
6. **Compliance**: Track which secrets access what resources

## References

- GitHub Secrets: https://docs.github.com/en/actions/security-guides/encrypted-secrets
- Kubernetes RBAC: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
- Slack Webhooks: https://api.slack.com/messaging/webhooks
- Snyk: https://docs.snyk.io
- SonarQube: https://docs.sonarqube.org
