# Actions System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a generic, data-driven Actions system that lets players activate archetype- and job-specific abilities via an `action` command and per-action shortcut aliases, consuming action points in combat or context-gating out of combat.

**Architecture:** Extend the existing `ClassFeature` struct with five new fields (shortcut, action_cost, contexts, activate_text already exists, effect block) and add an `ActionEffect` struct. Wire a new `action` command end-to-end through the full CMD-1–CMD-7 pipeline. Effect resolution lives in `internal/gameserver/action_handler.go`. The reference action is `brutal_surge` (aggressor archetype).

**Tech Stack:** Go, YAML (class_features.yaml), protobuf (game.proto), gRPC, existing ClassFeatureRegistry, existing ActionQueue/combat package.

**Design doc:** `docs/plans/2026-03-07-actions-system-design.md`

---

### Task 1: Extend ClassFeature Struct with ActionEffect and New Fields

**Files:**
- Modify: `internal/game/ruleset/class_feature.go`
- Modify: `content/class_features.yaml` (brutal_surge entry only)
- Test: `internal/game/ruleset/class_feature_test.go` (create if absent)

**Step 1: Write the failing tests**

Create `internal/game/ruleset/class_feature_test.go` (or add to existing):

```go
package ruleset_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadClassFeaturesFromBytes_ActiveFields(t *testing.T) {
    yaml := []byte(`
class_features:
  - id: brutal_surge
    name: Brutal Surge
    archetype: aggressor
    job: ""
    pf2e: rage
    active: true
    shortcut: surge
    action_cost: 1
    contexts:
      - combat
    activate_text: "The red haze drops."
    condition_id: brutal_surge_active
    description: "Enter a frenzy."
    effect:
      type: condition
      target: self
      condition_id: brutal_surge_active
`)
    features, err := ruleset.LoadClassFeaturesFromBytes(yaml)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(features) != 1 {
        t.Fatalf("expected 1 feature, got %d", len(features))
    }
    f := features[0]
    if f.Shortcut != "surge" {
        t.Errorf("Shortcut: got %q, want %q", f.Shortcut, "surge")
    }
    if f.ActionCost != 1 {
        t.Errorf("ActionCost: got %d, want 1", f.ActionCost)
    }
    if len(f.Contexts) != 1 || f.Contexts[0] != "combat" {
        t.Errorf("Contexts: got %v, want [combat]", f.Contexts)
    }
    if f.Effect == nil {
        t.Fatal("Effect must not be nil for active feature")
    }
    if f.Effect.Type != "condition" {
        t.Errorf("Effect.Type: got %q, want %q", f.Effect.Type, "condition")
    }
    if f.Effect.Target != "self" {
        t.Errorf("Effect.Target: got %q, want %q", f.Effect.Target, "self")
    }
    if f.Effect.ConditionID != "brutal_surge_active" {
        t.Errorf("Effect.ConditionID: got %q, want %q", f.Effect.ConditionID, "brutal_surge_active")
    }
}

func TestActionEffect_AllTypes(t *testing.T) {
    yaml := []byte(`
class_features:
  - id: heal_action
    name: Patch Job
    archetype: ""
    job: medic
    pf2e: treat_wounds
    active: true
    shortcut: patch
    action_cost: 2
    contexts:
      - combat
      - exploration
    activate_text: "You patch yourself up."
    description: "Restore HP."
    effect:
      type: heal
      amount: "1d6+2"
  - id: skill_action
    name: Assess
    archetype: ""
    job: scout
    pf2e: recall_knowledge
    active: true
    shortcut: assess
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You assess the situation."
    description: "Skill check."
    effect:
      type: skill_check
      skill: awareness
      dc: 15
`)
    features, err := ruleset.LoadClassFeaturesFromBytes(yaml)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(features) != 2 {
        t.Fatalf("expected 2, got %d", len(features))
    }
    heal := features[0]
    if heal.Effect == nil || heal.Effect.Type != "heal" {
        t.Errorf("heal: wrong effect type")
    }
    if heal.Effect.Amount != "1d6+2" {
        t.Errorf("heal.Amount: got %q", heal.Effect.Amount)
    }
    skill := features[1]
    if skill.Effect == nil || skill.Effect.Type != "skill_check" {
        t.Errorf("skill: wrong effect type")
    }
    if skill.Effect.DC != 15 {
        t.Errorf("skill.DC: got %d", skill.Effect.DC)
    }
}

func TestClassFeatureRegistry_ActiveOnly(t *testing.T) {
    features := []*ruleset.ClassFeature{
        {ID: "passive_feat", Active: false},
        {ID: "surge", Active: true, Shortcut: "surge", ActionCost: 1, Contexts: []string{"combat"}},
    }
    reg := ruleset.NewClassFeatureRegistry(features)
    active := reg.ActiveFeatures()
    if len(active) != 1 {
        t.Errorf("expected 1 active feature, got %d", len(active))
    }
    if active[0].ID != "surge" {
        t.Errorf("wrong active feature: %s", active[0].ID)
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/ruleset/... -run "TestLoadClassFeaturesFromBytes_ActiveFields|TestActionEffect_AllTypes|TestClassFeatureRegistry_ActiveOnly" -v
```
Expected: FAIL — fields don't exist yet.

