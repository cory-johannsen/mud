# Stacking Multipliers

**Issue:** [#246 Feature: Stacking multipliers](https://github.com/cory-johannsen/mud/issues/246)
**Status:** Spec
**Date:** 2026-04-21

## 1. Summary

Introduce an explicit damage resolution pipeline with named stages, replacing the current ad-hoc sequence of `EffectiveDamage()` + flat adds + open-coded crit `×2` + weakness/resistance scattered across the combat resolver. The pipeline enforces PF2E's multiplier-stacking rule: when more than one multiplier applies, extras are added as `+1` rather than multiplied together. Halving is a separate staged flag that applies after multiplication. A stage-by-stage breakdown is surfaced alongside each damage event in the combat narrative.

## 2. Requirements

- MULT-1: The damage pipeline MUST execute stages in the order: base → multiplier → halver → weakness → resistance → floor.
- MULT-2: Multiple positive multipliers MUST combine as `effective = 1 + Σ(M_i − 1)`; multiplication MUST NOT be chained.
- MULT-3: A single halver MUST floor-divide the post-multiplier damage by 2; additional halvers on the same event MUST NOT stack.
- MULT-4: Halvers MUST be applied after multipliers.
- MULT-5: Weakness MUST be applied after halvers, as a flat additive keyed on damage type.
- MULT-6: Resistance MUST be applied after weakness, as a flat subtractive keyed on damage type.
- MULT-7: The final damage value MUST be floored at 0.
- MULT-8: A `DamageMultiplier` MUST have `Factor > 1.0`; factors `≤ 1.0` MUST be rejected as programmer error.
- MULT-9: A tech effect with `Multiplier == 0.5` MUST be interpreted as a halver, not a numeric multiplier.
- MULT-10: A tech effect with `Multiplier` in `(0, 1)` and not equal to `0.5` MUST raise a load-time error.
- MULT-11: `ResolveDamage` MUST be pure — no RNG, no mutation, no I/O; given the same input it MUST return the same output.
- MULT-12: Every damage-producing call site in `internal/game/combat/round.go` MUST build its input exclusively via `buildDamageInput` and resolve exclusively via `ResolveDamage`; open-coded crit `×2`, weakness, or resistance arithmetic MUST be removed.
- MULT-13: `DamageResult.Breakdown` MUST contain exactly one entry per non-trivial stage in canonical order; `StageBase` MUST always be present.
- MULT-14: When any stage beyond `StageBase` contributes, the inline breakdown line MUST be appended to the combat narrative; otherwise no breakdown line is emitted.
- MULT-15: A player with `show_damage_breakdown` enabled MUST receive the verbose block in addition to the inline line for damage events they deal or receive.
- MULT-16: Reaction callbacks listening on `TriggerOnDamageTaken` MUST continue to see and mutate the final post-pipeline damage scalar; they MUST NOT receive the breakdown in v1.
- MULT-17: The `AttackResult.EffectiveDamage()` method MUST be marked deprecated but retained for external callers; in-package callers MUST migrate to `ResolveDamage`.

## 3. Pipeline

### 3.1 Stages

1. **Base accumulation.** Sum raw damage dice, weapon modifiers, ability-score mods, flat condition bonuses, extra weapon dice, passive-feat adds, and any other flat additive contributor.
2. **Multiplier combination.** Collect every `×M` source with `M > 1`. Combine via `effective = 1 + Σ(M_i − 1)`. Apply `after_mult = base × effective`.
3. **Halver application.** If any halver is present, `after_halve = floor(after_mult / 2)`. Multiple halvers collapse to a single halving.
4. **Weakness.** If the target has a flat weakness to the attack's damage type, `after_weak = after_halve + weakness_flat`.
5. **Resistance.** If the target has a flat resistance to the attack's damage type, `after_res = max(0, after_weak − resistance_flat)`.
6. **Floor.** `final = max(0, after_res)`.

### 3.2 Halver semantics

- A halver is a boolean flag in effect, not a numeric factor.
- Multiple halvers on the same damage event are a no-op beyond the first (per PF2E "half damage" wording).
- Crit (×2 multiplier) plus halver (basic-save success) cancels to net ×1, matching PF2E's intended interaction.
- The existing tech `Multiplier float64` field is interpreted as follows:
  - `== 0.5`: converted to a halver.
  - `> 1.0`: fed into the multiplier bucket with `Factor = Multiplier`.
  - In `(0, 1)` and not `0.5`: load-time error (no PF2E construct for arbitrary fractional multipliers).
  - `== 1.0` or unset: no multiplier contribution.

### 3.3 Ordering rationale

- Multiply, then halve, then type-adjust — matches PF2E rules text.
- Weakness before resistance so resistance's floor-at-0 behavior applies against the weakness-boosted value.
- Floor at the end catches any negative intermediate.

### 3.4 Non-stage semantics

- No support for non-damage-type-bound multipliers (e.g. "all damage you deal is doubled") in v1.
- No per-die multipliers — those are expressed upstream as extra dice, not as multipliers.
- Persistent damage, heal, and drain (tech effect types) stay on their current paths; only direct damage flows through this pipeline.

## 4. API

New file `internal/game/combat/damage.go` owns the types and the core function.

### 4.1 Input

```go
type DamageInput struct {
    Additives   []DamageAdditive   // flat contributors summed into base
    Multipliers []DamageMultiplier // each Factor > 1.0
    Halvers     []DamageHalver     // flag-like; any present triggers halving
    DamageType  string             // may be empty (untyped)
    Weakness    int                // pre-resolved flat weakness for DamageType; 0 if none
    Resistance  int                // pre-resolved flat resistance for DamageType; 0 if none
}

type DamageAdditive struct {
    Label  string
    Value  int     // may be negative
    Source string  // SourceID-style tag
}

type DamageMultiplier struct {
    Label  string
    Factor float64 // > 1.0
    Source string
}

type DamageHalver struct {
    Label  string
    Source string
}
```

### 4.2 Output

```go
type DamageResult struct {
    Final     int
    Breakdown []DamageBreakdownStep
}

type DamageBreakdownStep struct {
    Stage   DamageStage
    Before  int
    Delta   int    // signed change produced by this stage
    After   int
    Detail  string
    Sources []string
}

type DamageStage string

const (
    StageBase       DamageStage = "base"
    StageMultiplier DamageStage = "multiplier"
    StageHalver     DamageStage = "halver"
    StageWeakness   DamageStage = "weakness"
    StageResistance DamageStage = "resistance"
    StageFloor      DamageStage = "floor"
)
```

### 4.3 Core function

```go
// ResolveDamage runs the §3 pipeline on input and returns final damage and
// an ordered breakdown.
//
// Precondition: every Multipliers[i].Factor > 1.0.
// Postcondition: Final >= 0; Breakdown contains one entry per non-empty stage
// in canonical order (Base, Multiplier, Halver, Weakness, Resistance, Floor).
func ResolveDamage(in DamageInput) DamageResult
```

Pure: no RNG, no state mutation, no I/O.

### 4.4 Validation

- `Factor > 1.0` for every multiplier — caller error otherwise (panic in production; return sentinel in test helpers).
- `Additive.Value` may be negative (e.g. pre-pipeline condition damage penalty); additives sum into base before any multiplication.
- `DamageType` may be empty; weakness/resistance ints are zeroed by the caller when type is empty.

### 4.5 Deprecation

`AttackResult.EffectiveDamage()` is retained for external callers and marked deprecated. In-package damage sites migrate to `ResolveDamage`.

## 5. Integration

Every damage-producing call site in `internal/game/combat/round.go` is updated to go through a single `buildDamageInput` helper and then `ResolveDamage`. The v1 call sites are:

- Single-attack resolution.
- MAP second attack.
- Burst fire (per target).
- Automatic fire (per target).
- Throw / explosive.
- Tech direct-damage.

For each site `buildDamageInput(cbt, actor, target, attackResult, src) DamageInput` gathers:

- Dice + ability mod + weapon bonus → `DamageAdditive` entries.
- `condition.DamageBonus(...)` → `DamageAdditive` entry with `Source: "condition:*"`.
- Extra weapon dice (rolled by the caller using `src`) → a single `DamageAdditive` entry labelled per feat source.
- Passive feat adds → `DamageAdditive` entries.
- Crit: a `DamageMultiplier{Factor: 2.0, Label: "critical hit", Source: "engine:crit"}` when `AttackResult.Outcome == CritSuccess`.
- Tech halver on save success: `DamageHalver{Label: "basic save success", Source: "tech:<id>"}`.
- Weakness / resistance flat values resolved once from target.

The extra-weapon-dice crit-doubling math currently open-coded at `round.go:801` is removed — the extra dice become a `DamageAdditive`, and crit doubling happens via the multiplier stage on the whole base. This changes observable behavior for extra-weapon-dice crits only insofar as it routes through the canonical path; the computed value is the same (base × 2 includes the extra dice).

The reaction callback path (`TriggerOnDamageTaken` with `DamagePending *int`) continues to mutate the final post-pipeline scalar before `ApplyDamage`. Reactions do not see the breakdown in v1.

## 6. Output

### 6.1 Inline breakdown

Appended as a trailing line to the combat narrative only when `len(Breakdown) > 1` (i.e. anything non-trivial happened past `StageBase`).

```
Kira slashes the thug for 14 damage.
  (base 7 → ×3 [crit, vulnerable] = 21 → weakness +3 = 24 → resistance -10 = 14)

Kira slashes the thug for 6 damage.
  (base 6 — no modifiers)

Kira throws a grenade at the thug for 4 damage.
  (base 8 → halved [basic save success] = 4)
```

Rendering rules:

- Each stage step renders as `→ {delta-or-descriptor} = {after}`.
- Multiplier step renders the effective factor and a bracketed list of source labels so players can see why the effective multiplier is `×3` rather than `×4`.
- Weakness / resistance render as `→ weakness +{n} = …` / `→ resistance -{n} = …`.
- Halver renders as `→ halved [{reason}] = …`.
- Floor renders as `→ floored = 0` only when the pre-floor value would have been `< 0`.

### 6.2 Verbose breakdown (opt-in)

When the player has `show_damage_breakdown` enabled, a multi-line block is appended after the narrative:

```
Kira slashes the thug for 14 damage.
  base:       1d8 (5) + STR mod (+2) + weapon +1 sword = 8
              +2 flanking (circumstance) = 10
  multiplier: ×3 [critical hit (×2), vulnerable fire (×2)]
  after:      21
  halver:     none
  weakness:   +3 (fire)
  after:      24
  resistance: -10 (physical)
  final:      14
```

Each additive renders `label: value` on its own line; the multiplier line shows the combined factor and contributing source labels; halver/weakness/resistance each render on a single line. The verbose block is emitted only to the enabling observer — not to every combatant in the room.

### 6.3 Web client

- Inline form: muted trailing text under the primary damage line. No new WS message type — the breakdown is serialised into the existing damage event payload and the client decides whether to show it based on the user's pref.
- Verbose form: an "i" icon next to the damage number; click expands to reveal the full breakdown inline.

### 6.4 Telnet width handling

If the inline breakdown would exceed the console region width, the line wraps at `→` boundaries rather than mid-descriptor. A helper in `text_renderer.go` handles this (mirrors the existing width-wrapping for room descriptions).

### 6.5 Non-goals for output

- No per-combatant breakdown display preference (verbose is a global per-player pref).
- No color coding of multiplier vs weakness vs resistance beyond existing combat narrative styling.
- No interactive per-stage toggling.
- No separate "damage log" panel.

## 7. Testing

Per SWENG-5 / SWENG-5a, TDD with property-based tests where appropriate.

- `internal/game/combat/damage_test.go` — property-based:
  - For any positive multiplier set `{M_i}` with each `M_i > 1`, `ResolveDamage` yields `base × (1 + Σ(M_i − 1))` pre-halver.
  - Halver stage reduces damage by exactly `floor(after_mult / 2)` and is idempotent under multiplicity.
  - Weakness increases and resistance decreases final damage by their flat values, always in that order, with floor at 0.
  - Stage ordering invariant: damage pre-crit < damage post-crit for any non-trivial inputs.
- `internal/game/combat/damage_scenario_test.go` — PF2E regression anchors:
  - Crit alone: ×2.
  - Crit + vulnerable (both ×2): ×3.
  - Crit + vulnerable + hypothetical third ×2: ×4.
  - Crit + basic save success halver: net ×1.
  - Weakness 5 + resistance 10 + base 12: `12 + 5 − 10 = 7`.
  - Resistance greater than total: floor at 0.
  - Empty multipliers / halvers: base passthrough.
- `internal/game/combat/round_damage_pipeline_test.go` — end-to-end: every damage-producing call site routes through `ResolveDamage`; breakdown event payload shape verified for each site.
- `internal/game/combat/round_breakdown_narrative_test.go` — inline line emitted only when non-trivial; verbose block emitted only when pref enabled; width-wrapping at `→` boundaries.
- `internal/game/technology/tech_multiplier_load_test.go` — `Multiplier == 0.5` parses as halver; `0.3` rejected; `2.0` parses as multiplier.

## 8. Documentation

- `docs/architecture/combat.md` — new "Damage pipeline" section hosting the `MULT-N` requirements and a stage-ordered pseudocode diagram.
- Content-authoring documentation — clarify that tech `multiplier: 0.5` is the only legal fractional value and note the forthcoming `multiplier_factor:` authoring shape for doubling sources (deferred until such content is authored).

## 9. Non-Goals (v1)

- No new declarative multiplier YAML surface for feats/conditions. Current content has no multiplier-granting surface; when it does, a follow-up ticket adds a `multipliers:` list analogous to #245's `bonuses:` list. v1 multiplier sources are crit (engine-internal) and the existing tech `Multiplier` field.
- No per-die multipliers.
- No breakdown propagation through the reaction callback — reactions still mutate the final scalar.
- No changes to persistent-damage, heal, or drain paths.
- No combatant-facing breakdown controls beyond the single `show_damage_breakdown` pref.
- No breakdown history log or replay.

## 10. Open Questions for the Planner

- Exact location and shape of `buildDamageInput` — inline helper in `round.go` or a new `internal/game/combat/damage_input.go`.
- Whether the web client renders the verbose breakdown via an expandable "i" icon or always-on muted text; depends on existing combat-log UI conventions.
- Whether `show_damage_breakdown` is persisted on the character or on the session — match whatever the existing prefs system does.
- Whether the verbose block is emitted once per damage event or once per breakdown-enabled observer.
