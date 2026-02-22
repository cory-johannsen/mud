# Combat Stage 3 — MVP Combat Loop (PvE) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** First playable PvE combat — players can `attack <npc>`, initiative is rolled, turns alternate, HP is tracked, combatants die, and `flee` escapes.

**Architecture:** A new `internal/game/combat` package owns all combat state: the `Engine` holds active `Combat` instances keyed by room ID, each containing an initiative-ordered combatant list, current-turn pointer, and HP tracking. The engine exposes pure functions for attack resolution (4-tier PF2E success) and damage. The `CombatHandler` in `internal/gameserver` bridges gRPC commands to the engine, broadcasts `CombatEvent` protos to all players in the room, and drives simple NPC turns synchronously after each player action. No round timer yet (Stage 4).

**Tech Stack:** Go, `internal/game/dice` (existing `*dice.Roller`), `pgregory.net/rapid` for property-based tests, Protocol Buffers (new `AttackRequest`, `FleeRequest`, `CombatEvent` messages).

**Key domain rules:**
- Initiative: `d20 + DEX modifier`; DEX modifier = `(dex - 10) / 2`
- Proficiency bonus at level N: `2 + (N-1)/4` (PF2E simplified)
- Attack roll: `d20 + STR_or_DEX_mod + proficiency` vs target AC
- PF2E 4-tier: roll ≥ AC+10 → crit success (×2 dmg); roll ≥ AC → success (full dmg); roll ≥ AC-10 → failure (miss); roll < AC-10 → crit failure (miss + special narrative)
- Damage: `1d6 + STR_mod` (unarmed baseline; weapons come in Stage 7)
- NPC uses its own Abilities for modifiers; player uses `PlayerSession.CurrentHP` for HP tracking

---

### Task 1: Combat Domain Types

**Files:**
- Create: `internal/game/combat/combat.go`
- Create: `internal/game/combat/combat_test.go`

**Step 1: Write the failing tests**

Create `internal/game/combat/combat_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// --- Combatant ---

func TestCombatant_IsPlayer(t *testing.T) {
	p := combat.Combatant{Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20}
	n := combat.Combatant{Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18}
	assert.True(t, p.IsPlayer())
	assert.False(t, n.IsPlayer())
}

func TestCombatant_IsDead(t *testing.T) {
	c := combat.Combatant{Kind: combat.KindPlayer, Name: "X", MaxHP: 10, CurrentHP: 0}
	assert.True(t, c.IsDead())
	c.CurrentHP = 1
	assert.False(t, c.IsDead())
}

func TestCombatant_ApplyDamage(t *testing.T) {
	c := combat.Combatant{Kind: combat.KindNPC, Name: "G", MaxHP: 18, CurrentHP: 18}
	c.ApplyDamage(5)
	assert.Equal(t, 13, c.CurrentHP)
	c.ApplyDamage(20)
	assert.Equal(t, 0, c.CurrentHP) // floors at 0
}

func TestCombatant_Property_DamageNeverBelowZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxHP := rapid.IntRange(1, 200).Draw(rt, "max_hp")
		dmg := rapid.IntRange(0, 500).Draw(rt, "dmg")
		c := combat.Combatant{Kind: combat.KindNPC, Name: "X", MaxHP: maxHP, CurrentHP: maxHP}
		c.ApplyDamage(dmg)
		assert.GreaterOrEqual(rt, c.CurrentHP, 0)
	})
}

// --- OutcomeFor ---

func TestOutcomeFor(t *testing.T) {
	tests := []struct {
		roll int
		ac   int
		want combat.Outcome
	}{
		{30, 15, combat.CritSuccess},  // >= AC+10 (25)
		{25, 15, combat.CritSuccess},  // exactly AC+10
		{20, 15, combat.Success},      // >= AC
		{15, 15, combat.Success},      // exactly AC
		{10, 15, combat.Failure},      // >= AC-10 (5)
		{5, 15, combat.Failure},       // exactly AC-10
		{4, 15, combat.CritFailure},   // < AC-10
		{1, 15, combat.CritFailure},
	}
	for _, tc := range tests {
		got := combat.OutcomeFor(tc.roll, tc.ac)
		assert.Equal(t, tc.want, got, "roll=%d ac=%d", tc.roll, tc.ac)
	}
}

func TestOutcomeFor_Property_AllRollsMapToAnOutcome(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 40).Draw(rt, "roll")
		ac := rapid.IntRange(10, 30).Draw(rt, "ac")
		out := combat.OutcomeFor(roll, ac)
		assert.Contains(rt, []combat.Outcome{
			combat.CritSuccess, combat.Success, combat.Failure, combat.CritFailure,
		}, out)
	})
}

// --- ProficiencyBonus ---

func TestProficiencyBonus(t *testing.T) {
	tests := []struct{ level, want int }{
		{1, 2}, {2, 2}, {3, 2}, {4, 2},
		{5, 3}, {6, 3}, {7, 3}, {8, 3},
		{9, 4}, {17, 6}, {20, 6},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, combat.ProficiencyBonus(tc.level), "level=%d", tc.level)
	}
}

func TestProficiencyBonus_Property_AlwaysAtLeastTwo(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		assert.GreaterOrEqual(rt, combat.ProficiencyBonus(level), 2)
	})
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | head -5
```
Expected: build error — package does not exist.