**Step 3: Implement the changes**

In `internal/game/ruleset/class_feature.go`, add:

```go
// ActionEffect describes the mechanical outcome of activating an action.
//
// Precondition: Type must be one of "condition", "heal", "damage", "skill_check".
type ActionEffect struct {
    Type        string `yaml:"type"`         // condition | heal | damage | skill_check
    Target      string `yaml:"target"`       // self | target
    ConditionID string `yaml:"condition_id"` // for type=condition
    Amount      string `yaml:"amount"`       // for type=heal|damage (dice string or flat int)
    DamageType  string `yaml:"damage_type"`  // for type=damage
    Skill       string `yaml:"skill"`        // for type=skill_check
    DC          int    `yaml:"dc"`           // for type=skill_check
}
```

Extend `ClassFeature` struct with four new fields after `ConditionID`:

```go
    Shortcut   string        `yaml:"shortcut"`    // direct command alias; empty = no shortcut
    ActionCost int           `yaml:"action_cost"` // AP cost in combat; 1, 2, or 3
    Contexts   []string      `yaml:"contexts"`    // valid contexts: combat, exploration, downtime
    Effect     *ActionEffect `yaml:"effect"`      // nil for passive features
```

Add `ActiveFeatures()` method to `ClassFeatureRegistry`:

```go
// ActiveFeatures returns all features that are active (player-activated).
//
// Postcondition: Returns a slice of all active features; may be empty.
func (r *ClassFeatureRegistry) ActiveFeatures() []*ClassFeature {
    var out []*ClassFeature
    for _, f := range r.byID {
        if f.Active {
            out = append(out, f)
        }
    }
    return out
}
```

**Step 4: Update brutal_surge in content/class_features.yaml**

Replace the existing brutal_surge entry:

```yaml
  - id: brutal_surge
    name: Brutal Surge
    archetype: aggressor
    job: ""
    pf2e: rage
    active: true
    shortcut: surge
    action_cost: 1
    contexts:
      - combat
    activate_text: "The red haze drops and you move on pure instinct."
    condition_id: brutal_surge_active
    description: "Enter a combat frenzy: +2 melee damage, -2 AC until end of encounter."
    effect:
      type: condition
      target: self
      condition_id: brutal_surge_active
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/game/ruleset/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/game/ruleset/class_feature.go internal/game/ruleset/class_feature_test.go content/class_features.yaml
git commit -m "feat: extend ClassFeature with ActionEffect, shortcut, action_cost, contexts fields"
```

---

### Task 2: Add ActionUseAbility to Combat Package

**Files:**
- Modify: `internal/game/combat/action.go`
- Test: `internal/game/combat/action_test.go` (add to existing)

**Step 1: Write the failing test**

Add to `internal/game/combat/action_test.go`:

```go
func TestActionUseAbility_Cost(t *testing.T) {
    tests := []struct {
        cost     int
        expected int
    }{
        {1, 1},
        {2, 2},
        {3, 3},
    }
    for _, tt := range tests {
        qa := combat.QueuedAction{Type: combat.ActionUseAbility, AbilityID: "surge", AbilityCost: tt.cost}
        q := combat.NewActionQueue("player1", 3)
        if err := q.Enqueue(qa); err != nil {
            t.Errorf("cost=%d: unexpected Enqueue error: %v", tt.cost, err)
        }
        if q.RemainingPoints() != 3-tt.cost {
            t.Errorf("cost=%d: remaining=%d, want %d", tt.cost, q.RemainingPoints(), 3-tt.cost)
        }
    }
}

func TestActionUseAbility_InsufficientAP(t *testing.T) {
    q := combat.NewActionQueue("player1", 1)
    qa := combat.QueuedAction{Type: combat.ActionUseAbility, AbilityID: "surge", AbilityCost: 2}
    if err := q.Enqueue(qa); err == nil {
        t.Error("expected insufficient AP error, got nil")
    }
}

func TestActionUseAbility_String(t *testing.T) {
    if combat.ActionUseAbility.String() != "use_ability" {
        t.Errorf("String(): got %q, want %q", combat.ActionUseAbility.String(), "use_ability")
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/game/combat/... -run "TestActionUseAbility" -v
```
Expected: FAIL — ActionUseAbility doesn't exist.

**Step 3: Implement the changes**

In `internal/game/combat/action.go`:

Add `ActionUseAbility` to the const iota block (after `ActionThrow`):

```go
    ActionUseAbility                        // costs AbilityCost AP; activate a class ability
```

Add `AbilityCost` case to `Cost()`:

```go
    case ActionUseAbility:
        return 0 // cost comes from QueuedAction.AbilityCost
```

Add `use_ability` case to `String()`:

