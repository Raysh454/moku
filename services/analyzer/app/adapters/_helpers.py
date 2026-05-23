"""Shared adapter helpers: URL guards, subprocess wrappers, tempdir builders."""

import ipaddress
import logging
import shutil
import socket
import subprocess
import tempfile
from pathlib import Path
from urllib.parse import urlparse

from app.models.schemas import (
    _ALLOW_PRIVATE_HOSTS_ENV_VAR,
    _private_host_bypass_enabled,
)

logger = logging.getLogger(__name__)


def validate_target_url(url: str) -> str:
    """Validate a target URL, rejecting unsafe schemes and private addresses.

    Honors the `MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS` env flag so adapter-side
    validation matches the request-level guard in `app.models.schemas`. When
    the bypass is active, loopback/RFC1918 addresses are permitted with a
    warning logged; scheme/hostname/argument-injection checks still apply.
    """
    if not url or not isinstance(url, str):
        raise ValueError("url must be a non-empty string")
    if url.startswith("-"):
        raise ValueError("url cannot start with '-'")

    parsed = urlparse(url)
    if parsed.scheme not in {"http", "https"}:
        raise ValueError(f"unsupported url scheme: {parsed.scheme!r}")

    host = parsed.hostname
    if not host:
        raise ValueError("url must include a hostname")

    if _private_host_bypass_enabled():
        logger.warning(
            "adapter SSRF private-host guard bypassed for host %r via %s env flag",
            host,
            _ALLOW_PRIVATE_HOSTS_ENV_VAR,
        )
        return url

    candidates: list[str] = []
    try:
        ipaddress.ip_address(host)
        candidates.append(host)
    except ValueError:
        try:
            infos = socket.getaddrinfo(host, None)
        except socket.gaierror as exc:
            raise ValueError(f"failed to resolve host {host!r}: {exc}") from exc
        candidates.extend(info[4][0] for info in infos if info[4])

    for candidate in candidates:
        try:
            addr = ipaddress.ip_address(candidate)
        except ValueError:
            continue
        if (
            addr.is_private
            or addr.is_loopback
            or addr.is_link_local
            or addr.is_reserved
            or addr.is_multicast
            or addr.is_unspecified
        ):
            raise ValueError(
                f"host {host!r} resolves to a disallowed address: {candidate}"
            )

    return url


def run_subprocess(cmd: list[str], *, timeout: int, name: str) -> str:
    """Run an external scanner binary; raise `RuntimeError` on any failure."""
    try:
        result = subprocess.run(
            cmd,
            check=False,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except FileNotFoundError as exc:
        raise RuntimeError(f"{name} not found") from exc
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError(f"{name} timed out") from exc

    if result.returncode != 0:
        raise RuntimeError(f"{name} exited with code {result.returncode}")
    return result.stdout


def safe_temp_path(prefix: str) -> Path:
    """Return a freshly created temporary directory; caller cleans up."""
    return Path(tempfile.mkdtemp(prefix=prefix))


def remove_temp_path(path: Path) -> None:
    """Best-effort cleanup for a temp directory."""
    shutil.rmtree(path, ignore_errors=True)
