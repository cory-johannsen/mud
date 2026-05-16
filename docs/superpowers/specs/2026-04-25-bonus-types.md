---
title: Circumstance, Item, and Status Bonuses and Penalties
issue: https://github.com/cory-johannsen/mud/issues/259
date: 2026-04-25
status: spec
prefix: BTYPE
depends_on:
  - "#245 Duplicate effects handling (DEDUP-* — typed bonus pipeline)"
related:
  - "#247 Cover bonuses (uses circumstance type)"
  - "#252 Non-combat actions (off-guard etc. apply circumstance penalties)"
  - "#254 Detection states (off-guard via condition pipeline)"
  - "#265 Content migration to typed bonuses (sweep)"
```

# Circumstance, Item, and Status Bonuses and Penalties

## 1. Summary

The PF2E bonus-type pipeline already exists. Spec #245 (Duplicate Effects Handling, merged via PR #274) shipped:

- `internal/game/effect/bonus.go` — `BonusType` enum (`status`, `circumstance`, `item`, `untyped`), `Bonus` struct, `Stat` enum.
- `internal/game/effect/set.go` — `EffectSet` with source-id deduplication, ticking, calendar expiry.
- `internal/game/effect/resolve.go` — pure `Resolve(set, stat) Resolved` returning total + per-bonus contributions, marking each as active or suppressed by type-based stacking rules.
- `internal/game/effect/render/render.go` — telnet `EffectsBlock` formatter showing each contributing bonus with type and source.
- `internal/game/combat/combatant_effects.go` — `BuildCombatantEffects` ingests conditions, feat passive bonuses, tech passive bonuses, and weapon bonuses (item-typed).
- `internal/game/combat/round.go` — attack and AC computations call `effect.Resolve(...)` for typed totals.

Issue #259 asks for the **full** type system "implemented", with stacking rules enforced, source tracking, and a UI breakdown showing each contributing bonus by type. Most of that is done. The remaining four gaps are concrete:

1. **Armor AC bonus** is still a legacy flat field on `Combatant` — not yet flowing through the typed-bonus pipeline.
2. **Web UI breakdown** is missing. Only telnet has a renderer. Players using the web client see flat numbers without provenance.
3. **Combat log override narrative.** `OverrideNarrativeEvents` exists in `combatant_effects.go:189-214` but the events are not yet routed into the combat log surface that players actually read in either UI.
4. **Cover bonuses** (#247) and **content migration** (#265) are sibling tickets that consume this pipeline; this spec coordinates the contract those tickets rely on but does not own their work.

This spec ships the four gaps and locks the cross-cutting contract so that #265's content sweep and #247's cover model land cleanly.

## 2. Goals & Non-Goals

### 2.1 Goals

- BTYPE-G1: Armor AC bonus flows through `EffectSet` with `BonusType = item`, replacing the legacy flat field on `Combatant.ACMod`.
- BTYPE-G2: Web client displays an Effects panel mirroring telnet's `EffectsBlock`, with per-stat breakdown tooltips on AC, attack, and saving-throw totals.
- BTYPE-G3: Combat-log narrative entries fire when an active bonus becomes suppressed (or vice versa) due to a higher-typed bonus being applied or removed.
- BTYPE-G4: Per-stat breakdown queryable via a single resolver call (`effect.Resolve(set, stat) Resolved`), with each `Contribution` carrying source label suitable for UI rendering.
- BTYPE-G5: Existing tests for the typed-bonus pipeline continue to pass; new tests cover armor migration and override-event emission.

### 2.2 Non-Goals

- BTYPE-NG1: Content migration of conditions/feats/techs/equipment from flat fields to `bonuses:` lists. That is #265's job.
- BTYPE-NG2: Cover bonus model. That is #247's job.
- BTYPE-NG3: New bonus types beyond the four PF2E types.
- BTYPE-NG4: Server-driven UI animations on bonus changes. Provenance display is data-only.
- BTYPE-NG5: Rebalancing baseline numbers. Migration preserves current effective totals exactly.
- BTYPE-NG6: A general "modifier authoring UI" in the admin tools. YAML / code remains the workflow.

## 3. Glossary

- **Typed bonus**: a `Bonus` whose `Type` is one of `status`, `circumstance`, `item`, `untyped`.
- **Stacking rule**: typed bonuses of the same type do not stack (highest applies); bonuses of different types stack; untyped always stacks.
- **Source**: the `(sourceID, casterUID)` pair carried by a `Bonus`, used for de-duplication of repeated applications from the same effect.
- **Resolved**: the output of `effect.Resolve(set, stat)` — a total plus per-contribution active/suppressed annotations.
- **Effects panel**: a per-character UI surface listing every `EffectSet` member with its bonuses and active/suppressed state.

## 4. Requirements

### 4.1 Armor AC Migration

- BTYPE-1: Equipped armor's AC bonus MUST be applied to `Combatant.Effects` as a `Bonus{Type: item, Stat: AC, Value: armor.ACBonus, SourceID: "item:" + armorID, CasterUID: combatant.UID}` during `BuildCombatantEffects`.
- BTYPE-2: The legacy `Combatant.ACMod` field MUST remain readable for one release cycle and MUST be set by `BuildCombatantEffects` to `effect.Resolve(set, StatAC).Total - Combatant.AC` so that any code path that has not yet migrated to `effect.Resolve` continues to compute the same total.
- BTYPE-3: All `effect.Resolve(set, StatAC)` call sites MUST treat the resolved total as authoritative; the legacy `ACMod` field MUST NOT be added on top.
- BTYPE-4: A new test in `internal/game/combat/combatant_effects_test.go` MUST verify that equipping armor with `+2` AC produces a `Resolved.Total` of base AC + 2 with one item-typed contribution.
- BTYPE-5: A property test under `internal/game/combat/testdata/rapid/TestArmorACBonus_Property/` MUST verify that swapping armor recomputes the resolved AC correctly across many random armor and condition combinations.

### 4.2 Web Effects Panel

- BTYPE-6: A new web component `EffectsPanel.tsx` MUST live under `cmd/webclient/ui/src/game/character/` and consume a new gRPC view message `EffectsView` returning the active `EffectSet` for the player's character.
- BTYPE-7: Each effect in the panel MUST display: name, source (caster name when not self), per-bonus rows showing `<stat> <±value> <type>`, and an active/suppressed badge per bonus.
- BTYPE-8: Suppressed bonus rows MUST show the suppressing effect's name in a tooltip ("overridden by Heroism").
- BTYPE-9: A per-stat breakdown tooltip MUST be wired on the AC, attack, and save totals rendered in the character sheet and combat panels. Hovering the total MUST show every contributing `Bonus` with its source and active/suppressed state.
- BTYPE-10: The breakdown tooltip MUST refresh on every relevant state change (round tick, condition apply/remove, equipment swap).
- BTYPE-11: Telnet equivalent commands MUST exist:
  - `effects` — list active effects (uses existing `EffectsBlock`).
  - `effects detail <stat>` — list per-stat contributions for one stat (`ac`, `attack`, `damage`, `<save>`).

### 4.3 Combat Log Narrative

- BTYPE-12: `combatant_effects.go:OverrideNarrativeEvents` MUST emit structured events to the combat log when a contribution transitions between active and suppressed. The event format: `[EFFECT] <effect-name> on <combatant>: <stat> <±value> <type> is now <suppressed by X | active>`.
- BTYPE-13: The events MUST be deduplicated within a single resolver-update tick — at most one event per `(effect, stat, transition)` per tick.
- BTYPE-14: The events MUST be visible in both telnet combat console and web combat log; the web client renders them with a distinct icon.
- BTYPE-15: Events MUST NOT be emitted for the initial application of an effect (first-time activation is shown in the existing apply narrative); only transitions emit override events.

### 4.4 Cross-Spec Contracts

- BTYPE-16: Cover bonuses (#247) MUST emit as `Bonus{Type: circumstance, Stat: AC, SourceID: "cover:" + coverID, CasterUID: target.UID}`. The contract is locked here; the cover spec implements it.
- BTYPE-17: Off-guard from any source (skill action #252, detection state #254, etc.) MUST emit as `Bonus{Type: circumstance, Stat: AC, Value: -2, SourceID: "off_guard:<source>", CasterUID: ...}`.
- BTYPE-18: Frightened from skill actions (#252 NCA-32 et al.) MUST emit as a status penalty per stack: `Bonus{Type: status, Stat: ..., Value: -stacks, SourceID: "frightened:<source>", CasterUID: ...}`.
- BTYPE-19: Drug buffs (#258 DRUG-1) MUST emit each declared buff as a `Bonus` with the type the author chose; no special-case dispatch.
- BTYPE-20: Equipment runes / potency (#261 if/when authored) MUST emit as item-typed bonuses.

### 4.5 Tests

- BTYPE-21: All existing tests for `effect.Resolve`, `EffectSet`, `BuildCombatantEffects`, and `round.go` attack/AC computation MUST pass unchanged.
- BTYPE-22: New tests MUST cover:
  - Armor migration round-trip (BTYPE-4, BTYPE-5).
  - Web `EffectsView` proto wire round-trip.
  - Override-event dedup within a tick (BTYPE-13).
  - Per-stat breakdown query returns all contributions in `(active, suppressed)` order with correct labels (BTYPE-9).

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/combat/
  combatant_effects.go              # existing; armor migration added in BuildCombatantEffects
  effects_log.go                    # NEW: dedup'd transition-event emitter

internal/game/effect/
  resolve.go                        # existing; per-stat breakdown API confirmed
  render/render.go                  # existing; gains EffectDetail(stat) helper

internal/gameserver/
  grpc_service.go                   # GetEffects, GetEffectsDetail RPCs

api/proto/game/v1/game.proto
  EffectsView, EffectView, BonusView, EffectDetailView messages

cmd/webclient/ui/src/game/character/
  EffectsPanel.tsx                  # NEW
  StatTooltip.tsx                   # NEW: per-stat breakdown
  CharacterSheet.tsx                # existing; wires StatTooltip on AC / attack / saves

internal/frontend/telnet/
  effects_handler.go                # `effects` and `effects detail <stat>` commands

migrations/                         # none — Combat.Effects already persisted
```

