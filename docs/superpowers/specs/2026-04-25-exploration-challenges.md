---
title: Exploration Challenges
issue: https://github.com/cory-johannsen/mud/issues/255
date: 2026-04-25
status: spec
prefix: EXC
depends_on: []
related:
  - "Zone difficulty scaling spec (2026-04-19) — DC ladder by zone tier"
  - "Zone quests RRQ/VTQ spec (2026-04-13) — reward grant pipeline"
  - "#256 Exploration quests"
  - "#252 Non-combat actions (skill action DoS resolver shape)"
---

# Exploration Challenges

## 1. Summary

The phrase "exploration challenges" sounds like missing infrastructure, but most of the moving parts are already wired:

- Rooms carry a `SkillChecks []TriggerDef` field (`internal/game/world/model.go:183`).
- `applyRoomSkillChecks` runs on every `EnterRoom` and rolls `1d20 + AbilityMod + skillRankBonus(rank)` vs the trigger's DC (`grpc_service.go:3175-3263`).
- The resolver `skillcheck.Resolve` already produces full PF2E degrees of success and dispatches an `OutcomeMap` keyed by `crit_success` / `success` / `failure` / `crit_failure`.
- Outcomes already support effect types `damage`, `condition`, `deny`, `reveal`.
- XP is awarded on success and crit-success via `xpSvc.AwardSkillCheck` (`:3233`).
- Hazards, traps, Lua `on_enter`, room effects all run from the same enter-room pipeline.

What is genuinely missing for the issue text — *skill-check-based obstacles with multiple options, progress effects, and rewards* — is:

1. **Choice between skills.** A challenge may offer "Athletics OR Acrobatics OR Engineering Lore"; today a `TriggerDef` declares one `Skill`. No way to author a single challenge with multiple resolution paths.
2. **Multi-step / multi-stage challenges.** The current model fires once per `Trigger` per room entry. There is no notion of a sequence ("disable two consecutive locks") or a contested branch ("if you choose stealth, the next check is harder").
3. **Item / credit rewards on success.** Outcomes today can only `damage`, `condition`, `deny`, `reveal`. No `grant_item`, `grant_credits`, or `grant_xp_bonus` — XP is hard-coded by `xpSvc.AwardSkillCheck` and not authorable per-challenge.
4. **Progress effects.** "Affect progress" implies success unlocks an exit or a follow-on challenge; today the closest analogue is `deny` (block all action) but no "open this exit" / "advance this challenge".
5. **Persistence.** Challenges fire on every entry. There is no "completed once, do not re-fire" semantic, no per-character "you have already disarmed this trap".
6. **Player-facing UX.** When a challenge fires, there is no menu — the system rolls the prescribed skill silently. The issue implies the player should *choose* among options.

This spec lights up those five gaps as a `Challenge` content type that composes with the existing `SkillChecks` / `Triggers` / `Outcomes` / `Hazards` infrastructure, plus a per-character completion ledger so challenges don't perpetually re-fire.

## 2. Goals & Non-Goals

### 2.1 Goals

- EXC-G1: Authoring a multi-option, multi-stage exploration challenge in a single YAML record that lives alongside (and references) existing room skill-check triggers.
- EXC-G2: Player-facing choice menu (telnet + web) when a challenge offers more than one resolution path, with timeout fallback.
- EXC-G3: Outcome effects extended to grant items, credits, XP bonuses, advance challenge stage, and unlock exits.
- EXC-G4: Per-character completion ledger so a one-shot challenge does not re-fire after success.
- EXC-G5: Zone-level DC defaults pulled from the zone tier (per the 2026-04-19 zone difficulty scaling spec) so authors do not hand-roll DCs unless they want to override.
- EXC-G6: Backward compatibility: every existing `SkillChecks` trigger continues to fire as it does today; no content edits required to keep current rooms working.

### 2.2 Non-Goals

