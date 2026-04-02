# Reactive Block Fix

## Summary

Reactive Block (`reactive_block`) is a job feat for the Aggressor archetype that fires as a reaction when the player takes damage, prompting them to spend their reaction to reduce incoming damage via shield hardness. Currently the reaction fires correctly (modal appears) but subtracts zero damage because `WeaponDef.Hardness` is not modelled and `shieldHardness()` always returns 0. In addition, no requirement check enforces that a shield is equipped in the off-hand.

## Requirements

- REQ-RBF-1: `WeaponDef` MUST include a `Hardness int` field populated from YAML.
- REQ-RBF-2: Shield item definitions in `content/items/` MUST include a non-zero `hardness` value.
- REQ-RBF-3: `shieldHardness()` MUST return the `Hardness` value of the equipped off-hand shield, or 0 if none.
- REQ-RBF-4: `CheckReactionRequirement` MUST support a `"wielding_shield"` requirement that returns true when the active loadout preset has a shield (`WeaponKindShield`) equipped in the off-hand.
- REQ-RBF-5: The `reactive_block` feat YAML MUST include `requirement: wielding_shield` so the reaction is suppressed when no shield is equipped.
- REQ-RBF-6: After applying the `reduce_damage` effect, the server MUST push a message to the player indicating how much damage was blocked (e.g. "Reactive Block: blocked 3 damage."). If hardness equals or exceeds the incoming damage, the message MUST indicate the attack was fully blocked.
- REQ-RBF-7: All changes MUST be covered by new unit tests.

## Scope

- `internal/game/inventory/weapon.go` — add `Hardness int` to `WeaponDef`
- `content/items/*.yaml` (or wherever shields are defined) — populate `hardness` values
- `internal/gameserver/reaction_handler.go` — update `shieldHardness()` and `CheckReactionRequirement`
- `content/feats.yaml` — add `requirement: wielding_shield` to `reactive_block`
- `internal/gameserver/reaction_handler.go` — push feedback message after `reduce_damage` fires
- Tests in `internal/gameserver/reaction_handler_test.go` and `internal/game/inventory/`
