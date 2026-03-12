# Mental State System Design

**Goal:** Add an independent multi-track mental state system with severity escalation, duration-based escalation, and active/passive recovery.

## Architecture

Option C: Conditions-as-triggers + MentalStateManager-as-lifecycle-owner.

- `internal/game/mentalstate/` — new package: Track/Severity types, Manager, Effects
- MentalStateManager tracks active states per player (4 tracks × 4 severity levels), escalation timers, auto-recovery
- Mental state effects are expressed as conditions applied to `PlayerSession.Conditions`; the Manager owns which condition is active per track
- Conditions remain the enforcement mechanism for attack/AC/damage/action restrictions
- New fields added to `ConditionDef`:
  - `APReduction int \`yaml:"ap_reduction"\``
  - `SkipTurn bool \`yaml:"skip_turn"\``
  - `SkillPenalty int \`yaml:"skill_penalty"\`` — NEW
- Condition YAMLs for mental states load from `content/conditions/mental/` subdirectory. Server startup must call `LoadDirectory` twice: once for `content/conditions/` and once for `content/conditions/mental/`.
- MentalStateManager is in-memory only. Mental state is ephemeral — it resets on player disconnect/reconnect. No database persistence is required.
- The new generic `APReduction(s *ActiveSet) int` helper sums `Def.APReduction * Stacks` across all active conditions (analogous to `AttackBonus`/`ACBonus`). The existing `StunnedAPReduction` function continues to use its hardcoded lookup for backward compatibility; in `StartRoundWithSrc` the total reduction is `StunnedAPReduction(s) + APReduction(s)`.
- Calm command follows CMD-1 through CMD-7 for active recovery

## Tracks and Severity Levels

Four independent tracks. A player can have one active level per track simultaneously.

| Track | Level 1 | Level 2 | Level 3 |
|-------|---------|---------|---------|
| Fear | Uneasy | Panicked | Psychotic |
| Rage | Irritated | Enraged | Berserker |
| Despair | Discouraged | Despairing | Catatonic |
| Delirium | Confused | Delirious | Hallucinatory |

## Effects Per State

**Fear:**
- Uneasy: skill_penalty=-1
- Panicked: restrict_actions=[flee_disabled], skill_penalty=-2
- Psychotic: skill_penalty=-3

**Rage:**
- Irritated: damage_bonus=1, ac_penalty=1
- Enraged: damage_bonus=2, ac_penalty=2, restrict_actions=[flee]
- Berserker: damage_bonus=3, ac_penalty=3, restrict_actions=[flee], attack_penalty=2 (reckless)

**Despair:**
- Discouraged: ap_reduction=1
- Despairing: ap_reduction=2, attack_penalty=-1
- Catatonic: skip_turn=true

**Delirium:**
- Confused: attack_penalty=-1, skill_penalty=-1
- Delirious: attack_penalty=-2, skill_penalty=-2
- Hallucinatory: attack_penalty=-3, skill_penalty=-3, skip_turn=true

## Escalation

Duration-based: if player remains at a severity for N rounds without recovery, advances to next level. When both escalation threshold and auto-recovery threshold are reached on the same round, escalation takes priority.
- Level 1 → Level 2: Fear=3, Rage=4, Despair=5, Delirium=4 rounds
- Level 2 → Level 3: all tracks = 5 rounds

## Auto-Recovery

Level 1 states only (if no new trigger fires):
- Fear Uneasy: 3 rounds
- Rage Irritated: 4 rounds
- Despair Discouraged: 5 rounds
- Delirium Confused: 4 rounds
- Exception: Despair Catatonic (level 3) auto-recovers to Despairing after 3 rounds

Level 2+ states require active recovery via `calm` command or item.

## Active Recovery: `calm` Command

- `calm` — costs all remaining AP in combat; no AP cost out of combat
- Rolls `d20 + combat.AbilityMod(sess.Abilities.Grit)` (modifier = (Grit-10)/2, per PF2e formula) vs DC = 10 + (severity × 4)
- On success: worst active track steps down one severity level
- Recovery always steps down one level at a time (never jumps to Normal)

## Triggers

- Fear: HP drops to/below 25% of MaxHP during combat → apply Uneasy (min)
- Rage, Despair, Delirium: NPC abilities (future) and zone effects (future) — v1 only HP trigger for Fear
- All tracks: `ApplyTrigger` callable from NPC abilities and zone effects when those systems are added

## Condition IDs

One condition per track×severity:
`fear_uneasy`, `fear_panicked`, `fear_psychotic`,
`rage_irritated`, `rage_enraged`, `rage_berserker`,
`despair_discouraged`, `despair_despairing`, `despair_catatonic`,
`delirium_confused`, `delirium_delirious`, `delirium_hallucinatory`

Loaded from `content/conditions/mental/*.yaml`.

## Integration Points

- `combat_handler.go`: `AdvanceRound` called at end of each combat round; HP threshold check after player takes damage; condition apply/remove on state change
- `grpc_service.go`: `handleMove` checks `RestrictMove` via existing `condition.IsActionRestricted(sess.Conditions, "move")` — no change needed if Catatonic restricts "move"; `handleCalm` for active recovery
- `combat/engine.go`: AP reduction via `condition.APReduction(s)` and `condition.SkillPenalty(s)` — new helpers using new `APReduction` and `SkillPenalty` fields
- `internal/game/combat/` (round processing): SkipTurn checked via `condition.SkipTurn(s)` before queuing actions; flee restriction via existing `IsActionRestricted(s, "flee")`; `combat_handler.go` (`resolveAndAdvanceLocked`): `AdvanceRound` called after round completion
- PlayerSession.Conditions is `*condition.ActiveSet` (field `Conditions` in `internal/game/session/manager.go`); initialized at login.

## Future Work (Out of Scope for This Plan)

- Forced action execution: Panicked (random action), Psychotic/Berserker (attack nearest) — requires combat auto-execution mechanism
- NPC ability triggers for Rage, Despair, Delirium tracks
- Zone effect triggers for all tracks
- Non-self calm (ally target): `calm <player_name>`
