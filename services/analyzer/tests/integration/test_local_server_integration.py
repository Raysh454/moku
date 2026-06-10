"""Integration test that exercises the running analyzer service over HTTP."""

import os
import socket

import pytest
import requests


def _probe_local_server(host: str, port: int) -> bool:
    try:
        with socket.create_connection((host, port), timeout=0.5):
            return True
    except OSError:
        return False


_HOST = os.environ.get("MOKU_TEST_HOST", "127.0.0.1")
_PORT = int(os.environ.get("MOKU_TEST_PORT", "8080"))


@pytest.mark.integration
@pytest.mark.skipif(
    not _probe_local_server(_HOST, _PORT),
    reason=f"moku-analyzer not running on {_HOST}:{_PORT}",
)
def test_health_endpoint_returns_ok():
    resp = requests.get(f"http://{_HOST}:{_PORT}/health", timeout=5)
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
