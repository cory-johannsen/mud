# Non-Human NPCs

Adds animals, robots, and machines as distinct NPC types with unique combat verbs, loot rules, damage type interactions, and behavioral constraints.

Design spec: `docs/superpowers/specs/2026-03-21-non-human-npcs-design.md`

## Requirements

- [ ] REQ-NHN-1: Canonical `type` values MUST be `"animal"`, `"robot"`, `"machine"` (plus existing `"human"`, `"mutant"`).
- [ ] REQ-NHN-2: `Template.Validate()` MUST NOT reject unknown `type` values.
- [ ] REQ-NHN-3: `Template` MUST gain `IsAnimal()`, `IsRobot()`, `IsMachine()` bool helpers.
- [ ] REQ-NHN-4: `Template` MUST gain `AttackVerb string` (YAML: `attack_verb`; default `"attacks"`).
- [ ] REQ-NHN-5: `SpawnInstance` MUST propagate `AttackVerb` from template.
- [ ] REQ-NHN-6: `combat.Combatant` MUST gain `AttackVerb string`; NPC attackers use `Instance.AttackVerb`; player attackers default to `"attacks"`.
- [ ] REQ-NHN-7: `attackNarrative` MUST use `actor.AttackVerb` for all three attack slots.
- [ ] REQ-NHN-8: Animal `attack_verb` MUST be one of `"bites"`, `"claws"`, `"mauls"`, or `"slams"`. (Content convention.)
- [ ] REQ-NHN-9: `LootTable` MUST gain `OrganicDrops []OrganicDrop` (YAML: `organic_drops`) with fields `item_id`, `weight`, `quantity_min`, `quantity_max`. `LootTable.Validate()` MUST reject `weight <= 0`, `quantity_min < 1`, or `quantity_max < quantity_min`.
- [ ] REQ-NHN-10: Animal death MUST roll one `OrganicDrop` (weighted random) and place the quantity (uniform in `[quantity_min, quantity_max]`) on the room floor.
- [ ] REQ-NHN-11: `Template.Validate()` MUST return error if `IsAnimal()` is true and `Loot` contains credits, equipment, or `salvage_drop` entries.
- [ ] REQ-NHN-12: Animal faction enforcement deferred to `factions` feature; content convention in the interim.
- [ ] REQ-NHN-13: HTN planner MUST remove `say` operator tasks from plan before execution for animal instances. If the plan is then empty, fall back to simple attack behavior.
- [ ] REQ-NHN-14: Robot `SpawnInstance` MUST default `Resistances["bleed"] = 999` and `Resistances["poison"] = 999`.
- [ ] REQ-NHN-15: Template-defined resistance values MUST override robot defaults in `SpawnInstance`.
- [ ] REQ-NHN-16: `"electric"` is the canonical damage type for EMP/electric. Robot templates SHOULD set `Weaknesses["electric"]`. (Content convention.)
- [ ] REQ-NHN-17: `LootTable` MUST gain `SalvageDrop *SalvageDrop` (YAML: `salvage_drop`) with fields `item_ids []string`, `quantity_min int`, `quantity_max int`. `LootTable.Validate()` MUST reject non-nil `SalvageDrop` with empty `item_ids`, `quantity_min < 1`, or `quantity_max < quantity_min`.
- [ ] REQ-NHN-18: Robot death MUST select one item uniformly from `SalvageDrop.item_ids` and place quantity (uniform in `[quantity_min, quantity_max]`) on room floor, if `SalvageDrop` is non-nil.
- [ ] REQ-NHN-19: Robot `attack_verb` MUST be `"shoots"`, `"slams"`, or `"zaps"`. (Content convention.)
- [ ] REQ-NHN-20: `Template` MUST gain `Immobile bool` (YAML: `immobile`; default `false`).
- [ ] REQ-NHN-21: `SpawnInstance` MUST propagate `Immobile` from template.
- [ ] REQ-NHN-22: NPC movement system MUST skip instances where `instance.Immobile == true`.
- [ ] REQ-NHN-23: Machine templates MUST set `immobile: true`. (Content convention.)
- [ ] REQ-NHN-24: Machine templates MUST use `on_enemy_enters_room` trigger. Movement operators already blocked by REQ-NHN-22. (Content convention.)
- [ ] REQ-NHN-25: Machine death message MUST be `"<Name> is destroyed."` (determined by `instance.IsMachine()`).
- [ ] REQ-NHN-26: Machine `SpawnInstance` MUST apply same bleed/poison resistance defaults as robots. Template-defined values MUST override machine defaults (same rule as REQ-NHN-15).
- [ ] REQ-NHN-27: Machine loot uses existing equipment loot table; no credits. (Content convention.)
- [ ] REQ-NHN-28: `internal/game/combat/damage_types.go` MUST define constants: `DamageTypeBleed`, `DamageTypePoison`, `DamageTypeElectric`, `DamageTypePhysical`, `DamageTypeFire`.
- [ ] REQ-NHN-29: Internal code MUST use `damage_types.go` constants wherever these strings are referenced.