- EXC-NG1: Replacing `SkillChecks` triggers. The new `Challenge` model is additive; rooms may continue to declare bare `SkillChecks` for one-skill silent rolls.
- EXC-NG2: Branching challenge graphs with arbitrary topology. v1 supports linear stages and a single fork at most per stage.
- EXC-NG3: Real-time / timed challenges (e.g., "complete in 10 seconds of wall-clock"). Pure turn-based; the menu timeout is a politeness, not a mechanic.
- EXC-NG4: Group challenges where multiple players cooperate on different stages. v1 is single-character.
- EXC-NG5: Combat-mode integration. Exploration challenges only fire in exploration mode; entering combat suspends any in-progress challenge.
- EXC-NG6: Authoring tools / GUI for designers. YAML edits remain the workflow.

## 3. Glossary

- **Challenge**: a named, structured obstacle attached to a room (or a sequence of rooms). Composed of one or more **stages**.
- **Stage**: one decision point. Offers one or more **options**, each with a skill and DC.
- **Option**: a single resolution path within a stage — `{ skill, dc, ap_cost (optional), description }`.
- **Outcome**: one of `crit_success`, `success`, `failure`, `crit_failure`. Each option declares its own outcome block.
- **Effect**: a typed result applied on outcome resolution. Existing types: `damage`, `condition`, `deny`, `reveal`. New types in this spec: `grant_item`, `grant_credits`, `grant_xp`, `advance_stage`, `unlock_exit`, `complete_challenge`.
- **Completion ledger**: a per-character (per-account) record of which one-shot challenges have been completed.

## 4. Requirements

### 4.1 Content Model

- EXC-1: A new YAML schema `Challenge` MUST be supported under `content/exploration/challenges/<id>.yaml`. Each file MUST declare:
  - `id` (string, snake_case, unique).
  - `display_name` (string).
  - `description` (string) — narrative blurb shown when the challenge fires.
  - `trigger` (object) — when the challenge fires. Fields: `room_id` (or list), `event` (`on_enter` default; `on_use` for interactables), `min_zone_tier` (optional gate).
  - `repeatable` (bool, default `false`) — when `false`, completion is recorded in the per-character ledger and the challenge does not re-fire.
  - `stages` (list) — at least one entry. Each stage:
    - `id` (string, unique within challenge).
    - `narrative` (string) — what the player sees when this stage activates.
    - `options` (list, ≥ 1) — see EXC-2.
    - `outcomes` (object) — outcome → list of effects (see EXC-3).
- EXC-2: Each `option` MUST declare:
  - `id` (string, unique within stage).
  - `label` (string) — the menu text shown to the player (e.g., "Climb the fence (Athletics)").
  - `skill` (string) — id of the Mud skill used.
  - `dc` (object) — same shape as spec #252's `dc`: `{ kind: "fixed", value: int }`, `{ kind: "zone_tier" }`, or `{ kind: "formula", expr: string }`.
  - `ap_cost` (int, default `0`) — exploration-mode actions are typically free, but harder options can cost the equivalent of a 10-minute exploration tick.
- EXC-3: Each `outcomes[*]` value MUST be a list of effect blocks. Existing effect types remain valid; this spec adds:
  - `grant_item: { item_id, quantity }`.
  - `grant_credits: { amount }`.
  - `grant_xp: { amount }` — overrides the implicit `xpSvc.AwardSkillCheck` behavior when present.
  - `advance_stage: { stage_id }` — moves the challenge to the named next stage.
  - `unlock_exit: { direction }` — sets a per-character flag so the named exit becomes traversable for this player.
  - `complete_challenge` — terminates the challenge and writes to the completion ledger.
