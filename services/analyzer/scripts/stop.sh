#!/usr/bin/env bash
# stop.sh — terminate the Moku analyzer sidecar (Linux/macOS)
#
# Reads .run/sidecar.pid and kills the process. No-op if file is missing
# or the PID is already gone.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PID_FILE="${SERVICE_ROOT}/.run/sidecar.pid"

if [[ ! -f "${PID_FILE}" ]]; then
    echo "No pid file; sidecar not running."
    exit 0
fi

pid="$(cat "${PID_FILE}" 2>/dev/null || true)"
if [[ -z "${pid}" ]]; then
    rm -f "${PID_FILE}"
    echo "Empty pid file; cleaned up."
    exit 0
fi

if ! kill -0 "${pid}" 2>/dev/null; then
    rm -f "${PID_FILE}"
    echo "Process ${pid} not found; cleaned up pid file."
    exit 0
fi

if kill "${pid}" 2>/dev/null; then
    # Give it up to 5s to exit gracefully, then SIGKILL.
    for _ in 1 2 3 4 5; do
        if ! kill -0 "${pid}" 2>/dev/null; then
            break
        fi
        sleep 1
    done
    if kill -0 "${pid}" 2>/dev/null; then
        kill -9 "${pid}" 2>/dev/null || true
    fi
    echo "Stopped sidecar (PID ${pid})."
else
    echo "Failed to signal PID ${pid}" >&2
    exit 1
fi

rm -f "${PID_FILE}"
exit 0
