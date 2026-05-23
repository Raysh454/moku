"""SQLi plugin — boolean-differential + error-pattern SQL injection detection."""

import re
import uuid
from datetime import UTC, datetime

from app.core.evidence_store import get_evidence_store
from app.core.finding import Finding
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


class SQLiPlugin(BasePlugin):
    name = "sqli"

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
                    meta=dict(base_meta),
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
                    meta=dict(base_meta),
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
        body_lower = response_body.lower()

        matched_error: str | None = None
        for pattern in SQL_ERROR_PATTERNS:
            match = re.search(pattern, body_lower)
            if match:
                matched_error = match.group(0)
                break

        differential_detected = False
        meta_warning: str | None = None
        if test_case.mode == TestMode.DETECT:
            if baseline_body is not None and baseline_body != "":
                size_diff = abs(len(response_body) - len(baseline_body))
                diff_ratio = size_diff / len(baseline_body)
                if diff_ratio > _DIFF_THRESHOLD:
                    differential_detected = True
            else:
                meta_warning = "baseline_unavailable"

        if not matched_error and not differential_detected:
            return None

        if matched_error:
            idx = body_lower.find(matched_error[:20])
            snippet_start = max(0, idx - 100)
            snippet_end = min(len(response_body), idx + 300)
            snippet = response_body[snippet_start:snippet_end]
            matched_pattern = f"SQL error pattern detected: '{matched_error}'"
            confidence = 0.85
        else:
            snippet = response_body[:500]
            matched_pattern = (
                "Boolean differential detected — response size differs significantly "
                "from baseline"
            )
            confidence = 0.5

        evidence_payload = (
            f"PAYLOAD: {test_case.payload}\nRESPONSE SNIPPET:\n{snippet}"
        ).encode("utf-8", errors="replace")
        evidence_ref = get_evidence_store().save(
            data=evidence_payload,
            label=f"sqli_{test_case.mode.value}_response",
            job_id=test_case.meta.get("job_id"),
        )

        finding_meta: dict[str, object] = {
            "parameter": test_case.target_name,
            "mode": test_case.meta.get("mode"),
        }
        if meta_warning:
            finding_meta["warning"] = meta_warning

        return Finding(
            finding_id=f"sqli-{uuid.uuid4().hex[:8]}",
            plugin=self.name,
            scan_unit_url=test_case.injection_point,
            http_method="POST" if test_case.meta.get("mode") == "body" else "GET",
            payload_used=test_case.payload,
            matched_pattern=matched_pattern,
            response_snippet=snippet[:2048],
            evidence_refs=[evidence_ref],
            confidence=confidence,
            repro_steps=[
                f"1. Send request to {test_case.injection_point}",
                f"2. Set parameter '{test_case.target_name}' = '{test_case.payload}'",
                f"3. Observe: {matched_pattern}",
            ],
            timestamp=datetime.now(UTC),
            notes=f"Signal: {'SQL error' if matched_error else 'boolean differential'}",
            meta=finding_meta,
        )
