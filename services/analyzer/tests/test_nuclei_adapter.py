"""Tests for the Nuclei adapter."""

import json
from unittest.mock import MagicMock

import pytest

from app.adapters.nuclei_adapter import NucleiAdapter
from app.models.schemas import Backend, ScanRequest, Severity


def _completed(stdout: str, returncode: int = 0):
    completed = MagicMock()
    completed.returncode = returncode
    completed.stdout = stdout
    completed.stderr = ""
    return completed


def test_jsonl_parsing(monkeypatch):
    record = {
        "template-id": "apache-mod-status",
        "info": {
            "name": "Apache Mod Status",
            "severity": "medium",
            "description": "exposed",
            "classification": {"cwe-id": ["CWE-200"]},
        },
        "matched-at": "https://example.com/server-status",
    }
    stdout = json.dumps(record) + "\n"
    monkeypatch.setattr(
        "app.adapters._helpers.subprocess.run",
        lambda *a, **kw: _completed(stdout),
    )
    adapter = NucleiAdapter()
    findings = adapter.run_scan(
        ScanRequest(url="https://example.com", backend=Backend.NUCLEI)
    )
    assert len(findings) == 1
    assert findings[0].severity == Severity.MEDIUM.value
    assert findings[0].cwe == [200]


def test_rejects_private_target():
    adapter = NucleiAdapter()
    with pytest.raises(ValueError):
        adapter.run_scan(
            ScanRequest.model_construct(url="http://127.0.0.1/", backend=Backend.NUCLEI)
        )


def test_subprocess_failure_raises_runtime_error(monkeypatch):
    monkeypatch.setattr(
        "app.adapters._helpers.subprocess.run",
        lambda *a, **kw: _completed("", returncode=2),
    )
    adapter = NucleiAdapter()
    with pytest.raises(RuntimeError):
        adapter.run_scan(
            ScanRequest(url="https://example.com", backend=Backend.NUCLEI)
        )
