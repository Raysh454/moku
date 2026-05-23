"""Tests for the SQLi plugin."""

from app.core.scan_unit import ScanUnit, ScanUnitType
from app.core.test_case import TestMode
from app.plugins.sqli_plugin import SQLiPlugin


def _scan_unit() -> ScanUnit:
    return ScanUnit(
        type=ScanUnitType.URL,
        url="https://example.com/items",
        params={"id": "1"},
    )


def test_injection_point_is_full_url():
    plugin = SQLiPlugin()
    tests = plugin.generate_tests(_scan_unit())
    assert all(t.injection_point == "https://example.com/items" for t in tests)


def test_timestamp_is_timezone_aware():
    plugin = SQLiPlugin()
    tests = plugin.generate_tests(_scan_unit())
    detect_test = next(t for t in tests if t.mode == TestMode.DETECT)
    body = "You have an error in your SQL syntax"
    finding = plugin.analyze_response(
        test_case=detect_test,
        response_body=body,
        response_headers={},
        baseline_body="non-empty baseline",
    )
    assert finding is not None
    assert finding.timestamp.tzinfo is not None


def test_finding_carries_baseline_unavailable_when_baseline_empty():
    plugin = SQLiPlugin()
    tests = plugin.generate_tests(_scan_unit())
    detect_test = next(t for t in tests if t.mode == TestMode.DETECT)
    body = "ORA-12345: bad column"
    finding = plugin.analyze_response(
        test_case=detect_test,
        response_body=body,
        response_headers={},
        baseline_body="",
    )
    assert finding is not None
    assert finding.meta.get("warning") == "baseline_unavailable"
