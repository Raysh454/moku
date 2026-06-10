"""SQLi plugin — boolean-differential + error-pattern SQL injection detection.

Boolean-blind detection correlates the always-true (`' OR '1'='1`) and
always-false (`' OR '1'='2`) responses for the same parameter: a finding is
raised only when those two responses diverge materially. Comparing each
payload against the baseline instead would false-positive on any endpoint that
merely reflects the parameter (the equal-length true/false payloads inflate the
body identically). Correlation state is kept per plugin instance; the builtin
adapter constructs a fresh plugin set per scan, so the state is scan-scoped and
needs no locking.
"""

import re
import uuid
from datetime import UTC, datetime

from app.core.evidence_store import get_evidence_store
from app.core.finding import Finding, make_finding_id
from app.core.scan_unit import ScanUnit, ScanUnitType
from app.core.test_case import TestCase, TestMode
from app.plugins.base_plugin import BasePlugin

SQL_ERROR_PATTERNS = [
    r"sql syntax.*mysql",
    r"warning.*mysql_",
    r"unclosed quotation mark",
    r"quoted string not properly terminated",
    r"ora-\d{4,5}",
    r"sqlite3?\.",
    r"sqlstate\[",
    r"pg_query\(\)",
    r"supplied argument is not a valid mysql",
    r"you have an error in your sql",
    r"microsoft.*odbc.*sql",
    r"jdbc\..*exception",
]

_DIFF_THRESHOLD = 0.2
_ERROR_CONFIDENCE = 0.85
_DIFFERENTIAL_CONFIDENCE = 0.5


class SQLiPlugin(BasePlugin):
    name = "sqli"

    def __init__(self) -> None:
        # marker -> {"variant", "body", "payload"} for the first-seen half of a
        # true/false pair, awaiting its sibling. Scan-scoped (fresh plugin per
        # scan), so single-threaded access — no lock required.
        self._pending: dict[str, dict[str, str]] = {}

    def generate_tests(self, scan_unit: ScanUnit) -> list[TestCase]:
        targets: dict[str, str] = {}
        if scan_unit.type == ScanUnitType.PARAM and scan_unit.parameter_name:
            targets[scan_unit.parameter_name] = scan_unit.sample_value or "1"
        else:
            targets.update(scan_unit.params)

        mode_label = "query" if scan_unit.method.upper() == "GET" else "body"
        tests: list[TestCase] = []

        for param_name, sample_value in targets.items():
            marker = uuid.uuid4().hex[:8]
            base_meta = {"parameter": param_name, "mode": mode_label}

            tests.append(
                TestCase(
                    test_id=f"sqli-true-{param_name}-{marker}",
                    plugin_name=self.name,
                    injection_point=scan_unit.url,
                    target_name=param_name,
                    payload=f"{sample_value}' OR '1'='1",
                    marker=marker,
                    mode=TestMode.DETECT,
                    timeout=10,
                    meta={**base_meta, "sqli_variant": "true"},
                )
            )
            tests.append(
                TestCase(
                    test_id=f"sqli-false-{param_name}-{marker}",
                    plugin_name=self.name,
                    injection_point=scan_unit.url,
                    target_name=param_name,
                    payload=f"{sample_value}' OR '1'='2",
                    marker=marker,
                    mode=TestMode.DETECT,
                    timeout=10,
                    meta={**base_meta, "sqli_variant": "false"},
                )
            )
            tests.append(
                TestCase(
                    test_id=f"sqli-error-{param_name}-{marker}",
                    plugin_name=self.name,
                    injection_point=scan_unit.url,
                    target_name=param_name,
                    payload=f"{sample_value}'",
                    marker=marker,
                    mode=TestMode.CONFIRM,
                    timeout=10,
                    meta=dict(base_meta),
                )
            )

        return tests

    def analyze_response(
        self,
        test_case: TestCase,
        response_body: str,
        response_headers: dict,
        baseline_body: str = "",
    ) -> Finding | None:
        # 1) SQL error signature — independent, high-confidence, emit immediately.
        matched_error = self._match_sql_error(response_body)
        if matched_error:
            return self._build_finding(
                test_case=test_case,
                payload=test_case.payload,
                snippet=self._error_snippet(response_body, matched_error),
                matched_pattern=f"SQL error pattern detected: '{matched_error}'",
                confidence=_ERROR_CONFIDENCE,
                signal="SQL error",
            )

        # 2) Boolean-blind: correlate the always-true and always-false responses.
        variant = test_case.meta.get("sqli_variant")
        if (
            test_case.mode == TestMode.DETECT
            and variant in ("true", "false")
            and test_case.marker
        ):
            pending = self._pending.pop(test_case.marker, None)
            if pending is None:
                # First half of the pair — stash and wait for its sibling.
                self._pending[test_case.marker] = {
                    "variant": variant,
                    "body": response_body,
                    "payload": test_case.payload,
                }
                return None

            if variant == "true":
                true_body, true_payload = response_body, test_case.payload
                false_body = pending["body"]
            else:
                true_body, true_payload = pending["body"], pending["payload"]
                false_body = response_body

            if self._responses_diverge(true_body, false_body):
                return self._build_finding(
                    test_case=test_case,
                    payload=true_payload,
                    snippet=true_body[:500],
                    matched_pattern=(
                        "Boolean differential detected — the always-true and "
                        "always-false payloads produced materially different responses"
                    ),
                    confidence=_DIFFERENTIAL_CONFIDENCE,
                    signal="boolean differential",
                )

        return None

    def _match_sql_error(self, response_body: str) -> str | None:
        body_lower = response_body.lower()
        for pattern in SQL_ERROR_PATTERNS:
            match = re.search(pattern, body_lower)
            if match:
                return match.group(0)
        return None

    def _error_snippet(self, response_body: str, matched_error: str) -> str:
        idx = response_body.lower().find(matched_error[:20])
        start = max(0, idx - 100)
        end = min(len(response_body), idx + 300)
        return response_body[start:end]

    @staticmethod
    def _responses_diverge(true_body: str, false_body: str) -> bool:
        largest = max(len(true_body), len(false_body), 1)
        return abs(len(true_body) - len(false_body)) / largest > _DIFF_THRESHOLD

    def _build_finding(
        self,
        test_case: TestCase,
        payload: str,
        snippet: str,
        matched_pattern: str,
        confidence: float,
        signal: str,
    ) -> Finding:
        evidence_payload = (
            f"PAYLOAD: {payload}\nRESPONSE SNIPPET:\n{snippet}"
        ).encode("utf-8", errors="replace")
        evidence_ref = get_evidence_store().save(
            data=evidence_payload,
            label=f"sqli_{test_case.mode.value}_response",
            job_id=test_case.meta.get("job_id"),
        )
        return Finding(
            finding_id=make_finding_id("sqli"),
            plugin=self.name,
            scan_unit_url=test_case.injection_point,
            http_method="POST" if test_case.meta.get("mode") == "body" else "GET",
            payload_used=payload,
            matched_pattern=matched_pattern,
            response_snippet=snippet[:2048],
            evidence_refs=[evidence_ref],
            confidence=confidence,
            repro_steps=[
                f"1. Send request to {test_case.injection_point}",
                f"2. Set parameter '{test_case.target_name}' = '{payload}'",
                f"3. Observe: {matched_pattern}",
            ],
            timestamp=datetime.now(UTC),
            notes=f"Signal: {signal}",
            meta={
                "parameter": test_case.target_name,
                "mode": test_case.meta.get("mode"),
            },
        )
