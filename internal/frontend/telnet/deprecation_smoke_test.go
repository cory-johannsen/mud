package telnet

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/config"
)

// TestTelnetDeprecation_SmokeEndToEnd verifies the cross-task contract
// added by #325 entirely in-process:
//
//  1. The player port serves the rejector handler when AllowGameCommands
//     is false.  Connecting yields the redirect message and an EOF.
//  2. The headless port binds 127.0.0.1 only regardless of cfg.Host.
//
// This is the integration smoke test specified in plan Task 9.
func TestTelnetDeprecation_SmokeEndToEnd(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.TelnetConfig{
		Host:         "127.0.0.1",
		Port:         0,
		HeadlessPort: 0,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	// 1. Player port wired to the rejector.
	rejector := NewRejector("https://example.test", logger)
	playerAcceptor := NewAcceptor(cfg, rejector, logger)
	go func() { _ = playerAcceptor.ListenAndServe() }()
	defer playerAcceptor.Stop()
	waitForListen(t, playerAcceptor)

	pConn, err := net.DialTimeout("tcp", playerAcceptor.Addr(), 2*time.Second)
	require.NoError(t, err)

	// Drain initial telnet IAC negotiations and the rejector banner. We
	// explicitly look for the substring that the rejector emits.
	_ = pConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	pConn.Write([]byte{IAC, SB, OptNAWS, 0x00, 0x50, 0x00, 0x18, IAC, SE})
	got := readUntilSubstr(t, pConn, "web client", 2*time.Second)
	assert.Contains(t, got, "web client",
		"player port must emit a web-client redirect message")
	assert.Contains(t, got, "https://example.test",
		"rejector must echo the configured WebClientURL")

	// Connection MUST be closed shortly after the rejector banner.
	_ = pConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = pConn.Read(make([]byte, 1024))
	require.Error(t, err, "player connection must be closed after the redirect")
	pConn.Close()

	// 2. Headless port binds loopback only.
	headlessAcceptor := NewHeadlessAcceptor(cfg, rejector, logger)
	go func() { _ = headlessAcceptor.ListenAndServe() }()
	defer headlessAcceptor.Stop()
	waitForListen(t, headlessAcceptor)

	host, _, err := net.SplitHostPort(headlessAcceptor.Addr())
	require.NoError(t, err)
	addrs, err := net.LookupIP(host)
	require.NoError(t, err)
	loopback := false
	for _, a := range addrs {
		if a.IsLoopback() {
			loopback = true
			break
		}
	}
	assert.True(t, loopback,
		"headless acceptor MUST bind loopback only (got host=%q)", host)
}

// TestHeadlessAcceptor_IgnoresExternalHost verifies REQ-TD-3a: the acceptor
// forces 127.0.0.1 even when cfg.Host is set to a non-loopback address.
func TestHeadlessAcceptor_IgnoresExternalHost(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.TelnetConfig{
		Host:         "0.0.0.0", // would be invalid in prod via Validate(); allowed here for the test
		HeadlessPort: 0,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}
	rejector := NewRejector("", logger)
	a := NewHeadlessAcceptor(cfg, rejector, logger)
	go func() { _ = a.ListenAndServe() }()
	defer a.Stop()
	waitForListen(t, a)

	host, _, err := net.SplitHostPort(a.Addr())
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host,
		"headless acceptor must override cfg.Host to loopback")
}

func waitForListen(t *testing.T, a *Acceptor) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if a.IsRunning() && a.Addr() != "" {
			return
		}
		select {
		case <-deadline:
			t.Fatal("acceptor did not start in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func readUntilSubstr(t *testing.T, c net.Conn, sub string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var sb strings.Builder
	tmp := make([]byte, 4096)
	for time.Now().Before(deadline) {
		_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := c.Read(tmp)
		if n > 0 {
			sb.Write(tmp[:n])
			if strings.Contains(sb.String(), sub) {
				return sb.String()
			}
		}
		if err != nil && !isTimeoutErr(err) {
			return sb.String()
		}
	}
	return sb.String()
}

func isTimeoutErr(err error) bool {
	type timeouter interface{ Timeout() bool }
	te, ok := err.(timeouter)
	return ok && te.Timeout()
}
