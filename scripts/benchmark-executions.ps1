param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$RepoList = "$PSScriptRoot/benchmark-repos.json",
    [int]$PerRepoRuns = 1,
    [int]$TimeoutSeconds = 900,
    [int]$PollSeconds = 5,
    [string]$OutputJson = "$PSScriptRoot/benchmark-result.json"
)

$ErrorActionPreference = "Stop"

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Url,
        [object]$Body,
        [hashtable]$Headers
    )

    $params = @{ Method = $Method; Uri = $Url; ErrorAction = "Stop" }
    if ($Headers) { $params.Headers = $Headers }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = ($Body | ConvertTo-Json -Depth 8)
    }

    return Invoke-RestMethod @params
}

function New-RandomString([int]$len = 8) {
    $chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    $sb = New-Object System.Text.StringBuilder
    for ($i = 0; $i -lt $len; $i++) {
        [void]$sb.Append($chars[(Get-Random -Minimum 0 -Maximum $chars.Length)])
    }
    return $sb.ToString()
}

if (-not (Test-Path $RepoList)) {
    throw "Repository list not found: $RepoList"
}

$repos = Get-Content -Path $RepoList -Raw | ConvertFrom-Json
if (-not $repos -or $repos.Count -eq 0) {
    throw "Repository list is empty: $RepoList"
}

$apiRoot = "$BaseUrl/api/v1"
$runSuffix = New-RandomString 6
$username = "bench_$runSuffix"
$email = "$username@instantdeploy.local"
$password = "Bench123!"

Write-Host "[benchmark] signing up user: $username"
$signup = Invoke-Api -Method "POST" -Url "$apiRoot/auth/signup" -Body @{ username = $username; email = $email; password = $password } -Headers @{}
$verificationCode = "$($signup.verification_code)"
if ([string]::IsNullOrWhiteSpace($verificationCode)) {
    throw "verification_code missing in signup response. Run backend in development mode for benchmark automation."
}

Write-Host "[benchmark] verifying user"
[void](Invoke-Api -Method "POST" -Url "$apiRoot/auth/verify" -Body @{ username = $username; code = $verificationCode } -Headers @{})

Write-Host "[benchmark] logging in"
$login = Invoke-Api -Method "POST" -Url "$apiRoot/auth/login" -Body @{ username = $username; password = $password } -Headers @{}
$token = "$($login.token)"
if ([string]::IsNullOrWhiteSpace($token)) {
    throw "login token missing"
}
$authHeaders = @{ Authorization = "Bearer $token" }

$results = @()
$total = 0
$success = 0
$failed = 0

foreach ($repo in $repos) {
    for ($run = 1; $run -le $PerRepoRuns; $run++) {
        $total++
        $repoName = "$($repo.name)"
        $repository = "$($repo.repository)"
        $branch = if ([string]::IsNullOrWhiteSpace("$($repo.branch)")) { "main" } else { "$($repo.branch)" }

        Write-Host "[benchmark] run $total -> $repoName ($repository@$branch)"
        $startedAt = Get-Date
        $dep = $null
        $finalStatus = "unknown"
        $finalError = ""
        $durationSec = 0

        try {
            $dep = Invoke-Api -Method "POST" -Url "$apiRoot/deployments" -Body @{ repository = $repository; branch = $branch; url = "" } -Headers $authHeaders
            $depId = "$($dep.id)"
            if ([string]::IsNullOrWhiteSpace($depId)) {
                throw "deployment id missing"
            }

            $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
            $pollErrors = 0
            while ((Get-Date) -lt $deadline) {
                try {
                    $statusResp = Invoke-Api -Method "GET" -Url "$apiRoot/deployments/$depId/status" -Body $null -Headers $authHeaders
                    $currentStatus = "$($statusResp.status)"
                } catch {
                    $pollErrors++
                    $finalError = "status poll error #${pollErrors}: $($_.Exception.Message)"
                    Start-Sleep -Seconds $PollSeconds
                    continue
                }

                if ($currentStatus -eq "running") {
                    $finalStatus = "running"
                    $finalError = ""
                    break
                }
                if ($currentStatus -eq "failed") {
                    $finalStatus = "failed"
                    $finalError = "$($statusResp.error)"
                    break
                }

                Start-Sleep -Seconds $PollSeconds
            }

            if ($finalStatus -eq "unknown") {
                $finalStatus = "timeout"
                if ([string]::IsNullOrWhiteSpace($finalError) -and $pollErrors -gt 0) {
                    $finalError = "timed out after transient poll errors ($pollErrors)"
                }
            }
        } catch {
            $finalStatus = "error"
            $finalError = $_.Exception.Message
        }

        $durationSec = [int]((Get-Date) - $startedAt).TotalSeconds
        if ($finalStatus -eq "running") { $success++ } else { $failed++ }

        $results += [PSCustomObject]@{
            repoName     = $repoName
            repository   = $repository
            branch       = $branch
            run          = $run
            status       = $finalStatus
            durationSec  = $durationSec
            error        = $finalError
            timestampUtc = (Get-Date).ToUniversalTime().ToString("o")
        }
    }
}

$successRate = if ($total -eq 0) { 0 } else { [Math]::Round(($success * 100.0) / $total, 2) }
$summary = [PSCustomObject]@{
    baseUrl      = $BaseUrl
    totalRuns    = $total
    successRuns  = $success
    failedRuns   = $failed
    successRate  = $successRate
    timeoutSec   = $TimeoutSeconds
    pollSec      = $PollSeconds
    generatedUtc = (Get-Date).ToUniversalTime().ToString("o")
    results      = $results
}

$summary | ConvertTo-Json -Depth 8 | Set-Content -Path $OutputJson -Encoding UTF8

Write-Host ""
Write-Host "[benchmark] complete"
Write-Host "  total runs   : $total"
Write-Host "  success runs : $success"
Write-Host "  failed runs  : $failed"
Write-Host "  success rate : $successRate%"
Write-Host "  output       : $OutputJson"

if ($successRate -lt 95) {
    exit 2
}
exit 0
