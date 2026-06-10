"""Tests for the shared adapter helpers (URL guard, subprocess wrapper)."""

import subprocess
from unittest.mock import MagicMock, patch

import pytest

from app.adapters._helpers import run_subprocess, validate_target_url


class TestValidateTargetUrl:
    def test_rejects_file_scheme(self):
        with pytest.raises(ValueError):
            validate_target_url("file:///etc/passwd")

    def test_rejects_argv_injection_dash_prefix(self):
        with pytest.raises(ValueError):
            validate_target_url("-target")

    def test_rejects_loopback(self):
        with pytest.raises(ValueError):
            validate_target_url("http://127.0.0.1/")

    def test_rejects_private_after_resolution(self):
        with patch(
            "app.net_guard.socket.getaddrinfo",
            return_value=[(0, 0, 0, "", ("10.0.0.5", 0))],
        ):
            with pytest.raises(ValueError):
                validate_target_url("https://internal.example.com")

    def test_accepts_public_target(self):
        with patch(
            "app.net_guard.socket.getaddrinfo",
            return_value=[(0, 0, 0, "", ("8.8.8.8", 0))],
        ):
            assert validate_target_url("https://example.com") == "https://example.com"

    def test_allows_loopback_when_env_flag_set(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
        assert (
            validate_target_url("http://127.0.0.1:9999/admin")
            == "http://127.0.0.1:9999/admin"
        )

    def test_still_rejects_unsupported_scheme_when_env_flag_set(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
        with pytest.raises(ValueError):
            validate_target_url("file:///etc/passwd")

    def test_still_rejects_dash_prefix_when_env_flag_set(self, monkeypatch):
        monkeypatch.setenv("MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS", "true")
        with pytest.raises(ValueError):
            validate_target_url("-target")


class TestRunSubprocess:
    def test_raises_runtime_error_on_nonzero_exit(self):
        completed = MagicMock()
        completed.returncode = 2
        completed.stdout = "out"
        completed.stderr = "secret stderr that should not leak"
        with patch("app.adapters._helpers.subprocess.run", return_value=completed):
            with pytest.raises(RuntimeError) as exc_info:
                run_subprocess(["echo"], timeout=10, name="tool")
        assert "secret stderr" not in str(exc_info.value)

    def test_raises_runtime_error_on_missing_binary(self):
        with patch(
            "app.adapters._helpers.subprocess.run", side_effect=FileNotFoundError()
        ):
            with pytest.raises(RuntimeError) as exc_info:
                run_subprocess(["never"], timeout=10, name="never-tool")
        assert "never-tool" in str(exc_info.value)

    def test_raises_runtime_error_on_timeout(self):
        with patch(
            "app.adapters._helpers.subprocess.run",
            side_effect=subprocess.TimeoutExpired(cmd="x", timeout=1),
        ):
            with pytest.raises(RuntimeError):
                run_subprocess(["x"], timeout=1, name="x")
