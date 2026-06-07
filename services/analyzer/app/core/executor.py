"""Executor — sends test payloads, collects responses, drives plugins."""

import logging
import os
import threading
import time
from datetime import timedelta
from urllib.parse import urljoin, urlparse

import requests

from app.core.evidence_store import get_evidence_store
from app.core.finding import Finding
from app.core.scan_unit import ScanUnit
from app.core.test_case import TestCase
from app.net_guard import assert_public_host
from app.plugins.base_plugin import BasePlugin

_logger = logging.getLogger(__name__)

_DEFAULT_MAX_REQUESTS_PER_HOST = 30
_DEFAULT_REQUEST_DELAY_SECONDS = 0.5
_DEFAULT_USER_AGENT = "moku-analyzer/1.0 (security research)"
_EVIDENCE_TRUNCATE_BYTES = 4096
_MAX_REDIRECTS = 5
_REDIRECT_STATUSES = frozenset({301, 302, 303, 307, 308})

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
        max_duration: timedelta | None = None,
    ) -> list[Finding]:
        """Drive the per-scan loop: baseline, payload, plugin analysis.

        When `max_duration` is set it bounds the wall-clock budget for the
        payload loop: once the deadline passes, remaining test cases are
        skipped so an in-process scan cannot run unbounded.
        """
        findings: list[Finding] = []
        host = urlparse(scan_unit.url).hostname or ""
        deadline = None
        if max_duration is not None and max_duration.total_seconds() > 0:
            deadline = time.monotonic() + max_duration.total_seconds()

        baseline_body = self._fetch_baseline(scan_unit)
        baseline_unavailable = baseline_body is None
        _logger.info(
            "baseline fetched for %s (%s bytes)",
            scan_unit.url,
            0 if baseline_unavailable else len(baseline_body or ""),
        )

        for test_case in test_cases:
            if deadline is not None and time.monotonic() >= deadline:
                _logger.warning("scan deadline reached for %s — stopping", host)
                break
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

    def _guarded_request(
        self, method: str, url: str, **kwargs
    ) -> requests.Response:
        """Send a request, following redirects only to vetted public hosts.

        Automatic redirect following is disabled and each ``Location`` is
        re-validated with the shared SSRF guard before it is followed. This
        closes the redirect-to-internal-host bypass (a public target that
        302s to cloud metadata or an RFC1918 service). After the first hop the
        redirect is followed as a plain GET so injected payloads are never
        replayed to a different origin. Raises ``requests.RequestException``
        when a redirect targets a disallowed host or the chain is too long.
        """
        kwargs["allow_redirects"] = False
        current = url
        for _ in range(_MAX_REDIRECTS + 1):
            resp = self._session.request(method=method, url=current, **kwargs)
            if resp.status_code not in _REDIRECT_STATUSES:
                return resp
            location = resp.headers.get("Location")
            if not location:
                return resp
            nxt = urljoin(current, location)
            parsed = urlparse(nxt)
            if parsed.scheme not in {"http", "https"} or not parsed.hostname:
                raise requests.RequestException(
                    f"blocked redirect to unsafe location: {location!r}"
                )
            try:
                assert_public_host(parsed.hostname)
            except ValueError as exc:
                raise requests.RequestException(
                    f"blocked redirect to disallowed host: {parsed.hostname}"
                ) from exc
            current = nxt
            method = "GET"
            kwargs.pop("data", None)
            kwargs.pop("params", None)
        raise requests.RequestException(f"too many redirects (>{_MAX_REDIRECTS})")

    def _fetch_baseline(self, scan_unit: ScanUnit) -> str | None:
        """Fetch the page with no injected payload — `None` on failure."""
        try:
            self._apply_cookies(scan_unit.cookies)
            resp = self._guarded_request(
                "GET",
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

            resp = self._guarded_request(
                scan_unit.method,
                scan_unit.url,
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
