"""Tests for the disk-backed evidence store."""

import os
from datetime import timedelta
from pathlib import Path

import pytest

from app.core.evidence_store import EvidenceStore


def test_save_writes_under_per_job_dir(tmp_path):
    store = EvidenceStore(str(tmp_path))
    ref = store.save(b"hello world", label="resp", job_id="job-1")
    assert Path(ref.path).parent.name == "job-1"
    assert Path(ref.path).read_bytes() == b"hello world"


def test_save_bytes_round_trips_through_load(tmp_path):
    store = EvidenceStore(str(tmp_path))
    ref = store.save(b"bytes-data", label="resp", job_id="j")
    assert store.load(ref.sha256, job_id="j") == b"bytes-data"


def test_save_rejects_string(tmp_path):
    store = EvidenceStore(str(tmp_path))
    with pytest.raises(TypeError):
        store.save("string-data", label="resp")  # type: ignore[arg-type]


@pytest.mark.parametrize(
    "evil",
    [
        "../escape",
        "../../etc",
        "a/b",
        "a\\b",
        "..",
        ".",
    ],
)
def test_save_rejects_path_traversal_job_id(tmp_path, evil):
    store = EvidenceStore(str(tmp_path))
    with pytest.raises(ValueError):
        store.save(b"x", label="resp", job_id=evil)
    # nothing escaped the base dir
    assert list((tmp_path).rglob("x")) == []


def test_delete_for_job_rejects_path_traversal(tmp_path):
    store = EvidenceStore(str(tmp_path))
    with pytest.raises(ValueError):
        store.delete_for_job("../../etc")


def test_load_rejects_non_sha256(tmp_path):
    store = EvidenceStore(str(tmp_path))
    with pytest.raises(ValueError):
        store.load("../../etc/passwd", job_id="j")


def test_delete_for_job_removes_only_that_job(tmp_path):
    store = EvidenceStore(str(tmp_path))
    store.save(b"a", label="x", job_id="job-a")
    store.save(b"b", label="x", job_id="job-b")
    store.delete_for_job("job-a")
    assert not (Path(tmp_path) / "job-a").exists()
    assert (Path(tmp_path) / "job-b").exists()


def test_prune_max_age_drops_old_files(tmp_path):
    store = EvidenceStore(str(tmp_path))
    ref = store.save(b"old", label="x", job_id="job-c")
    path = Path(ref.path)
    old_time = path.stat().st_mtime - 10_000
    os.utime(path, (old_time, old_time))

    removed = store.prune(max_age=timedelta(seconds=1))
    assert removed == 1


def test_directory_not_created_on_import(monkeypatch, tmp_path):
    base = tmp_path / "evidence-no-touch"
    monkeypatch.setenv("MOKU_EVIDENCE_DIR", str(base))
    from app.core import evidence_store as module

    module._reset_evidence_store_for_tests()
    store = module.get_evidence_store()
    assert not base.exists()
    store.save(b"present", label="x", job_id="j")
    assert base.exists()