**Step 3: Write the implementation**

Create `internal/game/combat/combat.go`:

```go
// Package combat implements the PvE combat engine for Gunchete.
package combat

// Kind distinguishes player combatants from NPC combatants.
type Kind int

const (
	KindPlayer Kind = iota
	KindNPC
)

// Outcome is the PF2E 4-tier attack result.
type Outcome int

const (
	CritSuccess Outcome = iota
	Success
	Failure
	CritFailure
)

// String returns a human-readable outcome label.
func (o Outcome) String() string {
	switch o {
	case CritSuccess:
		return "critical success"
	case Success:
		return "success"
	case Failure:
		return "failure"
	case CritFailure:
		return "critical failure"
	default:
		return "unknown"
	}
}

// Combatant represents one participant in a combat — either a player or an NPC instance.
type Combatant struct {
	// ID is the unique identifier: UID for players, instance ID for NPCs.
	ID string
	// Kind distinguishes player from NPC.
	Kind Kind
	// Name is the display name.
	Name string
	// MaxHP is the combatant's maximum hit points.
	MaxHP int
	// CurrentHP is the combatant's current hit points.
	CurrentHP int
	// AC is the combatant's armor class.
	AC int
	// Level is the combatant's level (used for proficiency bonus).
	Level int
	// StrMod is the Strength ability modifier.
	StrMod int
	// DexMod is the Dexterity ability modifier.
	DexMod int
	// Initiative is the rolled initiative value used to order combatants.
	Initiative int
}

// IsPlayer reports whether this combatant is a player character.
func (c *Combatant) IsPlayer() bool { return c.Kind == KindPlayer }

// IsDead reports whether this combatant has zero or fewer hit points.
func (c *Combatant) IsDead() bool { return c.CurrentHP <= 0 }

// ApplyDamage reduces CurrentHP by amount, flooring at zero.
//
// Precondition: amount must be >= 0.
// Postcondition: CurrentHP >= 0.
func (c *Combatant) ApplyDamage(amount int) {
	c.CurrentHP -= amount
	if c.CurrentHP < 0 {
		c.CurrentHP = 0
	}
}

// OutcomeFor determines the PF2E 4-tier attack outcome for a given roll vs AC.
//
// Precondition: roll >= 1; ac >= 10.
// Postcondition: Returns one of CritSuccess, Success, Failure, CritFailure.
func OutcomeFor(roll, ac int) Outcome {
	switch {
	case roll >= ac+10:
		return CritSuccess
	case roll >= ac:
		return Success
	case roll >= ac-10:
		return Failure
	default:
		return CritFailure
	}
}

// ProficiencyBonus returns the PF2E simplified proficiency bonus for the given level.
// Formula: 2 + (level-1)/4, capped at level 20.
//
// Precondition: level >= 1.
// Postcondition: Returns >= 2.
func ProficiencyBonus(level int) int {
	return 2 + (level-1)/4
}

// AbilityMod computes the standard ability modifier: (score - 10) / 2.
func AbilityMod(score int) int {
	return (score - 10) / 2
}
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v -count=1 2>&1
```
Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/game/combat/combat.go internal/game/combat/combat_test.go
git commit -m "feat(combat): core domain types — Combatant, OutcomeFor, ProficiencyBonus (Task 1)"
```

---

### Task 2: Combat Engine — State Machine

**Files:**
- Create: `internal/game/combat/engine.go`
- Modify: `internal/game/combat/combat_test.go` (append engine tests)

**Step 1: Append engine tests**

```go
// --- Engine ---

func makeCombatants() []*combat.Combatant {
	return []*combat.Combatant{
		{ID: "player-1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "npc-ganger-1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 13, Level: 1, StrMod: 2, DexMod: 1},
	}
}

func TestEngine_StartCombat(t *testing.T) {
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room-alley", makeCombatants())
	require.NoError(t, err)
	assert.Equal(t, "room-alley", cbt.RoomID)
	assert.Len(t, cbt.Combatants, 2)
	assert.False(t, cbt.Over)
}

func TestEngine_StartCombat_DuplicateRoom(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants())
	require.NoError(t, err)
	_, err = eng.StartCombat("room-1", makeCombatants())
	assert.Error(t, err)
}

func TestEngine_GetCombat(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants())
	require.NoError(t, err)

	cbt, ok := eng.GetCombat("room-1")
	assert.True(t, ok)
	assert.Equal(t, "room-1", cbt.RoomID)

	_, ok = eng.GetCombat("room-missing")
	assert.False(t, ok)
}

