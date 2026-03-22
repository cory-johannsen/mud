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

## Example Workflows

### 1. Verify NPC Spawn

Connect as `claude_admin`, navigate to the target zone, and use the `spawn` command:

```
Username: claude_admin
Password: <password>
> go ne_portland
> spawn feral_dog
A feral dog appears.
> look
NE Portland — Cully Road
A cracked asphalt road...
Exits: north south
A feral dog is here.
>
```

Confirm the NPC appears in the room description. Use `attack feral_dog` to verify it engages in combat.

### 2. Place Item in Room

Connect as `claude_editor`, navigate to the target room, and use the `place` command:

```
Username: claude_editor
Password: <password>
> go rustbucket_ridge
> place tactical_boots here
Tactical Boots placed in the room.
> look
Rustbucket Ridge — Blade House
...
A pair of tactical boots lies here.
>
```

Confirm the item appears. Use `get tactical_boots` as `claude_player` to verify it can be picked up.

### 3. Test Combat Round

Connect as `claude_player`, find a target NPC, and run a full combat exchange:

```
Username: claude_player
Password: <password>
> go rustbucket_ridge
> attack gang_member
You attack the gang member.
You strike the gang member for 9 damage.
The gang member attacks you.
The gang member strikes you for 5 damage.
> attack
You strike the gang member for 11 damage.
The gang member is defeated.
You gain 45 XP.
>
```

Verify: damage values are within dice bounds, XP is awarded on kill, and the NPC is removed from the room after defeat.

---

## Notes

- The headless port (4002) is only available when `telnet.headless_port` is configured non-zero in the config.
- Output is plain text. No ANSI, no split-screen.
- The session goes through the same auth and character-select flow as a normal client.
- Use `quit` or close the connection to end cleanly.
