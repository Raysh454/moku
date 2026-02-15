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
