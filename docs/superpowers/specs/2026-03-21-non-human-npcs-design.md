# Non-Human NPCs Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `non-human-npcs` (priority 350)
**Dependencies:** `npc-behaviors` (for HTN operators `flee`, `call_for_help`; map fencing via `HomeRoom`/`WanderRadius`)

---

## Overview

Extends the NPC system to support three non-human archetypes — **animals**, **robots**, and **machines** — each with distinct combat verbs, loot rules, damage type interactions, and behavioral constraints. The existing `Type string` field on `npc.Template` (currently used for `predators_eye` passive matching) is the anchor for this system. The existing `Resistances` and `Weaknesses` maps are already wired into `applyResistanceWeakness` in `combat/round.go` and require no changes.

---

## 1. NPC Type Values

The `type` YAML field on `npc.Template` is currently a free string. This feature defines the canonical valid values and their semantics. Existing templates using `"human"` or `"mutant"` are unaffected.

- REQ-NHN-1: The canonical `type` values for non-human NPCs MUST be `"animal"`, `"robot"`, and `"machine"`. The existing values `"human"` and `"mutant"` remain valid.
- REQ-NHN-2: `npc.Template.Validate()` MUST NOT reject unknown `type` values (the field remains a free string for forward compatibility).
- REQ-NHN-3: A new `npc.Template.IsAnimal() bool`, `IsRobot() bool`, and `IsMachine() bool` helper MUST return `true` iff `t.Type` equals the respective canonical value. These helpers MUST be used internally wherever type-branching logic is needed.

---

## 2. Animals

Animals are organic creatures: hostile wildlife, mutant fauna, dogs, rats, coyotes. They use natural weapons (teeth, claws) and drop organic loot.

### 2.1 Attack Verbs

The `attackNarrative` function in `combat/round.go` currently uses hardcoded `"attacks"` and `"strikes"`. A new `AttackVerb string` field on `npc.Template` allows per-template override. The combat system reads the verb from the attacker's `Combatant` struct.

- REQ-NHN-4: `npc.Template` MUST gain an `AttackVerb string` field with YAML tag `yaml:"attack_verb"`. An empty value MUST default to `"attacks"`.
- REQ-NHN-5: `npc.Instance` MUST propagate `AttackVerb` from its template at spawn (in `SpawnInstance`).
- REQ-NHN-6: The `combat.Combatant` struct MUST gain an `AttackVerb string` field. When the attacker is an NPC, `AttackVerb` MUST be set from `Instance.AttackVerb`. When the attacker is a player, `AttackVerb` MUST default to `"attacks"`.
- REQ-NHN-7: `attackNarrative` in `combat/round.go` MUST use `actor.AttackVerb` (instead of the hardcoded string literals `"attacks"` and `"strikes"`) for all three attack slots (main, offhand-1, offhand-2).
- REQ-NHN-8: The standard `attack_verb` for animal templates MUST be one of `"bites"`, `"claws"`, `"mauls"`, or `"slams"` as appropriate per template. (Enforced by content convention, not by code validation.)

### 2.2 Animal Loot

Animals do not carry credits, gear, or manufactured items. They may drop organic resources.

- REQ-NHN-9: `npc.LootTable` MUST gain an `OrganicDrops []OrganicDrop` field with YAML tag `yaml:"organic_drops"`. Each `OrganicDrop` has fields `item_id string`, `weight int`, and `quantity_min int` / `quantity_max int`. `LootTable.Validate()` MUST return an error if any `OrganicDrop` has `weight <= 0`, `quantity_min < 1`, or `quantity_max < quantity_min`.
- REQ-NHN-10: When an animal NPC dies, if `template.IsAnimal()` is true and `template.Loot.OrganicDrops` is non-empty, the loot resolver MUST roll one drop from `OrganicDrops` (weighted random, same algorithm as existing loot tables) and add the result to the room's item floor. The quantity rolled MUST be in `[quantity_min, quantity_max]` (inclusive uniform random).
- REQ-NHN-11: Animal templates MUST NOT use manufactured loot. `Template.Validate()` MUST return an error if `IsAnimal()` is true and `Loot` contains non-zero `credits_min`, `credits_max`, any `equipment` entries, or a non-nil `salvage_drop`.

