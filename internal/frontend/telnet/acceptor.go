package telnet

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
)

// SessionHandler processes a connected Telnet session.
// Implementations handle the command loop for a single client.
type SessionHandler interface {
	HandleSession(ctx context.Context, conn *Conn) error
}

// Acceptor listens for Telnet connections on a TCP port and dispatches
// each connection to a SessionHandler.
type Acceptor struct {
	cfg      config.TelnetConfig
	handler  SessionHandler
	logger   *zap.Logger
	headless bool

	listener net.Listener
	wg       sync.WaitGroup
	quit     chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewAcceptor creates a Telnet acceptor with the given configuration.
//
// Precondition: cfg must have a valid port; handler and logger must be non-nil.
// Postcondition: Returns an Acceptor ready to be started with ListenAndServe.
func NewAcceptor(cfg config.TelnetConfig, handler SessionHandler, logger *zap.Logger) *Acceptor {
	return &Acceptor{
		cfg:     cfg,
		handler: handler,
		logger:  logger,
		quit:    make(chan struct{}),
	}
}

// NewHeadlessAcceptor creates a Telnet acceptor that wraps each accepted
// connection as a headless plain-text session (no ANSI, no split-screen).
//
// Precondition: cfg must have a valid port; handler and logger must be non-nil.
// Postcondition: Returns an Acceptor ready to be started; all connections will have Headless=true.
func NewHeadlessAcceptor(cfg config.TelnetConfig, handler SessionHandler, logger *zap.Logger) *Acceptor {
	return &Acceptor{
		cfg:      cfg,
		handler:  handler,
		logger:   logger,
		quit:     make(chan struct{}),
		headless: true,
	}
}

// Handler returns the SessionHandler used by this acceptor.
//
// Postcondition: Returns the non-nil handler configured at construction.
func (a *Acceptor) Handler() SessionHandler {
	return a.handler
}

// ListenAndServe starts the TCP listener and accepts connections until Stop is called.
// This method blocks until the acceptor is stopped.
//
// Precondition: The acceptor must not already be running.
// Postcondition: The listener is closed when this method returns.
func (a *Acceptor) ListenAndServe() error {
	start := time.Now()

	listener, err := net.Listen("tcp", a.cfg.Addr())
	if err != nil {
		return fmt.Errorf("listening on %s: %w", a.cfg.Addr(), err)
	}

	a.mu.Lock()
	a.listener = listener
	a.running = true
	a.mu.Unlock()

	a.logger.Info("telnet acceptor listening",
		zap.String("addr", listener.Addr().String()),
		zap.Duration("startup", time.Since(start)),
	)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-a.quit:
				return nil
			default:
				a.logger.Error("accepting connection", zap.Error(err))
				continue
			}
		}

		a.wg.Add(1)
		go a.handleConn(conn)
	}
}

// handleConn processes a single TCP connection.
func (a *Acceptor) handleConn(raw net.Conn) {
	defer a.wg.Done()
	start := time.Now()
	addr := raw.RemoteAddr().String()

	a.logger.Info("client connected",
		zap.String("remote_addr", addr),
	)

	var conn *Conn
	if a.headless {
		conn = NewHeadlessConn(raw, a.cfg.ReadTimeout, a.cfg.WriteTimeout)
	} else {
		conn = NewConn(raw, a.cfg.ReadTimeout, a.cfg.WriteTimeout)
	}
	defer conn.Close()

	if err := conn.Negotiate(); err != nil {
		a.logger.Error("telnet negotiation failed",
			zap.String("remote_addr", addr),
			zap.Error(err),
		)
		return
	}

	// Wait up to 1 second for NAWS window-size response.
	// Split-screen setup is deferred to gameBridge so the auth/char-select
	// flow can render without a pre-established scroll region.
	conn.AwaitNAWS(time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel context when quit signal received
	go func() {
		select {
		case <-a.quit:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := a.handler.HandleSession(ctx, conn); err != nil {
		a.logger.Debug("session ended",
			zap.String("remote_addr", addr),
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
	} else {
		a.logger.Info("session ended cleanly",
			zap.String("remote_addr", addr),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

// Stop gracefully stops the acceptor, closing the listener and waiting
// for all active sessions to finish.
//
// Postcondition: All connections are closed and goroutines have exited.
func (a *Acceptor) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}
	a.running = false

	close(a.quit)
	if a.listener != nil {
		a.listener.Close()
	}
	a.wg.Wait()

	a.logger.Info("telnet acceptor stopped")
}

// Addr returns the actual listening address, or empty string if not yet listening.
func (a *Acceptor) Addr() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listener != nil {
		return a.listener.Addr().String()
	}
	return ""
}

// IsRunning returns whether the acceptor is currently accepting connections.
func (a *Acceptor) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}
