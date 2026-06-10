"""Tests for the Nikto adapter."""

from unittest.mock import MagicMock

import pytest

from app.adapters.nikto_adapter import NiktoAdapter
from app.models.schemas import Backend, Confidence, ScanRequest, Severity


def _completed(stdout: str, returncode: int = 0):
    completed = MagicMock()
    completed.returncode = returncode
    completed.stdout = stdout
    completed.stderr = ""
    return completed


def test_parses_plus_prefixed_lines(monkeypatch):
    stdout = "\n".join(
        [
            "- Nikto v2.5",
            "+ Server: nginx",
            "+ Cookie session created without the secure flag",
            "  banner garbage",
        ]
    )
    monkeypatch.setattr(
        "app.adapters._helpers.subprocess.run",
        lambda *a, **kw: _completed(stdout),
    )
    adapter = NiktoAdapter()
    findings = adapter.run_scan(
        ScanRequest(url="https://example.com", backend=Backend.NIKTO)
    )
    assert len(findings) == 2
    assert findings[0].severity == Severity.INFO.value
    assert findings[0].confidence == Confidence.TENTATIVE.value


def test_rejects_private_target():
    adapter = NiktoAdapter()
    with pytest.raises(ValueError):
        adapter.run_scan(
            ScanRequest.model_construct(url="http://127.0.0.1/", backend=Backend.NIKTO)
        )
