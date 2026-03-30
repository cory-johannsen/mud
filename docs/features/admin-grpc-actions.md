# Admin gRPC Actions

**Slug:** admin-grpc-actions
**Status:** planned
**Priority:** 432
**Category:** ui
**Effort:** M

## Overview

Wires the web admin panel's no-op `SessionManager` stub to the gameserver via four new unary gRPC RPCs: list online sessions, kick a player, send a message to a player, and teleport a player to a room. Replaces `noOpSessionManager` in `cmd/webclient/` with a real `grpcSessionManager`.

## Dependencies

- `web-client` — provides the web admin panel and the no-op stubs being replaced

## Spec

`docs/superpowers/specs/2026-03-30-admin-grpc-actions-design.md`
