---
issue: 107
title: AI Item Content — AI Chainsaw and AI AK-47
slug: ai-item-content
date: 2026-04-20
---

## Summary

Define the content for the first two AI items: the AI Chainsaw and the AI
AK-47. Each item is a new `ItemDef` that references an existing `weapon_ref`,
gains a `CombatDomain`, and is backed by a new HTN domain file and embedded Lua
`CombatScript`. This spec covers data files only — no engine changes. It depends
on the AI Item Engine spec (`2026-04-20-ai-item-engine.md`) being implemented
first.

---

## Architecture Overview

- Two new item YAML files in `content/items/`: `ai_chainsaw.yaml`,
  `ai_ak47.yaml`.
- Two new HTN domain YAML files in `content/ai/`: `ai_chainsaw_combat.yaml`,
  `ai_ak47_combat.yaml`.
- Lua `CombatScript` is embedded inline in each item YAML under the
  `combat_script` key.
- No server Go changes are required beyond those delivered by the engine spec.

---

## Requirements

### REQ-AIC-1: AI Chainsaw item definition

A new file `content/items/ai_chainsaw.yaml` MUST be created with the following
fields:

```yaml
id: ai_chainsaw
name: AI Chainsaw
description: >
  A salvaged power tool with a neural interface spliced into the grip. It has
  opinions. Loud ones. It really wants to be used.
kind: weapon
weapon_ref: chainsaw
weight: 3.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_chainsaw_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.frenzy_active = function(self)
    return (self.state.kills or 0) >= 2
  end

  operators.overkill_strike = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "2d6+4", 2)
      if target.hp <= 0 then
        self.state.kills = (self.state.kills or 0) + 1
      end
      self.engine.say({"I CAN'T STOP!", "RAAAH!", "GIVE ME EVERYTHING!", "DO IT AGAIN!"})
    end
  end

  operators.attack_weakest = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "1d6+2")
      if target.hp <= 0 then
        self.state.kills = (self.state.kills or 0) + 1
      end
      self.engine.say({"YES!", "MORE!", "BLOOD!", "KEEP GOING!"})
    end
  end

  operators.say_hungry = function(self)
    self.engine.say({"Feed me...", "Who's next?", "I can smell them.", "Let me at them."})
  end
```

### REQ-AIC-2: AI Chainsaw HTN domain

A new file `content/ai/ai_chainsaw_combat.yaml` MUST be created:

```yaml
domain:
  id: ai_chainsaw_combat
  description: Bloodthirsty berserker. Targets weakest enemy, escalates on kills.

  tasks:
    - id: behave
      description: Root task — hunt or idle
    - id: hunt
      description: Engage and kill

  methods:
    - task: behave
      id: frenzy_mode
      precondition: frenzy_active
      subtasks: [overkill_strike]

    - task: behave
      id: hunt_mode
      precondition: has_enemy
      subtasks: [attack_weakest]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [say_hungry]

  operators:
    - id: overkill_strike
      action: lua_hook
      ap_cost: 2

    - id: attack_weakest
      action: lua_hook
      ap_cost: 1

    - id: say_hungry
      action: lua_hook
      ap_cost: 0
```

### REQ-AIC-3: AI Chainsaw behavior invariants

- REQ-AIC-3a: The AI Chainsaw MUST track kill count in `self.state.kills` across
  rounds within an encounter.
- REQ-AIC-3b: Once `self.state.kills >= 2`, the frenzy method MUST be selected
  over the standard hunt method for the remainder of the encounter.
- REQ-AIC-3c: The frenzy `overkill_strike` operator MUST cost 2 AP and deal
  `2d6+4` damage; the standard `attack_weakest` operator MUST cost 1 AP and deal
  `1d6+2` damage.
- REQ-AIC-3d: The idle fallback (`say_hungry`) MUST cost 0 AP and MUST always be
  reachable (empty precondition).

### REQ-AIC-4: AI AK-47 item definition

A new file `content/items/ai_ak47.yaml` MUST be created:

