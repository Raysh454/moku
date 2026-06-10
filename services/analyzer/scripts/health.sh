#!/usr/bin/env bash
# health.sh — one-shot health probe (Linux/macOS)
#
# GET http://${MOKU_ANALYZER_HOST:-127.0.0.1}:${MOKU_ANALYZER_PORT:-8181}/health
# Exits 0 on 200, non-zero otherwise.

set -euo pipefail

HOST="${MOKU_ANALYZER_HOST:-127.0.0.1}"
PORT="${MOKU_ANALYZER_PORT:-8181}"
URL="http://${HOST}:${PORT}/health"

if curl --max-time 3 --silent --fail --show-error "${URL}"; then
    echo
    exit 0
fi

echo "Health probe failed for ${URL}" >&2
exit 1
