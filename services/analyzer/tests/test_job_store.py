"""Tests for the rewritten in-memory `JobStore`."""

from datetime import UTC, datetime, timedelta

from app.core import job_store as job_store_module
from app.core.job_store import JobStore
from app.models.schemas import (
    Backend,
    Progress,
    ScanRequest,
    ScanStatus,
    ScanSummary,
)


def _build_request() -> ScanRequest:
    return ScanRequest(url="https://example.com", backend=Backend.BUILTIN)


class TestJobStoreCreate:
    def test_create_returns_uuid_hex(self):
        store = JobStore()
        job_id = store.create(_build_request())
        assert len(job_id) == 32
        assert all(c in "0123456789abcdef" for c in job_id)

    def test_create_records_submitted_at_and_pending_status(self):
        store = JobStore()
        job_id = store.create(_build_request())
        result = store.get(job_id)
        assert result is not None
        assert result.status == ScanStatus.PENDING.value
        assert isinstance(result.submitted_at, datetime)

    def test_get_request_returns_original(self):
        store = JobStore()
        request = _build_request()
        job_id = store.create(request)
        assert store.get_request(job_id) is request


class TestUpdateStatus:
    def test_update_status_preserves_submitted_at(self):
        store = JobStore()
        job_id = store.create(_build_request())
        before = store.get(job_id).submitted_at
        store.update_status(
            job_id,
            status=ScanStatus.COMPLETED,
            findings=[],
            summary=ScanSummary(),
            progress=Progress(percent=100.0, phase="completed"),
            completed_at=datetime.now(UTC),
        )
        result = store.get(job_id)
        assert result.submitted_at == before
        assert result.status == ScanStatus.COMPLETED.value

    def test_update_status_for_unknown_job_is_noop(self):
        store = JobStore()
        store.update_status("missing", status=ScanStatus.COMPLETED)


class TestPurgeOlderThan:
    def test_drops_expired_entries(self):
        now = datetime(2026, 1, 1, 12, 0, 0, tzinfo=UTC)
        store = JobStore(now_factory=lambda: now)
        old_job = store.create(_build_request())
        store.create(_build_request())

        result = store.get(old_job)
        result.submitted_at = now - timedelta(days=2)

        purged = store.purge_older_than(timedelta(days=1))
        assert purged == 1
        assert store.get(old_job) is None


class TestEviction:
    def test_eviction_when_max_jobs_exceeded(self, monkeypatch):
        monkeypatch.setattr(job_store_module, "MAX_JOBS", 2)
        store = JobStore()
        first = store.create(_build_request())
        second = store.create(_build_request())
        third = store.create(_build_request())
        # First entry must have been evicted to make room.
        assert store.get(first) is None
        assert store.get(second) is not None
        assert store.get(third) is not None

    def test_delete_removes_entry(self):
        store = JobStore()
        job_id = store.create(_build_request())
        store.delete(job_id)
        assert store.get(job_id) is None


def test_all_ids_returns_current_set():
    store = JobStore()
    first = store.create(_build_request())
    second = store.create(_build_request())
    ids = store.all_ids()
    assert first in ids
    assert second in ids
