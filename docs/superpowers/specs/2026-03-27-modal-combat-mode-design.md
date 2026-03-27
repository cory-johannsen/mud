# Modal Combat Mode — Design Spec

## Overview

When combat is initiated, the telnet client switches from the room screen buffer to a dedicated combat screen buffer with its own layout optimized for tactical display. The combat screen shows a linear battlefield, detailed combatant roster, and scrolling combat log. On combat end, a brief summary is displayed before auto-returning to room mode.

## Requirements

- REQ-CM-1: The frontend MUST implement a dual-buffer screen architecture — a room buffer (existing) and a combat buffer with independent layout geometry.
- REQ-CM-2: The combat buffer MUST swap in when combat starts (first `RoundStartEvent` received while not in combat mode) and swap out when combat ends (`CombatEvent` with type `COMBAT_EVENT_TYPE_END`).
- REQ-CM-3: The combat screen layout MUST use this top-to-bottom structure:
  - Header (1 row): `COMBAT: Round N` + location name
  - Battlefield (2–3 rows): 1D linear grid showing combatant positions with distances
  - Roster (N rows, one per combatant): turn marker `>>`, name, HP bar, AP indicator (on player's turn), active conditions
  - Divider (1 row)
  - Combat Log (fills remaining rows): scrolling combat event narration
  - Command Hint (1 row): available commands quick-reference
  - Prompt (1 row): combat input prompt
- REQ-CM-4: The `CombatModeHandler` MUST store combat state received from server events: current round, turn order, combatant HP/conditions, action points remaining.
- REQ-CM-5: The battlefield MUST render combatants on a 1D linear display with distances between them, e.g. `[You]━━8m━━[Goblin1]━━4m━━[Orc]`.
- REQ-CM-6: The roster MUST display one line per combatant with: turn marker (`>>` for active), name, HP bar (colored), AP dots (on player's turn), and condition tags.
- REQ-CM-7: Combat mode MUST allow these commands: `attack`, `strike`, `pass`, `flee`, `equip`, `loadout`, `reload`, `burst`, `auto`, `raise_shield`, `take_cover`, `grapple`, `trip`, `disarm`.
- REQ-CM-8: Combat mode MUST allow non-disruptive escape commands: `look`, `inventory`, `say`, `who`.
- REQ-CM-9: Combat mode MUST reject movement commands and other non-combat actions with the message: `You're in combat!`
- REQ-CM-10: On combat end, the combat screen MUST display a summary (XP gained, loot dropped, damage taken) for 3 seconds, then auto-transition back to room mode.
- REQ-CM-11: The combat screen MUST re-render correctly on terminal resize events.
- REQ-CM-12: All combat screen rendering MUST use absolute cursor positioning (no DECSTBM), consistent with the existing room screen approach.
- REQ-CM-13: The `CombatModeHandler.HandleInput` MUST route allowed commands to the gRPC stream and reject disallowed commands locally.

## Architecture

### Dual-Buffer Screen Swap

The `telnet.Conn` gains a screen-mode concept: `ScreenRoom` (default) and `ScreenCombat`. Each mode defines its own layout geometry:

- **Room layout** (existing): `roomLayout` — 10-row room region, divider, console, prompt.
- **Combat layout** (new): `combatLayout` — header, battlefield, roster, divider, combat log, command hint, prompt. Row counts for battlefield and roster are dynamic based on combatant count.

When `SetScreenMode(ScreenCombat)` is called, the connection:
1. Saves the current room buffer state (room text, console buffer, scroll offset).
2. Clears the screen.
3. Initializes the combat layout geometry.
4. Renders the initial combat screen.

When `SetScreenMode(ScreenRoom)` is called, the connection:
1. Clears the screen.
2. Restores the saved room buffer state.
3. Re-renders the room layout.

### Combat Layout Geometry

```
Row 1:              COMBAT: Round 2 — Ruins of Old Wooklyn
Row 2:              [You]━━━━8m━━━━[Goblin1]━━4m━━[Orc]
Row 3:                      [Goblin2]
Row 4:              ═══════════════════════════════════════
Row 5:              >> You      HP:████████░░  AP:●●●  [flat-footed]
Row 6:                 Goblin1  HP:███░░░░░░░  [bleeding]
Row 7:                 Orc      HP:█████████░
Row 8:                 Goblin2  HP:██████░░░░
Row 9:              ═══════════════════════════════════════
Rows 10..(H-2):     Scrolling combat log
Row H-1:            attack·strike·pass·flee·equip·reload
Row H:              [COMBAT] >>
```

The divider between roster and combat log is at `1 + battlefieldRows + rosterRows + 1`. Everything below scrolls. The command hint row is fixed at `H-1`.

### CombatModeHandler

Replaces the stub in `mode_stubs.go` with a full implementation in a new file `mode_combat.go`:

```go
type CombatModeHandler struct {
    round        int
    turnOrder    []string        // combatant names in initiative order
    activeTurn   string          // whose turn it is
    combatants   map[string]*CombatantState  // name → state
    actionsLeft  int
    roomName     string
    logLines     []string        // scrolling combat log buffer
    conn         *telnet.Conn    // for re-rendering
}

type CombatantState struct {
    Name       string
    HP         int
    MaxHP      int
    Conditions []string
    Position   int    // 1D position for battlefield rendering
}
```

**State updates**: `forwardServerEvents()` calls methods on the handler to update state:
- `UpdateRound(roundStart)` — new round, update turn order and AP
- `UpdateCombatEvent(event)` — append to log, update HP/conditions
- `UpdateCondition(event)` — add/remove condition from combatant
- `SetCombatEnd(summary)` — trigger summary display + auto-exit timer

Each update triggers a screen re-render of the affected regions only.

### Mode Transition Flow

1. `forwardServerEvents()` receives `RoundStartEvent` while `session.Mode() != ModeCombat`.
2. Bridge code creates/initializes `CombatModeHandler` with initial state.
3. `session.SetMode(conn, combatHandler)` is called.
4. `OnEnter()` calls `conn.SetScreenMode(ScreenCombat)` and renders initial combat screen.
5. During combat, server events update handler state and trigger partial re-renders.
6. On `CombatEvent` type `END`, handler displays summary, starts 3-second timer.
7. Timer fires: `session.SetMode(conn, session.Room())`.
8. `OnExit()` calls `conn.SetScreenMode(ScreenRoom)` to restore room buffer.

### Command Routing in Combat Mode

`CombatModeHandler.HandleInput` classifies input:

- **Combat commands** (attack, strike, pass, flee, etc.): Parse via `command.Parse()`, build `ClientMessage`, send to gRPC stream.
- **Escape commands** (look, say, inventory, who): Route to existing bridge handlers.
- **Blocked commands**: Print `You're in combat!` to the combat log area.
- **`q` / Escape**: No-op in combat (can't quit combat mode manually — only ends when combat ends or player flees).

### Allowed Command Sets

```go
var combatCommands = map[string]bool{
    "attack": true, "strike": true, "pass": true, "flee": true,
    "equip": true, "loadout": true, "reload": true,
    "burst": true, "auto": true,
    "raise_shield": true, "take_cover": true,
    "grapple": true, "trip": true, "disarm": true,
}

var escapeCommands = map[string]bool{
    "look": true, "inventory": true, "say": true, "who": true,
}
```

### Rendering

New functions in `text_renderer.go`:

- `RenderBattlefield(combatants map[string]*CombatantState, width int) []string` — 1D linear position display, wrapping at terminal width.
- `RenderRoster(combatants, turnOrder, activeTurn string, actionsLeft int, width int) []string` — one line per combatant with HP bar, AP, conditions.
- `RenderCombatHeader(round int, roomName string, width int) string` — centered header line.
- `RenderCommandHint(width int) string` — available commands, truncated to width.

New functions in `screen.go`:

- `SetScreenMode(mode ScreenMode)` — swap buffers, clear screen, initialize layout.
- `WriteCombatHeader(text string)` — absolute position row 1.
- `WriteBattlefield(lines []string)` — absolute position battlefield rows.
- `WriteRoster(lines []string)` — absolute position roster rows.
- `WriteCombatLog(text string)` — append to combat log buffer, scroll if needed, redraw fixed regions.
- `WriteCombatHint(text string)` — absolute position command hint row.

### Server Events → Screen Updates

| Server Event | Handler Method | Screen Update |
|---|---|---|
| `RoundStartEvent` (first) | `session.SetMode(combatHandler)` | Full combat screen init |
| `RoundStartEvent` (subsequent) | `UpdateRound()` | Header + roster + log append |
| `CombatEvent` (attack/death/flee) | `UpdateCombatEvent()` | Log append + roster HP update |
| `ConditionEvent` | `UpdateCondition()` | Roster condition update |
| `CombatEvent` (END) | `SetCombatEnd()` | Summary overlay → 3s → exit |
| Terminal resize | `OnResize()` | Full combat screen re-render |

### Combat Summary Screen

On combat end, the combat log area is replaced with a centered summary:

```
═══════════ COMBAT COMPLETE ═══════════
  Victory! 3 rounds
  XP: +150   Credits: +25
  Loot: Iron Sword, Health Potion
  Damage taken: 12
═══════════════════════════════════════
```

After 3 seconds, auto-transitions back to room mode.

## Dependencies

- Existing `ModeHandler` interface (no changes needed)
- Existing `telnet.Conn` screen management (extended with dual-buffer)
- Existing `command.Parse()` and bridge handler dispatch
- Existing `CombatEvent`, `RoundStartEvent`, `RoundEndEvent` proto messages

## Out of Scope

- 2D grid-based combat (future feature: `2d-combat-expansion`, depends on this)
- NPC AI display or decision visualization
- Multi-room combat
- Combat-specific sound/notification system

## Testing Strategy

- Property-based tests for `RenderBattlefield` — verify combatant positions render correctly for arbitrary combatant counts and terminal widths.
- Property-based tests for `RenderRoster` — verify HP bars scale correctly, conditions render, turn markers appear on correct combatant.
- Unit tests for `CombatModeHandler.HandleInput` — verify command routing: combat commands accepted, escape commands allowed, other commands rejected.
- Unit tests for screen mode swap — verify room buffer saved/restored correctly.
- Integration test for mode transition — simulate `RoundStartEvent` → verify `ModeCombat` active → simulate `CombatEvent END` → verify return to `ModeRoom`.
