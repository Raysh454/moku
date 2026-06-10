"""BaseAdapter — contract every vulnerability scanner adapter implements."""

from abc import ABC, abstractmethod
from typing import ClassVar

from app.models.schemas import Capabilities, Finding, ScanRequest


class BaseAdapter(ABC):
    """Adapter contract: validate a request, run a scan, report capabilities."""

    name: ClassVar[str] = "base"
    description: ClassVar[str] = ""

    @abstractmethod
    def run_scan(self, request: ScanRequest) -> list[Finding]:
        """Execute a scan for `request` and return zero or more findings."""

    @abstractmethod
    def capabilities(self) -> Capabilities:
        """Describe what this adapter can do."""