func TestEngine_EndCombat(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants())
	require.NoError(t, err)

	eng.EndCombat("room-1")
	_, ok := eng.GetCombat("room-1")
	assert.False(t, ok)
}

func TestCombat_CurrentTurn(t *testing.T) {
	eng := combat.NewEngine()
	cbt, _ := eng.StartCombat("room-1", makeCombatants())
	// CurrentTurn returns the combatant whose turn it is.
	current := cbt.CurrentTurn()
	require.NotNil(t, current)
	assert.NotEmpty(t, current.ID)
}

func TestCombat_AdvanceTurn(t *testing.T) {
	eng := combat.NewEngine()
	cbt, _ := eng.StartCombat("room-1", makeCombatants())
	first := cbt.CurrentTurn()
	cbt.AdvanceTurn()
	second := cbt.CurrentTurn()
	assert.NotEqual(t, first.ID, second.ID)
}

func TestCombat_AdvanceTurn_SkipsDead(t *testing.T) {
	eng := combat.NewEngine()
	c := makeCombatants()
	c[1].CurrentHP = 0 // NPC is dead
	cbt, _ := eng.StartCombat("room-1", c)
	// With only one living combatant, CurrentTurn is always the player
	for i := 0; i < 5; i++ {
		current := cbt.CurrentTurn()
		assert.Equal(t, combat.KindPlayer, current.Kind)
		cbt.AdvanceTurn()
	}
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestEngine -v 2>&1 | head -10
```
Expected: compile error — `combat.NewEngine` undefined.

**Step 3: Write the implementation**

Create `internal/game/combat/engine.go`:

```go
package combat

import (
	"fmt"
	"sync"
)

// Combat holds the live state of a single combat encounter in a room.
type Combat struct {
	// RoomID is the room where this combat takes place.
	RoomID string
	// Combatants is the initiative-ordered list of participants.
	// Order is set at StartCombat and does not change during combat.
	Combatants []*Combatant
	// turnIndex is the index into Combatants of the current actor.
	turnIndex int
	// Over is true when combat has been resolved.
	Over bool
}

// CurrentTurn returns the combatant whose turn it currently is.
// It always returns a living combatant; if the current slot is dead it advances first.
//
// Postcondition: Returns a non-nil living combatant, or nil if all are dead.
func (c *Combat) CurrentTurn() *Combatant {
	for range c.Combatants {
		cbt := c.Combatants[c.turnIndex]
		if !cbt.IsDead() {
			return cbt
		}
		c.turnIndex = (c.turnIndex + 1) % len(c.Combatants)
	}
	return nil
}

// AdvanceTurn moves to the next combatant in initiative order, skipping dead ones.
//
// Postcondition: turnIndex points to the next living combatant (or wraps).
func (c *Combat) AdvanceTurn() {
	c.turnIndex = (c.turnIndex + 1) % len(c.Combatants)
}

// LivingCombatants returns a snapshot of combatants with CurrentHP > 0.
func (c *Combat) LivingCombatants() []*Combatant {
	var alive []*Combatant
	for _, cbt := range c.Combatants {
		if !cbt.IsDead() {
			alive = append(alive, cbt)
		}
	}
	return alive
}

// HasLivingNPCs reports whether any NPC combatant is still alive.
func (c *Combat) HasLivingNPCs() bool {
	for _, cbt := range c.Combatants {
		if cbt.Kind == KindNPC && !cbt.IsDead() {
			return true
		}
	}
	return false
}

// HasLivingPlayers reports whether any player combatant is still alive.
func (c *Combat) HasLivingPlayers() bool {
	for _, cbt := range c.Combatants {
		if cbt.Kind == KindPlayer && !cbt.IsDead() {
			return true
		}
	}
	return false
}

// Engine manages all active Combat encounters, keyed by room ID.
// All methods are safe for concurrent use.
type Engine struct {
	mu      sync.RWMutex
	combats map[string]*Combat // roomID → Combat
}

// NewEngine creates an empty combat Engine.
func NewEngine() *Engine {
	return &Engine{combats: make(map[string]*Combat)}
}

// StartCombat begins a new combat in roomID with the given combatants.
// Combatants are sorted by Initiative descending before storing.
//
// Precondition: roomID must be non-empty; combatants must have at least 2 entries.
// Postcondition: Returns the new Combat or an error if combat is already active in roomID.
func (e *Engine) StartCombat(roomID string, combatants []*Combatant) (*Combat, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.combats[roomID]; exists {
		return nil, fmt.Errorf("combat already active in room %q", roomID)
	}

	// Sort by initiative descending (highest goes first).
	sorted := make([]*Combatant, len(combatants))
	copy(sorted, combatants)
	sortByInitiativeDesc(sorted)

	cbt := &Combat{
		RoomID:     roomID,
		Combatants: sorted,
	}
	e.combats[roomID] = cbt
	return cbt, nil
}

// GetCombat returns the active combat in roomID.
//
// Postcondition: Returns (combat, true) if found, or (nil, false) otherwise.
func (e *Engine) GetCombat(roomID string) (*Combat, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cbt, ok := e.combats[roomID]
	return cbt, ok
}

// EndCombat removes the combat record for roomID.
//
// Precondition: roomID must be non-empty.
func (e *Engine) EndCombat(roomID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.combats, roomID)
}

