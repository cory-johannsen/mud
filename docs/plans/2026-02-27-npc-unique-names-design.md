# NPC Unique Name Generation Design

Date: 2026-02-27

## Problem

When multiple NPCs of the same type occupy the same room, they are indistinguishable by name. `FindInRoom` uses prefix matching on `Instance.Name`, so both instances match any target string and the first one found wins arbitrarily.

## Solution

Assign a letter suffix (`A`, `B`, `C`, …) to each NPC instance's `Name` at spawn time when more than one instance of the same template occupies a room. Rooms with only one instance of a template receive no suffix.

Examples:
- One ganger in a room → `Ganger`
- Two gangers in a room → `Ganger A`, `Ganger B`
- Three gangers in a room → `Ganger A`, `Ganger B`, `Ganger C`

## Architecture

### Where

`Manager.Spawn()` in `internal/game/npc/manager.go` is the single point of instance creation. Suffix assignment lives here.

### How

At spawn time, `Spawn()` counts existing live instances of the same `TemplateID` in the target room. Based on that count, it determines which letter to assign to the new instance. It also retroactively renames the first instance if this is the second spawn (count goes from 0→1, triggering `A`/`B` assignment).

### Letter Assignment Rules

- Count of existing same-template instances in room before this spawn = `n`
- If `n == 0`: spawn with base name, no suffix (single NPC case)
- If `n == 1`: rename existing instance to `"<Name> A"`, spawn new as `"<Name> B"`
- If `n >= 2`: existing instances already have suffixes; spawn new as `"<Name> <Letter>"` where letter = `A + n`

### Constraints

- Maximum 26 instances per template per room (letters A–Z). Room spawn configs with `count > 26` are invalid but treated gracefully (suffix wraps or logs a warning).
- Suffix is part of `Instance.Name` — `FindInRoom` prefix matching works without changes (players type `ganger a` or `ganger b`).
- On respawn, the same logic applies: the respawned NPC gets the next available letter.

## Files Changed

- `internal/game/npc/manager.go` — `Spawn()` method updated with suffix logic
- `internal/game/npc/manager_test.go` — new tests for suffix assignment

## Testing

- Single spawn → no suffix
- Two spawns of same template → first renamed to `A`, second gets `B`
- Three spawns → `A`, `B`, `C`
- Two different templates in same room → each gets no suffix (only one of each)
- Death and respawn → suffix assigned correctly based on survivors
