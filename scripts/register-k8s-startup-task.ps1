param(
    [string]$TaskName = "InstantDeploy-K8s-Autostart"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Path $PSScriptRoot -Parent
$startScript = Join-Path $repoRoot "scripts\start-k8s-local.ps1"
$startupFolder = [Environment]::GetFolderPath("Startup")
$startupCmdPath = Join-Path $startupFolder "InstantDeploy-K8s-Autostart.cmd"

if (-not (Test-Path $startScript)) {
    throw "Required start script not found: $startScript"
}

$taskCommand = "`"powershell.exe`" -NoProfile -ExecutionPolicy Bypass -File `"$startScript`""

$queryProc = Start-Process -FilePath "schtasks.exe" -ArgumentList @("/Query", "/TN", $TaskName) -NoNewWindow -Wait -PassThru
if ($queryProc.ExitCode -eq 0) {
    Write-Host "[InstantDeploy] Existing task '$TaskName' found. Replacing..."
    schtasks /Delete /TN $TaskName /F | Out-Null
}

$createArgs = @(
    "/Create",
    "/SC", "ONLOGON",
    "/TN", $TaskName,
    "/TR", "`"$taskCommand`"",
    "/RL", "LIMITED",
    "/F"
)
$createProc = Start-Process -FilePath "schtasks.exe" -ArgumentList $createArgs -NoNewWindow -Wait -PassThru
if ($createProc.ExitCode -eq 0) {
    Write-Host "[InstantDeploy] Startup task created: $TaskName"
    Write-Host "[InstantDeploy] On each login it will run: $startScript"
    Write-Host "[InstantDeploy] To remove: schtasks /Delete /TN $TaskName /F"
    return
}

Write-Warning "[InstantDeploy] Scheduled task creation failed. Falling back to Startup folder launcher."

$startupCommandLines = @(
    "@echo off",
    "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$startScript`" -SkipRestart"
)
$startupCommand = ($startupCommandLines -join "`r`n") + "`r`n"
Set-Content -Path $startupCmdPath -Value $startupCommand -Encoding ASCII

Write-Host "[InstantDeploy] Startup launcher created: $startupCmdPath"
Write-Host "[InstantDeploy] It will run at login for the current user."
