"""Nuclei adapter — wraps the `nuclei` CLI and maps JSONL output to findings."""

import json
import logging
import uuid
from typing import Any

from app.adapters._helpers import run_subprocess, validate_target_url
from app.adapters.base import BaseAdapter
from app.models.schemas import (
    Backend,
    Capabilities,
    Confidence,
    Finding,
    ScanRequest,
    Severity,
)

_logger = logging.getLogger(__name__)

_SEVERITY_MAP: dict[str, Severity] = {
    "info": Severity.INFO,
    "low": Severity.LOW,
    "medium": Severity.MEDIUM,
    "high": Severity.HIGH,
    "critical": Severity.CRITICAL,
}


class NucleiAdapter(BaseAdapter):
    name = Backend.NUCLEI.value
    description = "Nuclei vulnerability scanner"

    def capabilities(self) -> Capabilities:
        return Capabilities(
            async_=False,
            supports_auth=False,
            supports_scope=False,
            supports_scan_profile=False,
            max_concurrent_scans=1,
            version="0.1.0",
        )

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        target = validate_target_url(str(request.url))
        output = run_subprocess(
            ["nuclei", "-u", target, "-silent", "-jsonl"],
            timeout=300,
            name="nuclei",
        )
        return self._parse_jsonl(output, target)

    def _parse_jsonl(self, output: str, target: str) -> list[Finding]:
        findings: list[Finding] = []
        for line in output.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except json.JSONDecodeError:
                _logger.warning("nuclei: skipping malformed JSONL line")
                continue

            info = record.get("info") or {}
            severity_str = str(info.get("severity") or "info").lower()
            severity = _SEVERITY_MAP.get(severity_str, Severity.INFO)
            title = info.get("name") or record.get("template-id") or "nuclei-finding"
            description = info.get("description")

            cwe = self._extract_cwe(info)
            raw_data: dict[str, Any] = {
                "template_id": record.get("template-id"),
                "matched_at": record.get("matched-at"),
                "type": record.get("type"),
                "info": info,
            }

            findings.append(
                Finding(
                    id=f"nuclei-{uuid.uuid4().hex[:8]}",
                    title=title,
                    severity=severity,
                    confidence=Confidence.FIRM,
                    url=record.get("matched-at") or target,
                    description=description,
                    cwe=cwe,
                    raw_data=raw_data,
                )
            )
        return findings

    def _extract_cwe(self, info: dict) -> list[int] | None:
        classification = info.get("classification") or {}
        cwe_ids = classification.get("cwe-id")
        if not cwe_ids:
            return None
        if isinstance(cwe_ids, str):
            cwe_ids = [cwe_ids]
        parsed: list[int] = []
        for entry in cwe_ids:
            try:
                parsed.append(int(str(entry).lower().removeprefix("cwe-")))
            except ValueError:
                continue
        return parsed or None
