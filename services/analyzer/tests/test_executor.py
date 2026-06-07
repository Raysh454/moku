"""Tests for the per-scan `Executor`."""

import threading
from datetime import timedelta
from unittest.mock import MagicMock

import pytest
import requests

from app.core import executor as executor_module
from app.core.executor import Executor
from app.core.finding import Finding
from app.core.scan_unit import ScanUnit, ScanUnitType
from app.core.test_case import TestCase, TestMode
from app.plugins.base_plugin import BasePlugin


class _MatchPlugin(BasePlugin):
    """Plugin that always emits one finding for the 'xss' test cases."""

    name = "xss"

    def generate_tests(self, scan_unit):  # pragma: no cover
        return []

    def analyze_response(self, **kwargs) -> Finding | None:
        return Finding(
            finding_id="x-1",
            plugin="xss",
            scan_unit_url="https://example.com",
            http_method="GET",
            payload_used="p",
            matched_pattern="m",
            response_snippet="s",
            confidence=0.9,
        )


def _test_case(test_id: str = "t") -> TestCase:
    return TestCase(
        test_id=test_id,
        plugin_name="xss",
        injection_point="https://example.com",
        target_name="q",
        payload="p",
        mode=TestMode.DETECT,
    )


def _scan_unit() -> ScanUnit:
    return ScanUnit(type=ScanUnitType.URL, url="https://example.com", params={"q": "x"})


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
        "request",
        MagicMock(side_effect=requests.RequestException("boom")),
    )
    scan_unit = ScanUnit(type=ScanUnitType.URL, url="https://example.com")
    assert executor._fetch_baseline(scan_unit) is None


def _redirect_response(location: str, status: int = 302):
    resp = MagicMock()
    resp.status_code = status
    resp.headers = {"Location": location}
    return resp


def _ok_response(text: str = "final"):
    resp = MagicMock()
    resp.status_code = 200
    resp.headers = {}
    resp.text = text
    return resp


def test_guarded_request_blocks_redirect_to_internal_host():
    executor = Executor()
    executor._session.request = MagicMock(
        return_value=_redirect_response("http://169.254.169.254/latest/meta-data/")
    )
    with pytest.raises(requests.RequestException):
        executor._guarded_request("GET", "https://example.com", timeout=5)


def test_guarded_request_follows_redirect_to_public_host(monkeypatch):
    monkeypatch.setattr(
        "app.net_guard.socket.getaddrinfo",
        lambda *a, **kw: [(0, 0, 0, "", ("8.8.8.8", 0))],
    )
    executor = Executor()
    executor._session.request = MagicMock(
        side_effect=[
            _redirect_response("https://other.example.org/next"),
            _ok_response("final-body"),
        ]
    )
    resp = executor._guarded_request("GET", "https://example.com", timeout=5)
    assert resp.text == "final-body"


def test_guarded_request_raises_on_redirect_loop(monkeypatch):
    monkeypatch.setattr(
        "app.net_guard.socket.getaddrinfo",
        lambda *a, **kw: [(0, 0, 0, "", ("8.8.8.8", 0))],
    )
    executor = Executor()
    executor._session.request = MagicMock(
        return_value=_redirect_response("https://other.example.org/loop")
    )
    with pytest.raises(requests.RequestException):
        executor._guarded_request("GET", "https://example.com", timeout=5)


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


class TestRun:
    def test_dispatches_to_matching_plugin(self, monkeypatch):
        monkeypatch.setattr(executor_module, "REQUEST_DELAY_SECONDS", 0)
        executor = Executor()
        executor._session.request = MagicMock(return_value=_ok_response())
        findings = executor.run(_scan_unit(), [_test_case()], [_MatchPlugin()])
        assert len(findings) == 1
        assert findings[0].plugin == "xss"

    def test_saves_evidence_per_test_case(self, monkeypatch, tmp_path):
        monkeypatch.setattr(executor_module, "REQUEST_DELAY_SECONDS", 0)
        saved = []
        fake_store = MagicMock()
        fake_store.save.side_effect = lambda **kw: saved.append(kw)
        monkeypatch.setattr(
            executor_module, "get_evidence_store", lambda: fake_store
        )
        executor = Executor()
        executor._session.request = MagicMock(return_value=_ok_response())
        executor.run(_scan_unit(), [_test_case()], [_MatchPlugin()])
        assert len(saved) == 1

    def test_stops_at_rate_limit(self, monkeypatch):
        monkeypatch.setattr(executor_module, "REQUEST_DELAY_SECONDS", 0)
        monkeypatch.setattr(executor_module, "MAX_REQUESTS_PER_HOST", 1)
        executor = Executor()
        executor._session.request = MagicMock(return_value=_ok_response())
        tcs = [_test_case(f"t{i}") for i in range(3)]
        executor.run(_scan_unit(), tcs, [_MatchPlugin()])
        # baseline (1) + exactly 1 payload before the rate-limit break.
        assert executor._session.request.call_count == 2

    def test_honors_max_duration_deadline(self, monkeypatch):
        monkeypatch.setattr(executor_module, "REQUEST_DELAY_SECONDS", 0)
        executor = Executor()
        executor._session.request = MagicMock(return_value=_ok_response())
        tcs = [_test_case(f"t{i}") for i in range(20)]
        findings = executor.run(
            _scan_unit(), tcs, [_MatchPlugin()], max_duration=timedelta(microseconds=1)
        )
        # A 1µs budget must cut the loop short well before all 20 cases run.
        assert len(findings) < len(tcs)

    def test_baseline_unavailable_is_flagged(self, monkeypatch):
        monkeypatch.setattr(executor_module, "REQUEST_DELAY_SECONDS", 0)
        executor = Executor()
        # First call (baseline) fails; subsequent (payload) succeed.
        executor._session.request = MagicMock(
            side_effect=[
                requests.RequestException("baseline down"),
                _ok_response(),
            ]
        )
        findings = executor.run(_scan_unit(), [_test_case()], [_MatchPlugin()])
        assert len(findings) == 1
        assert findings[0].meta.get("baseline_unavailable") is True
