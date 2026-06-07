"""Tests for `Finding` model defaults and the confidence-to-severity helper."""

from datetime import UTC, datetime

from app.core.finding import Finding, confidence_to_severity
from app.models.schemas import Severity


def test_timestamp_is_optional():
    finding = Finding(
        finding_id="x",
        plugin="xss",
        scan_unit_url="https://example.com",
        http_method="GET",
        payload_used="p",
        matched_pattern="m",
        response_snippet="r",
        confidence=0.5,
    )
    assert finding.timestamp is None


def test_timestamp_accepts_aware_datetime():
    when = datetime.now(UTC)
    finding = Finding(
        finding_id="x",
        plugin="xss",
        scan_unit_url="https://example.com",
        http_method="GET",
        payload_used="p",
        matched_pattern="m",
        response_snippet="r",
        confidence=0.5,
        timestamp=when,
    )
    assert finding.timestamp == when


class TestConfidenceToSeverity:
    def test_critical_at_0_9(self):
        assert confidence_to_severity(0.95) == Severity.CRITICAL

    def test_high_at_0_7(self):
        assert confidence_to_severity(0.75) == Severity.HIGH

    def test_medium_at_0_5(self):
        assert confidence_to_severity(0.55) == Severity.MEDIUM

    def test_low_at_0_3(self):
        assert confidence_to_severity(0.35) == Severity.LOW

    def test_info_below_0_3(self):
        assert confidence_to_severity(0.1) == Severity.INFO
