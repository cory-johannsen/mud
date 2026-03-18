# Tech Effect Resolution — Design Spec

**Date:** 2026-03-18

---

## Goal

Implement the effect resolution system for all three tech activation paths (prepared, spontaneous, innate), restructure the `TechEffect` model to support 4-tier PF2E save outcomes and attack-roll tiers, populate all 14 existing technology YAML files with real effect definitions, and add 4 new condition definitions required by those effects.

---

## Context

`handleUse` in `internal/gameserver/grpc_service.go` currently expends prepared/spontaneous/innate uses and returns a plain "You activate X" message. No effect resolution happens. The `TechEffect` model exists but uses a flat `[]TechEffect` list that cannot express per-tier outcomes. `ResolveSave` in `internal/game/combat/resolver.go` already implements 4-tier outcome resolution for `toughness`/`hustle`/`cool` save types and is ready to use. The combat system uses a 1-D position model (`Combatant.Position` in feet) so push/movement effects are directly implementable.

**PF2E → Gunchete conversion:** See `docs/requirements/pf2e-import-reference.md` for the canonical save-type mapping (Fortitude→toughness, Reflex→hustle, Will→cool), tradition mapping, and source spell data for all 14 technologies.

**Out of scope:** Amped Technology (separate sub-project); `chrome_reflex` fortune/reroll mechanic (requires Reactions system); `seismic_sense`/`moisture_reclaim` passive mechanics (separate sub-project); persistent damage (no DoT system); zone summon effects.

---

## Feature 1: Tiered Effect Model

### 1a. `TieredEffects` struct

File: `internal/game/technology/model.go`

Replace `Effects []TechEffect` and `AmpedEffects []TechEffect` on `TechnologyDef` with:

```go
// TieredEffects holds per-outcome effect lists for a technology.
// Only the tiers relevant to the tech's Resolution type need to be populated.
// Save-based: OnCritSuccess/OnSuccess/OnFailure/OnCritFailure
// Attack-based: OnMiss/OnHit/OnCritHit
// No-roll: OnApply
type TieredEffects struct {
    // Save-based tiers (resolution: "save")
    OnCritSuccess []TechEffect `yaml:"on_crit_success,omitempty"`
    OnSuccess     []TechEffect `yaml:"on_success,omitempty"`
    OnFailure     []TechEffect `yaml:"on_failure,omitempty"`
    OnCritFailure []TechEffect `yaml:"on_crit_failure,omitempty"`
    // Attack-based tiers (resolution: "attack")
    OnMiss    []TechEffect `yaml:"on_miss,omitempty"`
    OnHit     []TechEffect `yaml:"on_hit,omitempty"`
    OnCritHit []TechEffect `yaml:"on_crit_hit,omitempty"`
    // No-roll (resolution: "none")
    OnApply []TechEffect `yaml:"on_apply,omitempty"`
}
```

Update `TechnologyDef`:

```go
type TechnologyDef struct {
    // ... existing fields unchanged ...
    Resolution  string        `yaml:"resolution,omitempty"`   // "save" | "attack" | "none"
    Effects     TieredEffects `yaml:"effects,omitempty"`
    AmpedEffects TieredEffects `yaml:"amped_effects,omitempty"`
    // SaveType and SaveDC remain on TechnologyDef (drive save roll for resolution:"save")
}
```

**Precondition:** `Resolution` must be `"save"`, `"attack"`, or `"none"` when set. Empty string is treated as `"none"` for backwards compatibility with existing YAML that predates this change.

**Validation update:** Add to `Validate()`:
- If `Resolution == "save"`: `SaveType` must be non-empty and `SaveDC > 0`
- If `Resolution == "attack"`: `SaveType` must be empty
- If `Resolution == ""` or `"none"`: `SaveType` and `SaveDC` must be empty/zero

### 1b. `TechEffect` unchanged

