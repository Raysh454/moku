"""Tests for the CliScannerAdapter timeout policy (max_duration mapping)."""

from datetime import timedelta

from app.adapters.cli_scanner import CliScannerAdapter


class _Probe(CliScannerAdapter):
    name = "probe"
    description = "probe"
    default_timeout_seconds = 300

    def run_scan(self, request):  # pragma: no cover - not exercised here
        return []


def test_none_falls_back_to_default():
    assert _Probe()._timeout_seconds(None) == 300


def test_non_positive_falls_back_to_default():
    assert _Probe()._timeout_seconds(timedelta(seconds=0)) == 300


def test_sub_second_rounds_up_to_one_not_floored_to_default():
    # Regression: int(0.5)==0 used to fall back to the 300s default.
    assert _Probe()._timeout_seconds(timedelta(milliseconds=500)) == 1


def test_fractional_seconds_round_up():
    assert _Probe()._timeout_seconds(timedelta(seconds=1.2)) == 2


def test_whole_seconds_pass_through():
    assert _Probe()._timeout_seconds(timedelta(seconds=45)) == 45
