"""Tests for the BuiltinAdapter wiring and result mapping."""

from unittest.mock import MagicMock, patch

import pytest

from app.adapters.builtin_adapter import BuiltinAdapter
from app.core.finding import EvidenceRef
from app.core.finding import Finding as InternalFinding
from app.core.test_case import TestCase, TestMode
from app.models.schemas import Backend, Confidence, ScanRequest, Severity


def _stub_plugin_manager(test_cases):
    pm = MagicMock()
    pm.generate_tests.return_value = test_cases
    pm.get_plugins.return_value = []
    return pm


def _stub_test_case() -> TestCase:
    return TestCase(
        test_id="t1",
        plugin_name="xss",
        injection_point="https://example.com",
        target_name="q",
        payload="p",
        mode=TestMode.DETECT,
    )


@pytest.fixture()
def request_for(monkeypatch):
    def _make(url: str = "https://example.com") -> ScanRequest:
        return ScanRequest(url=url, backend=Backend.BUILTIN)

    return _make


def _internal_finding(plugin: str = "xss", confidence: float = 0.95) -> InternalFinding:
    return InternalFinding(
        finding_id=f"{plugin}-1",
        plugin=plugin,
        scan_unit_url="https://example.com/path",
        http_method="GET",
        payload_used="payload",
        matched_pattern="match",
        response_snippet="snippet",
        confidence=confidence,
        repro_steps=["1", "2"],
        evidence_refs=[
            EvidenceRef(sha256="abc", size=3, path="/tmp/abc", label="resp")
        ],
        meta={"parameter": "q", "mode": "query"},
    )


class TestSSRFGuard:
    def test_rejects_private_target(self, request_for):
        adapter = BuiltinAdapter()
        with pytest.raises(ValueError):
            adapter.run_scan(
                ScanRequest.model_construct(url="http://127.0.0.1/", backend=Backend.BUILTIN)
            )


class TestMapping:
    def test_title_is_plugin_name_uppercased(self, request_for):
        stub = _stub_plugin_manager([_stub_test_case()])
        adapter = BuiltinAdapter(plugin_manager_factory=lambda: stub)
        with patch("app.adapters.builtin_adapter.Executor") as exec_cls:
            exec_cls.return_value.run.return_value = [_internal_finding("sqli", 0.95)]
            result = adapter.run_scan(request_for())
        assert len(result) == 1
        assert result[0].title == "SQLI"
        assert result[0].severity == Severity.CRITICAL.value
        assert result[0].confidence == Confidence.CERTAIN.value

    def test_returns_mapped_findings_without_local_dedup(self, request_for):
        # Dedup is the runner's responsibility now; the adapter maps 1:1.
        stub = _stub_plugin_manager([_stub_test_case()])
        adapter = BuiltinAdapter(plugin_manager_factory=lambda: stub)
        with patch("app.adapters.builtin_adapter.Executor") as exec_cls:
            exec_cls.return_value.run.return_value = [
                _internal_finding("xss", 0.4),
                _internal_finding("xss", 0.95),
            ]
            result = adapter.run_scan(request_for())
        assert len(result) == 2