// sortByInitiativeDesc sorts combatants in place, highest initiative first.
func sortByInitiativeDesc(combatants []*Combatant) {
	n := len(combatants)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && combatants[j].Initiative > combatants[j-1].Initiative; j-- {
			combatants[j], combatants[j-1] = combatants[j-1], combatants[j]
		}
	}
}
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v -count=1 2>&1
```
Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/game/combat/engine.go internal/game/combat/combat_test.go
git commit -m "feat(combat): Engine state machine — StartCombat, turns, AdvanceTurn (Task 2)"
```

---

### Task 3: Attack Resolution (pure function)

**Files:**
- Create: `internal/game/combat/resolver.go`
- Modify: `internal/game/combat/combat_test.go` (append resolver tests)

**Step 1: Append resolver tests**

```go
// --- AttackResult / Resolve ---

func TestAttackResult_DamageByOutcome(t *testing.T) {
	tests := []struct {
		outcome combat.Outcome
		base    int
		want    int
	}{
		{combat.CritSuccess, 6, 12},
		{combat.Success, 6, 6},
		{combat.Failure, 6, 0},
		{combat.CritFailure, 6, 0},
	}
	for _, tc := range tests {
		ar := combat.AttackResult{Outcome: tc.outcome, BaseDamage: tc.base}
		assert.Equal(t, tc.want, ar.EffectiveDamage(), "outcome=%s base=%d", tc.outcome, tc.base)
	}
}

func TestAttackResult_Property_EffectiveDamageNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		outcome := combat.Outcome(rapid.IntRange(0, 3).Draw(rt, "outcome"))
		base := rapid.IntRange(0, 50).Draw(rt, "base")
		ar := combat.AttackResult{Outcome: outcome, BaseDamage: base}
		assert.GreaterOrEqual(rt, ar.EffectiveDamage(), 0)
	})
}

func TestResolveAttack_HitDealsPositiveDamage(t *testing.T) {
	// Use a deterministic source that always returns max value.
	attacker := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "A",
		MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 3, DexMod: 1}
	target := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "G",
		MaxHP: 18, CurrentHP: 18, AC: 10, Level: 1, StrMod: 2, DexMod: 1}

	// Roll d20=20 → total = 20 + 3 (str) + 2 (prof) = 25 vs AC 10 → crit success
	src := &fixedSource{val: 19} // Intn(20) → 19, +1 = 20; Intn(6) → 5, +1 = 6
	result := combat.ResolveAttack(attacker, target, src)
	assert.Equal(t, combat.CritSuccess, result.Outcome)
	assert.Greater(t, result.EffectiveDamage(), 0)
}

// fixedSource always returns val for any Intn call.
type fixedSource struct{ val int }

func (f *fixedSource) Intn(n int) int {
	if f.val >= n {
		return n - 1
	}
	return f.val
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestAttackResult -v 2>&1 | head -10
```
Expected: compile error — `combat.AttackResult` undefined.

**Step 3: Write the implementation**

Create `internal/game/combat/resolver.go`:

```go
package combat

// AttackResult holds the outcome of a single attack action.
type AttackResult struct {
	// AttackerID is the attacking combatant's ID.
	AttackerID string
	// TargetID is the defending combatant's ID.
	TargetID string
	// AttackRoll is the raw d20 result before modifiers.
	AttackRoll int
	// AttackTotal is the full attack roll: d20 + modifiers.
	AttackTotal int
	// Outcome is the PF2E 4-tier result.
	Outcome Outcome
	// BaseDamage is the raw damage roll (before crit doubling).
	BaseDamage int
	// DamageRoll is the individual die values.
	DamageRoll []int
}

// EffectiveDamage returns the damage dealt after applying the outcome multiplier.
//
// Postcondition: Returns >= 0.
func (r AttackResult) EffectiveDamage() int {
	switch r.Outcome {
	case CritSuccess:
		return r.BaseDamage * 2
	case Success:
		return r.BaseDamage
	default:
		return 0
	}
}

// Source is the subset of dice.Source used by the resolver — avoids a circular import.
type Source interface {
	Intn(n int) int
}

// ResolveAttack performs a full attack roll and damage roll for attacker vs target.
//
// Precondition: attacker and target must be non-nil, non-dead; src must be non-nil.
// Postcondition: Returns a fully populated AttackResult.
func ResolveAttack(attacker, target *Combatant, src Source) AttackResult {
	// Attack roll: d20 + STR modifier + proficiency bonus
	d20 := src.Intn(20) + 1
	atkMod := attacker.StrMod + ProficiencyBonus(attacker.Level)
	atkTotal := d20 + atkMod
	outcome := OutcomeFor(atkTotal, target.AC)

	// Damage roll: 1d6 + STR modifier (unarmed baseline)
	dmgDie := src.Intn(6) + 1
	strMod := attacker.StrMod
	if strMod < 0 {
		strMod = 0
	}
	baseDmg := dmgDie + strMod

	return AttackResult{
		AttackerID:  attacker.ID,
		TargetID:    target.ID,
		AttackRoll:  d20,
		AttackTotal: atkTotal,
		Outcome:     outcome,
		BaseDamage:  baseDmg,
		DamageRoll:  []int{dmgDie},
	}
}
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v -count=1 2>&1
```
Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/game/combat/resolver.go internal/game/combat/combat_test.go
git commit -m "feat(combat): AttackResult and ResolveAttack pure function (Task 3)"
```

---

### Task 4: Initiative Rolling

**Files:**
- Create: `internal/game/combat/initiative.go`
- Modify: `internal/game/combat/combat_test.go` (append initiative tests)

**Step 1: Append initiative tests**

```go
// --- RollInitiative ---

