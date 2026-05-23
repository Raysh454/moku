"""Convenience launcher for the analyzer service.

The default bind address is `127.0.0.1`. When binding to any non-loopback
interface you must also set `MOKU_ANALYZER_TOKEN` to require the shared
secret on inbound requests.
"""

import os

import uvicorn

if __name__ == "__main__":
    host = os.environ.get("MOKU_ANALYZER_HOST", "127.0.0.1")
    port = int(os.environ.get("MOKU_ANALYZER_PORT", "8080"))
    uvicorn.run("main:app", host=host, port=port, reload=False)
