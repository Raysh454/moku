"""Tests for the Shodan adapter."""

from unittest.mock import MagicMock

import pytest

from app.adapters.shodan_adapter import ShodanAdapter
from app.models.schemas import Backend, ScanRequest


def _response(status_code: int = 200, body: dict | None = None):
    resp = MagicMock()
    resp.status_code = status_code
    resp.json.return_value = body or {}
    resp.text = ""
    return resp


def test_parses_userinfo_url(monkeypatch):
    monkeypatch.setenv("SHODAN_API_KEY", "secret-key")
    monkeypatch.setattr(
        "app.adapters.shodan_adapter.socket.getaddrinfo",
        lambda host, port: [(0, 0, 0, "", ("8.8.8.8", 0))],
    )
    monkeypatch.setattr(
        "app.adapters.shodan_adapter.requests.get",
        lambda *a, **kw: _response(
            200,
            {"data": [{"port": 80, "product": "nginx"}], "hostnames": ["example.com"]},
        ),
    )
    adapter = ShodanAdapter()
    findings = adapter.run_scan(
        ScanRequest(
            url="https://user:pass@example.com:8080/path", backend=Backend.SHODAN
        )
    )
    assert len(findings) == 1
    assert "nginx" in findings[0].description


def test_rejects_private_target(monkeypatch):
    monkeypatch.setenv("SHODAN_API_KEY", "secret-key")
    monkeypatch.setattr(
        "app.adapters.shodan_adapter.socket.getaddrinfo",
        lambda host, port: [(0, 0, 0, "", ("127.0.0.1", 0))],
    )
    adapter = ShodanAdapter()
    with pytest.raises(ValueError):
        adapter.run_scan(
            ScanRequest(url="https://internal.example.com", backend=Backend.SHODAN)
        )


def test_transport_failure_does_not_leak_api_key(monkeypatch):
    import requests as real_requests

    monkeypatch.setenv("SHODAN_API_KEY", "super-secret-key")
    monkeypatch.setattr(
        "app.adapters.shodan_adapter.socket.getaddrinfo",
        lambda host, port: [(0, 0, 0, "", ("8.8.8.8", 0))],
    )

    def boom(*a, **kw):
        raise real_requests.RequestException("connect failed key=super-secret-key")

    monkeypatch.setattr("app.adapters.shodan_adapter.requests.get", boom)
    adapter = ShodanAdapter()
    with pytest.raises(RuntimeError) as exc_info:
        adapter.run_scan(
            ScanRequest(url="https://example.com", backend=Backend.SHODAN)
        )
    assert "super-secret-key" not in str(exc_info.value)
