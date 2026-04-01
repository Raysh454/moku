# Enumerator Package

The enumerator package provides interfaces and implementations for discovering URLs from various sources.

## Available Enumerators

### Wayback
Fetches historical URLs from web archive sources:
- **Wayback Machine** (web.archive.org) - Historical snapshots of websites
- **Common Crawl** (index.commoncrawl.org) - Large web crawl archive
- **VirusTotal** - Currently not implemented (requires API key)

**Features:**
- Concurrent fetching from all sources
- Automatic deduplication across sources
- Domain filtering (excludes subdomains)
- Graceful error handling (one source failure doesn't stop others)

**Usage:**
```go
import (
    "context"
    "github.com/raysh454/moku/internal/enumerator"
    "github.com/raysh454/moku/internal/logging"
    "github.com/raysh454/moku/internal/webclient"
)

// Create webclient
cfg := webclient.Config{Client: webclient.ClientNetHTTP}
logger := logging.NewStdoutLogger("app")
wc, _ := webclient.NewNetHTTPClient(cfg, logger, nil)

// Create Wayback enumerator
wayback := enumerator.NewWayback(wc, logger)

// Enumerate URLs
urls, err := wayback.Enumerate(context.Background(), "https://example.com", nil)
```

### Sitemap
Fetches URLs from XML sitemaps (sitemap.xml, sitemap_index.xml).

### Robots
Extracts paths from robots.txt files (Disallow, Allow, Sitemap directives).

### Spider
Crawls a website up to a specified depth, following links.

### Composite
Combines multiple enumerators and deduplicates results.

**Usage:**
```go
// Combine multiple enumerators
wayback := enumerator.NewWayback(wc, logger)
robots := enumerator.NewRobots(wc, logger)
sitemap := enumerator.NewSitemap(wc, logger)

composite := enumerator.NewComposite(
    []enumerator.Enumerator{wayback, robots, sitemap},
    logger,
)

// Get URLs from all sources
allURLs, err := composite.Enumerate(context.Background(), "https://example.com", nil)
```

## Configuration

### Wayback Configuration
For testing or custom archive endpoints:
```go
cfg := &enumerator.WaybackConfig{
    WaybackBaseURL: "http://custom-archive.org",  // Optional
    CCBaseURL:      "http://custom-cc.org",       // Optional
    VTBaseURL:      "",                           // Optional
}
wayback := enumerator.NewWaybackWithConfig(wc, logger, cfg)
```

## Progress Callbacks
All enumerators support optional progress callbacks:
```go
callback := func(current, total int) {
    fmt.Printf("Progress: %d/%d\n", current, total)
}
urls, err := wayback.Enumerate(ctx, target, callback)
```

## Domain Filtering
The Wayback enumerator automatically filters results to only include URLs from the exact target domain (no subdomains). For example, when enumerating `https://example.com`:
- ✅ `https://example.com/admin` - Included
- ✅ `https://example.com/api/v1` - Included
- ❌ `https://sub.example.com/page` - Excluded (subdomain)
- ❌ `https://other.com/page` - Excluded (different domain)
