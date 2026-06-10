"""CLI display helpers used by the standalone `scan.py` script.

Initialisation (colorama, terminal width probe) is deferred until the first
call so importing this module remains free of side effects.
"""

import shutil
import sys

from colorama import Fore, Style, init

_DEFAULT_TERMINAL_WIDTH = 100
_initialised = False
_terminal_width = _DEFAULT_TERMINAL_WIDTH


def _init_cli() -> int:
    global _initialised, _terminal_width
    if not _initialised:
        init(autoreset=True)
        _terminal_width = min(shutil.get_terminal_size().columns, _DEFAULT_TERMINAL_WIDTH)
        _initialised = True
    return _terminal_width


def _center(text: str) -> str:
    width = _init_cli()
    return text.center(width)


def print_banner() -> None:
    _init_cli()
    if sys.stdout.isatty():
        print("\x1b[2J\x1b[H", end="")
    print()

    art = [
        "  ██╗    ██╗███████╗██████╗ ",
        "  ██║    ██║██╔════╝██╔══██╗",
        "  ██║ █╗ ██║█████╗  ██████╔╝",
        "  ██║███╗██║██╔══╝  ██╔══██╗",
        "  ╚███╔███╔╝███████╗██████╔╝",
        "   ╚══╝╚══╝ ╚══════╝╚═════╝ ",
        "",
    ]
    for line in art:
        print(Fore.CYAN + Style.BRIGHT + line + Style.RESET_ALL)

    print()
    print(Style.BRIGHT + Fore.WHITE + _center("moku-analyzer  ·  v0.2.0") + Style.RESET_ALL)
    print()


def print_status(adapter_statuses=None) -> None:
    _init_cli()
    print(Style.BRIGHT + Fore.WHITE + "  Status" + Style.RESET_ALL)
    print("  " + "─" * 50)

    if adapter_statuses:
        for name, status_, note in adapter_statuses:
            if status_ == "ok":
                dot = Fore.GREEN + "●"
            elif status_ == "warn":
                dot = Fore.YELLOW + "●"
            else:
                dot = Fore.RED + "●"
            print(
                f"  {dot}{Style.RESET_ALL}  {name:<20}"
                f"{Fore.WHITE + Style.DIM}{note}{Style.RESET_ALL}"
            )

    print()


def print_menu() -> None:
    _init_cli()
    print("  " + "─" * 50)
    print(Style.BRIGHT + Fore.WHITE + "  What would you like to do?" + Style.RESET_ALL)
    print("  " + "─" * 50)
    items = [
        ("1", "Scan a target URL"),
        ("2", "Exit"),
    ]
    for num, label in items:
        print(
            f"  {Fore.CYAN + Style.BRIGHT}[{num}]{Style.RESET_ALL}  "
            f"{Fore.WHITE}{label}{Style.RESET_ALL}"
        )
    print()


def print_adapters() -> None:
    _init_cli()
    print()
    print("  " + "─" * 50)
    print(Style.BRIGHT + Fore.WHITE + "  Select a scanner:" + Style.RESET_ALL)
    print("  " + "─" * 50)
    adapters = [
        ("1", "builtin", "XSS, SQL Injection, CSRF detection"),
        ("2", "nuclei", "9000+ vulnerability templates"),
        ("3", "nikto", "Web server scanner"),
        ("4", "shodan", "Passive recon via Shodan"),
        ("5", "virustotal", "URL reputation check"),
        ("6", "zap", "OWASP ZAP active scanner"),
    ]
    for num, name, desc in adapters:
        color = Fore.GREEN if name == "builtin" else Fore.WHITE
        print(
            f"  {Fore.CYAN + Style.BRIGHT}[{num}]{Style.RESET_ALL}  "
            f"{color + Style.BRIGHT}{name:<15}{Style.RESET_ALL}"
            f"{Fore.WHITE + Style.DIM}{desc}{Style.RESET_ALL}"
        )
    print()


def print_scanning(url: str, adapter: str) -> None:
    _init_cli()
    print()
    print(f"  {Fore.CYAN + Style.BRIGHT}Scanning...{Style.RESET_ALL}")
    print(f"  {Fore.WHITE}Target :{Style.RESET_ALL}  {url}")
    print(f"  {Fore.WHITE}Scanner:{Style.RESET_ALL}  {adapter}")
    print(f"  {Fore.YELLOW}Please wait...{Style.RESET_ALL}")
    print()


def print_results(findings) -> None:
    _init_cli()
    print()
    if not findings:
        print(f"  {Fore.GREEN + Style.BRIGHT}No vulnerabilities found.{Style.RESET_ALL}")
        print()
        return

    print(f"  {Fore.RED + Style.BRIGHT}Found {len(findings)} findings{Style.RESET_ALL}")
    print("  " + "─" * 50)

    severity_colors = {
        "critical": Fore.RED,
        "high": Fore.RED,
        "medium": Fore.YELLOW,
        "low": Fore.BLUE,
        "info": Fore.CYAN,
    }

    for finding in findings:
        sev = finding.get("severity", "info").lower()
        color = severity_colors.get(sev, Fore.WHITE)
        print(
            f"\n  {color + Style.BRIGHT}[{sev.upper()}]{Style.RESET_ALL}  "
            f"{Fore.WHITE + Style.BRIGHT}{finding.get('title', '')}{Style.RESET_ALL}"
        )
        print(f"        {finding.get('description', '')}")
        if finding.get("evidence"):
            print(
                f"        {Fore.WHITE + Style.DIM}Evidence: "
                f"{finding['evidence']}{Style.RESET_ALL}"
            )
    print()


def print_success(msg: str) -> None:
    _init_cli()
    print(f"  {Fore.GREEN + Style.BRIGHT}{msg}{Style.RESET_ALL}")


def print_error(msg: str) -> None:
    _init_cli()
    print(f"  {Fore.RED + Style.BRIGHT}{msg}{Style.RESET_ALL}")


def print_info(msg: str) -> None:
    _init_cli()
    print(f"  {Fore.CYAN}{msg}{Style.RESET_ALL}")


def get_input(prompt: str) -> str:
    _init_cli()
    return input(f"  {Fore.CYAN + Style.BRIGHT}>{Style.RESET_ALL}  {prompt}: ").strip()
