param(
    [switch]$BuildImages,
    [int]$DockerTimeoutSeconds = 240,
    [int]$K8sTimeoutSeconds = 300,
    [switch]$SkipRestart
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Path $PSScriptRoot -Parent
$kustomizeDataStores = Join-Path $repoRoot "infrastructure\k8s\datastores"
$kustomizeBackend = Join-Path $repoRoot "infrastructure\k8s\backend"
$kustomizeFrontend = Join-Path $repoRoot "infrastructure\k8s\frontend"
$backendDir = Join-Path $repoRoot "backend"
$frontendDir = Join-Path $repoRoot "frontend"
$namespace = "instantdeploy"
$pidDir = Join-Path $PSScriptRoot ".pids"
$logDir = Join-Path $PSScriptRoot ".logs"

New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $logDir -Force | Out-Null

function Write-Step([string]$Message) {
    Write-Host "[InstantDeploy] $Message"
}

function Wait-ForDocker([int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            docker info | Out-Null
            return
        } catch {
            Start-Sleep -Seconds 5
        }
    }
    throw "Docker Desktop did not become ready within $TimeoutSeconds seconds."
}

function Wait-ForKubernetes([int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $nodes = kubectl get nodes --no-headers 2>$null
            if ($nodes) {
                return
            }
        } catch {
            # keep waiting
        }
        Start-Sleep -Seconds 5
    }
    throw "Kubernetes cluster did not become ready within $TimeoutSeconds seconds."
}

function Ensure-IngressController([int]$TimeoutSeconds) {
    $controller = kubectl get deployment ingress-nginx-controller -n ingress-nginx --ignore-not-found 2>$null
    if (-not [string]::IsNullOrWhiteSpace($controller)) {
        kubectl rollout status deployment/ingress-nginx-controller -n ingress-nginx --timeout="${TimeoutSeconds}s" | Out-Null
        Write-Step "Ingress controller is ready."
        return
    }

    Write-Step "Installing ingress-nginx controller..."
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml | Out-Null
    kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout="${TimeoutSeconds}s" | Out-Null
    Write-Step "Ingress controller installed and ready."
}

function Has-Deployment([string]$DeploymentName) {
    $result = kubectl get deployment $DeploymentName -n $namespace --ignore-not-found
    return -not [string]::IsNullOrWhiteSpace($result)
}

function Has-Service([string]$ServiceName) {
    $result = kubectl get service $ServiceName -n $namespace --ignore-not-found
    return -not [string]::IsNullOrWhiteSpace($result)
}

function Test-Http([string]$Url) {
    try {
        $resp = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 5
        return $resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500
    } catch {
        return $false
    }
}

function Ensure-PortForward([string]$Name, [string]$ServiceName, [int]$LocalPort, [int]$ServicePort) {
    $listener = Get-NetTCPConnection -LocalPort $LocalPort -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($listener) {
        return
    }

    $outLog = Join-Path $logDir ("{0}.out.log" -f $Name)
    $errLog = Join-Path $logDir ("{0}.err.log" -f $Name)
    $pidPath = Join-Path $pidDir ("{0}.pid" -f $Name)

    $pfCommand = "kubectl port-forward svc/$ServiceName ${LocalPort}:${ServicePort} -n $namespace"
    $encodedCommand = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($pfCommand))

    $proc = Start-Process -FilePath "powershell" `
        -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", $encodedCommand) `
        -RedirectStandardOutput $outLog `
        -RedirectStandardError $errLog `
        -PassThru

    $proc.Id | Set-Content -Path $pidPath -Encoding ASCII
    Start-Sleep -Seconds 2
    Write-Step ("Started port-forward '{0}' (PID {1}): localhost:{2} -> svc/{3}:{4}" -f $Name, $proc.Id, $LocalPort, $ServiceName, $ServicePort)
}

function Wait-ForDeployment([string[]]$Candidates, [string]$Description, [int]$TimeoutSeconds) {
    foreach ($name in $Candidates) {
        if (Has-Deployment -DeploymentName $name) {
            kubectl rollout status "deployment/$name" -n $namespace --timeout="${TimeoutSeconds}s" | Out-Null
            Write-Step "$Description is ready ($name)."
            return
        }
    }
    throw "$Description deployment not found in namespace '$namespace'."
}

function Ensure-BaseResources() {
    $hasBackend = (Has-Deployment -DeploymentName "instantdeploy-backend") -or (Has-Deployment -DeploymentName "instantdeploy-instantdeploy-backend")
    $hasFrontend = (Has-Deployment -DeploymentName "instantdeploy-frontend") -or (Has-Deployment -DeploymentName "instantdeploy-instantdeploy-frontend")
    $hasPostgres = (Has-Deployment -DeploymentName "instantdeploy-postgres") -or (Has-Deployment -DeploymentName "instantdeploy-instantdeploy-postgres")
    $hasRedis = (Has-Deployment -DeploymentName "instantdeploy-redis") -or (Has-Deployment -DeploymentName "instantdeploy-instantdeploy-redis")

    if ($hasBackend -and $hasFrontend -and $hasPostgres -and $hasRedis) {
        Write-Step "Existing Kubernetes resources detected. Skipping manifest re-apply."
        return
    }

    Write-Step "Bootstrapping Kubernetes resources (datastores/backend/frontend)..."
    kubectl apply -k $kustomizeDataStores | Out-Null
    kubectl apply -k $kustomizeBackend | Out-Null
    kubectl apply -k $kustomizeFrontend | Out-Null
}

