# moku-analyzer

A production-ready vulnerability analyzer service built for the moku platform. This service receives scan requests from moku's Go client, analyzes them using a pluggable adapter system, and returns structured vulnerability findings.

Built with Python + FastAPI + SQLite as part of a Final Year Project at DHA Suffa University.

## What This Does

Moku crawls websites and monitors for changes. When it finds a page, it sends it here for vulnerability analysis. This service:

- Receives a scan request (URL or HTML) from moku
- Runs it through a selected vulnerability scanner (adapter)
- Saves all results to SQLite database
- Generates professional reports (CSV auto-download, TXT optional)
- Returns structured vulnerability findings back to moku

The key design is the **Adapter Pattern** — any vulnerability scanner can be plugged in without changing the core system.

## Architecture

```
moku (Go client)
│
▼  HTTP
┌─────────────────────────────────────────┐
│          moku-analyzer                  │
│         (FastAPI Service)               │
│                                         │
│  POST /scan (submit job)                │
│  GET  /scan/{id} (poll results)         │
│  GET  /scan/{id}/download (get report)  │
│  GET  /health (status check)            │
│  GET  /adapters (list scanners)         │
│  GET  /scans (history)                  │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │    Adapter System + Plugins       │  │
│  │                                   │  │
│  │  ┌─ Adapters (external tools)    │  │
│  │  │  builtin | nuclei | nikto     │  │
│  │  │  shodan  | virustotal | zap   │  │
│  │  │                               │  │
│  │  └─ Plugins (dynamic analysis)   │  │
│  │     xss | sqli | csrf            │  │
│  └───────────────────────────────────┘  │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │    SQLite Database                │  │
│  │  • Scan history (permanent)       │  │
│  │  • Vulnerability findings         │  │
│  │  • Evidence blobs (SHA256)        │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
│
├─→ Nuclei CLI
├─→ Nikto CLI
├─→ OWASP ZAP
├─→ Shodan API
├─→ VirusTotal API
└─→ Built-in Dynamic Analyzer (Phase 2)
```

## Key Features

✅ **Async Job Engine** — Submit scans and poll for results, never block  
✅ **6 Vulnerability Scanners** — Nuclei, Nikto, Shodan, VirusTotal, ZAP, built-in  
✅ **3 Dynamic Plugins** — XSS detection, SQL injection detection, CSRF detection  
✅ **SQLite Database** — Permanent scan history, query anytime  
✅ **Professional Reports** — CSV (auto) + TXT (optional), auto-download  
✅ **Sequential Scan IDs** — scan_00001, scan_00002, etc.  
✅ **Authenticated Scanning** — Support for cookies, API tokens, session auth  
✅ **CLI Tool** — Simple one-command scanner from terminal  
✅ **44 Passing Tests** — Full test coverage  

## Real-World Vulnerability Discoveries

### Discovery 1: CSRF Vulnerability
**Target:** daraz.pk/account/change-email  
**Finding:** Cross-Site Request Forgery (CSRF) - No token validation  
**Severity:** Medium (CVSS 6.5)  
**Status:** Reported to Daraz & OpenBugBounty

### Discovery 2: XSS-CSRF Vulnerability  
**Target:** daraz.pk (all pages)  
**Finding:** Missing SameSite Cookie Attribute  
**Severity:** Medium (CVSS 6.5)  
**Status:** Reported to Daraz & OpenBugBounty

### Responsible Disclosure
- Email sent to customer.pk@care.daraz.com
- Submitted to OpenBugBounty for public tracking
- Professional reports generated in DOCX format
- Following 90-day disclosure timeline

## Project Structure

```
moku-analyzer/
├── main.py                        # FastAPI app entry point
├── run.py                         # Server startup
├── scan.py                        # CLI scanner tool
├── requirements.txt               # Python dependencies
├── moku_analyzer.db               # SQLite database
├── README.md                      # This file
│
├── app/
│   ├── api/
│   │   └── routes.py              # REST API endpoints
│   │
│   ├── core/
│   │   ├── database.py            # SQLite manager
│   │   ├── job_store.py           # Job queue
│   │   ├── runner.py              # Scan executor
│   │   ├── executor.py            # Test payload sender
│   │   ├── report_generator.py    # CSV/TXT report generation
│   │   └── evidence_store.py      # SHA256 evidence storage
│   │
│   ├── models/
│   │   └── schemas.py             # Pydantic data models
│   │
│   ├── adapters/
│   │   ├── base.py                # Abstract adapter interface
│   │   ├── registry.py            # Adapter registry
│   │   ├── builtin_adapter.py     # Dynamic analyzer (Phase 2)
│   │   ├── nuclei_adapter.py      # Nuclei CLI wrapper
│   │   ├── nikto_adapter.py       # Nikto CLI wrapper
│   │   ├── shodan_adapter.py      # Shodan API client
│   │   ├── virustotal_adapter.py  # VirusTotal API client
│   │   └── zap_adapter.py         # OWASP ZAP wrapper
│   │
│   └── plugins/
│       ├── base_plugin.py         # Abstract plugin interface
│       ├── xss_plugin.py          # XSS detection
│       ├── sqli_plugin.py         # SQL injection detection
│       ├── csrf_plugin.py         # CSRF detection
│       └── plugin_manager.py      # Plugin orchestrator
│
└── tests/                         # Test suite (44 tests)
```

## Setup & Installation

### Requirements

