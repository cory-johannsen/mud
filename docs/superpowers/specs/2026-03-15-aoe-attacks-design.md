# AoE Attack Type — Design Spec

**Date:** 2026-03-15

---

## Goal

Extend the existing explosive/throw system to support a `friendly_fire` flag and attacker-level-scaled save DCs. Update all existing grenade-type items with appropriate values. The Reflex-save-based damage resolution (`ResolveExplosive`) is already implemented; this spec closes the remaining gaps.

---

## Feature 1: Data Model

### ExplosiveDef extension (`internal/game/inventory/explosive.go`)

Add two fields to `ExplosiveDef`:

```go
// FriendlyFire controls whether this explosive damages allied combatants.
// When false (default), only enemy-kind combatants are targeted.
// When true, all living combatants except the thrower are targeted.
FriendlyFire bool `yaml:"friendly_fire,omitempty"`

// AoERadius is the blast radius in feet for AreaTypeBurst explosives.
// Stored for future position-based filtering; not actively used in target selection
// for this spec. Zero means room-wide effect (current behavior).
AoERadius int `yaml:"aoe_radius,omitempty"`
```

The `SaveDC` field already exists and represents the **base DC**. The effective DC applied in resolution is `SaveDC + attacker.Level`. No rename needed; the scaling is applied at resolution time.

### Updated YAML fields for existing explosives

| Item | aoe_radius | friendly_fire | Notes |
|------|------------|---------------|-------|
| frag_grenade | 0 | false | area_type: room; no change to radius |
| incendiary_grenade | 0 | false | area_type: room |
| emp_grenade | 0 | false | area_type: room |

`aoe_radius: 0` with `area_type: room` means all living targets in the room (existing behavior preserved).

---

## Feature 2: Resolution Changes

### Target collection (`internal/game/combat/round.go` — `resolveThrow`)

Replace the call to `livingEnemiesOf` in `resolveThrow` with a new helper `explosiveTargetsOf`. `livingEnemiesOf` is NOT deleted — it is used in other resolution paths and must remain.

Add `explosiveTargetsOf`:

```go
// explosiveTargetsOf returns the targets for an explosive based on its friendly_fire flag.
//
// Precondition: cbt and actor must be non-nil; grenade must be non-nil.
// Postcondition: returns living combatants that should be affected by the explosive.
//   If grenade.FriendlyFire is false, only enemy-kind combatants are returned.
//   If grenade.FriendlyFire is true, all living combatants except the actor are returned.
func explosiveTargetsOf(cbt *Combat, actor *Combatant, grenade *inventory.ExplosiveDef) []*Combatant {
    var out []*Combatant
    for _, c := range cbt.Combatants {
        if c.IsDead() || c.ID == actor.ID {
            continue
        }
        if !grenade.FriendlyFire && c.Kind == actor.Kind {
            continue
        }
        out = append(out, c)
    }
    return out
}
```

### Attacker-level DC scaling (`internal/game/combat/round.go` — `resolveThrow`)

Compute the effective DC before calling `ResolveExplosive`:

```go
effectiveDC := grenade.SaveDC + actor.Level
```

Pass `effectiveDC` into `ResolveExplosive` by threading it through. Since `ResolveExplosive` currently reads `grenade.SaveDC` directly, the cleanest approach is to pass the effective DC as a parameter override. Modify the signature:

```go
// ResolveExplosive resolves an explosive against all targets.
//
// Precondition: grenade and all targets must not be nil; effectiveDC >= 1.
// Postcondition: each target makes a Hustle save vs effectiveDC;
// damage scaled by save outcome (crit success: 0, success: half, failure: full, crit failure: double).
func ResolveExplosive(grenade *inventory.ExplosiveDef, targets []*Combatant, effectiveDC int, src Source) []ExplosiveResult
```

**All existing call sites of `ResolveExplosive` MUST be updated.** The implementer must run `grep -rn "ResolveExplosive" ./` to locate every call site and pass `grenade.SaveDC` as `effectiveDC` (preserving existing behavior). At time of writing, the only call site is `resolveThrow` in `round.go:960`. `resolveThrow` itself passes `grenade.SaveDC + actor.Level`.

### Narrative events

`resolveThrow` already emits per-target `RoundEvent` entries with save result and damage. No changes needed to narrative format.

---

## Feature 3: Content Updates

The three existing explosive YAMLs in `content/explosives/` receive:
- `friendly_fire: false` (explicit, matches default)
- `aoe_radius: 0` (explicit, matches default — room-wide)

No functional change; values are made explicit for clarity.

---

## Testing

- **REQ-T1**: `ResolveExplosive` called with `effectiveDC = grenade.SaveDC + 3` and a fixed die source that forces a specific roll: verify `SaveResult` matches `OutcomeFor(roll + targetMod, effectiveDC)` (not `OutcomeFor(roll + targetMod, grenade.SaveDC)`).
- **REQ-T2**: `explosiveTargetsOf` with `friendly_fire: false`: enemy-kind combatants included; ally-kind combatants excluded; actor excluded.
- **REQ-T3**: `explosiveTargetsOf` with `friendly_fire: true`: ally-kind combatants included; actor still excluded.
- **REQ-T4**: `explosiveTargetsOf` with actor as only living combatant returns empty slice regardless of `friendly_fire`.
- **REQ-T5**: `explosiveTargetsOf` with all non-actor combatants dead returns empty slice (no panic).
- **REQ-T6** (property): For any `actor.Level` in [1, 20], `resolveThrow` passes `grenade.SaveDC + actor.Level` to `ResolveExplosive`; verified via a test-double `src` that records the DC used.
- **REQ-T7**: Existing `TestResolveExplosive_*` tests remain green after signature change (pass `grenade.SaveDC` as `effectiveDC`).
- **REQ-T8**: `resolveThrow` integration — two enemies + one ally in combat; `friendly_fire: false` → ally not in event narrative; `friendly_fire: true` → ally name appears in event narrative.
