# Input Mode Refactor

**Slug:** `input-mode-refactor`
**Status:** spec
**Priority:** 230
**Category:** ui
**Effort:** M

## Summary

Refactor the telnet frontend's input routing from a scattered `mapMode bool` flag to a `ModeHandler` interface with a `SessionInputState` controller. Fixes the bug where `forwardServerEvents` overwrites the map prompt with the room prompt on every server event. Establishes a clean extensible pattern for all future interactive modes.

## Motivation

The current `mapModeState.mapMode bool` is checked only in `commandLoop`; `forwardServerEvents` runs concurrently and calls `buildPrompt()` unconditionally, clobbering the map prompt every few seconds. A proper `InputMode` type with a `ModeHandler` interface centralizes all prompt rendering and input dispatch.

## Modes

| Mode | Constant | Description |
|---|---|---|
| Room (default) | `ModeRoom` | Standard command entry, movement |
| Map | `ModeMap` | World/zone map navigation |
| Inventory | `ModeInventory` | Inventory and loot screen (stub) |
| Character Sheet | `ModeCharSheet` | Character sheet viewer (stub) |
| Editor | `ModeEditor` | World editor commands (stub) |
| Combat | `ModeCombat` | Combat display with positional grid (stub) |

## Requirements

See `docs/superpowers/specs/2026-03-23-input-mode-refactor-design.md` for the full requirement set (REQ-IMR-1 through REQ-IMR-32).

## Non-Goals

- Full UI for inventory, character sheet, editor, combat modes (stubs only in this feature)
- Protocol changes
- Screen layout changes