### 2.3 Animal Behavior

- REQ-NHN-12: Animal templates MUST NOT have faction affiliations. `Template.Validate()` MUST be updated to enforce this when `IsAnimal()` is true (once `Faction` is added to Template by the `factions` feature; for now this is a convention enforced by content review).
- REQ-NHN-13: HTN social operators (`say`) MUST NOT be applied to animal instances. The HTN planner MUST remove any task using the `say` operator from the plan before execution when `instance.IsAnimal()` is true. If removing `say` tasks leaves the plan empty, the planner MUST fall back to the simple attack behavior (the same fallback used when no HTN domain is defined).

---

## 3. Robots

Robots are manufactured entities: corporate security bots, scavenger drones, combat mechs. They use mechanical attacks and drop salvage.

### 3.1 Robot Damage Interactions

- REQ-NHN-14: Robot templates MUST default `Resistances["bleed"] = 999` and `Resistances["poison"] = 999` at spawn, unless the template YAML explicitly sets lower values. A resistance of 999 effectively means immunity (bleed/poison damage is reduced to 0 after applying flat reduction, clamped at 0 by `applyResistanceWeakness`).
- REQ-NHN-15: The `SpawnInstance` function MUST apply robot default resistances after copying `tmpl.Resistances` — template-defined values MUST override the defaults, not the other way around.
- REQ-NHN-16: The canonical damage type for EMP/electric attacks is `"electric"`. Robot templates SHOULD set `Weaknesses["electric"] = 5` (or higher) to represent their vulnerability. This is a content convention; no code validation is required.

### 3.2 Robot Loot

- REQ-NHN-17: `npc.LootTable` MUST gain a `SalvageDrop *SalvageDrop` field with YAML tag `yaml:"salvage_drop"`. `SalvageDrop` has fields `item_ids []string` (uniform random selection among the list) and `quantity_min int` / `quantity_max int`. `LootTable.Validate()` MUST return an error if `SalvageDrop` is non-nil and `item_ids` is empty, `quantity_min < 1`, or `quantity_max < quantity_min`.
- REQ-NHN-18: When a robot NPC dies, if `template.IsRobot()` is true and `template.Loot.SalvageDrop` is non-nil, the loot resolver MUST select one item uniformly at random from `SalvageDrop.item_ids` and place the rolled quantity (uniform random in `[quantity_min, quantity_max]`) on the room's item floor.

### 3.3 Robot Attack Verbs

- REQ-NHN-19: The standard `attack_verb` for robot templates MUST be `"shoots"`, `"slams"`, or `"zaps"` as appropriate per template. (Content convention.)

---

## 4. Machines

Machines are stationary automated systems: turrets, auto-cannons, mine dispensers. They cannot move and react purely to room-entry triggers.

### 4.1 Immobility

- REQ-NHN-20: `npc.Template` MUST gain an `Immobile bool` field with YAML tag `yaml:"immobile"`. Default is `false`.
- REQ-NHN-21: `npc.Instance` MUST propagate `Immobile` from its template at spawn.
- REQ-NHN-22: The NPC movement system (wander logic, patrol operators, `flee` operator) MUST skip any instance where `instance.Immobile == true`. Immobile NPCs MUST remain in their spawn room at all times.
- REQ-NHN-23: Machine templates MUST set `immobile: true` in their YAML. (Content convention; no code validation required beyond the field being propagated.)

### 4.2 Machine Behavior