```go
    case ActionUseAbility:
        return "use_ability"
```

Add `AbilityID` and `AbilityCost` fields to `QueuedAction`:

```go
    AbilityID   string // for ActionUseAbility; the ClassFeature ID
    AbilityCost int    // for ActionUseAbility; AP cost from ClassFeature.ActionCost
```

Update `Enqueue` to handle `ActionUseAbility` cost:

In `Enqueue`, before the generic `cost > q.remaining` check, add:

```go
    if a.Type == ActionUseAbility {
        cost = a.AbilityCost
    }
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/game/combat/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/game/combat/action.go internal/game/combat/action_test.go
git commit -m "feat: add ActionUseAbility type with dynamic AP cost to combat package"
```

---

### Task 3: Proto — ActionRequest Message

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto` to regenerate

**Step 1: Add ActionRequest to proto**

In `api/proto/game/v1/game.proto`, add the new message (place near other request messages):

```protobuf
message ActionRequest {
  string name   = 1; // action ID or shortcut alias
  string target = 2; // target NPC name; empty if not required
}
```

Add it to the `ClientMessage` oneof:

```protobuf
    ActionRequest action = <next_number>;
```

Find the current highest oneof field number by reading the proto file and use the next integer.

**Step 2: Regenerate**

```bash
cd /home/cjohannsen/src/mud
make proto
```
Expected: no errors; `internal/gameserver/gamev1/game.pb.go` updated.

**Step 3: Verify the generated code compiles**

```bash
go build ./...
```
Expected: PASS

**Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat: add ActionRequest proto message to ClientMessage oneof"
```

---

### Task 4: CMD-1 & CMD-2 — Handler Constant and BuiltinCommands Entry

**Files:**
- Modify: `internal/game/command/commands.go`
- Test: `internal/game/command/commands_test.go` (add if needed)

**Step 1: Write the failing test**

Add to `internal/game/command/commands_test.go`:

```go
func TestHandlerAction_InBuiltinCommands(t *testing.T) {
    cmds := command.BuiltinCommands()
    var found bool
    for _, c := range cmds {
        if c.Handler == command.HandlerAction {
            found = true
            if c.Name != "action" {
                t.Errorf("action command name: got %q, want %q", c.Name, "action")
            }
        }
    }
    if !found {
        t.Error("HandlerAction not found in BuiltinCommands")
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/command/... -run "TestHandlerAction_InBuiltinCommands" -v
```
Expected: FAIL

**Step 3: Add constant and command**

In `internal/game/command/commands.go`:

Add to the Handler constants block:
```go
    HandlerAction Handler = "action"
```

Add to `BuiltinCommands()` return slice:
```go
    {Name: "action", Handler: HandlerAction, Description: "Activate an archetype or job action. Usage: action [name] [target]"},
```

**Step 4: Run to verify pass**

```bash
go test ./internal/game/command/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/game/command/commands.go internal/game/command/commands_test.go
git commit -m "feat: add HandlerAction constant and BuiltinCommands entry for action command"
```

---

### Task 5: CMD-3 — HandleAction Function

**Files:**
- Create: `internal/game/command/action.go`
- Create: `internal/game/command/action_test.go`

**Step 1: Write the failing tests**

Create `internal/game/command/action_test.go`:

```go
package command_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleAction_NoArgs(t *testing.T) {
    req, err := command.HandleAction([]string{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if req.Name != "" {
        t.Errorf("Name: got %q, want empty", req.Name)
    }
    if req.Target != "" {
        t.Errorf("Target: got %q, want empty", req.Target)
    }
}

func TestHandleAction_NameOnly(t *testing.T) {
    req, err := command.HandleAction([]string{"surge"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if req.Name != "surge" {
        t.Errorf("Name: got %q, want %q", req.Name, "surge")
    }
    if req.Target != "" {
        t.Errorf("Target: got %q, want empty", req.Target)
    }
}

func TestHandleAction_NameAndTarget(t *testing.T) {
    req, err := command.HandleAction([]string{"slam", "Guard"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if req.Name != "slam" {
        t.Errorf("Name: got %q, want %q", req.Name, "slam")
    }
    if req.Target != "Guard" {
        t.Errorf("Target: got %q, want %q", req.Target, "Guard")
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/command/... -run "TestHandleAction" -v
```
Expected: FAIL

**Step 3: Implement HandleAction**

Create `internal/game/command/action.go`:

```go
package command

// ActionRequest is the parsed form of the action command.
//
// Precondition: Name may be empty (list mode); Target is empty when not required.
type ActionRequest struct {
    Name   string // action ID or shortcut; empty means list available actions
    Target string // target NPC name; empty if not required
}

// HandleAction parses the arguments for the "action" command.
//
// Precondition: args is the slice of words following "action" (may be empty).
// Postcondition: Returns a non-nil *ActionRequest and nil error in all valid cases.
func HandleAction(args []string) (*ActionRequest, error) {
    req := &ActionRequest{}
    if len(args) >= 1 {
        req.Name = args[0]
    }
    if len(args) >= 2 {
        req.Target = args[1]
    }
    return req, nil
}
```

