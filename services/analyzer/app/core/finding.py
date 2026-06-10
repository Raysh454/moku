"""Finding model and helpers shared by plugins and the builtin adapter."""

import uuid
from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field

from app.models.schemas import Severity

_FINDING_ID_HEX_WIDTH = 8


def make_finding_id(prefix: str) -> str:
    """Return a short, unique finding id of the form ``<prefix>-<8 hex>``.

    Centralises the id format so adapters and plugins do not each hand-roll
    ``uuid4().hex[:8]``; the width lives in one named constant.
    """
    return f"{prefix}-{uuid.uuid4().hex[:_FINDING_ID_HEX_WIDTH]}"


class EvidenceRef(BaseModel):
    sha256: str
    size: int
    path: str
    label: str


class Finding(BaseModel):
    finding_id: str
    plugin: str
    scan_unit_url: str
    http_method: str
    payload_used: str
    matched_pattern: str
    response_snippet: str
    evidence_refs: list[EvidenceRef] = Field(default_factory=list)
    confidence: float
    scoring_version: str = "v1"
    repro_steps: list[str] = Field(default_factory=list)
    timestamp: datetime | None = None
    notes: str | None = None
    meta: dict[str, Any] = Field(default_factory=dict)


def confidence_to_severity(c: float) -> Severity:
    """Map a 0.0–1.0 confidence score onto the tiered severity scale."""
    if c >= 0.9:
        return Severity.CRITICAL
    if c >= 0.7:
        return Severity.HIGH
    if c >= 0.5:
        return Severity.MEDIUM
    if c >= 0.3:
        return Severity.LOW
    return Severity.INFO
