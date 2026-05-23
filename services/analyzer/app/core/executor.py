"""Executor — sends test payloads, collects responses, drives plugins."""

import logging
import os
import threading
import time
from urllib.parse import urlparse

import requests

from app.core.evidence_store import get_evidence_store
from app.core.finding import Finding
from app.core.scan_unit import ScanUnit
from app.core.test_case import TestCase
from app.plugins.base_plugin import BasePlugin

_logger = logging.getLogger(__name__)

_DEFAULT_MAX_REQUESTS_PER_HOST = 30
_DEFAULT_REQUEST_DELAY_SECONDS = 0.5
_DEFAULT_USER_AGENT = "moku-analyzer/1.0 (security research)"
_EVIDENCE_TRUNCATE_BYTES = 4096

MAX_REQUESTS_PER_HOST = int(
    os.environ.get("MOKU_ANALYZER_MAX_REQ_PER_HOST", str(_DEFAULT_MAX_REQUESTS_PER_HOST))
)
REQUEST_DELAY_SECONDS = float(
    os.environ.get("MOKU_ANALYZER_REQ_DELAY_S", str(_DEFAULT_REQUEST_DELAY_SECONDS))
)


class Executor:
    """Send test payloads to the target, hand responses to plugins."""

    def __init__(self) -> None:
        self._request_counts: dict[str, int] = {}
        self._counter_lock = threading.Lock()
        self._session = requests.Session()
        self._user_agent = os.environ.get("MOKU_ANALYZER_UA", _DEFAULT_USER_AGENT)
        self._session.headers.update({"User-Agent": self._user_agent})

    def run(
        self,
        scan_unit: ScanUnit,
        test_cases: list[TestCase],
        plugins: list[BasePlugin],
    ) -> list[Finding]:
        """Drive the per-scan loop: baseline, payload, plugin analysis."""
        findings: list[Finding] = []
        host = urlparse(scan_unit.url).hostname or ""

        baseline_body = self._fetch_baseline(scan_unit)
        baseline_unavailable = baseline_body is None
        _logger.info(
            "baseline fetched for %s (%s bytes)",
            scan_unit.url,
            0 if baseline_unavailable else len(baseline_body or ""),
        )

        for test_case in test_cases:
            with self._counter_lock:
                count = self._request_counts.get(host, 0)
                if count >= MAX_REQUESTS_PER_HOST:
                    _logger.warning("rate limit reached for %s — stopping", host)
                    break
                self._request_counts[host] = count + 1

            response_body, response_headers = self._send(scan_unit, test_case)
            if response_body is None:
                _logger.info("no response for %s — skipping", test_case.test_id)
                continue

            evidence_payload = self._build_evidence_payload(test_case, response_body)
            get_evidence_store().save(
                data=evidence_payload.encode("utf-8", errors="replace"),
                label=f"{test_case.plugin_name}_{test_case.mode.value}",
                job_id=scan_unit.meta.get("job_id"),
            )

            for plugin in plugins:
                if plugin.name != test_case.plugin_name:
                    continue
                finding = plugin.analyze_response(
                    test_case=test_case,
                    response_body=response_body,
                    response_headers=response_headers,
                    baseline_body=baseline_body or "",
                )
                if finding is None:
                    continue
                if baseline_unavailable:
                    finding.meta["baseline_unavailable"] = True
                _logger.info(
                    "finding from %s confidence=%.2f on %s",
                    finding.plugin,
                    finding.confidence,
                    test_case.test_id,
                )
                findings.append(finding)

            time.sleep(REQUEST_DELAY_SECONDS)

        return findings

    def _apply_cookies(self, cookies: dict[str, str] | None) -> None:
        if not cookies:
            return
        for key, value in cookies.items():
            self._session.cookies.set(key, value)

    def _fetch_baseline(self, scan_unit: ScanUnit) -> str | None:
        """Fetch the page with no injected payload — `None` on failure."""
        try:
            self._apply_cookies(scan_unit.cookies)
            resp = self._session.get(
                scan_unit.url,
                params=scan_unit.params,
                headers=scan_unit.headers,
                timeout=10,
            )
            return resp.text
        except requests.RequestException as exc:
            _logger.warning("baseline fetch failed for %s: %s", scan_unit.url, exc)
            return None

    def _send(
        self,
        scan_unit: ScanUnit,
        test_case: TestCase,
    ) -> tuple[str | None, dict]:
        """Inject the payload into the targeted parameter and send the request."""
        try:
            self._apply_cookies(scan_unit.cookies)
            params = dict(scan_unit.params)
            params[test_case.target_name] = test_case.payload

            resp = self._session.request(
                method=scan_unit.method,
                url=scan_unit.url,
                params=params if scan_unit.method == "GET" else None,
                data=params if scan_unit.method == "POST" else None,
                headers=scan_unit.headers,
                timeout=test_case.timeout,
            )
            return resp.text, dict(resp.headers)
        except requests.Timeout:
            _logger.info("timeout on %s", test_case.test_id)
            return None, {}
        except requests.RequestException as exc:
            _logger.warning("request failed on %s: %s", test_case.test_id, exc)
            return None, {}

    def _build_evidence_payload(self, test_case: TestCase, response_body: str) -> str:
        body_bytes = response_body.encode("utf-8", errors="replace")
        if len(body_bytes) <= _EVIDENCE_TRUNCATE_BYTES:
            response_segment = response_body
            footer = ""
        else:
            response_segment = body_bytes[:_EVIDENCE_TRUNCATE_BYTES].decode(
                "utf-8", errors="replace"
            )
            footer = (
                f"\n... [TRUNCATED {len(body_bytes)} -> {_EVIDENCE_TRUNCATE_BYTES} bytes]"
            )
        return (
            f"TEST: {test_case.test_id}\n"
            f"PAYLOAD: {test_case.payload}\n"
            f"RESPONSE:\n{response_segment}{footer}"
        )
