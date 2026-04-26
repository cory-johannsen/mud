# Telnet Interface Deprecation

**Slug:** telnet-deprecation
**Status:** done
**Priority:** 490
**Category:** meta
**Effort:** M

## Overview

The web client is the primary player-facing interface. This feature formalises the deprecation of the telnet frontend: the telnet port is retained exclusively for automated testing (HeadlessClient, interactive test suite) and direct debug access via the `claude-gameserver-skill` Skill. No player-facing telnet login path is supported after this change.

## Dependencies

- web-client (primary player interface must be complete)
- interactive-test-suite (headless port must remain functional)
- claude-gameserver-skill (Skill document must be updated)

## Spec

See [docs/superpowers/specs/2026-04-13-telnet-deprecation-design.md](../superpowers/specs/2026-04-13-telnet-deprecation-design.md)

## Plan

See [docs/superpowers/plans/2026-04-26-telnet-deprecation.md](../superpowers/plans/2026-04-26-telnet-deprecation.md)

## Implementation summary (#325)

- Telnet player port (4000) now serves a `Rejector` handler by default that
  emits a redirect-to-web-client message and disconnects.
- Telnet headless port (4002) is bound to `127.0.0.1` only and accepts only
  seed-bootstrapped accounts (`claude_player`, `claude_editor`, `claude_admin`).
- Helm `frontend` Service is `ClusterIP` (port `4002` only); the previous
  `LoadBalancer:30400` exposure is removed.
- `telnet.allow_game_commands` (default `false`) re-enables the legacy player
  flow for time-bounded graceful sunset operations. MUST NOT be enabled in
  production.
- The `internal/frontend/handlers` package remains importable for the sunset
  flow but is no longer wired by default in production builds.
