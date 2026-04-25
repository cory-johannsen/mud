---
title: Non-Combat Actions Against Combat NPCs
issue: https://github.com/cory-johannsen/mud/issues/252
date: 2026-04-24
status: spec
prefix: NCA
depends_on: []
related:
  - "#247 Cover bonuses in combat (condition pipeline users)"
  - "#249 Targeting system in combat (target validation shared surface)"
  - "#260 Natural 1 / Natural 20 (degrees-of-success dependency)"
---

# Non-Combat Actions Against Combat NPCs

## 1. Summary

The issue asks to "allow players to use non-combat actions (Intimidate, Demoralize, Feint, etc.) against NPCs during combat — skill check vs NPC DC, apply conditions on success, surface in combat UI."

Research shows **the underlying plumbing already exists**: nine skill-vs-NPC combat actions are implemented as handlers in `internal/gameserver/grpc_service.go` (Feint, Demoralize, Grapple, Trip, Disarm, Shove, RaiseShield, TakeCover, FirstAid), they spend AP correctly, and they apply conditions via `combat_handler.ApplyCombatCondition`. What is missing is the three things that make this category actually useful at the table:

1. **Degrees of success.** Every handler today resolves binary pass/fail (except Shove, which has a partial crit check for distance). PF2E skill actions derive their power from having four outcomes — crit success, success, failure, crit failure — each with different condition tiers, durations, or backfires.
2. **Discovery.** There is no server RPC that lists available skill actions, no telnet command to enumerate them, and no web action-bar entry that surfaces them to players who don't already know the command names.
3. **Rule fidelity.** Demoralize applies a generic `-1 AC / -1 attack` instead of the PF2E **Frightened** condition; Feint applies a persistent `-2 AC` instead of the single-attack **Off-Guard** window. The custom effects are live in content and save data, so migration must be careful.

This spec delivers all three: a degrees-of-success resolver reused by every skill action, a canonical condition catalog in YAML, a `ListCombatActionsRequest` RPC that both surfaces and their UI can read, and an audit + migration of the nine existing handlers to PF2E-aligned behavior. It also adds the four PF2E skill actions that are highest-value and currently missing — **Create a Diversion**, **Bon Mot**, **Tumble Through**, and **Recall Knowledge** — behind the same uniform resolver.

## 2. Goals & Non-Goals

### 2.1 Goals

- NCA-G1: Every skill-vs-NPC combat action resolves through a single `SkillActionResolver` that produces one of the four PF2E degrees of success.
- NCA-G2: Condition effects are expressed in YAML (with id, duration, effect blocks) rather than inline in handlers — one source of truth per condition.
- NCA-G3: Players can discover which skill actions are available to them in combat through a single server query, on both telnet and web.
- NCA-G4: Nine existing handlers are migrated to the new resolver with no loss of gameplay intent and no silent nerf/buff of existing mechanics.
- NCA-G5: Four new skill actions — Create a Diversion, Bon Mot, Tumble Through, Recall Knowledge — ship behind the same resolver.
- NCA-G6: Target validation (melee reach vs range, LoS, valid-target-kind) is enforced server-side for every skill action; no AP is consumed on a validation failure.
- NCA-G7: Every existing test that pins current Feint / Demoralize / Grapple / Trip / Disarm / Shove / RaiseShield / TakeCover / FirstAid behavior passes after migration, via a compatibility layer in the resolver for the non-PF2E effects where the user explicitly chooses to keep them.

### 2.2 Non-Goals