The flat `TechEffect` struct fields are unchanged. The existing `type`, `dice`, `damage_type`, `amount`, `condition_id`, `value`, `duration`, `distance`, `direction` fields continue to work — they are now embedded inside `TieredEffects` tier slices rather than directly on `TechnologyDef`.

---

## Feature 2: Proto Change — UseMessage Target

File: `api/proto/game/v1/game.proto`

Add optional target field to `UseMessage`:

```protobuf
message UseMessage {
    string ability_id = 1;
    string target     = 2; // optional; empty = use combat default or self
}
```

Run `make proto` to regenerate `internal/gameserver/gamev1/game.pb.go`.

File: `internal/frontend/handlers/bridge_handlers.go`

Update `bridgeUse` to pass the second argument (if present) as `Target` on the proto message. The command parser splits on whitespace: `use <abilityID> [target]`.

---

## Feature 3: Effect Resolver

File: `internal/gameserver/tech_effect_resolver.go` (new)

```go
package gameserver

// ResolveTechEffects resolves all effects for a tech activation and returns
// a slice of human-readable result messages. It does not expend uses — the
// caller has already done that.
//
// Preconditions:
//   - sess must be non-nil
//   - tech must be non-nil and already validated
//   - target may be nil for self/utility/heal effects; required for damage/condition effects
//   - combatEngine may be nil when not in combat; condition effects applied out-of-combat
//     are silently skipped (no condition system available outside of combat)
//   - dice must be non-nil (used for damage and heal rolls)
//   - condRegistry must be non-nil
//
// Postconditions:
//   - Returns at least one message describing the outcome
//   - sess.CurrentHP and target.CurrentHP never go below 0
//   - sess.CurrentHP never exceeds sess.MaxHP
func ResolveTechEffects(
    sess         *session.PlayerSession,
    tech         *technology.TechnologyDef,
    targets      []*combat.Combatant, // empty slice for self/utility; one entry for single; all enemies for area
    combatEngine *combat.Combat,      // nil when not in combat
    condRegistry *condition.Registry,
    dice         dice.Roller,
    src          combat.Source,
) []string
```

**`techAttackMod(sess, tech)`** is a package-private function in `tech_effect_resolver.go`:
```go
// techAttackMod returns the tech attack modifier for the given session and tech.
// Formula: sess.Level/2 + primaryAbilityMod where primary ability is tradition-derived:
//   neural → Savvy, bio_synthetic → Grit, technical → Quickness.
func techAttackMod(sess *session.PlayerSession, tech *technology.TechnologyDef) int
```

### Area vs single-target

The caller builds the `targets` slice based on `tech.Targets`:
- `TargetsSelf` or `resolution:"none"`: pass `nil` or empty slice
- `TargetsSingle`: pass `[]*combat.Combatant{resolvedTarget}` (one combatant)
- `TargetsAllEnemies`: pass all enemy `Combatant`s from `combatEngine`

`ResolveTechEffects` iterates over `targets` and applies the same tier effects to each. Result messages are per-target: `"Nox Raider critically fails: Slowed 2."`, `"Rust Crawler fails: Slowed 1."`.

### Resolution branches

**`resolution: "none"` (or empty):**
Apply `tech.Effects.OnApply` directly to each element of `targets` (or to `sess` for heal). For utility effects, append description string once (not per target).

**`resolution: "save"`:**
For each target:
1. Call `combat.ResolveSave(tech.SaveType, target, tech.SaveDC, src)` → `outcome`
2. Select tier: `CritSuccess→OnCritSuccess`, `Success→OnSuccess`, `Failure→OnFailure`, `CritFailure→OnCritFailure`
3. Apply selected tier effects to that target
4. Prepend outcome label: `"<Name> fails: "`, `"<Name> critically fails: "`, etc.

**`resolution: "attack"`:**
For each target:
1. Roll `1d20 + techAttackMod(sess, tech)` vs `target.AC`
   - `>= target.AC + 10`: CritHit; select `OnCritHit`
   - `>= target.AC`: Hit; select `OnHit`
   - otherwise: Miss; select `OnMiss`
