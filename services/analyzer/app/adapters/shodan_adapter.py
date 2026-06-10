"""Shodan adapter — passive recon via the Shodan Host API."""

import logging
import os
from urllib.parse import urlparse

import requests

from app.adapters.base import BaseAdapter
from app.core.finding import make_finding_id
from app.models.schemas import (
    Backend,
    Capabilities,
    Confidence,
    Finding,
    ScanRequest,
    Severity,
)
from app.net_guard import assert_public_host

_logger = logging.getLogger(__name__)


class ShodanAdapter(BaseAdapter):
    name = Backend.SHODAN.value
    description = "Shodan passive reconnaissance"

    def capabilities(self) -> Capabilities:
        return Capabilities(
            async_=True,
            supports_auth=False,
            supports_scope=False,
            supports_scan_profile=False,
            max_concurrent_scans=1,
            version="0.1.0",
        )

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        api_key = os.getenv("SHODAN_API_KEY")
        if not api_key:
            raise RuntimeError("SHODAN_API_KEY is not set in environment")

        url = str(request.url)
        parsed = urlparse(url)
        hostname = parsed.hostname
        if not hostname:
            raise ValueError("url must include a hostname")

        ip = self._resolve_public_ip(hostname)
        if ip is None:
            raise RuntimeError("no public IP resolved for host")

        try:
            resp = requests.get(
                f"https://api.shodan.io/shodan/host/{ip}",
                headers={"Authorization": f"Bearer {api_key}"},
                timeout=30,
            )
        except requests.RequestException as exc:
            _logger.warning("shodan request failed: %s", exc.__class__.__name__)
            raise RuntimeError("shodan request failed") from None

        if resp.status_code != 200:
            raise RuntimeError(f"shodan returned status {resp.status_code}")

        data = resp.json()
        return self._map_findings(data, url, ip)

    def _resolve_public_ip(self, hostname: str) -> str | None:
        """Return the first vetted public IP for `hostname`, or None if none.

        Uses the shared SSRF guard so the private/loopback rejection and the
        `MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS` dev bypass behave identically to
        the request-level and adapter-level checks. Raises `ValueError` (via
        the guard) when the host resolves to a disallowed address.
        """
        vetted = assert_public_host(hostname)
        return vetted[0] if vetted else None

    def _map_findings(self, data: dict, url: str, ip: str) -> list[Finding]:
        findings: list[Finding] = []
        for service in data.get("data", []):
            port = service.get("port")
            product = service.get("product") or "unknown"
            findings.append(
                Finding(
                    id=make_finding_id("shodan"),
                    title="open-port",
                    severity=Severity.INFO,
                    confidence=Confidence.FIRM,
                    url=url,
                    description=f"Open port: {port} running {product}",
                    evidence=f"{port}/{product}",
                    raw_data={
                        "ip": ip,
                        "hostnames": data.get("hostnames", []),
                        "service": service,
                    },
                )
            )

        vulns = data.get("vulns")
        if isinstance(vulns, dict):
            iter_vulns = vulns.keys()
        elif isinstance(vulns, list):
            iter_vulns = vulns
        else:
            iter_vulns = []

        for item in iter_vulns:
            findings.append(
                Finding(
                    id=make_finding_id("shodan"),
                    title=str(item),
                    severity=Severity.HIGH,
                    confidence=Confidence.FIRM,
                    url=url,
                    description=f"Known CVE detected by Shodan: {item}",
                    raw_data={"ip": ip},
                )
            )
        return findings
