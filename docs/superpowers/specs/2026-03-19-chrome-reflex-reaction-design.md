# Chrome Reflex Reaction Integration Design

**Date:** 2026-03-19
**Feature:** Wire `chrome_reflex` innate tech into the Reactions system (Sub-project 2 of 3)

---

## Goal

Convert `chrome_reflex` from a manually-activated innate tech (`use chrome_reflex`) into a player reaction that fires automatically when the player fails a saving throw. Depends on the Reactions System infrastructure (Sub-project 1).

---

## Scope

In scope: extend `ReactionDef` to support multiple trigger types; add `reaction:` block to `chrome_reflex.yaml`; block manual `use` activation for techs with a reaction definition; update login registration and registry to handle multi-trigger reactions.

Out of scope: other reaction-bearing feats or techs (Sub-project 3); NPC reactions; `ReactionEffectStrike` implementation.

---

## Data Model

### REQ-CRX1
`ReactionDef` in `internal/game/reaction/trigger.go` MUST replace the `Trigger ReactionTriggerType` field with `Triggers []ReactionTriggerType` tagged `yaml:"triggers"`. The singular `Trigger` field MUST be removed.

### REQ-CRX2
`ReactionDef.Triggers` MUST contain at least one entry. A `ReactionDef` with an empty `Triggers` slice is invalid and MUST be treated as having no registered triggers (no-op at registration time).

### REQ-CRX3
`content/technologies/innate/chrome_reflex.yaml` MUST gain a `reaction:` block:
```yaml
reaction:
  triggers:
    - on_save_fail
    - on_save_crit_fail
  effect:
    type: reroll_save
    keep: better
```

The existing `effects.on_apply` utility entry MAY be removed since chrome_reflex no longer has a manual activation path.

---

## Registry

### REQ-CRX4
`ReactionRegistry.Register` in `internal/game/reaction/registry.go` MUST iterate `def.Triggers` and insert one `PlayerReaction` entry per trigger type. A single `chrome_reflex` registration produces two entries: one under `TriggerOnSaveFail` and one under `TriggerOnSaveCritFail`.

### REQ-CRX5
Because `ReactionsRemaining` is checked before any reaction fires, a player can spend chrome_reflex at most once per round regardless of how many trigger types are registered. This invariant requires no additional code beyond what Sub-project 1 already implements.

---

## Use Command Block

### REQ-CRX6
In `internal/gameserver/grpc_service.go`, inside the innate tech activation path (`handleUse` or equivalent), after looking up the `TechnologyDef` for the requested tech ID, MUST check whether `techDef.Reaction != nil`. If so, MUST send the player an informational message:

> `"<tech name> fires automatically as a reaction and cannot be activated manually."`

and return without activating the tech. The `use` command MUST NOT decrement uses or call `activateTechWithEffects` for any tech that declares a reaction.

---

## Login Registration

### REQ-CRX7
The existing login loop that iterates `sess.InnateTechs` and calls `sess.Reactions.Register(uid, techID, techDef.Name, *techDef.Reaction)` requires no structural change. Once `Register` iterates `def.Triggers` (REQ-CRX4), chrome_reflex is automatically registered for both trigger types.

---

## Testing

### REQ-CRX8
Unit tests in `internal/game/reaction/trigger_test.go` MUST be updated to use `Triggers []ReactionTriggerType` (replacing `Trigger`). YAML round-trip tests MUST verify that a `ReactionDef` with two triggers marshals and unmarshals correctly.

### REQ-CRX9
Unit tests in `internal/game/reaction/registry_test.go` MUST be updated to pass `Triggers` in `ReactionDef` literals. A new test MUST verify that `Register` with two triggers produces entries retrievable by both trigger types for the same UID.

### REQ-CRX10
A unit test MUST verify that `chrome_reflex.yaml` loads without error and that the parsed `TechnologyDef.Reaction` is non-nil with `Triggers` containing both `TriggerOnSaveFail` and `TriggerOnSaveCritFail`.

### REQ-CRX11
A unit test or integration test MUST verify that attempting to `use chrome_reflex` (or any tech with a non-nil `Reaction`) returns the "fires automatically as a reaction" message without activating the tech.

### REQ-CRX12
`go test ./internal/game/reaction/... ./internal/game/technology/... ./internal/gameserver/...` MUST pass after all changes.

---

## Architecture

### File Map

| File | Change |
|---|---|
| `internal/game/reaction/trigger.go` | Replace `Trigger` with `Triggers []ReactionTriggerType` |
| `internal/game/reaction/trigger_test.go` | Update YAML round-trip tests to use `Triggers` |
| `internal/game/reaction/registry.go` | `Register` iterates `def.Triggers` |
| `internal/game/reaction/registry_test.go` | Update test literals; add multi-trigger test |
| `content/technologies/innate/chrome_reflex.yaml` | Add `reaction:` block |
| `internal/gameserver/grpc_service.go` | Block `use` activation for reaction techs (REQ-CRX6) |
| `internal/gameserver/reaction_handler_test.go` | Update any `Trigger` field references |

### Data Flow

```
Login
  └─ iterate sess.InnateTechs
       └─ chrome_reflex: techDef.Reaction != nil
            └─ Register(uid, "chrome_reflex", "Chrome Reflex", def)
                 └─ for trigger in def.Triggers:
                      └─ byTrigger[trigger] = append(...)

Combat round — player fails save
  └─ fireReaction(uid, TriggerOnSaveFail, ctx)
       └─ buildReactionCallback:
            └─ ReactionsRemaining > 0 ✓
            └─ registry.Get(uid, TriggerOnSaveFail) → chrome_reflex ✓
            └─ CheckReactionRequirement("") → true ✓
            └─ prompt: "Reaction available: Chrome Reflex — you failed a saving throw. Use it? (yes / no)"
            └─ yes → ReactionsRemaining--; ApplyReactionEffect(reroll_save)

use chrome_reflex (manual attempt)
  └─ techDef.Reaction != nil
       └─ return "Chrome Reflex fires automatically as a reaction and cannot be activated manually."
```
