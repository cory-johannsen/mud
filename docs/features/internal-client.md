# Shared Client Library

**Slug:** internal-client
**Status:** spec
**Priority:** 425
**Category:** ui
**Effort:** L

## Overview

`internal/client` is a set of layered Go sub-packages providing the shared protocol, state, and rendering-contract layer for all game client binaries (`cmd/webclient`, `cmd/ebitenclient`). It eliminates duplication across six concerns: authentication, session lifecycle, message feed, character state, command history, and rendering contracts.

## Sub-Packages

| Package | Responsibility |
|---|---|
| `internal/client/render` | `ColorToken` constants, `FeedEntry`, `CharacterSnapshot`, renderer interfaces, `ColorMapper` |
| `internal/client/history` | Command ring buffer (cap 100), ↑/↓ cursor navigation, in-memory only |
| `internal/client/auth` | HTTP client for webclient REST API: login, register, character list/create/options |
| `internal/client/feed` | Goroutine-safe `ServerEvent` accumulation, color token assignment, cap enforcement (default 500) |
| `internal/client/session` | gRPC `GameService.Session` lifecycle, client state machine, reconnect backoff |

## Architecture

See `docs/superpowers/specs/2026-03-26-internal-client-architecture-design.md` for the full design.

## Dependencies

None — this package is a leaf dependency consumed by client binaries.

## Implementation Plan

`docs/superpowers/plans/2026-03-26-internal-client-library.md`
