package webclient

import (
	"net"
	"testing"
)

// TestIsDisallowedIP documents which resolved IPs the SSRF guard refuses to
// dial. The disallowed set covers loopback, RFC1918 private ranges,
// link-local, and the unspecified address; the allowed set is ordinary public
// IPv4/IPv6 addresses.
func TestIsDisallowedIP(t *testing.T) {
	cases := []struct {
		name       string
		ip         string
		disallowed bool
	}{
		{"ipv4_loopback", "127.0.0.1", true},
		{"ipv4_private_10", "10.0.0.1", true},
		{"ipv4_private_192_168", "192.168.1.1", true},
		{"ipv4_private_172_16", "172.16.0.1", true},
		{"ipv4_link_local", "169.254.1.1", true},
		{"ipv6_loopback", "::1", true},
		{"ipv6_unique_local", "fc00::1", true},
		{"ipv4_unspecified", "0.0.0.0", true},
		{"ipv4_public", "93.184.216.34", false},
		{"ipv6_public", "2606:2800:220:1:248:1893:25c8:1946", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", tc.ip)
			}
			if got := isDisallowedIP(ip); got != tc.disallowed {
				t.Errorf("isDisallowedIP(%s) = %v, want %v", tc.ip, got, tc.disallowed)
			}
		})
	}
}
