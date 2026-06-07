"""Background scan execution — bounded thread pool with redacted error reporting."""

import logging
import os
import re
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime

from app.adapters.registry import registry
from app.core.job_store import job_store
from app.models.schemas import (
    Backend,
    Progress,
    ScanStatus,
    ScanSummary,
    Severity,
)
from app.models.schemas import (
    Finding as ApiFinding,
)

_logger = logging.getLogger(__name__)

_MAX_WORKERS = int(os.getenv("MOKU_ANALYZER_WORKERS", "4"))
_executor = ThreadPoolExecutor(max_workers=_MAX_WORKERS, thread_name_prefix="moku-scan")

_SECRET_KV_PATTERN = re.compile(
    r"(?i)(api[_-]?key|token|password|authorization|key)\s*[:=]\s*([^&\s\"']+)"
)


def _redact_error(message: str) -> str:
    """Strip recognisable secrets out of error messages before logging/exposing.

    Matches ``key=value`` and ``key: value`` (with optional surrounding
    whitespace), normalising the redacted form to ``key=<redacted>`` so the
    key name survives for diagnostics while the value never leaks.
    """
    return _SECRET_KV_PATTERN.sub(r"\1=<redacted>", message)


def submit_scan_job(job_id: str) -> None:
    """Schedule `_run_job` on the bounded thread pool."""
    _executor.submit(_run_job, job_id)


def _run_job(job_id: str) -> None:
    request = job_store.get_request(job_id)
    if request is None:
        _logger.info("scan %s has vanished before execution", job_id)
        return

    job_store.update_status(job_id, status=ScanStatus.RUNNING)

    # Pin the evidence partition to the trusted, server-generated job id.
    # Adapters read raw_options["job_id"] for evidence storage; overwriting it
    # here ensures a caller-supplied value can never steer filesystem paths.
    request.raw_options["job_id"] = job_id

    try:
        backend_name = (
            request.backend.value
            if isinstance(request.backend, Backend)
            else str(request.backend)
        )
        adapter = registry.get(backend_name)
        findings: list[ApiFinding] = adapter.run_scan(request)
        deduped = _dedupe_findings(findings)
        summary = _build_summary(deduped)
        job_store.update_status(
            job_id,
            status=ScanStatus.COMPLETED,
            findings=deduped,
            summary=summary,
            completed_at=datetime.now(UTC),
            progress=Progress(percent=100, phase="completed", note=""),
        )
    except (RuntimeError, ValueError, OSError) as exc:
        _logger.exception("scan %s failed", job_id)
        job_store.update_status(
            job_id,
            status=ScanStatus.FAILED,
            error="Scan failed: " + _redact_error(str(exc)),
            completed_at=datetime.now(UTC),
        )
    except Exception as exc:
        _logger.exception("scan %s crashed with unexpected exception", job_id)
        job_store.update_status(
            job_id,
            status=ScanStatus.FAILED,
            error="Scan failed: " + _redact_error(repr(exc)),
            completed_at=datetime.now(UTC),
        )


_SEVERITY_RANK: dict[Severity, int] = {
    Severity.INFO: 0,
    Severity.LOW: 1,
    Severity.MEDIUM: 2,
    Severity.HIGH: 3,
    Severity.CRITICAL: 4,
}


def _dedupe_findings(findings: list[ApiFinding]) -> list[ApiFinding]:
    """Collapse findings sharing (title, parameter, url), keeping the most
    severe. This is the single dedup policy for every adapter — applied once,
    here, so adapters never carry their own (potentially divergent) copy."""
    best: dict[tuple[str, str | None, str | None], ApiFinding] = {}
    for finding in findings:
        key = (finding.title, finding.parameter, finding.url)
        current = best.get(key)
        if current is None or _severity_rank(finding) > _severity_rank(current):
            best[key] = finding
    return list(best.values())


def _severity_rank(finding: ApiFinding) -> int:
    return _SEVERITY_RANK.get(Severity(finding.severity), 0)


def _build_summary(findings: list[ApiFinding]) -> ScanSummary:
    counts: dict[Severity, int] = {sev: 0 for sev in Severity}
    for finding in findings:
        sev = Severity(finding.severity)
        counts[sev] += 1
    return ScanSummary(
        total=len(findings),
        info=counts[Severity.INFO],
        low=counts[Severity.LOW],
        medium=counts[Severity.MEDIUM],
        high=counts[Severity.HIGH],
        critical=counts[Severity.CRITICAL],
    )
