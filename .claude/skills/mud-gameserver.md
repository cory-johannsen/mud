# MUD Game Server Skill

Connect to the running MUD game server via the headless telnet port (4002) for feature testing and content work. This port outputs plain text — no ANSI escape codes, no split-screen layout — making it suitable for programmatic interaction.

---

## Connection

```bash
nc localhost 4002
# or
telnet localhost 4002
```

The server responds with a `Username:` prompt immediately on connect.

---

## Account Selection

| Role | Username | Use For |
|------|----------|---------|
| player | `claude_player` | Testing gameplay: movement, combat, inventory |
| editor | `claude_editor` | Placing content: items, NPCs, room descriptions |
| admin | `claude_admin` | World management: state inspection, spawning |

Password: set by `CLAUDE_ACCOUNT_PASSWORD` when `make seed-claude-accounts` was run.

---

## Session Flow

### Login

```
Username: claude_player
Password: <password>
```

On success the server sends the character selection menu. If no characters exist, type `new` to create one.

### After Character Select

```
<Room Name>
<Description>
Exits: <direction list>

>
```

The `> ` prompt means the server awaits a command.

### Prompt Recognition

- `Username: ` — awaiting login
- `Password: ` — awaiting password
- `> ` — in-game, awaiting command

---

## Plain-Text Output Format

Room output after entering a room or typing `look`:

```
<Room Name>
<Description line 1>
<Description line 2 if wrapped>
Exits: north south east

>
```

Console messages appear as plain lines:

```
You strike the guard for 12 damage.
The guard attacks you for 8 damage.
>
```

---

## Command Reference

| Command | Description |
|---------|-------------|
| `look` | Describe current room |
| `go <direction>` | Move to adjacent room |
| `north` / `south` / `east` / `west` / `up` / `down` | Move shorthand |
| `inventory` | List carried items |
| `equipment` | Show equipped items |
| `stats` | Character statistics |
| `get <item>` | Pick up item |
| `drop <item>` | Drop item |
| `attack <target>` | Initiate/continue combat |
| `say <text>` | Speak to room |
| `quit` | Disconnect gracefully |

---

## Notes

- The headless port (4002) is only available when `telnet.headless_port` is configured non-zero in the config.
- Output is plain text. No ANSI, no split-screen.
- The session goes through the same auth and character-select flow as a normal client.
- Use `quit` or close the connection to end cleanly.
