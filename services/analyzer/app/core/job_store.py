"""In-memory store for in-flight and recently-completed scan jobs.

Replaces the old SQLite + sequential-ID layer. UUIDv4 ids, TTL pruning,
soft cap on resident jobs, and the originating `ScanRequest` is kept so
the runner can resubmit without re-passing it.
"""

import asyncio
import logging
import os
import threading
import uuid
from collections.abc import Callable
from datetime import UTC, datetime, timedelta

from app.models.schemas import (
    Progress,
    ScanRequest,
    ScanResult,
    ScanStatus,
    ScanSummary,
)

_logger = logging.getLogger(__name__)

_DEFAULT_MAX_JOBS = 1024
MAX_JOBS = int(os.getenv("MOKU_ANALYZER_MAX_JOBS", str(_DEFAULT_MAX_JOBS)))

# Optional hard ceiling on total on-disk evidence (bytes). 0 = no size cap
# (retention is then bounded only by the TTL). The background pruner enforces
# it via EvidenceStore.prune, protecting active jobs' evidence from deletion.
MAX_EVIDENCE_BYTES = int(os.getenv("MOKU_EVIDENCE_MAX_BYTES", "0"))

_TERMINAL_STATUS_VALUES = frozenset(
    {ScanStatus.COMPLETED.value, ScanStatus.FAILED.value, ScanStatus.CANCELED.value}
)


def _is_terminal(status: ScanStatus | str) -> bool:
    """True for finished scans (completed/failed/canceled).

    Normalises both the enum (set via update_status) and the plain string
    (produced by Pydantic's use_enum_values on construction) representations.
    """
    value = status.value if isinstance(status, ScanStatus) else str(status)
    return value in _TERMINAL_STATUS_VALUES


def _age_reference(result: ScanResult) -> datetime:
    """Always-aware timestamp used to age a job: completed_at, else submitted_at.

    Normalises naive datetimes to UTC so eviction/purge never mix offset-naive
    and offset-aware values (which would raise TypeError).
    """
    reference = result.completed_at or result.submitted_at
    if reference.tzinfo is None:
        reference = reference.replace(tzinfo=UTC)
    return reference


class JobStoreFull(Exception):
    """Raised when a new job cannot be admitted because the store is full of
    still-active (pending/running) scans."""