2. Apply selected tier effects to that target

### Effect application (per `TechEffect`)

| `Type` | Action |
|--------|--------|
| `"damage"` | Roll `Dice` (via `dice.RollExpr`), add flat `Amount`, subtract from `target.CurrentHP` (floor 0) |
| `"heal"` | Roll `Dice`, add flat `Amount`, add to `sess.CurrentHP` (cap at `sess.MaxHP`) |
| `"condition"` | Look up `ConditionID` in `condRegistry`; if `combatEngine == nil`, skip silently; else call `combatEngine.Conditions[target.ID].Apply(uid, def, value, duration)` (enemy) or `combatEngine.Conditions[sess.UID].Apply(...)` (self); use `Value` as stacks, parse `Duration` for rounds |
| `"movement"` | Adjust `target.Position` by `Distance` feet in `Direction` (away = move target further from player position 0; floor 0) |
| `"utility"` | Append `Description` field as a message string (once, not per target) |

**Duration parsing:** `"rounds:N"` → N rounds; `"minutes:N"` → N×10 rounds; `"instant"` or empty → 1 round.

**`slowed` condition stacking:** `ap_penalty: 1` with `max_stacks: 2` means the penalty is multiplied by the current stack count — Slowed 1 = −1 AP, Slowed 2 = −2 AP. The combat turn-order logic reads `stacks * ap_penalty` when deducting AP at turn start.

---

## Feature 4: `handleUse` Update

File: `internal/gameserver/grpc_service.go`

### Signature change

```go
// handleUse processes a use request for the given ability.
// targetID is optional; empty string means "use combat default or self".
func (s *GameServiceServer) handleUse(uid, abilityID, targetID string) (*gamev1.ServerEvent, error)
```

All internal call sites updated. The dispatch function passes `msg.GetUse().GetTarget()` as `targetID`.

### Target resolution (hybrid)

After the tech is identified and before use is expended:

```
if tech.Targets == "self" OR tech.Resolution == "none":
    target = nil  (self/utility — no combat target needed)
else if in combat:
    if targetID != "":
        target = find Combatant by name in current combat (case-insensitive prefix match)
        if not found: return "No combatant named '<targetID>' in this fight."
    else:
        target = current combat target (the NPC the player is fighting)
        if no current target: return "Specify a target: use <tech> <target>"
else (not in combat):
    if targetID != "":
        return "You are not in combat."
    else if tech needs a target (damage/condition effects present):
        return "Specify a target: use <tech> <target>"
    else:
        target = nil  (utility/heal, no target needed out of combat)
```

### Effect resolution call

After use is expended (prepared slot / spontaneous pool / innate slot):

```go
targets := buildTargetSlice(techDef, target, combatEngine) // []*combat.Combatant
// s.rng satisfies combat.Source (interface { Intn(n int) int })
// s.dice satisfies dice.Roller (interface { RollExpr(expr string) (dice.Result, error) })
msgs := ResolveTechEffects(sess, techDef, targets, combatEngine, s.condRegistry, s.dice, s.rng)
// Build response from msgs
```

Where `buildTargetSlice` returns an empty slice for self/utility, a single-element slice for `TargetsSingle`, and all enemy combatants for `TargetsAllEnemies`. `combat.Source` is `interface { Intn(n int) int }` (defined in `internal/game/combat/resolver.go`) — `s.rng` implements it.

Return a `MessageEvent` containing all result lines joined by newline.

---

## Feature 5: Condition Definitions

New files in `content/conditions/`:

**`slowed.yaml`**
```yaml
id: slowed
name: Slowed
description: You lose actions each round due to impaired reactions. Slowed N = lose N AP per turn.
duration: rounds
max_stacks: 2
modifiers:
  ap_penalty: 1
```

