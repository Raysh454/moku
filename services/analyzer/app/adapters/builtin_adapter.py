"""BuiltinAdapter — Moku's reference active-scan analyzer (XSS, SQLi, CSRF)."""

from typing import Any
from urllib.parse import parse_qs, urlparse

from app.adapters._helpers import validate_target_url
from app.adapters.base import BaseAdapter
from app.core.executor import Executor
from app.core.finding import Finding as InternalFinding
from app.core.finding import _confidence_to_severity
from app.core.scan_unit import ScanUnit, ScanUnitType
from app.models.schemas import (
    Backend,
    Capabilities,
    Confidence,
    ScanRequest,
    Severity,
)
from app.models.schemas import (
    Finding as ApiFinding,
)
from app.plugins.csrf_plugin import CSRFPlugin
from app.plugins.plugin_manager import plugin_manager
from app.plugins.sqli_plugin import SQLiPlugin
from app.plugins.xss_plugin import XSSPlugin

_SEVERITY_RANK: dict[Severity, int] = {
    Severity.INFO: 0,
    Severity.LOW: 1,
    Severity.MEDIUM: 2,
    Severity.HIGH: 3,
    Severity.CRITICAL: 4,
}


def _severity_rank(s: Severity) -> int:
    return _SEVERITY_RANK.get(s, 0)


def _to_confidence_enum(c: float) -> Confidence:
    if c >= 0.9:
        return Confidence.CERTAIN
    if c >= 0.6:
        return Confidence.FIRM
    return Confidence.TENTATIVE


class BuiltinAdapter(BaseAdapter):
    name = Backend.BUILTIN.value
    description = "Moku built-in dynamic vulnerability analyzer"

    def __init__(self) -> None:
        self._plugins = [XSSPlugin(), SQLiPlugin(), CSRFPlugin()]

    def capabilities(self) -> Capabilities:
        return Capabilities(
            async_=False,
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
        if request.auth and request.auth.extra:
            cookies = dict(request.auth.extra)

        scan_unit = ScanUnit(
            type=ScanUnitType.URL,
            url=clean_url,
            params=params,
            cookies=cookies,
            meta={"job_id": request.raw_options.get("job_id")},
        )

        test_cases = plugin_manager.generate_tests(scan_unit)
        if not test_cases:
            return []

        for test_case in test_cases:
            test_case.meta.setdefault("job_id", scan_unit.meta.get("job_id"))

        executor = Executor()
        internal_findings = executor.run(
            scan_unit=scan_unit,
            test_cases=test_cases,
            plugins=self._plugins,
        )

        api_findings = [self._map_finding(f) for f in internal_findings]
        return self._dedupe(api_findings)

    def _map_finding(self, f: InternalFinding) -> ApiFinding:
        severity = _confidence_to_severity(f.confidence)
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

    def _dedupe(self, findings: list[ApiFinding]) -> list[ApiFinding]:
        best: dict[tuple[str, str | None, str | None], ApiFinding] = {}
        for finding in findings:
            key = (finding.title, finding.parameter, finding.url)
            current = best.get(key)
            if current is None or _severity_rank(
                Severity(finding.severity)
            ) > _severity_rank(Severity(current.severity)):
                best[key] = finding
        return list(best.values())
