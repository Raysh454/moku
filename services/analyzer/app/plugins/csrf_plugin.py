"""CSRF plugin — heuristic detection of missing CSRF protection on POST forms."""

import logging
import uuid
from datetime import UTC, datetime
from urllib.parse import urljoin, urlparse

from bs4 import BeautifulSoup, FeatureNotFound

from app.core.evidence_store import get_evidence_store
from app.core.finding import Finding
from app.core.scan_unit import ScanUnit
from app.core.test_case import TestCase, TestMode
from app.plugins.base_plugin import BasePlugin

_logger = logging.getLogger(__name__)

CSRF_TOKEN_NAMES = [
    "csrf",
    "csrf_token",
    "csrftoken",
    "_token",
    "authenticity_token",
    "token",
    "nonce",
    "_csrf",
    "xsrf",
    "xsrf_token",
    "__requestverificationtoken",
]


class CSRFPlugin(BasePlugin):
    name = "csrf"

    def generate_tests(self, scan_unit: ScanUnit) -> list[TestCase]:
        return [
            TestCase(
                test_id=f"csrf-inspect-{uuid.uuid4().hex[:8]}",
                plugin_name=self.name,
                injection_point=scan_unit.url,
                target_name="forms",
                payload="",
                mode=TestMode.DETECT,
                timeout=10,
                meta={"job_id": scan_unit.meta.get("job_id")},
            )
        ]

    def analyze_response(
        self,
        test_case: TestCase,
        response_body: str,
        response_headers: dict,
        baseline_body: str = "",
    ) -> Finding | None:
        soup = self._parse_html(response_body)
        if soup is None:
            return None

        scanned_url = (
            test_case.injection_point
            if test_case.injection_point.startswith("http")
            else test_case.meta.get("scan_unit_url", test_case.injection_point)
        )
        scanned_origin = urlparse(scanned_url).netloc

        findings_text: list[str] = []
        vulnerable_forms: list[dict] = []

        for form in soup.find_all("form"):
            method = (form.get("method") or "get").lower()
            if method != "post":
                continue

            action = form.get("action") or scanned_url
            action_origin = urlparse(urljoin(scanned_url, action)).netloc
            if action_origin and scanned_origin and action_origin != scanned_origin:
                continue

            input_names = [
                (i.get("name") or "").lower() for i in form.find_all("input")
            ]
            has_token = any(
                any(token in name for token in CSRF_TOKEN_NAMES) for name in input_names
            )
            if not has_token:
                vulnerable_forms.append(
                    {"action": action, "inputs": [n for n in input_names if n]}
                )
                findings_text.append(
                    f"POST form to '{action}' has no CSRF token field"
                )

        cookie_header = response_headers.get("Set-Cookie", "")
        if cookie_header and "samesite" not in cookie_header.lower():
            findings_text.append("Set-Cookie header missing SameSite attribute")

        if not findings_text:
            return None

        snippet = "\n".join(findings_text)
        evidence_payload = (
            f"URL: {test_case.injection_point}\nFINDINGS:\n{snippet}"
        ).encode("utf-8", errors="replace")
        evidence_ref = get_evidence_store().save(
            data=evidence_payload,
            label="csrf_heuristic",
            job_id=test_case.meta.get("job_id"),
        )

        return Finding(
            finding_id=f"csrf-{uuid.uuid4().hex[:8]}",
            plugin=self.name,
            scan_unit_url=test_case.injection_point,
            http_method="GET",
            payload_used="(heuristic — no payload sent)",
            matched_pattern=findings_text[0],
            response_snippet=snippet[:2048],
            evidence_refs=[evidence_ref],
            confidence=0.6,
            repro_steps=[
                f"1. Visit {test_case.injection_point}",
                "2. Inspect POST forms — no CSRF token present",
                "3. A forged cross-origin POST request would be accepted",
            ],
            timestamp=datetime.now(UTC),
            notes=(
                f"Found {len(vulnerable_forms)} vulnerable form(s). "
                "Heuristic only — no requests submitted."
            ),
            meta={"vulnerable_form_count": len(vulnerable_forms)},
        )

    def _parse_html(self, response_body: str) -> BeautifulSoup | None:
        try:
            return BeautifulSoup(response_body, "lxml")
        except FeatureNotFound:
            return BeautifulSoup(response_body, "html.parser")
        except (ValueError, TypeError):
            _logger.warning("csrf: failed to parse response body, skipping")
            return None
