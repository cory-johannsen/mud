---
issue: 107
title: AI Item Engine — active-participant combat AI for equipped items
slug: ai-item-engine
date: 2026-04-20
---

## Summary

Extend the combat system so that equipped items with a defined `CombatDomain`
become active participants in combat. Each AI item acts at the start of the
player's turn, draws from the shared AP pool (after contributing +1 AP), and
executes behavior driven by an HTN decision tree with Lua operator
implementations. Items maintain per-encounter state across rounds.

This spec covers the **engine only** — the data structures, combat phase
insertion, HTN+Lua runtime, and engine primitives. Content (specific AI Chainsaw
and AI AK-47 scripts) and quest delivery are covered in separate sub-specs.

---

## Architecture Overview

- **Phase insertion**: An "AI item phase" runs at the start of each combat round,
  before player input. Each equipped AI item gets a turn in equipment-slot order.
- **AP model**: Each AI item adds 1 AP to the shared pool at round start. Items
  may spend more than 1 AP; the player receives whatever AP remains after all
  items act.
- **Decision engine**: HTN domain defines the decision tree (tasks → methods →
  operators selected by Lua precondition hooks). Lua operator implementations
  call engine primitives to take action.
- **State**: `ItemInstance.CombatScriptState` holds per-encounter Lua state,
  initialized at combat start and cleared at combat end.
- **Proto**: No changes required. AI item effects (damage, speech, conditions)
  are communicated to the client through existing event types.

---

## Requirements

### REQ-AIE-1: ItemDef schema

`ItemDef` MUST gain two new fields:

- `CombatDomain string` — name of the HTN domain that drives this item's combat
  behavior. An empty string means the item is not an AI item and MUST be skipped
  during the AI item phase.
- `CombatScript string` — Lua source that defines operator implementations and
  precondition hooks for the named HTN domain.

### REQ-AIE-2: ItemInstance schema

`ItemInstance` MUST gain one new field:

- `CombatScriptState map[string]interface{}` — per-encounter Lua state table.
  MUST be initialized as an empty map when a combat encounter begins.
  MUST be serialized from the Lua `self.state` table after each item turn.
  MUST be cleared (reset to empty map) when combat ends for any reason (win,
  loss, flee, or escape).

### REQ-AIE-3: AI item phase — AP contribution

At the start of each combat round, before player input is requested, the
combat resolver MUST count the number of equipped items where `CombatDomain !=
""` (call this `N`) and add `N` to the player's AP pool for that round.

### REQ-AIE-4: AI item phase — turn execution

After AP contribution (REQ-AIE-3), the combat resolver MUST iterate over the
player's equipped items in equipment-slot order (main hand → off hand → armor →
accessories) and, for each item where `CombatDomain != ""`:

- REQ-AIE-4a: Initialize a Lua environment with:
  - `self.state` — deserialized from `item.CombatScriptState`
  - `self.combat` — a read-only snapshot of current combat state (enemy list with
    HP and conditions, player HP and current AP pool, round number)
  - `self.engine` — the engine primitive table (REQ-AIE-6)
- REQ-AIE-4b: Run the HTN planner for the item's `CombatDomain` against the
  current combat state. The planner selects operators using Lua precondition
  hooks and executes them in sequence.
- REQ-AIE-4c: Each operator execution MUST invoke the corresponding Lua function
  from the item's `CombatScript`.
- REQ-AIE-4d: If the shared AP pool reaches 0 at any point during the item's
  turn, the item's turn MUST end immediately with no further operators executed.
- REQ-AIE-4e: After the item's turn, serialize `self.state` back to
  `item.CombatScriptState`.

### REQ-AIE-5: AP non-negativity invariant

The shared AP pool MUST NOT go below 0. If an operator's AP cost would bring the
pool below 0, the operator MUST NOT execute and the item's turn MUST end.

### REQ-AIE-6: Engine primitives

The `self.engine` table MUST expose the following primitives to Lua operator
implementations:

- REQ-AIE-6a: `self.engine.attack(targetId string, formula string)` — resolves
  damage against `targetId` using `formula` (e.g. `"1d6+2"`) through the
  existing damage pipeline. MUST cost 1 AP per call.
- REQ-AIE-6b: `self.engine.say(textPool []string)` — selects a random entry from
  `textPool` and broadcasts it to the room as speech from the item. MUST cost 0
  AP.
