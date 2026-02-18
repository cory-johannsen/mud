// Package gameserver provides the game backend including
// world handlers, chat handlers, and the gRPC bridge service.
//
// The game server uses a custom session manager for room presence and
// broadcasting. When WebSocket transport is added in a future phase,
// Pitaya will be integrated as the native frontend for WebSocket clients.
// For now, the gRPC service handles all game logic dispatch directly.
package gameserver
