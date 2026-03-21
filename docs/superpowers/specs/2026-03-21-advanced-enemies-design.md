# Advanced Enemies Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `advanced-enemies` (priority 370)
**Dependencies:** `non-human-npcs` (for `IsAnimal`/`IsRobot`/`IsMachine` helpers, damage type constants)

---

## Overview

Extends the NPC and world systems with five difficulty tiers, NPC tags for content-driven targeting, NPC feats for passive combat bonuses, tier-scaled XP and loot, and boss rooms with hazards, minion spawns, and special abilities. All extensions are additive — existing NPCs and rooms without the new fields use safe defaults.

---

## 1. Difficulty Tiers

Each NPC template gains a `tier` field. The five canonical tiers affect XP awards, loot quantity scaling, HP pool multipliers, and map/combat messaging.

### 1.1 Tier Definitions

| Tier name   | `tier` value | XP mult | Loot mult | HP mult |
|-------------|-------------|---------|-----------|---------|
| Minion      | `"minion"`  | 0.5     | 0.5       | 0.75    |
| Standard    | `"standard"` | 1.0    | 1.0       | 1.0     |
| Elite       | `"elite"`   | 2.0     | 1.5       | 1.5     |
| Champion    | `"champion"` | 3.0    | 2.0       | 2.0     |
| Boss        | `"boss"`    | 5.0     | 3.0       | 3.0     |

The multipliers are stored in `xp_config.yaml` as a new `tier_multipliers` map and loaded into `XPConfig`. They are not hardcoded. The `TierMultiplier` struct is defined in `internal/game/xp/` alongside `XPConfig`, since `AwardKill` is the primary consumer.

### 1.2 Tier Fields

- REQ-AE-1: `npc.Template` MUST gain a `Tier string` field with YAML tag `yaml:"tier"`. An empty value MUST default to `"standard"` at runtime wherever tier logic is applied.
- REQ-AE-2: `Template.Validate()` MUST return an error if `Tier` is non-empty and not one of `"minion"`, `"standard"`, `"elite"`, `"champion"`, `"boss"`.
- REQ-AE-3: `XPConfig` MUST gain a `TierMultipliers map[string]TierMultiplier` field loaded from `xp_config.yaml`. `TierMultiplier` is defined in `internal/game/xp/` with exported fields `XP float64`, `Loot float64`, `HP float64`.
- REQ-AE-4: At server startup, `XPConfig` MUST be validated to contain entries for all five tier names. Missing entries MUST cause a fatal startup error.

### 1.3 HP Scaling

- REQ-AE-5: `SpawnInstance` MUST apply the tier HP multiplier: `instance.MaxHP = int(math.Ceil(float64(tmpl.MaxHP) * tierMultiplier.HP))`. `instance.CurrentHP` MUST be set to `instance.MaxHP` at spawn. `SpawnInstance` MUST accept the `*xp.XPConfig` as a parameter to look up the tier multiplier.

### 1.4 XP Scaling

