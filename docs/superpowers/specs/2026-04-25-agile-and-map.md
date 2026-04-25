---
title: Agile Weapon Trait + Multiple Attack Penalty Reduction
issue: https://github.com/cory-johannsen/mud/issues/262
date: 2026-04-25
status: spec
prefix: AGILE
depends_on:
  - "#253 Move trait for weapons (introduces internal/game/inventory/traits/ registry)"
related:
  - "#259 Bonus types (combat log breakdown surface)"
  - "#260 Natural 1/20 (sister attack-roll annotation)"
---

# Agile Weapon Trait + MAP Reduction

## 1. Summary

The Multiple Attack Penalty (MAP) is already fully implemented and tested:

- `mapPenaltyFor(attacksMade int)` (`internal/game/combat/round.go:385-394`) returns `0` for the first attack, `-5` for the second, `-10` for the third or later.
- `Combatant.AttacksMadeThisRound` counts attacks per round and resets in `StartRoundWithSrc`.
- Tests at `round_conditions_test.go:143-221` pin both `ActionAttack` (cross-action) and `ActionStrike` (intra-action) MAP behavior.

What is missing for issue #262 is a single rule: when the wielder's active weapon has the **agile** trait, the MAP becomes `-4 / -8` instead of `-5 / -10`. Today every weapon's MAP is the default. The agile trait already appears in some weapon YAML (e.g., `combat_knife.yaml: traits: [agile, finesse, gun-team]`) but the engine ignores it (`WeaponDef.Traits` is display-only — same status as the `move` trait until spec #253 lands the trait registry).

The combat-log breakdown for MAP is also currently silent: `mapPenaltyFor` applies `-5` or `-10` to the attack total but no narrative suffix tells the player why their roll was lower. The issue asks for the breakdown to "show the MAP value and source" — a small annotation change in the existing narrative emitter.

This spec depends on #253's trait registry for the recognition substrate, then ships:

1. The `agile` trait constant in the registry with a `MAP` reduction `Behavior` field.
2. A weapon-aware `mapPenaltyFor` that reads the wielder's active weapon and applies the reduction when `WeaponDef.HasTrait("agile")`.
3. A narrative annotation on every attack at MAP > 0 showing the penalty value and source.
4. Validation that existing tests for the default MAP continue to pass; new tests cover agile MAP reduction and the cross-action interaction.

## 2. Goals & Non-Goals

### 2.1 Goals

- AGILE-G1: A weapon with the `agile` trait reduces MAP to `-4` (second attack) and `-8` (third+).
- AGILE-G2: MAP is computed from the *wielder's currently active weapon at attack time*, not the weapon used for the prior attack — important when the player swaps weapons mid-turn.
- AGILE-G3: Combat-log narrative for every attack at MAP > 0 includes a clear annotation (`[MAP -5: 2nd attack]` or `[MAP -4: 2nd attack, agile]`).
- AGILE-G4: Existing MAP tests at `round_conditions_test.go:143-221` and `round_test.go` continue to pass.
- AGILE-G5: The trait registry from #253 is the only place where the agile→MAP-reduction rule is declared; resolver code consults the registry, not a hardcoded trait id.

### 2.2 Non-Goals

- AGILE-NG1: Implementing additional trait-driven attack mechanics (e.g., `forceful` ramp damage, `sweep` adjacency bonus). Each gets its own ticket; this one ships agile only.
- AGILE-NG2: Changing how MAP is counted (the `AttacksMadeThisRound` counter remains the source of truth).
- AGILE-NG3: Per-weapon MAP profiles authored in YAML (e.g., a weapon with `MAP: -3/-6` declared directly). The trait is the only authoring surface.
- AGILE-NG4: Asymmetric MAP (where the second attack with weapon A applies a different rule than the second attack with weapon B). The wielder's *current* weapon at attack time decides; switching weapons does not reset the counter.
- AGILE-NG5: A "Quick Trick" / "Triple Attack" feat that further modifies MAP. Out of scope.

## 3. Glossary

- **MAP (Multiple Attack Penalty)**: penalty applied to a combatant's second and subsequent attacks within a single round.
- **Agile**: a weapon trait that reduces MAP by 1 step (`-5/-10` → `-4/-8`).
- **Wielder's active weapon**: the weapon currently equipped in the combatant's primary hand at attack-resolution time.
- **Trait registry**: the map introduced by spec #253 at `internal/game/inventory/traits/` mapping trait id → behavior metadata.

