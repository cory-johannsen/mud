# Targeting System in Combat

**Issue:** [#249 Feature: Targeting system in combat](https://github.com/cory-johannsen/mud/issues/249)
**Status:** Spec
**Date:** 2026-04-22
**Related:** #247 Cover bonuses (reuses `GridCell` / `LineCells`)

## 1. Summary

Replace the current name-string targeting model with a deliberate, UID-keyed targeting system that covers single-target actions, sticky selection across rounds, single-enemy auto-target, and authored AoE template placement (burst, cone, line). Feats and techs declare their targeting requirements in YAML so the UX can present the right picker. Invalid targets surface a typed reason to the player at selection time; enqueue-time re-validation rejects stale picks. Cover objects do not occlude targeting in v1 — visibility and line-of-sight become a follow-on ticket whose checks plug into the existing validation pipeline as a no-op stage ready to be lit up.

## 2. Requirements

- TGT-1: The authoritative target identifier in `QueuedAction` MUST be a stable UID; name strings MUST only serve display purposes and MUST be resolved to UIDs at enqueue time.
- TGT-2: Every feat and tech MUST declare a `targeting:` block with a `category` one of `self | single_ally | single_enemy | single_any | aoe_burst | aoe_cone | aoe_line`.
- TGT-3: AoE categories MUST declare a corresponding shape parameter (`burst_radius`, `cone_length`, or `line_length`) > 0; mismatches MUST be load errors.
- TGT-4: `anchored_to_actor: true` MUST be accepted only on `aoe_burst`; any other category MUST be a load error.
- TGT-5: Target validation MUST run at selection time AND at enqueue time; selection-time results only preview, enqueue-time results reject the action.
- TGT-6: The validation pipeline MUST short-circuit on first failure and return a typed `TargetingError` with a human-readable `Detail`.
- TGT-7: `CurrentTargetUID` MUST persist across rounds until the target dies, flees, leaves combat, the combat ends, or the player issues `target clear`.
- TGT-8: When `CurrentTargetUID` becomes empty AND exactly one enemy combatant is alive in the combat, the server MUST auto-populate `CurrentTargetUID` with that enemy's UID.
- TGT-9: Entering additional enemies into the combat MUST NOT override an existing sticky target selection.
- TGT-10: Telnet MUST provide a `target` command supporting listing (no arg), setting by name / prefix / disambiguator / index, and clearing.
- TGT-11: Duplicate-name combatants MUST be disambiguated with `#N` suffixes deterministically, consistent across the `target` command and narrative output.
- TGT-12: Inline override (`attack <name>`, `use <ability> on <name>`) MUST NOT mutate `CurrentTargetUID`.
- TGT-13: AoE templates (burst, cone, line) MUST be expressible via telnet commands (`at X,Y` for burst; `cone|line <dir> <length>` for cone/line) and MUST support a `preview` + `confirm` workflow.
- TGT-14: The web client MUST support click-to-select for single-target actions and a placement-mode UI for AoE actions, with shape-specific cursor overlays.
- TGT-15: Invalid target clicks / commands MUST NOT silently fail; the reason MUST be surfaced (tooltip on web; inline message on telnet).
- TGT-16: Valid target lists in both clients MUST exclude combatants that fail any validation check, with an aggregate footer count in telnet output.
- TGT-17: AoE placements that would catch allies MUST be allowed but MUST surface a warning during preview/confirm (they MUST NOT be silently blocked).
- TGT-18: Stale targets at resolution time MUST NOT cause a re-validation; the existing resolver behaviour (e.g. "swings at empty air") MUST handle the stale case.
- TGT-19: `QueuedAction.TargetX/TargetY` MUST remain as deprecated back-compat fields; callers SHOULD use `AoETemplate`.
- TGT-20: Target selection and AoE placement state MUST be per-player per-combat; it MUST NOT leak across sessions or combats.
- TGT-21: The line-of-fire validation stage MUST exist as a no-op in v1 and MUST be designed to activate once the visibility / LoS system from the follow-up ticket lands, without further engine churn.

## 3. Architecture

### 3.1 Module layout

- `internal/game/combat/targeting.go` (new) — `TargetCategory`, `TargetingError`, `ValidateSingleTarget`, `ValidateAoE`, `AoETemplate`, `AoEShape`, per-shape `Cells()` routines, compass direction enum.
- `internal/game/combat/action.go` — `QueuedAction.TargetUID` and `AoETemplate` fields added.
- `internal/gameserver/combat_targeting.go` (new) — per-session `CombatTargeting` state, single-enemy auto-target rule, selection lifecycle.
- `internal/frontend/handlers/target_command.go` (new) — telnet `target` command, AoE `preview` / `confirm` / `cancel` commands, inline-override parsing.
- Web client — new WS messages / combat-UI target-panel / placement-mode overlays.

### 3.2 Lifecycle

1. **Combat start**: populate `CombatTargeting` for each player. If exactly one enemy is alive, set `CurrentTargetUID` to that enemy per TGT-8.
2. **During action submission**: `target` commands (telnet) or click events (web) update `CurrentTargetUID` or `PendingTemplate` after validation.
3. **Enqueue**: on `attack`/`strike`/`use`, the server validates via `ValidateSingleTarget` / `ValidateAoE`, resolves the UID, and writes `QueuedAction.TargetUID` (and `AoETemplate` if applicable). On failure, the action is not enqueued and the player is informed.
4. **Resolution**: existing resolver behaviour applies. No re-validation. Stale-target narratives continue to cover the case.
5. **Target death / flee / leaves combat / combat end**: selection cleared; auto-target rule re-evaluated when the clear happens mid-combat.

### 3.3 Dependencies

- **#247 (Cover bonuses)** — not a hard dep, but this spec reuses `GridCell` and `LineCells` from #247. If #247 ships first, targeting piggybacks on those types; otherwise, they are defined minimally here and shared.
- **No dependency on visibility / LoS** — the LoF validation stage is a no-op in v1; the follow-up ticket adds the actual check.

## 4. Data Model

### 4.1 Target categories

```go
type TargetCategory string

const (
    TargetSelf        TargetCategory = "self"
    TargetSingleAlly  TargetCategory = "single_ally"
    TargetSingleEnemy TargetCategory = "single_enemy"
    TargetSingleAny   TargetCategory = "single_any"
    TargetAoEBurst    TargetCategory = "aoe_burst"
    TargetAoECone     TargetCategory = "aoe_cone"
    TargetAoELine     TargetCategory = "aoe_line"
)
```

### 4.2 Feat / tech YAML

```yaml
targeting:
  category: aoe_burst
  max_range: 25            # feet; 0 = touch/self; unset defaults to weapon range
  requires_line_of_fire: true
  burst_radius: 10         # aoe_burst only
  cone_length: 30          # aoe_cone only
  line_length: 60          # aoe_line only
  anchored_to_actor: false # aoe_burst only; emanation if true
```

Loader rules:

- `category` required.
- `aoe_*` categories require the matching shape parameter > 0.
- `anchored_to_actor: true` legal only on `aoe_burst`.
- `max_range: 0` means touch/self (adjacent or self).
- Feats/techs without a `targeting:` block default to `single_enemy` for `ActionAttack`/`ActionStrike`/`ActionUseAbility`, and `self` for `ActionUseTech` with no declared effect target. Loader warns once per fallback.

### 4.3 AoE template

```go
type AoEShape string

const (
    AoEBurst AoEShape = "burst"
    AoECone  AoEShape = "cone"
    AoELine  AoEShape = "line"
)

type CompassDir int

const (
    DirN CompassDir = iota
    DirNE
    DirE
    DirSE
    DirS
    DirSW
    DirW
    DirNW
)

type AoETemplate struct {
    Shape   AoEShape
    // Burst
    CenterX, CenterY int
    Radius           int // feet
    // Cone / Line
    OriginX, OriginY int
    Direction        CompassDir
    Length           int // feet
}

func (t AoETemplate) Cells() []GridCell
```

`Cells()` semantics:

- **Burst**: all cells with Chebyshev distance ≤ `Radius / 5` from `(CenterX, CenterY)`.
- **Cone**: cells in a 90° cone from `(OriginX, OriginY)` in `Direction`, up to `Length / 5` cells. Exact cone variant fixed at planner time; spec expects the PF2E "square-fan" pattern.
- **Line**: cells along Bresenham from `(OriginX, OriginY)` stepping `Length / 5` cells in `Direction`, clipped to grid bounds.

### 4.4 `QueuedAction` extension

```go
type QueuedAction struct {
    Type        ActionType
    Target      string       // legacy display name; narrative only
    TargetUID   string       // NEW: authoritative identifier
    Direction   string
    WeaponID    string
    ExplosiveID string
    AbilityID   string
    AbilityCost int
    TargetX     int32        // deprecated; superseded by AoETemplate
    TargetY     int32        // deprecated; superseded by AoETemplate
    AoETemplate *AoETemplate // NEW: full AoE template when applicable
}
```

Round resolver prefers `AoETemplate` when present; falls back to a single-cell burst built from `TargetX/TargetY` for legacy call sites.

### 4.5 Per-session targeting state

```go
type CombatTargeting struct {
    CurrentTargetUID string
    PendingTemplate  *combat.AoETemplate
    LastAoECoords    *GridCell
}
```

Cleared on combat end, target death, target flee, or target leaves combat. Auto-target rule re-evaluates when the clear occurs mid-combat.

### 4.6 Validation types

```go
type TargetingError int

const (
    TargetOK TargetingError = iota
    ErrTargetMissing
    ErrTargetNotInCombat
    ErrTargetDead
    ErrOutOfRange
    ErrLineOfFireBlocked // reserved; no-op until visibility ticket lands
    ErrWrongCategory
    ErrAoEShapeInvalid
)

type TargetingResult struct {
    Err    TargetingError
    Detail string
}
```

## 5. Validation Pipeline

### 5.1 Single-target

```go
func ValidateSingleTarget(
    cbt *Combat,
    actor *Combatant,
    targetUID string,
    category TargetCategory,
    maxRangeFeet int,
    requiresLineOfFire bool,
) TargetingResult
```

Stages (short-circuit on failure):

1. **Resolution**: UID present in `cbt.Combatants`; else `ErrTargetMissing`.
2. **Same-combat**: same `Combat` instance; else `ErrTargetNotInCombat`.
3. **Alive**: `!target.IsDead()`; else `ErrTargetDead`.
4. **Category fit**: self/ally/enemy match based on `Kind` comparison; else `ErrWrongCategory`.
5. **Range**: `CombatRange(actor, target) <= maxRangeFeet`; else `ErrOutOfRange`. `maxRangeFeet == 0` → target must be self or adjacent (`<= 5 ft`).
6. **Line-of-fire (reserved)**: trivially passes in v1; becomes active when visibility ticket lands.

### 5.2 AoE

```go
func ValidateAoE(
    cbt *Combat,
    actor *Combatant,
    template AoETemplate,
    category TargetCategory,
    maxRangeFeet int,
    anchoredToActor bool,
) TargetingResult
```

Stages:

1. **Shape consistency**: `template.Shape` matches `category`; else `ErrAoEShapeInvalid`.
2. **Anchor**: if `anchoredToActor`, template center equals actor cell; else `ErrAoEShapeInvalid`.
3. **Shape parameters**: `Radius`/`Length` match ability-declared value; else `ErrAoEShapeInvalid`.
4. **Origin range**: origin/center within `maxRangeFeet`; else `ErrOutOfRange`.
5. **Grid bounds on origin**: origin inside `[0, GridWidth) × [0, GridHeight)`; else `ErrAoEShapeInvalid`. Individual template cells outside the grid are clipped at resolution, not rejected at validation.
6. **Line-of-fire on origin (reserved)**: v1 no-op.

### 5.3 Enqueue-time re-validation

The same validators run at enqueue time with fresh state. Failure at enqueue drops the action and reports the reason to the player. AP is NOT deducted for a dropped action.

### 5.4 Resolution-time stale-target handling

Validators are NOT re-run at resolution. The existing resolver behaviour ("swings at empty air" for dead targets, etc.) handles stale cases; the player accepts the risk when the action was queued.

### 5.5 UI-layer previews

Telnet `target <candidate>` runs `ValidateSingleTarget` with context inferred from the sticky action (or a default single-enemy context) and reports inline. Web filters target-click validity in the UI with per-combatant status.

### 5.6 Edge cases

- **Alive → dead mid-selection**: enqueue catches stale pick; sticky cleared on death.
- **Ambiguous names**: deterministic `#N` disambiguators; `target <name>` on ambiguous input lists options; `target <name>#N` picks one.
- **Target leaves combat**: sticky cleared; player notified; auto-target re-evaluated.
- **Dynamic Kind-switching**: not supported in v1; static `Kind` used for category fit.
- **Self with `maxRange > 0`**: passes (distance 0).

## 6. Sticky Selection and Auto-Target

### 6.1 Lifecycle

`CurrentTargetUID` persists until any of: target dies, target flees, target leaves combat, combat ends, `target clear`.

### 6.2 Auto-target rule (TGT-8)

When `CurrentTargetUID` becomes empty (combat start, target death, target flee, explicit clear), the server re-evaluates: if exactly one enemy combatant is alive from the player's perspective, the server sets `CurrentTargetUID` to that enemy's UID. Zero or ≥ 2 enemies → selection stays empty.

Properties:

- Auto-selection is indistinguishable from explicit selection thereafter.
- A second enemy entering combat does NOT override the sticky target (TGT-9).
- `target clear` in a single-enemy combat is effectively a no-op — selection immediately re-auto-selects to the sole enemy.
- The UI surfaces `auto-selected` / `Auto-targeted:` cues (see §7).

## 7. UI / UX

### 7.1 Telnet — `target` command

```
target                  — list valid targets; show current selection
target <name>           — set by exact / prefix / disambiguator
target <N>              — set by list index
target clear            — unset (auto-target re-evaluates)
```

Single-enemy combat listing:

```
> target
Auto-targeted: Thug#1 (HP 8/12, 10 ft E)
```

Multi-enemy listing:

```
> target
Current target: Thug#1 (HP 8/12, 10 ft E)
Valid targets:
  1) Thug#1   (HP 8/12, 10 ft E)      [current]
  2) Thug#2   (HP 12/12, 15 ft NE)
  3) Boss     (HP 30/30, 25 ft N)
  (2 combatants not targetable: 1 out of range)
```

Invalid-pick feedback:

```
> target thug#2
Cannot target Thug#2: out of range (30 ft, max 25 ft).
```

### 7.2 Telnet — action commands

```
attack          — uses sticky selection; errors if empty and no arg
attack <name>   — one-off override; sticky unchanged
strike ...      — identical
use <ability>                 — sticky
use <ability> on <name>       — override; sticky unchanged
```

Empty-selection failure: `Select a target first: target <name>`.

### 7.3 Telnet — AoE placement

```
throw grenade at 7,4                       — burst centered on (7,4)
use overload cone north 30                 — cone direction + length
use laser_sweep line east 60               — line direction + length
preview                                    — ASCII render of PendingTemplate
confirm                                    — enqueue action with previewed template
cancel                                     — drop PendingTemplate
```

Preview output example:

```
Preview — laser_sweep line east 60:
    0 1 2 3 4 5 6 7 8 9
  0 · · · · · · · · · ·
  1 · · · · · · · · · ·
  2 · · · · · · · · · ·
  3 · · · @ * * * * * *
  4 · · · · · · · · · ·
  Covered combatants: Thug#1 (row 3, col 5), Boss (row 3, col 9)
  Allies in area: none.
```

Ally warnings appear in preview but do not block confirm.

### 7.4 Telnet — combat prompt

```
HP: 22/30   AP: 3   R: 1   Target: Thug#1 (HP 8/12)
```

Null: `Target: none`.

### 7.5 Web — click-to-select

- Hover a combatant: tentative highlight (green for valid, red for invalid).
- Click a valid combatant: sets `CurrentTargetUID` via a `set_target` WS message.
- Selected combatant renders with a gold outline.
- Target panel shows name, HP, distance, status. When auto-selected, a muted `auto-selected` subheader appears until the player explicitly clicks to confirm/override.
- Invalid clicks show a transient tooltip reason and do not select.

### 7.6 Web — AoE placement

- Clicking an AoE ability enters placement mode.
- Burst: hover shows a circle overlay; click confirms.
- Cone: hover shows a fan from actor cell toward hover direction; click confirms.
- Line: hover shows a line from actor to hover cell direction; click confirms.
- Confirmation modal appears only when the preview catches allies.
- `Esc` cancels placement.

### 7.7 Web — ability buttons

Buttons disable (grayed with tooltip reason) when no valid target / placement exists for the current state.

### 7.8 Accessibility

- Telnet outputs honour color capability flags; all highlights have text fallbacks.
- Web keyboard conventions (existing) extend to target cycling and AoE cursor movement.

### 7.9 Non-goals for UI v1

- No tab-nearest-enemy smart target.
- No saved target preferences across combats.
- No per-target combat log filtering.
- No pre-attack outcome preview ("hit chance 75%").

## 8. Testing

Per SWENG-5 / SWENG-5a, TDD with property-based tests where appropriate.

- `internal/game/combat/targeting_test.go` — table tests for category fit matrix, range boundaries, AoE shape-param validation, `anchored_to_actor` restriction, dead-target rejection.
- `internal/game/combat/aoe_template_test.go` — property tests for burst / cone / line `Cells()` correctness across all eight cardinal + ordinal directions; grid clipping behaviour.
- `internal/gameserver/target_session_test.go` — scripted auto-target behaviour across 1 / 2 / 0 enemy states, target death → re-evaluation, sticky vs inline override, `target clear` re-auto-select.
- `internal/frontend/handlers/target_command_test.go` — telnet command tests for list / set / clear / disambiguation / invalid-reason surfacing.
- Web component tests — click-to-select highlights, hover tooltips, AoE placement cursor, ally warning modal, ability button enablement.

## 9. Documentation

- `docs/architecture/combat.md` — new "Targeting" section hosting `TGT-N` requirements, category taxonomy, validation pipeline, and auto-target rule.
- `docs/architecture/combat.md` — AoE subsection describing burst / cone / line geometry.
- Content-authoring documentation — the `targeting:` YAML block for feats and techs with worked examples per category.

## 10. Non-Goals (v1)

- No visibility / LoS / fog-of-war system (follow-up ticket).
- No smart-target keybindings beyond existing conventions.
- No saved target preferences across combats.
- No per-target combat log filtering.
- No pre-attack outcome previews.
- No dynamic `Kind`-switching for charmed combatants.
- No cross-room targeting.
- No predictive pathing for AoE placement.
- No alias resolvers (`last_attacker`, `nearest_enemy`, `lowest_hp`).

## 11. Open Questions for the Planner

- Exact protobuf shape for `SetTargetRequest` / `AoEPlacement` messages — prefer extending existing combat WS payloads.
- Where `CombatTargeting` state lives — session struct vs. per-combat-per-player map.
- Cone geometry variant — PF2E square-fan is the default; planner confirms the chosen variant.
- Scope of `#N` disambiguator — combat-only, or also in `look`, room listings, etc.

## 12. Follow-On Work

A separate ticket tracks the visibility / line-of-sight system:

- Introduce a per-cell occlusion/opacity model.
- Cover objects (or a new wall type) may or may not occlude — decision deferred to that ticket.
- Lights, smoke, and similar environmental modifiers feed occlusion.
- `ValidateSingleTarget` / `ValidateAoE` line-of-fire stage (currently no-op) lights up with the real check.
- UI: invisible / occluded combatants are not listed as valid targets; partial occlusion surfaces as a warning.
