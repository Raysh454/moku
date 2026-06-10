"""CliScannerAdapter — shared base for adapters that shell out to a binary.

The CLI-backed scanners (nikto, nuclei, zap) all run a single external process
and ignore auth/scope/profile, so they share one ``capabilities()`` declaration
and one ``max_duration``-aware timeout policy. Concrete adapters supply
``name``/``description`` and implement ``run_scan`` (the subprocess invocation
and output parsing genuinely differ per tool — nikto/nuclei parse stdout, zap
reads a JSON report file).
"""

import math
from datetime import timedelta
from typing import ClassVar

from app.adapters.base import BaseAdapter
from app.models.schemas import Capabilities

_CLI_ADAPTER_VERSION = "0.1.0"


class CliScannerAdapter(BaseAdapter):
    """Base for subprocess-backed scanner adapters."""

    #: Fallback wall-clock budget (seconds) when the request omits max_duration.
    default_timeout_seconds: ClassVar[int] = 300

    def capabilities(self) -> Capabilities:
        return Capabilities(
            async_=True,
            supports_auth=False,
            supports_scope=False,
            supports_scan_profile=False,
            max_concurrent_scans=1,
            version=_CLI_ADAPTER_VERSION,
        )

    def _timeout_seconds(self, max_duration: timedelta | None) -> int:
        """Honor ScanRequest.max_duration as the subprocess timeout.

        Rounds UP so a sub-second budget (e.g. 500ms) becomes a 1s cap rather
        than truncating to 0 and silently falling back to the (large) default.
        Falls back to ``default_timeout_seconds`` only when the request omits a
        duration or supplies a non-positive one.
        """
        if max_duration is None:
            return self.default_timeout_seconds
        total = max_duration.total_seconds()
        if total <= 0:
            return self.default_timeout_seconds
        return max(1, math.ceil(total))
