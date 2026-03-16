$ErrorActionPreference = "Stop"

$pidDir = Join-Path $PSScriptRoot ".pids"

function Stop-ByPidFile([string]$Name, [string]$PidFile) {
    if (-not (Test-Path $PidFile)) {
        Write-Host "[InstantDeploy] $Name PID file not found."
        return
    }

    $pidValue = Get-Content -Path $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $pidValue) {
        Remove-Item -Path $PidFile -Force -ErrorAction SilentlyContinue
        Write-Host "[InstantDeploy] $Name PID file was empty."
        return
    }

    $parsedPid = 0
    if (-not [int]::TryParse($pidValue, [ref]$parsedPid)) {
        Remove-Item -Path $PidFile -Force -ErrorAction SilentlyContinue
        Write-Host "[InstantDeploy] $Name PID file had invalid content."
        return
    }

    $proc = Get-Process -Id $parsedPid -ErrorAction SilentlyContinue
    if ($proc) {
        try {
            Stop-Process -Id $parsedPid -Force -ErrorAction Stop
            Write-Host "[InstantDeploy] Stopped $Name (PID $parsedPid)."
        } catch {
            Write-Warning "[InstantDeploy] Failed to stop $Name (PID $parsedPid): $($_.Exception.Message)"
        }
    } else {
        Write-Host "[InstantDeploy] $Name process not running (PID $parsedPid)."
    }

    Remove-Item -Path $PidFile -Force -ErrorAction SilentlyContinue
}

$backendPidFile = Join-Path $pidDir "backend.pid"
$frontendPidFile = Join-Path $pidDir "frontend.pid"

Stop-ByPidFile -Name "Backend" -PidFile $backendPidFile
Stop-ByPidFile -Name "Frontend" -PidFile $frontendPidFile

Write-Host "[InstantDeploy] Local dev processes stopped."
