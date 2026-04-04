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
- `content/weapons/*.yaml` — create shield weapon definitions with `hardness` values
- `internal/gameserver/reaction_handler.go` — update `shieldHardness()` and `CheckReactionRequirement`; add feedback message after `reduce_damage` fires
- `content/feats.yaml` — add `requirement: wielding_shield` to `reactive_block`
- Tests in `internal/gameserver/reaction_handler_test.go` and `internal/game/inventory/`

## Plan

All exact line numbers and struct definitions confirmed by reading source files before planning.

### Step 1 — Add `Hardness int` to `WeaponDef` (`internal/game/inventory/weapon.go`)

In the `WeaponDef` struct (line 39), add after `UpgradeSlots int` (line 61):

```go
// Hardness is the damage reduction provided when this weapon (shield) is used
// to block. Only relevant for WeaponKindShield. Loaded from YAML.
Hardness int `yaml:"hardness"`
```

No other changes to weapon.go needed — `IsShield()` already exists at line 100.

### Step 2 — Create shield weapon YAML files (`content/weapons/`)

Create three shield definitions. Each file must include `kind: shield`, a non-zero `hardness`, and `range_increment: 0`. Suggested files and values:

- `content/weapons/scrap_shield.yaml` — improvised salvage-tier shield, `hardness: 2`
- `content/weapons/riot_shield.yaml` — police-grade polycarbonate, `hardness: 3`, rarity `street`
- `content/weapons/ballistic_shield.yaml` — military-grade composite, `hardness: 5`, rarity `mil_spec`

Each must include all required `WeaponDef` fields: `id`, `name`, `description`, `damage_dice`, `damage_type`, `range_increment: 0`, `kind: shield`, `group`, `proficiency_category`, `rarity`, `hardness`.

### Step 3 — Fix `shieldHardness()` (`internal/gameserver/reaction_handler.go` lines 88–101)

Replace lines 99–100:
```go
// WeaponDef.Hardness is not yet modelled; return 0 until the field is added.
return 0
```
with:
```go
return preset.OffHand.Def.Hardness
```

Remove the stale TODO comment on line 83–84.

### Step 4 — Add `wielding_shield` case to `CheckReactionRequirement` (`reaction_handler.go` lines 29–44)

Insert a new case between `wielding_melee_weapon` and `default`:

```go
case "wielding_shield":
    if sess.LoadoutSet == nil {
        return false
    }
    preset := sess.LoadoutSet.ActivePreset()
    if preset == nil || preset.OffHand == nil || preset.OffHand.Def == nil {
        return false
    }
    return preset.OffHand.Def.Kind == inventory.WeaponKindShield
```

### Step 5 — Add `requirement: wielding_shield` to `reactive_block` feat (`content/feats.yaml` lines 2342–2353)

Verify where `Requirement` maps from in the feat-loading code (check the struct that `pr.Def` is an instance of). Add `requirement: wielding_shield` at the same level as `triggers` and `effect` inside the `reaction:` block, or at the feat top level — whichever matches the struct's YAML tag. The current entry:

```yaml
  - id: reactive_block
    ...
    reaction:
      triggers: [on_damage_taken]
      effect:
        type: reduce_damage
```

Expected result (requirement at feat top level, following the pattern used by other feats):

```yaml
  - id: reactive_block
    ...
    requirement: wielding_shield
    reaction:
      triggers: [on_damage_taken]
      effect:
        type: reduce_damage
```

If the struct tag maps `requirement` from inside the `reaction:` block, place it there instead — verify before editing.

### Step 6 — Add feedback message in `buildReactionCallback` (`reaction_handler.go` lines 255–261)

Replace:
```go
sess.ReactionsRemaining--
ApplyReactionEffect(sess, pr.Def.Effect, &ctx)
return true, nil
```
with:
```go
sess.ReactionsRemaining--
damageBeforeEffect := 0
if ctx.DamagePending != nil {
    damageBeforeEffect = *ctx.DamagePending
}
ApplyReactionEffect(sess, pr.Def.Effect, &ctx)
if pr.Def.Effect.Type == reaction.ReactionEffectReduceDamage && ctx.DamagePending != nil {
    blocked := damageBeforeEffect - *ctx.DamagePending
    if blocked >= damageBeforeEffect {
        s.pushMessageToUID(uid, "Reactive Block: fully blocked the attack.")
    } else {
        s.pushMessageToUID(uid, fmt.Sprintf("Reactive Block: blocked %d damage.", blocked))
    }
}
return true, nil
```

Add `"fmt"` to imports if not already present.

### Step 7 — Write tests

**`internal/gameserver/reaction_handler_test.go`** — add after the existing tests:

- `TestCheckReactionRequirement_WieldingShield_FalseWhenNoLoadout` — nil LoadoutSet → false
- `TestCheckReactionRequirement_WieldingShield_TrueWhenShieldEquipped` — OffHand has `WeaponKindShield` → true
- `TestCheckReactionRequirement_WieldingShield_FalseWhenOffHandNotShield` — OffHand has `WeaponKindOneHanded` → false
- `TestShieldHardness_ReturnsHardnessFromOffHandShield` — equip shield with Hardness=3; `shieldHardness()` returns 3
- `TestShieldHardness_ReturnsZeroWhenNoOffHand` — nil OffHand → 0
- `TestShieldHardness_ReturnsZeroWhenOffHandNotShield` — OffHand is one-handed weapon → 0
- `TestApplyReactionEffect_ReduceDamage_SubtractsShieldHardness` — shield Hardness=3, damage=7 → remaining=4
- `TestApplyReactionEffect_ReduceDamage_ClampsAtZeroWhenHardnessExceedsDamage` — shield Hardness=5, damage=3 → remaining=0

**`internal/game/inventory/`** — add a test verifying `WeaponDef.Hardness` is populated when loading a shield YAML with `hardness: 3`.

### Step 8 — Run tests and commit

```
mise exec -- go test ./internal/game/inventory/... ./internal/gameserver/...
```

All tests must pass. Commit: WeaponDef.Hardness field, shield weapon definitions, shieldHardness fix, wielding_shield requirement, feedback message, tests.
