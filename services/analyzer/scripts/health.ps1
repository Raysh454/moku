# health.ps1 — one-shot health probe (Windows)
#
# GET http://${MOKU_ANALYZER_HOST:-127.0.0.1}:${MOKU_ANALYZER_PORT:-8181}/health
# Exits 0 on 200, non-zero otherwise.

$ErrorActionPreference = "Stop"

$Host_ = if ($env:MOKU_ANALYZER_HOST) { $env:MOKU_ANALYZER_HOST } else { "127.0.0.1" }
$Port = if ($env:MOKU_ANALYZER_PORT) { $env:MOKU_ANALYZER_PORT } else { "8181" }
$Url = "http://${Host_}:${Port}/health"

try {
    $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
    if ($resp.StatusCode -eq 200) {
        Write-Output $resp.Content
        exit 0
    }
    Write-Error "Health endpoint returned status $($resp.StatusCode)"
    exit 1
} catch {
    Write-Error "Health probe failed: $_"
    exit 1
}