**Step 4: Run to verify pass**

```bash
go test ./internal/game/command/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/game/command/action.go internal/game/command/action_test.go
git commit -m "feat: implement HandleAction command parser"
```

---

### Task 6: CMD-5 — bridgeAction and bridgeHandlerMap Registration

**Files:**
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Test: `TestAllCommandHandlersAreWired` must pass (existing test)

**Step 1: Read the existing bridgeHandlerMap structure**

Read `internal/frontend/handlers/bridge_handlers.go` lines 1–60 to understand the `bridgeHandlerFunc` signature and map registration pattern.

**Step 2: Write the failing test check**

The existing test `TestAllCommandHandlersAreWired` in `internal/frontend/handlers/bridge_handlers_test.go` will fail as soon as `HandlerAction` is in `BuiltinCommands()` but not in `bridgeHandlerMap`. Verify:

```bash
go test ./internal/frontend/handlers/... -run "TestAllCommandHandlersAreWired" -v
```
Expected: FAIL with "HandlerAction not wired"

**Step 3: Add bridgeAction**

In `internal/frontend/handlers/bridge_handlers.go`, add the function (near other bridge functions):

```go
// bridgeAction converts the parsed action command into a proto ActionRequest
// and wraps it in a ClientMessage for transmission to the gameserver.
//
// Precondition: args is the raw argument slice from the command parser.
// Postcondition: Returns a marshalled proto ClientMessage or an error.
func bridgeAction(args []string) ([]byte, error) {
    req, err := command.HandleAction(args)
    if err != nil {
        return nil, err
    }
    msg := &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_Action{
            Action: &gamev1.ActionRequest{
                Name:   req.Name,
                Target: req.Target,
            },
        },
    }
    return proto.Marshal(msg)
}
```

Register in `bridgeHandlerMap`:

```go
    command.HandlerAction: bridgeAction,
```

**Step 4: Run to verify pass**

```bash
go test ./internal/frontend/handlers/... -v
```
Expected: PASS including `TestAllCommandHandlersAreWired`

**Step 5: Commit**

```bash
git add internal/frontend/handlers/bridge_handlers.go
git commit -m "feat: add bridgeAction and register HandlerAction in bridgeHandlerMap"
```

---

### Task 7: ActionHandler + ActionEffectResolver (gameserver)

**Files:**
- Create: `internal/gameserver/action_handler.go`
- Create: `internal/gameserver/action_handler_test.go`

**Step 1: Write failing tests**

Create `internal/gameserver/action_handler_test.go`:

```go
package gameserver_test

import (
    "context"
    "testing"

    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/gameserver"
)

// statusIdle is the int32 value for IDLE player status.
const statusIdle = int32(0)
const statusInCombat = int32(2)

func TestAvailableActions_CombatContext(t *testing.T) {
    features := []*ruleset.ClassFeature{
        {ID: "surge", Active: true, Contexts: []string{"combat"}},
        {ID: "patch", Active: true, Contexts: []string{"exploration"}},
        {ID: "passive", Active: false},
    }
    reg := ruleset.NewClassFeatureRegistry(features)
    sess := &session.PlayerSession{
        Status:       statusInCombat,
        PassiveFeats: map[string]bool{"surge": true, "patch": true, "passive": true},
    }
    actions := gameserver.AvailableActions(sess, reg, "combat")
    if len(actions) != 1 {
        t.Fatalf("expected 1 action in combat, got %d", len(actions))
    }
    if actions[0].ID != "surge" {
        t.Errorf("wrong action: %s", actions[0].ID)
    }
}

func TestAvailableActions_ExplorationContext(t *testing.T) {
    features := []*ruleset.ClassFeature{
        {ID: "surge", Active: true, Contexts: []string{"combat"}},
        {ID: "patch", Active: true, Contexts: []string{"exploration"}},
    }
    reg := ruleset.NewClassFeatureRegistry(features)
    sess := &session.PlayerSession{
        Status:       statusIdle,
        PassiveFeats: map[string]bool{"surge": true, "patch": true},
    }
    actions := gameserver.AvailableActions(sess, reg, "exploration")
    if len(actions) != 1 {
        t.Fatalf("expected 1 action in exploration, got %d", len(actions))
    }
    if actions[0].ID != "patch" {
        t.Errorf("wrong action: %s", actions[0].ID)
    }
}

func TestAvailableActions_UnownedFeature(t *testing.T) {
    features := []*ruleset.ClassFeature{
        {ID: "surge", Active: true, Contexts: []string{"combat"}},
    }
    reg := ruleset.NewClassFeatureRegistry(features)
    sess := &session.PlayerSession{
        Status:       statusInCombat,
        PassiveFeats: map[string]bool{}, // player doesn't have it
    }
    actions := gameserver.AvailableActions(sess, reg, "combat")
    if len(actions) != 0 {
        t.Errorf("expected 0 actions, got %d", len(actions))
    }
}

func TestContextForSession_InCombat(t *testing.T) {
    sess := &session.PlayerSession{Status: statusInCombat}
    ctx := gameserver.ContextForSession(sess)
    if ctx != "combat" {
        t.Errorf("got %q, want %q", ctx, "combat")
    }
}

func TestContextForSession_Idle(t *testing.T) {
    sess := &session.PlayerSession{Status: statusIdle}
    ctx := gameserver.ContextForSession(sess)
    if ctx != "exploration" {
        t.Errorf("got %q, want %q", ctx, "exploration")
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/gameserver/... -run "TestAvailableActions|TestContextForSession" -v
```
Expected: FAIL

