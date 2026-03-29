package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/cmd/webclient/session"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// echoServer upgrades, reads one message, echoes it, then waits for close.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		conn.SetPingHandler(func(appData string) error {
			return conn.WriteMessage(websocket.PongMessage, []byte(appData))
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
}

func TestSession_CancelContextClosesSession(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	wsConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	sess := session.New(ctx, cancel, wsConn, nil)

	done := make(chan struct{})
	go func() {
		sess.Wait()
		close(done)
	}()

	cancel() // trigger shutdown
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("session did not shut down after context cancel")
	}
}

func TestSession_PingInterval(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	wsConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	sess := session.New(ctx, cancel, wsConn, nil)
	sess.SetPingInterval(50 * time.Millisecond)
	sess.SetPongTimeout(30 * time.Millisecond)

	// Session should run without error for a short period while pongs arrive.
	done := make(chan struct{})
	go func() {
		sess.Wait()
		close(done)
	}()

	select {
	case <-done:
		// ok — context expired cleanly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("session did not exit after context timeout")
	}
	assert.NoError(t, sess.Err())
}
