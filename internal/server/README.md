# Server API Routes

This document describes the HTTP and WebSocket routes exposed by `internal/server`. All paths are relative to the server base URL (e.g. `http://localhost:8080`).

## Projects

### POST /projects
Create a new project.

Request body (JSON):
```json
{
  "slug": "optional-short-identifier",   // optional; normalized if provided; derived from name if empty
  "name": "Human project name",          // optional but recommended; if empty, derived from slug
  "description": "Optional description"  // optional
}
```

Response `201 Created` (JSON):
```json
{
  "id": "<uuid>",
  "slug": "<normalized-slug>",
  "name": "<name>",
  "description": "<description>",
  "created_at": 1730000000,
  "meta": "{}"
}
```

Errors:
- `400 Bad Request` – invalid JSON body.
- `500 Internal Server Error` – database or registry failure.

### GET /projects
List all projects.

Response `200 OK` (JSON array of projects):
```json
[
  {
    "id": "<uuid>",
    "slug": "<slug>",
    "name": "<name>",
    "description": "<description>",
    "created_at": 1730000000,
    "meta": "{}"
  }
]
```

Errors:
- `500 Internal Server Error` – registry/list failure.

## Websites

Websites are created under a project identified by slug or id via `{project}`.

### POST /projects/{project}/websites
Create a website under the given project.

Path params:
- `project` – project slug or id.

Request body (JSON):
```json
{
  "slug": "optional-website-slug",  // optional; normalized if provided; derived from origin if empty
  "origin": "https://example.com"   // required; base origin URL
}
```

Response `201 Created` (JSON):
```json
{
  "id": "<uuid>",
  "project_id": "<project-uuid>",
  "slug": "<normalized-slug>",
  "origin": "https://example.com",
  "created_at": 1730000000,
  "meta": "{}"
}
```

Errors:
- `400 Bad Request` – invalid JSON body.
- `500 Internal Server Error` – project not found, invalid origin, or registry error.

### GET /projects/{project}/websites
List websites for a project.

Path params:
- `project` – project slug or id.

Response `200 OK` (JSON array of websites):
```json
[
  {
    "id": "<uuid>",
    "project_id": "<project-uuid>",
    "slug": "<slug>",
    "origin": "https://example.com",
    "created_at": 1730000000,
    "meta": "{}"
  }
]
```

Errors:
- `500 Internal Server Error` – registry/list failure.

## Endpoints

Endpoints are URLs tracked under a website.

### POST /projects/{project}/websites/{site}/endpoints
Add endpoints (URLs) to a website.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Request body (JSON):
```json
{
  "urls": ["https://example.com/path1", "https://example.com/path2"],
  "source": "api"  // optional; defaults to "api" if empty
}
```

Response `201 Created` (JSON):
```json
{
  "added": ["https://example.com/path1", "https://example.com/path2"]
}
```

Errors:
- `400 Bad Request` – invalid JSON body.
- `500 Internal Server Error` – registry lookup failure or index error.

### GET /projects/{project}/websites/{site}/endpoints
List endpoints for a website.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Query params:
- `status` (optional) – filter by endpoint status as understood by the indexer (e.g. `new`, `fetched`, `error`, `*` for all; empty passes through as-is).
- `limit` (optional) – positive integer to limit number of results; ignored if <= 0 or invalid.

Response `200 OK` (JSON array of endpoints):

The exact shape is defined by `indexer.Endpoint` and includes at least a canonical URL and status; consult `internal/indexer` for full schema.

Errors:
- `500 Internal Server Error` – registry or index error.

### GET /projects/{project}/websites/{site}/endpoints/details
Get detailed information for a single endpoint.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Query params:
- `url` – **required** canonical URL of the endpoint.

Response `200 OK` (JSON):
```json
{
  "snapshot": { /* tracker Snapshot model */ },
  "score_result": { /* assessor.ScoreResult, may be null */ },
  "diff_with_prev": { /* assessor.SecurityDiff, may be null */ },
  "diff": { /* tracker.CombinedFileDiff, may be null */ }
}
```

Errors:
- `400 Bad Request` – missing `url` query parameter.
- `500 Internal Server Error` – registry or tracker failure.

## Jobs (REST)

Jobs represent long‑running operations (fetch and enumerate) associated with a website. Jobs are started via REST and can be monitored over REST or WebSocket.

