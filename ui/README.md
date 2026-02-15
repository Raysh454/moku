# Moku Demo UI (Isolated)

This is a fully isolated React + TypeScript GUI for demonstrating Moku using the existing backend API and demo server.

## Isolation guarantee

- Everything for the GUI lives under this `ui/` directory.
- No backend code changes are required for this UI.
- Removing the GUI is as simple as deleting the `ui/` directory.

## Prerequisites

- API server running (default: `http://localhost:8080`)
- Demo server running (default: `http://localhost:9999`)
- Node.js 18+

## Setup

```bash
cd ui
cp .env.example .env
npm install
npm run dev
```

Open the printed URL (usually `http://localhost:5173`).

## Configuration

Environment variables (in `.env`):

- `VITE_API_BASE_URL` (default in example: `/api`)
- `VITE_DEMO_BASE_URL` (default in example: `/demo`)

Notes:

- In local dev, the UI uses Vite proxy routes (`/api`, `/demo`, `/ws`) to avoid browser CORS issues.
- If you already have a `.env` with direct URLs, update it to match `.env.example` or remove it.

## What the UI covers

- Project and website creation/listing
- Enumerate and fetch jobs (REST + fetch via WebSocket)
- Job status table
- Endpoint listing with filters
- Endpoint details (`snapshot`, `score_result`, `security_diff`, `diff`)
- Demo controls (`reset`, `bump-all`, per-path version set)
- Raw JSON inspector and activity log
