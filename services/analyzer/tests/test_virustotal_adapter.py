"""Tests for the VirusTotal adapter."""

from unittest.mock import MagicMock

import pytest

from app.adapters.virustotal_adapter import VirusTotalAdapter
from app.models.schemas import Backend, ScanRequest


def _response(status_code: int = 200, body: dict | None = None):
    resp = MagicMock()
    resp.status_code = status_code
    resp.json.return_value = body or {}
    resp.text = ""
    return resp


def test_refuses_without_consent(monkeypatch):
    monkeypatch.setenv("VIRUSTOTAL_API_KEY", "k")
    adapter = VirusTotalAdapter()
    with pytest.raises(RuntimeError) as exc_info:
        adapter.run_scan(
            ScanRequest(url="https://example.com", backend=Backend.VIRUSTOTAL)
        )
    assert "consent" in str(exc_info.value)


def test_maps_completed_report(monkeypatch):
    monkeypatch.setenv("VIRUSTOTAL_API_KEY", "k")
    monkeypatch.setattr(
        "app.adapters.virustotal_adapter.requests.post",
        lambda *a, **kw: _response(200, {"data": {"id": "analysis-1"}}),
    )
    monkeypatch.setattr(
        "app.adapters.virustotal_adapter.requests.get",
        lambda *a, **kw: _response(
            200,
            {
                "data": {
                    "attributes": {
                        "status": "completed",
                        "results": {
                            "vendor-a": {"category": "malicious"},
                            "vendor-b": {"category": "harmless"},
                        },
                    }
                }
            },
        ),
    )
    adapter = VirusTotalAdapter()
    findings = adapter.run_scan(
        ScanRequest(
            url="https://example.com",
            backend=Backend.VIRUSTOTAL,
            raw_options={"virustotal_consent": "true"},
        )
    )
    titles = [f.title for f in findings]
    assert "malicious-url" in titles
    assert "virustotal-summary" in titles


def test_transport_error_does_not_leak_api_key(monkeypatch):
    import requests as real_requests

    monkeypatch.setenv("VIRUSTOTAL_API_KEY", "super-secret")

    def boom(*a, **kw):
        raise real_requests.RequestException("failed key=super-secret")

    monkeypatch.setattr("app.adapters.virustotal_adapter.requests.post", boom)
    adapter = VirusTotalAdapter()
    with pytest.raises(RuntimeError) as exc_info:
        adapter.run_scan(
            ScanRequest(
                url="https://example.com",
                backend=Backend.VIRUSTOTAL,
                raw_options={"virustotal_consent": "true"},
            )
        )
    assert "super-secret" not in str(exc_info.value)
