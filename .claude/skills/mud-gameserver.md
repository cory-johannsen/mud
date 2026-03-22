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
| player | `claude_player` | Testing gameplay: movement, combat, inventory, commands |
| editor | `claude_editor` | Placing content: creating items, NPCs, room descriptions |
| admin | `claude_admin` | World management: state inspection, spawning, overrides |

Password: set by the `CLAUDE_ACCOUNT_PASSWORD` environment variable when `make seed-claude-accounts` was run. Ask the operator if unknown.

---

## Session Flow

### Login

```
Username: claude_player
Password: <password>
```

On success the server sends the character selection menu:

```
Select a character:
  1. <character name>
  > (or 'new' to create)
```

If no characters exist yet, type `new` and follow the character creation prompts (name, class/job, etc.).

### After Character Select

The server outputs the current room:

```
<Room Name>
<Description lines, word-wrapped>
Exits: <direction> [<direction> ...]

>
```

The `> ` prompt indicates the server is waiting for a command.

### Prompt Recognition

- `Username: ` — server awaiting login
- `Password: ` — server awaiting password (input is not echoed)
- `> ` — logged in and in-game, awaiting command
- Any other line — server output (room views, messages, combat events)

---

## Plain-Text Output Format

Room output (after entering a room or typing `look`):

```
<Room Name>
<Description line 1>
<Description line 2 if wrapped>
Exits: north south east

>
```

Console messages (combat, system, NPC speech) appear as plain lines:

```
You strike the guard for 12 damage.
The guard attacks you for 8 damage.
>
```

---

## Command Reference

### Player Commands

| Command | Description | Expected Response |
|---------|-------------|-------------------|
| `look` | Describe current room | Room name, description, exits |
| `go <direction>` | Move to adjacent room | New room view |
| `north` / `south` / `east` / `west` / `up` / `down` | Move shorthand | New room view |
| `inventory` | List carried items | Item list |
| `equipment` | Show equipped items | Equipment slots |
| `stats` | Character statistics | Stat block |
| `get <item>` | Pick up item from room | Confirmation message |
| `drop <item>` | Drop item in room | Confirmation message |
| `attack <target>` | Initiate/continue combat | Combat event messages |
| `say <text>` | Speak to room | Echo of speech |
| `quit` | Disconnect gracefully | `Goodbye.` |

### Editor Commands

All player commands plus:

| Command | Description |
|---------|-------------|
| `edit room` | Open room editor (if implemented) |
| `spawn <npc-id>` | Spawn NPC in current room |
| `place <item-id>` | Place item in current room |

### Admin Commands

All editor commands plus:

| Command | Description |
|---------|-------------|
| `setrole <username> <role>` | Change account role |
| `shutdown` | Initiate server shutdown |
| `reload zones` | Reload zone content from disk |

---

## Example Workflows

### Workflow 1: Verify NPC Spawn

Goal: Confirm that a named NPC appears in a room after a spawn command.

```
# Connect as editor
nc localhost 4002
Username: claude_editor
Password: <password>
# Select or create character
> spawn guard_patrol_01
Guard Patrol 01 appears in the room.
> look
The Alley
A narrow passage between two buildings. A guard patrols here.
Exits: north south
Guard Patrol 01 is here.

>
```

Verify: `look` output contains the NPC name.

### Workflow 2: Place Item in Room

Goal: Place a medkit in a specific room and confirm a player can pick it up.

```
# Session 1: Editor places item
nc localhost 4002
Username: claude_editor
Password: <password>
> go north       # navigate to target room
> place medkit_standard
Medkit Standard is placed in the room.
> look
...
Medkit Standard is here.

# Session 2: Player picks it up
nc localhost 4002
Username: claude_player
Password: <password>
> go north
> get medkit
You pick up the Medkit Standard.
> inventory
  Medkit Standard
>
```

### Workflow 3: Test Combat Round

Goal: Confirm that a combat round completes and health is reduced.

```
nc localhost 4002
Username: claude_player
Password: <password>
> stats
HP: 30/30  AC: 15
> attack training_dummy
Combat begins.
You attack the Training Dummy.
You strike the Training Dummy for 7 damage. (23 HP remaining)
The Training Dummy attacks you.
The Training Dummy misses.
> stats
HP: 30/30  AC: 15
>
```

Verify: combat events appear as plain lines, `stats` reflects accurate HP.

---

## Notes

- The headless port (4002) is only available when `telnet.headless_port` is set to a non-zero value in the config (default in `configs/dev.yaml`: 4002).
- Output is plain text. No ANSI color codes, no cursor positioning, no split-screen.
- The session goes through the same auth and character-select flow as a normal telnet client.
- Use `quit` or close the connection to end the session cleanly.
- Seed the three Claude accounts once with: `make seed-claude-accounts CLAUDE_ACCOUNT_PASSWORD=<password> CONFIG=configs/dev.yaml`
