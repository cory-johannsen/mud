# Consumable Traps

Trap items as craftable and purchasable consumables that players deploy in or out of combat. A deployed trap is armed at the player's current combat position and fires when a combatant moves within trigger range. Extends the base `traps` feature with player-deployable trap items backed by `TrapTemplate` definitions.

See `docs/superpowers/specs/2026-03-21-consumable-traps-design.md` for the full design spec.

## Requirements

- [x] Trap consumable items
  - REQ-CTR-14: `ItemDef` MUST support `kind: trap` with a `trap_template_ref` field referencing a `TrapTemplate` ID.
  - REQ-CTR-13: `TrapTemplate` MUST expose `trigger_range_ft` (default 5) and `blast_radius_ft` (default 0) fields.
  - REQ-CTR-15: The `trap` package MUST export a `TrapKindConsumable = "consumable"` constant.
- [x] `deploy <trap item>` command
  - REQ-CTR-1: A `deploy <item>` command MUST exist, costing 1 AP in combat and 0 AP outside combat.
  - REQ-CTR-2: The `deploy` command MUST only accept items with `kind == "trap"`.
  - REQ-CTR-3: Deploying a trap MUST remove exactly 1 unit from the item stack in the player's backpack.
  - REQ-CTR-4: A deployed trap MUST be armed at the player's current combat position in combat, or at position 0 outside combat.
  - REQ-CTR-5: A deployed trap MUST be position-anchored — subsequent movement by the deploying player MUST NOT change the trap's `DeployPosition`.
- [x] TrapManager integration
  - REQ-CTR-16: `TrapManager` MUST provide `AddConsumableTrap(instanceID string, tmpl *TrapTemplate, deployPos int) error`.
  - REQ-CTR-17: `CombatHandler` MUST provide `CombatantPosition(uid string) int`.
- [x] Combat trigger (positional)
  - REQ-CTR-6: In combat, a deployed trap MUST fire when a combatant moves within `TriggerRangeFt` feet of `DeployPosition`.
  - REQ-CTR-7: Multiple overlapping consumable traps MUST all fire independently when a combatant enters their trigger range on the same movement action.
  - REQ-CTR-8: A consumable trap with `BlastRadiusFt == 0` MUST apply its payload to the triggering combatant only.
  - REQ-CTR-9: A consumable trap with `BlastRadiusFt > 0` MUST apply its payload to all combatants within `BlastRadiusFt` feet of `DeployPosition` at trigger time, including the deploying player if in range.
  - REQ-CTR-10: A consumable trap MUST be one-shot — it MUST be disarmed immediately after firing regardless of the template's `reset_mode`.
- [x] Out-of-combat deploy
  - REQ-CTR-11: A consumable trap deployed outside combat MUST fire on the next room entry that meets the trigger conditions.
- [x] Detectability and disarm
  - REQ-CTR-12: Deployed consumable traps MUST be detectable and disarmable via the existing `disarm` command.
- [x] Ready action integration
  - The ready-action feature uses this floor-state tracking to support "enemy enters room" as a trigger condition.

## Implementation

Completed 2026-03-21. `TrapTemplate` gained `trigger_range_ft` and `blast_radius_ft` fields. `TrapInstanceState` gained `DeployPosition` and `IsConsumable` fields. `TrapManager.AddConsumableTrap` arms player-deployed instances. `CombatHandler.SetOnCombatantMoved`/`CombatantPosition`/`CombatantsInRoom` support positional trigger checks. `checkConsumableTraps` and `fireConsumableTrapOnCombatant` in the service layer handle combat trigger + blast radius. `handleDeployTrap` dispatched via proto field 85.
