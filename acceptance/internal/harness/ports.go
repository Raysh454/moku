package harness

import (
	"net"
	"testing"
)

// FreePort reserves an ephemeral loopback port and releases it for the
// subprocess about to bind it.
func FreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("harness: listen on ephemeral port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
