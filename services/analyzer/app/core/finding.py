"""Finding model and helpers shared by plugins and the builtin adapter."""

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field

from app.models.schemas import Severity


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


def _confidence_to_severity(c: float) -> Severity:
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
