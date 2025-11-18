# WebClient and Logger Usage Guide

This document explains how to use the WebClient wrapper and Logger interface in the moku project.

## Overview

All modules now use:
- `interfaces.WebClient` for HTTP/browser operations instead of direct `net/http` or `chromedp`
- `interfaces.Logger` for structured logging instead of `fmt.Printf`

## Quick Start

### 1. Register Backends (in main.go or init())

```go
import "github.com/raysh454/moku/internal/webclient"

func main() {
    // Register default backends (nethttp and chromedp)
    webclient.RegisterDefaultBackends()
    
    // ... rest of your app initialization
}
```

### 2. Create a WebClient Instance

```go
import (
    "github.com/raysh454/moku/internal/webclient"
    "github.com/raysh454/moku/internal/interfaces"
)

// For nethttp backend:
cfg := map[string]interface{}{
    "backend": "nethttp",
    "timeout": 30, // seconds
}

wc, err := webclient.NewWebClient(cfg, logger)
if err != nil {
    log.Fatal(err)
}
defer wc.Close()

// For chromedp backend:
cfg := map[string]interface{}{
    "backend": "chromedp",
    "idle_after": 2, // seconds
    "headless": true,
}

wc, err := webclient.NewWebClient(cfg, logger)
if err != nil {
    log.Fatal(err)
}
defer wc.Close()
```

### 3. Pass WebClient and Logger to Modules

```go
import (
    "github.com/raysh454/moku/internal/fetcher"
    "github.com/raysh454/moku/internal/enumerator"
)

// Create fetcher
f, err := fetcher.New("/path/to/storage", 8, wc, logger)
if err != nil {
    log.Fatal(err)
}

// Create spider
spider := enumerator.NewSpider(2, wc, logger) // depth=2

// Use them
urls, err := spider.Enumerate("https://example.com")
if err != nil {
    log.Fatal(err)
}

f.Fetch(urls)
```

## WebClient Interface

```go
type WebClient interface {
    // Do executes a request with full control
    Do(ctx context.Context, req *model.Request) (*model.Response, error)
    
    // Get is a convenience method for simple GET requests
    Get(ctx context.Context, url string) (*model.Response, error)
    
    Close() error
}
```

### Making Requests

```go
import (
    "context"
    "github.com/raysh454/moku/internal/model"
)

// Simple GET
resp, err := wc.Get(context.Background(), "https://example.com")
if err != nil {
    return err
}

// Custom request
req := &model.Request{
    Method: "POST",
    URL: "https://example.com/api",
    Headers: http.Header{
        "Content-Type": []string{"application/json"},
    },
    Body: []byte(`{"key":"value"}`),
}

resp, err := wc.Do(context.Background(), req)
if err != nil {
    return err
}

// Response fields
statusCode := resp.StatusCode
headers := resp.Headers
body := resp.Body
fetchedAt := resp.FetchedAt
```

## Logger Interface

```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    With(fields ...Field) Logger
}
```

### Logging Examples

```go
import "github.com/raysh454/moku/internal/interfaces"

// Simple log
logger.Info("starting fetch operation")

// Log with fields
logger.Error("failed to fetch page",
    interfaces.Field{Key: "url", Value: pageUrl},
    interfaces.Field{Key: "error", Value: err})

// Create child logger with persistent fields
jobLogger := logger.With(
    interfaces.Field{Key: "job_id", Value: "12345"},
    interfaces.Field{Key: "target", Value: "example.com"})

jobLogger.Info("job started") // Will include job_id and target fields
```

## Registering Custom Backends

You can register custom backends:

```go
import "github.com/raysh454/moku/internal/webclient"

func init() {
    webclient.RegisterBackend("my-custom-backend", func(cfg map[string]interface{}, logger interfaces.Logger) (interfaces.WebClient, error) {
        // Create and return your custom WebClient implementation
        return &MyCustomClient{}, nil
    })
}
```

## Migration Notes

### Before (old code):
```go
// Direct net/http usage
resp, err := http.Get(url)
if err != nil {
    fmt.Printf("error: %v\n", err)
    return err
}
defer resp.Body.Close()
body, _ := io.ReadAll(resp.Body)
```

### After (new code):
```go
// Using WebClient and Logger
resp, err := wc.Get(ctx, url)
if err != nil {
    logger.Error("failed to fetch",
        interfaces.Field{Key: "url", Value: url},
        interfaces.Field{Key: "error", Value: err})
    return err
}
body := resp.Body
```

## Backend Selection

Choose the right backend for your use case:

- **nethttp**: Use for static pages, APIs, or when JavaScript execution is not needed. Fast and lightweight.
- **chromedp**: Use for JavaScript-heavy pages that need rendering. Slower but handles dynamic content.

## Error Handling

Always check for backend registration errors:

```go
wc, err := webclient.NewWebClient(cfg, logger)
if err != nil {
    // Handle error - possibly backend not registered
    logger.Error("failed to create webclient",
        interfaces.Field{Key: "error", Value: err})
    return err
}
defer wc.Close()
```

## Testing

In tests, create a simple webclient:

```go
import "github.com/raysh454/moku/internal/webclient"

func TestMyFunction(t *testing.T) {
    wc := webclient.NewNetHTTPClient(nil) // uses default http.Client
    
    // Pass to your function/module
    spider := enumerator.NewSpider(1, wc, nil)
    
    // Test...
}
```
