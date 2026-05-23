"""Adapter registry — stores scanner adapters keyed by their `name`."""

import logging

from app.adapters.base import BaseAdapter

_logger = logging.getLogger(__name__)


class AdapterNotFoundError(Exception):
    """Raised when a requested adapter name is not registered."""


class AdapterRegistry:
    """In-memory map from `adapter.name` to a `BaseAdapter` instance."""

    def __init__(self) -> None:
        self._adapters: dict[str, BaseAdapter] = {}

    def register(self, adapter: BaseAdapter) -> None:
        self._adapters[adapter.name] = adapter
        _logger.info("registered adapter: %s", adapter.name)

    def get(self, name: str) -> BaseAdapter:
        adapter = self._adapters.get(name)
        if adapter is None:
            raise AdapterNotFoundError(
                f"backend {name!r} is not registered. "
                f"Available: {sorted(self._adapters)}"
            )
        return adapter

    def has(self, name: str) -> bool:
        return name in self._adapters

    def available(self) -> list[str]:
        return list(self._adapters.keys())

    def unregister(self, name: str) -> None:
        self._adapters.pop(name, None)


registry = AdapterRegistry()
