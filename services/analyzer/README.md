# moku-analyzer

A vulnerability-analyzer sidecar for the [Moku](https://github.com/Raysh454/moku)
platform. Moku's Go orchestrator dispatches scan requests here over HTTP; this
service runs them through a pluggable adapter system and returns structured
findings.

Built with Python + FastAPI. State is held in-memory (no database); evidence
blobs are content-addressed on the filesystem.

## What this does

When Moku wants a target analyzed it POSTs a scan request to this service. The
service:

- accepts the request (`POST /scan`) and returns a server-generated job id
- runs the scan in the background on a bounded thread pool
- exposes status + structured findings for polling (`GET /scan/{job_id}`)
- dispatches to the requested **adapter** (built-in dynamic analyzer, or a
  wrapper around an external tool / API)

The core design is the **Adapter Pattern**: a new scanner is added by
implementing one small interface and registering it — no core changes.

## Architecture

```
Moku (Go orchestrator)  ──HTTP──▶  moku-analyzer (FastAPI)

  POST /scan               submit a scan          → 202 {job_id}
  GET  /scan/{job_id}      poll status + findings → ScanResult
  GET  /health             liveness + adapter list + contract_version
  GET  /capabilities?backend=<name>               → Capabilities

  api/        FastAPI routes + token auth dependency
  core/       job_store (in-memory, UUID, TTL pruning), runner
              (ThreadPoolExecutor), executor, evidence_store (sha256)
  adapters/   builtin · nuclei · nikto · shodan · virustotal · zap
              (CLI scanners share CliScannerAdapter; registry holds them)
  plugins/    xss · sqli · csrf  (built-in dynamic analysis, via PluginManager)
  net_guard   centralised SSRF guard (private/loopback/rebind rejection)
```

External dependencies are per-adapter: the `nuclei`/`nikto`/`zap` adapters shell
out to those CLIs; `shodan`/`virustotal` call their HTTP APIs with a key.

## Key properties

- **Async job engine** — submit returns immediately; scans run in the
  background and are polled.
- **6 adapters** — `builtin`, `nuclei`, `nikto`, `shodan`, `virustotal`, `zap`.
- **3 built-in plugins** — reflected XSS, SQL injection, CSRF.
- **In-memory job store** — UUIDv4 ids, soft cap, TTL pruning of *terminal*
  jobs only (active scans are never evicted; a full store returns `429`).
- **Filesystem evidence store** — request/response blobs addressed by sha256,
  partitioned per job, path-traversal hardened.
- **Security-first** — centralised SSRF guard (with redirect re-validation),
  constant-time shared-secret auth, fail-closed startup posture, secret
  redaction in error messages.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`  | `/health` | `{status, contract_version, backend, adapters_available}` |
| `GET`  | `/capabilities?backend=<name>` | `Capabilities` for one adapter |
| `POST` | `/scan` | Submit a scan; `202` + `{job_id}` (or `429` when full, `422` on unknown backend / disallowed target) |
| `GET`  | `/scan/{job_id}` | Poll; returns `ScanResult` (`404` if unknown) |

All routes sit behind the `X-Moku-Token` auth dependency (a no-op when
`MOKU_ANALYZER_TOKEN` is unset — see below).

## Setup

Requirements: Python 3.11+. Optionally the `nuclei`/`nikto`/`zap` CLIs and
Shodan/VirusTotal API keys for those adapters.

From the repo root, the `make.go` script drives the lifecycle:

```bash
go run make.go sidecar-install    # create .venv + install requirements + requirements-dev
go run make.go sidecar-start      # boot uvicorn (default 127.0.0.1:8181)
go run make.go sidecar-health     # probe /health
go run make.go sidecar-stop       # stop via PID file under services/analyzer/.run/
go run make.go sidecar-test       # pytest tests/
go run make.go schema-check       # round-trip the Go testdata fixtures through Pydantic
```

`go run make.go run-with-sidecar` boots the API server with the sidecar running automatically.

Manual launch:

```bash
cd services/analyzer
python -m venv .venv && . .venv/bin/activate        # or .venv\Scripts\activate
pip install -r requirements.txt -r requirements-dev.txt
python run.py                                        # serves 127.0.0.1:8181
```

Swagger UI is at `http://127.0.0.1:8181/docs`.

## Environment variables

| Variable | Default | Effect |
|----------|---------|--------|
| `MOKU_ANALYZER_HOST` | `127.0.0.1` | Interface uvicorn binds to (read by `run.py` and the start scripts). |
| `MOKU_ANALYZER_PORT` | `8181` | Port uvicorn binds to. |
| `MOKU_ANALYZER_TOKEN` | unset | Shared secret. When set, every request must carry a matching `X-Moku-Token` header (constant-time compare); unset = no-op, for loopback dev. **Required** to start on a non-loopback host. |
| `MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS` | unset | When `1`/`true`/`yes`, bypasses the SSRF guard that rejects loopback/RFC1918 targets. Local/demo use only; **refused at startup on a non-loopback bind**. Logged as a warning. |
| `MOKU_ANALYZER_WORKERS` | `4` | Size of the background scan thread pool. |
| `MOKU_ANALYZER_MAX_JOBS` | `1024` | Soft cap on resident jobs before terminal-job eviction / `429`. |
| `MOKU_EVIDENCE_DIR` | `~/.config/moku/evidence` | Root of the sha256 evidence store. |
| `MOKU_EVIDENCE_MAX_BYTES` | `0` (no cap) | Hard ceiling on total on-disk evidence. When >0 the background pruner trims the oldest blobs past this size (active scans' evidence is never trimmed). Retention is otherwise TTL-bound only. |
| `SHODAN_API_KEY` | unset | Consumed by the `shodan` adapter. |
| `VIRUSTOTAL_API_KEY` | unset | Consumed by the `virustotal` adapter. |

`main.py` calls `load_dotenv()`, so a `.env` in the service root is honored for
local development. Do not ship a `.env` that enables
`MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS` to a networked deployment — startup will
refuse it.

## Available adapters

| Adapter | Type | Requires | What it does |
|---------|------|----------|--------------|
| `builtin` | dynamic | nothing | reflected XSS, SQLi, CSRF probing (honors cookie/basic/bearer auth) |
| `nuclei` | CLI | `nuclei` | templated vulnerability scanning |
| `nikto` | CLI | `nikto` | web-server misconfiguration scan |
| `zap` | CLI | `zap.sh` | OWASP ZAP quick scan |
| `shodan` | API | `SHODAN_API_KEY` | passive host enrichment (open ports, CVEs) |
| `virustotal` | API | `VIRUSTOTAL_API_KEY` | URL reputation (requires explicit `raw_options.virustotal_consent=true`) |

The CLI adapters (`nuclei`/`nikto`/`zap`) share the `CliScannerAdapter` base,
which provides their common `capabilities()` and honors `ScanRequest.max_duration`
as the subprocess timeout.

## Adding an adapter

Implement the `BaseAdapter` contract and register it in
`register_default_adapters` (`app/app_factory.py`):

```python
from app.adapters.base import BaseAdapter
from app.models.schemas import Capabilities, Finding, ScanRequest


class MyAdapter(BaseAdapter):
    name = "myscanner"
    description = "My custom scanner"

    def run_scan(self, request: ScanRequest) -> list[Finding]:
        ...   # validate_target_url(str(request.url)) first, then scan
        return []

    def capabilities(self) -> Capabilities:
        return Capabilities(async_=True, max_concurrent_scans=1)
```

A CLI-backed scanner should subclass `CliScannerAdapter` instead and implement
`run_scan` using `run_subprocess` + a parser; it inherits `capabilities()` and
the `max_duration` timeout policy.

Adapter capabilities are kept in lock-step with the Go client via the shared
manifest at `../../internal/analyzer/testdata/capabilities.json`
(`test_capabilities_conformance.py` asserts it on the Python side; a Go test
asserts the other).

## Go ↔ Python contract

The Pydantic models in `app/models/schemas.py` round-trip with Moku's Go
`analyzer.ScanResult` / `analyzer.Capabilities` structs. Two integration notes:

- **`async` alias** — `Capabilities.async_` serializes to the JSON key `async`
  (`async` is reserved in Python). `populate_by_name=True`, so either name
  constructs the model; the wire form is always `"async"`.
- **Contract version** — `/health` reports `contract_version` (`CONTRACT_VERSION`
  in `schemas.py`); the Go client logs a warning on mismatch with its own
  `analyzer.SidecarContractVersion`.

`make schema-check` validates the committed Go fixtures against the Pydantic
`ScanResult`; `tests/test_contract_golden.py` pins the Python serializer output
to a golden the Go side decodes, so the contract is guarded in both directions.

## CLI tool

`scan.py` is a thin client against a running service:

```bash
python scan.py https://target.com [backend]   # defaults to builtin
```

It honors `MOKU_ANALYZER_URL` (default `http://127.0.0.1:8181` — set this if you
changed the port) and `MOKU_ANALYZER_TOKEN`.

## Testing

```bash
go run make.go sidecar-test       # or: cd services/analyzer && pytest tests/ -v
```

CI (`.github/workflows/ci.yml`) runs `ruff`, the full `pytest` suite, and
`schema_check.py` on every push/PR.

## License

See [LICENSE](LICENSE).
