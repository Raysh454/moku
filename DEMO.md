# Moku Demo Runbook

This guide shows how to run the full demo stack cleanly:

- Moku API server (port `8080`)
- Demo website server (port `9999`)
- Isolated React GUI (port `5173` by default)

You do **not** need to manually drive Swagger for the main demo flow anymore.

---

## 1) Prerequisites

- Go installed and available in `PATH`
- Node.js 18+ and npm

Optional but useful:
- `make` (for Swagger/doc regeneration and helper targets)

---

## 2) Start the backend services

Open two terminals at repo root.

### Terminal A — API server

```bash
go run .
```

Default API base URL:

- `http://localhost:8080`

Swagger remains available at:

- `http://localhost:8080/swagger/index.html`

### Terminal B — Demo website server

```bash
go run ./cmd/demoserver
```

Default demo base URL:

- `http://localhost:9999`

Demo control page (optional direct view):

- `http://localhost:9999/demo/control`

---

## 3) Start the isolated GUI

Open a third terminal:

```bash
cd ui
```

First time only:

```bash
npm install
```

Create local env file (first time only):

```bash
cp .env.example .env
```

If you are on Windows PowerShell and `cp` alias behaves differently, this also works:

```powershell
Copy-Item .env.example .env
```

Run the UI:

```bash
npm run dev
```

Open the URL shown by Vite (usually):

- `http://localhost:5173`

---

## 4) Recommended demo flow in the GUI

In order:

1. **Projects panel**
   - Create/select project (e.g. `demo-ui`).
2. **Websites panel**
   - Create/select website (origin should be `http://localhost:9999`).
3. **Jobs panel**
   - Click **Enumerate (REST)**.
   - Click **Fetch (REST)** (or **Fetch (WebSocket)**).
4. **Endpoints panel**
   - Load endpoints and choose one (start with `/`).
5. **Endpoint details panel**
   - Review snapshot, score, security diff, and content/header diff.
6. **Demo controls panel**
   - Click **Bump all versions**.
7. **Jobs panel**
   - Run **Fetch** again.
8. **Endpoint details panel**
   - Re-open the same endpoint and observe updated body/headers/diff/security changes.

Use:
- **Activity log** for action history
- **Raw JSON inspector** for exact backend payloads

---

## 5) Run automated verification (optional)

You can verify the same flow with automated e2e test:

```bash
go test -count=1 ./internal/server -run TestDemoE2E_HappyPath -v
```

---

## 5) Understanding Version Tracking

### Version Tracking Concepts

Moku uses a git-like version control system for website snapshots:

- **Version**: A commit representing the complete state of tracked pages at a point in time
- **Snapshot**: A captured web page (HTML + headers) belonging to a version
- **HEAD**: Points to the current version
- **Parent**: Each version (except the first) has a parent, creating a history chain

### Single Version Per Fetch

When you fetch multiple pages through the GUI or API, Moku creates **ONE version** for the entire operation, regardless of how many pages are fetched:

**Example**: Fetching 2500 pages
```
Before (old behavior):
├─ Batch 1 (pages 1-1024)    → Version V1 @ 10:00:01
├─ Batch 2 (pages 1025-2048) → Version V2 @ 10:00:03
└─ Batch 3 (pages 2049-2500) → Version V3 @ 10:00:05
Result: 3 versions in timeline

After (current behavior):
└─ Fetch 2500 pages → Version V1 @ 10:00:01 (2500 snapshots)
Result: 1 version in timeline
```

### Timeline Example

```
Initial commit           Fetch 100 pages        Update 50 pages
      ↓                       ↓                      ↓
   Version 1            Version 2              Version 3
   (5 pages)           (100 pages)             (50 pages)
      │                     │                      │
   Parent: -           Parent: V1             Parent: V2
```

Each version contains:
- Unique version ID (UUID)
- Commit message (e.g., "Fetch 100 pages")
- Author (e.g., "fetcher")
- Timestamp
- All snapshots added in that operation
- Computed diffs from parent version

### Viewing Version History

Through the API:

```bash
# List recent versions
curl http://localhost:8080/api/v1/projects/{project}/versions

# Get all snapshots for a version
curl http://localhost:8080/api/v1/projects/{project}/versions/{versionId}/snapshots

# Get diff between two versions
curl http://localhost:8080/api/v1/projects/{project}/diff?base={baseVersionId}&head={headVersionId}
```

### Storage Efficiency

Moku uses **content-addressed storage** with blob deduplication:

- Identical page content (same HTML) → stored once, reused across versions
- Only metadata (snapshot records) duplicates, not the actual HTML content
- Example: Fetching the same homepage 100 times → 1 blob stored, 100 tiny snapshot records

This makes it efficient to track many versions without exploding storage.

---

## 6) Troubleshooting

### Port already in use

- API (`8080`), demo server (`9999`), or UI (`5173`) may be occupied.
- Stop conflicting processes or run on alternate ports.

### UI cannot reach API/demo server

- Confirm both Go servers are running.
- Confirm `.env` values in `ui/.env`:
  - `VITE_API_BASE_URL=http://localhost:8080`
  - `VITE_DEMO_BASE_URL=http://localhost:9999`

### No endpoints after enumerate/fetch

- Ensure website origin points to running demo server.
- Re-run enumerate, then fetch with status `*`.
- Use **Raw JSON inspector** and **Activity log** to inspect failures.

### UI build/type issues

From `ui/`:

```bash
npm install
npm run build
```

---

## 7) Shutdown

Stop each terminal with `Ctrl+C`.

---

## 8) Isolation note

The GUI is intentionally isolated under `ui/`.

If you ever want to remove it from the project, delete that directory only.
