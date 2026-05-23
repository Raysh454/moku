"""Integration test for the ZAP adapter against a real binary."""

import os
import shutil

import pytest

from app.adapters.zap_adapter import ZAPAdapter
from app.models.schemas import Backend, ScanRequest


@pytest.mark.integration
@pytest.mark.skipif(not shutil.which("zap.sh"), reason="zap.sh binary not in PATH")
def test_zap_runs_against_public_target():
    target = os.environ.get("MOKU_TEST_TARGET_URL", "http://scanme.nmap.org")
    adapter = ZAPAdapter()
    findings = adapter.run_scan(
        ScanRequest(url=target, backend=Backend.ZAP)
    )
    assert isinstance(findings, list)
