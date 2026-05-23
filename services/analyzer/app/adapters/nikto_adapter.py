"""Nikto adapter — wraps the `nikto` CLI and parses line-based output."""

import logging
import uuid

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


class NiktoAdapter(BaseAdapter):
    name = Backend.NIKTO.value
    description = "Nikto web server scanner"

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
            ["nikto", "-h", target, "-nointeractive"],
            timeout=120,
            name="nikto",
        )
        return self._parse(output, target)

    def _parse(self, output: str, target: str) -> list[Finding]:
        findings: list[Finding] = []
        for line in output.splitlines():
            line = line.strip()
            if not line.startswith("+"):
                continue
            try:
                findings.append(
                    Finding(
                        id=f"nikto-{uuid.uuid4().hex[:8]}",
                        title=line[:80],
                        severity=Severity.INFO,
                        confidence=Confidence.TENTATIVE,
                        url=target,
                        description=line,
                        raw_data={"raw_line": line},
                    )
                )
            except (ValueError, TypeError) as exc:
                _logger.warning("nikto: skipped malformed line: %s", exc)
                continue
        return findings
