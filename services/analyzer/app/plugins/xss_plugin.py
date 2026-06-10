"""XSS plugin — detects reflected Cross-Site Scripting via unique markers."""

import secrets
from datetime import UTC, datetime

from app.core.evidence_store import get_evidence_store
from app.core.finding import Finding, make_finding_id
from app.core.scan_unit import ScanUnit, ScanUnitType
from app.core.test_case import TestCase, TestMode
from app.plugins.base_plugin import BasePlugin


class XSSPlugin(BasePlugin):
    name = "xss"

    def generate_tests(self, scan_unit: ScanUnit) -> list[TestCase]:
        targets: dict[str, str] = {}

        if scan_unit.type == ScanUnitType.PARAM and scan_unit.parameter_name:
            targets[scan_unit.parameter_name] = scan_unit.sample_value or ""
        else:
            targets.update(scan_unit.params)

        mode_label = "query" if scan_unit.method.upper() == "GET" else "body"
        tests: list[TestCase] = []

        for param_name, _ in targets.items():
            marker = f"__moku_xss_{secrets.token_hex(16)}"

            tests.append(
                TestCase(
                    test_id=f"xss-detect-{param_name}-{marker}",
                    plugin_name=self.name,
                    injection_point=scan_unit.url,
                    target_name=param_name,
                    payload=f"<{marker}>",
                    marker=marker,
                    mode=TestMode.DETECT,
                    timeout=10,
                    meta={"parameter": param_name, "mode": mode_label},
                )
            )
            tests.append(
                TestCase(
                    test_id=f"xss-confirm-{param_name}-{marker}",
                    plugin_name=self.name,
                    injection_point=scan_unit.url,
                    target_name=param_name,
                    payload=f'"><script>alert("{marker}")</script>',
                    marker=marker,
                    mode=TestMode.CONFIRM,
                    timeout=10,
                    meta={"parameter": param_name, "mode": mode_label},
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
        if not test_case.marker:
            return None

        marker = test_case.marker

        if test_case.mode == TestMode.DETECT:
            if f"<{marker}>" not in response_body:
                return None
            if f"&lt;{marker}&gt;" in response_body:
                return None
        elif test_case.mode == TestMode.CONFIRM:
            if marker not in response_body:
                return None
            if "script" not in response_body.lower():
                return None

        idx = response_body.find(marker)
        snippet_start = max(0, idx - 100)
        snippet_end = min(len(response_body), idx + 200)
        snippet = response_body[snippet_start:snippet_end]

        evidence_payload = (
            f"PAYLOAD: {test_case.payload}\nRESPONSE SNIPPET:\n{snippet}"
        ).encode("utf-8", errors="replace")
        evidence_ref = get_evidence_store().save(
            data=evidence_payload,
            label=f"xss_{test_case.mode.value}_response",
            job_id=test_case.meta.get("job_id"),
        )

        confidence = 0.4 if test_case.mode == TestMode.DETECT else 0.85

        return Finding(
            finding_id=make_finding_id("xss"),
            plugin=self.name,
            scan_unit_url=test_case.injection_point,
            http_method="POST" if test_case.meta.get("mode") == "body" else "GET",
            payload_used=test_case.payload,
            matched_pattern=f"Unescaped marker <{marker}> found in response",
            response_snippet=snippet[:2048],
            evidence_refs=[evidence_ref],
            confidence=confidence,
            repro_steps=[
                f"1. Send request to {test_case.injection_point}",
                f"2. Set parameter '{test_case.target_name}' = '{test_case.payload}'",
                "3. Observe unescaped reflection in response body",
            ],
            timestamp=datetime.now(UTC),
            notes=f"Stage: {test_case.mode.value}. Marker: {marker}",
            meta={
                "parameter": test_case.target_name,
                "mode": test_case.meta.get("mode"),
            },
        )
