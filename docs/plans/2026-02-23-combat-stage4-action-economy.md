# Combat Stage 4 — Three-Action Economy + Round Timer

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade combat to PF2E's 3-action economy: players queue up to 3 actions during a timed round window; all actions resolve in initiative order when the timer fires or all combatants have submitted.

**Architecture:** A new `ActionQueue` per combatant tracks queued actions and remaining AP. A `RoundTimer` goroutine per active combat fires after the configured duration, triggering round resolution via a `broadcastFn` callback injected into `CombatHandler`. The existing synchronous per-action attack flow is replaced by queue-then-resolve semantics. `attack <target>` (1 AP), `strike <target>` (2 AP, with MAP penalty on second hit), and `pass` (forfeit remaining AP) are the player-facing commands. `flee` remains immediate.

**Tech Stack:** Go, `pgregory.net/rapid`, Protocol Buffers, `time.AfterFunc` for timer, existing `dice.Roller`, `npc.Manager`, `session.Manager`.

**Key domain rules:**
- Each combatant gets 3 AP per round (configurable)
- `attack` costs 1 AP; `strike` costs 2 AP (two attacks, second at −5 MAP)
- `pass` costs 0 AP but marks the queue as submitted
- Round resolves when: (a) all living combatants have submitted, or (b) timer expires
- NPC queues: NPCs auto-queue one `attack` action at round start (simple AI, Stage 8 upgrades this)
- Resolution order: initiative order (already sorted in `Combat.Combatants`)
- `broadcastFn(roomID string, events []*gamev1.CombatEvent)` is injected at construction; called from timer goroutine

---

### Task 1: Action types + queue domain model

**Files:**
- Create: `internal/game/combat/action.go`
- Create: `internal/game/combat/action_test.go`

**Step 1: Write the failing tests**

Create `internal/game/combat/action_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestActionType_Cost(t *testing.T) {
	assert.Equal(t, 1, combat.ActionAttack.Cost())
	assert.Equal(t, 2, combat.ActionStrike.Cost())
	assert.Equal(t, 0, combat.ActionPass.Cost())
}

func TestActionType_String(t *testing.T) {
	assert.Equal(t, "attack", combat.ActionAttack.String())
	assert.Equal(t, "strike", combat.ActionStrike.String())
	assert.Equal(t, "pass", combat.ActionPass.String())
}

func TestActionQueue_Enqueue_Success(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "ganger"})
	require.NoError(t, err)
	assert.Equal(t, 2, q.Remaining)
	assert.Len(t, q.Actions, 1)
}

func TestActionQueue_Enqueue_InsufficientAP(t *testing.T) {
	q := combat.NewActionQueue("uid1", 1)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionStrike, Target: "ganger"})
	assert.Error(t, err)
	assert.Equal(t, 1, q.Remaining)
}

func TestActionQueue_IsSubmitted_AfterPass(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err)
	assert.True(t, q.IsSubmitted())
}

func TestActionQueue_IsSubmitted_FullSpend(t *testing.T) {
	q := combat.NewActionQueue("uid1", 2)
	require.NoError(t, q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "g"}))
	require.NoError(t, q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "g"}))
	assert.True(t, q.IsSubmitted())
}

func TestActionQueue_IsSubmitted_NotYet(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	require.NoError(t, q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "g"}))
	assert.False(t, q.IsSubmitted())
}

func TestActionQueue_HasPoints(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	assert.True(t, q.HasPoints())
	_ = q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	assert.False(t, q.HasPoints()) // pass sets remaining to 0
}

func TestPropertyActionQueue_RemainingNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxAP := rapid.IntRange(1, 6).Draw(rt, "max_ap")
		q := combat.NewActionQueue("uid1", maxAP)
		actionTypes := []combat.ActionType{combat.ActionAttack, combat.ActionStrike, combat.ActionPass}
		for i := 0; i < 10; i++ {
			idx := rapid.IntRange(0, len(actionTypes)-1).Draw(rt, "action")
			a := combat.QueuedAction{Type: actionTypes[idx], Target: "t"}
			_ = q.Enqueue(a) // ignore errors — some will fail, that's fine
			assert.GreaterOrEqual(rt, q.Remaining, 0)
		}
	})
}
```

**Step 2: Run to verify failure**

```
go test ./internal/game/combat/... -run TestActionType -v
```
Expected: FAIL — `combat.ActionAttack` undefined.

**Step 3: Implement `internal/game/combat/action.go`**

```go
package combat

import "fmt"

// ActionType identifies what a combatant intends to do on their turn.
type ActionType int

const (
	ActionAttack ActionType = iota + 1 // costs 1 AP
	ActionStrike                        // costs 2 AP; two attacks with MAP
	ActionPass                          // costs 0 AP; forfeits remaining actions
)

// Cost returns the action point cost of this action type.
func (a ActionType) Cost() int {
	switch a {
	case ActionAttack:
		return 1
	case ActionStrike:
		return 2
	case ActionPass:
		return 0
	default:
		return 1
	}
}

// String returns the human-readable name of this action type.
func (a ActionType) String() string {
	switch a {
	case ActionAttack:
		return "attack"
	case ActionStrike:
		return "strike"
	case ActionPass:
		return "pass"
	default:
		return "unknown"
	}
}

// QueuedAction represents one action a combatant intends to take this round.
type QueuedAction struct {
	Type   ActionType
	Target string // NPC name for attack/strike; empty for pass
}

// ActionQueue tracks a combatant's remaining action points and their queued actions.
//
// Invariant: Remaining >= 0 at all times.
// Invariant: sum of Cost() for all queued Actions <= MaxPoints.
type ActionQueue struct {
	UID       string
	MaxPoints int
	Remaining int
	Actions   []QueuedAction
}

// NewActionQueue creates an ActionQueue for the given combatant with actionsPerRound AP.
//
// Precondition: actionsPerRound > 0.
// Postcondition: Returns a non-nil queue with Remaining == actionsPerRound.
func NewActionQueue(uid string, actionsPerRound int) *ActionQueue {
	return &ActionQueue{
		UID:       uid,
		MaxPoints: actionsPerRound,
		Remaining: actionsPerRound,
	}
}

// Enqueue adds an action to the queue if sufficient AP remain.
//
// Postcondition: Returns an error if AP is insufficient; otherwise appends the action
// and decrements Remaining. For ActionPass, Remaining is set to 0.
func (q *ActionQueue) Enqueue(a QueuedAction) error {
	cost := a.Type.Cost()
	if a.Type == ActionPass {
		q.Actions = append(q.Actions, a)
		q.Remaining = 0
		return nil
	}
	if cost > q.Remaining {
		return fmt.Errorf("insufficient action points: need %d, have %d", cost, q.Remaining)
	}
	q.Actions = append(q.Actions, a)
	q.Remaining -= cost
	return nil
}

// HasPoints reports whether any AP remains and the queue is not yet submitted.
func (q *ActionQueue) HasPoints() bool {
	return q.Remaining > 0 && !q.IsSubmitted()
}

// IsSubmitted reports whether the combatant has used all AP or explicitly passed.
func (q *ActionQueue) IsSubmitted() bool {
	if q.Remaining == 0 {
		return true
	}
	for _, a := range q.Actions {
		if a.Type == ActionPass {
			return true
		}
	}
	return false
}
```

