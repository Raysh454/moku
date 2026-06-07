"""EvidenceStore — sha256-addressed blob storage for request/response evidence."""

import hashlib
import os
import re
import shutil
import time
from datetime import timedelta
from pathlib import Path

from app.core.finding import EvidenceRef

_SHA256_PATTERN = re.compile(r"\A[0-9a-f]{64}\Z")
_UNSAFE_SEGMENT_CHARS = ("/", "\\", "\x00")


def _default_base_dir() -> str:
    return os.environ.get("MOKU_EVIDENCE_DIR") or str(
        Path.home() / ".config" / "moku" / "evidence"
    )


class EvidenceStore:
    """Filesystem-backed evidence store keyed by sha256 of the blob.

    Blobs land in `<base>/<job_id>/<sha256>` so a single job's evidence can
    be purged in one call. The store is created lazily — importing the
    module never touches the filesystem.
    """

    def __init__(self, base_dir: str | None = None):
        self.base_dir = Path(base_dir or _default_base_dir())

    def _job_dir(self, job_id: str | None) -> Path:
        """Resolve the per-job evidence directory, rejecting path traversal.

        `job_id` is treated as a single, untrusted path segment. Anything that
        contains a path separator/NUL, is a relative-path token (`.`/`..`), or
        resolves outside `base_dir` is rejected so a caller-influenced job id
        can never escape the evidence root.
        """
        if not job_id:
            return self.base_dir / "_shared"
        if job_id in (".", "..") or any(c in job_id for c in _UNSAFE_SEGMENT_CHARS):
            raise ValueError(f"unsafe evidence job_id: {job_id!r}")
        candidate = (self.base_dir / job_id).resolve()
        if not candidate.is_relative_to(self.base_dir.resolve()):
            raise ValueError(f"evidence job_id escapes base dir: {job_id!r}")
        return candidate

    def save(self, data: bytes, label: str, job_id: str | None = None) -> EvidenceRef:
        """Persist `data` and return an `EvidenceRef` pointing at it."""
        if not isinstance(data, (bytes, bytearray)):
            raise TypeError("EvidenceStore.save requires bytes; encode strings first")
        raw = bytes(data)
        sha = hashlib.sha256(raw).hexdigest()
        target_dir = self._job_dir(job_id)
        target_dir.mkdir(parents=True, exist_ok=True)
        path = target_dir / sha

        if not path.exists():
            path.write_bytes(raw)

        return EvidenceRef(
            sha256=sha,
            size=len(raw),
            path=str(path),
            label=label,
        )

    def load(self, sha256: str, job_id: str | None = None) -> bytes:
        """Return the raw bytes of a previously stored blob."""
        if not _SHA256_PATTERN.match(sha256):
            raise ValueError(f"invalid sha256 digest: {sha256!r}")
        path = self._job_dir(job_id) / sha256
        if not path.exists():
            raise FileNotFoundError(f"evidence blob not found: {sha256}")
        return path.read_bytes()

    def delete_for_job(self, job_id: str) -> int:
        """Remove all blobs associated with `job_id`."""
        target = self._job_dir(job_id)
        if not target.exists():
            return 0
        count = sum(1 for _ in target.iterdir() if _.is_file())
        shutil.rmtree(target, ignore_errors=True)
        return count

    def prune(
        self,
        max_age: timedelta | None = None,
        max_bytes: int | None = None,
        protect: set[str] | None = None,
    ) -> int:
        """Drop blobs older than `max_age`, then (when `max_bytes` is set) trim
        the oldest remaining blobs until the total fits under it.

        `protect` is a set of job ids whose evidence must never be touched —
        pass the still-active (pending/running) jobs so an in-flight scan's
        evidence is never deleted out from under it. Filesystem races are
        tolerated: a blob that vanishes or is locked between discovery and
        deletion (a concurrent purge/eviction, or a Windows writer lock) is
        skipped rather than fatal.
        """
        if not self.base_dir.exists():
            return 0

        protected = protect or set()
        now = time.time()
        cutoff = now - max_age.total_seconds() if max_age is not None else None
        removed = 0

        files: list[tuple[float, int, Path]] = []
        for path in self.base_dir.rglob("*"):
            try:
                if not path.is_file():
                    continue
                if self._owning_job(path) in protected:
                    continue
                stat = path.stat()
            except OSError:
                continue  # vanished mid-walk; nothing to do
            if cutoff is not None and stat.st_mtime < cutoff:
                if self._safe_unlink(path):
                    removed += 1
                continue
            files.append((stat.st_mtime, stat.st_size, path))

        if max_bytes is not None:
            files.sort()
            total = sum(size for _, size, _ in files)
            for _, size, path in files:
                if total <= max_bytes:
                    break
                if self._safe_unlink(path):
                    total -= size
                    removed += 1

        return removed

    def _owning_job(self, path: Path) -> str | None:
        """Return the job-id directory a blob path belongs to, or None."""
        try:
            parts = path.relative_to(self.base_dir).parts
        except ValueError:
            return None
        return parts[0] if parts else None

    @staticmethod
    def _safe_unlink(path: Path) -> bool:
        """Delete a blob, tolerating concurrent removal / writer locks."""
        try:
            path.unlink(missing_ok=True)
            return True
        except OSError:
            return False


_store: EvidenceStore | None = None


def get_evidence_store() -> EvidenceStore:
    """Return a lazily-constructed module-singleton."""
    global _store
    if _store is None:
        _store = EvidenceStore()
    return _store


def _reset_evidence_store_for_tests() -> None:
    """Clear the cached singleton (test-only helper)."""
    global _store
    _store = None
