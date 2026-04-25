---
title: Social Encounters
issue: https://github.com/cory-johannsen/mud/issues/257
date: 2026-04-25
status: spec
prefix: SE
depends_on: []
related:
  - "#252 Non-combat actions (DoS resolver, condition catalog)"
  - "#255 Exploration challenges (multi-stage menu UX pattern)"
  - "Faction system (per-character reputation)"
---

# Social Encounters

## 1. Summary

Today, social interaction with an NPC is one-shot:
- `handleTalk` (`internal/gameserver/grpc_service_quest_giver.go:114-210`) routes to a quest-giver dialog or a placeholder line.
- `handleSeduce` (`grpc_service_seduce.go:29-80`) is a single d20-vs-Savvy opposed roll with a binary outcome (charmed / hostile).
- `handleBribe` (`grpc_service_bribe.go:46`) is a credit-cost transaction with no skill check.
- NPC `Disposition` (`internal/game/npc/instance.go:110-111`) lives on a four-step ladder (`hostile`, `wary`, `neutral`, `friendly`) but no system mutates it from in-game actions.
- Faction reputation (`internal/game/faction/service.go`) tracks per-character per-faction integers but is only changed by hand-coded events (kills, bribes through Fixers).

The issue asks for **structured social encounters**: a multi-skill, multi-round interaction with an accumulated success/failure tally against an NPC-derived DC, with outcomes that move `Disposition`, faction reputation, and unlock (or lock) quests / access / rewards.

This is essentially the PF2E **Influence** subsystem from the *Gamemastery Guide*. It composes naturally with spec #252's `SkillActionResolver` (DoS math is shared), spec #255's player-facing menu UX (multi-option choice with timeout), and the existing faction reputation pipeline. The new content type is a `SocialEncounter` definition; the new runtime is a per-(character, npc) session with a tally and a stage stream.

## 2. Goals & Non-Goals

### 2.1 Goals

