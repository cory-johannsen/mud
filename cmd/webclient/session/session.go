// Package session manages a player's WebSocket connection and optional gRPC stream lifetime.
package session

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const (
	defaultPingInterval = 30 * time.Second
	defaultPongTimeout  = 10 * time.Second
)

// Session manages a single player's WebSocket connection and optional gRPC stream.
//
// Precondition: wsConn must be a valid, open WebSocket connection.
// Postcondition: Session is alive until ctx is cancelled, WS closes, or gRPC stream closes.
type Session struct {
	ctx          context.Context
	cancel       context.CancelFunc
	wsConn       *websocket.Conn
	stream       gamev1.GameService_SessionClient // may be nil in tests
	pingInterval time.Duration
	pongTimeout  time.Duration
	wg           sync.WaitGroup
	err          error
	errMu        sync.Mutex
}

// New creates a Session.
//
// Precondition: ctx and cancel must correspond; wsConn must be non-nil.
// Postcondition: Returns a Session ready to have goroutines launched via Run().
func New(ctx context.Context, cancel context.CancelFunc, wsConn *websocket.Conn, stream gamev1.GameService_SessionClient) *Session {
	return &Session{
		ctx:          ctx,
		cancel:       cancel,
		wsConn:       wsConn,
		stream:       stream,
		pingInterval: defaultPingInterval,
		pongTimeout:  defaultPongTimeout,
	}
}

// SetPingInterval overrides the default 30s ping interval (used in tests).
func (s *Session) SetPingInterval(d time.Duration) { s.pingInterval = d }

// SetPongTimeout overrides the default 10s pong timeout (used in tests).
func (s *Session) SetPongTimeout(d time.Duration) { s.pongTimeout = d }

// Run starts the ping/pong keepalive loop.
//
// Postcondition: goroutine launched; call Wait() to block until session ends.
func (s *Session) Run() {
	s.wsConn.SetPongHandler(func(string) error {
		return s.wsConn.SetReadDeadline(time.Time{}) // reset deadline on pong
	})
	s.wg.Add(1)
	go s.pingLoop()
}

// Wait blocks until the session has fully stopped.
func (s *Session) Wait() { s.wg.Wait() }

// Err returns the first non-nil error that caused the session to stop.
// MUST be called after Wait() returns.
func (s *Session) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// Close sends a WebSocket close frame with the given code and cancels the session context.
//
// Postcondition: Context is cancelled; WS close message sent best-effort.
func (s *Session) Close(code int, text string) {
	_ = s.wsConn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, text),
	)
	s.cancel()
}

func (s *Session) setErr(err error) {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

// pingLoop sends a ping every s.pingInterval. If no pong is received within
// s.pongTimeout the session is closed.
func (s *Session) pingLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			_ = s.wsConn.Close()
			return
		case <-ticker.C:
			deadline := time.Now().Add(s.pongTimeout)
			if err := s.wsConn.SetWriteDeadline(deadline); err != nil {
				s.setErr(err)
				s.cancel()
				return
			}
			if err := s.wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.setErr(err)
				s.cancel()
				return
			}
			if err := s.wsConn.SetReadDeadline(deadline); err != nil {
				s.setErr(err)
				s.cancel()
				return
			}
		}
	}
}