func TestRollInitiative_SetsInitiativeField(t *testing.T) {
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "A", DexMod: 2},
		{ID: "n1", Kind: combat.KindNPC, Name: "G", DexMod: 1},
	}
	src := &fixedSource{val: 9} // Intn(20) → 9, +1 = 10
	combat.RollInitiative(combatants, src)

	// player: 10 + 2 = 12; npc: 10 + 1 = 11
	assert.Equal(t, 12, combatants[0].Initiative)
	assert.Equal(t, 11, combatants[1].Initiative)
}

func TestRollInitiative_Property_InitiativeAtLeastOnePlusMod(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dexMod := rapid.IntRange(-3, 5).Draw(rt, "dex_mod")
		c := &combat.Combatant{ID: "x", Kind: combat.KindNPC, Name: "X", DexMod: dexMod}
		src := &fixedSource{val: 0} // Intn(20) → 0, +1 = 1
		combat.RollInitiative([]*combat.Combatant{c}, src)
		// minimum roll is 1, so initiative >= 1 + dexMod
		assert.Equal(rt, 1+dexMod, c.Initiative)
	})
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestRollInitiative -v 2>&1 | head -10
```
Expected: compile error — `combat.RollInitiative` undefined.

**Step 3: Write the implementation**

Create `internal/game/combat/initiative.go`:

```go
package combat

// RollInitiative rolls initiative for all combatants and sets their Initiative field.
// Formula: d20 + DEX modifier.
//
// Precondition: combatants must be non-nil; src must be non-nil.
// Postcondition: Each combatant's Initiative field is set.
func RollInitiative(combatants []*Combatant, src Source) {
	for _, c := range combatants {
		roll := src.Intn(20) + 1
		c.Initiative = roll + c.DexMod
	}
}
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v -count=1 2>&1
```
Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/game/combat/initiative.go internal/game/combat/combat_test.go
git commit -m "feat(combat): RollInitiative — d20 + DEX modifier (Task 4)"
```

---

### Task 5: Proto — AttackRequest, FleeRequest, CombatEvent

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/`

**Step 1: Edit `api/proto/game/v1/game.proto`**

In `ClientMessage.payload` oneof, add after `examine = 10`:
```proto
    AttackRequest attack = 11;
    FleeRequest   flee   = 12;
```

In `ServerEvent.payload` oneof, add after `npc_view = 10`:
```proto
    CombatEvent combat_event = 11;
```

Add these new messages after the existing `NpcView` message:

```proto
// AttackRequest asks the server to attack a named target.
message AttackRequest {
  string target = 1;
}

// FleeRequest asks the server to attempt to flee combat.
message FleeRequest {}

// CombatEvent delivers combat narration to all players in the room.
message CombatEvent {
  CombatEventType type    = 1;
  string attacker         = 2;
  string target           = 3;
  int32  attack_roll      = 4;
  int32  attack_total     = 5;
  string outcome          = 6;
  int32  damage           = 7;
  int32  target_hp        = 8;
  string narrative        = 9;
}

// CombatEventType distinguishes combat narration events.
enum CombatEventType {
  COMBAT_EVENT_TYPE_UNSPECIFIED  = 0;
  COMBAT_EVENT_TYPE_INITIATIVE   = 1;
  COMBAT_EVENT_TYPE_ATTACK       = 2;
  COMBAT_EVENT_TYPE_DEATH        = 3;
  COMBAT_EVENT_TYPE_FLEE         = 4;
  COMBAT_EVENT_TYPE_END          = 5;
}
```

**Step 2: Regenerate Go bindings**

```
cd /home/cjohannsen/src/mud && mise run proto
```
If the task doesn't exist, check `mise tasks` or run protoc directly:
```
cd /home/cjohannsen/src/mud && protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  api/proto/game/v1/game.proto 2>&1
```

**Step 3: Verify gamev1 builds**

```
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/gamev1/... 2>&1
```
Expected: zero errors.

**Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(proto): AttackRequest, FleeRequest, CombatEvent messages (Task 5)"
```