**Step 4: Run tests**

```
go test ./internal/game/combat/... -run 'TestActionType|TestActionQueue|TestPropertyActionQueue' -v -count=1
```
Expected: all PASS.

**Step 5: Commit**

```
git add internal/game/combat/action.go internal/game/combat/action_test.go
git commit -m "feat(combat): ActionType, ActionQueue — 3-AP queue domain model (Stage 4 Task 1)"
```

---

### Task 2: Combat struct — round + action queues

**Files:**
- Modify: `internal/game/combat/engine.go`
- Modify: `internal/game/combat/engine_test.go` (create if not present; add new tests)

**Step 1: Write the failing tests**

Add to `internal/game/combat/engine_test.go` (create file if needed; import path `package combat_test`):

```go
func TestCombat_StartRound_IncrementsRound(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	assert.Equal(t, 0, cbt.Round)
	cbt.StartRound(3)
	assert.Equal(t, 1, cbt.Round)
	cbt.StartRound(3)
	assert.Equal(t, 2, cbt.Round)
}

func TestCombat_StartRound_ResetsQueues(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	cbt.StartRound(3)
	for _, c := range cbt.Combatants {
		q := cbt.ActionQueues[c.ID]
		require.NotNil(t, q)
		assert.Equal(t, 3, q.Remaining)
	}
}

func TestCombat_StartRound_SkipsDeadCombatants(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	cbt.Combatants[1].CurrentHP = 0 // kill second combatant
	cbt.StartRound(3)
	assert.Nil(t, cbt.ActionQueues[cbt.Combatants[1].ID])
}

func TestCombat_QueueAction_Success(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	cbt.StartRound(3)
	uid := cbt.Combatants[0].ID
	err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAttack, Target: "target"})
	require.NoError(t, err)
	assert.Equal(t, 2, cbt.ActionQueues[uid].Remaining)
}

func TestCombat_QueueAction_UnknownUID(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	cbt.StartRound(3)
	err := cbt.QueueAction("nobody", combat.QueuedAction{Type: combat.ActionAttack})
	assert.Error(t, err)
}

func TestCombat_AllActionsSubmitted_False(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	cbt.StartRound(3)
	assert.False(t, cbt.AllActionsSubmitted())
}

func TestCombat_AllActionsSubmitted_True(t *testing.T) {
	cbt := makeTwoCombatantCombat(t)
	cbt.StartRound(3)
	for _, c := range cbt.LivingCombatants() {
		_ = cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionPass})
	}
	assert.True(t, cbt.AllActionsSubmitted())
}

func TestPropertyCombat_RoundMonotonicallyIncreases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cbt := makeTwoCombatantCombat(t)
		n := rapid.IntRange(1, 20).Draw(rt, "rounds")
		for i := 1; i <= n; i++ {
			cbt.StartRound(3)
			assert.Equal(rt, i, cbt.Round)
		}
	})
}

// makeTwoCombatantCombat returns a Combat with two living combatants (no engine, for unit tests).
func makeTwoCombatantCombat(t *testing.T) *combat.Combat {
	t.Helper()
	return &combat.Combat{
		RoomID: "room1",
		Combatants: []*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
			{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1},
		},
		ActionQueues: make(map[string]*combat.ActionQueue),
	}
}
```

**Step 2: Run to verify failure**

```
go test ./internal/game/combat/... -run TestCombat_StartRound -v
```
Expected: FAIL — `cbt.StartRound` undefined, `Combat.ActionQueues` undefined.

**Step 3: Modify `internal/game/combat/engine.go`**

Add `Round int` and `ActionQueues map[string]*ActionQueue` to the `Combat` struct:

```go
type Combat struct {
	RoomID       string
	Combatants   []*Combatant
	turnIndex    int
	Over         bool
	Round        int
	ActionQueues map[string]*ActionQueue
}
```

Update `Engine.StartCombat` to initialize `ActionQueues`:

```go
cbt := &Combat{
	RoomID:       roomID,
	Combatants:   sorted,
	ActionQueues: make(map[string]*ActionQueue),
}
```

Add three new methods to `Combat` after the existing ones:

```go
// StartRound increments Round and resets ActionQueues for all living combatants.
//
// Postcondition: Round is incremented; each living combatant has a fresh ActionQueue
// with actionsPerRound AP. Dead combatants have no queue entry.
func (c *Combat) StartRound(actionsPerRound int) {
	c.Round++
	c.ActionQueues = make(map[string]*ActionQueue)
	for _, cbt := range c.Combatants {
		if !cbt.IsDead() {
			c.ActionQueues[cbt.ID] = NewActionQueue(cbt.ID, actionsPerRound)
		}
	}
}

// QueueAction enqueues an action for the combatant with the given UID.
//
// Precondition: uid must be a living combatant with an active queue.
// Postcondition: Returns error if uid not found or AP insufficient; otherwise action is appended.
func (c *Combat) QueueAction(uid string, a QueuedAction) error {
	q, ok := c.ActionQueues[uid]
	if !ok {
		return fmt.Errorf("no action queue for combatant %q", uid)
	}
	return q.Enqueue(a)
}

// AllActionsSubmitted reports whether every living combatant's queue IsSubmitted.
//
// Postcondition: Returns true iff all living combatants have no remaining AP or have passed.
func (c *Combat) AllActionsSubmitted() bool {
	for _, cbt := range c.Combatants {
		if cbt.IsDead() {
			continue
		}
		q, ok := c.ActionQueues[cbt.ID]
		if !ok || !q.IsSubmitted() {
			return false
		}
	}
	return true
}
```

**Step 4: Run tests**

```
go test ./internal/game/combat/... -count=1 -race -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: all PASS.

**Step 5: Commit**

```
git add internal/game/combat/engine.go internal/game/combat/engine_test.go
git commit -m "feat(combat): Combat.StartRound, QueueAction, AllActionsSubmitted (Stage 4 Task 2)"
```

---

### Task 3: Round resolution

**Files:**
- Create: `internal/game/combat/round.go`
- Create: `internal/game/combat/round_test.go`

**Step 1: Write the failing tests**

Create `internal/game/combat/round_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

type fixedSrc struct{ val int }

func (f fixedSrc) Intn(n int) int {
	if f.val >= n {
		return n - 1
	}
	return f.val
}

func makeRoundCombat(t *testing.T) *combat.Combat {
	t.Helper()
	cbt := &combat.Combat{
		RoomID: "room1",
		Combatants: []*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
			{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1, StrMod: 1, DexMod: 0},
		},
		ActionQueues: make(map[string]*combat.ActionQueue),
	}
	cbt.StartRound(3)
	return cbt
}

func noopUpdater(id string, hp int) {}

func TestResolveRound_AllPass(t *testing.T) {
	cbt := makeRoundCombat(t)
	for _, c := range cbt.LivingCombatants() {
		require.NoError(t, cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionPass}))
	}
	events := combat.ResolveRound(cbt, fixedSrc{10}, noopUpdater)
	assert.Len(t, events, 2) // one pass event per combatant
	for _, e := range events {
		assert.Equal(t, combat.ActionPass, e.ActionType)
		assert.Nil(t, e.AttackResult)
	}
}

