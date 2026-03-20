# Local Domain Runbook (Docker Desktop Kubernetes)

This runbook exposes InstantDeploy locally on a custom domain such as chatslm.com.

## Preconditions

- Docker Desktop is running
- Kubernetes is enabled in Docker Desktop
- InstantDeploy manifests are deployed:
  - `./deploy-k8s.ps1` on Windows
  - `./deploy-k8s.sh` on Linux/macOS

## 1. Install NGINX Ingress Controller (one-time)

```powershell
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml
kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=180s
```

## 2. Verify ingress resources

```powershell
kubectl get ingress -n instantdeploy
kubectl get svc -n instantdeploy
```

Expected ingress host: `chatslm.com`

## 3. Map domain to localhost

Edit hosts file:

- Windows: `C:\Windows\System32\drivers\etc\hosts`
- Linux/macOS: `/etc/hosts`

Add:

```text
127.0.0.1 chatslm.com
```

## 4. Access platform

- Frontend via domain: `http://chatslm.com`
- API via domain: `http://chatslm.com/api/v1/health`

## 5. Troubleshooting

### Ingress not reachable

```powershell
kubectl get pods -n ingress-nginx
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller --tail=100
```

### Backend unavailable

```powershell
kubectl get pods -n instantdeploy
kubectl logs -n instantdeploy deployment/instantdeploy-backend --tail=120
```

### Domain works but websocket fails

- Ensure ingress rule includes `/ws` path
- Check browser console for websocket URL and status