---

### Task 6: CombatHandler — attack + flee + NPC turn

**Files:**
- Create: `internal/gameserver/combat_handler.go`

**Step 1: Create `internal/gameserver/combat_handler.go`**

This handler bridges gRPC commands to the combat engine, drives NPC turns, and returns `CombatEvent` protos.

```go
package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// CombatHandler handles attack, flee, and NPC turn execution.
type CombatHandler struct {
	engine   *combat.Engine
	npcMgr   *npc.Manager
	sessions *session.Manager
	dice     *dice.Roller
}

// NewCombatHandler creates a CombatHandler.
//
// Precondition: all arguments must be non-nil.
func NewCombatHandler(engine *combat.Engine, npcMgr *npc.Manager, sessions *session.Manager, diceRoller *dice.Roller) *CombatHandler {
	return &CombatHandler{engine: engine, npcMgr: npcMgr, sessions: sessions, dice: diceRoller}
}

// Attack processes a player's attack on a named NPC target.
// If no combat is active in the room, it starts one with initiative rolls.
// After the player's attack, any living NPC takes its turn immediately.
//
// Precondition: uid must be a valid connected player; target must be non-empty.
// Postcondition: Returns a slice of CombatEvents to broadcast, or an error.
func (h *CombatHandler) Attack(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Find the target NPC in the room.
	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}
	if inst.IsDead() {
		return nil, fmt.Errorf("%s is already dead", inst.Name)
	}

	// Build or fetch combat.
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		cbt = h.startCombat(sess, inst)
	}

	var events []*gamev1.CombatEvent

	// Verify it's the player's turn.
	current := cbt.CurrentTurn()
	if current == nil {
		h.engine.EndCombat(sess.RoomID)
		return nil, fmt.Errorf("combat is over")
	}
	if current.ID != uid {
		return nil, fmt.Errorf("it's not your turn")
	}

	// Find player and target combatants.
	playerCbt := h.findCombatant(cbt, uid)
	npcCbt := h.findCombatant(cbt, inst.ID)
	if playerCbt == nil || npcCbt == nil {
		return nil, fmt.Errorf("combatant not found in combat")
	}

	// Player attacks NPC.
	atkResult := combat.ResolveAttack(playerCbt, npcCbt, h.dice.Src())
	dmg := atkResult.EffectiveDamage()
	inst.CurrentHP -= dmg
	if inst.CurrentHP < 0 {
		inst.CurrentHP = 0
	}
	npcCbt.ApplyDamage(dmg)

	events = append(events, &gamev1.CombatEvent{
		Type:        gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:    sess.CharName,
		Target:      inst.Name,
		AttackRoll:  int32(atkResult.AttackRoll),
		AttackTotal: int32(atkResult.AttackTotal),
		Outcome:     atkResult.Outcome.String(),
		Damage:      int32(dmg),
		TargetHp:    int32(inst.CurrentHP),
		Narrative:   h.attackNarrative(sess.CharName, inst.Name, atkResult),
	})

	// Check NPC death.
	if npcCbt.IsDead() {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
			Target:    inst.Name,
			Narrative: fmt.Sprintf("%s falls to the ground.", inst.Name),
		})
		if !cbt.HasLivingNPCs() {
			h.engine.EndCombat(sess.RoomID)
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
				Narrative: "Combat is over. You stand victorious.",
			})
			return events, nil
		}
	}

	// Advance to NPC turn(s) and execute.
	cbt.AdvanceTurn()
	npcEvents := h.runNPCTurns(cbt, sess)
	events = append(events, npcEvents...)

	// Check if player died.
	if playerCbt.IsDead() {
		h.engine.EndCombat(sess.RoomID)
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
			Narrative: "Everything goes dark.",
		})
		return events, nil
	}

	return events, nil
}

// Flee attempts to remove the player from combat.
// Uses an opposed Athletics check: player d20+STR vs highest NPC d20+STR.
//
// Precondition: uid must be a valid connected player in active combat.
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

	// Opposed check: player d20+STR vs best NPC d20+STR.
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

// startCombat initialises a new combat between the player session and one NPC instance.
func (h *CombatHandler) startCombat(sess *session.PlayerSession, inst *npc.Instance) *combat.Combat {
	playerCbt := &combat.Combatant{
		ID:        sess.UID,
		Kind:      combat.KindPlayer,
		Name:      sess.CharName,
		MaxHP:     sess.CurrentHP,
		CurrentHP: sess.CurrentHP,
		AC:        12, // baseline; replaced by equipment in Stage 7
		Level:     1,
		StrMod:    2, // TODO Stage 7: pull from character abilities
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
		StrMod:    combat.AbilityMod(inst.Perception), // rough stand-in; Stage 8 uses real abilities
		DexMod:    1,
	}

	combatants := []*combat.Combatant{playerCbt, npcCbt}
	combat.RollInitiative(combatants, h.dice.Src())

	cbt, _ := h.engine.StartCombat(sess.RoomID, combatants)
	return cbt
}

// runNPCTurns executes all consecutive NPC turns until a player's turn comes up.
func (h *CombatHandler) runNPCTurns(cbt *combat.Combat, sess *session.PlayerSession) []*gamev1.CombatEvent {
	var events []*gamev1.CombatEvent
	for {
		current := cbt.CurrentTurn()
		if current == nil || current.Kind == combat.KindPlayer {
			break
		}

		// Find the player combatant as the NPC target.
		playerCbt := h.findCombatant(cbt, sess.UID)
		if playerCbt == nil {
			break
		}

		atkResult := combat.ResolveAttack(current, playerCbt, h.dice.Src())
		dmg := atkResult.EffectiveDamage()
		playerCbt.ApplyDamage(dmg)
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}

		events = append(events, &gamev1.CombatEvent{
			Type:        gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:    current.Name,
			Target:      sess.CharName,
			AttackRoll:  int32(atkResult.AttackRoll),
			AttackTotal: int32(atkResult.AttackTotal),
			Outcome:     atkResult.Outcome.String(),
			Damage:      int32(dmg),
			TargetHp:    int32(sess.CurrentHP),
			Narrative:   h.attackNarrative(current.Name, sess.CharName, atkResult),
		})

		if playerCbt.IsDead() {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
				Target:    sess.CharName,
				Narrative: fmt.Sprintf("%s is incapacitated!", sess.CharName),
			})
			break
		}

		cbt.AdvanceTurn()
	}
	return events
}

// findCombatant looks up a combatant by ID in the combat.
func (h *CombatHandler) findCombatant(cbt *combat.Combat, id string) *combat.Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// bestNPCCombatant returns the living NPC with the highest StrMod (for flee checks).
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

// removeCombatant marks a combatant as dead (HP=0) to remove them from the turn cycle.
func (h *CombatHandler) removeCombatant(cbt *combat.Combat, id string) {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			c.CurrentHP = 0
			return
		}
	}
}

// attackNarrative generates a brief text description of an attack.
func (h *CombatHandler) attackNarrative(attacker, target string, result combat.AttackResult) string {
	switch result.Outcome {
	case combat.CritSuccess:
		return fmt.Sprintf("%s lands a devastating blow on %s for %d damage!", attacker, target, result.EffectiveDamage())
	case combat.Success:
		return fmt.Sprintf("%s hits %s for %d damage.", attacker, target, result.EffectiveDamage())
	case combat.Failure:
		return fmt.Sprintf("%s swings at %s but misses.", attacker, target)
	default:
		return fmt.Sprintf("%s fumbles their attack against %s.", attacker, target)
	}
}
```

