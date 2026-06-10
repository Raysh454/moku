"""Token-based authentication dependency for the analyzer API."""

import hmac
import os

from fastapi import Header, HTTPException, status

_INTERNAL_TOKEN_ENV = "MOKU_ANALYZER_TOKEN"
_HEADER_NAME = "X-Moku-Token"


def require_internal_token(
    x_moku_token: str = Header(default="", alias=_HEADER_NAME),
) -> None:
    """Validate the shared-secret header used by Moku to authenticate requests.

    When `MOKU_ANALYZER_TOKEN` is unset the dependency is a no-op so local
    development works without ceremony. When it is set we require the header
    to match exactly using constant-time comparison.
    """
    expected = os.environ.get(_INTERNAL_TOKEN_ENV)
    if not expected:
        return
    if not hmac.compare_digest(x_moku_token, expected):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid or missing token",
        )
