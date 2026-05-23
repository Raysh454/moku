"""Integration test for the Nikto adapter against a real binary."""

import os
import shutil

import pytest

from app.adapters.nikto_adapter import NiktoAdapter
from app.models.schemas import Backend, ScanRequest


@pytest.mark.integration
@pytest.mark.skipif(not shutil.which("nikto"), reason="nikto binary not in PATH")
def test_nikto_runs_against_public_target():
    target = os.environ.get("MOKU_TEST_TARGET_URL", "http://scanme.nmap.org")
    adapter = NiktoAdapter()
    findings = adapter.run_scan(
        ScanRequest(url=target, backend=Backend.NIKTO)
    )
    assert isinstance(findings, list)
