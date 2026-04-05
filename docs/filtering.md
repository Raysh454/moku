# URL & Response Filtering System

Moku's filtering system allows you to control which URLs are fetched and processed, reducing noise from irrelevant content while preserving security-relevant data.

## Architecture Overview

```
Discovery → Indexer (stores all URLs) → Filter Engine → Fetcher (fetches unfiltered URLs) → Tracker
                                             ↓
                                    Mark filtered URLs with status="filtered"
```

### Key Design Decisions

1. **Store All, Filter Some**: All discovered URLs are stored in the indexer for audit visibility. Filtered URLs are marked with status="filtered" and a reason, not deleted.

2. **Filter at Indexer Level**: Filtering happens when the indexer returns URLs to the fetcher. This provides a single source of truth for filtering logic.

3. **Post-Fetch Status Filtering**: HTTP status codes (e.g., 404) are checked after fetching. URLs returning filtered status codes are marked as filtered instead of being stored as snapshots.

4. **Skip-Only Model**: The system uses skip lists (blocklists) only, not allowlists. This simplifies configuration and makes behavior more predictable.

## Configuration Hierarchy

Filter configuration is hierarchical, with later levels overriding earlier ones:

```
Global Config (app defaults)
    ↓
Website Config (websites.config JSON column)
    ↓  
Filter Rules (filter_rules table)
    ↓
API Overrides (per-request parameters)
```

### Priority System

When multiple rules could match the same URL:
- **Pattern rules** (priority 100) > **Extension rules** (priority 50) > **Status code rules** (priority 25)
- This allows patterns like "skip all .js except vendor libraries"

## Configuration Options

### Skip Extensions

File extensions to skip when fetching. These are typically binary files that don't contain security-relevant content.

Default skip extensions:
- Images: `.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.ico`, `.webp`
- Video: `.mp4`, `.avi`, `.mov`, `.wmv`, `.flv`, `.webm`, `.mkv`
- Audio: `.mp3`, `.wav`, `.ogg`, `.m4a`, `.flac`
- Archives: `.zip`, `.rar`, `.7z`, `.tar.gz`, `.bz2`, `.iso`
- Executables: `.exe`, `.dll`, `.so`, `.dylib`, `.bin`
- Fonts: `.ttf`, `.woff`, `.woff2`, `.eot`, `.otf`
- Documents: `.pdf`, `.doc`, `.docx`, `.ppt`, `.pptx`

**Note**: `.svg` is NOT skipped by default because SVG files can contain JavaScript and are a known XSS vector.

### Skip Patterns

URL patterns (glob syntax) to skip. Examples:
- `*/assets/*` - Skip all URLs containing /assets/
- `*/vendor/*.min.js` - Skip minified vendor JavaScript
- `*.cdn.example.com/*` - Skip CDN URLs

### Skip Status Codes

HTTP status codes to filter after fetching. By default, no status codes are filtered.

Commonly filtered:
- `404` - Not Found (usually dead links)
- `410` - Gone (permanently removed)

**Note**: 401 (Unauthorized) and 403 (Forbidden) are NOT filtered by default as they are security signals indicating protected resources.

## Security Considerations

### Extensions to Keep (Security-Relevant)

These file types may contain vulnerabilities, misconfigurations, or sensitive data:

- **Scripts**: `.js`, `.jsx`, `.ts`, `.mjs` (XSS, API keys, logic flaws)
- **Server-side**: `.php`, `.jsp`, `.asp`, `.aspx`, `.py`, `.rb`, `.go` (injection, RCE)
- **Config**: `.json`, `.xml`, `.yml`, `.yaml`, `.env`, `.config`, `.ini` (credentials, secrets)
- **Data**: `.txt`, `.csv`, `.log`, `.bak` (data exposure, backups)
- **Markup**: `.html`, `.htm`, `.xhtml`, `.svg` (XSS, injection)

### Status Codes to Keep

- **2xx**: All success codes (200, 201, 204, etc.)
- **3xx**: Redirects reveal structure
- **401/403**: Authentication/authorization endpoints
- **405**: Method Not Allowed reveals allowed methods
- **5xx**: Server errors can reveal vulnerabilities and stack traces

## API Reference

### Filter Rules CRUD

#### List Rules
```
GET /projects/{project}/websites/{site}/filters
```

Response:
```json
{
  "rules": [
    {
      "id": "abc123",
      "website_id": "xyz789",
      "rule_type": "extension",
      "rule_value": ".jpg",
      "priority": 50,
      "enabled": true,
      "created_at": 1609459200,
      "updated_at": 1609459200
    }
  ]
}
```