**Step 3: Implement action_handler.go**

Create `internal/gameserver/action_handler.go`:

```go
package gameserver

import (
    "context"
    "fmt"
    "strings"

    "go.uber.org/zap"
    "google.golang.org/protobuf/proto"

    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const (
    combatContext      = "combat"
    explorationContext = "exploration"
    downtimeContext    = "downtime"
    statusInCombat     = int32(2) // gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT
    statusUnconscious  = int32(3) // gamev1.CombatStatus_COMBAT_STATUS_UNCONSCIOUS
)

// ContextForSession returns the current gameplay context string for a session.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns "combat", "exploration", or "downtime".
func ContextForSession(sess *session.PlayerSession) string {
    switch sess.Status {
    case statusInCombat:
        return combatContext
    default:
        return explorationContext
    }
}

// AvailableActions returns the active class features owned by the player that
// are valid in the given context string.
//
// Precondition: sess, reg must be non-nil; context must be non-empty.
// Postcondition: Returns only features the player owns, are active, and match context.
func AvailableActions(sess *session.PlayerSession, reg *ruleset.ClassFeatureRegistry, ctx string) []*ruleset.ClassFeature {
    var out []*ruleset.ClassFeature
    for _, f := range reg.ActiveFeatures() {
        if !sess.PassiveFeats[f.ID] {
            continue
        }
        for _, c := range f.Contexts {
            if c == ctx {
                out = append(out, f)
                break
            }
        }
    }
    return out
}

// ActionHandler resolves player-activated class feature actions.
//
// Precondition: All fields must be non-nil.
type ActionHandler struct {
    sessions   *session.Manager
    registry   *ruleset.ClassFeatureRegistry
    condReg    *condition.Registry
    npcMgr     *npc.Manager
    combatH    *CombatHandler
    charSaver  CharacterSaver
    logger     *zap.Logger
}

// NewActionHandler constructs an ActionHandler.
//
// Precondition: All arguments must be non-nil.
// Postcondition: Returns a non-nil *ActionHandler.
func NewActionHandler(
    sessions *session.Manager,
    registry *ruleset.ClassFeatureRegistry,
    condReg *condition.Registry,
    npcMgr *npc.Manager,
    combatH *CombatHandler,
    charSaver CharacterSaver,
    logger *zap.Logger,
) *ActionHandler {
    return &ActionHandler{
        sessions:  sessions,
        registry:  registry,
        condReg:   condReg,
        npcMgr:    npcMgr,
        combatH:   combatH,
        charSaver: charSaver,
        logger:    logger,
    }
}

// Handle processes an action request for the given session.
//
// Precondition: sess must be non-nil; name may be empty (list mode).
// Postcondition: Sends server events to the player; returns nil or an error.
func (h *ActionHandler) Handle(ctx context.Context, sess *session.PlayerSession, name, target string) error {
    // Unconscious players cannot act.
    if sess.Status == statusUnconscious {
        return h.sendMessage(sess, "You are unconscious and cannot act.")
    }

    gameCtx := ContextForSession(sess)

    // List mode: no name provided.
    if name == "" {
        return h.listActions(sess, gameCtx)
    }

    // Resolve the feature by ID or shortcut.
    feature := h.resolveFeature(name)
    if feature == nil {
        return h.sendMessage(sess, fmt.Sprintf("You don't know that action."))
    }

    // Check the player owns this feature.
    if !sess.PassiveFeats[feature.ID] {
        return h.sendMessage(sess, "You don't know that action.")
    }

    // Context validation.
    if !featureValidInContext(feature, gameCtx) {
        if gameCtx == combatContext {
            return h.sendMessage(sess, "You can't do that in the middle of a fight.")
        }
        return h.sendMessage(sess, "That action is only available in combat.")
    }

    // Target validation for targeted effects.
    if feature.Effect != nil && feature.Effect.Target == "target" && target == "" {
        return h.sendMessage(sess, fmt.Sprintf("Usage: %s <target>", firstOf(feature.Shortcut, feature.ID)))
    }

    // Combat: queue the action.
    if gameCtx == combatContext {
        return h.queueCombatAction(ctx, sess, feature, target)
    }

    // Out of combat: resolve immediately.
    return h.resolveEffect(ctx, sess, feature, target)
}

func (h *ActionHandler) resolveFeature(name string) *ruleset.ClassFeature {
    lower := strings.ToLower(name)
    // Try by ID first.
    if f, ok := h.registry.ClassFeature(lower); ok {
        return f
    }
    // Try by shortcut.
    for _, f := range h.registry.ActiveFeatures() {
        if strings.ToLower(f.Shortcut) == lower {
            return f
        }
    }
    return nil
}

func featureValidInContext(f *ruleset.ClassFeature, ctx string) bool {
    for _, c := range f.Contexts {
        if c == ctx {
            return true
        }
    }
    return false
}

func (h *ActionHandler) listActions(sess *session.PlayerSession, gameCtx string) error {
    actions := AvailableActions(sess, h.registry, gameCtx)
    if len(actions) == 0 {
        return h.sendMessage(sess, "No actions available in this context.")
    }
    var sb strings.Builder
    sb.WriteString("Available Actions:\n")
    for _, f := range actions {
        costStr := fmt.Sprintf("%d action", f.ActionCost)
        if f.ActionCost != 1 {
            costStr += "s"
        }
        sb.WriteString(fmt.Sprintf("  %-10s [%s]  %s — %s\n",
            firstOf(f.Shortcut, f.ID), costStr, f.Name, f.Description))
    }
    return h.sendMessage(sess, sb.String())
}

func (h *ActionHandler) queueCombatAction(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
    qa := combat.QueuedAction{
        Type:        combat.ActionUseAbility,
        AbilityID:   feature.ID,
        AbilityCost: feature.ActionCost,
        Target:      target,
    }
    if err := h.combatH.ActivateAbility(sess.UID, qa); err != nil {
        return h.sendMessage(sess, fmt.Sprintf("Not enough actions — %s costs %d action(s) (you have %d remaining).",
            firstOf(feature.Shortcut, feature.ID), feature.ActionCost, h.combatH.RemainingAP(sess.UID)))
    }
    return h.sendMessage(sess, fmt.Sprintf("You activate %s — %s", feature.Name, feature.ActivateText))
}

func (h *ActionHandler) resolveEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
    if feature.Effect == nil {
        return h.sendMessage(sess, fmt.Sprintf("You activate %s — %s", feature.Name, feature.ActivateText))
    }
    switch feature.Effect.Type {
    case "condition":
        return h.resolveConditionEffect(ctx, sess, feature, target)
    case "heal":
        return h.resolveHealEffect(ctx, sess, feature)
    case "damage":
        return h.resolveDamageEffect(ctx, sess, feature, target)
    case "skill_check":
        return h.resolveSkillCheckEffect(ctx, sess, feature)
    default:
        h.logger.Warn("unknown effect type", zap.String("type", feature.Effect.Type))
        return h.sendMessage(sess, fmt.Sprintf("You activate %s.", feature.Name))
    }
}

func (h *ActionHandler) resolveConditionEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
    condID := feature.Effect.ConditionID
    if feature.Effect.Target == "self" || target == "" {
        if err := h.condReg.Apply(condID, sess); err != nil {
            return h.sendMessage(sess, fmt.Sprintf("Cannot apply condition: %v", err))
        }
        return h.sendMessage(sess, fmt.Sprintf("%s — %s", feature.Name, feature.ActivateText))
    }
    // Target NPC: not yet implemented; placeholder.
    return h.sendMessage(sess, fmt.Sprintf("You activate %s on %s.", feature.Name, target))
}

func (h *ActionHandler) resolveHealEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature) error {
    rolled, err := dice.Roll(feature.Effect.Amount)
    if err != nil {
        return fmt.Errorf("resolving heal effect: %w", err)
    }
    newHP := sess.CurrentHP + rolled
    if newHP > sess.MaxHP {
        newHP = sess.MaxHP
    }
    sess.CurrentHP = newHP
    if h.charSaver != nil {
        _ = h.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, newHP)
    }
    h.sendHPUpdate(sess, newHP)
    return h.sendMessage(sess, fmt.Sprintf("%s — You recover %d HP. (%d/%d)",
        feature.Name, rolled, newHP, sess.MaxHP))
}

func (h *ActionHandler) resolveDamageEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
    // Damage to NPC target — stub for future implementation.
    return h.sendMessage(sess, fmt.Sprintf("You activate %s against %s.", feature.Name, target))
}

func (h *ActionHandler) resolveSkillCheckEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature) error {
    // Skill check delegation — stub for future implementation.
    return h.sendMessage(sess, fmt.Sprintf("You make a %s check (DC %d).", feature.Effect.Skill, feature.Effect.DC))
}

func (h *ActionHandler) sendMessage(sess *session.PlayerSession, msg string) error {
    evt := &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_Message{
            Message: &gamev1.MessageEvent{
                Content: msg,
                Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
            },
        },
    }
    data, err := proto.Marshal(evt)
    if err != nil {
        return err
    }
    return sess.Entity.Push(data)
}

func (h *ActionHandler) sendHPUpdate(sess *session.PlayerSession, newHP int) {
    evt := &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_HpUpdate{
            HpUpdate: &gamev1.HpUpdateEvent{
                CurrentHp: int32(newHP),
                MaxHp:     int32(sess.MaxHP),
            },
        },
    }
    if data, err := proto.Marshal(evt); err == nil {
        _ = sess.Entity.Push(data)
    }
}

func firstOf(a, b string) string {
    if a != "" {
        return a
    }
    return b
}
```

