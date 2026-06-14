"""Test fixtures, isolation helpers, and the canonical `StubAdapter`."""

from __future__ import annotations

from unittest.mock import MagicMock

import pytest
from fastapi.testclient import TestClient

from app.adapters.base import BaseAdapter
from app.adapters.registry import AdapterRegistry
from app.app_factory import create_app
from app.models.schemas import (
    Backend,
    Capabilities,
    Confidence,
    Finding,
    ScanRequest,
    Severity,
)


class StubAdapter(BaseAdapter):
    """Deterministic in-process adapter used in tests; never touches the network."""

    name = Backend.BUILTIN.value
    description = "Stub adapter for tests"

    def __init__(self, findings: list[Finding] | None = None) -> None:
        self._findings = findings if findings is not None else [_default_finding()]

    def capabilities(self) -> Capabilities:
        return Capabilities(
            async_=False,
            supports_auth=True,
            supports_scope=False,
            supports_scan_profile=False,
            max_concurrent_scans=1,
            version="test",
        )

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        return list(self._findings)


def _default_finding() -> Finding:
    return Finding(
        id="stub-1",
        title="STUB",
        severity=Severity.LOW,
        confidence=Confidence.FIRM,
        url="https://example.com",
        description="stub finding",
    )


def _register_stub(registry: AdapterRegistry) -> None:
    registry.register(StubAdapter())


@pytest.fixture()
def client() -> TestClient:
    app = create_app(register=_register_stub)
    with TestClient(app) as test_client:
        yield test_client


@pytest.fixture()
def mock_subprocess(monkeypatch):
    def _patch(returncode: int = 0, stdout: str = "", stderr: str = ""):
        mock = MagicMock()
        completed = MagicMock()
        completed.returncode = returncode
        completed.stdout = stdout
        completed.stderr = stderr
        mock.return_value = completed
        monkeypatch.setattr("subprocess.run", mock)
        return mock

    return _patch


@pytest.fixture()
def mock_requests(monkeypatch):
    def _patch(get=None, post=None):
        if get is not None:
            monkeypatch.setattr("requests.get", get)
        if post is not None:
            monkeypatch.setattr("requests.post", post)

    return _patch


@pytest.fixture(autouse=True)
def _isolate_evidence_dir(tmp_path, monkeypatch):
    target = tmp_path / "evidence"
    monkeypatch.setenv("MOKU_EVIDENCE_DIR", str(target))
    from app.core import evidence_store as evidence_store_module

    evidence_store_module._reset_evidence_store_for_tests()
    yield
    evidence_store_module._reset_evidence_store_for_tests()


@pytest.fixture(autouse=True)
def _stub_dns(monkeypatch):
    """Resolve hostnames to a fixed public IP so the suite never hits real DNS.

    ScanRequest validation (and the SSRF guard) resolve any non-IP hostname;
    without this, ~dozens of tests constructing ScanRequest(url="https://...")
    would do real network DNS, violating F.I.R.S.T. and flaking in CI. Tests
    that need specific resolution (test_net_guard, adapter SSRF tests) override
    this with their own monkeypatch, which takes precedence.
    """
    monkeypatch.setattr(
        "app.net_guard.socket.getaddrinfo",
        lambda *a, **kw: [(0, 0, 0, "", ("93.184.216.34", 0))],
    )


@pytest.fixture(autouse=True)
def _disallow_private_hosts_for_tests(monkeypatch):
    monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "false")
    monkeypatch.setenv("MOKU_ALLOW_PRIVATE_HOSTS", "false")
