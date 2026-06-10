"""PluginManager — collects TestCases from the registered plugins (pure)."""

import logging

from app.core.scan_unit import ScanUnit
from app.core.test_case import TestCase
from app.plugins.base_plugin import BasePlugin
from app.plugins.csrf_plugin import CSRFPlugin
from app.plugins.sqli_plugin import SQLiPlugin
from app.plugins.xss_plugin import XSSPlugin

_logger = logging.getLogger(__name__)


class PluginManager:
    def __init__(self) -> None:
        self._plugins: list[BasePlugin] = [
            XSSPlugin(),
            SQLiPlugin(),
            CSRFPlugin(),
        ]

    def generate_tests(self, scan_unit: ScanUnit) -> list[TestCase]:
        tests: list[TestCase] = []
        for plugin in self._plugins:
            if scan_unit.plugins and plugin.name not in scan_unit.plugins:
                continue
            plugin_tests = plugin.generate_tests(scan_unit)
            tests.extend(plugin_tests)
            _logger.debug("%s generated %d tests", plugin.name, len(plugin_tests))
        return tests

    def get_plugins(self) -> list[BasePlugin]:
        return self._plugins
