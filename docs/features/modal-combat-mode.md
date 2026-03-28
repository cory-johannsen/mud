# Modal Combat Mode

## Overview

When combat starts, the telnet UI switches to a dedicated combat screen buffer with its own layout: battlefield position display, detailed combatant roster with HP/AP/conditions, scrolling combat log, and combat-specific command routing. On combat end, a summary is shown briefly before auto-returning to room mode.

## Requirements

- [x] Dual-buffer screen architecture (room buffer + combat buffer) in `telnet.Conn`
- [x] Combat screen layout: header → battlefield → roster → divider → combat log → command hint → prompt
- [x] 1D linear battlefield rendering with distances between combatants
- [x] Detailed roster: turn marker, name, HP bar, AP dots, condition tags
- [x] `CombatModeHandler` with full combat state tracking (round, turn order, HP, conditions, AP)
- [x] Mode transition on first `RoundStartEvent`, exit on `CombatEvent END`
- [x] Combat-first command routing: combat commands primary, escape commands (look/say/inventory/who) allowed, movement blocked
- [x] Combat summary display (XP, loot, damage) for 3 seconds before auto-return to room mode
- [x] Resize-safe rendering with absolute cursor positioning (no DECSTBM)
- [x] Property-based tests for battlefield and roster rendering
- [x] Unit tests for command routing and mode transitions

## Dependencies

- `advanced-combat` (combat distance/position mechanics)
