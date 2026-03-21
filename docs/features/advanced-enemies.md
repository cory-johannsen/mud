# Advanced Enemies

Extends the NPC and world systems with five difficulty tiers, NPC tags for content-driven targeting, NPC feats for passive combat bonuses, tier-scaled XP and loot, and boss rooms with hazards, minion spawns, and special abilities.

Design spec: `docs/superpowers/specs/2026-03-21-advanced-enemies-design.md`

## Requirements

- [ ] REQ-AE-1: `npc.Template` MUST gain `Tier string` (YAML: `tier`); empty defaults to `"standard"`.
- [ ] REQ-AE-2: `Template.Validate()` MUST reject non-empty `Tier` not in the five canonical values.
- [ ] REQ-AE-3: `XPConfig` MUST gain `TierMultipliers map[string]TierMultiplier` (defined in `internal/game/xp/`) with exported fields `XP`, `Loot`, `HP float64`.
- [ ] REQ-AE-4: Missing tier entries in `TierMultipliers` at startup MUST cause a fatal error.
- [ ] REQ-AE-5: `SpawnInstance` MUST accept `*xp.XPConfig` and apply tier HP multiplier (`ceil(MaxHP * tierMult.HP)`).
- [ ] REQ-AE-6: `xp.Service.AwardKill` MUST accept a `tier string` parameter and multiply the base award by `tierMult.XP` (round up); empty tier assumes `"standard"`.
- [ ] REQ-AE-7: `npc.Template` MUST gain `Tags []string` (YAML: `tags`).
- [ ] REQ-AE-8: `SpawnInstance` MUST propagate `Tags` from template to instance.
- [ ] REQ-AE-9: Tag vocabulary MUST NOT be code-enforced.
- [ ] REQ-AE-10: `npc.Instance` MUST gain `HasTag(tag string) bool`.
- [ ] REQ-AE-11: Player feats MUST support `TargetTags []string` (YAML: `target_tags`); bonus applies when target has at least one matching tag.
- [ ] REQ-AE-12: `ruleset.Feat` MUST gain `AllowNPC bool` (YAML: `allow_npc`; default `false`).
- [ ] REQ-AE-13: `npc.Template` MUST gain `Feats []string` (YAML: `feats`).
- [ ] REQ-AE-14: `npc.Template` MUST gain `ValidateWithRegistry(registry *ruleset.FeatRegistry) error`; MUST reject feat IDs not in the registry or lacking `AllowNPC`.
- [ ] REQ-AE-15: `SpawnInstance` MUST propagate `Feats` from template to instance.
- [ ] REQ-AE-16: Combat engine MUST apply NPC passive feat bonuses before each round using `FeatRegistry`.
- [ ] REQ-AE-17: Initial NPC-valid feats MUST include `tough`, `brutal_strike`, `evasive`, `pack_tactics`; `pack_tactics` evaluates ally presence via `RoomCombatState`.
- [ ] REQ-AE-18: `tough` flat +5 HP bonus MUST be applied at spawn time after the tier multiplier; `CurrentHP` set to final `MaxHP` after both steps.
- [ ] REQ-AE-19: `LootTable` MUST gain `TierScale bool` (YAML: `tier_scale`); when true, quantity ranges scaled by tier `Loot` multiplier (ceil, minimum 1).
- [ ] REQ-AE-20: `credits_min`/`credits_max` MUST also scale when `TierScale == true`.
- [ ] REQ-AE-21: `XPConfig.Awards` MUST gain `BossKillBonusXP int`.
- [ ] REQ-AE-22: On boss death, the combat engine MUST iterate players in the room via the session manager and award each `BossKillBonusXP` extra XP via `AwardXPAmount`.
- [ ] REQ-AE-23: `world.Room` MUST gain `BossRoom bool` (YAML: `boss_room,omitempty`).
- [ ] REQ-AE-24: Map renderer MUST render boss room tiles as `<BB>` (angle brackets).
- [ ] REQ-AE-25: On boss respawn in a boss room, the `RespawnManager` MUST cancel pending timers and immediately respawn all `Room.Spawns` entries.
- [ ] REQ-AE-26: Non-boss deaths while boss is alive follow individual respawn timers.
- [ ] REQ-AE-27: `world.Room` MUST gain `Hazards []HazardDef` (YAML: `hazards,omitempty`); `HazardDef` MUST use exported fields with YAML tags; `HazardDef.Validate()` MUST reject invalid `Trigger` or empty `DamageExpr`.
- [ ] REQ-AE-28: `"on_enter"` hazards fire on player room entry; `"round_start"` hazards fire each combat round for all players.
- [ ] REQ-AE-29: Hazard damage applied directly to `sess.CurrentHP`; condition applied if `ConditionID` is non-empty.
- [ ] REQ-AE-30: Hazard `Message` MUST be sent to each affected player via `conn.WriteConsole`.
- [ ] REQ-AE-31: `npc.Template` MUST gain `BossAbilities []BossAbility` (YAML: `boss_abilities`); all `BossAbility` and `BossAbilityEffect` fields MUST be exported with YAML tags.
- [ ] REQ-AE-32: `BossAbilityEffect` MUST have exactly one field set: `AoeCondition` (non-empty string), `AoeDamageExpr` (non-empty string), or `HealPct` (non-zero int).
- [ ] REQ-AE-33: `Template.Validate()` MUST reject `BossAbility` with empty `ID`/`Name`, invalid `Trigger`, unparseable non-empty `Cooldown`, non-zero `TriggerValue` on `"on_damage_taken"`, or `BossAbilityEffect` not satisfying REQ-AE-32.
- [ ] REQ-AE-34: `npc.Instance` MUST gain `AbilityCooldowns map[string]time.Time` initialized at spawn.
- [ ] REQ-AE-35: Combat engine MUST evaluate boss abilities each round using trigger type, trigger value, and cooldown.
- [ ] REQ-AE-36: On ability fire: announce, execute effect, record cooldown (empty `Cooldown` means no cooldown entry recorded).
- [ ] REQ-AE-37: AoE damage MUST be rolled once and applied identically to all players; not mitigated by AC.
- [ ] REQ-AE-38: Boss ability processing MUST occur before normal NPC attack resolution in the same round.
- [ ] REQ-AE-39: `npc.Template.SpecialAbilities []string` (YAML: `special_abilities`) MUST be renamed to `SenseAbilities []string` (YAML: `sense_abilities`); all code references and NPC YAML files MUST be updated; no backwards-compatible aliasing.
