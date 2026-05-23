"""
BasePlugin — every vulnerability plugin must extend this.
Two responsibilities only:
  1. generate_tests() — pure, no network, just build TestCase objects
  2. analyze_response() — look at response, return Finding or None
"""
from abc import ABC, abstractmethod

from app.core.finding import Finding
from app.core.scan_unit import ScanUnit
from app.core.test_case import TestCase


class BasePlugin(ABC):
    name: str = "base"

    @abstractmethod
    def generate_tests(self, scan_unit: ScanUnit) -> list[TestCase]:
        """
        Pure function — no network calls.
        Returns list of TestCase objects to run against the target.
        """
        ...

    @abstractmethod
    def analyze_response(
        self,
        test_case: TestCase,
        response_body: str,
        response_headers: dict,
        baseline_body: str = "",
    ) -> Finding | None:
        """
        Look at the response and decide if a vulnerability exists.
        Returns a Finding if confirmed, None if not.
        """
        ...