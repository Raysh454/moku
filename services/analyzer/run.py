"""Convenience launcher for the analyzer service.

The default bind address is `127.0.0.1:8181`. When binding to any non-loopback
interface you must also set `MOKU_ANALYZER_TOKEN` to require the shared secret
on inbound requests; startup fails closed otherwise.
"""

import os

import uvicorn

from app.app_factory import enforce_startup_security_posture

if __name__ == "__main__":
    host = os.environ.get("MOKU_ANALYZER_HOST", "127.0.0.1")
    port = int(os.environ.get("MOKU_ANALYZER_PORT", "8181"))
    enforce_startup_security_posture()
    uvicorn.run("main:app", host=host, port=port, reload=False)