**Step 4: Run to verify pass**

```bash
go test ./internal/gameserver/... -run "TestAvailableActions|TestContextForSession" -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gameserver/action_handler.go internal/gameserver/action_handler_test.go
git commit -m "feat: implement ActionHandler with AvailableActions and ActionEffectResolver"
```

---

### Task 8: CombatHandler — ActivateAbility and RemainingAP

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/combat_handler_test.go` (add)

**Step 1: Write the failing tests**

Find the existing test file and add:

```go
func TestCombatHandler_ActivateAbility_InsufficientAP(t *testing.T) {
    // Build a minimal combat handler and enroll a player with 1 AP remaining.
    // Attempt to ActivateAbility with cost 2; expect error.
    // (Use the existing test helper pattern from combat_handler_test.go)
}

func TestCombatHandler_RemainingAP(t *testing.T) {
    // After consuming 1 AP, RemainingAP should return 2 (from 3 total).
}
```

Look at `internal/gameserver/combat_handler_test.go` first to understand the helper pattern before writing these tests; the specific helper pattern varies.

**Step 2: Run to verify failure**

```bash
go test ./internal/gameserver/... -run "TestCombatHandler_ActivateAbility|TestCombatHandler_RemainingAP" -v
```
Expected: FAIL — methods don't exist yet.

**Step 3: Add methods to CombatHandler**

In `internal/gameserver/combat_handler.go`, add:

```go
// ActivateAbility queues an ActionUseAbility for the combatant identified by uid.
//
// Precondition: uid must be non-empty; qa.Type must be ActionUseAbility.
// Postcondition: Returns nil on success or an error if the combatant has insufficient AP.
func (h *CombatHandler) ActivateAbility(uid string, qa combat.QueuedAction) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    q, ok := h.queues[uid]
    if !ok {
        return fmt.Errorf("combatant %s not found", uid)
    }
    return q.Enqueue(qa)
}

