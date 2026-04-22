# Duplicate Effects Handling

**Issue:** [#245 Feature: Duplicate effects handling](https://github.com/cory-johannsen/mud/issues/245)
**Status:** Spec
**Date:** 2026-04-21

## 1. Summary

Introduce a unified typed-bonus pipeline that governs how duplicate effects stack and reconcile across the combat system. Condition bonuses, feat/tech passives, and equipment bonuses all flow through a single `effect.EffectSet` per combatant. A PF2E-style taxonomy — `status`, `circumstance`, `item`, `untyped` — determines which bonuses stack and which are suppressed. Same-source re-applies dedup on `(bearer_uid, source_id, caster_uid)`. Different sources coexist as separate effects and are reconciled at read time by the typed-bonus rule.

Players see every applied effect on their character sheet, each annotated `(active)` or `(overridden by <winner>)` so it is always clear why a bonus does or does not contribute.

## 2. Requirements

- DEDUP-1: Effects MUST be deduplicated on apply by the composite key `(bearer_uid, source_id, caster_uid)`; re-applying with the same key MUST overwrite the prior instance's bonuses and duration.
- DEDUP-2: Bonuses MUST be typed as one of `status`, `circumstance`, `item`, or `untyped`; missing type MUST default to `untyped` at load time.
- DEDUP-3: Within each of `status`, `circumstance`, and `item`, only the single highest positive bonus MUST contribute to the total for a given stat; all others MUST be marked suppressed.
- DEDUP-4: Within each of `status`, `circumstance`, and `item`, only the single most-negative penalty MUST contribute to the total for a given stat; all others MUST be marked suppressed.
- DEDUP-5: `untyped` bonuses and penalties MUST always stack additively.
- DEDUP-6: When two same-type bonuses tie on absolute value, the winner MUST be determined by lexicographic ordering of `(SourceID, CasterUID)` ascending.
- DEDUP-7: `Resolve(set, stat)` MUST be pure: two calls with an unchanged `EffectSet` MUST return identical results.
- DEDUP-8: `EffectSet` MUST increment a monotonic version counter on every mutation.
- DEDUP-9: An effect with multiple `Bonus` entries targeting the same stat MUST be treated as multiple contributions — one per entry — each subject to the typed-bonus rule independently.
- DEDUP-10: Condition, feat, tech, and equipment bonuses MUST flow through the same `EffectSet` pipeline; no subsystem MAY compute bonus totals outside `effect.Resolve`.
- DEDUP-11: Legacy flat-field bonuses in `ConditionDef` (e.g. `attack_bonus`) MUST be synthesised into untyped `Bonus` entries at load time; mixing flat fields and `bonuses:` in the same def MUST be a load error.
- DEDUP-12: Effects applied by a caster MUST persist after the caster exits combat unless the effect declares `linked_to_caster: true`, in which case they MUST be removed when the caster exits.
- DEDUP-13: The character sheet UI MUST display all applied effects with an `(active)` or `(overridden by X)` annotation per bonus.
- DEDUP-14: A narrative event MUST be emitted when an effect transitions between `active` and `overridden` as a result of a new application or removal.
- DEDUP-15: A bonus of value 0 MUST be rejected at YAML load time.
- DEDUP-16: Stat matching MUST support prefix-on-colon inheritance: a bonus to `skill` MUST contribute to queries for `skill:<id>`; a bonus to `skill:stealth` MUST NOT contribute to queries for `skill:savvy`.

## 3. Architecture

### 3.1 Package layout

New package `internal/game/effect/` owns the model:

- `Bonus` — a single typed numeric contribution to one stat.
- `Effect` — a named bundle of bonuses tied to `(SourceID, CasterUID)`.
- `EffectSet` — per-bearer collection with dedup and version counter.
- `Resolve(set, stat)` — pure function projecting the set to a `Resolved` view per stat.
- `Provider` — interface for any subsystem (feats, techs, equipment) that contributes effects to a combatant at creation time.

Condition-derived effects are owned by the existing `condition.ActiveSet`, which acquires responsibility for maintaining an internal `EffectSet` in lock-step with condition state. Callers MUST NOT mutate that `EffectSet` directly; they go through `ActiveSet.Apply` / `Remove` / `Tick` as today.

Non-condition effects (feat/tech passives, equipment bonuses) are applied directly to `combat.Combatant.Effects *effect.EffectSet` by each provider.

### 3.2 Bonus types and stacking rules

Types: `status`, `circumstance`, `item`, `untyped`.

- Within each of `status`, `circumstance`, `item`:
  - Positive bonuses: only the highest single value contributes.
  - Negative penalties: only the most negative single value contributes.
- `untyped` positive and negative values both always stack.
- Bonuses and penalties are reconciled separately within each typed bucket, then summed across buckets.

### 3.3 Dedup key

`(bearer_uid, source_id, caster_uid)`.

`caster_uid` may be empty for sources without an originator entity (equipped items, ambient effects, self-granted feat passives); the key degenerates to `(bearer_uid, source_id)` in that case. Applying the same key twice overwrites the earlier effect.

Two *different* casters granting the same `source_id` coexist as separate `Effect` entries and are reconciled by the typed-bonus rule at read time — this is expected and correct.

### 3.4 Persistence

No database schema change. Condition-derived effects are persisted via the existing condition system; non-condition effects are rebuilt each session from feat/tech/equipment state (stateless, like reaction budgets in #244).

### 3.5 Performance

`Resolve` recomputes from scratch on each call. `EffectSet.Version()` increments on every mutation. v1 ships without caching; call sites that profile as hot may later key a cache on `(version, stat)`. The existing `condition.AttackBonus`-style wrappers retain their signatures, so call-site churn in the combat resolver is zero.

## 4. Data Model

### 4.1 Bonus type enum

```go
type BonusType string

const (
    BonusTypeStatus       BonusType = "status"
    BonusTypeCircumstance BonusType = "circumstance"
    BonusTypeItem         BonusType = "item"
    BonusTypeUntyped      BonusType = "untyped"
)
```

### 4.2 Stat enum

```go
type Stat string

const (
    // Combat surfaces
    StatAttack    Stat = "attack"
    StatAC        Stat = "ac"
    StatDamage    Stat = "damage"
    StatSpeed     Stat = "speed"

    // Gunchete ability scores
    StatBrutality Stat = "brutality"
    StatGrit      Stat = "grit"
    StatQuickness Stat = "quickness"
    StatReasoning Stat = "reasoning"
    StatSavvy     Stat = "savvy"
    StatFlair     Stat = "flair"

    // Generic skill (applies to all skill checks); per-skill uses "skill:<id>"
    StatSkill Stat = "skill"
)
```

Unknown stat strings are legal — they just never match any `Resolve` query. This is the back-compat hatch for vestigial PF2E stats (`reflex`, `fortitude`, `will`) in existing content; the follow-on content-migration ticket sweeps them out.

Stats may be registered dynamically via `effect.RegisterStat(stat Stat)` from dependent packages so `effect` stays dependency-free.

### 4.3 Bonus

```go
type Bonus struct {
    Stat  Stat      `yaml:"stat"`
    Value int       `yaml:"value"` // positive = bonus, negative = penalty
    Type  BonusType `yaml:"type"`  // default "untyped"
}
```

A `Value` of 0 is a YAML load error.

### 4.4 Effect

```go
type Effect struct {
    EffectID       string
    SourceID       string
    CasterUID      string
    Bonuses        []Bonus
    DurKind        DurationKind
    DurRemain      int
    ExpiresAt      *time.Time
    Annotation     string
    LinkedToCaster bool
}

type DurationKind string

const (
    DurationRounds      DurationKind = "rounds"
    DurationUntilRemove DurationKind = "until_remove"
    DurationPermanent   DurationKind = "permanent"
    DurationEncounter   DurationKind = "encounter"
    DurationCalendar    DurationKind = "calendar"
)
```

### 4.5 EffectSet

```go
type EffectSet struct { /* unexported */ }

func NewEffectSet() *EffectSet
func (s *EffectSet) Apply(e Effect)
func (s *EffectSet) Remove(key effectKey)
func (s *EffectSet) RemoveBySource(sourceID string)
func (s *EffectSet) RemoveByCaster(casterUID string) // filters to LinkedToCaster == true
func (s *EffectSet) Tick() []effectKey
func (s *EffectSet) TickCalendar(now time.Time) []effectKey
func (s *EffectSet) ClearEncounter()
func (s *EffectSet) All() []Effect
func (s *EffectSet) Version() uint64
```

A nil `EffectSet` is legal; every method is a safe no-op / zero-value return.

### 4.6 Resolved view

```go
type Resolved struct {
    Stat         Stat
    Total        int
    Contributing []Contribution
    Suppressed   []Contribution
}

type Contribution struct {
    EffectID    string
    SourceID    string
    CasterUID   string
    BonusType   BonusType
    Value       int
    OverriddenBy *ContributionRef // non-nil only in Suppressed
}

type ContributionRef struct {
    EffectID  string
    SourceID  string
    CasterUID string
}
```

## 5. Algorithm

### 5.1 Resolve

```
Resolve(set, stat):
  matches = []Contribution
  for each effect in set:
    for each bonus in effect.Bonuses:
      if bonus.Stat matches stat:         # prefix-on-colon match, §5.2
        matches.append({effect.ids, bonus.Type, bonus.Value})

  if matches is empty:
    return Resolved{Stat: stat, Total: 0}

  buckets := split matches by (type, sign)
  total := 0
  contributing, suppressed := [], []

  for each type in [status, circumstance, item]:
    # Bonuses: highest single wins
    pick winner := max(bucket.bonuses, by: Value, tiebreak: lex(SourceID, CasterUID))
    if winner exists:
      total += winner.Value
      contributing.append(winner)
      for loser in bucket.bonuses \ {winner}:
        suppressed.append(loser with OverriddenBy = ref(winner))

    # Penalties: worst single wins
    pick winner := min(bucket.penalties, by: Value, tiebreak: lex(SourceID, CasterUID))
    if winner exists:
      total += winner.Value
      contributing.append(winner)
      for loser in bucket.penalties \ {winner}:
        suppressed.append(loser with OverriddenBy = ref(winner))

  # Untyped: always stack
  for each m in buckets[untyped].bonuses + buckets[untyped].penalties:
    total += m.Value
    contributing.append(m)

  return Resolved{stat, total, contributing, suppressed}
```

### 5.2 Stat matching (prefix-on-colon)

A queried stat inherits contributions from itself and from each of its colon-ancestor forms. `skill:stealth` inherits from `skill`; `skill` inherits from nothing. `skill:stealth` does NOT inherit from `skill:savvy`. This makes the existing generic-skill-penalty and per-skill-bonus behaviors expressible without special-casing.

### 5.3 Determinism

Within each bucket, ties on value are broken by lexicographic `(SourceID, CasterUID)` ascending. `Resolve` is pure: identical inputs yield identical `Resolved` values, independent of map iteration order.

### 5.4 Stacks

Stack count (e.g. frightened 2) is flattened into `Bonus.Value` at the moment the `ActiveSet` applies the effect. `Resolve` never multiplies by stacks. Stack updates re-apply the effect with adjusted values under the same dedup key.

## 6. Integration

### 6.1 Conditions (backward-compatible)

- `ConditionDef` gains an optional `bonuses: []Bonus` list.
- When present, `bonuses` is authoritative; the flat fields (`attack_bonus`, `ac_penalty`, …) MUST NOT also appear on the same def (load error).
- When absent, flat fields synthesise an equivalent list of `type: untyped` bonuses at load time.
- `ActiveSet.Apply`/`ApplyTagged`/`Remove`/`Tick` maintain a paired `EffectSet`. On apply, build `Effect{EffectID: def.ID, SourceID: "condition:" + def.ID, CasterUID: source, Bonuses: def.Bonuses * stackCount}` and dedup-insert. On stack change, re-apply with new values. On remove/tick, remove.
- `ActiveSet.Effects() *effect.EffectSet` exposes the merged view; callers MUST NOT mutate directly.
- `condition/modifiers.go` is rewritten: each named function becomes a thin wrapper over `effect.Resolve`. For legacy-untyped content the return value is identical to today.

### 6.2 Feats and techs

- `FeatDef` and `TechnologyDef` gain optional `passive_bonuses: []Bonus` (identical schema).
- At combat entry, a provider-style registration gathers effects from the combatant's active feats and techs and applies them to the combatant's `EffectSet`. `SourceID = "feat:<id>"` or `"tech:<id>"`, `CasterUID = bearerUID`, `DurKind = DurationUntilRemove`.

### 6.3 Equipment

- Equipped items produce effects via a provider. `SourceID = "item:<instance_id>"`, `CasterUID = ""`, `DurKind = DurationUntilRemove`.
- On equip, apply; on unequip, remove by source-id.
- Existing ad-hoc aggregations (e.g. `weaponModifierDamageBonus`) become thin wrappers over `effect.Resolve` filtered to item-type contributions, so observable behavior is unchanged while the data flows through the unified pipeline.
- v1 scope: migrate the equipment bonus call sites already summed into attack/AC/damage totals by the combat resolver (roughly the 10 call sites identified in `internal/game/combat/round.go`). Non-combat equipment effects (e.g. per-skill bonuses from accessories) are migrated opportunistically during implementation; the long tail is tracked by the follow-on content-migration ticket.

### 6.4 Combatant wiring

- `combat.Combatant` gains `Effects *effect.EffectSet`.
- Populated at combatant creation by merging:
  1. `Conditions.Effects()` from the `condition.ActiveSet`.
  2. Feat/tech/item `Provider` outputs applied directly.
- Combat resolver reads switch from `condition.AttackBonus(cbt.Conditions[actor.ID])` to `effect.Resolve(actor.Effects, effect.StatAttack).Total`. The `modifiers.go` rewrite (6.1) means this is a mechanical find/replace with zero behavioral change for unmigrated content.

### 6.5 Caster departure

Effects remain on the bearer when the caster exits combat unless `LinkedToCaster: true`. `EffectSet.RemoveByCaster(casterUID)` filters on `LinkedToCaster` and is the only removal path tied to caster presence.

## 7. UI / UX

### 7.1 Telnet — effects block

```
Effects:
  Heroism           (from Kira)      attack +1 status        (active)
                                     grit +1 status          (active)
  Inspire Courage   (from Xin)       attack +1 status        (overridden by Heroism)
  +1 Sword          (item)           damage +1 item          (active)
  Frightened 2      (self)           attack -2 status        (active)
```

Rules:
- One line per effect; bonuses indented as sub-lines when an effect has multiple.
- "from {name}" when `CasterUID != ""` and caster is known; otherwise a bucket label (`item`, `feat`, `tech`, `condition`, `self`).
- `(active)` in green, `(overridden by X)` in yellow.

New telnet commands:
- `effects` — print the block.
- `effects detail <effect_id>` — per-stat breakdown for one effect (contributing + overridden).

### 7.2 Web — effects panel

- A new "Effects" panel on the character sheet UI, replacing the existing conditions list.
- Each row: icon, name, caster, bonus summary, active/overridden chip.
- Hover / click → inline expansion with each `Bonus`, its type badge (`STATUS` / `CIRCUMSTANCE` / `ITEM` / `UNTYPED`), and its contribution state.
- Keyboard shortcut / button toggles a per-stat "breakdown" view listing winning bonuses, totals, and collapsible overridden bonuses.

### 7.3 Combat log narrative

```
[EFFECT] Kira casts Heroism on you (+1 status to attack).
[EFFECT] Heroism fades.
[EFFECT] Inspire Courage from Xin is overridden by Heroism.
```

Override narration fires only when a newly-applied or removed effect causes an existing effect to transition between `active` and `overridden` (computed by diffing pre-apply and post-apply `Resolve` per affected stat). This scopes log output to user-visible changes.

### 7.4 Tooltips on rolled numbers (web only)

Hovering a to-hit or AC value in a combat message shows its contributing stack (e.g. `+6 attack = +3 base, +2 status (Heroism), +1 item (+1 sword)`). Overridden contributions are not shown in tooltips — those belong on the effects panel.

### 7.5 Non-goals for v1 UI

- No in-UI tooling to force-override tie-breaks (tie-break is deterministic per 5.3).
- No filter/sort controls on the effects panel.
- No automated "redundant buff" warnings.
- No new icon art beyond reusing existing shield / sword / potion icons.

## 8. Data Model Changes

### 8.1 YAML shape

```yaml
# ConditionDef / FeatDef / TechnologyDef
bonuses:
  - stat: attack
    value: 2
    type: status
  - stat: ac
    value: -1
    type: circumstance
```

Legacy flat fields (`attack_bonus`, `ac_penalty`, `damage_bonus`, `reflex_bonus`, `stealth_bonus`, `skill_penalty`, `skill_penalties`, `flair_bonus`) remain accepted and synthesise untyped entries. Mixing flat fields and `bonuses:` in the same file is a load error.

### 8.2 Persistence

No database schema change. All typed-bonus data is round-scoped or session-scoped and reconstructed from static content + live condition state.

## 9. Testing

Per SWENG-5 / SWENG-5a, TDD with property-based tests where appropriate.

- `internal/game/effect/bonus_test.go` — table tests: bonus type validation, value-zero rejection, stat string validation.
- `internal/game/effect/set_test.go` — property tests: apply/remove/tick sequences; dedup key invariants; version counter monotonicity; `RemoveByCaster` filter correctness with `LinkedToCaster`.
- `internal/game/effect/resolve_test.go` — property tests:
  - Random-mix correctness: computed winner matches the declarative rule (highest by type, worst penalty by type, untyped all-stack).
  - Tie-break determinism across shuffled input orderings.
  - Prefix-on-colon propagation: `skill` contributes to every `skill:<id>` query.
- `internal/game/effect/resolve_scenario_test.go` — scripted PF2E-style scenarios (two same-type status bonuses, stacking circumstance penalties, untyped additivity) as regression anchors.
- `internal/game/condition/modifiers_compat_test.go` — golden test: for a suite of `ActiveSet` states built from legacy flat-field condition YAML, the rewritten `modifiers.go` functions return identical values to the current implementation.
- `internal/game/combat/round_effect_test.go` — end-to-end: conditioned combatants resolve correctly; equipped items contribute via the effect pipeline; feat passives contribute; mid-combat effect apply produces the narrative override event.
- Frontend: telnet formatting table-tests; webclient component tests for the effects panel chips and the inline breakdown expansion.

## 10. Documentation

- `docs/architecture/combat.md` — add a "Bonus stacking" section hosting the `DEDUP-N` requirements and the `Resolve` algorithm pseudocode.
- `docs/architecture/CHARACTERS.md` — update to describe `effect.EffectSet` on `Combatant` and the provider pattern.
- Content-authoring documentation — new section covering the `bonuses:` YAML shape, bonus types, and the legacy-field fallback.

## 11. Non-Goals (v1)

- No caching layer on `Resolve` — every read recomputes (version counter in place for future caching).
- No automatic content migration — existing condition YAML keeps flat fields and untyped-stacking behavior. Follow-on content-migration ticket tracks the sweep.
- No new bonus types beyond PF2E's four.
- No conditional bonuses ("+1 attack if wielding a sword") — separate feature.
- No formula / expression support in bonus values — integers only.
- No player-facing tooling to force different tie-break behavior.
- No "redundant buff" warnings.
- No cross-bearer effect visibility beyond the existing combat log narrative.

## 12. Open Questions for the Planner

- Exact file locations for the new `internal/game/effect/` package.
- Whether `effect.Provider` registrations are pull-based (engine queries at combatant creation) or push-based (subsystems register at init time).
- Where equipment-driven effect construction lives: `internal/inventory/`, combat, or a new `internal/equipment/effects.go`.
- Telnet formatting specifics (column widths, color codes) — needs alignment with existing screen regions.

## 13. Follow-On Work

A separate ticket MUST be created alongside this spec to track:

- Sweep every existing `ConditionDef` / `FeatDef` / `TechnologyDef` YAML file to migrate flat-field bonuses into the `bonuses:` list, assigning correct `status` / `circumstance` / `item` / `untyped` types per PF2E conventions.
- Remove vestigial PF2E save fields (`reflex_bonus`, and any analogous fort/will references) from definitions, since Gunchete uses the six ability scores (Brutality / Grit / Quickness / Reasoning / Savvy / Flair) instead of PF2E's Reflex/Fortitude/Will saves. `ReflexBonus` in `ConditionDef` is not consumed outside the condition package itself — it is harmless ballast that should be removed as part of the sweep.
- Audit equipment/item bonus providers for correct bonus typing (weapon potency → `item`, armor → `item`, cover → `circumstance`, etc.).
- Once migration completes, tighten the loader: mixing legacy flat fields and `bonuses:` becomes an immediate error (already enforced by DEDUP-11); the eventual goal is to remove legacy flat-field support entirely.
