"""Conformance: Python adapter capabilities match the shared manifest.

The manifest at internal/analyzer/testdata/capabilities.json is the single
source of truth for sidecar adapter capabilities. The Go client is checked
against it by TestSidecarCapabilities_MatchSharedManifest; this is the Python
half, so the two sides cannot drift.
"""

import json
from pathlib import Path

import pytest

from app.adapters.registry import AdapterRegistry
from app.app_factory import register_default_adapters

_MANIFEST_PATH = (
    Path(__file__).resolve().parents[3]
    / "internal"
    / "analyzer"
    / "testdata"
    / "capabilities.json"
)


def _load_manifest() -> dict[str, dict]:
    data = json.loads(_MANIFEST_PATH.read_text(encoding="utf-8"))
    return {name: entry for name, entry in data.items() if name != "_comment"}


def _registered_adapters() -> AdapterRegistry:
    registry = AdapterRegistry()
    register_default_adapters(registry)
    return registry


@pytest.mark.parametrize("name", sorted(_load_manifest()))
def test_adapter_capabilities_match_manifest(name):
    want = _load_manifest()[name]
    adapter = _registered_adapters().get(name)
    caps = adapter.capabilities()
    assert caps.async_ == want["async"], name
    assert caps.supports_auth == want["supports_auth"], name
    assert caps.supports_scope == want["supports_scope"], name
    assert caps.supports_scan_profile == want["supports_scan_profile"], name
    assert caps.max_concurrent_scans == want["max_concurrent_scans"], name


def test_manifest_covers_every_registered_adapter():
    manifest = _load_manifest()
    registered = set(_registered_adapters().available())
    assert registered == set(manifest), (registered, set(manifest))
