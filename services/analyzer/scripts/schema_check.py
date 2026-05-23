"""Validate sidecar JSON fixtures against the ScanResult schema.

Used by `make schema-check` to verify that the Go-side test fixtures
(internal/analyzer/testdata/sidecar/*.json) still satisfy the Pydantic
contract exposed by services/analyzer/app/models/schemas.py.

Exits 0 on success (including the no-fixtures case), non-zero on the
first validation failure with a printed traceback-style message.
"""

from __future__ import annotations

import glob
import os
import sys
from pathlib import Path

from app.models.schemas import ScanResult


def _fixture_glob() -> str:
    here = Path(__file__).resolve()
    repo_root = here.parents[3]  # services/analyzer/scripts/ -> repo root
    return str(repo_root / "internal" / "analyzer" / "testdata" / "sidecar" / "*.json")


def main() -> int:
    pattern = _fixture_glob()
    fixtures = sorted(glob.glob(pattern))
    if not fixtures:
        print(f"no fixtures found at {pattern}; skipping")
        return 0
    for path in fixtures:
        try:
            with open(path, encoding="utf-8") as fh:
                ScanResult.model_validate_json(fh.read())
        except Exception as exc:  # noqa: BLE001 -- want any failure to surface
            print(f"FAIL {os.path.basename(path)}: {exc}", file=sys.stderr)
            return 1
        print(f"OK {os.path.basename(path)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
