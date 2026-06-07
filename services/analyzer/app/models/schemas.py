"""Pydantic schemas matching the Moku Go-side Analyzer contract."""

from __future__ import annotations

import re
from datetime import UTC, datetime, timedelta
from enum import Enum
from typing import Any
from urllib.parse import urlparse

from pydantic import (
    BaseModel,
    ConfigDict,
    Field,
    HttpUrl,
    field_serializer,
    field_validator,
    model_validator,
)

from app.net_guard import assert_public_host

# Version of the Go<->Python wire contract (request/response shapes). Bumped
# only on a breaking change to the JSON payloads. Exposed on /health so the Go
# client can detect skew against its own analyzer.SidecarContractVersion.
CONTRACT_VERSION = "1"


class ScanStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELED = "canceled"


class Severity(str, Enum):
    INFO = "info"
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


class Confidence(str, Enum):
    TENTATIVE = "tentative"
    FIRM = "firm"
    CERTAIN = "certain"


class Backend(str, Enum):
    BUILTIN = "builtin"
    NUCLEI = "nuclei"
    NIKTO = "nikto"
    SHODAN = "shodan"
    VIRUSTOTAL = "virustotal"
    ZAP = "zap"


class Profile(str, Enum):
    QUICK = "quick"
    BALANCED = "balanced"
    THOROUGH = "thorough"


class HealthStatus(str, Enum):
    OK = "ok"
    DEGRADED = "degraded"
    UNAVAILABLE = "unavailable"


_BASE_CONFIG = ConfigDict(
    populate_by_name=True,
    extra="forbid",
    use_enum_values=True,
)


_DURATION_PATTERN = re.compile(
    r"(?P<value>\d+(?:\.\d+)?)(?P<unit>ns|us|µs|ms|s|m|h)"
)
_DURATION_UNIT_TO_SECONDS = {
    "ns": 1e-9,
    "us": 1e-6,
    "µs": 1e-6,
    "ms": 1e-3,
    "s": 1.0,
    "m": 60.0,
    "h": 3600.0,
}


def _parse_go_duration(value: Any) -> timedelta | None:
    """Coerce Go-style duration strings or int-ns values into a `timedelta`."""
    if value is None:
        return None
    if isinstance(value, timedelta):
        return value
    if isinstance(value, bool):
        raise ValueError("max_duration cannot be a boolean")
    if isinstance(value, int):
        return timedelta(microseconds=value / 1000)
    if isinstance(value, float):
        return timedelta(seconds=value)
    if isinstance(value, str):
        stripped = value.strip()
        if not stripped:
            raise ValueError("max_duration string is empty")
        matches = list(_DURATION_PATTERN.finditer(stripped))
        consumed = "".join(m.group(0) for m in matches)
        if not matches or consumed != stripped:
            raise ValueError(f"invalid duration: {value!r}")
        total = 0.0
        for m in matches:
            total += float(m.group("value")) * _DURATION_UNIT_TO_SECONDS[m.group("unit")]
        return timedelta(seconds=total)
    raise TypeError(f"unsupported duration type: {type(value).__name__}")


class Scope(BaseModel):
    model_config = _BASE_CONFIG

    include_hosts: list[str] = Field(default_factory=list)
    exclude_hosts: list[str] = Field(default_factory=list)
    include_paths: list[str] = Field(default_factory=list)
    exclude_paths: list[str] = Field(default_factory=list)


class Auth(BaseModel):
    model_config = _BASE_CONFIG

    type: str | None = None
    username: str | None = None
    password: str | None = None
    token: str | None = None
    extra: dict[str, str] = Field(default_factory=dict)


class ScanRequest(BaseModel):
    model_config = _BASE_CONFIG

    url: HttpUrl = Field(..., max_length=2048)
    backend: Backend
    profile: Profile | None = None
    scope: Scope | None = None
    auth: Auth | None = None
    max_duration: timedelta | None = None
    raw_options: dict[str, str] = Field(default_factory=dict)

    @field_validator("max_duration", mode="before")
    @classmethod
    def _coerce_max_duration(cls, value: Any) -> timedelta | None:
        return _parse_go_duration(value)

    @model_validator(mode="after")
    def _reject_disallowed_targets(self) -> ScanRequest:
        parsed = urlparse(str(self.url))
        if parsed.scheme not in {"http", "https"}:
            raise ValueError(f"unsupported url scheme: {parsed.scheme!r}")
        host = parsed.hostname
        if not host:
            raise ValueError("url must include a hostname")
        assert_public_host(host)
        return self


class Finding(BaseModel):
    model_config = _BASE_CONFIG

    id: str
    title: str
    severity: Severity
    confidence: Confidence
    url: str | None = None
    path: str | None = None
    method: str | None = None
    parameter: str | None = None
    cwe: list[int] | None = None
    wasc: list[int] | None = None
    description: str | None = None
    evidence: str | None = None
    remediation: str | None = None
    references: list[str] | None = None
    raw_data: dict[str, Any] | None = None


class ScanSummary(BaseModel):
    model_config = _BASE_CONFIG

    total: int = 0
    info: int = 0
    low: int = 0
    medium: int = 0
    high: int = 0
    critical: int = 0


class Progress(BaseModel):
    model_config = _BASE_CONFIG

    percent: int = 0
    phase: str = ""
    note: str = ""


class ScanResult(BaseModel):
    model_config = _BASE_CONFIG

    job_id: str
    backend: Backend
    status: ScanStatus
    url: str | None = None
    error: str | None = None
    submitted_at: datetime
    completed_at: datetime | None = None
    progress: Progress = Field(default_factory=Progress)
    findings: list[Finding] = Field(default_factory=list)
    summary: ScanSummary | None = None
    raw_data: dict[str, Any] = Field(default_factory=dict)

    @field_serializer("submitted_at", "completed_at")
    def _serialize_datetime(self, value: datetime | None) -> str | None:
        if value is None:
            return None
        if value.tzinfo is None:
            value = value.replace(tzinfo=UTC)
        return value.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


class Capabilities(BaseModel):
    """Static metadata describing what a sidecar adapter supports.

    Field-name alias: ``async_`` in Python (trailing underscore to avoid the
    reserved ``async`` keyword) is exposed over JSON as the literal key
    ``"async"`` to match Moku's Go-side ``Capabilities.Async`` field. Because
    ``_BASE_CONFIG`` sets ``populate_by_name=True``, producers may use either
    the Python name or the JSON alias when constructing instances; the wire
    representation is always ``"async"``.
    """

    model_config = _BASE_CONFIG

    async_: bool = Field(False, alias="async")
    supports_auth: bool = False
    supports_scope: bool = False
    supports_scan_profile: bool = False
    max_concurrent_scans: int = 1
    version: str = "0.1.0"


class HealthResponse(BaseModel):
    model_config = _BASE_CONFIG

    status: HealthStatus
    contract_version: str = CONTRACT_VERSION
    backend: Backend | None = None
    adapters_available: list[str] = Field(default_factory=list)


class SubmitResponse(BaseModel):
    model_config = _BASE_CONFIG

    job_id: str
