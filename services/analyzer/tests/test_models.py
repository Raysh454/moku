"""Schema-level tests for the rewritten Pydantic models."""

import json
from datetime import UTC, datetime, timedelta

import pytest
from pydantic import ValidationError

from app.models.schemas import (
    Auth,
    Backend,
    Capabilities,
    Confidence,
    Finding,
    HealthResponse,
    HealthStatus,
    Progress,
    ScanRequest,
    ScanResult,
    ScanStatus,
    ScanSummary,
    Scope,
    Severity,
    SubmitResponse,
)


class TestScanRequestModel:
    def test_minimal_request_round_trips(self):
        req = ScanRequest(url="https://example.com", backend=Backend.BUILTIN)
        as_json = req.model_dump_json()
        round_tripped = ScanRequest.model_validate_json(as_json)
        assert round_tripped.backend == Backend.BUILTIN.value

    def test_rejects_unknown_field(self):
        with pytest.raises(ValidationError):
            ScanRequest(
                url="https://example.com", backend=Backend.BUILTIN, mystery="x"
            )

    def test_rejects_private_loopback(self):
        with pytest.raises(ValidationError):
            ScanRequest(url="http://127.0.0.1/", backend=Backend.BUILTIN)

    def test_rejects_file_scheme(self):
        with pytest.raises(ValidationError):
            ScanRequest(url="file:///etc/passwd", backend=Backend.BUILTIN)

    def test_max_duration_accepts_go_string(self):
        req = ScanRequest(
            url="https://example.com",
            backend=Backend.BUILTIN,
            max_duration="5m30s",
        )
        assert req.max_duration == timedelta(minutes=5, seconds=30)

    def test_max_duration_accepts_int_ns(self):
        req = ScanRequest(
            url="https://example.com",
            backend=Backend.BUILTIN,
            max_duration=1_000_000_000,
        )
        assert req.max_duration == timedelta(seconds=1)

    def test_scope_and_auth_round_trip(self):
        req = ScanRequest(
            url="https://example.com",
            backend=Backend.BUILTIN,
            scope=Scope(include_hosts=["example.com"]),
            auth=Auth(type="basic", username="u", password="p"),
        )
        dumped = req.model_dump(by_alias=True)
        assert dumped["scope"]["include_hosts"] == ["example.com"]
        assert dumped["auth"]["username"] == "u"


class TestCapabilitiesAlias:
    def test_async_serializes_as_async_key(self):
        caps = Capabilities(async_=True, supports_auth=True)
        as_json = caps.model_dump_json(by_alias=True)
        parsed = json.loads(as_json)
        assert parsed["async"] is True
        assert "async_" not in parsed

    def test_async_round_trips_via_alias(self):
        caps = Capabilities.model_validate({"async": True, "supports_auth": True})
        assert caps.async_ is True


class TestScanResultSerialization:
    def test_status_completed_emits_completed_string(self):
        result = ScanResult(
            job_id="abc",
            backend=Backend.BUILTIN,
            status=ScanStatus.COMPLETED,
            submitted_at=datetime(2026, 1, 1, tzinfo=UTC),
        )
        dumped = json.loads(result.model_dump_json(by_alias=True))
        assert dumped["status"] == "completed"

    def test_submitted_at_serializes_as_rfc3339_zulu(self):
        result = ScanResult(
            job_id="abc",
            backend=Backend.BUILTIN,
            status=ScanStatus.PENDING,
            submitted_at=datetime(2026, 1, 1, 12, 30, 45, 123000, tzinfo=UTC),
        )
        dumped = json.loads(result.model_dump_json(by_alias=True))
        assert dumped["submitted_at"].endswith("Z")


class TestFindingEnums:
    def test_severity_emits_lower_case_string(self):
        finding = Finding(
            id="f1",
            title="x",
            severity=Severity.CRITICAL,
            confidence=Confidence.CERTAIN,
        )
        dumped = json.loads(finding.model_dump_json(by_alias=True))
        assert dumped["severity"] == "critical"
        assert dumped["confidence"] == "certain"


class TestHealthShape:
    def test_health_response_serializes(self):
        resp = HealthResponse(
            status=HealthStatus.OK,
            adapters_available=["builtin"],
        )
        dumped = json.loads(resp.model_dump_json(by_alias=True))
        assert dumped["status"] == "ok"
        assert dumped["adapters_available"] == ["builtin"]


class TestSubmitResponse:
    def test_submit_response_carries_job_id(self):
        resp = SubmitResponse(job_id="deadbeef")
        assert resp.job_id == "deadbeef"


class TestProgressAndSummary:
    def test_progress_defaults(self):
        progress = Progress()
        assert progress.percent == 0.0
        assert progress.phase == ""

    def test_summary_defaults_to_zero(self):
        summary = ScanSummary()
        assert summary.total == 0
        assert summary.info == 0
