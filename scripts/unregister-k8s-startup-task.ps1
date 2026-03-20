param(
    [string]$TaskName = "InstantDeploy-K8s-Autostart"
)

$ErrorActionPreference = "Stop"

$startupFolder = [Environment]::GetFolderPath("Startup")
$startupCmdPath = Join-Path $startupFolder "InstantDeploy-K8s-Autostart.cmd"

$null = cmd /c "schtasks /Delete /TN \"$TaskName\" /F >nul 2>&1"
if ($LASTEXITCODE -eq 0) {
    Write-Host "[InstantDeploy] Startup task removed: $TaskName"
} else {
    Write-Host "[InstantDeploy] Scheduled task not found or already removed: $TaskName"
}

if (Test-Path $startupCmdPath) {
    Remove-Item -Path $startupCmdPath -Force
    Write-Host "[InstantDeploy] Startup launcher removed: $startupCmdPath"
}
