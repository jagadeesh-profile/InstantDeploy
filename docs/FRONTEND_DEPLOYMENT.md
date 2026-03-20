# Frontend Production Deployment Guide

## Overview

This guide covers building and deploying the React frontend to production.

## Architecture

```
┌─────────────────────────────┐
│      Load Balancer          │
│   (CloudFlare/AWS ALB)      │
└──────────────┬──────────────┘
               │
        ┌──────▼──────┐
        │   Ingress   │
        │  (nginx)    │
        └──────┬──────┘
               │
        ┌──────▼──────────────┐
        │ Frontend Pod Pool   │
        │  (2-5 replicas)     │
        │                     │
        │ ┌────────────────┐  │
        │ │ Nginx Server   │  │
        │ │ - Cache        │  │
        │ │ - Compression  │  │
        │ │ - Security     │  │
        │ └────────────────┘  │
        └──────┬──────────────┘
               │
        ┌──────▼──────┐
        │ Backend API │
        │ (internal)  │
        └─────────────┘
```

## Quick Start

### 1. Build Locally

```bash
cd frontend

# Install dependencies
npm install

# Build for production
VITE_API_URL=https://api.instantdeploy.example.com \
VITE_WS_URL=wss://api.instantdeploy.example.com \
npm run build

# Verify build
npm run preview
```

### 2. Build Docker Image

```bash
# Using the build script
cd scripts
chmod +x build-frontend-prod.sh
./build-frontend-prod.sh \
  --api-url "https://api.instantdeploy.example.com" \
  --registry "docker.io/your-org"

# Or manually
docker build -f frontend/Dockerfile.prod \
  --build-arg VITE_API_URL=https://api.instantdeploy.example.com \
  -t instantdeploy-frontend:latest .
```

### 3. Deploy to Kubernetes

```bash
# Update Kubernetes deployment
kubectl set image deployment/instantdeploy-frontend \
  frontend=instantdeploy-frontend:latest \
  -n instantdeploy

# Or apply full manifest
kubectl apply -f frontend-deployment.yaml
```

## Environment Configuration

### Build-time Variables

Set during Docker build:

```dockerfile
ARG VITE_API_URL=https://api.instantdeploy.example.com
ARG VITE_WS_URL=wss://api.instantdeploy.example.com
```

### Runtime Variables

Set in deployment/pod environment:

```yaml
env:
  - name: VITE_API_URL
    value: https://api.instantdeploy.example.com
  - name: VITE_WS_URL
    value: wss://api.instantdeploy.example.com
```

## Performance Optimizations

### Code Splitting

The frontend automatically code-splits routes using React.lazy:

```javascript
const Dashboard = lazy(() => import('@/pages/DashboardPage'))
const Login = lazy(() => import('@/pages/LoginPage'))
```

### Asset Optimization

Webpack automatically:
- Minifies CSS and JavaScript
- Optimizes images
- Generates source maps
- Creates chunks for better caching

### Browser Caching

Nginx configuration caches:
- **Static assets** (JS, CSS, images): 1 year
- **HTML files**: 1 day (must-revalidate)
- **API responses**: 5 minutes (configurable)

### Compression

- gzip compression enabled for all text files
- Compression ratio: ~70%
- Typical bundle: 150KB → 50KB (gzipped)

## Security

### Content Security Policy

```
default-src 'self'
script-src 'self' 'unsafe-inline' (for Vite HMR)
style-src 'self' 'unsafe-inline'
img-src 'self' data: https:
connect-src 'self' ws: wss:
```

### Security Headers

- **HSTS**: 1 year max-age
- **X-Frame-Options**: SAMEORIGIN
- **X-Content-Type-Options**: nosniff
- **CSP**: Strict policy

### Authentication

- JWT stored in localStorage
- Automatic refresh on API 401
- Secure WebSocket connections (wss://)

## Frontend Deployment Checklist

- [ ] Environment variables configured correctly
- [ ] API endpoints point to production backend
- [ ] WebSocket URLs use wss:// (secure)
- [ ] Build passes TypeScript checks
- [ ] No console errors during build
- [ ] Bundle size acceptable (<500KB gzipped)
- [ ] All routes tested locally
- [ ] Security headers verified
- [ ] CSP policy updated if needed
- [ ] Docker image built and tested
- [ ] Image pushed to registry
- [ ] Kubernetes manifest updated
- [ ] Deployment rolled out successfully
- [ ] Health checks passing

## Monitoring Frontend

### Performance Metrics

Monitor from browser console:

```javascript
// Web Vitals
window.addEventListener('web-vital', (event) => {
  console.log(event.detail)
})
```

### Application Errors

Frontend error tracking (e.g., Sentry):

```javascript
import * as Sentry from "@sentry/react"

Sentry.init({
  dsn: "your-sentry-dsn",
  environment: "production",
  tracesSampleRate: 0.1,
})
```

### API Performance

Monitor API response times in Network tab or:

```javascript
const perf = performance.getEntriesByType("navigation")[0]
console.log(`Backend latency: ${perf.responseStart - perf.requestStart}ms`)
```

## Troubleshooting

### Build Fails

```bash
# Clear cache and rebuild
rm -rf node_modules dist
npm ci
npm run build
```

### Runtime Errors

```bash
# Check browser console for errors
# Enable debug logging
VITE_DEBUG=true npm run build

# Check API connectivity
curl https://api.instantdeploy.example.com/health
```

### WebSocket Connection Fails

```bash
# Verify WebSocket URL matches backend
# Check backend logs for connection issues
# Ensure wss:// for HTTPS

# Test connection
wscat -c wss://api.instantdeploy.example.com/ws
```

### Nginx Not Proxying API

```bash
# Check nginx logs in container
kubectl logs -n instantdeploy deployment/instantdeploy-frontend

# Test backend connectivity from pod
kubectl exec -it -n instantdeploy deployment/instantdeploy-frontend -c frontend -- \
  wget -O - http://instantdeploy-backend:8080/api/v1/health
```

## Rollback

If new deployment has issues:

```bash
# View rollout history
kubectl rollout history deployment/instantdeploy-frontend -n instantdeploy

# Rollback to previous version
kubectl rollout undo deployment/instantdeploy-frontend -n instantdeploy
```

## Next Steps

1. **Set up CDN** (CloudFlare, AWS CloudFront)
2. **Enable caching** for static assets globally
3. **Monitor frontend metrics** with Grafana
4. **Set up error tracking** with Sentry
5. **Configure domain** and SSL certificate
