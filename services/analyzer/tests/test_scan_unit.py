"""Behavioural tests for `ScanUnit` and `TestCase` meta defaults."""

from app.core.scan_unit import ScanUnit, ScanUnitType
from app.core.test_case import TestCase, TestMode


def test_scan_unit_meta_default_is_isolated_per_instance():
    a = ScanUnit(type=ScanUnitType.URL, url="https://a.example")
    b = ScanUnit(type=ScanUnitType.URL, url="https://b.example")
    a.meta["job_id"] = "job-a"
    assert b.meta == {}


def test_test_case_meta_default_is_isolated_per_instance():
    a = TestCase(
        test_id="1",
        plugin_name="xss",
        injection_point="https://example.com",
        target_name="q",
        payload="p",
        mode=TestMode.DETECT,
    )
    b = TestCase(
        test_id="2",
        plugin_name="xss",
        injection_point="https://example.com",
        target_name="q",
        payload="p",
        mode=TestMode.DETECT,
    )
    a.meta["x"] = 1
    assert b.meta == {}
