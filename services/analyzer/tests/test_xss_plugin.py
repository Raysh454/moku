"""Tests for the XSS plugin."""

from app.core.scan_unit import ScanUnit, ScanUnitType
from app.plugins.xss_plugin import XSSPlugin


def _scan_unit() -> ScanUnit:
    return ScanUnit(
        type=ScanUnitType.URL,
        url="https://example.com/path",
        params={"q": ""},
        method="GET",
    )


def test_marker_is_high_entropy_and_well_prefixed():
    plugin = XSSPlugin()
    tests = plugin.generate_tests(_scan_unit())
    assert tests
    marker = tests[0].marker
    assert marker.startswith("__moku_xss_")
    assert len(marker) >= len("__moku_xss_") + 32


def test_injection_point_is_full_url():
    plugin = XSSPlugin()
    tests = plugin.generate_tests(_scan_unit())
    assert tests[0].injection_point == "https://example.com/path"


def test_meta_carries_mode_and_parameter():
    plugin = XSSPlugin()
    tests = plugin.generate_tests(_scan_unit())
    assert tests[0].meta == {"parameter": "q", "mode": "query"}


def test_timestamp_is_timezone_aware():
    plugin = XSSPlugin()
    tests = plugin.generate_tests(_scan_unit())
    detect = tests[0]
    body = f"<html>here is <{detect.marker}> reflected"
    finding = plugin.analyze_response(
        test_case=detect,
        response_body=body,
        response_headers={},
        baseline_body="",
    )
    assert finding is not None
    assert finding.timestamp is not None
    assert finding.timestamp.tzinfo is not None
