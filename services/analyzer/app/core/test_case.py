"""TestCase — a single test the Executor sends against a target."""

from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


class TestMode(str, Enum):
    __test__ = False

    DETECT = "detect"
    CONFIRM = "confirm"


class TestCase(BaseModel):
    __test__ = False

    test_id: str
    plugin_name: str
    injection_point: str
    target_name: str
    payload: str
    marker: str | None = None
    mode: TestMode = TestMode.DETECT
    require_headless: bool = False
    timeout: int = 10
    retries: int = 1

    meta: dict[str, Any] = Field(default_factory=dict)