// RemainingAP returns the number of action points remaining for combatant uid.
//
// Precondition: uid must be non-empty.
// Postcondition: Returns 0 if the combatant is not found.
func (h *CombatHandler) RemainingAP(uid string) int {
    h.mu.Lock()
    defer h.mu.Unlock()
    q, ok := h.queues[uid]
    if !ok {
        return 0
    }
    return q.RemainingPoints()
}
```

Note: check the exact field name for the queues map (may be `actionQueues` — read the file before implementing).

**Step 4: Run to verify pass**

```bash
go test ./internal/gameserver/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_test.go
git commit -m "feat: add ActivateAbility and RemainingAP to CombatHandler"
```

---

### Task 9: CMD-6 — handleAction in grpc_service.go

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

**Step 1: Read grpc_service.go to understand dispatch switch structure**

Read `internal/gameserver/grpc_service.go` around the `dispatch` type switch to understand where to add the new case (search for `case *gamev1.ClientMessage_Use:` as the reference).

**Step 2: Write the failing integration test**

Add to `internal/gameserver/grpc_service_test.go` (if it exists):

```go
func TestHandleAction_UnknownAction(t *testing.T) {
    // Send an ActionRequest with name "nonexistent" and verify a "You don't know that action." message is returned.
    // Pattern follows existing handleUse test patterns.
}
```

**Step 3: Add handleAction function and wire dispatch**

In `internal/gameserver/grpc_service.go`:

Add `handleAction` near `handleUse`:

```go
// handleAction resolves a player-activated class feature action.
//
// Precondition: sess must be non-nil; req.Name may be empty (list mode).
// Postcondition: Returns nil or an error from ActionHandler.Handle.
func (s *GameServer) handleAction(ctx context.Context, sess *session.PlayerSession, req *gamev1.ActionRequest) error {
    return s.actionH.Handle(ctx, sess, req.GetName(), req.GetTarget())
}
```

Wire into the dispatch type switch:

```go
    case *gamev1.ClientMessage_Action:
        return s.handleAction(ctx, sess, p.Action)
