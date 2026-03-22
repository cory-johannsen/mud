# Claude Game Server Skill Design

## Overview

A Claude Code skill that gives Claude agent sessions direct, programmatic access to the running MUD game server. The skill connects via a dedicated headless telnet port (plain-text, no ANSI/split-screen) using one of three pre-seeded accounts (`claude_player`, `claude_editor`, `claude_admin`). Claude can test game features as a player, create/place content as an editor, and manage world state as an admin.

---

## Requirements

- REQ-CGS-1: A second telnet listener MUST accept connections on a configurable port (`frontend.headless_port`; default 4002). If `headless_port` is 0 or absent, the listener MUST NOT start.
- REQ-CGS-2: Connections on the headless port MUST receive plain `\r\n`-terminated text output with no ANSI escape codes, no cursor positioning, and no split-screen layout.
- REQ-CGS-3: `InitScreen` on a headless connection MUST be a no-op.
- REQ-CGS-4: Room views on a headless connection MUST be rendered as: room name (line 1), description (wrapped lines), exits (one line, format: `Exits: north south east`), and a blank line.
- REQ-CGS-5: Console messages on a headless connection MUST be written as plain lines terminated with `\r\n`.
- REQ-CGS-6: The prompt on a headless connection MUST be `> ` with no ANSI codes.
- REQ-CGS-7: Three accounts MUST be seeded in PostgreSQL by `cmd/seed-claude-accounts/main.go`: `claude_player` (role: player), `claude_editor` (role: editor), `claude_admin` (role: admin). The password is read from `CLAUDE_ACCOUNT_PASSWORD` environment variable (fatal error if absent).
- REQ-CGS-8: Each seeded account MUST be idempotent — running the tool twice MUST NOT create duplicates (upsert semantics).
- REQ-CGS-9: The skill document at `.claude/skills/mud-gameserver.md` MUST document: connection command, account/password selection, session flow (prompts to expect), command reference by role, and three example workflows.

---

## Architecture

### Headless Mode on `telnet.Conn`

Add `Headless bool` to `telnet.Conn`. When `true`:

- `InitScreen()` — no-op
- `WriteRoom(content string)` — strip ANSI from `content`; write as plain lines followed by `\r\n`
- `WriteConsole(text string)` — write `text + "\r\n"` directly to the TCP conn
- `WritePrompt(text string)` — write `"> "` (no ANSI)
- Color/formatting helpers — all ANSI stripping delegated to a new `StripANSI(s string) string` utility in `telnet/ansi.go`

No new interface extraction is needed. The existing handlers (`auth.go`, `game_bridge.go`, `bridge_handlers.go`) call the same `Conn` methods unchanged; headless behavior is encapsulated entirely in `Conn`.

### Second Acceptor

`TelnetConfig` gains a `HeadlessPort int` field. In `cmd/frontend/main.go`, if `cfg.Telnet.HeadlessPort != 0`, a second `telnet.Acceptor` is started on that port. The acceptor sets `conn.Headless = true` after wrapping the raw TCP connection. All other acceptor logic is identical.

### Account Seeding Tool

`cmd/seed-claude-accounts/main.go` — mirrors `cmd/setrole/main.go`. Accepts `-config` flag. Reads `CLAUDE_ACCOUNT_PASSWORD` from env. Uses `internal/storage/postgres/AccountRepository` to upsert the three accounts. Exits 0 on success, 1 on error.

### Skill Document

`.claude/skills/mud-gameserver.md` — a reference skill following the `mud-*.md` convention. Contains:
- Connection: `nc localhost 4002` or `telnet localhost 4002`
- Account selection table (role → username)
- Session flow: prompts to expect at login, character select, room entry
- Plain-text output format: how to read room name, description, exits, messages
- Command reference: player, editor, admin commands with expected responses
- Three example workflows: verify NPC spawn, place item in room, test combat round

---

## File Map

| File | Change |
|------|--------|
| `internal/frontend/telnet/conn.go` | Add `Headless bool` field; branch in `InitScreen`, `WriteRoom`, `WriteConsole`, `WritePrompt` |
| `internal/frontend/telnet/ansi.go` | Add `StripANSI(s string) string` utility |
| `internal/frontend/telnet/conn_test.go` | Add headless mode tests |
| `internal/frontend/telnet/acceptor.go` | Pass `Headless bool` option when wrapping conn |
| `internal/config/config.go` | Add `HeadlessPort int` to `TelnetConfig` |
| `cmd/frontend/main.go` | Start second acceptor when `HeadlessPort != 0` |
| `cmd/seed-claude-accounts/main.go` | New tool — upsert three claude accounts |
| `.claude/skills/mud-gameserver.md` | New skill document |

---

## Non-Goals

- No structured JSON output — Claude reads plain text as a human would.
- No gRPC bypass — the headless port still goes through the full telnet→gRPC bridge.
- No automated Claude-as-player tests in the CI pipeline (those are integration tests, separate feature).
- No per-command response parsing library — the skill document describes output format in prose.
