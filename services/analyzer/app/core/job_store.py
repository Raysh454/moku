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
        with self._lock:
            if len(self._jobs) >= MAX_JOBS:
                self._evict_oldest_locked()
            self._jobs[job_id] = result
            self._requests[job_id] = request
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

    def purge_older_than(self, age: timedelta) -> int:
        cutoff = self._now() - age
        purged: list[str] = []
        with self._lock:
            for job_id, result in list(self._jobs.items()):
                submitted = result.submitted_at
                if submitted.tzinfo is None:
                    submitted = submitted.replace(tzinfo=UTC)
                if submitted < cutoff:
                    purged.append(job_id)
                    self._jobs.pop(job_id, None)
                    self._requests.pop(job_id, None)
        if purged:
            self._dispose_evidence(purged)
        return len(purged)

    def _evict_oldest_locked(self) -> None:
        if not self._jobs:
            return
        oldest_id = min(
            self._jobs,
            key=lambda jid: self._jobs[jid].submitted_at,
        )
        self._jobs.pop(oldest_id, None)
        self._requests.pop(oldest_id, None)
        self._dispose_evidence([oldest_id])

    def _dispose_evidence(self, job_ids: list[str]) -> None:
        try:
            from app.core.evidence_store import get_evidence_store

            store = get_evidence_store()
            for job_id in job_ids:
                store.delete_for_job(job_id)
        except (OSError, RuntimeError) as exc:
            _logger.warning("failed to dispose evidence for purged jobs: %s", exc)


job_store = JobStore()


async def _prune_forever(interval_seconds: int, ttl_seconds: int) -> None:
    while True:
        try:
            await asyncio.sleep(interval_seconds)
            job_store.purge_older_than(timedelta(seconds=ttl_seconds))
        except asyncio.CancelledError:
            raise
        except (OSError, RuntimeError) as exc:
            _logger.warning("job-store prune iteration failed: %s", exc)


def start_pruner(
    loop: asyncio.AbstractEventLoop,
    interval_seconds: int = 300,
    ttl_seconds: int = 86400,
) -> asyncio.Task:
    return loop.create_task(_prune_forever(interval_seconds, ttl_seconds))