- SE-G1: A social encounter is a multi-round, multi-skill interaction whose progression depends on accumulated successes vs failures against per-NPC discovery and influence DCs.
- SE-G2: Each round, the player chooses one influence skill from a menu of options the NPC permits. The DoS resolver from spec #252 grades the roll.
- SE-G3: Encounters terminate on threshold success, threshold failure, round-limit timeout, or explicit walk-away.
- SE-G4: Encounter outcomes drive NPC `Disposition`, faction reputation deltas, and unlock effects via the same effect-block dispatch as exploration challenges (#255).
- SE-G5: NPCs declare *biases*: skills the NPC is receptive to (success counts ×2 toward influence threshold) and skills the NPC resists (success counts ×0.5; failures ×2).
- SE-G6: NPCs declare *discovery DCs*: a Recall Knowledge / Society / Streetwise check that, when succeeded prior to or during the encounter, reveals their biases. Discovery is per-character and persistent.
- SE-G7: Existing `handleTalk`, `handleSeduce`, `handleBribe` interactions continue to work; encounters are an opt-in path declared on individual NPCs.

### 2.2 Non-Goals

- SE-NG1: Multi-NPC group conversations (e.g., a debate between three NPCs and the player). v1 is one player + one NPC.
- SE-NG2: Real-time / timed dialogue choices.
- SE-NG3: NLP / free-text input. Choices are menu-selected.
- SE-NG4: Replacing seduction or bribe handlers; encounters are a sibling system that may or may not eventually subsume them (out of scope for v1).
- SE-NG5: Persistent NPC memory beyond the disposition / reputation / discovery state already tracked.
- SE-NG6: Authoring GUI; YAML only.

## 3. Glossary

- **Social encounter**: a structured, multi-round interaction between one player and one NPC, declared in `content/social/encounters/`.
- **Round**: one player action within an encounter (one skill choice + one roll).
- **Tally**: paired counters `(successes, failures)` accumulated during the encounter.
- **Influence threshold**: the success count required to win the encounter; declared per encounter, scaled by NPC level.
- **Failure threshold**: the failure count that ends the encounter in failure; default `3` unless the NPC declares otherwise.
- **Bias**: per-NPC multiplier on a skill's contribution. `receptive` = ×2 success, `resistant` = ×0.5 success and ×2 failure, `neutral` = ×1.
- **Discovery**: a per-(character, npc) flag set when the player has identified the NPC's biases via a Recall Knowledge / Society check.

## 4. Requirements

### 4.1 Content Model

- SE-1: A new YAML schema `SocialEncounter` MUST be supported under `content/social/encounters/<id>.yaml` with fields:
  - `id`, `display_name`, `description`.
  - `npc_id` (string) — bound to one NPC template id.
  - `goal` (string, free-form) — the player-facing goal: "extract information", "negotiate access", etc.
  - `influence_threshold` (int) — default `4`, scaled per SE-12.
  - `failure_threshold` (int) — default `3`.
  - `round_limit` (int) — default `6`; encounter ends in failure on hitting the limit.
  - `discovery_dc` (object) — `{ kind: "fixed", value: int }` or `{ kind: "npc_level", expr: string }`; default `{ kind: "npc_level", expr: "10 + level" }`.
  - `discovery_skills` (list of strings) — skills accepted for the discovery roll (default: `reasoning`, `street_smarts`, `lore_<faction>`).
  - `influence_skills` (list of objects) — each `{ skill, bias: receptive | neutral | resistant, dc: { kind, value | expr } }`. The DC may differ per skill; default to NPC's Will DC.
  - `outcomes` (object) — keys `success`, `failure`, `walk_away`; each is a list of effect blocks (per spec #255 EXC-3 plus extensions in SE-2).
- SE-2: Effect-block types MUST extend the existing dispatch with:
  - `set_disposition: <hostile|wary|neutral|friendly>` — overrides the current disposition.
  - `shift_disposition: <int>` — moves disposition by N steps (clamped).
  - `change_faction_rep: { faction_id, delta }` — applies via the faction service.
  - `unlock_quest: { quest_id }` — adds quest id to the NPC's offered pool for this character.
  - `lock_quest: { quest_id }` — removes a quest id from the offered pool for this character.
  - `unlock_dialogue_topic: { topic_id }` — registers a per-(character, npc) dialogue topic flag.
  Existing types from #255 (`grant_item`, `grant_credits`, `grant_xp`) remain valid here.
- SE-3: The loader MUST validate that every `npc_id` resolves to a known NPC template; every `quest_id` to a known quest; every `faction_id` to a known faction; every skill to a known Mud skill.
- SE-4: At least three exemplar social encounters MUST be authored: (a) negotiate-a-deal with a Fixer, (b) extract-information from a Wary informant, (c) gain-an-ally with a Friendly faction NPC. The implementer MUST coordinate with the user on which existing NPCs to attach the encounters to.

### 4.2 Discovery

- SE-5: A new RPC `BeginRecallNPCRequest{ npc_uid }` MUST roll the player's best `discovery_skills` skill vs `discovery_dc` and, on success, set a per-(character, npc) `BiasesDiscovered = true` flag. On failure, no retry until the NPC's disposition changes.
- SE-6: When biases are discovered, the influence-skill menu (SE-9) MUST visually annotate each skill with its bias (`(receptive)`, `(resistant)`, `(neutral)`). When undiscovered, the annotation is hidden.
- SE-7: A new table `character_npc_discoveries(character_id, npc_id, biases_discovered bool, discovered_at)` MUST be migrated.

### 4.3 Encounter Session

- SE-8: A new package `internal/game/social/encounter/` MUST host the runtime:
  - `Service.Begin(charID, npcUID, encounterID) (*Session, error)` — returns the active session view.
  - `Service.Choose(sessionID, skillID) (RoundResult, error)` — rolls the chosen skill, updates tally, returns DoS and per-round narrative.
  - `Service.WalkAway(sessionID) error` — resolves with `walk_away` outcomes; encounter ends.
- SE-9: Each round, `Service.Begin` and `Service.Choose` MUST return an `EncounterView` containing: `goal`, `tally`, `round_index`, `round_limit`, `available_skills` (list with bias annotation per SE-6), `walk_away_allowed bool`.
- SE-10: A round's tally update MUST follow PF2E DoS:
  - `crit_success`: +2 success (×NPC bias multiplier on success counter).
  - `success`: +1 success.
  - `failure`: +1 failure (×NPC bias multiplier on failure counter when bias is `resistant`).
  - `crit_failure`: +2 failure.
  Bias multipliers apply on success counter for `receptive` and `resistant`; on failure counter for `resistant`.
- SE-11: When `successes >= influence_threshold` → encounter ends in `success`. When `failures >= failure_threshold` → encounter ends in `failure`. When `round_index >= round_limit` and neither threshold met → encounter ends in `failure`. Walk-away is always permitted when `walk_away_allowed` is true.
- SE-12: `influence_threshold` MUST scale with NPC level: `effective_threshold = max(2, base_threshold + floor((npc.level - char.level)/2))`. Caps at `base_threshold + 4`.
- SE-13: An encounter MUST be cancelled and removed when the player enters combat with the same NPC, leaves the room, or disconnects. On cancellation, no outcome effects fire (the encounter is treated as walk-away with no `walk_away` block executed).

### 4.4 Effect Application

- SE-14: When the encounter ends, the resolver MUST apply each effect block in the matching outcome list in authoring order via the same dispatch used by exploration challenges (#255), extended per SE-2.
- SE-15: A `set_disposition` or `shift_disposition` effect MUST update `npc.Instance.Disposition` and persist via the existing NPC instance store.
- SE-16: A `change_faction_rep` effect MUST call `factionSvc.SaveRep` with the delta applied to the current value.
- SE-17: An `unlock_quest` / `lock_quest` effect MUST add or remove the quest id from a per-(character, npc) overlay layer that `handleTalk` consults when offering quests. The base NPC `QuestIDs` pool is unchanged.
- SE-18: An `unlock_dialogue_topic` effect MUST insert the topic id into a per-(character, npc) flag set so future dialogue branches that gate on the topic become available. Topics are out-of-scope to fully define here; this requirement reserves the wiring.

### 4.5 Player UX — Telnet

- SE-19: A new telnet command `social <npc_name>` MUST initiate the encounter when the named NPC has a social encounter declared. If multiple encounters are valid, list them and let the player pick.
- SE-20: Each round MUST present the menu: `Round <i>/<round_limit> — successes <s>/<thr>, failures <f>/<flim>` followed by `<n>) <skill_label>` lines and a `0) walk away` line when permitted.
- SE-21: A 90-second no-input timeout MUST end the encounter as walk-away.
- SE-22: Per-round narrative MUST include the rolled total, DC, DoS, and the tally delta.

### 4.6 Player UX — Web

- SE-23: A new web modal `SocialEncounterModal.tsx` MUST present the same surface as telnet with skill buttons, tally bars (success / failure / round), and a walk-away button.
- SE-24: Bias annotations on skill buttons MUST appear only when `BiasesDiscovered` is true for that NPC.
- SE-25: Discovery roll MUST be initiable from the modal as a separate "Recall Knowledge" button before the first influence round; success unlocks bias display from that round onward.

### 4.7 Migration & Compatibility

- SE-26: `handleTalk` MUST be extended so that, when an NPC has at least one social encounter declared and the player has not yet completed it (or it is repeatable), the talk response includes a "Begin social encounter" option. The existing quest-giver path remains the default.
- SE-27: `handleSeduce` and `handleBribe` MUST continue to work unchanged. A future ticket can migrate them under the encounter framework if desired.

### 4.8 Tests

- SE-28: New tests in `internal/game/social/encounter/service_test.go` MUST cover:
  - Tally math under each DoS combination and bias category.
  - Threshold scaling vs NPC level.
  - Round-limit termination.
  - Walk-away terminates with `walk_away` outcomes.
  - Cancellation on combat-entry / room-leave produces no effect application.
  - Discovery RPC sets the persistent flag and is one-shot per disposition state.
- SE-29: Property tests under `internal/game/social/encounter/testdata/rapid/` MUST verify determinism (same seed → same trajectory) and tally monotonicity.

## 5. Architecture

### 5.1 Where the new code lives

```
content/social/encounters/
  *.yaml                                    # 3+ exemplars

internal/game/social/encounter/
  def.go, service.go, store.go, effect.go
  service_test.go
  testdata/rapid/                           # property tests

internal/game/npc/
  instance.go                               # existing; per-(character, npc) overlays added

internal/storage/postgres/
  social_encounter_progress.go              # NEW
  npc_discoveries.go                        # NEW

internal/gameserver/
  grpc_service.go                           # BeginRecallNPC, BeginSocial, ChooseSkill, WalkAway RPCs
  grpc_service_quest_giver.go               # handleTalk extension for "Begin social encounter"

api/proto/game/v1/game.proto
  SocialEncounterView, RoundResult, EncounterRequest messages

cmd/webclient/ui/src/game/social/
  SocialEncounterModal.tsx

migrations/
  NNN_character_npc_discoveries.up.sql / .down.sql
```

### 5.2 Encounter flow

```
player: social <npc_name>
   │
   ▼
handleSocialBegin → encounter.Service.Begin → SocialEncounterView (round 0)
   │
   ▼
loop:
   client renders menu (telnet console / web modal)
   player picks skill
   handleSocialChoose → encounter.Service.Choose
      roll d20 + skillBonus vs option.dc → DoS via shared resolver (#252)
      tally += DoS_delta * bias_multiplier
      narrative emitted
      if successes >= threshold: outcome = success → terminate
      if failures >= failure_threshold: outcome = failure → terminate
      if round_index >= round_limit: outcome = failure → terminate
      else return next SocialEncounterView (round_index += 1)
   │
   ▼
on terminate:
   apply outcomes[outcome] effect blocks (set_disposition, change_faction_rep,
      unlock_quest, grant_item, ...)
   write completion state
   close session
```

### 5.3 Single sources of truth

- DoS computation: shared with `internal/game/skillaction/resolver.go` (per spec #252).
- Effect dispatch: shared with `internal/game/exploration/challenge/effect.go` (per spec #255), extended per SE-2.
- Disposition: `internal/game/npc/instance.go` `Disposition` field.
- Faction reputation: `internal/game/faction/service.go`.

## 6. Open Questions

- SE-Q1: Should the discovery roll be one-shot per encounter or one-shot per NPC-disposition cycle? Recommendation: per disposition cycle — when the NPC's disposition changes (set/shift effect or external event), the discovery flag clears, requiring a new Recall Knowledge.
- SE-Q2: Does failing an encounter make subsequent encounters with the same NPC harder (e.g., +2 to all DCs for 24 hours)? Recommendation: defer to the implementer to decide based on user feedback at landing time; v1 ships no penalty.
- SE-Q3: Should the influence-threshold scaling per SE-12 floor at 2 or 1? PF2E uses 2 as the practical minimum. Recommendation: keep at 2.
- SE-Q4: Walk-away on an in-progress encounter — does it cost in-fiction reputation (-1 with NPC's faction)? Recommendation: no by default; authors can declare a `walk_away` outcome with a `change_faction_rep: -1` block if they want that drama.
- SE-Q5: Should bribery integrate with encounters (spend credits to convert a failure into a success)? Recommendation: yes, as a follow-on; v1 keeps bribe and encounter as separate paths.

## 7. Acceptance

- [ ] Three exemplar social encounters load, validate, and resolve end-to-end on telnet and web.
- [ ] Tally math correctly applies bias multipliers per SE-10.
- [ ] An encounter against a higher-level NPC has a higher effective influence threshold per SE-12.
- [ ] Discovery RPC sets the persistent flag and the bias annotations appear in subsequent rounds only.
- [ ] Outcome effects (`set_disposition`, `change_faction_rep`, `unlock_quest`, `grant_item`) apply correctly and persist.
- [ ] Walk-away terminates with the `walk_away` outcome block applied.
- [ ] Cancellation on combat-entry produces no effect application.
- [ ] Existing `handleTalk`, `handleSeduce`, `handleBribe` continue to work for NPCs without social encounters declared.

## 8. Out-of-Scope Follow-Ons

- SE-F1: Multi-NPC group conversations.
- SE-F2: Real-time / timed choice mechanics.
- SE-F3: Free-text dialogue input.
- SE-F4: NPC long-term memory of past social encounters beyond disposition / discovery / unlock state.
- SE-F5: Bribery integration into encounters (per SE-Q5).
- SE-F6: Migrating seduction / bribe handlers under the encounter framework.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/257
- Talk handler: `internal/gameserver/grpc_service_quest_giver.go:114-210`
- Seduce handler: `internal/gameserver/grpc_service_seduce.go:29-80`
- Bribe handler: `internal/gameserver/grpc_service_bribe.go:46`
- NPC instance Disposition: `internal/game/npc/instance.go:110-111`
- NPC template Disposition: `internal/game/npc/template.go:128-130`
- Faction service: `internal/game/faction/service.go:1-58`
- Skill-action DoS resolver (sibling): `docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md` (NCA-7)
- Effect dispatch pattern: `docs/superpowers/specs/2026-04-25-exploration-challenges.md` (EXC-3)
- PF2E Influence subsystem: Pathfinder 2e Gamemastery Guide, Subsystems chapter
