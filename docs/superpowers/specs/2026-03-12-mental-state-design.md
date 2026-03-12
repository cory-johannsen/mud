# Mental State System Design

**Goal:** Add an independent multi-track mental state system with severity escalation, duration-based escalation, and active/passive recovery.

## Architecture

Option C: Conditions-as-triggers + MentalStateManager-as-lifecycle-owner.

- `internal/game/mentalstate/` — new package: Track/Severity types, Manager, Effects
- MentalStateManager tracks active states per player (4 tracks × 4 severity levels), escalation timers, auto-recovery
- Mental state effects are expressed as conditions applied to `PlayerSession.Conditions`; the Manager owns which condition is active per track
- Conditions remain the enforcement mechanism for attack/AC/damage/action restrictions
- New fields added to `ConditionDef`: `APReduction int`, `SkipTurn bool`
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
- Panicked: restrict_actions=[flee_disabled, random forced action — v2], skill_penalty=-2
- Psychotic: restrict_actions=[attack_nearest forced — v2], skill_penalty=-3

**Rage:**
- Irritated: damage_bonus=1, ac_penalty=1
- Enraged: damage_bonus=2, ac_penalty=2, restrict_actions=[flee]
- Berserker: damage_bonus=3, ac_penalty=3, restrict_actions=[flee], attack_penalty=-2 (reckless)

**Despair:**
- Discouraged: ap_reduction=1
- Despairing: ap_reduction=2, attack_penalty=-1
- Catatonic: skip_turn=true

**Delirium:**
- Confused: attack_penalty=-1, skill_penalty=-1
- Delirious: attack_penalty=-2, skill_penalty=-2
- Hallucinatory: attack_penalty=-3, skill_penalty=-3, skip_turn=true

## Escalation

Duration-based: if player remains at a severity for N rounds without recovery, advances to next level.
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
- Rolls `d20 + GritMod` vs DC = 10 + (severity × 4)
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
- `combat/engine.go`: AP reduction via `condition.APReduction(s)` — new helper using new `APReduction` field
- `round.go`: SkipTurn via `condition.SkipTurn(s)` — new helper; flee restriction via existing `IsActionRestricted`