## 4. Requirements

### 4.1 Trait Registry Entry

- AGILE-1: The trait registry from #253 MUST gain an entry `traits.Agile = "agile"` with a `Behavior` field `MAPReductionSteps int` set to `1`.
- AGILE-2: When #253's spec lands first, this spec consumes the existing registry shape; when this spec lands first, it adds the `Agile` constant and `MAPReductionSteps` field to the registry contract authored in #253. Either ordering produces the same end state — the implementer of the second one MUST verify the registry exposes both `Move` and `Agile` correctly.
- AGILE-3: `WeaponDef.HasTrait("agile")` (helper from #253 WMOVE-5) MUST be the only call any non-registry code makes when checking for the trait.

### 4.2 MAP Computation

- AGILE-4: `mapPenaltyFor` MUST be refactored to take the wielder's `*WeaponDef` parameter: `mapPenaltyFor(attacksMade int, weapon *inventory.WeaponDef) int`.
- AGILE-5: The function MUST consult the trait registry: when `weapon != nil && weapon.HasTrait("agile")`, return `0`, `-4`, `-8` for attack indices `1`, `2`, `3+`. Otherwise return the existing `0`, `-5`, `-10`.
- AGILE-6: All call sites of `mapPenaltyFor` MUST be updated to pass the attacker's currently-equipped weapon. Where the attack is unarmed, `weapon` is `nil` and the default penalties apply.
- AGILE-7: When `MAPReductionSteps > 1` (reserved for future traits), the penalties MUST be reduced by `5 * steps` per step but not below `0`. v1 only ships agile (1 step), but the function MUST be future-proofed.
- AGILE-8: When the wielder swaps weapons mid-turn between attacks, each attack uses the weapon active at the moment of resolution. Reading the `Combatant.WeaponID` field at attack time gives the correct weapon — confirm in test (AGILE-15).

### 4.3 Combat Log Annotation

- AGILE-9: `attackNarrative` (`internal/game/combat/round.go:163-199`) MUST emit a `[MAP <value>: <ordinal> attack<, agile?>]` suffix when `mapPenaltyFor` returned a non-zero value. Examples:
  - `[MAP -5: 2nd attack]`
  - `[MAP -4: 2nd attack, agile]`
  - `[MAP -10: 3rd attack]`
  - `[MAP -8: 3rd attack, agile]`
- AGILE-10: The annotation MUST appear inline with the existing `1d20 (N) ±mod = total vs AC X` breakdown so the player sees the MAP component without expanding any UI.
- AGILE-11: First-attack narratives (MAP = 0) MUST NOT include the annotation.

### 4.4 Tests

- AGILE-12: All existing MAP tests MUST pass unchanged. Specifically:
  - `TestResolveRound_CrossAction_MAP_SecondAttackAt5`
  - `TestResolveRound_CrossAction_MAP_ThirdAttackAt10`
  - `TestResolveRound_CrossAction_MAP_ResetsAtRoundStart`
  - `TestResolveRound_Strike_MAPPenalty`
  These all use weapons without the `agile` trait so they continue to see `-5 / -10`.
- AGILE-13: New tests in `internal/game/combat/round_agile_map_test.go` MUST cover:
  - Second attack with an agile weapon: `MAP == -4`.
  - Third attack with an agile weapon: `MAP == -8`.
  - Cross-action: two `ActionAttack` actions, agile weapon → second at `-4`.
  - Intra-action: `ActionStrike` (two attacks bundled) with agile weapon → second at `-4`.
  - Round reset: agile MAP resets to `0` at `StartRound`.
- AGILE-14: A test MUST cover the future-proofing of AGILE-7: simulate `MAPReductionSteps == 2` and verify penalties become `-3 / -6`.
- AGILE-15: A test MUST cover the weapon-swap scenario: first attack with non-agile (penalty `-5` for next attack), swap to agile, second attack reads agile (penalty `-4`); third attack reads whatever is equipped then.
- AGILE-16: A test MUST verify the narrative annotation format (AGILE-9) for both default and agile weapons.

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/inventory/traits/
  registry.go           # existing (#253); add Agile constant + MAPReductionSteps field

internal/game/combat/
  round.go              # mapPenaltyFor signature change; attackNarrative annotation
  round_agile_map_test.go # NEW

internal/gameserver/
  combat_handler.go     # update any direct callers of mapPenaltyFor (none expected outside round.go)
```

### 5.2 Attack flow with MAP

```
ResolveRound
   │
   ├── for each queued attack action by attacker:
   │
   ▼
attackerWeapon := lookupWeapon(attacker.WeaponID)
mapPenalty := mapPenaltyFor(attacker.AttacksMadeThisRound, attackerWeapon)
attackTotal := d20 + attackerMods + mapPenalty
   │
   ▼
attackNarrative(...) emits "[MAP -X: nth attack[, agile]]" suffix when mapPenalty < 0
   │
   ▼
attacker.AttacksMadeThisRound += 1 (existing)
```

### 5.3 Single sources of truth

- Trait id and reduction steps: `internal/game/inventory/traits/registry.go` only.
- MAP computation: `mapPenaltyFor` only.
- Annotation format: `attackNarrative` only.

## 6. Open Questions

- AGILE-Q1: When the wielder dual-wields and the off-hand attack is with an agile weapon while the main-hand was not, does the off-hand attack get the agile MAP? PF2E says yes — the *attacking* weapon's traits decide. AGILE-8 captures this rule. Confirm with the user when implementation starts.
- AGILE-Q2: Should the annotation also reveal the *other* MAP-relevant traits if/when they exist (e.g., a future `nimble` trait)? Recommendation: yes — the annotation notes any contributing trait by id. v1 only has `agile`.
- AGILE-Q3: The current `mapPenaltyFor` signature is `(attacksMade int) int`. Adding a `*WeaponDef` parameter is a breaking change. Should we add a `MapPenaltyFor(weapon, attacksMade)` method on `Combatant` instead and migrate gradually? Recommendation: change the function in one PR — only round.go consumes it.
- AGILE-Q4: When the `agile` trait shows up on a weapon authored before #253's registry validation, does the loader log a warning (per WMOVE-4)? Recommendation: yes, but the registry is built first so the warning never fires for stock content.
- AGILE-Q5: Should the agile MAP reduction also apply when the action is a *non-attack* (e.g., a maneuver like Trip)? PF2E says yes — agile weapons reduce MAP for any check that "counts as an attack". Recommendation: scope v1 to attacks proper. Maneuvers can be folded in later when the skill-action resolver (#252) wires up MAP-counting.

## 7. Acceptance

- [ ] All existing MAP tests pass without modification.
- [ ] New `round_agile_map_test.go` tests pass.
- [ ] A weapon with `traits: [agile]` produces a second attack at `-4` MAP.
- [ ] A weapon without the trait still produces `-5 / -10`.
- [ ] Combat log annotation appears on every attack at MAP > 0 with the correct format.
- [ ] Trait registry exposes `Agile` and `Move` simultaneously (regardless of which spec landed first).
- [ ] Weapon-swap mid-turn correctly applies per-attack-time trait reading.

## 8. Out-of-Scope Follow-Ons

- AGILE-F1: Other weapon traits (`forceful`, `sweep`, `backswing`, `propulsive`, `fatal`, `deadly`, `thrown`, `reach`, `finesse`).
- AGILE-F2: MAP reduction for skill-action maneuvers (per AGILE-Q5).
- AGILE-F3: Feats / abilities that further modify MAP (e.g., a hypothetical `Triple Attack` reducing the third-attack MAP by an additional step).
- AGILE-F4: Sound / visual cue when MAP fires.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/262
- Existing MAP function: `internal/game/combat/round.go:385-394` (`mapPenaltyFor`)
- Per-round attack counter: `Combatant.AttacksMadeThisRound`, reset in `StartRoundWithSrc`
- Existing MAP tests: `internal/game/combat/round_conditions_test.go:143-221`
- Strike MAP test: `internal/game/combat/round_test.go` (`TestResolveRound_Strike_MAPPenalty`)
- Trait registry (predecessor spec): `docs/superpowers/specs/2026-04-24-move-trait-for-weapons.md` WMOVE-1..5
- Weapon traits field: `internal/game/inventory/weapon.go:48`
- Combat narrative emitter: `internal/game/combat/round.go:163-199` (`attackNarrative`)
- Bonus breakdown precedent: `docs/superpowers/specs/2026-04-25-bonus-types.md` BTYPE-9