- REQ-AE-6: `xp.Service.AwardKill` MUST accept a `tier string` parameter (the NPC's tier value) and multiply the base XP award by `tierMultiplier.XP` for that tier before computing the award. The rounded integer result (round up via `math.Ceil`) is passed to the internal `award` function. When `tier` is empty, `"standard"` MUST be assumed.

---

## 2. NPC Tags

Tags are free-form string labels attached to NPC templates for targeting by player feats, loot table filters, and HTN domain selection.

- REQ-AE-7: `npc.Template` MUST gain a `Tags []string` field with YAML tag `yaml:"tags"`. An empty slice means no tags.
- REQ-AE-8: `SpawnInstance` MUST propagate `Tags` from the template to the instance.
- REQ-AE-9: `Template.Validate()` MUST NOT enforce a restricted tag vocabulary. Tags are content-driven free strings.
- REQ-AE-10: `npc.Instance` MUST gain a method `HasTag(tag string) bool` that returns true iff the tag is present in `instance.Tags`.
- REQ-AE-11: Player feats that grant bonuses against tagged targets MUST use `HasTag` to check the target at resolution time. The feat definition MUST support an optional `TargetTags []string` field (YAML: `target_tags`); the feat bonus applies only when the target has at least one matching tag.

---

## 3. NPC Feats

NPC templates may carry passive feats that grant combat bonuses. Only feats explicitly marked `allow_npc: true` in their definition are valid for NPCs.

### 3.1 Feat Definition Extension

- REQ-AE-12: `ruleset.Feat` MUST gain an `AllowNPC bool` field with YAML tag `yaml:"allow_npc"`. Default is `false`. Feats with `AllowNPC == false` MUST NOT be assigned to NPC templates.

### 3.2 Template Feats Field

- REQ-AE-13: `npc.Template` MUST gain a `Feats []string` field with YAML tag `yaml:"feats"`. Each entry is a feat ID.
- REQ-AE-14: A new `ValidateWithRegistry(registry *ruleset.FeatRegistry) error` method MUST be added to `npc.Template`. It MUST verify each feat ID in `Feats` exists in `registry` and has `AllowNPC == true`. Any invalid feat ID MUST return a validation error. The existing `Validate()` method is unchanged; `ValidateWithRegistry` is called separately at startup when the feat registry is available.
- REQ-AE-15: `SpawnInstance` MUST propagate `Feats` from the template to the instance.

### 3.3 Passive Feat Bonuses at Combat Resolution

- REQ-AE-16: Before each NPC combat round, the combat engine MUST compute the NPC's effective stats by applying passive feat bonuses from `instance.Feats`. The bonus lookup MUST use the same `FeatRegistry` used for player feats.
- REQ-AE-17: The initial NPC-valid feat set MUST include:
  - `tough` (`allow_npc: true`): +5 max HP (applied at spawn, not at round resolution).
  - `brutal_strike` (`allow_npc: true`): +2 damage on all attacks.
  - `evasive` (`allow_npc: true`): +2 AC.
  - `pack_tactics` (`allow_npc: true`): +2 attack bonus when at least one ally NPC is in the same room targeting the same player. The combat engine MUST query the active NPC instance list for the room (available via the `RoomCombatState`) to evaluate this condition.
- REQ-AE-18: `tough` feat bonus MUST be applied at `SpawnInstance` time after the tier HP multiplier (REQ-AE-5). The order is: (1) apply tier multiplier to get scaled `MaxHP`; (2) add the `tough` flat +5 to the scaled `MaxHP`. `CurrentHP` MUST be set to final `MaxHP` after both steps. The `tough` bonus is not itself multiplied by the tier multiplier.

---

## 4. XP & Loot Scaling

### 4.1 Loot Tier Scaling

- REQ-AE-19: `npc.LootTable` MUST gain a `TierScale bool` field with YAML tag `yaml:"tier_scale"`. When `true`, the loot resolver MUST multiply `quantity_min` and `quantity_max` by the NPC's tier `Loot` multiplier (rounded up via `math.Ceil`, minimum 1) before rolling quantity.
- REQ-AE-20: Loot table `credits_min` and `credits_max` MUST also be scaled by the tier `Loot` multiplier when `TierScale == true`.

### 4.2 Boss Kill Bonus

- REQ-AE-21: `XPConfig.Awards` MUST gain a `BossKillBonusXP int` field loaded from `xp_config.yaml`.
- REQ-AE-22: When an NPC with `Tier == "boss"` dies, the combat engine MUST iterate all `PlayerSession` entries whose `RoomID` matches the boss's room (obtained from the session manager) and award each `BossKillBonusXP` additional XP via `xp.Service.AwardXPAmount`, on top of the normal per-killer award.

---

## 5. Boss Rooms

### 5.1 Room Boss Flag

- REQ-AE-23: `world.Room` MUST gain a `BossRoom bool` field with YAML tag `yaml:"boss_room,omitempty"`. Default is `false`.
- REQ-AE-24: The map renderer (`RenderMap` in `text_renderer.go`) MUST render boss room tiles with `<BB>` (angle brackets) instead of `[BB]` to distinguish them visually, consistent with the zone-map current-room style.

### 5.2 Coordinated Boss Respawn

- REQ-AE-25: When a boss NPC (tier `"boss"`) in a `boss_room` respawns, the `RespawnManager` MUST look up the room's `Spawns []RoomSpawnConfig`, cancel any pending individual respawn timers for those entries, and immediately respawn all NPCs defined in that list. This represents the boss "calling its forces" on return.
- REQ-AE-26: Non-boss NPCs in a boss room that die while the boss is still alive MUST follow their individual respawn timers normally; the coordinated respawn (REQ-AE-25) applies only when the boss itself respawns.

### 5.3 Room Hazards

- REQ-AE-27: `world.Room` MUST gain a `Hazards []HazardDef` field with YAML tag `yaml:"hazards,omitempty"`. `HazardDef` is defined in `internal/game/world/` with exported fields and YAML tags: `ID string` (YAML: `id`), `Trigger string` (YAML: `trigger`; valid values: `"on_enter"` | `"round_start"`), `DamageExpr string` (YAML: `damage_expr`), `DamageType string` (YAML: `damage_type`), `ConditionID string` (YAML: `condition_id`; optional), `Message string` (YAML: `message`). `HazardDef.Validate()` MUST return an error if `Trigger` is not `"on_enter"` or `"round_start"`, or if `DamageExpr` is empty.
- REQ-AE-28: On `Trigger == "on_enter"`, the hazard MUST fire when any player enters the room. On `Trigger == "round_start"`, the hazard MUST fire at the start of each combat round for all players in the room.
- REQ-AE-29: Hazard damage MUST be rolled via the existing dice roller (`s.dice.RollExpr(hazard.DamageExpr)`) and applied directly to `sess.CurrentHP`. If `ConditionID` is non-empty, the condition MUST be applied to the player.
- REQ-AE-30: Hazard `Message` MUST be sent to each affected player via `conn.WriteConsole`.

---

## 6. Boss Abilities

Boss NPCs (tier `"boss"`) support a list of special abilities triggered during combat.

### 6.1 Field Rename

- REQ-AE-39: `npc.Template.SpecialAbilities []string` (YAML: `special_abilities`) MUST be renamed to `SenseAbilities []string` with YAML tag `yaml:"sense_abilities"`. All references in the codebase MUST be updated. Existing NPC YAML files using `special_abilities` MUST be migrated to `sense_abilities` as part of this change; no backwards-compatible aliasing is provided. This frees `special_abilities` as a YAML key for future use.

### 6.2 BossAbility Definition

- REQ-AE-31: `npc.Template` MUST gain a `BossAbilities []BossAbility` field with YAML tag `yaml:"boss_abilities"`. `BossAbility` is a struct with exported fields and YAML tags: `ID string` (YAML: `id`), `Name string` (YAML: `name`), `Trigger string` (YAML: `trigger`; valid values: `"hp_pct_below"` | `"round_start"` | `"on_damage_taken"`), `TriggerValue int` (YAML: `trigger_value`; HP percentage threshold for `"hp_pct_below"`, e.g. 50 = below 50% HP; round number for `"round_start"` where 0 = every round; unused for `"on_damage_taken"`), `Cooldown string` (YAML: `cooldown`; Go duration string, e.g. `"30s"`; empty string MUST be treated as no cooldown), `Effect BossAbilityEffect` (YAML: `effect`).
- REQ-AE-32: `BossAbilityEffect` is a struct with exported fields and YAML tags: `AoeCondition string` (YAML: `aoe_condition`), `AoeDamageExpr string` (YAML: `aoe_damage_expr`), `HealPct int` (YAML: `heal_pct`). Exactly one field MUST be set: `AoeCondition` is set when non-empty; `AoeDamageExpr` is set when non-empty; `HealPct` is set when non-zero. A `BossAbilityEffect` with all fields at their zero values, or with more than one field non-zero/non-empty, is invalid.
- REQ-AE-33: `Template.Validate()` MUST return an error for any `BossAbility` with: empty `ID` or `Name`, `Trigger` not in the three canonical values, non-empty `Cooldown` that fails `time.ParseDuration`, or a `BossAbilityEffect` that does not satisfy the exactly-one-field rule of REQ-AE-32. A non-zero `TriggerValue` on a `BossAbility` with `Trigger == "on_damage_taken"` MUST produce a validation error.
- REQ-AE-34: `npc.Instance` MUST gain an `AbilityCooldowns map[string]time.Time` field (keyed by ability ID), initialized to an empty map at spawn.

### 6.3 Boss Ability Execution

- REQ-AE-35: At the start of each combat round (after NPC AI planning, before action execution), the combat engine MUST evaluate each `BossAbility` for active boss instances in the room:
  - `"hp_pct_below"`: fire if `int(100*currentHP/maxHP) < TriggerValue` and cooldown elapsed.
  - `"round_start"`: fire if `TriggerValue == 0` (every round) OR current round number == `TriggerValue`, and cooldown elapsed.
  - `"on_damage_taken"`: fire if the boss took damage this round and cooldown elapsed.
- REQ-AE-36: When a boss ability fires, the combat engine MUST:
  - Send the message `"<Boss Name> uses <Ability Name>!"` to all players in the room.
  - Execute the `BossAbilityEffect` (apply AoE condition, deal AoE damage, or heal boss).
  - Record `AbilityCooldowns[ability.ID] = time.Now().Add(parsedCooldown)`. When `Cooldown` is empty, no cooldown entry is recorded.
- REQ-AE-37: AoE damage from boss abilities MUST be rolled once and applied identically to all players in the room. It MUST NOT be mitigated by player AC (it is an environmental effect, not an attack roll).
- REQ-AE-38: Boss ability announcements and effects MUST be processed before normal NPC attack resolution in the same round.

---

## 7. Requirements Summary

- REQ-AE-1: `npc.Template` MUST gain `Tier string` (YAML: `tier`); empty defaults to `"standard"`.
- REQ-AE-2: `Template.Validate()` MUST reject non-empty `Tier` not in the five canonical values.
- REQ-AE-3: `XPConfig` MUST gain `TierMultipliers map[string]TierMultiplier` (defined in `internal/game/xp/`) with exported fields `XP`, `Loot`, `HP float64`.
- REQ-AE-4: Missing tier entries in `TierMultipliers` at startup MUST cause a fatal error.
- REQ-AE-5: `SpawnInstance` MUST accept `*xp.XPConfig` and apply tier HP multiplier (`ceil(MaxHP * tierMult.HP)`).
- REQ-AE-6: `xp.Service.AwardKill` MUST accept a `tier string` parameter and multiply the base award by `tierMult.XP` (round up); empty tier assumes `"standard"`.
- REQ-AE-7: `npc.Template` MUST gain `Tags []string` (YAML: `tags`).
- REQ-AE-8: `SpawnInstance` MUST propagate `Tags` from template to instance.
- REQ-AE-9: Tag vocabulary MUST NOT be code-enforced.
- REQ-AE-10: `npc.Instance` MUST gain `HasTag(tag string) bool`.
- REQ-AE-11: Player feats MUST support `TargetTags []string` (YAML: `target_tags`); bonus applies when target has at least one matching tag.
- REQ-AE-12: `ruleset.Feat` MUST gain `AllowNPC bool` (YAML: `allow_npc`; default `false`).
- REQ-AE-13: `npc.Template` MUST gain `Feats []string` (YAML: `feats`).
- REQ-AE-14: `npc.Template` MUST gain `ValidateWithRegistry(registry *ruleset.FeatRegistry) error`; MUST reject feat IDs not in the registry or lacking `AllowNPC`.
- REQ-AE-15: `SpawnInstance` MUST propagate `Feats` from template to instance.
- REQ-AE-16: Combat engine MUST apply NPC passive feat bonuses before each round using `FeatRegistry`.
- REQ-AE-17: Initial NPC-valid feats MUST include `tough`, `brutal_strike`, `evasive`, `pack_tactics`; `pack_tactics` evaluates ally presence via `RoomCombatState`.
- REQ-AE-18: `tough` flat +5 HP bonus MUST be applied at spawn time after the tier multiplier; `CurrentHP` set to final `MaxHP` after both steps.
- REQ-AE-19: `LootTable` MUST gain `TierScale bool` (YAML: `tier_scale`); when true, quantity ranges scaled by tier `Loot` multiplier (ceil, minimum 1).
- REQ-AE-20: `credits_min`/`credits_max` MUST also scale when `TierScale == true`.
- REQ-AE-21: `XPConfig.Awards` MUST gain `BossKillBonusXP int`.
- REQ-AE-22: On boss death, the combat engine MUST iterate players in the room via the session manager and award each `BossKillBonusXP` extra XP via `AwardXPAmount`.
- REQ-AE-23: `world.Room` MUST gain `BossRoom bool` (YAML: `boss_room,omitempty`).
- REQ-AE-24: Map renderer MUST render boss room tiles as `<BB>` (angle brackets).
- REQ-AE-25: On boss respawn in a boss room, the `RespawnManager` MUST cancel pending timers and immediately respawn all `Room.Spawns` entries.
- REQ-AE-26: Non-boss deaths while boss is alive follow individual respawn timers.
- REQ-AE-27: `world.Room` MUST gain `Hazards []HazardDef` (YAML: `hazards,omitempty`); `HazardDef` MUST use exported fields with YAML tags; `HazardDef.Validate()` MUST reject invalid `Trigger` or empty `DamageExpr`.
- REQ-AE-28: `"on_enter"` hazards fire on player room entry; `"round_start"` hazards fire each combat round for all players.
- REQ-AE-29: Hazard damage applied directly to `sess.CurrentHP`; condition applied if `ConditionID` is non-empty.
- REQ-AE-30: Hazard `Message` MUST be sent to each affected player via `conn.WriteConsole`.
- REQ-AE-31: `npc.Template` MUST gain `BossAbilities []BossAbility` (YAML: `boss_abilities`); all `BossAbility` and `BossAbilityEffect` fields MUST be exported with YAML tags.
- REQ-AE-32: `BossAbilityEffect` MUST have exactly one field set: `AoeCondition` (non-empty string), `AoeDamageExpr` (non-empty string), or `HealPct` (non-zero int).
- REQ-AE-33: `Template.Validate()` MUST reject `BossAbility` with empty `ID`/`Name`, invalid `Trigger`, unparseable non-empty `Cooldown`, non-zero `TriggerValue` on `"on_damage_taken"`, or `BossAbilityEffect` not satisfying REQ-AE-32.
- REQ-AE-34: `npc.Instance` MUST gain `AbilityCooldowns map[string]time.Time` initialized at spawn.
- REQ-AE-35: Combat engine MUST evaluate boss abilities each round using trigger type, trigger value, and cooldown.
- REQ-AE-36: On ability fire: announce, execute effect, record cooldown (empty `Cooldown` means no cooldown entry recorded).
- REQ-AE-37: AoE damage MUST be rolled once and applied identically to all players; not mitigated by AC.
- REQ-AE-38: Boss ability processing MUST occur before normal NPC attack resolution in the same round.
- REQ-AE-39: `npc.Template.SpecialAbilities []string` (YAML: `special_abilities`) MUST be renamed to `SenseAbilities []string` (YAML: `sense_abilities`); all code references and NPC YAML files MUST be updated; no backwards-compatible aliasing.
