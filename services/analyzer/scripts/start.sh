#!/usr/bin/env bash
# start.sh — bring up the Moku analyzer sidecar (Linux/macOS)
#
# Resolves services/analyzer/.venv/bin/python, creating the venv (and
# installing requirements.txt) on demand. Launches uvicorn detached and
# writes its PID to .run/sidecar.pid with logs going to .run/sidecar.log.
# Polls /health for up to 30s and exits 0 on the first 200.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
VENV_DIR="${SERVICE_ROOT}/.venv"
RUN_DIR="${SERVICE_ROOT}/.run"
PID_FILE="${RUN_DIR}/sidecar.pid"
LOG_FILE="${RUN_DIR}/sidecar.log"
PYTHON_EXE="${VENV_DIR}/bin/python"

HOST="${MOKU_ANALYZER_HOST:-127.0.0.1}"
PORT="${MOKU_ANALYZER_PORT:-8181}"

mkdir -p "${RUN_DIR}"

# Already running? Check the pid file.
if [[ -f "${PID_FILE}" ]]; then
    existing_pid="$(cat "${PID_FILE}" 2>/dev/null || true)"
    if [[ -n "${existing_pid}" ]] && kill -0 "${existing_pid}" 2>/dev/null; then
        echo "Sidecar already running (PID ${existing_pid})."
        if curl --max-time 2 --silent --fail "http://${HOST}:${PORT}/health" >/dev/null; then
            echo "Healthy on http://${HOST}:${PORT}"
            exit 0
        fi
        echo "PID exists but /health is not responding; run stop first."
        exit 1
    fi
    rm -f "${PID_FILE}"
fi

# Provision venv if missing.
if [[ ! -x "${PYTHON_EXE}" ]]; then
    echo "Creating venv at ${VENV_DIR} ..."
    python3 -m venv "${VENV_DIR}"
    echo "Installing requirements ..."
    "${PYTHON_EXE}" -m pip install --upgrade pip
    "${PYTHON_EXE}" -m pip install -r "${SERVICE_ROOT}/requirements.txt"
fi

echo "Starting sidecar on http://${HOST}:${PORT} ..."
cd "${SERVICE_ROOT}"
nohup "${PYTHON_EXE}" -m uvicorn main:app --host "${HOST}" --port "${PORT}" \
    >"${LOG_FILE}" 2>&1 &
SIDECAR_PID=$!
echo "${SIDECAR_PID}" > "${PID_FILE}"
echo "Sidecar PID ${SIDECAR_PID} (log: ${LOG_FILE})"

# Poll /health for up to 30 s.
HEALTH_URL="http://${HOST}:${PORT}/health"
for _ in $(seq 1 30); do
    sleep 1
    if ! kill -0 "${SIDECAR_PID}" 2>/dev/null; then
        echo "Sidecar process exited early. See ${LOG_FILE}" >&2
        exit 1
    fi
    if curl --max-time 1 --silent --fail "${HEALTH_URL}" >/dev/null; then
        echo "Sidecar healthy on ${HEALTH_URL}"
        exit 0
    fi
done

echo "Sidecar did not become healthy within 30 s. See ${LOG_FILE}" >&2
exit 1
