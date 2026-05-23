"""Tests for the per-scan `Executor`."""

import threading
from unittest.mock import MagicMock

import requests

from app.core.executor import Executor
from app.core.finding import Finding
from app.core.scan_unit import ScanUnit, ScanUnitType
from app.core.test_case import TestCase, TestMode
from app.plugins.base_plugin import BasePlugin


class _Plugin(BasePlugin):
    name = "stub"

    def generate_tests(self, scan_unit):  # pragma: no cover
        return []

    def analyze_response(self, **kwargs) -> Finding | None:
        return None


def test_request_counter_is_thread_safe():
    executor = Executor()

    def bump():
        for _ in range(100):
            with executor._counter_lock:
                executor._request_counts["host"] = (
                    executor._request_counts.get("host", 0) + 1
                )

    threads = [threading.Thread(target=bump) for _ in range(5)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    assert executor._request_counts["host"] == 500


def test_fetch_baseline_returns_none_on_failure(monkeypatch):
    executor = Executor()
    monkeypatch.setattr(
        executor._session,
        "get",
        MagicMock(side_effect=requests.RequestException("boom")),
    )
    scan_unit = ScanUnit(type=ScanUnitType.URL, url="https://example.com")
    assert executor._fetch_baseline(scan_unit) is None


def test_truncation_marker_added_on_large_body():
    executor = Executor()
    test_case = TestCase(
        test_id="t",
        plugin_name="x",
        injection_point="https://example.com",
        target_name="q",
        payload="p",
        mode=TestMode.DETECT,
    )
    body = "A" * 10_000
    payload = executor._build_evidence_payload(test_case, body)
    assert "TRUNCATED 10000" in payload


def test_user_agent_honors_env(monkeypatch):
    monkeypatch.setenv("MOKU_ANALYZER_UA", "custom-agent/9")
    executor = Executor()
    assert executor._session.headers["User-Agent"] == "custom-agent/9"


def test_apply_cookies_no_op_when_empty():
    executor = Executor()
    executor._apply_cookies(None)
    executor._apply_cookies({})
