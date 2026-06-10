package webclient

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrPrivateHostBlocked is returned when the SSRF guard refuses to dial a host
// that resolves to a loopback, private, link-local, or unspecified IP address.
// It is wrapped, so callers should test with errors.Is.
var ErrPrivateHostBlocked = errors.New("webclient: destination resolves to a disallowed private host")

// ErrBodyTooLarge is returned when a response body exceeds the configured
// MaxBodyBytes cap. The body is rejected outright rather than truncated, so a
// partial read can never corrupt a snapshot or diff. It is wrapped, so callers
// should test with errors.Is.
var ErrBodyTooLarge = errors.New("webclient: response body exceeds maximum allowed size")

// DefaultMaxBodyBytes caps response bodies at 10 MiB when Config.MaxBodyBytes
// is left at its zero value.
const DefaultMaxBodyBytes int64 = 10 << 20

// maxRedirectHops bounds the redirect chain a single request may follow before
// the client gives up, preventing redirect loops from hanging the fetcher.
const maxRedirectHops = 10

// dialGuardTimeout bounds how long the guarded dialer waits to establish a TCP
// connection. It mirrors the overall client timeout so a slow connect cannot
// outlive the request budget.
const dialGuardTimeout = 30 * time.Second

// isDisallowedIP reports whether ip belongs to a range the SSRF guard must
// refuse: loopback, RFC1918/ULA private, link-local (unicast or multicast), or
// the unspecified address. These are the addresses an attacker would target to
// reach internal services, so we never dial them unless the operator has opted
// out via Config.AllowPrivateHosts.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// guardDialControl runs as net.Dialer.Control, which the runtime invokes with
// the post-DNS-resolution address. Enforcing here — after the name has been
// resolved to a concrete ip:port — closes the DNS-rebinding TOCTOU window that
// a pre-dial hostname check would leave open. The same choke point covers the
// origin URL and every redirect hop, since each hop dials through this dialer.
func guardDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: cannot parse dial address %q: %v", ErrPrivateHostBlocked, address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("%w: dial address %q is not a literal IP", ErrPrivateHostBlocked, host)
	}
	if isDisallowedIP(ip) {
		return fmt.Errorf("%w: %s", ErrPrivateHostBlocked, ip)
	}
	return nil
}

// newGuardedHTTPClient builds an *http.Client hardened against SSRF and runaway
// redirects. When cfg.AllowPrivateHosts is false (the default), every dial is
// routed through guardDialControl so private destinations are refused at the
// socket layer. The redirect chain is bounded by maxRedirectHops regardless of
// that setting. The overall 30s timeout is preserved.
func newGuardedHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if !cfg.AllowPrivateHosts {
		dialer := &net.Dialer{
			Timeout: dialGuardTimeout,
			Control: guardDialControl,
		}
		transport.DialContext = dialer.DialContext
	}

	return &http.Client{
		Timeout:   dialGuardTimeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirectHops {
				return fmt.Errorf("stopped after %d redirects", maxRedirectHops)
			}
			return nil
		},
	}
}