func TestResolveRound_AttackHits(t *testing.T) {
	cbt := makeRoundCombat(t)
	// Use high fixed roll so attack always hits
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "n1"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	events := combat.ResolveRound(cbt, fixedSrc{15}, noopUpdater)
	// First event is player's attack
	atk := events[0]
	assert.Equal(t, combat.ActionAttack, atk.ActionType)
	require.NotNil(t, atk.AttackResult)
}

func TestResolveRound_AttackKills(t *testing.T) {
	cbt := makeRoundCombat(t)
	cbt.Combatants[1].CurrentHP = 1 // Ganger almost dead
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "n1"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	updated := map[string]int{}
	events := combat.ResolveRound(cbt, fixedSrc{18}, func(id string, hp int) { updated[id] = hp })
	// Ganger should be dead
	assert.Equal(t, 0, cbt.Combatants[1].CurrentHP)
	assert.Contains(t, updated, "n1")
	// Some event should describe the kill
	found := false
	for _, e := range events {
		if e.ActorID == "p1" && e.AttackResult != nil && e.AttackResult.EffectiveDamage() > 0 {
			found = true
		}
	}
	assert.True(t, found, "expected a damaging attack event")
}

func TestResolveRound_Strike_TwoAttacks(t *testing.T) {
	cbt := makeRoundCombat(t)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "n1"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, fixedSrc{10}, noopUpdater)
	attackEvents := 0
	for _, e := range events {
		if e.ActorID == "p1" && e.ActionType == combat.ActionStrike {
			attackEvents++
		}
	}
	assert.Equal(t, 2, attackEvents, "strike should produce 2 attack events")
}

func TestResolveRound_Strike_MAPPenalty(t *testing.T) {
	cbt := makeRoundCombat(t)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "n1"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, fixedSrc{10}, noopUpdater)
	strikeEvents := []combat.RoundEvent{}
	for _, e := range events {
		if e.ActorID == "p1" && e.ActionType == combat.ActionStrike {
			strikeEvents = append(strikeEvents, e)
		}
	}
	require.Len(t, strikeEvents, 2)
	// Second attack total should be 5 less than first (MAP)
	first := strikeEvents[0].AttackResult.AttackTotal
	second := strikeEvents[1].AttackResult.AttackTotal
	assert.Equal(t, first-5, second)
}

func TestResolveRound_DeadCombatantSkipped(t *testing.T) {
	cbt := makeRoundCombat(t)
	cbt.Combatants[1].CurrentHP = 0 // kill NPC before round resolves
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, fixedSrc{10}, noopUpdater)
	for _, e := range events {
		assert.NotEqual(t, "n1", e.ActorID, "dead combatant should produce no events")
	}
}

func TestPropertyResolveRound_DamageNeverExceedsStartingHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		startHP := rapid.IntRange(1, 50).Draw(rt, "start_hp")
		roll := rapid.IntRange(0, 19).Draw(rt, "roll")

		cbt := &combat.Combat{
			RoomID: "r1",
			Combatants: []*combat.Combatant{
				{ID: "p1", Kind: combat.KindPlayer, Name: "A", MaxHP: 30, CurrentHP: 30, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
				{ID: "n1", Kind: combat.KindNPC, Name: "G", MaxHP: startHP, CurrentHP: startHP, AC: 12, Level: 1, StrMod: 1, DexMod: 0},
			},
			ActionQueues: make(map[string]*combat.ActionQueue),
		}
		cbt.StartRound(3)
		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "n1"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		combat.ResolveRound(cbt, fixedSrc{roll}, noopUpdater)
		assert.GreaterOrEqual(rt, cbt.Combatants[1].CurrentHP, 0)
	})
}
```

**Step 2: Run to verify failure**

```
go test ./internal/game/combat/... -run TestResolveRound -v
```
Expected: FAIL — `combat.ResolveRound` undefined, `combat.RoundEvent` undefined.

**Step 3: Implement `internal/game/combat/round.go`**

```go
package combat

import "fmt"

// RoundEvent records what happened when one action was resolved.
type RoundEvent struct {
	AttackResult *AttackResult // nil for pass
	ActionType   ActionType
	ActorID      string
	ActorName    string
	Narrative    string
}

// ResolveRound processes all queued actions for cbt in initiative order (Combatants slice order).
// For each living combatant it iterates their queued actions:
//   - ActionAttack: one ResolveAttack call; damage applied to target.
//   - ActionStrike: two ResolveAttack calls; second has a −5 MAP penalty on attack total.
//   - ActionPass: narrative event, no damage.
//
// targetUpdater is called after each damage application with (combatantID, newHP) so the
// caller can sync HP back to external stores (npc.Instance, session.PlayerSession).
//
// Precondition: cbt and src must not be nil; targetUpdater may be nil (no-op if nil).
// Postcondition: Returns ordered RoundEvents; all damage applied in-place on Combatants.
func ResolveRound(cbt *Combat, src Source, targetUpdater func(id string, hp int)) []RoundEvent {
	if targetUpdater == nil {
		targetUpdater = func(id string, hp int) {}
	}

	var events []RoundEvent

	for _, actor := range cbt.Combatants {
		if actor.IsDead() {
			continue
		}
		q, ok := cbt.ActionQueues[actor.ID]
		if !ok {
			continue
		}

		for _, action := range q.Actions {
			switch action.Type {
			case ActionPass:
				events = append(events, RoundEvent{
					ActionType: ActionPass,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					Narrative:  fmt.Sprintf("%s passes.", actor.Name),
				})

			case ActionAttack:
				target := findCombatantByName(cbt, action.Target)
				if target == nil || target.IsDead() {
					events = append(events, RoundEvent{
						ActionType: ActionAttack,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s swings at nothing.", actor.Name),
					})
					continue
				}
				result := ResolveAttack(actor, target, src)
				applyDamageAndUpdate(target, result.EffectiveDamage(), targetUpdater)
				events = append(events, RoundEvent{
					AttackResult: &result,
					ActionType:   ActionAttack,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    attackNarrative(actor.Name, target.Name, result),
				})

			case ActionStrike:
				target := findCombatantByName(cbt, action.Target)
				if target == nil || target.IsDead() {
					events = append(events, RoundEvent{
						ActionType: ActionStrike,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s attacks nothing.", actor.Name),
					})
					continue
				}
				// First attack — normal
				r1 := ResolveAttack(actor, target, src)
				applyDamageAndUpdate(target, r1.EffectiveDamage(), targetUpdater)
				events = append(events, RoundEvent{
					AttackResult: &r1,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    attackNarrative(actor.Name, target.Name, r1),
				})
				// Second attack — MAP −5
				r2 := ResolveAttack(actor, target, src)
				r2.AttackTotal -= 5
				// Re-evaluate outcome with MAP-adjusted total
				r2.Outcome = OutcomeFor(r2.AttackTotal, target.AC)
				if target.IsDead() {
					events = append(events, RoundEvent{
						AttackResult: &r2,
						ActionType:   ActionStrike,
						ActorID:      actor.ID,
						ActorName:    actor.Name,
						Narrative:    fmt.Sprintf("%s's follow-up swing hits nothing — target already down.", actor.Name),
					})
					continue
				}
				applyDamageAndUpdate(target, r2.EffectiveDamage(), targetUpdater)
				events = append(events, RoundEvent{
					AttackResult: &r2,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    attackNarrative(actor.Name, target.Name, r2),
				})
			}
		}
	}

	return events
}

