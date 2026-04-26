// Package telnet provides the MUD's telnet acceptor and Conn primitives.
//
// As of telnet-deprecation #325, the package is retained for headless
// test/debug access only. The web client is the supported player surface;
// the player-facing telnet flow has been retired.
//
// Wiring:
//   - The "player" Acceptor wires a Rejector handler by default. The
//     handler emits a redirect-to-web-client message and closes. Operators
//     may temporarily re-enable the legacy player flow via the
//     telnet.allow_game_commands config flag for graceful sunset.
//   - The HeadlessAcceptor binds 127.0.0.1 only (regardless of cfg.Host)
//     and routes connections to the seed-authorized debug surface.
//
// See docs/superpowers/specs/2026-04-13-telnet-deprecation-design.md and
// docs/features/telnet-deprecation.md for the broader context.
package telnet
