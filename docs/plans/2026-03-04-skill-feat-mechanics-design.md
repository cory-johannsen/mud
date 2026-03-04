# Skill & Feat Mechanical Effects — Design Document

**Date:** 2026-03-04
**Status:** Approved

---

## Overview

Implement mechanical effects for skills, feats, and class features. This covers:
1. Automatic skill check triggers in exploration/interaction contexts
2. Active feat and class feature effects via the condition system
3. Passive feat and class feature effects checked at roll time

Skills in combat (Grapple, Hide mid-fight) are deferred to a future stage.

---

## Section 1: Core Skill Check Framework

### Formula

```
roll = d20 + ability_mod + proficiency_bonus
```

Proficiency bonus table:

| Rank       | Bonus |
|------------|-------|
| untrained  | +0    |
| trained    | +2    |
| expert     | +4    |
| master     | +6    |
| legendary  | +8    |

### Outcome Tiers (matches existing combat system)

| Outcome        | Condition         |
|----------------|-------------------|
| Critical Success | roll ≥ DC + 10  |
| Success          | roll ≥ DC       |
| Failure          | roll < DC       |
| Critical Failure | roll < DC − 10  |

### New Types

```go
// internal/game/skill/check.go

type CheckOutcome int

const (
    CritSuccess CheckOutcome = iota
    Success
    Failure
    CritFailure
)

type SkillCheck struct {
    SkillID     string
    CharacterID int64
    UID         string
    DC          int
    Context     string // human-readable label, e.g., "picking the lock"
}

type CheckResult struct {
    Check   SkillCheck
    Roll    int   // raw d20
    Total   int   // roll + ability_mod + proficiency_bonus
    Outcome CheckOutcome
}

type Resolver struct {
    skills    map[string]*ruleset.Skill   // id → Skill
    dice      dice.Source
    scriptMgr scripting.Manager
}

func (r *Resolver) Resolve(ctx context.Context, session *session.Session, check SkillCheck) (CheckResult, error)
```

### Lua Hook

After Go resolves the check, the Lua hook fires:

```lua
-- scripts/skill_checks/<skill_id>.lua (optional override)
function on_skill_check(uid, skill_id, roll_total, dc, outcome)
    -- return nil to accept Go result
    -- return "crit_success"|"success"|"failure"|"crit_failure" to override
end
```

---

## Section 2: Trigger System

Skill check triggers are declared in existing YAML content files.

### Room Triggers (`content/zones/<zone>/<room>.yaml`)

```yaml
skill_checks:
  - skill: parkour
    dc: 14
    trigger: on_enter
    outcomes:
      crit_success:
        message: "You vault the rubble effortlessly."
      success:
        message: "You pick your way through carefully."
      failure:
        message: "You stumble badly."
        effect:
          type: damage
          formula: "1d4"
      crit_failure:
        message: "You fall hard."
        effect:
          type: damage
          formula: "2d4"
```

Supported room triggers: `on_enter`

### NPC Triggers (`content/npcs/<npc>.yaml`)

```yaml
skill_checks:
  - skill: smooth_talk
    dc: 16
    trigger: on_greet
    outcomes:
      success:
        message: "They warm up to you immediately."
      failure:
        message: "They regard you with suspicion."
        effect:
          type: condition
          id: distrusted
```

Supported NPC triggers: `on_greet`

### Item Triggers (`content/items/<item>.yaml`)

```yaml
skill_checks:
  - skill: grift
    dc: 12
    trigger: on_use
    outcomes:
      success:
        message: "The lock clicks open."
        effect:
          type: reveal
          target: room_exit_north
      failure:
        message: "The lock holds."
        effect:
          type: deny
      crit_failure:
        message: "You break your pick in the lock."
        effect:
          type: deny
```

Supported item triggers: `on_use`

### Supported Effect Types

| Type        | Description                                      |
|-------------|--------------------------------------------------|
| `damage`    | Deal HP damage using a dice formula              |
| `condition` | Apply a named condition (existing system)        |
| `deny`      | Block the action (door stays locked, etc.)       |
| `reveal`    | Expose a hidden room feature, exit, or item      |

### Lua Outcome Hook

```lua
function on_skill_check_outcome(uid, skill_id, outcome, effect_type)
    -- custom side effects; return false to suppress default effect
end
```

---

## Section 3: Trigger Registration

The `SkillCheckTriggerLoader` reads YAML at server startup and registers trigger handlers:

- Room triggers → registered in `RoomRegistry`; fired by the movement handler when a player enters a room
- NPC triggers → registered in `NPCRegistry`; fired by the NPC greeting/interaction handler
- Item triggers → registered in `ItemRegistry`; fired by the `use` command handler

No new file types. All YAML additions are backward-compatible (optional blocks).

---

## Section 4: Active Feat & Class Feature Effects

When `use <feat_or_feature>` is invoked:

1. Server looks up the feat/class feature in the registry
2. Applies a **named condition** to the character using the existing condition system
3. The condition carries mechanical deltas (`damage_bonus`, `ac_penalty`, etc.)
4. Duration: `encounter` (cleared on combat end) or `timed` (N rounds)

### Example Condition File

```yaml
# content/conditions/brutal_surge_active.yaml
id: brutal_surge_active
name: Brutal Surge Active
damage_bonus: 2
ac_penalty: -2
duration: encounter
description: "You're in a combat frenzy: +2 melee damage, -2 AC."
```

### Mapping: Feat/Feature → Condition ID

Each active feat/class feature in `feats.yaml` / `class_features.yaml` gains a `condition_id` field:

```yaml
- id: brutal_surge
  name: Brutal Surge
  active: true
  condition_id: brutal_surge_active   # NEW FIELD
  activate_text: "The red haze drops and you move on pure instinct."
```

For feats/features with non-standard effects, the Lua hook fires instead:

```lua
function on_feat_activate(uid, feat_id)
    -- custom logic (e.g., inspire_courage for allies)
end
```

---

## Section 5: Passive Feat & Class Feature Effects

Always-on effects are checked at roll time in the combat resolver and skill check resolver.

### Implementation

Each passive feat/class feature maps to a **checker interface** in Go:

```go
// internal/game/feat/passive.go
type PassiveChecker interface {
    Applies(ctx PassiveContext) bool
    Apply(result *RollResult)
}

type PassiveContext struct {
    CharacterID int64
    Feats       []string   // character's feat IDs
    Features    []string   // character's class feature IDs
    AttackType  string     // "melee", "ranged", "stealth", etc.
    IsHidden    bool       // attacker was hidden
}
```

Registered passive checkers (initial set):
- `sucker_punch` — adds sneak attack damage when attacking from stealth
- `street_brawler` — triggers attack of opportunity when enemy leaves threat range
- `predators_eye` — bonus to first attack vs unaware target
- `zone_awareness` — removes difficult terrain movement penalty

Lua hook for custom passive logic:

```lua
function on_passive_feat_check(uid, feat_id, context)
    -- return bonus (number) or nil
end
```

---

## Implementation Scope

This design is large. Implementation is staged:

**Stage 1:** Core skill check framework (resolver, proficiency formula, Lua hook) + room `on_enter` trigger only
**Stage 2:** NPC `on_greet` + item `on_use` triggers
**Stage 3:** Active feat/class feature → condition mapping
**Stage 4:** Passive feat/class feature checkers

Each stage is independently deployable.