Note: `h.dice.Src()` requires a `Src()` method on `*dice.Roller`. This is added in Task 7.

**Step 2: Build check (expect one error about Src())**

```
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1 | head -10
```
Expected: error about `Src` undefined on `*dice.Roller` — fixed in Task 7.

**Step 3: Commit as-is (will be fixed in Task 7)**

```bash
git add internal/gameserver/combat_handler.go
git commit -m "feat(gameserver): CombatHandler — attack, flee, NPC turns (Task 6)"
```

---

### Task 7: Expose dice.Roller Source + wire CombatHandler

**Files:**
- Modify: `internal/game/dice/logged_roller.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`

**Step 1: Add `Src()` method to `dice.Roller`**

In `internal/game/dice/logged_roller.go`, add after the struct definition:

```go
// Src returns the underlying Source for use in packages that need direct access.
//
// Postcondition: Returns the non-nil Source this Roller was created with.
func (r *Roller) Src() Source {
	return r.src
}
```

**Step 2: Wire CombatHandler into GameServiceServer**

In `internal/gameserver/grpc_service.go`:

Add `combatH *CombatHandler` field to `GameServiceServer` struct (after `npcH`).

Update `NewGameServiceServer` to accept `combatHandler *CombatHandler` as final parameter and set `combatH: combatHandler`.

In `dispatch`, add before `default`:
```go
case *gamev1.ClientMessage_Attack:
    return s.handleAttack(uid, p.Attack)
case *gamev1.ClientMessage_Flee:
    return s.handleFlee(uid)
```

Add handler methods:
```go
func (s *GameServiceServer) handleAttack(uid string, req *gamev1.AttackRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Attack(uid, req.Target)
	if err != nil {
		return nil, err
	}
	// Broadcast all events to room except the player; return the first to the player.
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for i, evt := range events {
			serverEvt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: evt},
			}
			if i == 0 {
				continue // returned directly below
			}
			data, _ := proto.Marshal(serverEvt)
			_ = sess.Entity.Push(data)
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	if len(events) == 0 {
		return nil, nil
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) handleFlee(uid string) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Flee(uid)
	if err != nil {
		return nil, err
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok && len(events) > 0 {
		s.broadcastCombatEvent(sess.RoomID, uid, events[0])
	}
	if len(events) == 0 {
		return nil, nil
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) broadcastCombatEvent(roomID, excludeUID string, evt *gamev1.CombatEvent) {
	s.broadcastToRoom(roomID, excludeUID, &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: evt},
	})
}
```

