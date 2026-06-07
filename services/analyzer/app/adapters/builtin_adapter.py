"""BuiltinAdapter — Moku's reference active-scan analyzer (XSS, SQLi, CSRF)."""

import base64
from typing import Any
from urllib.parse import parse_qs, urlparse

from app.adapters._helpers import validate_target_url
from app.adapters.base import BaseAdapter
from app.core.executor import Executor
from app.core.finding import Finding as InternalFinding
from app.core.finding import confidence_to_severity
from app.core.scan_unit import ScanUnit, ScanUnitType
from app.models.schemas import (
    Auth,
    Backend,
    Capabilities,
    Confidence,
    ScanRequest,
)
from app.models.schemas import (
    Finding as ApiFinding,
)
from app.plugins.plugin_manager import PluginManager, plugin_manager


def _auth_header(auth: Auth | None) -> str | None:
    """Translate a request's Auth into an Authorization header value.

    Supports bearer tokens and HTTP basic credentials; cookie-based auth is
    handled separately via ``Auth.extra``. Returns None when no header-based
    credential is present.
    """
    if auth is None:
        return None
    if auth.token:
        return f"Bearer {auth.token}"
    if auth.username is not None and auth.password is not None:
        raw = f"{auth.username}:{auth.password}".encode()
        return "Basic " + base64.b64encode(raw).decode("ascii")
    return None


def _to_confidence_enum(c: float) -> Confidence:
    if c >= 0.9:
        return Confidence.CERTAIN
    if c >= 0.6:
        return Confidence.FIRM
    return Confidence.TENTATIVE


class BuiltinAdapter(BaseAdapter):
    name = Backend.BUILTIN.value
    description = "Moku built-in dynamic vulnerability analyzer"

    def __init__(self, plugins: PluginManager | None = None) -> None:
        # PluginManager is the single source of truth for the plugin roster;
        # injectable so tests can supply a stub without monkeypatching globals.
        self._plugin_manager = plugins or plugin_manager

    def capabilities(self) -> Capabilities:
        return Capabilities(
            async_=True,
            supports_auth=True,
            supports_scope=False,
            supports_scan_profile=False,
            max_concurrent_scans=1,
            version="0.1.0",
        )

    def run_scan(self, request: ScanRequest) -> list[ApiFinding]:
        target = validate_target_url(str(request.url))
        parsed = urlparse(target)
        params = {k: v[0] for k, v in parse_qs(parsed.query).items()}
        clean_url = f"{parsed.scheme}://{parsed.netloc}{parsed.path}"

        cookies: dict[str, str] = {}
        headers: dict[str, str] = {}
        if request.auth:
            if request.auth.extra:
                cookies = dict(request.auth.extra)
            auth_header = _auth_header(request.auth)
            if auth_header:
                headers["Authorization"] = auth_header

        scan_unit = ScanUnit(
            type=ScanUnitType.URL,
            url=clean_url,
            params=params,
            headers=headers,
            cookies=cookies,
            meta={"job_id": request.raw_options.get("job_id")},
        )

        test_cases = self._plugin_manager.generate_tests(scan_unit)
        if not test_cases:
            return []

        for test_case in test_cases:
            test_case.meta.setdefault("job_id", scan_unit.meta.get("job_id"))

        executor = Executor()
        internal_findings = executor.run(
            scan_unit=scan_unit,
            test_cases=test_cases,
            plugins=self._plugin_manager.get_plugins(),
            max_duration=request.max_duration,
        )

        # Deduplication is applied once, centrally, by the runner for every
        # adapter (see app.core.runner._dedupe_findings), so the adapter just
        # maps and returns.
        return [self._map_finding(f) for f in internal_findings]

    def _map_finding(self, f: InternalFinding) -> ApiFinding:
        severity = confidence_to_severity(f.confidence)
        confidence = _to_confidence_enum(f.confidence)
        raw_data: dict[str, Any] = {
            "payload": f.payload_used,
            "repro_steps": f.repro_steps,
            "evidence_refs": [e.sha256 for e in f.evidence_refs],
        }
        if f.meta.get("baseline_unavailable"):
            raw_data["baseline_unavailable"] = True
        if f.meta.get("warning"):
            raw_data["warning"] = f.meta["warning"]

        return ApiFinding(
            id=f.finding_id,
            title=f.plugin.upper(),
            severity=severity,
            confidence=confidence,
            url=f.scan_unit_url,
            parameter=f.meta.get("parameter"),
            method=f.http_method,
            description=f.matched_pattern,
            evidence=f.response_snippet,
            references=[],
            raw_data=raw_data,
        )
