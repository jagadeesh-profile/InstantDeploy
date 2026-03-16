param(
    [int]$BackendPort = 8082,
    [int]$FrontendPort = 5173
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Path $PSScriptRoot -Parent
$backendDir = Join-Path $root "backend"
$frontendDir = Join-Path $root "frontend"
$infraCompose = Join-Path $root "infrastructure\docker-compose.yml"
$pidDir = Join-Path $PSScriptRoot ".pids"
$logDir = Join-Path $PSScriptRoot ".logs"

New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $logDir -Force | Out-Null

function Stop-PortListener([int]$Port) {
    $conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($conn) {
        try {
            Stop-Process -Id $conn.OwningProcess -Force -ErrorAction Stop
            Write-Host "[InstantDeploy] Freed port $Port (PID $($conn.OwningProcess))."
        } catch {
            Write-Warning "[InstantDeploy] Failed to stop PID $($conn.OwningProcess) on port ${Port}: $($_.Exception.Message)"
        }
    }
}

function Start-Background([string]$Name, [string]$Command, [string]$OutLog, [string]$ErrLog, [string]$PidPath) {
    $encodedCommand = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($Command))
    $proc = Start-Process -FilePath "powershell" `
        -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", $encodedCommand) `
        -RedirectStandardOutput $OutLog `
        -RedirectStandardError $ErrLog `
        -PassThru

    $proc.Id | Set-Content -Path $PidPath -Encoding ASCII
    Write-Host "[InstantDeploy] $Name started (PID $($proc.Id))."
}

Write-Host "[InstantDeploy] Starting redis/postgres dependencies..."
docker compose -f $infraCompose up -d postgres redis | Out-Null

Stop-PortListener -Port $BackendPort
Stop-PortListener -Port $FrontendPort

$backendPidPath = Join-Path $pidDir "backend.pid"
$frontendPidPath = Join-Path $pidDir "frontend.pid"

$backendOut = Join-Path $logDir "backend.out.log"
$backendErr = Join-Path $logDir "backend.err.log"
$frontendOut = Join-Path $logDir "frontend.out.log"
$frontendErr = Join-Path $logDir "frontend.err.log"

$goBinDir = Join-Path $env:USERPROFILE "tools\go\bin"
$backendApiUrl = "http://localhost:$BackendPort"

$backendCommand = @"
`$env:Path = "$goBinDir;`$env:Path"
`$env:PORT = "$BackendPort"
`$env:DATABASE_URL = "postgres://instantdeploy_user:instantdeploy_pass@localhost:5432/instantdeploy?sslmode=disable"
`$env:REDIS_ADDR = "localhost:6379"
`$env:BUILD_QUEUE_KEY = "instantdeploy:build_queue:local"
go -C "$backendDir" run ./cmd/server
"@

$frontendCommand = @"
Set-Location "$frontendDir"
`$env:VITE_API_URL = "$backendApiUrl"
npm run dev -- --host=0.0.0.0 --port=$FrontendPort
"@

Start-Background -Name "Backend" -Command $backendCommand -OutLog $backendOut -ErrLog $backendErr -PidPath $backendPidPath
Start-Sleep -Seconds 2
Start-Background -Name "Frontend" -Command $frontendCommand -OutLog $frontendOut -ErrLog $frontendErr -PidPath $frontendPidPath

Write-Host "[InstantDeploy] Local dev stack started."
Write-Host "[InstantDeploy] Backend:  http://localhost:$BackendPort/api/v1/health"
Write-Host "[InstantDeploy] Frontend: http://localhost:$FrontendPort"
Write-Host "[InstantDeploy] Logs:     $logDir"
