"""Behavioural tests for the background scan runner."""

import time

import pytest

from app.adapters.base import BaseAdapter
from app.adapters.registry import AdapterRegistry
from app.core import runner as runner_module
from app.core.job_store import JobStore
from app.models.schemas import (
    Backend,
    Capabilities,
    Confidence,
    Finding,
    ScanRequest,
    ScanStatus,
    Severity,
)


class _StubAdapter(BaseAdapter):
    name = Backend.BUILTIN.value
    description = "stub"

    def __init__(
        self,
        findings: list[Finding] | None = None,
        raise_exc: Exception | None = None,
    ):
        self._findings = findings or []
        self._raise = raise_exc

    def capabilities(self) -> Capabilities:
        return Capabilities()

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        if self._raise:
            raise self._raise
        return list(self._findings)


def _wait(store: JobStore, job_id: str, target: set[ScanStatus]):
    for _ in range(200):
        result = store.get(job_id)
        if result and result.status in {s.value for s in target}:
            return result
        time.sleep(0.02)
    raise AssertionError("timed out waiting for status")


@pytest.fixture()
def isolated_runner(monkeypatch):
    fresh_store = JobStore()
    fresh_registry = AdapterRegistry()
    monkeypatch.setattr(runner_module, "job_store", fresh_store)
    monkeypatch.setattr(runner_module, "registry", fresh_registry)
    yield fresh_store, fresh_registry


class TestRedaction:
    def test_redacts_api_key_with_placeholder(self):
        # Positive assertion: value gone AND key name + placeholder preserved.
        redacted = runner_module._redact_error("oops api_key=abcdef tail")
        assert "abcdef" not in redacted
        assert "api_key=<redacted>" in redacted
        assert "tail" in redacted

    def test_redacts_token(self):
        redacted = runner_module._redact_error("token=xxx and key=yyy")
        assert "xxx" not in redacted
        assert "yyy" not in redacted

    def test_redacts_colon_separated_secret(self):
        # The regex must also catch "key: value" form, not only "key=value".
        redacted = runner_module._redact_error("auth failed password: hunter2")
        assert "hunter2" not in redacted
        assert "password=<redacted>" in redacted

    def test_redacts_bearer_token_after_scheme(self):
        redacted = runner_module._redact_error(
            "401 from Authorization: Bearer eyJhbGciOi.JIUzI1.secretpart tail"
        )
        assert "eyJhbGciOi.JIUzI1.secretpart" not in redacted
        assert "tail" in redacted

    def test_redacts_json_quoted_secret(self):
        redacted = runner_module._redact_error('{"api_key": "leakme123"}')
        assert "leakme123" not in redacted

    def test_redacts_url_embedded_credentials(self):
        redacted = runner_module._redact_error("dial https://user:s3cr3t@host/path failed")
        assert "s3cr3t" not in redacted

    def test_does_not_over_redact_key_suffixed_words(self):
        # "monkey"/"turnkey" must not trip the bare-"key" alternative.
        assert runner_module._redact_error("monkey=banana") == "monkey=banana"
        assert runner_module._redact_error("turnkey=ready") == "turnkey=ready"

    def test_leaves_unrelated_messages(self):
        assert runner_module._redact_error("plain error") == "plain error"


class TestRunJob:
    def test_failed_scan_records_static_prefix(self, isolated_runner):
        store, registry = isolated_runner
        registry.register(_StubAdapter(raise_exc=RuntimeError("secret api_key=abc")))

        request = ScanRequest(url="https://example.com", backend=Backend.BUILTIN)
        job_id = store.create(request)
        runner_module._run_job(job_id)
        result = store.get(job_id)
        assert result.status == ScanStatus.FAILED.value
        assert result.error.startswith("Scan failed: ")
        assert "abc" not in result.error
        assert result.completed_at is not None

    def test_successful_scan_sets_summary_and_findings(self, isolated_runner):
        store, registry = isolated_runner
        finding = Finding(
            id="f1",
            title="XSS",
            severity=Severity.HIGH,
            confidence=Confidence.FIRM,
            url="https://example.com",
        )
        registry.register(_StubAdapter(findings=[finding]))
        request = ScanRequest(url="https://example.com", backend=Backend.BUILTIN)
        job_id = store.create(request)
        runner_module._run_job(job_id)
        result = store.get(job_id)
        assert result.status == ScanStatus.COMPLETED.value
        assert len(result.findings) == 1
        assert result.summary.total == 1
        assert result.summary.high == 1

    def test_noop_when_job_vanishes_between_submit_and_run(self, isolated_runner):
        store, _ = isolated_runner
        runner_module._run_job("missing")


class TestDedupe:
    def test_keeps_highest_severity_duplicate(self):
        # Severity is the tiebreak: the lower-severity entry is dropped even
        # though it has higher confidence, pinning the canonical rule.
        high_sev = Finding(
            id="1",
            title="XSS",
            severity=Severity.HIGH,
            confidence=Confidence.TENTATIVE,
            url="https://example.com",
            parameter="q",
        )
        low_sev = Finding(
            id="2",
            title="XSS",
            severity=Severity.LOW,
            confidence=Confidence.CERTAIN,
            url="https://example.com",
            parameter="q",
        )
        deduped = runner_module._dedupe_findings([low_sev, high_sev])
        assert len(deduped) == 1
        assert deduped[0].id == "1"

    def test_equal_severity_tie_breaks_on_confidence(self):
        # Same (title,parameter,url) and severity but different confidence:
        # the higher-confidence finding must win deterministically.
        low_conf = Finding(
            id="1", title="XSS", severity=Severity.HIGH,
            confidence=Confidence.TENTATIVE, url="https://example.com", parameter="q",
        )
        high_conf = Finding(
            id="2", title="XSS", severity=Severity.HIGH,
            confidence=Confidence.CERTAIN, url="https://example.com", parameter="q",
        )
        deduped = runner_module._dedupe_findings([low_conf, high_conf])
        assert len(deduped) == 1
        assert deduped[0].id == "2"

    def test_keeps_distinct_findings(self):
        a = Finding(
            id="1", title="XSS", severity=Severity.LOW,
            confidence=Confidence.FIRM, url="https://example.com", parameter="q",
        )
        b = Finding(
            id="2", title="SQLI", severity=Severity.HIGH,
            confidence=Confidence.FIRM, url="https://example.com", parameter="q",
        )
        deduped = runner_module._dedupe_findings([a, b])
        assert len(deduped) == 2