- EXC-4: When `outcomes[*]` does not include either `advance_stage` or `complete_challenge`, the challenge MUST terminate after that outcome (implicit completion). This avoids forcing every author to write a terminator on simple one-stage challenges.
- EXC-5: The loader MUST validate that every `advance_stage.stage_id` resolves to a stage in the same challenge; every `unlock_exit.direction` resolves to a real room exit; every `grant_item.item_id` resolves to a known item; every `skill` resolves to a known Mud skill.
- EXC-6: At least three exemplar challenges MUST be authored as part of this work to exercise: (a) one-stage, two-options, repeatable; (b) two-stage, single-option, one-shot; (c) single stage with branching outcomes that route to different stages on success vs failure.

### 4.2 Resolver

- EXC-7: A new package `internal/game/exploration/challenge/` MUST host the resolver:
  - `Service` constructor taking `dice.Roller`, `xpSvc`, item registry, condition registry, completion-ledger store.
  - `Activate(ctx, char, challengeID) (*Session, error)` — opens a session against the named challenge from the named character; returns the active stage and its menu.
  - `Resolve(ctx, sessionID, optionID) (Outcome, error)` — rolls the chosen option, applies effects, advances or completes the session.
  - `Cancel(ctx, sessionID) error` — abandon (e.g., entering combat).
- EXC-8: `Resolve` MUST compute the degree of success using the same PF2E rules as spec #252's `SkillActionResolver` (natural 1/20 ±10 band). The two resolvers MUST share the DoS computation function — extract from #252 if needed.
- EXC-9: When a challenge's stage has exactly one option AND the room author has chosen `auto_resolve: true` on the stage, `Activate` MUST proceed straight to `Resolve` without prompting the player. This preserves today's `SkillChecks` silent-roll behavior for migrated content.
- EXC-10: A challenge session MUST be cancelled and removed when the character enters combat, leaves the room, or disconnects.
- EXC-11: When a challenge is `repeatable: false` and a `complete_challenge` effect fires, the resolver MUST write the (character_id, challenge_id) pair to the completion ledger and the challenge MUST NOT re-fire on subsequent room entries.

### 4.3 Player UX — Telnet

- EXC-12: When a challenge with more than one option (or `auto_resolve: false`) activates, the server MUST send a numbered menu to the telnet console: `Challenge: <display_name> — <stage.narrative>` followed by one line per option `<n>) <option.label>` and a `cancel` line `0) walk away`.
- EXC-13: The player MUST be able to choose by typing the option number, the option id, or `cancel` / `0`.
- EXC-14: A 60-second no-input timeout MUST auto-`cancel` the menu and emit `You hesitate too long.`. Combat or movement during the timeout window also cancels.
- EXC-15: The narrative output for each outcome MUST appear in the console with the rolled total, the DC, and the DoS rendered explicitly (`Roll 17 + 5 = 22 vs DC 18 → success`).

### 4.4 Player UX — Web

- EXC-16: The web client MUST render an in-flow modal showing the challenge banner, narrative, and option buttons. The modal closes automatically on resolution.
- EXC-17: Each option button MUST display its skill icon, label, DC summary (e.g., "Athletics DC 18"), and AP-cost badge if non-zero.
- EXC-18: The cancel button MUST always be present; the same auto-cancel semantics from EXC-14 apply.
- EXC-19: Outcome rendering MUST include a roll detail panel (roll, mod, total, DC, DoS) collapsible behind a "Show roll" toggle.

### 4.5 Per-Character Completion Ledger

- EXC-20: A new database table `character_completed_challenges` MUST be created with columns `character_id` (FK), `challenge_id` (text), `completed_at` (timestamp). Composite primary key on `(character_id, challenge_id)`.
- EXC-21: A migration `migrations/NNN_character_completed_challenges.up.sql` and `.down.sql` MUST be authored. The migration number is the next available after current head.
- EXC-22: A repository `internal/game/exploration/challenge/store.go` MUST expose `IsCompleted(charID, challengeID) (bool, error)` and `MarkCompleted(charID, challengeID) error`.
- EXC-23: The challenge service MUST consult `IsCompleted` before activating a non-repeatable challenge and silently skip when true.

### 4.6 Zone-Tier DC Defaults