### 5.2 Resolution flow (unchanged from #245, recapped)

```
Bonus contributors (conditions, feats, techs, equipment, drugs, cover)
   │  produce typed Bonus entries
   ▼
EffectSet.Apply (deduplicates by SourceID + CasterUID)
   │
   ▼
effect.Resolve(set, stat) → Resolved{ Total, Contributions [{Bonus, Active, SuppressedBy}] }
   │
   ▼
combat resolver / character sheet / web tooltip read Resolved
   │
   ▼
on transitions: combatant_effects.OverrideNarrativeEvents → combat log
```

### 5.3 Single sources of truth

- Bonus type vocabulary: `internal/game/effect/bonus.go` only.
- Stacking rules: `internal/game/effect/resolve.go` only.
- Contribution rendering source labels: `internal/game/effect/render/render.go` (telnet) and the wire `BonusView` (web), both derived from `Bonus.SourceID`.
- AC: `effect.Resolve(set, StatAC).Total` — no parallel summation paths after BTYPE-2 sunset.

## 6. Open Questions

- BTYPE-Q1: When does the legacy `Combatant.ACMod` field get deleted? BTYPE-2 keeps it for one release. Recommendation: delete immediately after #265 (content migration) lands, since #265 will have audited every remaining caller.
- BTYPE-Q2: Per-stat breakdown tooltip refresh frequency — every render, or only on state change events? Recommendation: state-change events only, with a fallback recompute on round tick.
- BTYPE-Q3: Web effects panel ordering — by source, by type, or by recent change? Recommendation: by source first (group bonuses from the same effect together), within source by stat. Mirrors telnet `EffectsBlock`.
- BTYPE-Q4: Should saving-throw totals (Will, Fort, Reflex — even though Mud uses Will only via the Awareness shorthand) get the same breakdown tooltip treatment as AC and attack? Recommendation: yes — applies the same plumbing.
- BTYPE-Q5: Does the override-event narrative spam during a busy round (5+ effect changes per tick)? Recommendation: dedup window is one tick (BTYPE-13); collapse a burst of overrides for the same effect into a single line ("Heroism overrides 2 effects on Kira") if the count exceeds 3 per tick.

