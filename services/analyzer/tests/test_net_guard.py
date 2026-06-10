"""Tests for the centralised SSRF guard (app.net_guard)."""

import ipaddress

import pytest

from app import net_guard


class TestIsDisallowedAddress:
    @pytest.mark.parametrize(
        "addr",
        [
            "127.0.0.1",  # loopback
            "10.0.0.5",  # private
            "192.168.1.1",  # private
            "169.254.169.254",  # link-local (cloud metadata)
            "0.0.0.0",  # unspecified
            "100.64.0.1",  # CGNAT (RFC 6598) — caught via not-is-global
            "224.0.0.1",  # multicast
            "::1",  # ipv6 loopback
            "::ffff:127.0.0.1",  # ipv4-mapped loopback
            "::ffff:169.254.169.254",  # ipv4-mapped link-local metadata
            "::ffff:10.0.0.1",  # ipv4-mapped private
        ],
    )
    def test_disallows_dangerous_addresses(self, addr):
        assert net_guard.is_disallowed_address(ipaddress.ip_address(addr)) is True

    @pytest.mark.parametrize("addr", ["8.8.8.8", "1.1.1.1", "93.184.216.34"])
    def test_allows_public_addresses(self, addr):
        assert net_guard.is_disallowed_address(ipaddress.ip_address(addr)) is False


class TestAssertPublicHost:
    def test_rejects_loopback_literal(self):
        with pytest.raises(ValueError):
            net_guard.assert_public_host("127.0.0.1")

    def test_rejects_ipv4_mapped_metadata_after_resolution(self, monkeypatch):
        monkeypatch.setattr(
            "app.net_guard.socket.getaddrinfo",
            lambda *a, **kw: [(0, 0, 0, "", ("::ffff:169.254.169.254", 0))],
        )
        with pytest.raises(ValueError):
            net_guard.assert_public_host("rebind.example.com")

    def test_returns_vetted_ip_for_public_host(self, monkeypatch):
        monkeypatch.setattr(
            "app.net_guard.socket.getaddrinfo",
            lambda *a, **kw: [(0, 0, 0, "", ("8.8.8.8", 0))],
        )
        assert net_guard.assert_public_host("example.com") == ["8.8.8.8"]

    def test_bypass_returns_resolved_without_rejection(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
        assert net_guard.assert_public_host("127.0.0.1") == ["127.0.0.1"]
