"""Tests for the deferred init of `cli_display`."""

from unittest.mock import patch


def test_importing_module_does_not_call_init():
    import importlib

    import app.core.cli_display as module

    with patch.object(module, "init") as init_mock:
        importlib.reload(module)
        assert init_mock.call_count == 0


def test_print_banner_skips_clear_when_not_tty(monkeypatch, capsys):
    import app.core.cli_display as module

    monkeypatch.setattr(module.sys.stdout, "isatty", lambda: False)
    module._initialised = True
    module.print_banner()
    captured = capsys.readouterr().out
    assert "\x1b[2J" not in captured
