param(
    [ValidateSet("local", "k8s", "all")]
    [string]$Target = "all",
    [switch]$SkipBuild,
    [switch]$SkipImageSync,
    [string]$Namespace = "instantdeploy",
    [string]$K8sApiURL = "http://localhost:30080"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Path $PSScriptRoot -Parent
$composeFile = Join-Path $root "infrastructure\docker-compose.yml"
$backendDir = Join-Path $root "backend"
$frontendDir = Join-Path $root "frontend"
$k8sRoot = Join-Path $root "infrastructure\k8s"

function Remove-StaleLocalContainers {
    $stale = @("instantdeploy-frontend", "instantdeploy-backend")
    foreach ($name in $stale) {
        $existing = docker ps -a --filter "name=^/$name$" --format "{{.Names}}"
        if ($existing -contains $name) {
            Write-Host "[InstantDeploy] Removing stale container '$name' to avoid compose conflicts..."
            docker rm -f $name | Out-Null
        }
    }
}

function Get-AvailablePort {
    param(
        [int]$PreferredPort = 8080,
        [int]$FallbackStart = 18080,
        [int]$FallbackEnd = 18120
    )

    $preferredInUse = Get-NetTCPConnection -LocalPort $PreferredPort -State Listen -ErrorAction SilentlyContinue
    if (-not $preferredInUse) {
        return $PreferredPort
    }

    for ($port = $FallbackStart; $port -le $FallbackEnd; $port++) {
        $inUse = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
        if (-not $inUse) {
            Write-Warning "[InstantDeploy] Port $PreferredPort is already in use. Using fallback local backend port $port."
            return $port
        }
    }

    throw "[InstantDeploy] No available fallback port found in range $FallbackStart-$FallbackEnd."
}

function Sync-ImageToK8sNodes {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Image
    )

    $nodeLines = kubectl get nodes -o name
    $nodes = @($nodeLines | ForEach-Object {
        $line = $_.Trim()
        if ($line.StartsWith("node/")) {
            $line.Substring(5)
        } elseif ($line) {
            $line
        }
    } | Where-Object { $_ })
    if (-not $nodes) {
        Write-Warning "[InstantDeploy] No Kubernetes nodes discovered, skipping image sync for '$Image'."
        return
    }

    foreach ($node in $nodes) {
        $trimmed = $node.Trim()
        if (-not $trimmed) {
            continue
        }

        $nodeContainer = docker ps --format "{{.Names}}" --filter "name=^/$trimmed$"
        if (-not ($nodeContainer -contains $trimmed)) {
            Write-Warning "[InstantDeploy] Node runtime container '$trimmed' not found in Docker; skipping image sync for this node."
            continue
        }

        Write-Host "[InstantDeploy] Loading image '$Image' into node runtime '$trimmed'..."
        docker save $Image | docker exec -i $trimmed ctr -n k8s.io images import -
    }
}

function Invoke-LocalDeploy {
    Write-Host "[InstantDeploy] Deploying local Docker stack..."
    Remove-StaleLocalContainers

    $localBackendPort = Get-AvailablePort
    $previousBackendPort = $env:BACKEND_HOST_PORT
    $env:BACKEND_HOST_PORT = "$localBackendPort"
    try {
        docker compose -f $composeFile up -d --build
    } finally {
        $env:BACKEND_HOST_PORT = $previousBackendPort
    }

    Write-Host "[InstantDeploy] Waiting for local health checks..."
    Start-Sleep -Seconds 4
    try {
        $backend = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:$localBackendPort/api/v1/health" -TimeoutSec 6
        Write-Host "[InstantDeploy] Local backend healthy: HTTP $($backend.StatusCode)"
    } catch {
        Write-Warning "[InstantDeploy] Local backend health probe failed: $($_.Exception.Message)"
    }

    try {
        $frontend = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:5173" -TimeoutSec 6
        Write-Host "[InstantDeploy] Local frontend healthy: HTTP $($frontend.StatusCode)"
    } catch {
        Write-Warning "[InstantDeploy] Local frontend health probe failed: $($_.Exception.Message)"
    }
}

function Invoke-K8sDeploy {
    Write-Host "[InstantDeploy] Deploying Kubernetes stack in namespace '$Namespace'..."

    if (-not $SkipBuild) {
        Write-Host "[InstantDeploy] Building backend image for Kubernetes..."
        docker build -t instantdeploy-backend:local $backendDir

        Write-Host "[InstantDeploy] Building frontend image for Kubernetes..."
        docker build --build-arg "VITE_API_URL=$K8sApiURL" -t instantdeploy-frontend:local $frontendDir
    } else {
        Write-Host "[InstantDeploy] Skipping image build (--SkipBuild)."
    }

    if ($SkipImageSync) {
        Write-Host "[InstantDeploy] Skipping image sync to Kubernetes nodes (--SkipImageSync)."
    } else {
        Sync-ImageToK8sNodes -Image "instantdeploy-backend:local"
        Sync-ImageToK8sNodes -Image "instantdeploy-frontend:local"
    }

    kubectl apply -k $k8sRoot

    kubectl -n $Namespace rollout status deployment/instantdeploy-postgres --timeout=180s
    kubectl -n $Namespace rollout status deployment/instantdeploy-redis --timeout=180s
    kubectl -n $Namespace rollout status deployment/instantdeploy-backend --timeout=180s
    kubectl -n $Namespace rollout status deployment/instantdeploy-frontend --timeout=180s

    Write-Host "[InstantDeploy] Kubernetes resources deployed."
    kubectl -n $Namespace get deploy,svc,pods -o wide

    try {
        $k8sBackend = Invoke-WebRequest -UseBasicParsing -Uri "$K8sApiURL/api/v1/health" -TimeoutSec 8
        Write-Host "[InstantDeploy] K8s backend healthy at ${K8sApiURL}: HTTP $($k8sBackend.StatusCode)"
    } catch {
        $endpointIP = kubectl -n $Namespace get endpoints instantdeploy-backend -o jsonpath='{.subsets[0].addresses[0].ip}'
        if ($endpointIP) {
            Write-Host "[InstantDeploy] K8s backend service has active endpoint: $endpointIP (host probe at ${K8sApiURL} unavailable from current context)."
        } else {
            Write-Warning "[InstantDeploy] K8s backend health probe failed at ${K8sApiURL}: $($_.Exception.Message)"
        }
    }
}

if ($Target -eq "local" -or $Target -eq "all") {
    Invoke-LocalDeploy
}

if ($Target -eq "k8s" -or $Target -eq "all") {
    Invoke-K8sDeploy
}

Write-Host "[InstantDeploy] Deploy-all completed."