class JobStore:
    """Thread-safe in-memory store keyed by job id."""

    def __init__(self, now_factory: Callable[[], datetime] | None = None) -> None:
        self._jobs: dict[str, ScanResult] = {}
        self._requests: dict[str, ScanRequest] = {}
        self._lock = threading.Lock()
        self._now = now_factory or (lambda: datetime.now(UTC))

    def create(self, request: ScanRequest) -> str:
        job_id = uuid.uuid4().hex
        result = ScanResult(
            job_id=job_id,
            backend=request.backend,
            status=ScanStatus.PENDING,
            url=str(request.url),
            submitted_at=self._now(),
            progress=Progress(),
            findings=[],
            summary=None,
            raw_data={},
        )
        evicted: str | None = None
        with self._lock:
            if len(self._jobs) >= MAX_JOBS:
                evicted = self._evict_oldest_terminal_locked()
                if evicted is None:
                    raise JobStoreFull(
                        f"job store is full ({MAX_JOBS} active scans); retry later"
                    )
            self._jobs[job_id] = result
            self._requests[job_id] = request
        # Dispose evidence (blocking rmtree) AFTER releasing the lock, mirroring
        # purge_older_than, so disk I/O never stalls concurrent store operations.
        if evicted is not None:
            self._dispose_evidence([evicted])
        return job_id

    def get(self, job_id: str) -> ScanResult | None:
        with self._lock:
            return self._jobs.get(job_id)

    def get_request(self, job_id: str) -> ScanRequest | None:
        with self._lock:
            return self._requests.get(job_id)

    def update_status(
        self,
        job_id: str,
        *,
        status: ScanStatus | None = None,
        error: str | None = None,
        progress: Progress | None = None,
        findings: list | None = None,
        summary: ScanSummary | None = None,
        completed_at: datetime | None = None,
    ) -> None:
        with self._lock:
            result = self._jobs.get(job_id)
            if result is None:
                return
            if status is not None:
                result.status = status
            if error is not None:
                result.error = error
            if progress is not None:
                result.progress = progress
            if findings is not None:
                result.findings = findings
            if summary is not None:
                result.summary = summary
            if completed_at is not None:
                result.completed_at = completed_at

    def delete(self, job_id: str) -> None:
        with self._lock:
            self._jobs.pop(job_id, None)
            self._requests.pop(job_id, None)

    def all_ids(self) -> list[str]:
        with self._lock:
            return list(self._jobs.keys())

    def active_ids(self) -> set[str]:
        """Job ids that are still pending/running — their evidence must be
        protected from size-based pruning while a scan is in flight."""
        with self._lock:
            return {
                job_id
                for job_id, result in self._jobs.items()
                if not _is_terminal(result.status)
            }

    def purge_older_than(self, age: timedelta) -> int:
        """Drop terminal jobs older than `age`.

        Only finished scans are eligible: a pending/running scan is never
        purged out from under the runner, regardless of how old it is. The age
        is measured from `completed_at` (falling back to `submitted_at`).
        """
        cutoff = self._now() - age
        purged: list[str] = []
        with self._lock:
            for job_id, result in list(self._jobs.items()):
                if not _is_terminal(result.status):
                    continue
                if _age_reference(result) < cutoff:
                    purged.append(job_id)
                    self._jobs.pop(job_id, None)
                    self._requests.pop(job_id, None)
        if purged:
            self._dispose_evidence(purged)
        return len(purged)

    def _evict_oldest_terminal_locked(self) -> str | None:
        """Evict the oldest finished job to make room and return its id, or None
        when every resident job is still active (so the caller refuses the new
        submission rather than dropping an in-flight scan). Evidence disposal is
        left to the caller to run OUTSIDE the lock."""
        terminal = [
            (job_id, result)
            for job_id, result in self._jobs.items()
            if _is_terminal(result.status)
        ]
        if not terminal:
            return None
        oldest_id = min(terminal, key=lambda item: _age_reference(item[1]))[0]
        self._jobs.pop(oldest_id, None)
        self._requests.pop(oldest_id, None)
        return oldest_id

    def _dispose_evidence(self, job_ids: list[str]) -> None:
        try:
            from app.core.evidence_store import get_evidence_store

            store = get_evidence_store()
            for job_id in job_ids:
                store.delete_for_job(job_id)
        except (OSError, RuntimeError) as exc:
            _logger.warning("failed to dispose evidence for purged jobs: %s", exc)


job_store = JobStore()


async def _prune_forever(
    interval_seconds: int,
    ttl_seconds: int,
    max_evidence_bytes: int = 0,
) -> None:
    while True:
        try:
            await asyncio.sleep(interval_seconds)
            job_store.purge_older_than(timedelta(seconds=ttl_seconds))
            if max_evidence_bytes > 0:
                # Enforce the size cap OUTSIDE any store lock, protecting the
                # evidence of jobs still in flight from being trimmed.
                from app.core.evidence_store import get_evidence_store

                get_evidence_store().prune(
                    max_bytes=max_evidence_bytes,
                    protect=job_store.active_ids(),
                )
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001 -- one bad iteration must not kill the loop
            _logger.warning("job-store prune iteration failed: %s", exc)


def start_pruner(
    loop: asyncio.AbstractEventLoop,
    interval_seconds: int = 300,
    ttl_seconds: int = 86400,
    max_evidence_bytes: int | None = None,
) -> asyncio.Task:
    if max_evidence_bytes is None:
        max_evidence_bytes = MAX_EVIDENCE_BYTES
    return loop.create_task(
        _prune_forever(interval_seconds, ttl_seconds, max_evidence_bytes)
    )
