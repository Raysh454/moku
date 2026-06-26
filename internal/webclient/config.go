package webclient

type Client string

const (
	ClientNetHTTP  Client = "nethttp"
	ClientChromedp Client = "chromedp"
	// ClientTLS selects the tls-client backend: net/http-class speed with a real
	// Chrome TLS/HTTP2 fingerprint to clear Cloudflare's transport-layer gate.
	ClientTLS Client = "tls"
)

// Config is the minimal interface required for constructing a WebClient.
// It is implemented by app.Config without creating an import cycle.
type Config struct {
	Client Client

	// AllowPrivateHosts disables the SSRF dialer guard, permitting requests to
	// loopback, RFC1918/ULA private, link-local, and unspecified addresses.
	// It defaults to false so the client fails closed against SSRF. This
	// mirrors the sidecar's MOKU_ANALYZER_ALLOW_PRIVATE_HOSTS escape hatch and
	// is intended only for local verification against the demo server; leave it
	// unset in production. In app.DefaultConfig it is populated from the
	// MOKU_ALLOW_PRIVATE_HOSTS environment variable.
	AllowPrivateHosts bool

	// MaxBodyBytes caps the size of a response body the client will read. A
	// response exceeding the cap is rejected with ErrBodyTooLarge rather than
	// truncated, since a partial body would corrupt snapshots and diffs. When
	// left at its zero value the client falls back to DefaultMaxBodyBytes
	// (10 MiB). The cap applies even when a custom *http.Client is injected.
	MaxBodyBytes int64

	// ChromePath, when set, points the chromedp backend at a specific browser
	// binary — e.g. chrome-headless-shell, which renders the identical Blink DOM
	// as full Chrome at a lower binary/RAM/startup cost. Empty uses chromedp's
	// default browser discovery. Ignored by the non-browser backends.
	ChromePath string
}