- EXC-24: A new helper `dc.ForZoneTier(tier int, difficulty Difficulty) int` MUST live in the existing zone-difficulty package and return PF2E DC-by-level scaled by a difficulty modifier (`Easy = -2`, `Standard = 0`, `Hard = +2`, `Severe = +5`, `Extreme = +10`). The mapping table MUST follow PF2E DCs by Level.
- EXC-25: When an option's `dc.kind == "zone_tier"`, the resolver MUST call `dc.ForZoneTier(zone.Tier(), option.Difficulty)` to compute the DC at activation time.
- EXC-26: When an option's `dc.kind == "fixed"` the value is used as-is; this is the explicit author override.

### 4.7 Exit Unlocks

- EXC-27: A new field `unlocked_exits []string` MUST be added to the per-character session state. When a `unlock_exit` effect fires, the named direction is appended for the duration of the session.
- EXC-28: Movement validation in `handleMove` (or equivalent) MUST allow traversal through a normally-locked exit when the player has the unlock in `unlocked_exits`.
- EXC-29: Unlocks are session-scoped by default. A per-effect flag `persist: true` MUST allow unlocks to be written to a `character_unlocked_exits` table for permanent unlocks (e.g., a door that stays open for that character forever). The default `false` matches PF2E's typical "you climbed it once but the rope is gone" semantics.

### 4.8 Migration of Existing Content

- EXC-30: Existing room-level `SkillChecks` triggers MUST continue to work unchanged. The challenge model is opt-in.
- EXC-31: A documentation note in `docs/architecture/` MUST describe when to use a `SkillChecks` trigger vs a `Challenge`. Heuristic: silent single-roll = trigger; player-facing menu, multi-stage, or grants rewards = challenge.

### 4.9 Telemetry

- EXC-32: Every challenge activation, option selection, outcome, and completion MUST emit a structured log line: `character_id`, `challenge_id`, `stage_id`, `option_id`, `roll`, `dc`, `dos`, `outcome_effects`. Feeds tuning and balance review.

## 5. Architecture

### 5.1 Where the new code lives

```
content/exploration/challenges/
  *.yaml                                # at least 3 exemplars

internal/game/exploration/challenge/
  def.go                                # YAML schemas: Challenge, Stage, Option, Effect
  service.go                            # Activate / Resolve / Cancel
  store.go                              # CompletionLedger repository
  effect.go                             # Effect dispatch (extends existing skillcheck effects)
  service_test.go, store_test.go

internal/gameserver/
  grpc_service.go                       # ListChallenges, ActivateChallenge, ResolveChallenge RPCs
  grpc_service_explore.go               # NEW: thin handlers wiring the service

api/proto/game/v1/game.proto
  ChallengeView, ChallengeStageView, ChallengeOptionView, ChallengeOutcomeView messages
  ActivateChallengeRequest / ResolveChallengeRequest

migrations/
  NNN_character_completed_challenges.up.sql / .down.sql
  NNN_character_unlocked_exits.up.sql   / .down.sql  (only if EXC-29 persistence ships)

cmd/webclient/ui/src/game/exploration/
  ChallengeModal.tsx                    # NEW: in-flow challenge UI

internal/frontend/telnet/
  challenge_handler.go                  # NEW: numbered menu render + input handling
```

### 5.2 Activation flow

```
player enters room
   │
   ▼
applyRoomSkillChecks (existing, unchanged) — silent triggers fire
   │
   ▼
resolveRoomChallenges (NEW) — for each Challenge whose trigger matches,
                              IsCompleted? → skip ;  else → service.Activate
   │
   ▼
service.Activate(char, challengeID) → opens session, returns ChallengeView with stage 0
   │
   ▼
client renders menu (telnet console / web modal)
   │
   ▼
player picks option → service.Resolve(sessionID, optionID)
   │
   ▼
roll → DoS → effects:
   damage / condition / deny / reveal — existing effect dispatch reused
   grant_item / grant_credits / grant_xp / advance_stage / unlock_exit / complete_challenge — new effect dispatch
   │
   ▼
if advance_stage: emit next ChallengeView; else terminate session
   │
   ▼
on complete_challenge with !repeatable: ledger write
```