- REQ-AIE-6c: `self.engine.buff(targetId string, effectId string, rounds int)` —
  applies the named status condition to `targetId` for `rounds` rounds via the
  existing condition system. MUST cost 1 AP per call.
- REQ-AIE-6d: `self.engine.debuff(targetId string, effectId string, rounds int)`
  — same as `buff` but applies a negative condition. MUST cost 1 AP per call.
- REQ-AIE-6e: Primitives that cost more than 1 AP (multi-AP actions) MUST be
  supported by passing an optional `cost int` parameter to `attack`, `buff`, and
  `debuff`. If omitted, cost defaults to 1. The pool MUST be decremented by the
  specified cost before the action resolves, subject to REQ-AIE-5.

### REQ-AIE-7: HTN domain registry

`GameServiceServer` MUST maintain a registry mapping domain name (string) →
compiled HTN domain, separate from the NPC domain registry. AI item domains MUST
be registered at server startup when item definitions are loaded.

### REQ-AIE-8: HTN precondition hooks

HTN method preconditions for AI item domains MUST be Lua functions defined in
the item's `CombatScript` under a `preconditions` table:

```lua
preconditions.enemy_exists = function(self)
  return #self.combat.enemies > 0
end
```

The HTN planner MUST call these functions to evaluate method eligibility. Every
HTN domain for an AI item MUST define a fallback method with no precondition (or
a precondition that always returns true) to ensure the planner always finds a
valid plan.

### REQ-AIE-9: Multiple AI items

When multiple AI items are equipped simultaneously:

- REQ-AIE-9a: Each item MUST contribute +1 AP to the pool independently
  (REQ-AIE-3).
- REQ-AIE-9b: Each item MUST take its turn in equipment-slot order (REQ-AIE-4).
- REQ-AIE-9c: Items MUST NOT share `CombatScriptState`; each item's state is
  isolated.
- REQ-AIE-9d: Items MUST NOT coordinate directly. Each item evaluates its own
  HTN domain against the current combat state at the time its turn starts (which
  includes AP already spent by earlier items in the same round).

### REQ-AIE-10: Combat state snapshot

The `self.combat` snapshot provided to each item MUST include:

- `self.combat.enemies` — list of `{id, name, hp, maxHp, conditions[]}`
- `self.combat.player` — `{hp, maxHp, ap, conditions[]}`
- `self.combat.round` — current round number (integer)
- `self.combat.weakest_enemy()` — convenience function returning the enemy with
  lowest current HP ratio; returns `nil` if no enemies remain.

### REQ-AIE-11: Test coverage

- REQ-AIE-11a: `TestAIItemPhase_AddsAP` — equipping N AI items adds exactly N AP
  to the round pool.
- REQ-AIE-11b: `TestAIItemPhase_OperatorConsumesAP` — a single-AP operator
  decrements the pool by 1.
- REQ-AIE-11c: `TestAIItemPhase_MultipleAPAction` — an operator with `cost=2`
  decrements the pool by 2 and leaves the player with correspondingly fewer AP.
- REQ-AIE-11d: `TestAIItemPhase_PoolExhausted_TurnEnds` — when AP pool hits 0
  mid-turn, no further operators execute.
- REQ-AIE-11e: `TestAIItemPhase_StatePersistedAcrossRounds` — `self.state`
  mutations in round N are visible in round N+1.
- REQ-AIE-11f: `TestAIItemPhase_StateClearedOnCombatEnd` — `CombatScriptState`
  is empty after combat ends (win, loss, flee).
- REQ-AIE-11g: `TestAIItemPhase_MultipleItems_SlotOrder` — two AI items act in
  equipment-slot order; second item sees AP already spent by first.
- REQ-AIE-11h: Property test — for any combat state with any number of equipped
  AI items, the AP pool MUST NEVER go below 0 after the AI item phase.
- REQ-AIE-11i: HTN planner selects correct method given precondition state.
- REQ-AIE-11j: HTN planner falls back to idle method when no other precondition
  is satisfied.

---

## Out of Scope

- AI item content (specific Chainsaw/AK-47 HTN domains and Lua scripts) — covered
  in the AI Item Content sub-spec.
- Quest delivery for AI items — covered in the Quest Delivery sub-spec.
- Cross-item coordination — items do not communicate with each other.
- Persistent state across separate combat encounters — `CombatScriptState` is
  per-encounter only.
- Client-side AI item UI changes — effects reach the client through existing
  damage/speech/condition event types.