- REQ-NHN-24: Machine templates MUST use `on_enemy_enters_room` as their primary HTN trigger. Movement-based HTN operators (`flee`, patrol) are already prevented by immobility enforcement (REQ-NHN-22); this is a content convention that makes the intent explicit in the domain YAML.
- REQ-NHN-25: When a machine NPC is destroyed (HP reaches 0), the death message MUST be `"<Name> is destroyed."` instead of the standard `"<Name> is dead."`. This is determined by `instance.IsMachine()`.
- REQ-NHN-26: Machine instances MUST have the same bleed/poison resistance defaults as robots (REQ-NHN-14), applied in `SpawnInstance` when `template.IsMachine()` is true. Template-defined `bleed`/`poison` resistance values on machine templates MUST override the defaults (same rule as REQ-NHN-15 for robots).

### 4.3 Machine Loot

- REQ-NHN-27: Machine loot MUST use the existing equipment loot table (for components and items). Machine templates SHOULD NOT include credits loot. (Content convention.)

---

## 5. Damage Type Canonicalization

The game has ad-hoc damage type strings. This feature canonicalizes the set used for non-human NPC interactions.

- REQ-NHN-28: The canonical damage type strings MUST be defined in a new file `internal/game/combat/damage_types.go` (same package as `round.go`) as package-level string constants: `DamageTypeBleed = "bleed"`, `DamageTypePoison = "poison"`, `DamageTypeElectric = "electric"`, `DamageTypePhysical = "physical"`, `DamageTypeFire = "fire"`. Additional types MAY be added by future features.
- REQ-NHN-29: The `damage_types.go` constants MUST be used in all internal code that references these strings (including `SpawnInstance` default resistance application). Existing template YAML files that already use these string literals are unaffected (YAML decodes to the same values).

---

## 6. Example Templates

### 6.1 Animal: Feral Dog

```yaml
id: feral_dog
name: Feral Dog
type: animal
level: 2
max_hp: 18
ac: 12
awareness: 3
attack_verb: bites
abilities:
  brutality: 3
  grit: 2
  quickness: 4
  reasoning: 1
  savvy: 1
  flair: 1
loot:
  organic_drops:
    - item_id: dog_meat
      weight: 10
      quantity_min: 1
      quantity_max: 2
respawn_delay: 10m
disposition: hostile
```

### 6.2 Robot: Security Drone

```yaml
id: security_drone
name: Security Drone
type: robot
level: 4
max_hp: 35
ac: 15
awareness: 5
attack_verb: shoots
abilities:
  brutality: 4
  grit: 3
  quickness: 3
  reasoning: 2
  savvy: 1
  flair: 1
weaknesses:
  electric: 5
loot:
  salvage_drop:
    item_ids:
      - circuit_board
      - power_cell
      - scrap_metal
    quantity_min: 1
    quantity_max: 2
respawn_delay: 30m
disposition: hostile
```

### 6.3 Machine: Auto-Turret

```yaml
id: auto_turret
name: Auto-Turret
type: machine
level: 5
max_hp: 50
ac: 16
awareness: 6
attack_verb: shoots
immobile: true
abilities:
  brutality: 5
  grit: 4
  quickness: 2
  reasoning: 1
  savvy: 1
  flair: 1
weaknesses:
  electric: 8
loot:
  equipment:
    - id: turret_barrel
      weight: 10
respawn_delay: 60m
disposition: hostile
```

---

## 7. Requirements Summary

