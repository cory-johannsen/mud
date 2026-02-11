package telnet

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/config"
)

// echoHandler is a test SessionHandler that echoes lines back to the client.
type echoHandler struct {
	sessionCount atomic.Int32
}

func (h *echoHandler) HandleSession(_ context.Context, conn *Conn) error {
	h.sessionCount.Add(1)
	for {
		line, err := conn.ReadLine()
		if err != nil {
			return err
		}
		if line == "quit" {
			_ = conn.WriteLine("bye")
			return nil
		}
		_ = conn.WriteLine("echo: " + line)
	}
}

func TestAcceptorStartAndStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	handler := &echoHandler{}
	cfg := config.TelnetConfig{
		Host:         "127.0.0.1",
		Port:         0, // random port
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	acc := NewAcceptor(cfg, handler, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- acc.ListenAndServe()
	}()

	// Wait for the acceptor to start listening
	deadline := time.After(2 * time.Second)
	for {
		if acc.IsRunning() && acc.Addr() != "" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("acceptor did not start in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	addr := acc.Addr()
	require.NotEmpty(t, addr)

	// Connect a client
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)

	// Read initial telnet negotiations (IAC sequences)
	buf := make([]byte, 256)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Read(buf)

	// Send a message
	_, err = conn.Write([]byte("hello\r\n"))
	require.NoError(t, err)

	// Read response
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Contains(t, string(buf[:n]), "echo: hello")

	// Quit
	_, _ = conn.Write([]byte("quit\r\n"))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ = conn.Read(buf)
	assert.Contains(t, string(buf[:n]), "bye")

	conn.Close()

	// Stop acceptor
	acc.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("acceptor did not stop in time")
	}

	assert.Equal(t, int32(1), handler.sessionCount.Load())
}

func TestAcceptorMultipleClients(t *testing.T) {
	logger := zaptest.NewLogger(t)
	handler := &echoHandler{}
	cfg := config.TelnetConfig{
		Host:         "127.0.0.1",
		Port:         0,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	acc := NewAcceptor(cfg, handler, logger)

	go func() {
		_ = acc.ListenAndServe()
	}()

	// Wait for the acceptor to start
	deadline := time.After(2 * time.Second)
	for {
		if acc.IsRunning() && acc.Addr() != "" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("acceptor did not start in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	addr := acc.Addr()

	// Connect multiple clients
	const numClients = 3
	conns := make([]net.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		require.NoError(t, err)
		conns[i] = conn
		// Read negotiation
		buf := make([]byte, 256)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = conn.Read(buf)
	}

	// Each client quits
	for _, conn := range conns {
		_, _ = conn.Write([]byte("quit\r\n"))
		buf := make([]byte, 256)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = conn.Read(buf)
		conn.Close()
	}

	// Give sessions time to complete
	time.Sleep(100 * time.Millisecond)

	acc.Stop()
	assert.Equal(t, int32(numClients), handler.sessionCount.Load())
}
