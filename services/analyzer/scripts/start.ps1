# start.ps1 — bring up the Moku analyzer sidecar (Windows)
#
# Resolves services/analyzer/.venv/Scripts/python.exe, creating the venv
# (and installing requirements.txt) on demand. Launches uvicorn detached
# and writes its PID to .run/sidecar.pid with logs going to .run/sidecar.log.
# Polls /health for up to 30s and exits 0 on the first 200.

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ServiceRoot = Split-Path -Parent $ScriptDir
$VenvDir = Join-Path $ServiceRoot ".venv"
$RunDir = Join-Path $ServiceRoot ".run"
$PidFile = Join-Path $RunDir "sidecar.pid"
$LogFile = Join-Path $RunDir "sidecar.log"
$PythonExe = Join-Path $VenvDir "Scripts\python.exe"

$Host_ = if ($env:MOKU_ANALYZER_HOST) { $env:MOKU_ANALYZER_HOST } else { "127.0.0.1" }
$Port = if ($env:MOKU_ANALYZER_PORT) { $env:MOKU_ANALYZER_PORT } else { "8181" }

if (-not (Test-Path $RunDir)) {
    New-Item -ItemType Directory -Path $RunDir | Out-Null
}

# Already running? Check the pid file.
if (Test-Path $PidFile) {
    $existingPid = Get-Content $PidFile -ErrorAction SilentlyContinue
    if ($existingPid) {
        $proc = Get-Process -Id $existingPid -ErrorAction SilentlyContinue
        if ($proc) {
            Write-Output "Sidecar already running (PID $existingPid)."
            # Still confirm health is good before returning success.
            try {
                $resp = Invoke-WebRequest -Uri "http://${Host_}:${Port}/health" -UseBasicParsing -TimeoutSec 2
                if ($resp.StatusCode -eq 200) {
                    Write-Output "Healthy on http://${Host_}:${Port}"
                    exit 0
                }
            } catch {
                Write-Output "PID exists but /health is not responding; will not restart automatically. Run stop first."
                exit 1
            }
        } else {
            # Stale pid file
            Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
        }
    }
}

# Provision venv if missing.
if (-not (Test-Path $PythonExe)) {
    Write-Output "Creating venv at $VenvDir ..."
    python -m venv $VenvDir
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to create venv"
        exit 1
    }
    Write-Output "Installing requirements ..."
    & $PythonExe -m pip install --upgrade pip
    & $PythonExe -m pip install -r (Join-Path $ServiceRoot "requirements.txt")
    if ($LASTEXITCODE -ne 0) {
        Write-Error "pip install failed"
        exit 1
    }
}

# Start uvicorn detached, redirecting stdout/stderr to log file.
Write-Output "Starting sidecar on http://${Host_}:${Port} ..."
$startInfo = New-Object System.Diagnostics.ProcessStartInfo
$startInfo.FileName = $PythonExe
$startInfo.Arguments = "-m uvicorn main:app --host $Host_ --port $Port"
$startInfo.WorkingDirectory = $ServiceRoot
$startInfo.RedirectStandardOutput = $true
$startInfo.RedirectStandardError = $true
$startInfo.UseShellExecute = $false
$startInfo.CreateNoWindow = $true

# Open the log file for both stdout and stderr.
$logStream = [System.IO.StreamWriter]::new($LogFile, $false)
$logStream.AutoFlush = $true

$proc = New-Object System.Diagnostics.Process
$proc.StartInfo = $startInfo
$proc.EnableRaisingEvents = $true

$outHandler = {
    if ($EventArgs.Data) { $Event.MessageData.WriteLine($EventArgs.Data) }
}
Register-ObjectEvent -InputObject $proc -EventName OutputDataReceived -Action $outHandler -MessageData $logStream | Out-Null
Register-ObjectEvent -InputObject $proc -EventName ErrorDataReceived -Action $outHandler -MessageData $logStream | Out-Null

[void]$proc.Start()
$proc.BeginOutputReadLine()
$proc.BeginErrorReadLine()

Set-Content -Path $PidFile -Value $proc.Id
Write-Output "Sidecar PID $($proc.Id) (log: $LogFile)"

# Poll /health for up to 30 s.
$healthUrl = "http://${Host_}:${Port}/health"
$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 1
    if ($proc.HasExited) {
        Write-Error "Sidecar process exited early. See $LogFile"
        exit 1
    }
    try {
        $resp = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing -TimeoutSec 2
        if ($resp.StatusCode -eq 200) {
            Write-Output "Sidecar healthy on $healthUrl"
            exit 0
        }
    } catch {
        # not ready yet
    }
}

Write-Error "Sidecar did not become healthy within 30 s. See $LogFile"
exit 1