- REQ-NHN-1: Canonical `type` values MUST be `"animal"`, `"robot"`, `"machine"` (plus existing `"human"`, `"mutant"`).
- REQ-NHN-2: `Template.Validate()` MUST NOT reject unknown `type` values.
- REQ-NHN-3: `Template` MUST gain `IsAnimal()`, `IsRobot()`, `IsMachine()` bool helpers.
- REQ-NHN-4: `Template` MUST gain `AttackVerb string` (YAML: `attack_verb`; default `"attacks"`).
- REQ-NHN-5: `SpawnInstance` MUST propagate `AttackVerb` from template.
- REQ-NHN-6: `combat.Combatant` MUST gain `AttackVerb string`; NPC attackers use `Instance.AttackVerb`; player attackers default to `"attacks"`.
- REQ-NHN-7: `attackNarrative` MUST use `actor.AttackVerb` for all three attack slots.
- REQ-NHN-8: Animal `attack_verb` MUST be one of `"bites"`, `"claws"`, `"mauls"`, or `"slams"`. (Content convention.)
- REQ-NHN-9: `LootTable` MUST gain `OrganicDrops []OrganicDrop` (YAML: `organic_drops`) with fields `item_id`, `weight`, `quantity_min`, `quantity_max`. `LootTable.Validate()` MUST reject `weight <= 0`, `quantity_min < 1`, or `quantity_max < quantity_min`.
- REQ-NHN-10: Animal death MUST roll one `OrganicDrop` (weighted random) and place the quantity (uniform in `[quantity_min, quantity_max]`) on the room floor.
- REQ-NHN-11: `Template.Validate()` MUST return error if `IsAnimal()` is true and `Loot` contains credits, equipment, or `salvage_drop` entries.
- REQ-NHN-12: Animal faction enforcement deferred to `factions` feature; content convention in the interim.
- REQ-NHN-13: HTN planner MUST remove `say` operator tasks from plan before execution for animal instances. If the plan is then empty, fall back to simple attack behavior.
- REQ-NHN-14: Robot `SpawnInstance` MUST default `Resistances["bleed"] = 999` and `Resistances["poison"] = 999`.
- REQ-NHN-15: Template-defined resistance values MUST override robot defaults in `SpawnInstance`.
- REQ-NHN-16: `"electric"` is the canonical damage type for EMP/electric. Robot templates SHOULD set `Weaknesses["electric"]`.
- REQ-NHN-17: `LootTable` MUST gain `SalvageDrop *SalvageDrop` (YAML: `salvage_drop`) with fields `item_ids []string` and `quantity_min int`, `quantity_max int`. `LootTable.Validate()` MUST reject non-nil `SalvageDrop` with empty `item_ids`, `quantity_min < 1`, or `quantity_max < quantity_min`.
- REQ-NHN-18: Robot death MUST select one item uniformly from `SalvageDrop.item_ids` and place quantity (uniform in `[quantity_min, quantity_max]`) on room floor, if `SalvageDrop` is non-nil.
- REQ-NHN-19: Robot `attack_verb` MUST be `"shoots"`, `"slams"`, or `"zaps"`. (Content convention.)
- REQ-NHN-20: `Template` MUST gain `Immobile bool` (YAML: `immobile`; default `false`).
- REQ-NHN-21: `SpawnInstance` MUST propagate `Immobile` from template.
- REQ-NHN-22: NPC movement system MUST skip instances where `instance.Immobile == true`.
- REQ-NHN-23: Machine templates MUST set `immobile: true`. (Content convention.)
- REQ-NHN-24: Machine templates MUST use `on_enemy_enters_room` trigger. Movement operators are already blocked by REQ-NHN-22 immobility enforcement. (Content convention for explicit clarity.)
- REQ-NHN-25: Machine death message MUST be `"<Name> is destroyed."` (determined by `instance.IsMachine()`).
- REQ-NHN-26: Machine `SpawnInstance` MUST apply same bleed/poison resistance defaults as robots. Template-defined values MUST override machine defaults (same rule as REQ-NHN-15).
- REQ-NHN-27: Machine loot uses existing equipment loot table; no credits. (Content convention.)
- REQ-NHN-28: A new file `internal/game/combat/damage_types.go` (same package as `round.go`) MUST define package-level string constants: `DamageTypeBleed = "bleed"`, `DamageTypePoison = "poison"`, `DamageTypeElectric = "electric"`, `DamageTypePhysical = "physical"`, `DamageTypeFire = "fire"`.
- REQ-NHN-29: Internal code MUST use `damage_types.go` constants wherever these strings are referenced.
