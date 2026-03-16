param(
    [ValidateSet("start", "stop", "status", "logs")]
    [string]$Action = "status",
    [int]$BackendPort = 8082,
    [int]$FrontendPort = 5173,
    [ValidateSet("backend", "frontend", "all")]
    [string]$Service = "all",
    [int]$Tail = 80
)

$ErrorActionPreference = "Stop"

$startScript = Join-Path $PSScriptRoot "start-local-dev.ps1"
$stopScript = Join-Path $PSScriptRoot "stop-local-dev.ps1"
$pidDir = Join-Path $PSScriptRoot ".pids"
$logDir = Join-Path $PSScriptRoot ".logs"

function Get-PidFromFile([string]$PidFile) {
    if (-not (Test-Path $PidFile)) {
        return $null
    }

    $value = Get-Content -Path $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $value) {
        return $null
    }

    $parsedPid = 0
    if (-not [int]::TryParse($value, [ref]$parsedPid)) {
        return $null
    }

    return $parsedPid
}

function Print-ServiceStatus([string]$Name, [string]$PidFile, [int]$Port) {
    $servicePid = Get-PidFromFile -PidFile $PidFile
    if ($null -eq $servicePid) {
        $portConn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($portConn) {
            Write-Host "[InstantDeploy] ${Name}: running (PID $($portConn.OwningProcess), unmanaged by script, port $Port)."
        } else {
            Write-Host "[InstantDeploy] ${Name}: stopped (no pid file)."
        }
        return
    }

    $proc = Get-Process -Id $servicePid -ErrorAction SilentlyContinue
    if ($proc) {
        Write-Host "[InstantDeploy] ${Name}: running (PID $servicePid, port $Port)."
    } else {
        Write-Host "[InstantDeploy] ${Name}: stale pid file (PID $servicePid not running)."
    }
}

function Print-Log([string]$Name, [string]$Path, [int]$TailLines) {
    Write-Host "[InstantDeploy] $Name log: $Path"
    if (-not (Test-Path $Path)) {
        Write-Host "[InstantDeploy] $Name log file not found."
        return
    }

    Get-Content -Path $Path -Tail $TailLines
}

switch ($Action) {
    "start" {
        if (-not (Test-Path $startScript)) {
            throw "Missing script: $startScript"
        }

        & $startScript -BackendPort $BackendPort -FrontendPort $FrontendPort
        exit $LASTEXITCODE
    }
    "stop" {
        if (-not (Test-Path $stopScript)) {
            throw "Missing script: $stopScript"
        }

        & $stopScript
        exit $LASTEXITCODE
    }
    "status" {
        $backendPidFile = Join-Path $pidDir "backend.pid"
        $frontendPidFile = Join-Path $pidDir "frontend.pid"

        Print-ServiceStatus -Name "Backend" -PidFile $backendPidFile -Port $BackendPort
        Print-ServiceStatus -Name "Frontend" -PidFile $frontendPidFile -Port $FrontendPort

        $backendHealth = "http://localhost:$BackendPort/api/v1/health"
        $frontendRoot = "http://localhost:$FrontendPort"
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri $backendHealth -TimeoutSec 4
            Write-Host "[InstantDeploy] Backend health: HTTP $($resp.StatusCode)"
        } catch {
            Write-Host "[InstantDeploy] Backend health: unavailable"
        }

        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri $frontendRoot -TimeoutSec 4
            Write-Host "[InstantDeploy] Frontend health: HTTP $($resp.StatusCode)"
        } catch {
            Write-Host "[InstantDeploy] Frontend health: unavailable"
        }
    }
    "logs" {
        if (-not (Test-Path $logDir)) {
            Write-Host "[InstantDeploy] Log directory not found: $logDir"
            return
        }

        if ($Service -eq "backend" -or $Service -eq "all") {
            Print-Log -Name "Backend stdout" -Path (Join-Path $logDir "backend.out.log") -TailLines $Tail
            Print-Log -Name "Backend stderr" -Path (Join-Path $logDir "backend.err.log") -TailLines $Tail
        }

        if ($Service -eq "frontend" -or $Service -eq "all") {
            Print-Log -Name "Frontend stdout" -Path (Join-Path $logDir "frontend.out.log") -TailLines $Tail
            Print-Log -Name "Frontend stderr" -Path (Join-Path $logDir "frontend.err.log") -TailLines $Tail
        }
    }
}