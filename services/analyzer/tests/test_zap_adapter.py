"""Tests for the ZAP adapter."""

import json
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from app.adapters.zap_adapter import ZAPAdapter
from app.models.schemas import Backend, ScanRequest, Severity


def _build_run(report: dict, target_file_index: int = -1):
    def fake_run(cmd, **kwargs):
        output_path = Path(cmd[target_file_index])
        output_path.write_text(json.dumps(report), encoding="utf-8")
        completed = MagicMock()
        completed.returncode = 0
        completed.stdout = ""
        completed.stderr = ""
        return completed

    return fake_run


def test_writes_inside_temp_dir_and_parses(monkeypatch):
    report = {
        "site": [
            {
                "alerts": [
                    {
                        "alert": "Cross Site Scripting",
                        "risk": "high",
                        "evidence": "<script>",
                        "solution": "escape output",
                        "param": "q",
                    }
                ]
            }
        ]
    }
    monkeypatch.setattr(
        "app.adapters._helpers.subprocess.run", _build_run(report)
    )
    adapter = ZAPAdapter()
    findings = adapter.run_scan(
        ScanRequest(url="https://example.com", backend=Backend.ZAP)
    )
    assert len(findings) == 1
    assert findings[0].severity == Severity.HIGH.value


def test_rejects_private_target():
    adapter = ZAPAdapter()
    with pytest.raises(ValueError):
        adapter.run_scan(
            ScanRequest.model_construct(url="http://127.0.0.1/", backend=Backend.ZAP)
        )