```yaml
id: ai_ak47
name: AI AK-47
description: >
  A battered Kalashnikov with a cracked polymer stock and a jury-rigged
  cognitive module zip-tied to the receiver. It's figured out the pattern.
  It's always figured out the pattern.
kind: weapon
weapon_ref: assault_rifle
weight: 3.5
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_ak47_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.enough_ap_for_full_sequence = function(self)
    return self.combat.player.ap >= 3
  end

  operators.burst_and_theorize = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "2d6", 2)
      self.engine.debuff(target.id, "fear", 2)
      self.engine.say({
        "They sent this one specifically for you.",
        "Classic distraction unit.",
        "I've seen this pattern before — it ends badly for them.",
        "Every faction has one of these. You know what that means.",
        "This connects to something much bigger. Trust me."
      })
    end
  end

  operators.quick_shot = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "1d6+1")
      self.engine.say({"Noted.", "As expected.", "They're all connected."})
    end
  end
```

### REQ-AIC-5: AI AK-47 HTN domain

A new file `content/ai/ai_ak47_combat.yaml` MUST be created:

```yaml
domain:
  id: ai_ak47_combat
  description: Paranoid conspiracy theorist. Burst attack + fear debuff + monologue.

  tasks:
    - id: behave
      description: Root task — theorize or suppress

  methods:
    - task: behave
      id: full_sequence
      precondition: enough_ap_for_full_sequence
      subtasks: [burst_and_theorize]

    - task: behave
      id: quick_mode
      precondition: has_enemy
      subtasks: [quick_shot]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [quick_shot]

  operators:
    - id: burst_and_theorize
      action: lua_hook
      ap_cost: 3

    - id: quick_shot
      action: lua_hook
      ap_cost: 1
```

### REQ-AIC-6: AI AK-47 behavior invariants

- REQ-AIC-6a: When the shared AP pool is ≥ 3, the AK-47 MUST use `burst_and_theorize`:
  attack nearest enemy for 2 AP (`2d6` damage), apply `fear` debuff for 1 AP, say
  a conspiracy line for 0 AP.
- REQ-AIC-6b: When the shared AP pool is < 3, the AK-47 MUST fall back to
  `quick_shot`: attack nearest enemy for 1 AP (`1d6+1` damage), say a clipped
  theory line.
- REQ-AIC-6c: The `fear` debuff MUST last 2 rounds.
- REQ-AIC-6d: The AK-47 MUST NOT track cross-round state beyond what the engine
  provides in `self.combat` (no `self.state` usage required for this item).

### REQ-AIC-7: Test coverage

- REQ-AIC-7a: `TestAIChainsaw_KillCountEscalates` — after 2 kills in one
  encounter the frenzy method is selected on the next round.
- REQ-AIC-7b: `TestAIChainsaw_OverkillCosts2AP` — frenzy operator decrements
  pool by 2.
- REQ-AIC-7c: `TestAIChainsaw_IdleFallback` — with no enemies, `say_hungry`
  fires and costs 0 AP.
- REQ-AIC-7d: `TestAIAK47_FullSequenceAt3AP` — with AP ≥ 3, `burst_and_theorize`
  fires: attack (2 AP) + debuff (1 AP) + say (0 AP).
- REQ-AIC-7e: `TestAIAK47_QuickShotBelow3AP` — with AP < 3, `quick_shot` fires.
- REQ-AIC-7f: `TestAIAK47_FearDebuffDuration` — fear condition lasts exactly 2
  rounds.

---

## Dependencies

- REQ-DEP-1: This spec MUST NOT be implemented until the AI Item Engine spec
  (`2026-04-20-ai-item-engine.md`) is fully implemented and merged.
- REQ-DEP-2: The `lua_hook` operator action type, `fear` condition ID, and
  `nearest_enemy()` combat snapshot helper MUST be defined by the engine spec
  before these content files can be loaded.

---

## Out of Scope

- Additional AI item variants (other weapons, armor, other factions) — to be
  enumerated and specced in a follow-on AI Item Content Expansion spec.
- Quest delivery for these items — covered in the Quest Delivery sub-spec.
- Balancing pass on damage formulas and AP costs — subject to playtesting after
  initial implementation.
