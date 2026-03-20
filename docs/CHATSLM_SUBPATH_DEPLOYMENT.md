# Chatslm Subpath Deployment

This project can be published as a second app inside an existing chatslm.com website.

Target path:
- /instantdeploy/

## What This Setup Does

- Builds frontend with router base path set to /instantdeploy/
- Uses same-domain API paths:
  - /instantdeploy-api/v1
  - /instantdeploy-ws
- Publishes only to gh-pages:/instantdeploy so existing root app stays unchanged
- Adds /instantdeploy/404.html for SPA refresh fallback

## Workflow Added

GitHub Actions workflow:
- .github/workflows/deploy-chatslm-subpath.yml

Triggers:
- Push to main when frontend files change
- Manual run from Actions tab

## Required Platform Routing

Your edge/proxy must route:
- /instantdeploy-api/* to InstantDeploy backend
- /instantdeploy-ws to InstantDeploy backend websocket

If routing is not configured, the UI will load but deployments/auth requests will fail.

## Repository Configuration

Case A: same repository hosts chatslm.com
- No extra variables needed
- Workflow deploys with GITHUB_TOKEN to gh-pages branch

Case B: different repository hosts chatslm.com
- Add repository variable:
  - CHATSLM_PAGES_REPO = owner/repo
- Add repository secret:
  - PAGES_DEPLOY_TOKEN = PAT with repo write access

## Result URL

- https://chatslm.com/instantdeploy/

## Add Navigation Link In Existing Site

Add a menu item/button in your current chatslm app:
- Label: Instant Deploy
- URL: /instantdeploy/