"""Nikto adapter — wraps the `nikto` CLI and parses line-based output."""

import logging

from app.adapters._helpers import run_subprocess, validate_target_url
from app.adapters.cli_scanner import CliScannerAdapter
from app.core.finding import make_finding_id
from app.models.schemas import (
    Backend,
    Confidence,
    Finding,
    ScanRequest,
    Severity,
)

_logger = logging.getLogger(__name__)
_TITLE_MAX_LENGTH = 80


class NiktoAdapter(CliScannerAdapter):
    name = Backend.NIKTO.value
    description = "Nikto web server scanner"
    default_timeout_seconds = 120

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        target = validate_target_url(str(request.url))
        output = run_subprocess(
            ["nikto", "-h", target, "-nointeractive"],
            timeout=self._timeout_seconds(request.max_duration),
            name="nikto",
        )
        return self._parse(output, target)

    def _parse(self, output: str, target: str) -> list[Finding]:
        findings: list[Finding] = []
        for raw_line in output.splitlines():
            line = raw_line.strip()
            if not line.startswith("+"):
                continue
            try:
                findings.append(
                    Finding(
                        id=make_finding_id("nikto"),
                        title=line[:_TITLE_MAX_LENGTH],
                        severity=Severity.INFO,
                        confidence=Confidence.TENTATIVE,
                        url=target,
                        description=line,
                        raw_data={"raw_line": line},
                    )
                )
            except (ValueError, TypeError):
                # One unparseable line must not abort the whole scan.
                _logger.warning("nikto: skipped malformed output line")
                continue
        return findings
