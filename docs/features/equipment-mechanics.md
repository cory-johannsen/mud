# Equipment Mechanics Expansion

Adds rarity tiers, per-instance durability, item modifiers (tuned/defective/cursed), equipment set bonuses, team-based consumable effectiveness, and five new consumable items. See `docs/superpowers/specs/2026-03-21-equipment-mechanics-design.md` for full design spec.

## Requirements

### Rarity

- [ ] REQ-EM-1: `rarity` field required on all weapon/armor YAML; absence fatal at startup
- [ ] REQ-EM-2: Base stats multiplied by rarity stat multiplier at load time; per-instance modifier effects applied on top at resolution
- [ ] REQ-EM-3: Min level enforced at equip time with message: `"You need to be level <N> to equip <item name>."`
- [ ] REQ-EM-4: Item names color-coded by rarity tier (ANSI) in all inventory/equipment displays

### Durability

- [ ] REQ-EM-5: Active weapon loses 1 durability per attack roll (hit or miss)
- [ ] REQ-EM-6: Struck armor slot loses 1 durability per hit received
- [ ] REQ-EM-7: Broken armor (Durability==0) contributes 0 AC; broken weapon contributes 0 damage beyond unarmed base
- [ ] REQ-EM-8: Modifier adjustments not applied to broken items
- [ ] REQ-EM-9: Destruction roll made when item reaches 0 durability
- [ ] REQ-EM-10: Destroyed items removed permanently; player notified with `"Your <item name> has been destroyed."`
- [ ] REQ-EM-11: Broken items remain and can be repaired
- [ ] REQ-EM-12: MaxDurability and DestructionChance MUST match Section 2.3 constants in spec
- [ ] REQ-EM-13: `repair <item>` requires `repair_kit`; handler consumes kit before calling RepairField
- [ ] REQ-EM-14: `repair <item>` restores 1d6 durability, capped at MaxDurability
- [ ] REQ-EM-15: `repair <item>` costs 1 AP in combat
- [ ] REQ-EM-16: Downtime Repair restores full durability; cost = `ceil((Max-Current)/10)` days, min 1
- [ ] REQ-EM-17: `durability == -1` sentinel triggers InitDurability (set to MaxDurability) on load

### Modifiers

- [ ] REQ-EM-18: Tuned items display as `"Tuned <name>"`
- [ ] REQ-EM-19: Defective items display as `"Defective <name>"`
- [ ] REQ-EM-20: Cursed items display as normal name until equipped; then `"Cursed <name>"`
- [ ] REQ-EM-21: Modifier effects applied at resolution time on top of rarity-multiplied base
- [ ] REQ-EM-22: `ComputedDefenses` applies modifier AC adjustment per SlottedItem; 0 if broken
- [ ] REQ-EM-23: Attack handler applies modifier damage adjustment per EquippedWeapon; 0 if broken
- [ ] REQ-EM-24: `unequip` fails for cursed items with `"This item is cursed and cannot be removed."`
- [ ] REQ-EM-25: Uncursed items transition to defective (Modifier="defective", CurseRevealed=false)
- [ ] REQ-EM-26: Modifier spawn probabilities MUST match Section 3.2 constants in spec
- [ ] REQ-EM-27: Merchants MUST NOT stock cursed items

### Equipment Sets

- [ ] REQ-EM-28: `threshold: full` resolves to `len(pieces)` at load time
- [ ] REQ-EM-29: Set bonuses evaluated at login and on equipment change
- [ ] REQ-EM-30: Set bonuses rarity-independent
- [ ] REQ-EM-31: Active set bonuses displayed on character sheet
- [ ] REQ-EM-32: Unrecognized set effect types fatal at load
- [ ] REQ-EM-33: Unresolvable `condition_id` in set bonus fatal at load
- [ ] REQ-EM-34: `SetRegistry.ActiveBonuses` is a pure function
- [ ] REQ-EM-35: Derived-stats path applies all active set bonuses

### Team Mechanic

- [ ] REQ-EM-36: Invalid `ItemDef.Team` value fatal at load
- [ ] REQ-EM-37: Consumable effects multiplied by team effectiveness (1.25× match / 0.75× oppose / 1.0× neutral)
- [ ] REQ-EM-38: Multiplied values floored
- [ ] REQ-EM-39: `PlayerSession.Team` loaded from `characters.team`; empty = 1.0× multiplier

### New Consumables

- [ ] REQ-EM-40: All six new items (Whore's Pasta, Poontangesca, 4Loko, Old English, Penjamin Franklin, Repair Kit) loaded at startup; missing YAML files fatal
- [ ] REQ-EM-41: `consume_check` uses d20 + stat modifier vs DC; PF2E four-tier critical failure applies `on_critical_failure` effects
- [ ] REQ-EM-42: `apply_disease` and `apply_toxin` condition IDs validated at load; unresolvable IDs fatal
- [ ] REQ-EM-43: `remove_conditions` clears listed conditions before applying new ones

### Architecture

- [ ] REQ-EM-44: `DeductDurability` and `RepairField` are pure functions; all DB persistence in caller
- [ ] REQ-EM-45: `ApplyConsumable` uses `ConsumableTarget` interface; MUST NOT import `internal/game/session`
