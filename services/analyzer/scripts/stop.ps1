# stop.ps1 — terminate the Moku analyzer sidecar (Windows)
#
# Reads .run/sidecar.pid and kills the process. No-op if file is missing
# or the PID is already gone.

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ServiceRoot = Split-Path -Parent $ScriptDir
$PidFile = Join-Path $ServiceRoot ".run\sidecar.pid"

if (-not (Test-Path $PidFile)) {
    Write-Output "No pid file; sidecar not running."
    exit 0
}

$pidValue = (Get-Content $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1)
if (-not $pidValue) {
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    Write-Output "Empty pid file; cleaned up."
    exit 0
}

$proc = Get-Process -Id $pidValue -ErrorAction SilentlyContinue
if (-not $proc) {
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    Write-Output "Process $pidValue not found; cleaned up pid file."
    exit 0
}

try {
    Stop-Process -Id $pidValue -Force -ErrorAction Stop
    Write-Output "Stopped sidecar (PID $pidValue)."
} catch {
    Write-Error "Failed to stop PID $pidValue : $_"
    exit 1
}

Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
exit 0
