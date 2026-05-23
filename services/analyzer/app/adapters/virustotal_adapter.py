"""VirusTotal adapter — checks URL reputation against vendor databases."""

import logging
import os
import time
import uuid

import requests

from app.adapters._helpers import validate_target_url
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

_SUBMIT_URL = "https://www.virustotal.com/api/v3/urls"
_ANALYSIS_URL_TMPL = "https://www.virustotal.com/api/v3/analyses/{}"
_MAX_URL_LENGTH = 2048
_POLL_ATTEMPTS = 10
_POLL_DELAY_SECONDS = 2


class VirusTotalAdapter(BaseAdapter):
    name = Backend.VIRUSTOTAL.value
    description = "VirusTotal URL reputation check"

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
        api_key = os.getenv("VIRUSTOTAL_API_KEY")
        if not api_key:
            raise RuntimeError("VIRUSTOTAL_API_KEY is not set in environment")

        consent = (request.raw_options.get("virustotal_consent") or "").lower()
        if consent != "true":
            raise RuntimeError(
                "virustotal submission requires raw_options.virustotal_consent=true"
            )

        target = validate_target_url(str(request.url))
        if len(target) > _MAX_URL_LENGTH:
            raise ValueError(f"url exceeds {_MAX_URL_LENGTH} characters")

        analysis_id = self._submit_url(target, api_key)
        report = self._poll_analysis(analysis_id, api_key)
        return self._map_results(report, target)

    def _submit_url(self, target: str, api_key: str) -> str:
        headers = {"x-apikey": api_key}
        try:
            resp = requests.post(
                _SUBMIT_URL,
                headers=headers,
                data={"url": target},
                timeout=30,
            )
        except requests.RequestException as exc:
            _logger.warning("virustotal submit failed: %s", exc.__class__.__name__)
            raise RuntimeError("virustotal request failed") from None

        if resp.status_code not in (200, 201):
            raise RuntimeError(f"virustotal returned status {resp.status_code}")

        analysis_id = resp.json().get("data", {}).get("id")
        if not analysis_id:
            raise RuntimeError("virustotal response missing analysis id")
        return analysis_id

    def _poll_analysis(self, analysis_id: str, api_key: str) -> dict:
        headers = {"x-apikey": api_key}
        url = _ANALYSIS_URL_TMPL.format(analysis_id)
        for _ in range(_POLL_ATTEMPTS):
            try:
                resp = requests.get(url, headers=headers, timeout=30)
            except requests.RequestException as exc:
                _logger.warning("virustotal poll failed: %s", exc.__class__.__name__)
                raise RuntimeError("virustotal request failed") from None
            if resp.status_code != 200:
                raise RuntimeError(f"virustotal returned status {resp.status_code}")

            data = resp.json().get("data", {})
            if data.get("attributes", {}).get("status") == "completed":
                return data
            time.sleep(_POLL_DELAY_SECONDS)

        raise RuntimeError("virustotal analysis did not complete in time")

    def _map_results(self, report: dict, target: str) -> list[Finding]:
        results = report.get("attributes", {}).get("results", {})
        findings: list[Finding] = []
        malicious_count = 0
        total_vendors = len(results)

        for vendor, detection in results.items():
            category = str(detection.get("category", "")).lower()
            if category in {"malicious", "suspicious"}:
                malicious_count += 1
                findings.append(
                    Finding(
                        id=f"virustotal-{uuid.uuid4().hex[:8]}",
                        title="malicious-url",
                        severity=Severity.CRITICAL,
                        confidence=Confidence.FIRM,
                        url=target,
                        description=f"Flagged as {category} by {vendor}",
                        evidence=str(detection),
                        raw_data={"vendor": vendor, "result": detection},
                    )
                )

        findings.append(
            Finding(
                id=f"virustotal-{uuid.uuid4().hex[:8]}",
                title="virustotal-summary",
                severity=Severity.HIGH if malicious_count > 0 else Severity.INFO,
                confidence=Confidence.FIRM,
                url=target,
                description=(
                    f"{malicious_count}/{total_vendors} vendors labeled URL "
                    "malicious or suspicious"
                ),
                raw_data={
                    "malicious_count": malicious_count,
                    "total_vendors": total_vendors,
                },
            )
        )
        return findings
