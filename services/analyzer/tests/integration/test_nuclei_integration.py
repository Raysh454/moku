"""Integration test for the Nuclei adapter against a real binary."""

import os
import shutil

import pytest

from app.adapters.nuclei_adapter import NucleiAdapter
from app.models.schemas import Backend, ScanRequest


@pytest.mark.integration
@pytest.mark.skipif(not shutil.which("nuclei"), reason="nuclei binary not in PATH")
def test_nuclei_runs_against_public_target():
    target = os.environ.get("MOKU_TEST_TARGET_URL", "http://scanme.nmap.org")
    adapter = NucleiAdapter()
    findings = adapter.run_scan(
        ScanRequest(url=target, backend=Backend.NUCLEI)
    )
    assert isinstance(findings, list)