## 7. Acceptance

- [ ] All existing typed-bonus tests pass.
- [ ] Armor AC bonus appears in `EffectSet` as item-typed; `effect.Resolve(set, StatAC).Total` includes it; legacy `ACMod` field equals the resolved total minus base AC.
- [ ] Web character sheet displays an Effects panel; AC / attack / save totals show breakdown tooltips on hover.
- [ ] Combat log emits override events on bonus suppression transitions, deduplicated per tick.
- [ ] Telnet `effects` and `effects detail <stat>` commands print correct breakdowns.
- [ ] No regression in the cover, condition, or skill-action paths that consume the typed pipeline.

## 8. Out-of-Scope Follow-Ons

- BTYPE-F1: Weapon-rune bonuses (covered by #261).
- BTYPE-F2: Status-effect rebalancing.
- BTYPE-F3: Animated UI feedback on bonus changes.
- BTYPE-F4: Admin UI for live editing typed bonuses on a session.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/259
- Predecessor spec: `docs/superpowers/specs/2026-04-21-duplicate-effects-handling.md`
- Bonus model: `internal/game/effect/bonus.go:1-85`
- Effect set: `internal/game/effect/set.go:1-191`
- Resolver: `internal/game/effect/resolve.go:1-163`
- Telnet renderer: `internal/game/effect/render/render.go:1-144`
- Combatant effects pipeline: `internal/game/combat/combatant_effects.go:1-215`
- Override narrative emitter (currently un-routed): `internal/game/combat/combatant_effects.go:189-214`
- Combat round AC/attack call sites: `internal/game/combat/round.go`
- Content migration ticket: https://github.com/cory-johannsen/mud/issues/265
