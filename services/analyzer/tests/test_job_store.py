"""Tests for the rewritten in-memory `JobStore`."""

import threading
import time
from datetime import UTC, datetime, timedelta

import pytest

from app.core import job_store as job_store_module
from app.core.job_store import JobStore, JobStoreFull
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
    def test_drops_expired_terminal_entries(self):
        now = datetime(2026, 1, 1, 12, 0, 0, tzinfo=UTC)
        store = JobStore(now_factory=lambda: now)
        old_job = store.create(_build_request())
        store.create(_build_request())

        store.update_status(
            old_job,
            status=ScanStatus.COMPLETED,
            completed_at=now - timedelta(days=2),
        )

        purged = store.purge_older_than(timedelta(days=1))
        assert purged == 1
        assert store.get(old_job) is None

    def test_keeps_active_entries_even_when_old(self):
        now = datetime(2026, 1, 1, 12, 0, 0, tzinfo=UTC)
        store = JobStore(now_factory=lambda: now)
        job_id = store.create(_build_request())
        # Old, but still pending — must never be purged out from under the runner.
        store.get(job_id).submitted_at = now - timedelta(days=2)

        assert store.purge_older_than(timedelta(days=1)) == 0
        assert store.get(job_id) is not None


class TestEviction:
    def test_evicts_oldest_terminal_when_full(self, monkeypatch):
        monkeypatch.setattr(job_store_module, "MAX_JOBS", 2)
        store = JobStore()
        first = store.create(_build_request())
        store.update_status(
            first, status=ScanStatus.COMPLETED, completed_at=datetime.now(UTC)
        )
        second = store.create(_build_request())  # pending
        third = store.create(_build_request())  # full -> evict terminal 'first'
        assert store.get(first) is None
        assert store.get(second) is not None
        assert store.get(third) is not None

    def test_refuses_new_job_when_full_of_active(self, monkeypatch):
        monkeypatch.setattr(job_store_module, "MAX_JOBS", 2)
        store = JobStore()
        store.create(_build_request())  # pending
        store.create(_build_request())  # pending
        with pytest.raises(JobStoreFull):
            store.create(_build_request())

    def test_delete_removes_entry(self):
        store = JobStore()
        job_id = store.create(_build_request())
        store.delete(job_id)
        assert store.get(job_id) is None

    def test_eviction_does_not_hold_lock_during_evidence_disposal(self, monkeypatch):
        # Eviction disposes the evicted job's evidence (a blocking shutil.rmtree).
        # That disposal must happen AFTER the store lock is released, or every
        # concurrent get()/create() stalls behind disk I/O. Reproduce with a
        # deliberately slow delete_for_job and assert a concurrent get() is not
        # blocked for its duration.
        monkeypatch.setattr(job_store_module, "MAX_JOBS", 1)
        delete_started = threading.Event()
        slow_delete_seconds = 0.5

        def slow_delete(self, job_id):
            delete_started.set()
            time.sleep(slow_delete_seconds)
            return 0

        monkeypatch.setattr(
            "app.core.evidence_store.EvidenceStore.delete_for_job", slow_delete
        )

        store = JobStore()
        first = store.create(_build_request())
        store.update_status(
            first, status=ScanStatus.COMPLETED, completed_at=datetime.now(UTC)
        )

        # This create() hits the cap, evicts the terminal job, and disposes its
        # evidence (slow) — with the fix, after releasing the lock.
        creator = threading.Thread(target=lambda: store.create(_build_request()))
        creator.start()
        try:
            assert delete_started.wait(timeout=2.0), "eviction disposal never started"
            start = time.perf_counter()
            store.get("does-not-matter")
            elapsed = time.perf_counter() - start
        finally:
            creator.join()

        assert elapsed < slow_delete_seconds / 2, (
            f"get() blocked {elapsed:.3f}s during eviction rmtree — the store "
            "lock is held across filesystem I/O"
        )


def test_all_ids_returns_current_set():
    store = JobStore()
    first = store.create(_build_request())
    second = store.create(_build_request())
    ids = store.all_ids()
    assert first in ids
    assert second in ids


class TestPruner:
    def test_loop_purges_then_honors_cancel(self, monkeypatch):
        import asyncio

        calls: list = []
        monkeypatch.setattr(
            job_store_module.job_store,
            "purge_older_than",
            lambda age: calls.append(age) or 0,
        )

        async def drive():
            task = asyncio.ensure_future(job_store_module._prune_forever(0, 10))
            for _ in range(5):
                await asyncio.sleep(0)
            task.cancel()
            with pytest.raises(asyncio.CancelledError):
                await task

        asyncio.run(drive())
        assert calls  # at least one prune iteration ran

    def test_loop_swallows_transient_oserror_and_continues(self, monkeypatch):
        import asyncio

        state = {"n": 0}

        def flaky(age):
            state["n"] += 1
            if state["n"] == 1:
                raise OSError("disk blip")
            return 0

        monkeypatch.setattr(
            job_store_module.job_store, "purge_older_than", flaky
        )

        async def drive():
            task = asyncio.ensure_future(job_store_module._prune_forever(0, 10))
            for _ in range(8):
                await asyncio.sleep(0)
            task.cancel()
            try:
                await task
            except asyncio.CancelledError:
                pass

        asyncio.run(drive())
        assert state["n"] >= 2  # survived the OSError and kept looping
