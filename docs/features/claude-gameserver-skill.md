# Claude Game Server Skill

A Claude Code skill giving Claude agent sessions direct, programmatic access to the running MUD game server via a dedicated headless telnet port (plain-text, no ANSI/split-screen). Three pre-seeded accounts (`claude_player`, `claude_editor`, `claude_admin`) provide player, editor, and admin access for feature testing and content work. See `docs/superpowers/specs/2026-03-21-claude-gameserver-skill-design.md` for the full design spec.

## Requirements

- REQ-CGS-1: A second telnet listener MUST accept connections on a configurable port (`frontend.headless_port`; default 4002). If `headless_port` is 0 or absent, the listener MUST NOT start.
- REQ-CGS-2: Connections on the headless port MUST receive plain `\r\n`-terminated text output with no ANSI escape codes, no cursor positioning, and no split-screen layout.
- REQ-CGS-3: `InitScreen` on a headless connection MUST be a no-op.
- REQ-CGS-4: Room views on a headless connection MUST be rendered as: room name (line 1), description (wrapped lines), exits (one line, format: `Exits: north south east`), and a blank line.
- REQ-CGS-5: Console messages on a headless connection MUST be written as plain lines terminated with `\r\n`.
- REQ-CGS-6: The prompt on a headless connection MUST be `> ` with no ANSI codes.
- REQ-CGS-7: Three accounts MUST be seeded in PostgreSQL by `cmd/seed-claude-accounts/main.go`: `claude_player` (role: player), `claude_editor` (role: editor), `claude_admin` (role: admin). The password is read as plain-text from `CLAUDE_ACCOUNT_PASSWORD` environment variable (fatal error if absent); the tool MUST hash it via the existing `AccountRepository.Create()` bcrypt path.
- REQ-CGS-8: Each seeded account MUST be idempotent — running the tool twice MUST NOT create duplicates. The tool MUST use a three-step upsert: (1) fetch by username, (2) if absent create via `AccountRepository.Create()`, (3) if present update role via `AccountRepository.SetRole()`.
- REQ-CGS-9: The skill document at `.claude/skills/mud-gameserver.md` MUST document: connection command, account/password selection, session flow (prompts to expect), command reference by role, and three example workflows.
