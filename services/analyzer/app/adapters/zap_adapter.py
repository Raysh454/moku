"""ZAP adapter — runs `zap.sh` quick scan and parses the JSON report."""

import json
import logging
import tempfile
from pathlib import Path

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

_RISK_TO_SEVERITY: dict[str, Severity] = {
    "high": Severity.HIGH,
    "medium": Severity.MEDIUM,
    "low": Severity.LOW,
    "informational": Severity.INFO,
    "info": Severity.INFO,
}


class ZAPAdapter(CliScannerAdapter):
    name = Backend.ZAP.value
    description = "OWASP ZAP active web vulnerability scanner"
    default_timeout_seconds = 600

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        target = validate_target_url(str(request.url))

        with tempfile.TemporaryDirectory(prefix="moku-zap-") as tmpdir:
            output_file = Path(tmpdir) / "zap_results.json"
            cmd = [
                "zap.sh",
                "-cmd",
                "-quickurl",
                target,
                "-quickout",
                str(output_file),
            ]
            run_subprocess(
                cmd,
                timeout=self._timeout_seconds(request.max_duration),
                name="zap",
            )

            if not output_file.exists():
                raise RuntimeError("zap did not produce an output file")

            try:
                report = json.loads(output_file.read_text(encoding="utf-8"))
            except json.JSONDecodeError as exc:
                raise RuntimeError("failed to parse zap output JSON") from exc

        return self._map_alerts(report, target)

    def _map_alerts(self, report: dict, target: str) -> list[Finding]:
        findings: list[Finding] = []
        for site in report.get("site", []):
            for alert in site.get("alerts", []):
                risk = str(alert.get("risk", "info")).lower()
                severity = _RISK_TO_SEVERITY.get(risk, Severity.INFO)
                findings.append(
                    Finding(
                        id=make_finding_id("zap"),
                        title=alert.get("alert", "zap-alert"),
                        severity=severity,
                        confidence=Confidence.FIRM,
                        url=target,
                        parameter=alert.get("param") or None,
                        description=alert.get("alert", "ZAP finding"),
                        evidence=alert.get("evidence"),
                        remediation=alert.get("solution"),
                        raw_data={"alert": alert},
                    )
                )
        return findings
