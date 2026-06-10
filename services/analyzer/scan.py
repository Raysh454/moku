"""moku-analyzer CLI — thin client that talks to the running service."""

import os
import sys
import time

import requests

from app.core.cli_display import (
    get_input,
    print_adapters,
    print_banner,
    print_error,
    print_info,
    print_menu,
    print_results,
    print_scanning,
    print_status,
    print_success,
)

API = os.environ.get("MOKU_ANALYZER_URL", "http://127.0.0.1:8181")
TOKEN = os.environ.get("MOKU_ANALYZER_TOKEN", "")


def _headers() -> dict:
    return {"X-Moku-Token": TOKEN} if TOKEN else {}


def scan(url: str, backend: str = "builtin") -> None:
    print_scanning(url, backend)

    payload = {"url": url, "backend": backend}
    try:
        response = requests.post(f"{API}/scan", json=payload, headers=_headers(), timeout=30)
        response.raise_for_status()
        job_id = response.json()["job_id"]
    except (requests.RequestException, KeyError, ValueError) as exc:
        print_error(f"Could not submit scan: {exc}")
        sys.exit(1)

    for elapsed in range(60):
        time.sleep(5)
        try:
            result_response = requests.get(
                f"{API}/scan/{job_id}", headers=_headers(), timeout=30
            )
            result = result_response.json()
        except (requests.RequestException, ValueError) as exc:
            print_error(f"Could not poll scan: {exc}")
            return

        status_ = result.get("status")
        if status_ in {"pending", "running"}:
            print_info(f"Scanning... ({(elapsed + 1) * 5}s)")
            continue
        if status_ == "completed":
            findings = result.get("findings", [])
            print_results(findings)
            print_success(f"Scan ID: {job_id}")
            return
        if status_ == "failed":
            print_error(f"Scan failed: {result.get('error')}")
            return

    print_error("Scan timed out")


if __name__ == "__main__":
    if len(sys.argv) >= 2:
        scan(sys.argv[1], sys.argv[2] if len(sys.argv) > 2 else "builtin")
        sys.exit(0)

    print_banner()
    print_status(
        adapter_statuses=[
            ("Builtin", "ok", "ready"),
        ],
    )

    while True:
        print_menu()
        choice = get_input("Enter choice (1-2)")
        if choice == "1":
            target = get_input("Enter target URL")
            if not target:
                print_error("URL is required")
                continue
            print_adapters()
            adapter_map = {
                "1": "builtin",
                "2": "nuclei",
                "3": "nikto",
                "4": "shodan",
                "5": "virustotal",
                "6": "zap",
            }
            sel = get_input("Select adapter (1-6) [default: 1]") or "1"
            scan(target, adapter_map.get(sel, "builtin"))
        elif choice == "2":
            print_info("Goodbye!")
            sys.exit(0)
        else:
            print_error("Invalid choice")
