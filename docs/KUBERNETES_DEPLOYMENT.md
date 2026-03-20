# InstantDeploy Kubernetes Production Deployment Guide

## Prerequisites

- Kubernetes cluster (v1.24+) with RBAC enabled
- `kubectl` configured with access to your cluster
- `kustomize` (v3.0+) or kubectl built-in kustomize
- `docker` CLI for building images
- Helm (optional, for Prometheus/Loki setup)

## Quick Start

### 1. Prepare Your Environment

```bash
cd scripts

# Export your cluster settings
export CLUSTER_CONTEXT="your-cluster-name"
export DOMAIN="instantdeploy.example.com"
export DOCKER_REGISTRY="docker.io"
export DOCKER_USERNAME="your-docker-username"
export DOCKER_PASSWORD="your-docker-password"
export REGISTRY_URL="docker.io/your-org"  # For private registry
export GITHUB_TOKEN="your-github-token"    # For GitHub API access

# Linux/macOS
chmod +x deploy-k8s-prod.sh
./deploy-k8s-prod.sh
```

### 2. Verify Deployment

```bash
# Check deployment status
kubectl get pods -n instantdeploy
kubectl get deployments -n instantdeploy
kubectl get services -n instantdeploy

# View logs
kubectl logs -n instantdeploy deployment/instantdeploy-backend -f
kubectl logs -n instantdeploy deployment/instantdeploy-frontend -f
kubectl logs -n instantdeploy deployment/instantdeploy-postgres -f
```

### 3. Access Your Application

```bash
# Port-forward backend
kubectl port-forward -n instantdeploy svc/instantdeploy-backend 8080:8080

# Port-forward frontend
kubectl port-forward -n instantdeploy svc/instantdeploy-frontend 80:80

# API Health Check
curl http://localhost:8080/api/v1/health
```

## Architecture Overview

```
┌─────────────────────────────────────────┐
│         Internet/Load Balancer          │
│  (with TLS termination via cert-manager)│
└──────────────────┬──────────────────────┘
                   │
┌──────────────────┴──────────────────────┐
│         Ingress Controller               │
│    (nginx or cloud provider native)      │
└──────────────────┬──────────────────────┘
                   │
        ┌──────────┴──────────┐
        │                     │
    ┌───▼────┐        ┌───────▼─┐
    │Frontend │        │Backend  │
    │ (2-5    │        │ (2-10   │
    │replicas)│        │replicas)│
    └────┬────┘        └────┬────┘
         │                  │
         │           ┌──────┴──────┐
         │           │             │
         │      ┌────▼────┐   ┌────▼────┐
         │      │PostgreSQL   │Redis    │
         │      │(1 replica) │(1 rep)  │
         │      └───────────┘ └─────────┘
         │
    ┌────▼────────────────┐
    │  Docker-in-Docker   │
    │ (backend sidecar)   │
    │ for building images │
    └─────────────────────┘
```

## Deployment Components

### Networking
- **Ingress**: Routes external traffic to frontend and backend
- **NetworkPolicy**: Restricts pod-to-pod communication
- **Service**: Internal load balancing within cluster

### Scaling
- **HPA (Horizontal Pod Autoscaler)**: Scales deployments based on CPU/memory
  - Backend: 2-10 replicas (scales at 70% CPU, 80% memory)
  - Frontend: 2-5 replicas (scales at 60% CPU, 70% memory)
- **PDB (Pod Disruption Budget)**: Ensures minimum availability during disruptions

### Storage
- **PersistentVolumeClaim**: Provides persistent storage for PostgreSQL
- **EmptyDir**: Temporary storage for builds and Docker-in-Docker

### Security
- **RBAC**: Role-Based Access Control for service accounts
- **NetworkPolicy**: Network segmentation
- **TLS/SSL**: Automatic certificate management with cert-manager

## Configuration Management

### Secrets

Create or update secrets before deployment:

```bash
# PostgreSQL credentials
kubectl create secret generic instantdeploy-postgres-secret \
  -n instantdeploy \
  --from-literal=POSTGRES_USER=postgres \
  --from-literal=POSTGRES_PASSWORD=<strong-password> \
  --from-literal=POSTGRES_DB=instantdeploy

# Backend secrets
kubectl create secret generic instantdeploy-backend-secret \
  -n instantdeploy \
  --from-literal=JWT_SECRET=$(openssl rand -base64 32) \
  --from-literal=GITHUB_TOKEN=<your-github-token>

# Docker registry (if using private registry)
kubectl create secret docker-registry instantdeploy-registry \
  -n instantdeploy \
  --docker-server=<registry-url> \
  --docker-username=<username> \
  --docker-password=<password>
```

### ConfigMaps

Update configmap for backend environment:

```bash
kubectl set env configmap/instantdeploy-backend-config \
  -n instantdeploy \
  JWT_EXPIRY_MINUTES=120 \
  ENV=production
```

## Certificate Management

The deployment uses cert-manager for automatic TLS certificates:

1. **Prerequisites**:
   - cert-manager must be installed: `helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace`
   - A ClusterIssuer for Let's Encrypt must exist

2. **Create ClusterIssuer**:

```yaml
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
```

## Monitoring & Logging

See Task #2 for comprehensive monitoring setup.

## Troubleshooting

### Pod stuck in Pending

```bash
kubectl describe pod <pod-name> -n instantdeploy
# Check for resource constraints, affinity requirements, or image pull errors
```

### Pod stuck in CrashLoopBackOff

```bash
kubectl logs <pod-name> -n instantdeploy
# Check for startup errors, configuration issues
kubectl logs <pod-name> -n instantdeploy --previous  # Previous attempt logs
```

### Database connection issues

```bash
# Check PostgreSQL service discovery
kubectl get service postgres -n instantdeploy
kubectl run -it --rm debug --image=alpine:latest --restart=Never -- sh
# nslookup postgres.instantdeploy.svc.cluster.local
```

### Docker build failures in backend

```bash
# Check DinD (Docker-in-Docker) sidecar
kubectl logs <backend-pod> -c dind -n instantdeploy
# Ensure sufficient storage: kubectl get pv
# Check DinD image availability
```

## Performance Tuning

### Resource Limits

```yaml
# For compute-intensive deployments, increase:
resources:
  requests:
    cpu: 500m        # Changed from 100m
    memory: 512Mi     # Changed from 128Mi
  limits:
    cpu: "2"         # Changed from 1
    memory: 1Gi      # Changed from 512Mi
```

### Autoscaling Thresholds

```bash
kubectl autoscale deployment instantdeploy-backend \
  -n instantdeploy \
  --min=3 \
  --max=20 \
  --cpu-percent=50
```

## Cleanup

```bash
# Delete entire deployment
kubectl delete namespace instantdeploy

# Or selective deletion
kubectl delete deployment instantdeploy-backend -n instantdeploy
kubectl delete service instantdeploy-backend -n instantdeploy
```

## Advanced: Custom Image Registry

If using a private registry:

1. Push images:
```bash
docker tag instantdeploy-backend:latest my-registry.com/instantdeploy-backend:v1
docker push my-registry.com/instantdeploy-backend:v1
```

2. Update kustomization:
```bash
cd infrastructure/k8s
kustomize edit set image instantdeploy-backend=my-registry.com/instantdeploy-backend:v1
```

3. Create image pull secret before deployment

## Next Steps

1. Configure monitoring & logging (Task #2)
2. Set up CI/CD pipelines (Task #7)
3. Run performance tests under load (Task #6)
4. Document runbooks for operations (Task #5)