- NCA-NG1: Redesigning the ability-score → skill mapping. Mud uses Brutality/Grit/Quickness/Reasoning/Savvy/Flair with custom skill names (`muscle`, `grift`, `smooth_talk`, `patch_job`, `hustle`, `toughness`, etc.) instead of PF2E's ability map. This spec reuses the existing mapping.
- NCA-NG2: General-purpose "use any skill on any target" sandbox. Only the actions declared in YAML with a `combat: true` flag are callable in combat.
- NCA-NG3: NPC skill actions against players. NPCs get their own path through the HTN planner and are out of scope here (a separate ticket can generalize the resolver once player-side proves out).
- NCA-NG4: Action macros, aliases, and hotbars. UI surfacing is the listing RPC + action-bar entries; binding to hotkeys is a follow-on.
- NCA-NG5: Persistent skill cooldowns / once-per-combat limits. AP economy is the only gate in v1.
- NCA-NG6: Animations / visual effects on the combat map. Narrative line + condition badge is sufficient v1 feedback.

## 3. Glossary

- **Skill action**: a combat action whose resolution is a skill check against a target DC (e.g., Demoralize, Feint).
- **Degree of success (DoS)**: one of `CritSuccess`, `Success`, `Failure`, `CritFailure`; derived from `roll - DC` per PF2E rules.
- **Resolver**: `SkillActionResolver` — a single server-side function that takes `(actor, target, actionDef, roll)` and returns an outcome.
- **Skill action definition**: a YAML record in `content/skill_actions/` describing an action's inputs and outputs.
- **Condition definition**: a YAML record in `content/conditions/` describing a named, reusable effect that can be applied for a duration.
- **Canonical condition**: a condition whose semantics match PF2E (e.g., `frightened`, `off_guard`, `prone`, `clumsy`, `stupefied`).
- **Legacy condition**: an existing Mud condition that the audit decides to keep as-is (e.g., Demoralize's `-1 AC / -1 attack` bundle if the user chooses rule fidelity loss over migration disruption).

## 4. Requirements

### 4.1 Content Catalog

- NCA-1: A new content directory `content/skill_actions/` MUST hold one YAML file per skill action. Each file MUST declare:
  - `id` (string, snake_case, unique): e.g., `demoralize`, `feint`.
  - `display_name` (string).
  - `description` (string): short narrative blurb, used in listing RPC.
  - `ap_cost` (int): action-point cost.
  - `skill` (string): id of the Mud skill used (`smooth_talk`, `grift`, `muscle`, etc.).
  - `dc` (object): either `{ kind: "target_perception" }`, `{ kind: "target_will" }`, `{ kind: "target_ac" }`, `{ kind: "fixed", value: int }`, or `{ kind: "formula", expr: string }` for rare bespoke DCs.
  - `range` (object): either `{ kind: "melee_reach" }`, `{ kind: "ranged", feet: int }`, or `{ kind: "self" }`.
  - `target_kinds` (list): which combatant kinds are valid (`npc`, `player`, `self`, `any`).
  - `outcomes` (object keyed by DoS): for each of `crit_success`, `success`, `failure`, `crit_failure`, a list of effect blocks (`apply_condition: {id, stacks, duration_rounds}` / `damage: {expr}` / `move: {feet}` / `narrative: string`).
- NCA-2: A new content directory `content/conditions/` MUST hold one YAML file per canonical condition. Each file MUST declare `id`, `display_name`, `description`, `effects` (list of typed bonuses per the #245 bonuses model), and `stacking` (one of `independent`, `replace_if_higher`, `max_stacks`).
- NCA-3: The loader MUST validate: every `outcomes.*.apply_condition.id` resolves to a condition file; every `skill` resolves to a known Mud skill id; every numeric field is non-negative.
- NCA-4: At least the following conditions MUST be authored in `content/conditions/`:
  - `frightened` (1-4 stacks, status penalty to checks and DCs equal to stacks, ticks down by 1 per round).
  - `off_guard` (single-event or duration variant; grants `-2 circumstance AC` to the affected combatant).
  - `clumsy` (1-4 stacks, status penalty to Dex-based checks and AC equal to stacks).
  - `stupefied` (1-4 stacks, status penalty to Int/Wis/Cha-based checks equal to stacks).
  - `prone` (binary, `-2 circumstance attack`, triggers flat-footed vs melee).
  - `grabbed` (binary, `-2 circumstance AC`, no move).
  - `fleeing` (binary, forces flee-AP spend on its turn, 1 round).
- NCA-5: Legacy condition bundles that the audit chooses to keep (see §4.6) MUST be authored as conditions in `content/conditions/` with explicit ids rather than inlined in handlers.

### 4.2 Resolver

- NCA-6: A new package `internal/game/skillaction/` MUST provide:
  - `type Outcome` with fields `DoS DegreeOfSuccess`, `Roll int`, `DC int`, `Bonus int`, `Narrative string`.
  - `func Resolve(ctx ResolveContext, def *ActionDef) (Outcome, error)` — the single resolver.
  - `type ResolveContext` carrying actor, target, Combat, dice roller, and an `Apply` callback for applying effect blocks.
- NCA-7: `Resolve` MUST compute the DoS as:
  - `CritSuccess` when `roll + bonus >= DC + 10` OR roll is a natural 20 and `roll + bonus >= DC`.
  - `Success` when `roll + bonus >= DC` but not crit.
  - `Failure` when `roll + bonus < DC` but not crit failure.
  - `CritFailure` when `roll + bonus <= DC - 10` OR roll is a natural 1 and `roll + bonus < DC`.
  The natural-1/20 rules follow PF2E: a natural 20 bumps the DoS up by one step; a natural 1 bumps it down by one step. This is captured separately from the ±10 band.
- NCA-8: `Resolve` MUST apply every effect block listed in `outcomes[DoS]` via the existing condition / damage / movement pipelines. Effects MUST be applied in authoring order.
- NCA-9: `Resolve` MUST emit a narrative line per outcome, falling back to a generic "Actor's <action> on Target <outcome>." template when `narrative` is omitted in the YAML.
- NCA-10: `Resolve` MUST NOT consume AP — AP is deducted by the handler before the call (mirrors today's pattern so test pins at the handler layer survive).
- NCA-11: The resolver MUST be pure-function except for the `Apply` callback — no direct mutation of `Combat`.

### 4.3 Target Validation

- NCA-12: A new helper `skillaction.ValidateTarget(ctx, def)` MUST be called before the resolver runs. Failures MUST return a structured error naming the missing precondition.
- NCA-13: For `range.kind == "melee_reach"`, validation MUST require Chebyshev distance of 1 cell between actor and target.
- NCA-14: For `range.kind == "ranged", feet: N`, validation MUST require Chebyshev distance ≤ `N/5` cells.
- NCA-15: Validation MUST honor the line-of-fire no-op extension point from #249; once #267 (LoS) ships, a `PostValidateTarget` hook is consulted without resolver changes.
- NCA-16: Validation MUST reject a target whose `kind` is not in the action's `target_kinds` list.
- NCA-17: When validation fails, the handler MUST NOT consume AP and MUST surface the named precondition to the client.

### 4.4 Discovery RPC

- NCA-18: A new proto message `ListCombatActionsRequest` MUST be added with optional fields `actor_uid`, `target_uid`; server infers the session's actor if `actor_uid` empty.
- NCA-19: The matching `ListCombatActionsResponse` MUST return `repeated CombatActionEntry` with fields `id`, `display_name`, `description`, `ap_cost`, `skill`, `dc_summary` (human-readable, e.g., "vs Target Perception DC"), `range_summary` (e.g., "melee reach"), `available` (bool — true if actor has AP and target validation would pass for the supplied target), and `unavailable_reason` (string — filled when `available == false`).
- NCA-20: The RPC MUST enumerate every action in `content/skill_actions/` where the action's `target_kinds` and `range` permit the supplied target (or all actions when `target_uid` is empty).
- NCA-21: `available == false` with `unavailable_reason` MUST be surfaced — the UI shows greyed-out actions with tooltip reasons ("out of range", "insufficient AP", "target is not an NPC").
- NCA-22: The RPC MUST NOT leak GM-only information (e.g., exact target DCs). `dc_summary` is the kind/source only.

### 4.5 UI Surfacing

- NCA-23: **Telnet**: a new command `skills` (alias `s`) MUST list available skill actions in the console region. Output format is one line per action: `<id>  <display_name> (AP <cost>, <range>, <dc_summary>) — <available|unavail: reason>`.
- NCA-24: **Telnet**: each existing skill action command (`feint`, `demoralize`, etc.) MUST continue to work unchanged. The new `skills` command is additive.
- NCA-25: **Web**: the combat action bar MUST render a new "Skill Actions" submenu. Entries come from `ListCombatActionsRequest`; unavailable entries render greyed-out with the reason in a tooltip; clicking an available entry opens the usual target-picker flow.
- NCA-26: **Web**: skill-action buttons MUST use the same AP-cost display and click-to-execute affordance as existing combat buttons. No new interaction pattern.
- NCA-27: The UI listing MUST refresh after each round tick; the client does not cache per-turn availability.

### 4.6 Audit & Migration of Existing Handlers

- NCA-28: Each of the nine existing handlers (`Feint`, `Demoralize`, `Grapple`, `Trip`, `Disarm`, `Shove`, `RaiseShield`, `TakeCover`, `FirstAid`) MUST be migrated to the new resolver via a YAML skill-action definition. Target behavior is summarized below; the implementer MUST confirm each choice with the user before migration lands.

| Action | Today | Proposed | Notes |
|---|---|---|---|
| Demoralize | `-1 AC / -1 attack` on success only | **Frightened 1** on success, **Frightened 2** on crit success | PF2E-aligned. Breaks existing save data holding the legacy effect — migration pass per NCA-29. |
| Feint | `-2 AC` persistent on success | **Off-Guard** window (next melee attack only) on success; full-turn on crit success; *actor* off-guard on crit failure | PF2E-aligned. |
| Grapple | `grabbed` on success | `grabbed` on success, `restrained` on crit success, actor `off_guard` on crit failure | Slightly expanded. |
| Trip | `prone` on success | `prone` on success, `prone + fall damage d6` on crit success, actor `prone` on crit failure | Slightly expanded. |
| Disarm | Weapon removed on success | Target takes `-2` to attacks with held weapon on success; weapon drops on crit success; actor `off_guard` on crit failure | Moves closer to PF2E staged outcome. |
| Shove | 5 ft push, 10 ft on DC+10 | 5 ft on success, 10 ft + prone on crit success, no move on failure, actor prone on crit failure | Consolidates existing ad-hoc crit. |
| RaiseShield | `+2 AC` + `shield_raised` | No change (not a skill check) | Stays out of resolver path. |
| TakeCover | cover-tier condition + cover set | No change today (but see #266 for next step) | Stays out of resolver path. |
| FirstAid | DC 15, heal 2d8+4 | DoS-graded healing: fail=0, success=2d8+4, crit success=4d8+8, crit failure=actor loses 1d4 HP (gear malfunction) | Adds richer table output. |
- NCA-29: The migration MUST include a runtime compatibility step: any in-flight combat whose `Combatant.Conditions` slice contains the legacy ad-hoc entries (`-1 AC from demoralize`, `-2 AC from feint`) MUST be mapped at combat-start to the closest canonical condition (`frightened:1`, `off_guard:1-round`). Save/load tests cover this.
- NCA-30: Each migrated handler MUST keep the same proto request shape so client code does not break; the handler body becomes a thin wrapper that builds a `ResolveContext` and calls the resolver.
- NCA-31: The existing tests (`grpc_service_test.go` Feint suite and equivalents for the other actions) MUST pass unchanged or, when they pin a post-migration-divergent behavior, be updated in the same commit with a clear test comment marking the PF2E alignment change.

### 4.7 New Actions

- NCA-32: Four new skill actions MUST be authored in `content/skill_actions/`:
  - `create_a_diversion` — `grift` vs Perception DC; on success all enemies are `off_guard` to actor's next attack this turn; auditory/visual traits.
  - `bon_mot` — `smooth_talk` vs Will DC; on success target is `-2 status` to Perception and Will saves for 1 minute; crit success doubles duration.
  - `tumble_through` — `hustle` vs target Reflex DC; on success actor may move through the target's space as if it were difficult terrain; crit failure leaves actor prone in the target's threatened cell.
  - `recall_knowledge` — `reasoning` vs fixed DC derived from target's level (`14 + level`); on success reveals one piece of target metadata (weakness, immunity, notable ability) per outcome tier.
- NCA-33: `recall_knowledge` MUST NOT consume AP on crit failure (the actor shares **incorrect** information — still a meaningful in-fiction cost but does not spend combat economy). This is a deliberate PF2E nod handled as a special-case `ap_refund: true` field in the `crit_failure` outcome YAML.

### 4.8 Telemetry & Narrative

- NCA-34: Every resolved skill action MUST emit a structured log entry: `action_id`, `actor_uid`, `target_uid`, `roll`, `bonus`, `dc`, `dos`. Feeds post-combat review and balance tuning.
- NCA-35: Narrative output MUST include the DoS in text (e.g., `Kira's Demoralize succeeds critically — the thug is Frightened 2.`). Narrative variants per DoS are authored in the YAML (NCA-1 `narrative` field).

## 5. Architecture

### 5.1 Where the new code lives

```
content/
  conditions/
    frightened.yaml, off_guard.yaml, clumsy.yaml, stupefied.yaml,
    prone.yaml, grabbed.yaml, fleeing.yaml
  skill_actions/
    demoralize.yaml, feint.yaml, grapple.yaml, trip.yaml, disarm.yaml,
    shove.yaml, first_aid.yaml,
    create_a_diversion.yaml, bon_mot.yaml, tumble_through.yaml, recall_knowledge.yaml

internal/game/skillaction/
  def.go            # ActionDef, DC, Range, Outcome structs + YAML loader
  resolver.go       # Resolve(), DoS computation, natural 1/20 rules
  target.go         # ValidateTarget + melee/range checks
  apply.go          # Apply callback: condition / damage / move dispatch

internal/game/condition/
  def.go            # existing; ensure YAML loader reads content/conditions/*.yaml

internal/gameserver/
  grpc_service.go   # existing nine handlers trimmed to thin wrappers;
                    #   new ListCombatActions RPC added.
  grpc_service_skillaction_test.go  # NEW: end-to-end coverage per action

api/proto/game/v1/game.proto
  ListCombatActionsRequest / Response, CombatActionEntry messages
  (existing per-action request messages retained unchanged)
```

### 5.2 Runtime flow

```
client → proto request (FeintRequest, DemoralizeRequest, ...)
       → grpc_service handler (validate session, find actor + target)
       → combatH.SpendAP(actor, def.ap_cost)
       → skillaction.ValidateTarget(ctx, def) — may return structured error (refund AP)
       → dice.RollExpr("1d20")
       → skillaction.Resolve(ctx, def) →
             compute DoS
             apply outcomes[DoS] effect blocks
             emit narrative
             return Outcome
       → handler appends Outcome to combat log, sends response
```

### 5.3 Single sources of truth

- Condition definitions: `content/conditions/*.yaml` loaded once at startup.
- Skill action definitions: `content/skill_actions/*.yaml` loaded once at startup.
- Degrees of success math: `internal/game/skillaction/resolver.go` only.
- Target validation: `internal/game/skillaction/target.go` only.

## 6. Open Questions

- NCA-Q1: The audit table in §4.6 proposes PF2E-aligned outcomes but Demoralize's current `-1 AC / -1 attack` is a **known, shipped** effect some content may rely on. Do we (a) migrate fully to Frightened, (b) keep the legacy bundle as a second condition `demoralized_legacy` and retire it over time, or (c) expose a server flag so ops can choose? Recommendation: (a) + NCA-29 compatibility mapping — cleaner, fewer long-term code paths.
- NCA-Q2: `recall_knowledge` surfaces NPC metadata. Which metadata is player-visible (AC, HP, weaknesses) vs GM-only (full stat block)? Recommendation: two tiers — success surfaces one of `[weaknesses, notable ability, immunity]`; crit success surfaces two. Never surface exact HP or AC.
- NCA-Q3: Should `ListCombatActionsRequest` also include the existing non-skill combat actions (Attack, Stride, Reload, Use Tech) for a unified action-bar populate? Recommendation: no — keep this RPC skill-specific, and let the client merge with its existing action list. Smaller blast radius.
- NCA-Q4: Telnet's `skills` output could be long. Pagination or category filters (`skills offensive`, `skills support`)? Recommendation: no pagination v1; 13 actions fit comfortably on one telnet screen. Revisit if the count grows.
- NCA-Q5: NCA-33's AP refund on `recall_knowledge` crit failure is a special case. Should we generalize `ap_refund` as a per-outcome YAML field any action can use? Recommendation: yes — cost is low, and future actions (e.g., `aid` failure) may want it.

## 7. Acceptance

- [ ] Every file in `content/skill_actions/` loads and validates at server startup.
- [ ] `ListCombatActionsRequest` returns the correct availability for each action given the supplied actor + optional target.
- [ ] All nine migrated handlers pass their pre-existing tests (with comment-annotated updates where PF2E alignment diverges).
- [ ] Four new skill actions resolve end-to-end in a manual combat on both telnet and web.
- [ ] Condition catalog in `content/conditions/` includes the seven required entries and they are applied with correct duration and stacking.
- [ ] A load test creates a combat whose save data holds legacy Demoralize/Feint ad-hoc entries and confirms the compatibility mapping in NCA-29 produces the canonical conditions.
- [ ] Telnet `skills` command lists every content-declared action with correct availability flags.
- [ ] Web combat action bar renders the Skill Actions submenu with correct greyed-out reasons.

## 8. Out-of-Scope Follow-Ons

- NCA-F1: NPC skill actions (HTN-planned Demoralize / Feint against players).
- NCA-F2: Once-per-combat / cooldown gates.
- NCA-F3: Skill-action animations or VFX on the combat map.
- NCA-F4: Hotbar / macro bindings for skill actions.
- NCA-F5: Team-based skill actions (combined Demoralize from multiple actors scaling the Frightened tier).
- NCA-F6: Skill-action feats (feats that modify a specific skill action, e.g., `Intimidating Prowess` applying Strength to Demoralize).

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/252
- Existing handlers and AP pattern: `internal/gameserver/grpc_service.go:9987,10072,10189,10272,10333,10398,10459,10520,10601`
- Condition apply API: `internal/gameserver/combat_handler.go:1364` (`ApplyCombatCondition`)
- Active condition struct: `internal/game/condition/active.go:12`
- Feint tests: `internal/gameserver/grpc_service_test.go` (`TestHandleFeint_*`)
- PF2E reference (Demoralize): `vendor/pf2e-data/packs/pf2e/actions/skill/demoralize.json`
- PF2E reference (Feint): `vendor/pf2e-data/packs/pf2e/actions/skill/feint.json`
- Condition definitions target location: `internal/game/condition/` (existing package)
- Dice roller: `internal/gameserver/grpc_service.go` dependency `s.dice`
- Cover spec: `docs/superpowers/specs/2026-04-21-cover-bonuses-in-combat.md`
- AoE spec (for eventual multi-target skill actions): `docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md`
- Targeting spec (line-of-fire extension point): `docs/superpowers/specs/2026-04-22-targeting-system-in-combat.md`