**`immobilized.yaml`**
```yaml
id: immobilized
name: Immobilized
description: You cannot move. Your speed is reduced to 0.
duration: rounds
max_stacks: 1
modifiers:
  speed_penalty: 999
```

**`blinded.yaml`**
```yaml
id: blinded
name: Blinded
description: You cannot see. You take -4 to all attack rolls and targets are concealed from you.
duration: rounds
max_stacks: 1
modifiers:
  attack_penalty: 4
```

**`fleeing.yaml`**
```yaml
id: fleeing
name: Fleeing
description: You must use all your actions to move away from the source of your fear.
duration: rounds
max_stacks: 1
actions:
  forced_action: flee
```

---

## Feature 6: Technology YAML Updates

All 14 tech YAML files gain a `resolution:` field and tiered `effects:` block. `save_type` and `save_dc` remain at the top level of `TechnologyDef`.

### Neural tradition (`content/technologies/neural/`)

**`mind_spike.yaml`** — add/replace:
```yaml
resolution: save
save_type: cool
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: damage
      dice: 1d3
      damage_type: mental
  on_failure:
    - type: damage
      dice: 1d6
      damage_type: mental
  on_crit_failure:
    - type: damage
      dice: 1d6
      damage_type: mental
    - type: condition
      condition_id: stunned
      value: 1
      duration: rounds:1
amped_effects:
  on_crit_success: []
  on_success:
    - type: damage
      dice: 1d6
      damage_type: mental
  on_failure:
    - type: damage
      dice: 2d6
      damage_type: mental
  on_crit_failure:
    - type: damage
      dice: 2d6
      damage_type: mental
    - type: condition
      condition_id: stunned
      value: 1
      duration: rounds:1
```

**`neural_static.yaml`** — add/replace:
```yaml
resolution: save
save_type: hustle
save_dc: 15
effects:
  on_crit_success: []
  on_success: []
  on_failure:
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: slowed
      value: 2
      duration: rounds:1
amped_effects:
  on_crit_success: []
  on_success: []
  on_failure:
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:2
  on_crit_failure:
    - type: condition
      condition_id: slowed
      value: 2
      duration: rounds:2
```

**`synaptic_surge.yaml`** — add/replace:
```yaml
resolution: save
save_type: cool
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: frightened
      value: 1
      duration: rounds:1
  on_failure:
    - type: damage
      dice: 2d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
  on_crit_failure:
    - type: damage
      dice: 4d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 3
      duration: rounds:1
amped_effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: frightened
      value: 1
      duration: rounds:1
  on_failure:
    - type: damage
      dice: 4d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
  on_crit_failure:
    - type: damage
      dice: 6d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 3
      duration: rounds:1
```

### Innate tradition (`content/technologies/innate/`)

**`blackout_pulse.yaml`** — add/replace:
```yaml
targets: all_enemies
resolution: none
effects:
  on_apply:
    - type: condition
      condition_id: blinded
      value: 1
      duration: rounds:1
```
*(uses existing `TargetsAllEnemies = "all_enemies"` constant; the caller builds the full enemy combatant slice)*

**`arc_lights.yaml`** — add/replace:
```yaml
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Three hovering arc-light drones illuminate the area, shedding bright light in a wide radius."
```

**`pressure_burst.yaml`** — add/replace:
```yaml
resolution: attack
effects:
  on_miss: []
  on_hit:
    - type: damage
      dice: 3d6
      damage_type: bludgeoning
    - type: movement
      distance: 5
      direction: away
  on_crit_hit:
    - type: damage
      dice: 6d6
      damage_type: bludgeoning
    - type: movement
      distance: 10
      direction: away
```

**`nanite_infusion.yaml`** — add/replace:
```yaml
resolution: none
effects:
  on_apply:
    - type: heal
      dice: 1d8
      amount: 8
```

