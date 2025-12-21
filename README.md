# moku

Public bug-bounty surfaces are increasingly saturated, while access to high‑value private programs requires demonstrated impact on public targets. Moku continuously monitors sites for new pages and content changes, surfacing newly exposed or modified functionality so researchers can prioritize fresh, less‑tested attack paths.

A small, modular Go platform for fetching, versioning, and assessing web content with a security focus. The project is intended as an extensible foundation for building reproducible, explainable, and auditable pipelines that fetch pages, compute explainable security‑relevance scores, track snapshots over time, and augment results with external vulnerability analysis.

This README gives a high‑level overview and the endgame vision first, then a technical footnote describing the current scaffolding and implementation state.

---

## High-level overview

moku splits responsibilities into small, well‑scoped components so each area can evolve independently and be tested in isolation:

- Fetch (webclient)
  - Pluggable backends that retrieve page content: a net/http-based backend for most cases and a headless-browser backend for JS-rendered content.
  - The net/http backend is configurable and intended to be usable anywhere you need a configured `*http.Client` (so you can issue arbitrary API calls with the same transport settings).

- Assess (assessor)
  - A heuristics-driven assessor computes an explainable score for a document. The assessor returns a structured `ScoreResult` that includes numeric score, normalized score, confidence, evidence items, matched rules and metadata.
  - The project emphasizes explainability: evidence and matched rule identifiers make it possible to inspect why a document received a given score.

- Track (tracker)
  - A lightweight, git-like snapshot store to capture versions of pages and compute diffs. This supports auditing, history, and rollback workflows.

- Augment (analyzer)
  - A small client abstraction for talking to external vulnerability analyzers (for example, a Python FastAPI-based scanner). The analyzer client supports async job submission, polling for results, and convenience helpers to wait for completion.

Design principles
- Small explicit interfaces for dependency injection and testability.
- Separation of concerns: fetching, scoring, versioning, and vulnerability scanning are decoupled.
- Pluggable components so you can swap implementations (e.g., a different fetching backend or analyzer transport).
- Incremental development: scaffolds compile and expose neutral or explicit-not-implemented behavior so the system can be wired and tested before full implementations are added.

Audience
- Security engineers and researchers building scanning pipelines.
- Platform engineers building capture, audit, and content-replay systems.
- Developers experimenting with rule/heuristics-driven scoring and integrations with vulnerability scanners.

---

## Endgame — what we are building toward

The long-term vision for moku is a production-quality, extensible platform for web-content security assessment with clear auditability and explainability:

- Robust fetchers that cover:
  - High-throughput net/http fetching with configurable TLS, proxy, retries and instrumentation.
  - Headless-browser fetches for pages that require JS execution and rendering.

- Deterministic, explainable scoring:
  - A full-featured heuristics engine: feature extraction, rule definitions (with ids and severity), rule registry, weighted aggregation and versioned scoring algorithms so results are reproducible.
  - Human-readable evidence items and tooling to trace scores back to matched rules and features.

- Snapshot/versioning and diffs:
  - Efficient snapshot storage (content-addressed or delta storage).
  - DOM-aware diffing and textual diffs, plus a compact API for history browsing and rollbacks.

- Vulnerability analysis integration:
  - A stable analyzer client with async job model, webhooks, and health checks.
  - A reference analyzer implementation (e.g., FastAPI Python service) that can be replaced by existing scanners or custom analyzers.

- Production hardening:
  - Observability: metrics, structured logs and traces.
  - Security: authenticated analyzer endpoints, rate limiting, and resource isolation for scans.
  - CI/CD with contract tests that validate the analyzer client against the analyzer API.

- Usability:
  - A simple CLI and/or HTTP API to perform fetch → assess → commit → scan workflows.
  - Configurable rule packs and a UI for analysts to review findings and tune rules.

---

## How the pieces fit together (conceptual pipeline)

1. Fetcher (webclient) retrieves page content and returns a canonical `model.Response`.
2. Assessor consumes raw HTML bytes (or `model.Response`) and returns a `ScoreResult`:
   - `ScoreResult` contains numeric score, normalized score, confidence, evidence items, matched rules, and metadata.
3. Tracker receives snapshots of pages and stores a `Version` (commit). `DiffResult` can be computed between versions.
4. Analyzer client sends HTML (or URL) to an external analyzer; results can be attached to `ScoreResult.Meta` or stored in the tracker for cross-reference.
5. UI/CLI/Orchestration layer composes these steps and provides workflows: snapshot & score, scan & persist, compare historical changes, etc.

---

## Quickstart (developer)

