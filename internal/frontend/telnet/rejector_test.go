package telnet

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestRejector_PrintsRedirectAndCloses verifies REQ-TD-2a / REQ-TD-2b: when
// AllowGameCommands is false, the player port emits a redirect message that
// references the web client and then closes the session.
func TestRejector_PrintsRedirectAndCloses(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := NewRejector("https://example.test", logger)

	// Wire the rejector through a real Conn over a piped TCP pair so we can
	// observe the message and the close behavior end-to-end.
	server, client := net.Pipe()
	defer client.Close()

	conn := NewConn(server, 100*time.Millisecond, time.Second)
	done := make(chan struct{})
	go func() {
		_ = r.HandleSession(context.Background(), conn)
		_ = conn.Close()
		close(done)
	}()

	buf := make([]byte, 1024)
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	n, err := client.Read(buf)
	require.NoError(t, err, "client must receive the rejector banner")
	got := string(buf[:n])
	assert.Contains(t, got, "web client", "banner must reference the web client surface")
	assert.Contains(t, got, "https://example.test", "banner must include the configured web client URL")

	// After the rejector returns, the connection MUST be closed.
	select {
	case <-done:
		// Good: HandleSession returned promptly and the deferred close fired.
	case <-time.After(time.Second):
		t.Fatal("rejector did not close the session within timeout")
	}

	// Subsequent reads must return EOF (or an equivalent net.Pipe closed err).
	_ = client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = client.Read(make([]byte, 1))
	require.Error(t, err, "expected EOF after rejector closed the connection")
}

// TestRejector_DefaultsURLWhenEmpty exercises the empty-URL branch.
func TestRejector_DefaultsURLWhenEmpty(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := NewRejector("", logger)
	assert.True(t, strings.HasPrefix(r.WebClientURL, "http"),
		"empty URL must be replaced with a non-empty placeholder")
}