**`atmospheric_surge.yaml`** — add/replace:
```yaml
targets: all_enemies
resolution: save
save_type: toughness
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: utility
      description: "The surge buffets you but you hold your ground."
  on_failure:
    - type: condition
      condition_id: prone
      value: 1
  on_crit_failure:
    - type: damage
      dice: 2d6
      damage_type: bludgeoning
    - type: condition
      condition_id: prone
      value: 1
    - type: movement
      distance: 30
      direction: away
```

**`viscous_spray.yaml`** — add/replace:
```yaml
resolution: save
save_type: hustle
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:1
  on_failure:
    - type: condition
      condition_id: immobilized
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: immobilized
      value: 1
      duration: rounds:2
```

**`chrome_reflex.yaml`** — add/replace:
```yaml
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your chrome-augmented nervous system fires. You feel a momentary surge of neural clarity."
```

**`seismic_sense.yaml`** — add/replace:
```yaml
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your bone-conduction implants detect ground vibrations. You sense the movement of all creatures in the room through the floor."
```

**`moisture_reclaim.yaml`** — add/replace:
```yaml
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your condensation filters extract 2 gallons of potable water from the ambient air."
```

**`terror_broadcast.yaml`** — add/replace:
```yaml
targets: all_enemies
resolution: save
save_type: cool
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: frightened
      value: 1
  on_failure:
    - type: condition
      condition_id: frightened
      value: 2
  on_crit_failure:
    - type: condition
      condition_id: frightened
      value: 2
    - type: condition
      condition_id: fleeing
      value: 1
      duration: rounds:1
```

**`acid_spit.yaml`** — add/replace:
```yaml
resolution: attack
effects:
  on_miss: []
  on_hit:
    - type: damage
      dice: 1d6
      damage_type: acid
  on_crit_hit:
    - type: damage
      dice: 2d6
      damage_type: acid
```
*(persistent acid deferred — no DoT system)*

---

## Testing

All tests use TDD + property-based testing (SWENG-5, SWENG-5a).

### `internal/game/technology/model_test.go` — update existing

- **REQ-TER1**: `Validate()` rejects `resolution:"save"` with empty `SaveType`
- **REQ-TER2**: `Validate()` rejects `resolution:"save"` with `SaveDC == 0`
- **REQ-TER3**: `Validate()` accepts `resolution:"none"` with empty `SaveType`/zero `SaveDC`
- **REQ-TER4**: `Validate()` rejects `resolution:"attack"` with non-empty `SaveType`

### `internal/gameserver/tech_effect_resolver_test.go` — new (package `gameserver`)

- **REQ-TER5**: Save-based tech — `ResolveTechEffects` applies `OnFailure` effects when `ResolveSave` returns `Failure`
- **REQ-TER6**: Save-based tech — `ResolveTechEffects` applies no effects on `CritSuccess`
- **REQ-TER7**: Damage effect — `target.CurrentHP` decreases by rolled amount; never below 0
- **REQ-TER8**: Heal effect — `sess.CurrentHP` increases by rolled amount; never above `MaxHP`
- **REQ-TER9**: Condition effect — condition present in `sess.Conditions` after apply
- **REQ-TER10**: Movement effect — `target.Position` increases by `Distance` when direction is `"away"`; floored at 0
- **REQ-TER11**: Attack-based tech — `OnHit` effects applied on hit; `OnMiss` (empty) on miss
- **REQ-TER12** (property, `pgregory.net/rapid`): For any save-based tech, `OnCritSuccess` tier is never applied when outcome is `Failure` or `CritFailure`
- **REQ-TER13** (property): Damage output always within dice bounds (≥ min roll, ≤ max roll + flat amount)
- **REQ-TER14** (property): `target.CurrentHP` never goes negative across N random damage applications
- **REQ-TER21**: Area-targeting tech (`TargetsAllEnemies`) — `ResolveTechEffects` applies effects to every target in the slice; result messages contain one entry per target
- **REQ-TER22** (property): Area-targeting tech with N enemies — result message count equals N (one per target)

### `internal/gameserver/grpc_service_use_target_test.go` — new (package `gameserver`)