func applyDamageAndUpdate(target *Combatant, dmg int, updater func(id string, hp int)) {
	target.ApplyDamage(dmg)
	updater(target.ID, target.CurrentHP)
}

func findCombatantByName(cbt *Combat, name string) *Combatant {
	for _, c := range cbt.Combatants {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func attackNarrative(attacker, target string, result AttackResult) string {
	switch result.Outcome {
	case CritSuccess:
		return fmt.Sprintf("%s lands a devastating blow on %s for %d damage!", attacker, target, result.EffectiveDamage())
	case Success:
		return fmt.Sprintf("%s hits %s for %d damage.", attacker, target, result.EffectiveDamage())
	case Failure:
		return fmt.Sprintf("%s swings at %s but misses.", attacker, target)
	default:
		return fmt.Sprintf("%s fumbles against %s.", attacker, target)
	}
}
```

**Step 4: Run tests**

```
go test ./internal/game/combat/... -count=1 -race -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: all PASS.

**Step 5: Commit**

```
git add internal/game/combat/round.go internal/game/combat/round_test.go
git commit -m "feat(combat): ResolveRound — queue-based resolution with MAP for Strike (Stage 4 Task 3)"
```

---

### Task 4: Round timer

**Files:**
- Create: `internal/game/combat/timer.go`
- Create: `internal/game/combat/timer_test.go`

**Step 1: Write the failing tests**

Create `internal/game/combat/timer_test.go`:

```go
package combat_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
)

func TestRoundTimer_Fires(t *testing.T) {
	var fired atomic.Bool
	rt := combat.NewRoundTimer(20*time.Millisecond, func() { fired.Store(true) })
	defer rt.Stop()
	time.Sleep(50 * time.Millisecond)
	assert.True(t, fired.Load(), "timer should have fired")
}

func TestRoundTimer_Stop_PreventsCallback(t *testing.T) {
	var fired atomic.Bool
	rt := combat.NewRoundTimer(50*time.Millisecond, func() { fired.Store(true) })
	rt.Stop()
	time.Sleep(80 * time.Millisecond)
	assert.False(t, fired.Load(), "stopped timer should not fire")
}

func TestRoundTimer_Reset_ExtendsDeadline(t *testing.T) {
	var count atomic.Int32
	rt := combat.NewRoundTimer(30*time.Millisecond, func() { count.Add(1) })
	time.Sleep(15 * time.Millisecond)
	// Reset before first fire — new deadline is 30ms from now
	rt.Reset(30*time.Millisecond, func() { count.Add(1) })
	time.Sleep(20 * time.Millisecond)
	// Only 20ms since reset, should not have fired yet
	assert.Equal(t, int32(0), count.Load(), "should not have fired after reset + 20ms")
	time.Sleep(20 * time.Millisecond)
	// Now 40ms since reset — should have fired once
	assert.Equal(t, int32(1), count.Load())
	rt.Stop()
}

func TestRoundTimer_StopIdempotent(t *testing.T) {
	rt := combat.NewRoundTimer(50*time.Millisecond, func() {})
	assert.NotPanics(t, func() {
		rt.Stop()
		rt.Stop()
		rt.Stop()
	})
}
```

**Step 2: Run to verify failure**

```
go test ./internal/game/combat/... -run TestRoundTimer -v
```
Expected: FAIL — `combat.NewRoundTimer` undefined.

**Step 3: Implement `internal/game/combat/timer.go`**

```go
package combat

import (
	"sync"
	"time"
)

// RoundTimer fires a callback after a configurable duration unless stopped.
// It is safe for concurrent use.
type RoundTimer struct {
	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

// NewRoundTimer creates and starts a timer that calls onFire after duration.
// onFire is called in a separate goroutine managed by the Go runtime.
//
// Precondition: duration > 0; onFire must not be nil.
// Postcondition: Returns a running RoundTimer; onFire will be called unless Stop is called first.
func NewRoundTimer(duration time.Duration, onFire func()) *RoundTimer {
	rt := &RoundTimer{}
	rt.timer = time.AfterFunc(duration, func() {
		rt.mu.Lock()
		s := rt.stopped
		rt.mu.Unlock()
		if !s {
			onFire()
		}
	})
	return rt
}

// Reset cancels the current timer and starts a new one with the provided duration and callback.
//
// Postcondition: onFire will be called after duration from now unless Stop is called first.
func (rt *RoundTimer) Reset(duration time.Duration, onFire func()) {
	rt.mu.Lock()
	rt.stopped = false
	rt.mu.Unlock()
	rt.timer.Stop()
	rt.timer = time.AfterFunc(duration, func() {
		rt.mu.Lock()
		s := rt.stopped
		rt.mu.Unlock()
		if !s {
			onFire()
		}
	})
}

// Stop prevents the callback from firing. Safe to call multiple times.
//
// Postcondition: onFire will not be called after Stop returns.
func (rt *RoundTimer) Stop() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.stopped = true
	rt.timer.Stop()
}
```

**Step 4: Run tests**

```
go test ./internal/game/combat/... -count=1 -race -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: all PASS.

**Step 5: Commit**

```
git add internal/game/combat/timer.go internal/game/combat/timer_test.go
git commit -m "feat(combat): RoundTimer — cancelable per-round timer goroutine (Stage 4 Task 4)"
```

---

### Task 5: Proto additions

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `make proto`
- Create: `internal/gameserver/gamev1/proto_test.go`

**Step 1: Edit `api/proto/game/v1/game.proto`**

Add to `ClientMessage` oneof after field 12:
```protobuf
    PassRequest   pass   = 13;
    StrikeRequest strike = 14;
```

Add to `ServerEvent` oneof after field 11:
```protobuf
    RoundStartEvent round_start = 12;
    RoundEndEvent   round_end   = 13;
```

Add new message definitions after `FleeRequest {}`:
```protobuf
// PassRequest forfeits the player's remaining action points for the current round.
message PassRequest {}

// StrikeRequest queues a 2-AP full attack routine (two attacks with MAP) against a target.
message StrikeRequest {
  string target = 1;
}

// RoundStartEvent is broadcast to all combatants when a new round begins.
message RoundStartEvent {
  int32           round            = 1;
  int32           actions_per_turn = 2;
  int32           duration_ms      = 3;
  repeated string turn_order       = 4;
}

// RoundEndEvent is broadcast when a round's actions have resolved.
message RoundEndEvent {
  int32 round = 1;
}
```

**Step 2: Regenerate**

```
make proto
```
Expected: regenerates `game.pb.go` and `game_grpc.pb.go` without errors.

**Step 3: Write compile + roundtrip test**

Create `internal/gameserver/gamev1/proto_test.go`:

```go
package gamev1_test

import (
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProto_RoundStartEvent_Roundtrip(t *testing.T) {
	orig := &gamev1.RoundStartEvent{
		Round:          3,
		ActionsPerTurn: 3,
		DurationMs:     6000,
		TurnOrder:      []string{"Alice", "Ganger"},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	got := &gamev1.RoundStartEvent{}
	require.NoError(t, proto.Unmarshal(data, got))
	assert.Equal(t, orig.Round, got.Round)
	assert.Equal(t, orig.DurationMs, got.DurationMs)
	assert.Equal(t, orig.TurnOrder, got.TurnOrder)
}

func TestProto_PassRequest_Roundtrip(t *testing.T) {
	orig := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	got := &gamev1.ClientMessage{}
	require.NoError(t, proto.Unmarshal(data, got))
	_, ok := got.Payload.(*gamev1.ClientMessage_Pass)
	assert.True(t, ok)
}

func TestProto_StrikeRequest_Roundtrip(t *testing.T) {
	orig := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: "ganger"}},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	got := &gamev1.ClientMessage{}
	require.NoError(t, proto.Unmarshal(data, got))
	strike, ok := got.Payload.(*gamev1.ClientMessage_Strike)
	require.True(t, ok)
	assert.Equal(t, "ganger", strike.Strike.Target)
}
```

**Step 4: Run tests**

```
go test ./internal/gameserver/gamev1/... -count=1 -v
```
Expected: all PASS.

**Step 5: Commit**

```
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/ internal/gameserver/gamev1/proto_test.go
git commit -m "feat(proto): PassRequest, StrikeRequest, RoundStartEvent, RoundEndEvent (Stage 4 Task 5)"
```

---

### Task 6: CombatHandler refactor

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_test.go`

**Step 1: Write the failing tests**

Create `internal/gameserver/combat_handler_test.go`:

```go
package gameserver_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type capturedBroadcast struct {
	mu     sync.Mutex
	events []*gamev1.CombatEvent
}

func (c *capturedBroadcast) fn(roomID string, events []*gamev1.CombatEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, events...)
}

func (c *capturedBroadcast) all() []*gamev1.CombatEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*gamev1.CombatEvent, len(c.events))
	copy(out, c.events)
	return out
}

func makeCombatHandler(t *testing.T, broadcast *capturedBroadcast) (*gameserver.CombatHandler, *session.Manager, *npc.Manager) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	engine := combat.NewEngine()
	roller := dice.NewRoller(logger)

	h := gameserver.NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		broadcast.fn,
		200*time.Millisecond, // short timer for tests
	)
	return h, sessMgr, npcMgr
}

func spawnTestNPC(t *testing.T, npcMgr *npc.Manager, roomID string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:   "ganger_tmpl",
		Name: "Ganger",
		Stats: ruleset.Stats{
			Strength: 12, Dexterity: 10, Constitution: 12,
			Intelligence: 8, Wisdom: 8, Charisma: 8,
		},
		Level:       1,
		MaxHP:       18,
		AC:          12,
		Perception:  10,
		Description: "A gang member.",
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)
	return inst
}

func addTestPlayer(t *testing.T, sessMgr *session.Manager, uid, roomID string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(uid, "Alice", roomID, 1, "Alice", 20)
	require.NoError(t, err)
	return sess
}

func TestCombatHandler_Attack_StartsCombat(t *testing.T) {
	bc := &capturedBroadcast{}
	h, sessMgr, npcMgr := makeCombatHandler(t, bc)
	addTestPlayer(t, sessMgr, "p1", "room1")
	spawnTestNPC(t, npcMgr, "room1")

	events, err := h.Attack("p1", "Ganger")
	require.NoError(t, err)
	// Should return initiative events + round-start confirmation
	assert.NotEmpty(t, events)
	found := false
	for _, e := range events {
		if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE {
			found = true
		}
	}
	assert.True(t, found, "expected initiative event on combat start")
}

func TestCombatHandler_Attack_QueuesAction(t *testing.T) {
	bc := &capturedBroadcast{}
	h, sessMgr, npcMgr := makeCombatHandler(t, bc)
	addTestPlayer(t, sessMgr, "p1", "room1")
	spawnTestNPC(t, npcMgr, "room1")

	_, err := h.Attack("p1", "Ganger") // start combat
	require.NoError(t, err)

	// Second attack queues without resolving
	events, err := h.Attack("p1", "Ganger")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	// No COMBAT_EVENT_TYPE_END yet
	for _, e := range events {
		assert.NotEqual(t, gamev1.CombatEventType_COMBAT_EVENT_TYPE_END, e.Type)
	}
}

func TestCombatHandler_Pass_ForfeitsAP(t *testing.T) {
	bc := &capturedBroadcast{}
	h, sessMgr, npcMgr := makeCombatHandler(t, bc)
	addTestPlayer(t, sessMgr, "p1", "room1")
	spawnTestNPC(t, npcMgr, "room1")

	_, err := h.Attack("p1", "Ganger") // start combat + queue first attack
	require.NoError(t, err)

	events, err := h.Pass("p1")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
}

func TestCombatHandler_TimerFires_ResolvesRound(t *testing.T) {
	bc := &capturedBroadcast{}
	h, sessMgr, npcMgr := makeCombatHandler(t, bc)
	addTestPlayer(t, sessMgr, "p1", "room1")
	spawnTestNPC(t, npcMgr, "room1")

	_, err := h.Attack("p1", "Ganger") // start combat
	require.NoError(t, err)

	// Wait for timer to fire (200ms in test)
	time.Sleep(350 * time.Millisecond)

	// broadcastFn should have received resolution events
	events := bc.all()
	assert.NotEmpty(t, events, "timer should have broadcast round resolution events")
}

func TestCombatHandler_Strike_Costs2AP(t *testing.T) {
	bc := &capturedBroadcast{}
	h, sessMgr, npcMgr := makeCombatHandler(t, bc)
	addTestPlayer(t, sessMgr, "p1", "room1")
	spawnTestNPC(t, npcMgr, "room1")

	_, err := h.Attack("p1", "Ganger") // start combat
	require.NoError(t, err)

	events, err := h.Strike("p1", "Ganger")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	// Check narrative mentions strike
	found := false
	for _, e := range events {
		if e.Narrative != "" {
			found = true
		}
	}
	assert.True(t, found)
}
```

**Step 2: Run to verify failure**

```
go test ./internal/gameserver/... -run TestCombatHandler -v
```
Expected: FAIL — `NewCombatHandler` wrong signature, `Pass`/`Strike` methods missing.

**Step 3: Rewrite `internal/gameserver/combat_handler.go`**

```go
package gameserver

import (
	"fmt"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// CombatHandler handles attack, flee, pass, strike, and round timer execution.
//
// Precondition: All fields must be non-nil after construction.
type CombatHandler struct {
	engine        *combat.Engine
	npcMgr        *npc.Manager
	sessions      *session.Manager
	dice          *dice.Roller
	broadcastFn   func(roomID string, events []*gamev1.CombatEvent)
	roundDuration time.Duration
	timersMu      sync.Mutex
	timers        map[string]*combat.RoundTimer
}

// NewCombatHandler creates a CombatHandler.
//
// Precondition: all arguments must be non-nil; roundDuration must be > 0.
// Postcondition: Returns a non-nil CombatHandler.
func NewCombatHandler(
	engine *combat.Engine,
	npcMgr *npc.Manager,
	sessions *session.Manager,
	diceRoller *dice.Roller,
	broadcastFn func(roomID string, events []*gamev1.CombatEvent),
	roundDuration time.Duration,
) *CombatHandler {
	return &CombatHandler{
		engine:        engine,
		npcMgr:        npcMgr,
		sessions:      sessions,
		dice:          diceRoller,
		broadcastFn:   broadcastFn,
		roundDuration: roundDuration,
		timers:        make(map[string]*combat.RoundTimer),
	}
}

// Attack queues a 1-AP attack action for the player.
// If no combat is active, starts combat with initiative rolls and broadcasts the round start.
//
// Postcondition: Returns events describing combat start (if new) or queue confirmation; error on fatal issues.
func (h *CombatHandler) Attack(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}
	if inst.IsDead() {
		return nil, fmt.Errorf("%s is already dead", inst.Name)
	}

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		var initEvents []*gamev1.CombatEvent
		cbt, initEvents = h.startCombat(sess, inst)
		// Queue initial attack as part of start
		_ = cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAttack, Target: inst.Name})
		h.autoQueueNPCs(cbt)
		h.startTimer(sess.RoomID)
		initEvents = append(initEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  sess.CharName,
			Target:    inst.Name,
			Narrative: fmt.Sprintf("You queue an attack against %s. Actions remaining: %d.", inst.Name, cbt.ActionQueues[uid].Remaining),
		})
		return initEvents, nil
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAttack, Target: inst.Name}); err != nil {
		return nil, err
	}

	events := []*gamev1.CombatEvent{{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Target:    inst.Name,
		Narrative: fmt.Sprintf("You queue an attack against %s. Actions remaining: %d.", inst.Name, cbt.ActionQueues[uid].Remaining),
	}}

	if cbt.AllActionsSubmitted() {
		h.cancelTimer(sess.RoomID)
		h.resolveAndAdvance(sess.RoomID)
		return events, nil
	}
	return events, nil
}

// Strike queues a 2-AP strike action for the player (two attacks with MAP).
//
// Postcondition: Returns queue-confirmation event; error if no combat or insufficient AP.
func (h *CombatHandler) Strike(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}
	if inst.IsDead() {
		return nil, fmt.Errorf("%s is already dead", inst.Name)
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionStrike, Target: inst.Name}); err != nil {
		return nil, err
	}

	events := []*gamev1.CombatEvent{{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Target:    inst.Name,
		Narrative: fmt.Sprintf("You ready a full strike against %s. Actions remaining: %d.", inst.Name, cbt.ActionQueues[uid].Remaining),
	}}

	if cbt.AllActionsSubmitted() {
		h.cancelTimer(sess.RoomID)
		h.resolveAndAdvance(sess.RoomID)
		return events, nil
	}
	return events, nil
}

// Pass forfeits the player's remaining action points for this round.
//
// Postcondition: Returns pass-confirmation event; triggers early resolution if all submitted.
func (h *CombatHandler) Pass(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		return nil, err
	}

	events := []*gamev1.CombatEvent{{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Narrative: "You pass your remaining actions.",
	}}

	if cbt.AllActionsSubmitted() {
		h.cancelTimer(sess.RoomID)
		h.resolveAndAdvance(sess.RoomID)
		return events, nil
	}
	return events, nil
}

// Flee attempts to remove the player from combat using an opposed Athletics check.
//
// Postcondition: Returns events describing the flee attempt outcome.
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	playerCbt := h.findCombatant(cbt, uid)
	if playerCbt == nil {
		return nil, fmt.Errorf("you are not a combatant")
	}

	playerRoll, _ := h.dice.RollExpr("d20")
	playerTotal := playerRoll.Total() + playerCbt.StrMod

	bestNPC := h.bestNPCCombatant(cbt)
	npcTotal := 0
	if bestNPC != nil {
		npcRoll, _ := h.dice.RollExpr("d20")
		npcTotal = npcRoll.Total() + bestNPC.StrMod
	}

	var events []*gamev1.CombatEvent
	if playerTotal > npcTotal {
		h.removeCombatant(cbt, uid)
		if !cbt.HasLivingPlayers() {
			h.cancelTimer(sess.RoomID)
			h.engine.EndCombat(sess.RoomID)
		}
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  sess.CharName,
			Narrative: fmt.Sprintf("%s breaks free and runs!", sess.CharName),
		})
	} else {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  sess.CharName,
			Narrative: fmt.Sprintf("%s tries to flee but can't escape!", sess.CharName),
		})
	}
	return events, nil
}

// resolveAndAdvance resolves the current round and either starts the next or ends combat.
func (h *CombatHandler) resolveAndAdvance(roomID string) {
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return
	}

	roundEvents := combat.ResolveRound(cbt, h.dice.Src(), func(id string, hp int) {
		if inst := h.npcMgr.Get2(id); inst != nil {
			inst.CurrentHP = hp
			return
		}
		if sess, ok := h.sessions.GetPlayer(id); ok {
			sess.CurrentHP = hp
		}
	})

	var events []*gamev1.CombatEvent
	for _, re := range roundEvents {
		e := roundEventToProto(re)
		events = append(events, e)
		// Emit death events for kills
		target := h.findCombatant(cbt, re.ActorID)
		_ = target
	}

	// Emit death events for newly dead combatants
	for _, c := range cbt.Combatants {
		if c.IsDead() {
			// Check if we already have a death event
			alreadyDead := false
			for _, e := range events {
				if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH && e.Target == c.Name {
					alreadyDead = true
					break
				}
			}
			if !alreadyDead {
				// Was just killed this round
				found := false
				for _, re := range roundEvents {
					if re.AttackResult != nil && re.AttackResult.TargetID == c.ID && re.AttackResult.EffectiveDamage() >= c.MaxHP {
						found = true
					}
				}
				if found {
					events = append(events, &gamev1.CombatEvent{
						Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
						Target:    c.Name,
						Narrative: fmt.Sprintf("%s falls.", c.Name),
					})
				}
			}
		}
	}

	// Round end event
	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
		Narrative: fmt.Sprintf("=== Round %d resolved. ===", cbt.Round),
	})

	h.broadcastFn(roomID, events)

	if !cbt.HasLivingNPCs() || !cbt.HasLivingPlayers() {
		h.engine.EndCombat(roomID)
		finalMsg := "Combat is over. You stand victorious."
		if !cbt.HasLivingPlayers() {
			finalMsg = "Everything goes dark."
		}
		h.broadcastFn(roomID, []*gamev1.CombatEvent{{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
			Narrative: finalMsg,
		}})
		return
	}

	// Start next round
	cbt.StartRound(3)
	h.autoQueueNPCs(cbt)

	turnOrder := make([]string, 0, len(cbt.Combatants))
	for _, c := range cbt.LivingCombatants() {
		turnOrder = append(turnOrder, c.Name)
	}
	roundStartEvents := []*gamev1.CombatEvent{{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
		Narrative: fmt.Sprintf("=== Round %d begins. Actions: 3. [%.0fs] === Turn order: %v", cbt.Round, h.roundDuration.Seconds(), turnOrder),
	}}
	h.broadcastFn(roomID, roundStartEvents)
	h.startTimer(roomID)
}

func (h *CombatHandler) startCombat(sess *session.PlayerSession, inst *npc.Instance) (*combat.Combat, []*gamev1.CombatEvent) {
	playerCbt := &combat.Combatant{
		ID:        sess.UID,
		Kind:      combat.KindPlayer,
		Name:      sess.CharName,
		MaxHP:     sess.CurrentHP,
		CurrentHP: sess.CurrentHP,
		AC:        12,
		Level:     1,
		StrMod:    2,
		DexMod:    1,
	}
	npcCbt := &combat.Combatant{
		ID:        inst.ID,
		Kind:      combat.KindNPC,
		Name:      inst.Name,
		MaxHP:     inst.MaxHP,
		CurrentHP: inst.CurrentHP,
		AC:        inst.AC,
		Level:     inst.Level,
		StrMod:    combat.AbilityMod(inst.Perception),
		DexMod:    1,
	}

	combatants := []*combat.Combatant{playerCbt, npcCbt}
	combat.RollInitiative(combatants, h.dice.Src())

	cbt, _ := h.engine.StartCombat(sess.RoomID, combatants)
	cbt.StartRound(3)

	var events []*gamev1.CombatEvent
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Attacker:  c.Name,
			Narrative: fmt.Sprintf("%s rolls initiative: %d", c.Name, c.Initiative),
		})
	}

	turnOrder := make([]string, 0, len(cbt.Combatants))
	for _, c := range cbt.LivingCombatants() {
		turnOrder = append(turnOrder, c.Name)
	}
	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
		Narrative: fmt.Sprintf("=== Round 1 begins. Actions: 3. [%.0fs] === Turn order: %v", h.roundDuration.Seconds(), turnOrder),
	})

	return cbt, events
}

// autoQueueNPCs gives each living NPC a single attack action targeting the first living player.
func (h *CombatHandler) autoQueueNPCs(cbt *combat.Combat) {
	var playerTarget string
	for _, c := range cbt.LivingCombatants() {
		if c.Kind == combat.KindPlayer {
			playerTarget = c.Name
			break
		}
	}
	if playerTarget == "" {
		return
	}
	for _, c := range cbt.LivingCombatants() {
		if c.Kind == combat.KindNPC {
			_ = cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: playerTarget})
		}
	}
}

func (h *CombatHandler) startTimer(roomID string) {
	h.timersMu.Lock()
	defer h.timersMu.Unlock()
	if existing, ok := h.timers[roomID]; ok {
		existing.Stop()
	}
	h.timers[roomID] = combat.NewRoundTimer(h.roundDuration, func() {
		h.resolveAndAdvance(roomID)
	})
}

func (h *CombatHandler) cancelTimer(roomID string) {
	h.timersMu.Lock()
	defer h.timersMu.Unlock()
	if t, ok := h.timers[roomID]; ok {
		t.Stop()
		delete(h.timers, roomID)
	}
}

func (h *CombatHandler) findCombatant(cbt *combat.Combat, id string) *combat.Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (h *CombatHandler) bestNPCCombatant(cbt *combat.Combat) *combat.Combatant {
	var best *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC && !c.IsDead() {
			if best == nil || c.StrMod > best.StrMod {
				best = c
			}
		}
	}
	return best
}

func (h *CombatHandler) removeCombatant(cbt *combat.Combat, id string) {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			c.CurrentHP = 0
			return
		}
	}
}

func roundEventToProto(re combat.RoundEvent) *gamev1.CombatEvent {
	e := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  re.ActorName,
		Narrative: re.Narrative,
	}
	if re.AttackResult != nil {
		e.AttackRoll = int32(re.AttackResult.AttackRoll)
		e.AttackTotal = int32(re.AttackResult.AttackTotal)
		e.Outcome = re.AttackResult.Outcome.String()
		e.Damage = int32(re.AttackResult.EffectiveDamage())
	}
	if re.ActionType == combat.ActionPass {
		e.Type = gamev1.CombatEventType_COMBAT_EVENT_TYPE_UNSPECIFIED
	}
	return e
}
```

Note: `npc.Manager.Get2(id string)` is needed (returns `*npc.Instance` by ID, non-bool form). Add it to `npc/manager.go`:

```go
// Get2 returns the Instance with the given ID, or nil if not found.
func (m *Manager) Get2(id string) *npc.Instance {
    inst, _ := m.Get(id)
    return inst
}
```

Wait — the existing `Get(id string) (*Instance, bool)` already exists. Just use it inline:

```go
if inst, ok := h.npcMgr.Get(id); ok {
    inst.CurrentHP = hp
    return
}
```

Update the `targetUpdater` in `resolveAndAdvance` accordingly.

**Step 4: Run tests**

```
go test ./internal/gameserver/... -count=1 -race -run TestCombatHandler -v 2>&1 | grep -E "PASS|FAIL|Error"
```
Expected: all PASS.

**Step 5: Commit**

```
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_test.go
git commit -m "feat(gameserver): CombatHandler — 3-AP queue, round timer, Strike/Pass (Stage 4 Task 6)"
```

---

### Task 7: GameServiceServer wiring

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`
- Modify: `internal/config/config.go`
- Modify: `configs/dev.yaml`

**Step 1: Add `RoundDurationMs` to config**

In `internal/config/config.go`, update `GameServerConfig`:

```go
type GameServerConfig struct {
	GRPCHost        string `mapstructure:"grpc_host"`
	GRPCPort        int    `mapstructure:"grpc_port"`
	RoundDurationMs int    `mapstructure:"round_duration_ms"`
}
```

In `configs/dev.yaml`, add under `gameserver:`:
```yaml
gameserver:
  grpc_host: 127.0.0.1
  grpc_port: 50051
  round_duration_ms: 6000
```

**Step 2: Update `cmd/gameserver/main.go`**

Find where `NewCombatHandler` is called and update the call to pass `broadcastFn` and `roundDuration`. The `broadcastFn` must be set after `GameServiceServer` is created; use a closure:

```go
roundDuration := time.Duration(cfg.GameServer.RoundDurationMs) * time.Millisecond
if roundDuration <= 0 {
    roundDuration = 6 * time.Second
}

// broadcastFn is set after grpcServer is created (closure captures pointer)
var grpcServer *gameserver.GameServiceServer
broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
    if grpcServer != nil {
        grpcServer.BroadcastCombatEvents(roomID, events)
    }
}

combatHandler := gameserver.NewCombatHandler(
    combatEngine, npcManager, sessionManager, roller,
    broadcastFn,
    roundDuration,
)
grpcServer = gameserver.NewGameServiceServer(/* existing args */, combatHandler)
```

**Step 3: Add `BroadcastCombatEvents` to `GameServiceServer`**

In `internal/gameserver/grpc_service.go`, add a public method:

```go
// BroadcastCombatEvents sends combat events to all players in the room.
// This is called by CombatHandler's round timer callback.
//
// Postcondition: Each event is delivered to all sessions in roomID.
func (s *GameServiceServer) BroadcastCombatEvents(roomID string, events []*gamev1.CombatEvent) {
	for _, evt := range events {
		s.broadcastCombatEvent(roomID, "", evt)
	}
}
```

**Step 4: Add dispatch cases for `Pass` and `Strike`**

In the `ClientMessage` dispatch switch in `grpc_service.go`:

```go
case *gamev1.ClientMessage_Pass:
    resp, err = s.handlePass(uid)
case *gamev1.ClientMessage_Strike:
    resp, err = s.handleStrike(uid, msg.GetStrike())
```

Add the handlers:

```go
func (s *GameServiceServer) handlePass(uid string) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Pass(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) handleStrike(uid string, req *gamev1.StrikeRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Strike(uid, req.GetTarget())
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}
```

**Step 5: Build and run tests**

```
go build ./... && go test ./... -count=1 -race 2>&1 | grep -E "^(ok|FAIL)"
```
Expected: all packages pass.

**Step 6: Commit**

```
git add internal/config/config.go configs/dev.yaml internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat(gameserver): wire Pass/Strike dispatch, BroadcastCombatEvents, round_duration_ms config (Stage 4 Task 7)"
```

---

### Task 8: Frontend commands + renderer

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Write the failing renderer tests**

Add to `internal/frontend/handlers/text_renderer_test.go`:

```go
func TestRenderRoundStartEvent(t *testing.T) {
	evt := &gamev1.RoundStartEvent{
		Round:          1,
		ActionsPerTurn: 3,
		DurationMs:     6000,
		TurnOrder:      []string{"Alice", "Ganger"},
	}
	result := handlers.RenderRoundStartEvent(evt)
	assert.Contains(t, result, "Round 1")
	assert.Contains(t, result, "Actions: 3")
	assert.Contains(t, result, "6s")
	assert.Contains(t, result, "Alice")
}

func TestRenderRoundEndEvent(t *testing.T) {
	evt := &gamev1.RoundEndEvent{Round: 2}
	result := handlers.RenderRoundEndEvent(evt)
	assert.Contains(t, result, "Round 2")
	assert.Contains(t, result, "resolved")
}
```

**Step 2: Run to verify failure**

```
go test ./internal/frontend/handlers/... -run TestRenderRound -v
```
Expected: FAIL — `handlers.RenderRoundStartEvent` undefined.

**Step 3: Add renderer functions to `internal/frontend/handlers/text_renderer.go`**

```go
// RenderRoundStartEvent formats a round-start combat banner.
//
// Postcondition: Returns an ANSI-colored string showing round number, action count, and timer.
func RenderRoundStartEvent(rs *gamev1.RoundStartEvent) string {
	durationSec := int(rs.DurationMs / 1000)
	order := strings.Join(rs.TurnOrder, ", ")
	return telnet.Colorize(telnet.BrightYellow,
		fmt.Sprintf("=== Round %d begins. Actions: %d. [%ds] ===", rs.Round, rs.ActionsPerTurn, durationSec),
	) + "\r\n" +
		telnet.Colorize(telnet.White, "Turn order: "+order) + "\r\n"
}

// RenderRoundEndEvent formats a round-end combat banner.
//
// Postcondition: Returns an ANSI-colored string indicating round resolution.
func RenderRoundEndEvent(re *gamev1.RoundEndEvent) string {
	return telnet.Colorize(telnet.BrightYellow, fmt.Sprintf("=== Round %d resolved. ===", re.Round)) + "\r\n"
}
```

**Step 4: Add commands to `internal/game/command/commands.go`**

Add handler constants:
```go
const (
    // existing ...
    HandlerPass   = "pass"
    HandlerStrike = "strike"
)
```

Register commands in the `init` or `NewRegistry` setup (wherever `attack` and `flee` are registered):
```go
{Name: "pass",   Aliases: []string{"p"},  Help: "Forfeit remaining actions this round.",  Category: "combat",   Handler: HandlerPass},
{Name: "strike", Aliases: []string{"st"}, Help: "Full attack routine (2 AP) against target.", Category: "combat", Handler: HandlerStrike},
```

**Step 5: Add dispatch cases in `internal/frontend/handlers/game_bridge.go`**

In the command dispatch switch:
```go
case command.HandlerPass:
    msg = &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}},
    }

