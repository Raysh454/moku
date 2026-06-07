"""Bidirectional Go<->Python wire-contract guard.

schema_check.py proves Python can *read* the Go-shaped fixtures. This closes
the other direction: it pins the exact JSON the Python serializer *produces*
to a committed golden that the Go consumer decodes
(TestSidecarFixtures_DecodeCleanly in internal/analyzer/sidecar_analyzer_test.go).
If the Python serializer drifts (e.g. datetime precision/offset changes), this
test fails and the golden must be regenerated — at which point the Go decode
test exercises the new format too.
"""

import json
from datetime import UTC, datetime
from pathlib import Path

from app.models.schemas import (
    Backend,
    Confidence,
    Finding,
    Progress,
    ScanResult,
    ScanStatus,
    ScanSummary,
    Severity,
)

_GOLDEN_PATH = (
    Path(__file__).resolve().parents[3]
    / "internal"
    / "analyzer"
    / "testdata"
    / "sidecar"
    / "scan_result_python_serialized.json"
)


def _build_reference_scan_result() -> ScanResult:
    return ScanResult(
        job_id="abc123def456abc123def456abc12345",
        backend=Backend.BUILTIN,
        status=ScanStatus.COMPLETED,
        url="https://example.com/",
        submitted_at=datetime(2026, 1, 15, 10, 30, 45, 123000, tzinfo=UTC),
        completed_at=datetime(2026, 1, 15, 10, 31, 15, 456000, tzinfo=UTC),
        progress=Progress(percent=100, phase="completed", note=""),
        findings=[
            Finding(
                id="f1",
                title="XSS",
                severity=Severity.HIGH,
                confidence=Confidence.FIRM,
                url="https://example.com/",
                parameter="q",
                description="reflected",
                evidence="<script>",
            )
        ],
        summary=ScanSummary(total=1, high=1),
        raw_data={},
    )


def test_serializer_output_matches_committed_go_golden():
    produced = json.loads(
        _build_reference_scan_result().model_dump_json(by_alias=True)
    )
    golden = json.loads(_GOLDEN_PATH.read_text(encoding="utf-8"))
    assert produced == golden


def test_serializer_emits_millisecond_z_datetime():
    produced = json.loads(
        _build_reference_scan_result().model_dump_json(by_alias=True)
    )
    # The Go time.Time consumer relies on RFC3339; pin the exact shape.
    assert produced["submitted_at"] == "2026-01-15T10:30:45.123Z"