Prerequisites
- Go toolchain (1.18+ recommended)
- Git

Build
```bash
go build ./...
```

Tests
```bash
go test ./...
```
Note: The repository contains scaffolding tests (external-package style). Many scaffolds intentionally return `not implemented` or neutral results — tests assert that behavior. Some components (chromedp backend) are intentionally non-functional in the current dev branch and tests reflect that.

Compose/wire components (example)
- Create a net/http-backed WebClient (use `nil` for a default `*http.Client` or inject a configured client):
```go
wc, err := nethttp.NewNetHTTPClient(cfg, logger, nil) // pass nil for default http.Client
```

- Create an assessor (heuristics-based scaffold):
```go
assr, err := assessor.NewHeuristicsAssessor(assessorCfg, logger)
res, _ := assr.ScoreHTML(ctx, []byte("<html>...</html>"), "https://example.com")
```

- Create the analyzer client and reuse the same http.Client if desired:
```go
hc := wc.(*nethttp.NetHTTPClient).HTTPClient() // or use extraction helper if available
aCfg := &analyzer.Config{BaseURL: "http://analyzer:8080", HTTPClient: hc}
an := analyzer.NewAnalyzerClient(aCfg, logger)
jobID, _ := an.SubmitScan(ctx, &models.ScanRequest{Method: "html", HTML: string(body)})
```

General recommendations
- Use dependency injection to pass configured `*http.Client` and logger implementations to make tests deterministic.
- Run long-running analyzer scans asynchronously (submit job then poll or accept a webhook callback).
- Keep scoring rules and weights under version control so results are auditable.

---

## Footnote — technical details & current state (dev branch)

This footnote summarizes the current scaffolding and known caveats on the `dev` branch so you know what is implemented and what is scaffolded:

Core packages
- internal/webclient
  - `nethttp`: the net/http-based backend. The constructor accepts an injected `*http.Client` so code that needs raw API calls (such as analyzer HTTP requests) can reuse the same transport (timeouts, TLS, proxy). The client exposes:
    - `Do(ctx, *model.Request) -> *model.Response`
    - `DoHTTPRequest(ctx, *http.Request) -> *http.Response`
    - `HTTPClient() -> *http.Client`
  - `chromedp`: headless-browser backend scaffold. The constructor and methods exist; chromedp behavior is intentionally not fully implemented in `dev` and many tests assert "not implemented" behavior.

- internal/assessor
  - `HeuristicsAssessor` scaffold: accepts HTML bytes or a `model.Response` and returns a neutral `ScoreResult`. `ScoreResult` and `EvidenceItem` types are available. The heuristics pipeline (feature extraction, rule evaluation and aggregation) is to be implemented next.

- internal/tracker
  - Snapshot, `Version` and `Diff` models are present. An in-memory tracker scaffold exists; methods currently return an explicit `ErrNotImplemented`. Intended future work: in-memory storage, ID generation, text/DOM diffing, and optional persistent backends.

- internal/analyzer
  - The analyzer client provides a client abstraction and a default HTTP implementation (`default_analyzer.go`). It supports:
    - `SubmitScan`, `GetScan`, `ScanAndWait`, `Health`, `Close`
  - The analyzer client is configurable with an injected `*http.Client` so it can reuse `nethttp` transports when desired.
  - Model types for analyzer payloads/results are available under `internal/models` (ScanRequest, ScanResult, Vulnerability).

Models & interfaces
- Cross-package interfaces (`WebClient`, `Assessor`, `Tracker`, `Analyzer`, etc.) live in their respective packages, and domain models are kept close to their components (for example `internal/tracker/models`, `internal/analyzer/analyzer_models`, and `internal/assessor`). When adding new models, prefer keeping types in the most relevant component package instead of a single global model package.

Testing
- Tests use the external-package pattern (`package foo_test`) to exercise the public API.
- Scaffolding tests are present for most components and assert expected neutral/not-implemented behavior to document intended future behavior.
- If you prefer a global test logger, add a small `internal/logging` noop logger to simplify test setup.

Known caveats and TODOs
- `chromedp` backend needs further hardening (browser lifecycle management, broader test coverage, performance tuning).
- Assessor: extend rule packs, feature extraction, and weighting to produce richer, better-calibrated scores.
- Tracker: add garbage collection for unreachable blobs, pagination/filtering for history queries, and additional performance tuning for large histories.
- Analyzer: add integration tests and a simple reference analyzer (e.g., Python FastAPI) for contract testing.
- Continue to keep model types close to their components to avoid confusion between similarly named types.