Write-Step "Waiting for Docker Desktop..."
Wait-ForDocker -TimeoutSeconds $DockerTimeoutSeconds
Write-Step "Docker Desktop is ready."

Write-Step "Waiting for Kubernetes cluster..."
Wait-ForKubernetes -TimeoutSeconds $K8sTimeoutSeconds
Write-Step "Kubernetes cluster is ready."

Ensure-IngressController -TimeoutSeconds $K8sTimeoutSeconds

if ($BuildImages) {
    Write-Step "Building backend image (instantdeploy-backend:local)..."
    docker build -t instantdeploy-backend:local $backendDir | Out-Null

    Write-Step "Building frontend image (instantdeploy-frontend:local)..."
    docker build -t instantdeploy-frontend:local $frontendDir | Out-Null
}

Ensure-BaseResources

if (-not $SkipRestart) {
    Write-Step "Restarting backend and frontend deployments..."
    if (Has-Deployment -DeploymentName "instantdeploy-backend") {
        kubectl rollout restart deployment/instantdeploy-backend -n $namespace | Out-Null
    } elseif (Has-Deployment -DeploymentName "instantdeploy-instantdeploy-backend") {
        kubectl rollout restart deployment/instantdeploy-instantdeploy-backend -n $namespace | Out-Null
    }

    if (Has-Deployment -DeploymentName "instantdeploy-frontend") {
        kubectl rollout restart deployment/instantdeploy-frontend -n $namespace | Out-Null
    } elseif (Has-Deployment -DeploymentName "instantdeploy-instantdeploy-frontend") {
        kubectl rollout restart deployment/instantdeploy-instantdeploy-frontend -n $namespace | Out-Null
    }
}

Write-Step "Waiting for datastores..."
Wait-ForDeployment -Candidates @("instantdeploy-postgres", "instantdeploy-instantdeploy-postgres") -Description "PostgreSQL" -TimeoutSeconds 180
Wait-ForDeployment -Candidates @("instantdeploy-redis", "instantdeploy-instantdeploy-redis") -Description "Redis" -TimeoutSeconds 120

Write-Step "Waiting for backend/frontend..."
Wait-ForDeployment -Candidates @("instantdeploy-backend", "instantdeploy-instantdeploy-backend") -Description "Backend" -TimeoutSeconds 240
Wait-ForDeployment -Candidates @("instantdeploy-frontend", "instantdeploy-instantdeploy-frontend") -Description "Frontend" -TimeoutSeconds 180

$backendUrl = "http://localhost:30080/api/v1/health"
$frontendUrl = "http://localhost:30000"

if (-not (Test-Http -Url $backendUrl)) {
    if (Has-Service -ServiceName "instantdeploy-backend") {
        Ensure-PortForward -Name "backend-portforward" -ServiceName "instantdeploy-backend" -LocalPort 30080 -ServicePort 8080
    } elseif (Has-Service -ServiceName "instantdeploy-instantdeploy-backend") {
        Ensure-PortForward -Name "backend-portforward" -ServiceName "instantdeploy-instantdeploy-backend" -LocalPort 30080 -ServicePort 8080
    }
}

if (-not (Test-Http -Url $frontendUrl)) {
    if (Has-Service -ServiceName "instantdeploy-frontend") {
        Ensure-PortForward -Name "frontend-portforward" -ServiceName "instantdeploy-frontend" -LocalPort 30000 -ServicePort 80
    } elseif (Has-Service -ServiceName "instantdeploy-instantdeploy-frontend") {
        Ensure-PortForward -Name "frontend-portforward" -ServiceName "instantdeploy-instantdeploy-frontend" -LocalPort 30000 -ServicePort 80
    }
}

Ensure-PortForward -Name "ingress-portforward" -ServiceName "ingress-nginx-controller" -LocalPort 8088 -ServicePort 80

$backendHealthy = Test-Http -Url $backendUrl

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host " InstantDeploy Kubernetes stack is ready" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host " Frontend: http://localhost:30000"
Write-Host " Backend:  http://localhost:30080"
Write-Host " Ingress:  http://localhost:8088/instantdeploy/"
if ($backendHealthy) {
    Write-Host " Health:   http://localhost:30080/api/v1/health (OK)" -ForegroundColor Green
} else {
    Write-Host " Health:   http://localhost:30080/api/v1/health (check manually)" -ForegroundColor Yellow
}