### POST /projects/{project}/websites/{site}/jobs/fetch
Start a fetch job over endpoints for a website.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Request body (JSON, optional; defaults applied when fields missing):
```json
{
  "status": "new",  // optional; default "new"; passed to FetchWebsiteEndpoints; "*" means all when empty inside orchestrator
  "limit": 100        // optional; default 100 when <= 0
}
```

Response `202 Accepted` (JSON job object):
```json
{
  "id": "<uuid>",
  "type": "fetch",
  "project": "<project-identifier>",
  "website": "<website-slug>",
  "status": "pending|running|done|failed|canceled",
  "error": "optional error string",
  "started_at": "2025-01-01T00:00:00Z",
  "ended_at": "2025-01-01T00:00:10Z",
  "security_overview": { /* assessor.SecurityDiffOverview, may be null */ },
  "enumerated_urls": null
}
```

Notes:
- Body is decoded but missing/zero values are replaced by defaults; invalid JSON is tolerated (ignored) in handler by design.

Errors:
- `500 Internal Server Error` – orchestrator unable to start job (e.g., closed or registry error).

### POST /projects/{project}/websites/{site}/jobs/enumerate
Start an enumerate job to discover URLs for a website.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Request body (JSON, optional):
```json
{
  "concurrency": 4  // optional; default 4 when <= 0
}
```

Response `202 Accepted` (JSON job object):
```json
{
  "id": "<uuid>",
  "type": "enumerate",
  "project": "<project-identifier>",
  "website": "<website-slug>",
  "status": "pending|running|done|failed|canceled",
  "error": "optional error string",
  "started_at": "2025-01-01T00:00:00Z",
  "ended_at": "2025-01-01T00:00:10Z",
  "security_overview": null,
  "enumerated_urls": ["https://example.com/"]
}
```

Errors:
- `500 Internal Server Error` – orchestrator unable to start job.

### GET /jobs
List all currently known jobs.

Response `200 OK` (JSON array of jobs):
```json
[
  {
    "id": "<uuid>",
    "type": "fetch|enumerate",
    "project": "<project-identifier>",
    "website": "<website-slug>",
    "status": "pending|running|done|failed|canceled",
    "error": "optional error string",
    "started_at": "2025-01-01T00:00:00Z",
    "ended_at": "2025-01-01T00:00:10Z",
    "security_overview": { /* optional */ },
    "enumerated_urls": ["..."]
  }
]
```

Notes:
- Jobs may be evicted after `JobRetentionTime` (see `app.Config`).

### GET /jobs/{jobID}
Get a single job by ID.

Path params:
- `jobID` – job UUID.

Response `200 OK` (JSON job object, same shape as above).

Errors:
- `404 Not Found` – job not present (unknown or already cleaned up).

### DELETE /jobs/{jobID}
Cancel a running job.

Path params:
- `jobID` – job UUID.

Response `204 No Content` – cancellation requested (idempotent; no error if job gone).

## WebSocket Routes

WebSocket routes stream job events (status/progress/results) as JSON messages.

### GET /ws/projects/{project}/websites/{site}/fetch
Open a WebSocket that starts a fetch job and streams its events.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Query params:
- `status` (optional) – same semantics as REST fetch job; default `new` if empty.
- `limit` (optional) – positive integer; default 100; invalid/non‑positive ignored.

Behavior:
- Connection upgrade starts a fetch job internally (same semantics as POST jobs/fetch).
- First message is the job object; subsequent messages are `JobEvent` JSON objects of shape:
  ```json
  {
    "job_id": "<uuid>",
    "type": "status|progress|result",
    "status": "pending|running|done|failed|canceled",   // for status/result events
    "error": "optional error message",                  // for failure/cancel events
    "processed": 10,                                     // for progress events
    "total": 100                                         // for progress events
  }
  ```
- On write error (e.g. client disconnect), the server cancels the job.

### GET /ws/projects/{project}/websites/{site}/enumerate
Open a WebSocket that starts an enumerate job and streams its events.

Path params:
- `project` – project slug or id.
- `site` – website slug.

Query params:
- `concurrency` (optional) – positive integer; default 4; invalid/non‑positive ignored.

Behavior:
- Connection upgrade starts an enumerate job (same semantics as POST jobs/enumerate).
- First message is the job object; subsequent messages are `JobEvent` JSON objects as above.
- On write error, the server cancels the job.

## Notes

- Authentication, authorization, and CORS are not enforced in `internal/server` as written; callers should front this with appropriate middleware in production.
- Error messages are passed through from underlying orchestrator/registry components and are primarily intended for debugging and internal tooling rather than public exposure.