- **REQ-TER15**: `handleUse` with `targetID=""` not in combat, damage tech → returns "Specify a target"
- **REQ-TER16**: `handleUse` with `targetID="goblin"` not in combat → returns "You are not in combat."
- **REQ-TER17**: `handleUse` with self-targeting tech out of combat → resolves without error
- **REQ-TER18**: `handleUse` with `targetID=""` in combat → uses current combat target

### `internal/game/technology/registry_test.go` — extend

- **REQ-TER19**: All 14 tech YAML files load without error after model change
- **REQ-TER20**: Each loaded tech with `resolution:"save"` has non-empty `SaveType` and `SaveDC > 0`

---

## Requirements

- REQ-TER1–4: Model validation covers Resolution/SaveType/SaveDC combinations
- REQ-TER5–14: `ResolveTechEffects` correctly applies tiered effects for all resolution types
- REQ-TER15–18: `handleUse` hybrid target resolution works for all target scenarios
- REQ-TER19–20: All 14 tech YAMLs load cleanly with new model
- REQ-TER21–22: Area-targeting (`TargetsAllEnemies`) applies effects to all targets in the slice
- REQ-COND1: Four new condition definitions (slowed, immobilized, blinded, fleeing) load without error
- REQ-SCOPE1: No changes to Amped Technology, Reactions system, or passive effect system

---

## Files Changed

| Action | Path | Notes |
|--------|------|-------|
| Modify | `internal/game/technology/model.go` | Add `TieredEffects`; `Resolution` field; update `TechnologyDef` |
| Modify | `internal/game/technology/model_test.go` | Update validation tests for new model |
| Modify | `api/proto/game/v1/game.proto` | Add `target` field to `UseMessage` |
| Modify | `internal/frontend/handlers/bridge_handlers.go` | Pass target arg in `bridgeUse` |
| Create | `internal/gameserver/tech_effect_resolver.go` | `ResolveTechEffects` function |
| Create | `internal/gameserver/tech_effect_resolver_test.go` | REQ-TER5–14 |
| Modify | `internal/gameserver/grpc_service.go` | `handleUse` gains `targetID`; calls resolver |
| Create | `internal/gameserver/grpc_service_use_target_test.go` | REQ-TER15–18 |
| Modify | `internal/game/technology/registry_test.go` | REQ-TER19–20 |
| Modify | `content/technologies/neural/mind_spike.yaml` | Tiered effects |
| Modify | `content/technologies/neural/neural_static.yaml` | Tiered effects |
| Modify | `content/technologies/neural/synaptic_surge.yaml` | Tiered effects |
| Modify | `content/technologies/innate/blackout_pulse.yaml` | Tiered effects |
| Modify | `content/technologies/innate/arc_lights.yaml` | Tiered effects |
| Modify | `content/technologies/innate/pressure_burst.yaml` | Tiered effects |
| Modify | `content/technologies/innate/nanite_infusion.yaml` | Tiered effects |
| Modify | `content/technologies/innate/atmospheric_surge.yaml` | Tiered effects |
| Modify | `content/technologies/innate/viscous_spray.yaml` | Tiered effects |
| Modify | `content/technologies/innate/chrome_reflex.yaml` | Tiered effects |
| Modify | `content/technologies/innate/seismic_sense.yaml` | Tiered effects |
| Modify | `content/technologies/innate/moisture_reclaim.yaml` | Tiered effects |
| Modify | `content/technologies/innate/terror_broadcast.yaml` | Tiered effects |
| Modify | `content/technologies/innate/acid_spit.yaml` | Tiered effects |
| Create | `content/conditions/slowed.yaml` | New condition |
| Create | `content/conditions/immobilized.yaml` | New condition |
| Create | `content/conditions/blinded.yaml` | New condition |
| Create | `content/conditions/fleeing.yaml` | New condition |
| Modify | `docs/requirements/FEATURES.md` | Mark effect resolution items complete |
