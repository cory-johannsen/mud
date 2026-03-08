# Actions System Design

**Date:** 2026-03-07
**Status:** Approved

---

## Goal

Implement a generic, data-driven Actions system that lets players activate archetype- and job-specific abilities. Actions consume action points in combat (existing economy), are context-gated outside combat, and are invoked via a single `action` command plus per-action shortcut aliases.

## Architecture

The Actions system extends the existing `ClassFeature` infrastructure. No new DB tables, no new registry. Active class features (`active: true`) become Actions — the distinction is purely in the YAML fields.

---

## Section 1: Data Model — ClassFeature Schema Extensions

Five new fields added to active entries in `content/class_features.yaml`:

```yaml
- id: brutal_surge
  name: Brutal Surge
  archetype: aggressor
  job: ""
  pf2e: rage
  active: true
  shortcut: surge            # direct command alias
  action_cost: 1             # 1, 2, or 3 action points consumed in combat
  contexts:                  # where the action is valid
    - combat                 # "combat" | "exploration" | "downtime"
  activate_text: "The red haze drops and you move on pure instinct."
  effect:
    type: condition          # condition | heal | damage | skill_check
    target: self             # self | target
    condition_id: brutal_surge_active
  description: "Enter a combat frenzy: +2 melee damage, -2 AC until end of encounter."
```

### Effect Type Vocabulary

| `type` | Required fields | Meaning |
|---|---|---|
| `condition` | `target`, `condition_id` | Apply a condition to self or a named target |
| `heal` | `amount` (dice string or flat int) | Restore HP to self |
| `damage` | `target`, `amount`, `damage_type` | Deal damage to target |
| `skill_check` | `skill`, `dc` | Trigger a skill check |

Passive class features (`active: false`) require no new fields.

### Go Struct Extensions

```go
// ActionEffect describes the mechanical outcome of activating an action.
type ActionEffect struct {
    Type        string `yaml:"type"`         // condition | heal | damage | skill_check
    Target      string `yaml:"target"`       // self | target
    ConditionID string `yaml:"condition_id"` // for type=condition
    Amount      string `yaml:"amount"`       // for type=heal|damage (dice string or flat int)
    DamageType  string `yaml:"damage_type"`  // for type=damage
    Skill       string `yaml:"skill"`        // for type=skill_check
    DC          int    `yaml:"dc"`           // for type=skill_check
}

// ClassFeature extended fields:
// Shortcut   string        `yaml:"shortcut"`
// ActionCost int           `yaml:"action_cost"`
// Contexts   []string      `yaml:"contexts"`
// Effect     *ActionEffect `yaml:"effect"`
```

---

## Section 2: Action Registry & Per-Character Availability

- **No new registry.** `ClassFeatureRegistry.GetByID(id)` already provides lookup.
- **Per-character availability** is derived at invocation time by intersecting `sess.PassiveFeats` (map of feature IDs the player has) with registry entries where `active == true`.
- A helper `availableActions(sess, registry, context string) []*ClassFeature` filters to features owned by the player that are valid in the current context.

---

## Section 3: The `action` Command + Shortcut Auto-Registration

### The `action` command

- `action` (no args) — lists all actions valid in current context:
  ```
  Available Actions:
    surge  [1 action]  Brutal Surge — Enter a combat frenzy...
    patch  [2 actions] Patch Job    — Spend materials to restore 1d6+2 HP...
  ```
- `action <name>` — activates by ID or name prefix
- `action <name> <target>` — for target-requiring effects

### Shortcut auto-registration

Each active class feature with a non-empty `shortcut` is registered as a command alias at server boot. `surge` behaves identically to `action surge`. Shortcut collision at registration time causes a startup panic with a clear message.

### Command wiring (CMD-1 → CMD-7)

- `HandlerAction` constant in `commands.go`
- `Command{...}` entry in `BuiltinCommands()`
- `HandleAction` in `internal/game/command/action.go`
- Proto: `ActionRequest { string name = 1; string target = 2; }` in `ClientMessage` oneof
- `bridgeAction` in `bridge_handlers.go`
- `handleAction` in `grpc_service.go` → `ActionHandler`
- Per-shortcut: `Handler<Shortcut>` constant + `Command` entry + bridge wiring, all delegating to `handleAction` with action ID pre-filled

---

## Section 4: Combat Integration

When `handleAction` is called with `sess.Status == IN_COMBAT`:

1. Look up `action_cost` from registry
2. Submit `ActionCombatAction` to `CombatHandler` (new action type alongside `ActionAttack`, `ActionStrike`, `ActionPass`)
3. `ActionQueue` deducts `action_cost` — insufficient points → error: `"Not enough actions — surge costs 1 action (you have 0 remaining)"`
4. During `resolveAndAdvance`, effect resolved in initiative order
5. Narrative broadcast: `"[Name] activates Brutal Surge — The red haze drops..."`

---

## Section 5: Effect Resolution

New `ActionEffectResolver` in `internal/game/action/resolver.go`:

### `condition` — apply to self or target
- Self: `condRegistry.Apply(condID, sess)` — same path as combat conditions
- Target (NPC): apply to NPC instance active condition set
- Narrative pushed to player console + broadcast to room

### `heal` — restore HP to self
- `amount` parsed by `dice.Roller` (e.g. `"1d6+2"` or `"5"`)
- `newHP = min(sess.MaxHP, sess.CurrentHP + rolled)`
- Persisted via `charSaver.SaveState`
- `HpUpdateEvent` pushed to refresh prompt bar
- Narrative: `"You patch yourself up for 5 HP. (18/20)"`

### `damage` — deal damage to target
- Resolved through existing `applyResistanceWeakness`
- Generates `CombatEvent` broadcast to room
- NPC HP updated; death/loot handled by existing post-resolution logic

### `skill_check` — trigger a skill check
- Delegates to existing `skillcheck` package with `skill` and `dc` from effect
- Result narrative pushed to player console (pass/fail text)
- Future: chained secondary effects on pass/fail

---

## Section 6: Context Validation & Error Handling

| Player state | Valid contexts |
|---|---|
| `Status == IN_COMBAT` | `combat` only |
| `Status == IDLE` | `exploration`, `downtime` |
| `Status == UNCONSCIOUS` | none |

### Error messages
- Wrong context (in combat): `"You can't do that in the middle of a fight."`
- Wrong context (out of combat): `"That action is only available in combat."`
- Not in player's feature set: `"You don't know that action."`
- Insufficient action points: `"Not enough actions — surge costs 1 action (you have 0 remaining)."`
- Target required but missing: `"Usage: <shortcut> <target>"`
- Target not found: `"You don't see that here."`

---

## Implementation Scope (Phase 1)

Phase 1 implements the full system with `brutal_surge` as the reference action. Additional archetype/job actions are added in subsequent small batches per FEATURES.md.

**Files touched:**
- `content/class_features.yaml` — add new fields to existing active features
- `internal/game/ruleset/class_feature.go` — extend `ClassFeature` struct + `ActionEffect` struct
- `internal/game/command/commands.go` — `HandlerAction` + per-shortcut constants
- `internal/game/command/action.go` — `HandleAction` (new file)
- `api/proto/game/v1/game.proto` — `ActionRequest` message
- `internal/frontend/handlers/bridge_handlers.go` — `bridgeAction`
- `internal/gameserver/action_handler.go` — `ActionHandler` + `ActionEffectResolver` (new file)
- `internal/gameserver/grpc_service.go` — `handleAction` dispatch
- `internal/game/combat/combat.go` — `ActionCombatAction` type
