"""Tests for the SQLi plugin."""

from app.core.scan_unit import ScanUnit, ScanUnitType
from app.plugins.sqli_plugin import SQLiPlugin


def _scan_unit() -> ScanUnit:
    return ScanUnit(
        type=ScanUnitType.URL,
        url="https://example.com/items",
        params={"id": "1"},
    )


def _detect_pair(plugin: SQLiPlugin):
    tests = plugin.generate_tests(_scan_unit())
    true_tc = next(t for t in tests if t.test_id.startswith("sqli-true-"))
    false_tc = next(t for t in tests if t.test_id.startswith("sqli-false-"))
    return true_tc, false_tc


def test_injection_point_is_full_url():
    plugin = SQLiPlugin()
    tests = plugin.generate_tests(_scan_unit())
    assert all(t.injection_point == "https://example.com/items" for t in tests)


def test_error_pattern_is_detected_with_high_confidence():
    plugin = SQLiPlugin()
    true_tc, _ = _detect_pair(plugin)
    finding = plugin.analyze_response(
        test_case=true_tc,
        response_body="You have an error in your SQL syntax",
        response_headers={},
        baseline_body="the normal page",
    )
    assert finding is not None
    assert finding.confidence == 0.85
    assert finding.timestamp.tzinfo is not None


def test_first_of_pair_is_stashed_and_returns_none():
    plugin = SQLiPlugin()
    true_tc, _ = _detect_pair(plugin)
    assert plugin.analyze_response(true_tc, "rows rows rows", {}, baseline_body="page") is None


def test_no_finding_when_true_and_false_responses_match():
    # Reflecting / non-vulnerable endpoint: the equal-length true/false payloads
    # yield equal-length responses, so there is NO boolean differential — even
    # though both responses differ from the baseline (they reflect the payload).
    plugin = SQLiPlugin()
    true_tc, false_tc = _detect_pair(plugin)
    baseline = "<html>results for 1</html>"
    true_body = f"<html>results for {true_tc.payload}</html>"
    false_body = f"<html>results for {false_tc.payload}</html>"
    assert plugin.analyze_response(true_tc, true_body, {}, baseline_body=baseline) is None
    assert plugin.analyze_response(false_tc, false_body, {}, baseline_body=baseline) is None


def test_finding_when_true_and_false_responses_diverge():
    # Vulnerable boolean-blind: the always-true payload returns rows, the
    # always-false payload returns none -> the responses diverge -> finding.
    plugin = SQLiPlugin()
    true_tc, false_tc = _detect_pair(plugin)
    baseline = "<html>results for 1</html>"
    true_body = "<tr>row</tr>" * 50
    false_body = "no results"
    assert plugin.analyze_response(true_tc, true_body, {}, baseline_body=baseline) is None
    finding = plugin.analyze_response(false_tc, false_body, {}, baseline_body=baseline)
    assert finding is not None
    assert finding.confidence == 0.5
    assert finding.payload_used == true_tc.payload  # attributed to the true payload
    assert finding.timestamp.tzinfo is not None


def test_state_is_per_instance_not_shared():
    # Two plugin instances (per-scan) must not correlate each other's responses.
    plugin_a = SQLiPlugin()
    plugin_b = SQLiPlugin()
    true_a, _ = _detect_pair(plugin_a)
    true_b, false_b = _detect_pair(plugin_b)
    assert plugin_a.analyze_response(true_a, "x" * 500, {}, baseline_body="p") is None
    # plugin_b has only seen nothing yet; its own pair must correlate independently.
    assert plugin_b.analyze_response(true_b, "y" * 10, {}, baseline_body="p") is None
    finding = plugin_b.analyze_response(false_b, "y" * 500, {}, baseline_body="p")
    assert finding is not None  # b's own true/false diverged