**Step 3: Wire into `cmd/gameserver/main.go`**

After `npcHandler` is created, add:
```go
combatEngine := combat.NewEngine()
combatHandler := gameserver.NewCombatHandler(combatEngine, npcMgr, sessMgr, diceRoller)
```

Add import `"github.com/cory-johannsen/mud/internal/game/combat"`.

Update `NewGameServiceServer` call to pass `combatHandler` as final argument.

**Step 4: Build all**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: zero errors.

**Step 5: Run all non-Postgres tests**

```
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v storage/postgres) -count=1 2>&1 | grep -E "^ok|FAIL"
```
Expected: all `ok`.

**Step 6: Commit**

```bash
git add internal/game/dice/logged_roller.go internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat(gameserver): wire CombatHandler, expose dice.Roller.Src() (Task 7)"
```

---

### Task 8: Register attack + flee commands; render CombatEvent in frontend

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/game_bridge.go`

**Step 1: Add constants and commands**

In `internal/game/command/commands.go`, add to Handler constants:
```go
HandlerAttack = "attack"
HandlerFlee   = "flee"
```

In `BuiltinCommands()`, add to World commands section:
```go
{Name: "attack", Aliases: []string{"att", "kill"}, Help: "Attack a target", Category: CategoryWorld, Handler: HandlerAttack},
{Name: "flee", Aliases: []string{"run"}, Help: "Attempt to flee combat", Category: CategoryWorld, Handler: HandlerFlee},
```

**Step 2: Add `RenderCombatEvent` to `text_renderer.go`**

```go
// RenderCombatEvent formats a CombatEvent as colored Telnet text.
func RenderCombatEvent(ce *gamev1.CombatEvent) string {
	switch ce.Type {
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK:
		color := telnet.BrightWhite
		if ce.Damage > 0 {
			color = telnet.BrightRed
		}
		return telnet.Colorf(color, "[Combat] %s", ce.Narrative)
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH:
		return telnet.Colorf(telnet.Red, "[Combat] %s", ce.Narrative)
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE:
		return telnet.Colorf(telnet.Yellow, "[Combat] %s", ce.Narrative)
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_END:
		return telnet.Colorf(telnet.BrightYellow, "[Combat] %s", ce.Narrative)
	default:
		return telnet.Colorf(telnet.White, "[Combat] %s", ce.Narrative)
	}
}
```

**Step 3: Add `attack` and `flee` to `commandLoop` in `game_bridge.go`**

In the `switch cmd.Handler` block, add before `HandlerExamine`:
```go
case command.HandlerAttack:
    if parsed.RawArgs == "" {
        _ = conn.WriteLine(telnet.Colorize(telnet.Red, "Attack what?"))
        _ = conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName))
        continue
    }
    msg = &gamev1.ClientMessage{
        RequestId: reqID,
        Payload: &gamev1.ClientMessage_Attack{
            Attack: &gamev1.AttackRequest{Target: parsed.RawArgs},
        },
    }

case command.HandlerFlee:
    msg = &gamev1.ClientMessage{
        RequestId: reqID,
        Payload:   &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}},
    }
```

**Step 4: Handle `CombatEvent` in `forwardServerEvents`**

In the `switch p := resp.Payload.(type)` block, add:
```go
case *gamev1.ServerEvent_CombatEvent:
    text = RenderCombatEvent(p.CombatEvent)
```

**Step 5: Build and test**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
```
cd /home/cjohannsen/src/mud && go test ./internal/frontend/... ./internal/game/command/... -count=1 2>&1
```
Expected: zero errors, all PASS.

**Step 6: Commit**

```bash
git add internal/game/command/commands.go internal/frontend/handlers/text_renderer.go internal/frontend/handlers/game_bridge.go
git commit -m "feat(frontend): attack/flee commands, RenderCombatEvent renderer (Task 8)"
```

---

### Task 9: Final Verification + Push

**Step 1: Run all non-Postgres tests with race detector**

```
cd /home/cjohannsen/src/mud && go test -race $(go list ./... | grep -v storage/postgres) -count=1 2>&1 | grep -E "^ok|FAIL"
```
Expected: all `ok`, no races.

**Step 2: Run combat package tests verbosely**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v -count=1 2>&1
```
Expected: all PASS.

**Step 3: Build all binaries**

```
cd /home/cjohannsen/src/mud && go build ./cmd/... 2>&1
```
Expected: zero errors.

**Step 4: Push**

```bash
git push origin main
```