case command.HandlerStrike:
    if len(args) == 0 {
        _ = conn.WriteLine(telnet.Colorize(telnet.Red, "Usage: strike <target>"))
        continue
    }
    msg = &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_Strike{
            Strike: &gamev1.StrikeRequest{Target: strings.Join(args, " ")},
        },
    }
```

In `forwardServerEvents`, add cases for the new ServerEvent types:
```go
case *gamev1.ServerEvent_RoundStart:
    line = handlers.RenderRoundStartEvent(payload.RoundStart)
case *gamev1.ServerEvent_RoundEnd:
    line = handlers.RenderRoundEndEvent(payload.RoundEnd)
```

Note: `RoundStartEvent` and `RoundEndEvent` are currently broadcast as `CombatEvent` narratives in the handler (Task 6 implementation). The proto fields `round_start` (field 12) and `round_end` (field 13) on `ServerEvent` are defined but the gameserver uses `CombatEvent` narratives for now. The renderer cases above are for future use; they will become active when `grpc_service.go` sends them as structured events in a later cleanup pass.

**Step 6: Run tests**

```
go test ./internal/frontend/... -count=1 -race 2>&1 | grep -E "^(ok|FAIL)"
```
Expected: all PASS.

**Step 7: Commit**

```
git add internal/game/command/commands.go internal/frontend/handlers/game_bridge.go internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat(frontend): pass/strike commands, RenderRoundStartEvent/RoundEndEvent (Stage 4 Task 8)"
```

---

### Task 9: Final verification

**Step 1: Full test suite**

```
go test ./... -count=1 -race 2>&1 | grep -E "^(ok|FAIL)"
```
Expected: all packages report `ok`.

**Step 2: Build**

```
make build
```
Expected: `bin/frontend` and `bin/gameserver` produced without errors.

**Step 3: Docker**

```
docker compose -f deployments/docker/docker-compose.yml up --build -d
docker compose -f deployments/docker/docker-compose.yml logs -f frontend gameserver
```
Expected: both containers start, no fatal errors in logs.

**Step 4: Telnet smoke test**

Connect with `telnet localhost 4000` or TinTin++. Login, walk to a room with NPCs.

1. `attack ganger` → see initiative rolls + `=== Round 1 begins. Actions: 3. [6s] ===` + "You queue an attack"
2. Wait 6 seconds → see round resolution: attack results, then `=== Round 1 resolved. ===` → `=== Round 2 begins...`
3. `attack ganger` then `pass` → round resolves immediately (all submitted)
4. `strike ganger` → see "Actions remaining: 1" (2 AP spent)
5. `flee` → still works immediately, exits combat

**Step 5: Commit if anything was cleaned up**

```
git add -A && git status  # verify nothing unintended
git commit -m "chore: stage 4 final verification clean-up" --allow-empty
```

---

## Verification Checklist

- [ ] `go test ./internal/game/combat/... -count=1 -race` — all new combat tests pass
- [ ] `go test ./internal/gameserver/... -count=1 -race` — handler tests pass
- [ ] `go test ./internal/frontend/... -count=1 -race` — renderer tests pass
- [ ] `go test ./... -count=1 -race` — full suite passes (no regressions)
- [ ] `make build` — both binaries build cleanly
- [ ] `docker compose up --build` — containers start without error
- [ ] Telnet: `attack <npc>` starts combat and displays round-start banner with timer
- [ ] Telnet: waiting 6s auto-resolves round; next round begins automatically
- [ ] Telnet: `attack`, then `pass` resolves round immediately
- [ ] Telnet: `strike <npc>` queues 2-AP action; "Actions remaining: 1"
- [ ] Telnet: `flee` exits combat immediately (no timer involvement)
- [ ] Telnet: second player in same room sees broadcast round events