#### Create Rule
```
POST /projects/{project}/websites/{site}/filters

{
  "rule_type": "extension",  // "extension", "pattern", or "status_code"
  "rule_value": ".jpg",      // extension, glob pattern, or status code
  "action": "skip"           // currently only "skip" is supported
}
```

#### Get Rule
```
GET /projects/{project}/websites/{site}/filters/{ruleID}
```

#### Update Rule
```
PUT /projects/{project}/websites/{site}/filters/{ruleID}

{
  "rule_value": ".jpeg",
  "enabled": false
}
```

#### Delete Rule
```
DELETE /projects/{project}/websites/{site}/filters/{ruleID}
```

#### Toggle Rule
```
POST /projects/{project}/websites/{site}/filters/{ruleID}/toggle
```

### Website Filter Config

Quick configuration stored in the website's config JSON column.

#### Get Config
```
GET /projects/{project}/websites/{site}/filters/config
```

Response:
```json
{
  "skip_extensions": [".jpg", ".png"],
  "skip_patterns": ["*/assets/*"],
  "skip_status_codes": [404]
}
```

#### Update Config
```
PUT /projects/{project}/websites/{site}/filters/config

{
  "skip_extensions": [".jpg", ".png", ".gif"],
  "skip_patterns": [],
  "skip_status_codes": [404, 410]
}
```

### Filtered Endpoints

#### List Filtered Endpoints
```
GET /projects/{project}/websites/{site}/endpoints/filtered?limit=100
```

Response:
```json
{
  "endpoints": [
    {
      "url": "https://example.com/image.jpg",
      "filter_reason": "extension:.jpg",
      "filtered_at": 1609459200
    }
  ]
}
```

#### Unfilter Endpoints
```
POST /projects/{project}/websites/{site}/endpoints/unfilter

{
  "urls": ["https://example.com/image.jpg"]
}
```

Or unfilter all:
```json
{
  "all": true
}
```

#### Endpoint Stats
```
GET /projects/{project}/websites/{site}/endpoints/stats
```

Response:
```json
{
  "total": 1000,
  "new": 100,
  "pending": 50,
  "fetched": 800,
  "filtered": 45,
  "failed": 5
}
```

### Fetch with Filter Overrides

When starting a fetch job, you can provide filter overrides:

```
POST /projects/{project}/websites/{site}/jobs/fetch

{
  "max_pages": 100,
  "filter_overrides": {
    "skip_extensions": [".pdf"],
    "skip_status_codes": [404, 410]
  }
}
```

## UI Guide

### Filter Configuration Panel

The Filter Configuration panel is accessible from the website view. It has three tabs:

1. **Rules**: Manage individual filter rules
   - Add new rules by type (extension, pattern, status code)
   - Enable/disable rules without deleting them
   - Delete rules you no longer need

2. **Filtered Endpoints**: View URLs that have been filtered
   - See the filter reason for each URL
   - Unfilter individual URLs or all at once

3. **Config Preview**: View the merged filter configuration
   - Shows the effective configuration combining all sources

### Common Recipes

#### Skip all images
Add extension rules for: `.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.ico`, `.webp`

#### Skip vendor JavaScript but keep application code
1. Add extension rule: skip `.js`
2. Add pattern rule: keep `*/vendor/*.js` (patterns have higher priority)

#### Filter 404 responses
Add status code rule: `404`

#### Skip CDN content
Add pattern rule: `*.cdn.example.com/*`

## Troubleshooting

### Filtered URL Not Appearing in Filtered List
- Check if the URL was filtered before fetch (extension/pattern) or after (status code)
- Pre-fetch filtering marks URLs immediately; status code filtering only happens after attempting to fetch

### Rule Not Being Applied
- Verify the rule is enabled
- Check rule priority - patterns override extensions
- Ensure rule value is correct (extensions need leading dot, patterns use glob syntax)

### Unfiltred URLs Not Being Fetched
- After unflitering, URLs return to "pending" status
- Run a new fetch job to process them

## Database Schema

Filter rules are stored in `registry.db`:

```sql
CREATE TABLE filter_rules (
    id TEXT PRIMARY KEY,
    website_id TEXT NOT NULL,
    rule_type TEXT NOT NULL,     -- "extension", "pattern", "status_code"
    rule_value TEXT NOT NULL,    -- ".jpg", "*/media/*", "404"
    action TEXT NOT NULL,        -- "skip"
    priority INTEGER DEFAULT 0,
    enabled BOOLEAN DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (website_id) REFERENCES websites(id) ON DELETE CASCADE
);
```

Website-level config is stored in the `websites.config` JSON column.

Filtered endpoint status is stored in `endpoints.status = 'filtered'` with the reason in `endpoints.meta` JSON.
