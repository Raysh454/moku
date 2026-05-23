"""Tests for `AdapterRegistry`."""

import pytest

from app.adapters.base import BaseAdapter
from app.adapters.registry import AdapterNotFoundError, AdapterRegistry
from app.models.schemas import Backend, Capabilities, Finding, ScanRequest


class _DummyAdapter(BaseAdapter):
    name = Backend.BUILTIN.value
    description = "dummy"

    def capabilities(self) -> Capabilities:
        return Capabilities()

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        return []


def test_has_returns_false_for_unknown():
    registry = AdapterRegistry()
    assert registry.has("nope") is False


def test_has_returns_true_after_registration():
    registry = AdapterRegistry()
    registry.register(_DummyAdapter())
    assert registry.has(Backend.BUILTIN.value) is True


def test_get_raises_adapter_not_found_error():
    registry = AdapterRegistry()
    with pytest.raises(AdapterNotFoundError) as exc_info:
        registry.get("nope")
    message = str(exc_info.value)
    assert not message.startswith("'")
    assert not message.endswith("'")


def test_available_returns_registered_names():
    registry = AdapterRegistry()
    registry.register(_DummyAdapter())
    assert registry.available() == [Backend.BUILTIN.value]