```

Add `actionH *ActionHandler` field to `GameServer` struct and wire it in `NewGameServer` (or equivalent constructor — read the constructor to find the pattern).

**Step 4: Build and run all tests**

```bash
go build ./...
go test ./internal/gameserver/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat: wire handleAction into GameServer dispatch and ActionHandler field"
```

---

### Task 10: Shortcut Auto-Registration at Server Boot

**Files:**
- Modify: `cmd/gameserver/main.go`
- Modify: `internal/game/command/commands.go` (shortcut registration helper)

**Step 1: Write the failing test**

Add to `internal/game/command/commands_test.go`:

```go
func TestRegisterShortcuts_NoDuplicates(t *testing.T) {
    features := []*ruleset.ClassFeature{
        {ID: "surge", Active: true, Shortcut: "surge"},
        {ID: "patch", Active: true, Shortcut: "patch"},
    }
    // Should not panic.
    cmds := command.RegisterShortcuts(features, command.BuiltinCommands())
    shortcuts := map[string]bool{}
    for _, c := range cmds {
        if shortcuts[c.Name] {
            t.Errorf("duplicate command name: %s", c.Name)
        }
        shortcuts[c.Name] = true
    }
}

func TestRegisterShortcuts_Collision(t *testing.T) {
    features := []*ruleset.ClassFeature{
        {ID: "attack2", Active: true, Shortcut: "attack"}, // "attack" already exists
    }
    defer func() {
        if r := recover(); r == nil {
            t.Error("expected panic on shortcut collision, got none")
        }
    }()
    command.RegisterShortcuts(features, command.BuiltinCommands())
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/command/... -run "TestRegisterShortcuts" -v
```
Expected: FAIL

**Step 3: Implement RegisterShortcuts**

In `internal/game/command/commands.go`, add:

```go
// RegisterShortcuts builds shortcut Command entries for active class features.
// Panics at startup if any shortcut collides with an existing command name.
//
// Precondition: features must be non-nil; existing must be the current command slice.
// Postcondition: Returns extended slice with one additional entry per shortcut; panics on collision.
func RegisterShortcuts(features []*ruleset.ClassFeature, existing []Command) []Command {
    names := make(map[string]bool, len(existing))
    for _, c := range existing {
        names[c.Name] = true
    }
    for _, f := range features {
        if !f.Active || f.Shortcut == "" {
            continue
        }
        if names[f.Shortcut] {
            panic(fmt.Sprintf("command.RegisterShortcuts: shortcut %q (feature %s) collides with existing command", f.Shortcut, f.ID))
        }
        names[f.Shortcut] = true
        existing = append(existing, Command{
            Name:        f.Shortcut,
            Handler:     HandlerAction,
            Description: fmt.Sprintf("Shortcut for action %s.", f.Name),
        })
    }
    return existing
}
```

Note: `RegisterShortcuts` needs an import of `ruleset` package — add to the import block.

**Step 4: Wire in main.go**

After the ClassFeatureRegistry is built (find the line `ruleset.NewClassFeatureRegistry(...)`):

```go
// Register per-action shortcuts from active class features.
allCmds := command.RegisterShortcuts(classFeatures, command.BuiltinCommands())
// Pass allCmds to wherever the command dispatcher is initialized.
```

Read `cmd/gameserver/main.go` to find the exact initialization point before implementing.

**Step 5: Run to verify pass**

```bash
go test ./internal/game/command/... -v
go build ./...
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/game/command/commands.go internal/game/command/commands_test.go cmd/gameserver/main.go
git commit -m "feat: shortcut auto-registration at server boot from active ClassFeatures"
```

---

### Task 11: Wire ActionHandler into GameServer Constructor

**Files:**
- Modify: `cmd/gameserver/main.go`
- Modify: `internal/gameserver/grpc_service.go` (constructor)

**Step 1: Read the GameServer constructor**

Read `internal/gameserver/grpc_service.go` to find `NewGameServer` (or equivalent) and identify where to add `actionH`.

**Step 2: Wire ActionHandler**

In `cmd/gameserver/main.go`, after the existing handler constructions (near `NewRegenManager`):

```go
actionH := gameserver.NewActionHandler(sessMgr, featureRegistry, condRegistry, npcMgr, combatHandler, charRepo, logger)
```

Pass `actionH` to `NewGameServer`. Add it as a parameter or as a field set after construction (follow the existing pattern).

**Step 3: Build**

```bash
go build ./...
```
Expected: PASS

**Step 4: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 5: Commit**

```bash
git add cmd/gameserver/main.go internal/gameserver/grpc_service.go
git commit -m "feat: wire ActionHandler into GameServer constructor in main.go"
```

---

### Task 12: End-to-End Verification

**Step 1: Build and deploy**

```bash
make k8s-redeploy
```

**Step 2: Manual smoke test**

Connect via telnet and verify:
1. `action` — lists `surge [1 action] Brutal Surge — Enter a combat frenzy...` (only if player has brutal_surge feat)
2. `action surge` out of combat — `"That action is only available in combat."`
3. Enter combat; `action surge` — activates brutal_surge condition; `brutal_surge_active` applied; narrative sent
4. `surge` shortcut — same as `action surge`
5. Player without brutal_surge feat — `"You don't know that action."`
6. `action surge` with 0 AP remaining in combat — `"Not enough actions — surge costs 1 action (you have 0 remaining)."`

**Step 3: Run full test suite one final time**

```bash
go test ./... -count=1
```
Expected: all PASS
