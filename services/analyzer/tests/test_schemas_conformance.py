"""Wire-format conformance tests for the schemas matching the Moku Go contract."""

import json
from datetime import UTC, datetime, timedelta

import pytest
from pydantic import ValidationError

from app.models.schemas import (
    Backend,
    Capabilities,
    Confidence,
    Finding,
    ScanRequest,
    ScanResult,
    ScanStatus,
    Severity,
)


def test_scan_request_round_trip_snake_case():
    payload = {
        "url": "https://example.com",
        "backend": "builtin",
        "raw_options": {"depth": "3"},
    }
    req = ScanRequest.model_validate(payload)
    dumped = req.model_dump(by_alias=True)
    assert dumped["raw_options"] == {"depth": "3"}


def test_scan_request_rejects_unknown_field():
    with pytest.raises(ValidationError):
        ScanRequest.model_validate(
            {"url": "https://example.com", "backend": "builtin", "extra": True}
        )


def test_scan_request_rejects_private_ip():
    with pytest.raises(ValidationError):
        ScanRequest.model_validate({"url": "http://10.0.0.1/", "backend": "builtin"})


def test_scan_request_max_duration_accepts_go_string_and_int_ns():
    via_string = ScanRequest.model_validate(
        {"url": "https://example.com", "backend": "builtin", "max_duration": "1h"}
    )
    assert via_string.max_duration == timedelta(hours=1)

    via_int = ScanRequest.model_validate(
        {
            "url": "https://example.com",
            "backend": "builtin",
            "max_duration": 5_000_000_000,
        }
    )
    assert via_int.max_duration == timedelta(seconds=5)


def test_capabilities_serializes_async_key():
    caps = Capabilities(async_=True)
    dumped = json.loads(caps.model_dump_json(by_alias=True))
    assert "async" in dumped
    assert "async_" not in dumped


def test_scan_result_serializes_status_completed_not_done():
    result = ScanResult(
        job_id="x",
        backend=Backend.BUILTIN,
        status=ScanStatus.COMPLETED,
        submitted_at=datetime(2026, 1, 1, tzinfo=UTC),
    )
    dumped = json.loads(result.model_dump_json(by_alias=True))
    assert dumped["status"] == "completed"


def test_finding_severity_and_confidence_enums_emit_lower_case_strings():
    finding = Finding(
        id="f1",
        title="x",
        severity=Severity.HIGH,
        confidence=Confidence.FIRM,
    )
    dumped = json.loads(finding.model_dump_json(by_alias=True))
    assert dumped["severity"] == "high"
    assert dumped["confidence"] == "firm"


def test_reject_private_host_blocks_loopback_by_default(monkeypatch):
    monkeypatch.delenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", raising=False)
    with pytest.raises(ValidationError):
        ScanRequest.model_validate(
            {"url": "http://127.0.0.1:9999/", "backend": "builtin"}
        )


def test_reject_private_host_allows_loopback_when_env_flag_set(monkeypatch):
    monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
    req = ScanRequest.model_validate(
        {"url": "http://127.0.0.1:9999/", "backend": "builtin"}
    )
    assert str(req.url).startswith("http://127.0.0.1:9999")


def test_reject_private_host_still_blocks_javascript_scheme(monkeypatch):
    monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
    with pytest.raises(ValidationError):
        ScanRequest.model_validate(
            {"url": "javascript:alert(1)", "backend": "builtin"}
        )