- Python 3.11+
- pip
- (Optional) Nuclei, Nikto, OWASP ZAP installed for those adapters
- (Optional) Shodan API key from [shodan.io](https://shodan.io)
- (Optional) VirusTotal API key from [virustotal.com](https://virustotal.com)

### Installation

**Step 1 — Clone:**
```bash
git clone https://github.com/Shaheer005/moku-analyzer.git
cd moku-analyzer
```

**Step 2 — Virtual environment:**
```bash
python -m venv .venv
.venv\Scripts\activate          # Windows
source .venv/bin/activate       # Mac/Linux
```

**Step 3 — Install dependencies:**
```bash
pip install -r requirements.txt
```

**Step 4 — Create .env file (project root):**
```
SHODAN_API_KEY=your_key_here
VIRUSTOTAL_API_KEY=your_key_here
```

**Step 5 — Run the server:**
```bash
python run.py
```

Server starts at http://127.0.0.1:8080

## Usage

### From FastAPI Swagger UI

Open http://127.0.0.1:8080/docs in your browser.

### From CLI Tool

**Run a scan:**
```bash
python scan.py https://target.com
# CSV report auto-downloads to Downloads folder
# Asked if you want TXT as well
```

**View scan history:**
```bash
python scan.py --history
# Shows all past scans with IDs
```

**Download old scan report:**
```bash
python scan.py --download scan_00001 csv
python scan.py --download scan_00001 txt
```

**Export all scans:**
```bash
python scan.py --export-all
```

### From moku Go Client

**Submit scan:**
```bash
POST http://moku-analyzer-url/scan
{
  "method": "url",
  "url": "http://target.com",
  "adapter": "nuclei"
}
```

**Poll results:**
```bash
GET http://moku-analyzer-url/scan/{job_id}
# Returns: {status, vulnerabilities[]}
```

**Download report:**
```bash
GET http://moku-analyzer-url/scan/{job_id}/download?format=csv
# Returns: CSV file for download
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check + adapter list |
| GET | `/adapters` | Available analyzers for UI |
| GET | `/scans` | Scan history |
| POST | `/scan` | Submit scan job (returns job_id) |
| GET | `/scan/{id}` | Poll for results |
| GET | `/scan/{id}/download` | Download report (CSV/TXT) |

### Schema Notes for Integrators

The Pydantic models in `app/models/schemas.py` are designed to round-trip
with Moku's Go-side `analyzer.Capabilities` / `analyzer.ScanResult` structs.
One field on `Capabilities` uses a name alias because `async` is a reserved
keyword in Python:

| JSON key (wire) | Python field name | Notes |
|-----------------|-------------------|-------|
| `async`         | `async_`          | Defined as `Field(False, alias="async")`. `_BASE_CONFIG` sets `populate_by_name=True`, so callers can construct `Capabilities(async_=True)` or `Capabilities(async=True)` — both work. Serialization always emits `"async"`. |

If you write a new adapter or consumer in Python, prefer the alias
(`async_`) for attribute access and rely on Pydantic's serializer to emit
the correct JSON key. Go and other clients should always use `"async"` on
the wire.

## Available Analyzers

| Analyzer | Type | Requires | What It Does |
|----------|------|----------|--------------|
| builtin | Dynamic | Nothing | XSS, SQL injection, CSRF, headers checks |
| nuclei | CLI | nuclei tool | 9000+ vulnerability templates |
| nikto | CLI | nikto tool | Web server misconfigurations |
| shodan | API | API key | Open ports, services, CVEs (passive) |
| virustotal | API | API key | URL reputation (90+ vendors) |
| zap | CLI | ZAP tool | Active web vulnerability scanner |

## Adding Your Own Analyzer

Create a new adapter in 3 steps:

**Step 1 — Create `app/adapters/myanalyzer_adapter.py`:**
```python
from app.adapters.base import BaseAdapter
from app.models.schemas import Vulnerability, Severity
from typing import List

class MyAnalyzerAdapter(BaseAdapter):
    name = "myanalyzer"
    description = "My custom analyzer"

    def scan_url(self, url: str) -> List[Vulnerability]:
        # Your scanning logic here
        return []

    def scan_html(self, html: str, source_url: str = "") -> List[Vulnerability]:
        # Optional: scan raw HTML
        return []
```

**Step 2 — Register in `main.py`:**
```python
from app.adapters.myanalyzer_adapter import MyAnalyzerAdapter
registry.register(MyAnalyzerAdapter())
```

**Step 3 — Done. It works immediately.**

## Database

All scans are stored in SQLite (`moku_analyzer.db`):

- **scans table** — scan metadata, severity counts, timestamps
- **vulnerabilities table** — individual findings with confidence scores

Query anytime with `python scan.py --history` or via the API.

## Testing

Run all 44 tests:
```bash
python -m pytest tests/ -v
```

## Development

**With auto-reload (clears jobs on restart):**
```bash
python -m uvicorn main:app --reload --port 8080
```

**Production (persistent jobs):**
```bash
python run.py
```

**Interactive docs:**
http://127.0.0.1:8080/docs

## Phase Roadmap

| Phase | Features | Deadline | Status |
|-------|----------|----------|--------|
| Phase 1 | FastAPI service, 6 adapters, async job engine, 44 tests | April 2026 | ✅ Complete |
| Phase 2 | Built-in analyzer: XSS, SQLi, CSRF detection, plugins, evidence storage | May 2026 | ✅ Complete |

## Related Projects

- [moku](https://github.com/Shaheer005/moku) — Main platform (Go)
- [Nuclei](https://nuclei.projectdiscovery.io/) — Template scanner
- [OWASP ZAP](https://www.zaproxy.org/) — Web app security scanner
- [Shodan](https://www.shodan.io/) — Internet scan database
- [VirusTotal](https://virustotal.com/) — Malware/URL reputation

## License

See LICENSE file.

## Author

**Shaheer Ahmed**  
2026
