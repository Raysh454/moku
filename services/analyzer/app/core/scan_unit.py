"""ScanUnit — the input model the dynamic analyzer hands to plugins."""

from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


class ScanUnitType(str, Enum):
    URL = "url"
    FORM = "form"
    PARAM = "param"


class ParamLocation(str, Enum):
    QUERY = "query"
    FORM = "form"
    HEADER = "header"
    COOKIE = "cookie"
    PATH = "path"


class FormInput(BaseModel):
    name: str
    input_type: str
    sample: str | None = None
    sensitive: bool = False


class ScanUnit(BaseModel):
    type: ScanUnitType
    url: str
    method: str = "GET"
    params: dict[str, str] = Field(default_factory=dict)
    headers: dict[str, str] = Field(default_factory=dict)
    cookies: dict[str, str] = Field(default_factory=dict)
    snapshot_id: str | None = None
    auth_required: bool = False

    form_id: str | None = None
    form_action: str | None = None
    inputs: list[FormInput] = Field(default_factory=list)

    parameter_name: str | None = None
    location: ParamLocation | None = None
    sample_value: str | None = None

    plugins: list[str] = Field(default_factory=list)
    allow_aggressive: bool = False
    headless_preference: str = "auto"

    meta: dict[str, Any] = Field(default_factory=dict)
