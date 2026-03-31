package attacksurface

import (
	"slices"
	"strings"
)

var benignHeadersExact = []string{
	// General HTTP plumbing
	"date",
	"age",
	"etag",
	"last-modified",
	"vary",
	"accept-ranges",
	"content-length",
	"content-encoding",
	"transfer-encoding",
	"connection",
	"keep-alive",

	// CDN / proxy
	"server",
	"via",
	"x-cache",
	"x-cache-hits",
	"x-served-by",
	"x-timer",
	"cf-ray",
	"cf-cache-status",
	"x-amz-cf-id",
	"x-amz-cf-pop",
	"x-amz-request-id",
	"x-amz-id-2",

	// Trace / correlation IDs
	"x-request-id",
	"x-correlation-id",
	"traceparent",
	"tracestate",
	"x-b3-traceid",
	"x-b3-spanid",
	"x-b3-parentspanid",
	"x-b3-sampled",
	"x-b3-flags",

	// Timing / performance diagnostics
	"server-timing",
	"x-response-time",
	"x-runtime",
}

var benignHeaderPrefixes = []string{
	"x-amz-",
	"x-cdn-",
	"cf-",
	"x-cloud-trace-",
	"x-goog-",
}

func IsBenignHeader(name string) bool {
	lower := strings.ToLower(name)
	if slices.Contains(benignHeadersExact, lower) {
		return true
	}
	for _, prefix := range benignHeaderPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