### 5.3 Single sources of truth

- DoS computation: shared with `internal/game/skillaction/resolver.go` (per spec #252).
- DC-by-zone-tier mapping: `dc.ForZoneTier` in the zone-difficulty package.
- Item / credit / XP grants: existing `xpSvc`, `creditSvc`, item registry.
- Completion state: `character_completed_challenges` table only.

## 6. Open Questions

- EXC-Q1: Should the cancel option (`0`) cost any AP / time? PF2E "spending an exploration action to even attempt" is a real cost. Recommendation: cancel is free in v1 — losing the reward is enough disincentive.
- EXC-Q2: When two challenges trigger on the same room entry, do they queue (one menu, then the next) or race? Recommendation: queue strictly by YAML declaration order; menus open serially.
- EXC-Q3: Repeated runs through a `repeatable: true` challenge — should there be diminishing returns on rewards (e.g., XP halved on subsequent successes)? Recommendation: no in v1; authors who want this can author multiple stages.
- EXC-Q4: Should `unlock_exit` be visible to other players in the same room? Recommendation: no — unlocks are per-character state, mirroring PF2E's "you climbed up but the rope dangled back; your buddy still has to climb".
- EXC-Q5: The completion ledger stores `(character_id, challenge_id)`. What about a "global" / shared completion (one-time world events)? Recommendation: defer to a future ticket; not in v1.

## 7. Acceptance

- [ ] Three exemplar challenges author and validate at server startup.
- [ ] Telnet menu and web modal both render correctly and accept input within the 60-second window.
- [ ] An `unlock_exit` effect successfully grants traversal through a normally-locked exit on the same session.
- [ ] A non-repeatable challenge does not re-fire on second entry to the same room by the same character.
- [ ] A repeatable challenge re-fires on every entry and the player gets the menu each time.
- [ ] `dc.ForZoneTier` returns the PF2E DC-by-level for the zone's tier scaled by the option's difficulty modifier.
- [ ] Existing `SkillChecks` triggers continue to fire unchanged on rooms that have not migrated to challenges.
- [ ] Migration applies and rolls back cleanly on a fresh database.

## 8. Out-of-Scope Follow-Ons

- EXC-F1: Group challenges (multi-character cooperation).
- EXC-F2: Branching challenge graphs beyond single-fork-per-stage.
- EXC-F3: Wall-clock time limits on stages.
- EXC-F4: Combat-mode challenges (e.g., "you have 3 rounds to disable the bomb mid-fight").
- EXC-F5: Authoring GUI in the admin UI.
- EXC-F6: Faction reputation effects on challenge outcomes.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/255
- Room model with `SkillChecks`: `internal/game/world/model.go:183`
- Room enter pipeline: `internal/gameserver/grpc_service.go:2782-2920`
- Existing skill-check resolver: `internal/gameserver/grpc_service.go:3175-3263`
- Skill check types: `internal/game/skillcheck/types.go:30-75`
- XP award helper: `internal/gameserver/grpc_service.go:3233-3250` (`xpSvc.AwardSkillCheck`)
- Quest reward shape (item/credits/XP precedent): `internal/game/quest/def.go:12-43`
- Hazards: `internal/game/world/model.go:217-236`
- Zone difficulty scaling: `docs/superpowers/specs/2026-04-19-zone-difficulty-scaling.md`
- Zone quest reward pipeline: `docs/superpowers/specs/2026-04-13-zone-quests-rrq-vtq.md`
- Skill action DoS resolver (sibling): `docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md` (NCA-7 natural-1/20 rule)
- PF2E Seek action: `vendor/pf2e-data/packs/pf2e/actions/basic/seek.json`
