# InstantDeploy Production Deployment Guide

Complete guide for deploying InstantDeploy to production.

## Table of Contents

1. **[Architecture](#architecture)**
2. **[Prerequisites](#prerequisites)**
3. **[Quick Start](#quick-start)**
4. **[Deployment Steps](#deployment-steps)**
5. **[Verification](#verification)**
6. **[Monitoring](#monitoring)**
7. **[Troubleshooting](#troubleshooting)**
8. **[Operations](#operations)**

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Internet                         │
│        (CloudFlare, AWS Route 53, etc.)             │
└─────────────────────────────┬───────────────────────┘
                              │
                    ┌─────────▼────────┐
                    │  Load Balancer   │
                    │  (TLS/HTTPS)     │
                    └─────────┬────────┘
                              │
        ┌─────────────────────┴──────────────────────┐
        │                                            │
   ┌────▼────┐                          ┌──────────▼────┐
   │Nginx/   │                          │ Nginx/        │
   │Ingress  │                          │ Ingress       │
   │(frontend)                          │ (backend)     │
   └────┬────┘                          └──────────┬────┘
        │                                          │
  ┌─────▼──────────────┐                 ┌────────▼──────┐
  │Frontend Pods       │                 │Backend Pod    │
  │(React + Node.js)   │                 │(Go Runtime)   │
  │(2-5 replicas)      │                 │(2-10 replicas)│
  └────┬──────────────────────────────────────┬───────────┘
       │‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾│
       │                                │
       ├─────────────────────┬──────────┤
       │                     │          │
  ┌────▼────┐        ┌──────▼────┐ ┌──▼────┐
  │PostgreSQL        │Redis      │ │DinD   │
  │(Persistent)      │(Cache)    │ │(Build)│
  └─────────┘        └───────────┘ └───────┘
```

## Prerequisites

### System Requirements

- **Kubernetes Cluster**: v1.24+ with 4+ cores, 8GB RAM minimum
- **Docker Registry**: Public or private (Docker Hub, ECR, GCR, etc.)
- **Persistence**: StorageClass configured for PostgreSQL
- **DNS**: Ability to create records for your domain
- **TLS Certificates**: Automatic via cert-manager

### Tools Required

```bash
# Install required tools
# Kubernetes
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# Helm (for optional monitoring setup)
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Kustomize (kubectl built-in v1.24+)
kubectl kustomize --version

# Docker CLI
docker --version
```

### Accounts Required

- **GitHub**: API token for repository cloning
- **Docker Registry**: Account for image storage
- **Domain Registrar**: Domain for your cluster
- **Email**: For Let's Encrypt SSL certificates

## Quick Start

### 1. Prepare Configuration

```bash
# Clone repository
git clone https://github.com/yourusername/instantdeploy.git
cd instantdeploy

# Set environment variables
export CLUSTER_CONTEXT="production"  # Your k8s cluster
export DOMAIN="instantdeploy.example.com"
export REGISTRY="docker.io/your-org"
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"

# Verify cluster access
kubectl cluster-info
kubectl get nodes
```

### 2. Deploy Kubernetes Resources

```bash
# Create namespace and secrets
cd scripts
chmod +x deploy-k8s-prod.sh

./deploy-k8s-prod.sh \
  --domain $DOMAIN \
  --registry $REGISTRY
```

### 3. Deploy Monitoring Stack

```bash
chmod +x deploy-monitoring.sh
./deploy-monitoring.sh
```

### 4. Deploy Frontend

```bash
chmod +x build-frontend-prod.sh

./build-frontend-prod.sh \
  --api-url "https://api.$DOMAIN" \
  --ws-url "wss://api.$DOMAIN"
```

### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -n instantdeploy

# Check services
kubectl get svc -n instantdeploy

# Test health
kubectl port-forward -n instantdeploy svc/instantdeploy-backend 8080:8080
curl http://localhost:8080/api/v1/health
```

## Deployment Steps

### Step 1: Prepare Infrastructure

```bash
# Create cluster
# AWS EKS
eksctl create cluster --name instantdeploy-prod --region us-east-1 --nodegroup-name standard --node-type t3.medium --nodes 3

# Or GKE
gcloud container clusters create instantdeploy-prod --zone us-central1-a --num-nodes 3 --machine-type n1-standard-2

# Or AKS
az aks create --resource-group myResourceGroup --name instantdeploy-prod --node-count 3 --vm-set-type VirtualMachineScaleSets
```

### Step 2: Configure kubectl

```bash
# Get kubeconfig
# AWS
aws eks update-kubeconfig --name instantdeploy-prod --region us-east-1

# GKE
gcloud container clusters get-credentials instantdeploy-prod --zone us-central1-a

# AKS
az aks get-credentials --resource-group myResourceGroup --name instantdeploy-prod
```

### Step 3: Install cert-manager

```bash
# Install cert-manager for SSL management
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create Let's Encrypt issuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@instantdeploy.example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: nginx
EOF
```

### Step 4: Install Ingress Nginx

```bash
# Install nginx ingress controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml

# Wait for load balancer IP
kubectl get svc -n ingress-nginx nginx-ingress-ingress-nginx
```

### Step 5: Deploy Application

```bash
# Build and push images
./build-frontend-prod.sh --push

# Deploy
./deploy-k8s-prod.sh

# Verify
kubectl get pods -n instantdeploy -w  # Wait for "Running" status
```

## Verification

### 1. Check Deployment Status

```bash
# All pods running
kubectl get pods -n instantdeploy
kubectl get pods -n monitoring

# Check pod readiness
kubectl get deployment -n instantdeploy -o wide
```

### 2. Test API Endpoints

```bash
# Get backend service IP/port
kubectl get svc instantdeploy-backend -n instantdeploy

# Test health
curl http://<backend-ip>:8080/api/v1/health

# Test API
curl http://<backend-ip>:8080/api/v1/repositories?query=github.com
```

### 3. Test Frontend

```bash
# Get frontend service IP/port
kubectl get svc instantdeploy-frontend -n instantdeploy

# Test in browser
# https://<your-domain>
```

### 4. Check Monitoring

```bash
# Port-forward monitoring tools
kubectl port-forward -n monitoring svc/prometheus 9090:9090
kubectl port-forward -n monitoring svc/grafana 3000:3000
kubectl port-forward -n monitoring svc/loki 3100:3100

# Access dashboards
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000
# Loki: http://localhost:3100
```

## Monitoring

See [MONITORING_SETUP.md](./MONITORING_SETUP.md) for complete monitoring details.

### Key Metrics

```sql
-- Request rate
rate(http_request_total[5m])

-- Error rate
rate(http_request_total{status=~"5.."}[5m])

-- Latency p95
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

-- Pod restarts
rate(kube_pod_container_status_restarts_total[1h])
```

### Alerts

Critical alerts trigger for:
- Backend service down (2+ minutes)
- High error rate (>5%)
- High latency (p95 > 1 second)
- Pod crash loops

## Troubleshooting

### Pod Stuck in Pending

```bash
kubectl describe pod <pod-name> -n instantdeploy
# Check: insufficient resources, image pull errors, affinity issues
```

### Pod CrashLoopBackOff

```bash
kubectl logs <pod-name> -n instantdeploy
kubectl logs <pod-name> -n instantdeploy --previous

# Check configuration, environment variables, dependencies
```

### Service Unreachable

```bash
# Check service exists
kubectl get svc -n instantdeploy

# Check endpoint
kubectl get endpoints -n instantdeploy

# Test from another pod
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://instantdeploy-backend:8080/api/v1/health
```

### Database Connection Failed

```bash
# Check postgres pod
kubectl get pods -n instantdeploy -l app=instantdeploy-postgres

# Test connection
kubectl exec -it <postgres-pod> -n instantdeploy -- \
  psql -U postgres -d instantdeploy -c "SELECT 1"
```

See full [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) guide.

## Operations

### Scaling

```bash
# Manual scaling
kubectl scale deployment instantdeploy-backend --replicas=5 -n instantdeploy

# Check autoscaler
kubectl get hpa -n instantdeploy
```

### Updates

```bash
# Update image
kubectl set image deployment/instantdeploy-backend \
  backend=docker.io/your-org/instantdeploy-backend:v2.0 \
  -n instantdeploy

# Wait for rollout
kubectl rollout status deployment/instantdeploy-backend -n instantdeploy

# Rollback if needed
kubectl rollout undo deployment/instantdeploy-backend -n instantdeploy
```

### Backup

```bash
# Backup database
kubectl exec <postgres-pod> -n instantdeploy -- \
  pg_dump -U postgres instantdeploy > backup.sql

# Backup entire namespace
kubectl get all -n instantdeploy -o yaml > namespace-backup.yaml
```

### Logs

```bash
# All logs for deployment
kubectl logs -f deployment/instantdeploy-backend -n instantdeploy

# Logs with timestamps
kubectl logs deployment/instantdeploy-backend -n instantdeploy --timestamps=true

# Previous logs (if pod crashed)
kubectl logs deployment/instantdeploy-backend -n instantdeploy --previous
```

## Security

- All data encrypted in transit (TLS 1.2+)
- Database password stored in Kubernetes secret
- Network policies restrict pod communication
- RBAC restricts service account permissions
- Regular security updates applied

## Cost Optimization

- **Autoscaling**: Pods scale from 2-10 based on load
- **Resource limits**: Prevents runaway costs
- **Pod disruption budgets**: High availability with lower cost
- **Reserved instances**: For predictable baseline load

## Support

- **Issues**: GitHub Issues
- **Documentation**: https://github.com/yourusername/instantdeploy/wiki
- **Community**: Slack/Discord channel

---

**Last Updated**: 2026-03-18  
**Version**: 1.0  
**Status**: Production Ready ✓
