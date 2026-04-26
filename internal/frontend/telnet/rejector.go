package telnet

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Rejector is a SessionHandler that prints a redirect-to-web-client message
// to the connecting client and closes the connection. It is wired as the
// player-port handler whenever telnet.allow_game_commands is false (the
// default), per the telnet-deprecation rollout (#325).
//
// Precondition: WebClientURL should be a non-empty URL string.
// Postcondition: Connections receive the rejector message and disconnect.
type Rejector struct {
	WebClientURL string
	logger       *zap.Logger
}

// NewRejector constructs a Rejector handler.
//
// Precondition: webClientURL may be empty (a placeholder is substituted);
// logger must be non-nil.
// Postcondition: returns a ready handler implementing SessionHandler.
func NewRejector(webClientURL string, logger *zap.Logger) *Rejector {
	if webClientURL == "" {
		webClientURL = "https://gunchete.local"
	}
	return &Rejector{WebClientURL: webClientURL, logger: logger}
}

// rejectorBanner is the message sent to a connecting telnet client when the
// player surface is disabled. The message intentionally references "the web
// client" so test assertions can match a stable substring.
const rejectorBanner = `
              Gunchete -- Telnet Player Surface Retired

The web client is the supported player surface for new gameplay.
Telnet is retained only for plain-text system debugging.

  Web client: %s

Connection closing.
`

// HandleSession implements SessionHandler. It writes the rejector banner and
// returns; the acceptor's deferred Close shuts the connection.
//
// Precondition: conn must be non-nil and writable.
// Postcondition: the banner has been written; the session ends immediately.
func (r *Rejector) HandleSession(_ context.Context, conn *Conn) error {
	start := time.Now()
	msg := fmt.Sprintf(rejectorBanner, r.WebClientURL)
	if err := conn.Write([]byte(msg)); err != nil {
		r.logger.Debug("rejector write failed",
			zap.String("remote_addr", conn.RemoteAddr().String()),
			zap.Error(err),
		)
		return nil
	}
	r.logger.Info("telnet player connection rejected",
		zap.String("remote_addr", conn.RemoteAddr().String()),
		zap.Duration("elapsed", time.Since(start)),
	)
	return nil
}
