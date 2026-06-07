"""Tests for the inbound auth gate and fail-closed startup posture."""

import pytest
from fastapi.testclient import TestClient

from app.app_factory import create_app


def _register_nothing(_registry) -> None:
    return None


class TestAuthGate:
    def test_noop_when_token_unset(self, client, monkeypatch):
        monkeypatch.delenv("MOKU_ANALYZER_TOKEN", raising=False)
        assert client.get("/health").status_code == 200

    def test_accepts_matching_token(self, client, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_TOKEN", "s3cret")
        resp = client.get("/health", headers={"X-Moku-Token": "s3cret"})
        assert resp.status_code == 200

    def test_rejects_wrong_token(self, client, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_TOKEN", "s3cret")
        resp = client.get("/health", headers={"X-Moku-Token": "nope"})
        assert resp.status_code == 401

    def test_rejects_missing_header_when_token_set(self, client, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_TOKEN", "s3cret")
        assert client.get("/health").status_code == 401


class TestStartupPosture:
    def test_loopback_without_token_starts(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_HOST", "127.0.0.1")
        monkeypatch.delenv("MOKU_ANALYZER_TOKEN", raising=False)
        with TestClient(create_app(register=_register_nothing)):
            pass  # no error == started

    def test_non_loopback_without_token_refuses(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_HOST", "0.0.0.0")
        monkeypatch.delenv("MOKU_ANALYZER_TOKEN", raising=False)
        with pytest.raises(RuntimeError):
            with TestClient(create_app(register=_register_nothing)):
                pass

    def test_non_loopback_with_token_starts(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_HOST", "0.0.0.0")
        monkeypatch.setenv("MOKU_ANALYZER_TOKEN", "tok")
        monkeypatch.delenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", raising=False)
        with TestClient(create_app(register=_register_nothing)):
            pass

    def test_non_loopback_with_ssrf_bypass_refuses(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_HOST", "0.0.0.0")
        monkeypatch.setenv("MOKU_ANALYZER_TOKEN", "tok")
        monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
        with pytest.raises(RuntimeError):
            with TestClient(create_app(register=_register_nothing)):
                pass
