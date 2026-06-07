"""Centralised SSRF guard for analyzer target hosts.

Single source of truth for rejecting private / loopback / link-local /
reserved / multicast / unspecified destinations. Imported by the request-model
validator (:mod:`app.models.schemas`), the adapter URL helper
(:mod:`app.adapters._helpers`), and the Shodan adapter so the trust boundary
is defined exactly once.

Honours ``MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS`` for local development and
automated verification against the demo server (``localhost:9999``).
Production deployments MUST leave it unset.

This module intentionally depends on nothing else in the package so both the
models layer and the adapters layer can import it without a cycle.
"""

from __future__ import annotations

import ipaddress
import logging
import os
import socket

logger = logging.getLogger(__name__)

_ALLOW_PRIVATE_HOSTS_ENV_VAR = "MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS"
_TRUTHY_ENV_VALUES = frozenset({"1", "true", "yes"})

_IPAddress = ipaddress.IPv4Address | ipaddress.IPv6Address


def private_host_bypass_enabled() -> bool:
    """Return True iff the SSRF private-host bypass env flag is set truthy."""
    raw = os.getenv(_ALLOW_PRIVATE_HOSTS_ENV_VAR)
    if raw is None:
        return False
    return raw.strip().lower() in _TRUTHY_ENV_VALUES


def is_disallowed_address(addr: _IPAddress) -> bool:
    """Return True for any address an outbound scan must never reach.

    IPv4-mapped IPv6 addresses (``::ffff:a.b.c.d``) are unwrapped to their
    embedded IPv4 form first, so a mapped loopback/link-local address such as
    ``::ffff:169.254.169.254`` is judged by the address it actually reaches
    rather than slipping through the IPv6 predicates.
    """
    if isinstance(addr, ipaddress.IPv6Address) and addr.ipv4_mapped is not None:
        addr = addr.ipv4_mapped
    # `not is_global` is the catch-all: it also covers ranges the explicit
    # predicates miss, notably CGNAT 100.64.0.0/10 (RFC 6598) and the
    # benchmarking/documentation reserved blocks. The explicit checks are kept
    # for readability and as belt-and-suspenders.
    return (
        addr.is_private
        or addr.is_loopback
        or addr.is_link_local
        or addr.is_reserved
        or addr.is_multicast
        or addr.is_unspecified
        or not addr.is_global
    )


def resolve_host_addresses(host: str) -> list[str]:
    """Resolve `host` to candidate IP strings (the literal if already an IP).

    Raises ``ValueError`` when DNS resolution fails.
    """
    try:
        ipaddress.ip_address(host)
        return [host]
    except ValueError:
        pass
    try:
        infos = socket.getaddrinfo(host, None)
    except socket.gaierror as exc:
        raise ValueError(f"failed to resolve host {host!r}: {exc}") from exc
    return [info[4][0] for info in infos if info[4]]


def assert_public_host(host: str) -> list[str]:
    """Validate that `host` resolves only to public addresses.

    Returns the list of vetted IP strings. Raises ``ValueError`` if any
    resolved address is disallowed. When the dev bypass flag is set the
    rejection is skipped (a warning is logged) and the resolved addresses are
    returned unfiltered.

    NOTE: this is a *best-effort* guard. Because the returned addresses are not
    pinned for the subsequent connection, a host whose DNS record changes
    between this check and the actual socket connect (DNS rebinding / TOCTOU)
    can still be reached. Callers that need a hard guarantee must connect to a
    vetted IP from the returned list rather than re-resolving the hostname.
    """
    if private_host_bypass_enabled():
        logger.warning(
            "SSRF private-host guard bypassed for host %r via %s env flag",
            host,
            _ALLOW_PRIVATE_HOSTS_ENV_VAR,
        )
        try:
            return resolve_host_addresses(host)
        except ValueError:
            return []

    candidates = resolve_host_addresses(host)
    vetted: list[str] = []
    for candidate in candidates:
        try:
            addr = ipaddress.ip_address(candidate)
        except ValueError:
            continue
        if is_disallowed_address(addr):
            raise ValueError(
                f"host {host!r} resolves to a disallowed address: {candidate}"
            )
        vetted.append(candidate)
    return vetted
