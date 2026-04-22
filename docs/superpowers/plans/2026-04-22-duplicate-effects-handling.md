# Duplicate Effects Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a typed-bonus pipeline (`internal/game/effect/`) that unifies condition, feat/tech passive, and equipment bonus computation under PF2E stacking rules — highest-per-type wins for `status`/`circumstance`/`item`; `untyped` always stacks.

**Architecture:** New `internal/game/effect/` package provides `Bonus`, `Effect`, `EffectSet`, and `Resolve`. `condition.ActiveSet` gains an internal `EffectSet` maintained in lock-step; `modifiers.go` becomes thin wrappers. `ClassFeature` and `TechnologyDef` gain `PassiveBonuses []effect.Bonus`. `combat.Combatant` gains `Effects *effect.EffectSet` populated at creation from all sources; `round.go` migrates the 10 condition-bonus call sites to `effect.Resolve(actor.Effects, stat).Total`.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based tests), TypeScript/React (effects panel), existing proto `EffectsSummary` field for web transport.

---

## File Map

**New files:**
- `internal/game/effect/bonus.go` — `BonusType`, `Stat`, `Bonus`, `DurationKind` types; validation
- `internal/game/effect/bonus_test.go`
- `internal/game/effect/set.go` — `Effect`, `effectKey`, `EffectSet` with all methods
- `internal/game/effect/set_test.go` — property tests
- `internal/game/effect/resolve.go` — `Resolved`, `Contribution`, `ContributionRef`, `Resolve()`
- `internal/game/effect/resolve_test.go` — property tests
- `internal/game/effect/resolve_scenario_test.go` — PF2E scenario tests
- `internal/game/condition/modifiers_compat_test.go` — golden tests
- `internal/game/combat/combatant_effects.go` — `BuildCombatantEffects`, sync helpers
- `internal/game/combat/round_effect_test.go` — end-to-end effect pipeline test

**Modified files:**
- `internal/game/condition/definition.go` — add `Bonuses []Bonus`; synthesis of flat fields at load time
- `internal/game/condition/active.go` — add internal `*effect.EffectSet`; `Effects()` method; sync in Apply/Remove/Tick
- `internal/game/condition/modifiers.go` — rewrite as `effect.Resolve` wrappers
- `internal/game/ruleset/class_feature.go` — add `PassiveBonuses []effect.Bonus`
- `internal/game/technology/model.go` — add `PassiveBonuses []effect.Bonus`
- `internal/game/combat/combat.go` — `Combatant.Effects *effect.EffectSet`
- `internal/game/combat/round.go` — migrate 10 condition-bonus call sites
- `internal/gameserver/combat_handler.go` — populate `Effects` at combatant creation
- `internal/frontend/handlers/mode_combat.go` — `effects` / `effects detail` commands
- `internal/frontend/handlers/text_renderer.go` — `renderEffectsBlock()`
- `cmd/webclient/ui/src/game/drawers/DrawerContainer.tsx` — add `'effects'` drawer type
- `docs/architecture/combat.md` — "Bonus stacking" section
- `docs/architecture/CHARACTERS.md` — EffectSet on Combatant; provider pattern

---

## Task 1: `effect` package — core types and validation

**Files:**
- Create: `internal/game/effect/bonus.go`
- Create: `internal/game/effect/bonus_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/game/effect/bonus_test.go
package effect_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestBonusType_ValidValues(t *testing.T) {
    for _, bt := range []effect.BonusType{
        effect.BonusTypeStatus, effect.BonusTypeCircumstance,
        effect.BonusTypeItem, effect.BonusTypeUntyped,
    } {
        assert.NotEmpty(t, string(bt))
    }
}

func TestBonus_Validate_ZeroValueRejected(t *testing.T) {
    b := effect.Bonus{Stat: effect.StatAttack, Value: 0, Type: effect.BonusTypeUntyped}
    err := b.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "value 0")
}

func TestBonus_Validate_ValidBonus(t *testing.T) {
    b := effect.Bonus{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}
    assert.NoError(t, b.Validate())
}

func TestBonus_Validate_ValidPenalty(t *testing.T) {
    b := effect.Bonus{Stat: effect.StatAC, Value: -2, Type: effect.BonusTypeCircumstance}
    assert.NoError(t, b.Validate())
}

func TestBonus_DefaultType_Untyped(t *testing.T) {
    b := effect.Bonus{Stat: effect.StatDamage, Value: 1}
    b.Normalise()
    assert.Equal(t, effect.BonusTypeUntyped, b.Type)
}

func TestProperty_Bonus_NonZeroAlwaysValid(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        v := rapid.Int().Filter(func(x int) bool { return x != 0 }).Draw(rt, "value").(int)
        b := effect.Bonus{Stat: effect.StatAttack, Value: v, Type: effect.BonusTypeStatus}
        assert.NoError(rt, b.Validate())
    })
}

func TestStatMatches_ExactMatch(t *testing.T) {
    assert.True(t, effect.StatMatches(effect.StatAttack, effect.StatAttack))
    assert.True(t, effect.StatMatches(effect.Stat("skill:stealth"), effect.Stat("skill:stealth")))
}

func TestStatMatches_PrefixInheritance(t *testing.T) {
    // querying "skill:stealth" — a bonus to "skill" contributes
    assert.True(t, effect.StatMatches(effect.Stat("skill"), effect.Stat("skill:stealth")))
}

func TestStatMatches_NoCrossSkillInheritance(t *testing.T) {
    // "skill:savvy" does NOT contribute to "skill:stealth"
    assert.False(t, effect.StatMatches(effect.Stat("skill:savvy"), effect.Stat("skill:stealth")))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/effect/... 2>&1 | head -20
```
Expected: `cannot find package` error.

- [ ] **Step 3: Write the implementation**

```go
// internal/game/effect/bonus.go
package effect

import (
    "fmt"
    "strings"
)

// BonusType classifies how bonuses of the same type stack.
type BonusType string

const (
    BonusTypeStatus       BonusType = "status"
    BonusTypeCircumstance BonusType = "circumstance"
    BonusTypeItem         BonusType = "item"
    BonusTypeUntyped      BonusType = "untyped"
)

// Stat identifies what a bonus applies to.
type Stat string

const (
    StatAttack    Stat = "attack"
    StatAC        Stat = "ac"
    StatDamage    Stat = "damage"
    StatSpeed     Stat = "speed"
    StatBrutality Stat = "brutality"
    StatGrit      Stat = "grit"
    StatQuickness Stat = "quickness"
    StatReasoning Stat = "reasoning"
    StatSavvy     Stat = "savvy"
    StatFlair     Stat = "flair"
    StatSkill     Stat = "skill"
)

// DurationKind indicates how an effect's duration is tracked.
type DurationKind string

const (
    DurationRounds      DurationKind = "rounds"
    DurationUntilRemove DurationKind = "until_remove"
    DurationPermanent   DurationKind = "permanent"
    DurationEncounter   DurationKind = "encounter"
    DurationCalendar    DurationKind = "calendar"
)

// Bonus is a single typed numeric contribution to one stat.
// Precondition: Value must not be zero (enforced by Validate).
type Bonus struct {
    Stat  Stat      `yaml:"stat"`
    Value int       `yaml:"value"` // positive = bonus, negative = penalty; 0 is invalid
    Type  BonusType `yaml:"type"`  // defaults to BonusTypeUntyped via Normalise
}

// Validate returns an error if the Bonus is malformed.
// Postcondition: returns non-nil error iff Value == 0.
func (b Bonus) Validate() error {
    if b.Value == 0 {
        return fmt.Errorf("effect.Bonus: value 0 is not permitted (stat %q, type %q)", b.Stat, b.Type)
    }
    return nil
}

// Normalise sets Type to BonusTypeUntyped if it is empty.
// Call this after YAML unmarshal to apply the default-type rule (DEDUP-2).
func (b *Bonus) Normalise() {
    if b.Type == "" {
        b.Type = BonusTypeUntyped
    }
}

// StatMatches reports whether a bonus on bonusStat contributes to a query for queryStat.
// Per DEDUP-16: a bonus to "skill" contributes to any "skill:<id>" query;
// a bonus to "skill:stealth" does NOT contribute to "skill:savvy".
func StatMatches(bonusStat, queryStat Stat) bool {
    if bonusStat == queryStat {
        return true
    }
    // prefix-on-colon: bonusStat "skill" matches queryStat "skill:stealth"
    prefix := string(bonusStat) + ":"
    return strings.HasPrefix(string(queryStat), prefix)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/effect/... -v 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/effect/bonus.go internal/game/effect/bonus_test.go && git commit -m "feat(effect): core Bonus, Stat, BonusType, DurationKind types with StatMatches (#245)"
```

---

## Task 2: `effect` package — Effect and EffectSet

**Files:**
- Create: `internal/game/effect/set.go`
- Create: `internal/game/effect/set_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/game/effect/set_test.go
package effect_test

import (
    "testing"
    "time"

    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestEffectSet_NilSafeVersion(t *testing.T) {
    var s *effect.EffectSet
    assert.Equal(t, uint64(0), s.Version())
}

func TestEffectSet_NilSafeAll(t *testing.T) {
    var s *effect.EffectSet
    assert.Nil(t, s.All())
}

func TestEffectSet_ApplyAndRetrieve(t *testing.T) {
    s := effect.NewEffectSet()
    e := effect.Effect{
        EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 2,
    }
    s.Apply(e)
    all := s.All()
    require.Len(t, all, 1)
    assert.Equal(t, "e1", all[0].EffectID)
}

func TestEffectSet_Apply_SameKeyOverwrites(t *testing.T) {
    s := effect.NewEffectSet()
    e1 := effect.Effect{EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 2}
    e2 := effect.Effect{EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 3}
    s.Apply(e1)
    s.Apply(e2)
    all := s.All()
    require.Len(t, all, 1)
    assert.Equal(t, -2, all[0].Bonuses[0].Value)
}

func TestEffectSet_RemoveBySource(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "feat:toughness", CasterUID: "uid1",
        Bonuses: []effect.Bonus{{Stat: effect.StatGrit, Value: 1, Type: effect.BonusTypeUntyped}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:prone", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 2})
    s.RemoveBySource("feat:toughness")
    all := s.All()
    require.Len(t, all, 1)
    assert.Equal(t, "e2", all[0].EffectID)
}

func TestEffectSet_RemoveByCaster_OnlyLinked(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:blessed", CasterUID: "ally1",
        LinkedToCaster: true,
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:frightened", CasterUID: "enemy1",
        LinkedToCaster: false,
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.RemoveByCaster("ally1")
    all := s.All()
    require.Len(t, all, 1)
    assert.Equal(t, "e2", all[0].EffectID)
}

func TestEffectSet_Tick_DecrementsAndExpires(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:dazzled", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 1})
    expired := s.Tick()
    require.Len(t, expired, 0) // DurRemain 1→0; expire next Tick per round semantics
    expired2 := s.Tick()
    assert.Len(t, expired2, 1)
    assert.Len(t, s.All(), 0)
}

func TestEffectSet_ClearEncounter(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:inspired", CasterUID: "",
        DurKind: effect.DurationEncounter})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "feat:resolve", CasterUID: "",
        DurKind: effect.DurationPermanent})
    s.ClearEncounter()
    all := s.All()
    require.Len(t, all, 1)
    assert.Equal(t, "e2", all[0].EffectID)
}

func TestEffectSet_Version_MonotonicallyIncreases(t *testing.T) {
    s := effect.NewEffectSet()
    v0 := s.Version()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "s", Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: 1, Type: effect.BonusTypeItem}}, DurKind: effect.DurationPermanent})
    v1 := s.Version()
    s.RemoveBySource("s")
    v2 := s.Version()
    assert.Less(t, v0, v1)
    assert.Less(t, v1, v2)
}

func TestProperty_EffectSet_Version_AlwaysIncreases(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        s := effect.NewEffectSet()
        n := rapid.IntRange(1, 10).Draw(rt, "n").(int)
        prev := s.Version()
        for i := 0; i < n; i++ {
            s.Apply(effect.Effect{
                EffectID: fmt.Sprintf("e%d", i),
                SourceID: fmt.Sprintf("src%d", i),
                Bonuses:  []effect.Bonus{{Stat: effect.StatAttack, Value: i + 1, Type: effect.BonusTypeUntyped}},
                DurKind:  effect.DurationPermanent,
            })
            v := s.Version()
            assert.Greater(rt, v, prev)
            prev = v
        }
    })
}
```

Note: add `"fmt"` to the import in the test file.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/effect/... 2>&1 | grep -E "FAIL|undefined"
```

- [ ] **Step 3: Write the implementation**

```go
// internal/game/effect/set.go
package effect

import (
    "sort"
    "time"
)

// effectKey uniquely identifies a single effect on a bearer.
// Per DEDUP-1: (bearer_uid, source_id, caster_uid).
// The bearer_uid is implicit (the EffectSet is per-bearer).
type effectKey struct {
    sourceID  string
    casterUID string
}

// Effect is a named bundle of bonuses from one source.
type Effect struct {
    EffectID       string
    SourceID       string
    CasterUID      string
    Bonuses        []Bonus
    DurKind        DurationKind
    DurRemain      int        // meaningful only for DurationRounds
    ExpiresAt      *time.Time // meaningful only for DurationCalendar
    Annotation     string
    LinkedToCaster bool // if true: removed when caster exits (DEDUP-12)
}

// EffectSet is a per-bearer collection of effects with dedup and version counter.
// A nil *EffectSet is safe: all methods are no-ops / zero-value returns.
type EffectSet struct {
    effects map[effectKey]Effect
    version uint64
}

// NewEffectSet returns an empty, initialized EffectSet.
func NewEffectSet() *EffectSet {
    return &EffectSet{effects: make(map[effectKey]Effect)}
}

func (s *EffectSet) key(e Effect) effectKey {
    return effectKey{sourceID: e.SourceID, casterUID: e.CasterUID}
}

// Apply inserts or overwrites the effect identified by (SourceID, CasterUID). (DEDUP-1)
func (s *EffectSet) Apply(e Effect) {
    if s == nil {
        return
    }
    s.effects[s.key(e)] = e
    s.version++
}

// Remove deletes the effect with the given (sourceID, casterUID) key.
func (s *EffectSet) Remove(sourceID, casterUID string) {
    if s == nil {
        return
    }
    k := effectKey{sourceID: sourceID, casterUID: casterUID}
    if _, ok := s.effects[k]; ok {
        delete(s.effects, k)
        s.version++
    }
}

// RemoveBySource removes all effects whose SourceID matches. (DEDUP-1)
func (s *EffectSet) RemoveBySource(sourceID string) {
    if s == nil {
        return
    }
    changed := false
    for k := range s.effects {
        if k.sourceID == sourceID {
            delete(s.effects, k)
            changed = true
        }
    }
    if changed {
        s.version++
    }
}

// RemoveByCaster removes all LinkedToCaster effects whose CasterUID matches. (DEDUP-12)
func (s *EffectSet) RemoveByCaster(casterUID string) {
    if s == nil {
        return
    }
    changed := false
    for k, e := range s.effects {
        if e.LinkedToCaster && k.casterUID == casterUID {
            delete(s.effects, k)
            changed = true
        }
    }
    if changed {
        s.version++
    }
}

// Tick decrements DurationRounds effects and removes those whose DurRemain reaches 0.
// Returns the EffectIDs of removed effects.
func (s *EffectSet) Tick() []string {
    if s == nil {
        return nil
    }
    var expired []string
    for k, e := range s.effects {
        if e.DurKind != DurationRounds {
            continue
        }
        e.DurRemain--
        if e.DurRemain <= 0 {
            expired = append(expired, e.EffectID)
            delete(s.effects, k)
        } else {
            s.effects[k] = e
        }
    }
    if len(expired) > 0 {
        s.version++
    }
    return expired
}

// TickCalendar removes effects whose ExpiresAt is before or equal to now.
func (s *EffectSet) TickCalendar(now time.Time) []string {
    if s == nil {
        return nil
    }
    var expired []string
    for k, e := range s.effects {
        if e.DurKind != DurationCalendar || e.ExpiresAt == nil {
            continue
        }
        if !now.Before(*e.ExpiresAt) {
            expired = append(expired, e.EffectID)
            delete(s.effects, k)
        }
    }
    if len(expired) > 0 {
        s.version++
    }
    return expired
}

// ClearEncounter removes all DurationEncounter effects.
func (s *EffectSet) ClearEncounter() {
    if s == nil {
        return
    }
    changed := false
    for k, e := range s.effects {
        if e.DurKind == DurationEncounter {
            delete(s.effects, k)
            changed = true
        }
    }
    if changed {
        s.version++
    }
}

// All returns a stable-sorted snapshot of all active effects.
func (s *EffectSet) All() []Effect {
    if s == nil {
        return nil
    }
    out := make([]Effect, 0, len(s.effects))
    for _, e := range s.effects {
        out = append(out, e)
    }
    sort.Slice(out, func(i, j int) bool {
        if out[i].SourceID != out[j].SourceID {
            return out[i].SourceID < out[j].SourceID
        }
        return out[i].CasterUID < out[j].CasterUID
    })
    return out
}

// Version returns the monotonic mutation counter. (DEDUP-8)
func (s *EffectSet) Version() uint64 {
    if s == nil {
        return 0
    }
    return s.version
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/effect/... -v -run "TestEffectSet" 2>&1 | tail -30
```

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/effect/set.go internal/game/effect/set_test.go && git commit -m "feat(effect): Effect, EffectSet — apply/remove/tick/version with dedup key (#245)"
```

---

## Task 3: `effect` package — Resolve function

**Files:**
- Create: `internal/game/effect/resolve.go`
- Create: `internal/game/effect/resolve_test.go`
- Create: `internal/game/effect/resolve_scenario_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/game/effect/resolve_test.go
package effect_test

import (
    "testing"

    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestResolve_EmptySet_ReturnsZero(t *testing.T) {
    s := effect.NewEffectSet()
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 0, r.Total)
    assert.Empty(t, r.Contributing)
    assert.Empty(t, r.Suppressed)
}

func TestResolve_NilSet_ReturnsZero(t *testing.T) {
    r := effect.Resolve(nil, effect.StatAttack)
    assert.Equal(t, 0, r.Total)
}

func TestResolve_SingleBonus_Contributes(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:blessed", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 2, r.Total)
    require.Len(t, r.Contributing, 1)
    assert.Nil(t, r.Contributing[0].OverriddenBy)
    assert.Empty(t, r.Suppressed)
}

func TestResolve_SameType_HighestWins(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:heroism", CasterUID: "caster1",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 3, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:inspire_courage", CasterUID: "caster2",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 3, r.Total)
    require.Len(t, r.Contributing, 1)
    require.Len(t, r.Suppressed, 1)
    assert.NotNil(t, r.Suppressed[0].OverriddenBy)
    assert.Equal(t, "e1", r.Suppressed[0].OverriddenBy.EffectID)
}

func TestResolve_UntypedAlwaysStacks(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "feat:inspire", CasterUID: "uid",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeUntyped}},
        DurKind: effect.DurationPermanent})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "feat:focus", CasterUID: "uid",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeUntyped}},
        DurKind: effect.DurationPermanent})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 3, r.Total)
    assert.Len(t, r.Contributing, 2)
    assert.Empty(t, r.Suppressed)
}

func TestResolve_MixedTypes_IndependentBuckets(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:heroism", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "item:plus1sword", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeItem}},
        DurKind: effect.DurationUntilRemove})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 3, r.Total)
    assert.Len(t, r.Contributing, 2)
}

func TestResolve_Penalty_WorstPenaltyWins(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:frightened2", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 2})
    s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:sickened1", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 1})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, -2, r.Total) // only the worst penalty contributes
    assert.Len(t, r.Contributing, 1)
    assert.Len(t, r.Suppressed, 1)
}

func TestResolve_TieBreak_LexicographicOrder(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e_b", SourceID: "condition:bless", CasterUID: "caster_b",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "e_a", SourceID: "condition:aid", CasterUID: "caster_a",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    r := effect.Resolve(s, effect.StatAttack)
    require.Len(t, r.Contributing, 1)
    // lex winner: "condition:aid"/"caster_a" < "condition:bless"/"caster_b"
    assert.Equal(t, "e_a", r.Contributing[0].EffectID)
}

func TestResolve_Pure_IdenticalInputSameOutput(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeCircumstance}},
        DurKind: effect.DurationRounds, DurRemain: 2})
    r1 := effect.Resolve(s, effect.StatAC)
    r2 := effect.Resolve(s, effect.StatAC)
    assert.Equal(t, r1.Total, r2.Total)
    assert.Equal(t, len(r1.Contributing), len(r2.Contributing))
}

func TestProperty_Resolve_UntypedSumsAll(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        s := effect.NewEffectSet()
        n := rapid.IntRange(1, 5).Draw(rt, "n").(int)
        total := 0
        for i := 0; i < n; i++ {
            v := rapid.Int().Filter(func(x int) bool { return x != 0 }).Draw(rt, "v").(int)
            s.Apply(effect.Effect{
                EffectID: fmt.Sprintf("e%d", i),
                SourceID: fmt.Sprintf("src%d", i),
                Bonuses:  []effect.Bonus{{Stat: effect.StatDamage, Value: v, Type: effect.BonusTypeUntyped}},
                DurKind:  effect.DurationPermanent,
            })
            total += v
        }
        r := effect.Resolve(s, effect.StatDamage)
        assert.Equal(rt, total, r.Total)
    })
}

func TestResolve_PrefixOnColon_SkillContributesToSkillStealth(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:skulker", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatSkill, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationPermanent})
    r := effect.Resolve(s, effect.Stat("skill:stealth"))
    assert.Equal(t, -1, r.Total)
}
```

```go
// internal/game/effect/resolve_scenario_test.go
package effect_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

// Scenario: two same-type status bonuses — only the highest contributes.
func TestScenario_TwoStatusBonuses(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "kira",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
            {Stat: effect.StatGrit, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "inspire", SourceID: "condition:inspire_courage", CasterUID: "xin",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 1, r.Total)     // only one status bonus contributes
    assert.Len(t, r.Suppressed, 1)  // the other is suppressed
}

// Scenario: stacking circumstance penalties — only the worst contributes.
func TestScenario_StackingCircumstancePenalties(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "wet_floor", SourceID: "env:wet_floor", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeCircumstance}},
        DurKind: effect.DurationEncounter})
    s.Apply(effect.Effect{EffectID: "blinded", SourceID: "condition:blinded", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -3, Type: effect.BonusTypeCircumstance}},
        DurKind: effect.DurationRounds, DurRemain: 2})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, -3, r.Total)    // only the worst penalty counts
    assert.Len(t, r.Suppressed, 1)
}

// Scenario: untyped bonuses from different sources all stack.
func TestScenario_UntypedAdditivity(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "haste_dmg", SourceID: "condition:haste", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatDamage, Value: 2, Type: effect.BonusTypeUntyped}},
        DurKind: effect.DurationRounds, DurRemain: 3})
    s.Apply(effect.Effect{EffectID: "rage_dmg", SourceID: "feat:rage", CasterUID: "self",
        Bonuses: []effect.Bonus{{Stat: effect.StatDamage, Value: 3, Type: effect.BonusTypeUntyped}},
        DurKind: effect.DurationEncounter})
    r := effect.Resolve(s, effect.StatDamage)
    assert.Equal(t, 5, r.Total)
    assert.Len(t, r.Contributing, 2)
    assert.Empty(t, r.Suppressed)
}

// Scenario: bonus and penalty from different typed sources — both contribute.
func TestScenario_BonusAndPenaltySameType(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "bless", SourceID: "condition:bless", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    s.Apply(effect.Effect{EffectID: "frightened", SourceID: "condition:frightened1", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationRounds, DurRemain: 2})
    r := effect.Resolve(s, effect.StatAttack)
    assert.Equal(t, 0, r.Total) // +1 status bonus + -1 status penalty
    assert.Len(t, r.Contributing, 2)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/effect/... 2>&1 | grep -E "FAIL|undefined"
```

- [ ] **Step 3: Write the Resolve implementation**

```go
// internal/game/effect/resolve.go
package effect

import "sort"

// Resolved is the output of Resolve for one stat.
type Resolved struct {
    Stat         Stat
    Total        int
    Contributing []Contribution
    Suppressed   []Contribution
}

// Contribution is one bonus's contribution record.
type Contribution struct {
    EffectID    string
    SourceID    string
    CasterUID   string
    BonusType   BonusType
    Value       int
    OverriddenBy *ContributionRef // non-nil only in Suppressed list
}

// ContributionRef identifies the winning effect that caused suppression.
type ContributionRef struct {
    EffectID  string
    SourceID  string
    CasterUID string
}

// candidate is an internal working type for Resolve.
type candidate struct {
    effectID  string
    sourceID  string
    casterUID string
    bonusType BonusType
    value     int
}

// Resolve computes the net bonus for stat across all effects in set. (DEDUP-7: pure)
// A nil set returns a zero Resolved.
func Resolve(set *EffectSet, stat Stat) Resolved {
    if set == nil {
        return Resolved{Stat: stat}
    }

    // Collect all matching bonus contributions.
    var matches []candidate
    for _, e := range set.All() {
        for _, b := range e.Bonuses {
            if StatMatches(b.Stat, stat) {
                matches = append(matches, candidate{
                    effectID:  e.EffectID,
                    sourceID:  e.SourceID,
                    casterUID: e.CasterUID,
                    bonusType: b.Type,
                    value:     b.Value,
                })
            }
        }
    }

    if len(matches) == 0 {
        return Resolved{Stat: stat}
    }

    var contributing []Contribution
    var suppressed []Contribution
    total := 0

    // Process typed buckets (status, circumstance, item) — highest bonus wins, worst penalty wins.
    for _, bt := range []BonusType{BonusTypeStatus, BonusTypeCircumstance, BonusTypeItem} {
        var pos, neg []candidate
        for _, m := range matches {
            if m.bonusType != bt {
                continue
            }
            if m.value > 0 {
                pos = append(pos, m)
            } else {
                neg = append(neg, m)
            }
        }

        if winner, losers := pickHighest(pos); winner != nil {
            total += winner.value
            contributing = append(contributing, toContribution(*winner, nil))
            ref := &ContributionRef{EffectID: winner.effectID, SourceID: winner.sourceID, CasterUID: winner.casterUID}
            for _, l := range losers {
                suppressed = append(suppressed, toContribution(l, ref))
            }
        }

        if winner, losers := pickLowest(neg); winner != nil {
            total += winner.value
            contributing = append(contributing, toContribution(*winner, nil))
            ref := &ContributionRef{EffectID: winner.effectID, SourceID: winner.sourceID, CasterUID: winner.casterUID}
            for _, l := range losers {
                suppressed = append(suppressed, toContribution(l, ref))
            }
        }
    }

    // Untyped: all stack.
    for _, m := range matches {
        if m.bonusType == BonusTypeUntyped {
            total += m.value
            contributing = append(contributing, toContribution(m, nil))
        }
    }

    return Resolved{Stat: stat, Total: total, Contributing: contributing, Suppressed: suppressed}
}

// pickHighest returns the candidate with the maximum value (lex tiebreak), plus the losers.
// Precondition: all candidates have value > 0.
func pickHighest(cs []candidate) (*candidate, []candidate) {
    if len(cs) == 0 {
        return nil, nil
    }
    sort.Slice(cs, func(i, j int) bool {
        if cs[i].value != cs[j].value {
            return cs[i].value > cs[j].value
        }
        // tie: ascending lex (SourceID, CasterUID) — first is winner (DEDUP-6)
        if cs[i].sourceID != cs[j].sourceID {
            return cs[i].sourceID < cs[j].sourceID
        }
        return cs[i].casterUID < cs[j].casterUID
    })
    return &cs[0], cs[1:]
}

// pickLowest returns the candidate with the minimum value (lex tiebreak), plus the losers.
// Precondition: all candidates have value < 0.
func pickLowest(cs []candidate) (*candidate, []candidate) {
    if len(cs) == 0 {
        return nil, nil
    }
    sort.Slice(cs, func(i, j int) bool {
        if cs[i].value != cs[j].value {
            return cs[i].value < cs[j].value
        }
        if cs[i].sourceID != cs[j].sourceID {
            return cs[i].sourceID < cs[j].sourceID
        }
        return cs[i].casterUID < cs[j].casterUID
    })
    return &cs[0], cs[1:]
}

func toContribution(c candidate, overriddenBy *ContributionRef) Contribution {
    return Contribution{
        EffectID:    c.effectID,
        SourceID:    c.sourceID,
        CasterUID:   c.casterUID,
        BonusType:   c.bonusType,
        Value:       c.value,
        OverriddenBy: overriddenBy,
    }
}
```

- [ ] **Step 4: Run all effect tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/effect/... -v 2>&1 | tail -40
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/effect/resolve.go internal/game/effect/resolve_test.go internal/game/effect/resolve_scenario_test.go && git commit -m "feat(effect): Resolve — typed-bonus pipeline with PF2E stacking rules (#245)"
```

---

## Task 4: Condition integration — ConditionDef gains `Bonuses`, ActiveSet gains internal EffectSet

**Files:**
- Modify: `internal/game/condition/definition.go`
- Modify: `internal/game/condition/active.go`
- Modify: `internal/game/condition/modifiers.go`
- Create: `internal/game/condition/modifiers_compat_test.go`

- [ ] **Step 1: Write the compat golden tests first**

These tests assert that the rewritten modifiers.go returns identical values to the current behavior for legacy flat-field conditions.

```go
// internal/game/condition/modifiers_compat_test.go
package condition_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/condition"
)

// buildDef creates a ConditionDef with only flat fields (legacy path).
func buildDef(id string, opts ...func(*condition.ConditionDef)) *condition.ConditionDef {
    d := &condition.ConditionDef{ID: id, Name: id, MaxStacks: 0, DurationType: "permanent"}
    for _, o := range opts {
        o(d)
    }
    return d
}

func TestModifiersCompat_AttackBonus_Positive(t *testing.T) {
    s := condition.NewActiveSet()
    def := buildDef("inspired", func(d *condition.ConditionDef) { d.AttackBonus = 2 })
    _ = s.Apply("uid", def, 1, -1)
    assert.Equal(t, 2, condition.AttackBonus(s))
}

func TestModifiersCompat_AttackBonus_Negative(t *testing.T) {
    s := condition.NewActiveSet()
    def := buildDef("frightened", func(d *condition.ConditionDef) { d.AttackPenalty = 1 })
    _ = s.Apply("uid", def, 1, -1)
    assert.Equal(t, -1, condition.AttackBonus(s))
}

func TestModifiersCompat_ACBonus_Mixed(t *testing.T) {
    s := condition.NewActiveSet()
    def := buildDef("prone", func(d *condition.ConditionDef) { d.ACPenalty = 2 })
    _ = s.Apply("uid", def, 1, -1)
    assert.Equal(t, -2, condition.ACBonus(s))
}

func TestModifiersCompat_DamageBonus(t *testing.T) {
    s := condition.NewActiveSet()
    def := buildDef("rage", func(d *condition.ConditionDef) { d.DamageBonus = 3 })
    _ = s.Apply("uid", def, 1, -1)
    assert.Equal(t, 3, condition.DamageBonus(s))
}

func TestModifiersCompat_SkillPenalty_FlatAndPerSkill(t *testing.T) {
    s := condition.NewActiveSet()
    def := buildDef("sickened", func(d *condition.ConditionDef) {
        d.SkillPenalty = 1
        d.SkillPenalties = map[string]int{"flair": 2}
    })
    _ = s.Apply("uid", def, 1, -1)
    assert.Equal(t, 1, condition.SkillPenalty(s))
}
```

- [ ] **Step 2: Run compat tests to record baseline (must pass before any changes)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v -run "TestModifiersCompat" 2>&1 | tail -20
```
Expected: all PASS (tests passing against current code confirms the baseline).

- [ ] **Step 3: Add `Bonuses []effect.Bonus` to `ConditionDef` and synthesis logic**

In `internal/game/condition/definition.go`, add after the existing fields and modify the YAML loader:

```go
// Add this import at the top of definition.go:
import "github.com/cory-johannsen/mud/internal/game/effect"

// Add to ConditionDef struct (after existing fields, before any methods):
// Bonuses is the authoritative typed-bonus list. When non-empty, flat bonus fields
// (AttackBonus, AttackPenalty, ACBonus, ACPenalty, etc.) MUST NOT also be set (DEDUP-11).
// When absent, flat fields are synthesised as untyped bonuses at load time.
Bonuses []effect.Bonus `yaml:"bonuses,omitempty"`
```

Add a `SynthesiseBonuses()` method on `ConditionDef` that converts flat fields to `[]effect.Bonus`:

```go
// SynthesiseBonuses populates Bonuses from flat fields if Bonuses is empty.
// Precondition: called after YAML unmarshal.
// Postcondition: Bonuses is non-nil; each synthesised entry has Type == BonusTypeUntyped.
// Returns an error if both Bonuses and flat fields are set (DEDUP-11).
func (d *ConditionDef) SynthesiseBonuses() error {
    hasBonusesField := len(d.Bonuses) > 0
    hasFlatFields := d.AttackBonus != 0 || d.AttackPenalty != 0 ||
        d.ACBonus != 0 || d.ACPenalty != 0 ||
        d.DamageBonus != 0 || d.StealthBonus != 0 ||
        d.FlairBonus != 0 || d.SkillPenalty != 0 ||
        len(d.SkillPenalties) > 0
    if hasBonusesField && hasFlatFields {
        return fmt.Errorf("condition %q: cannot mix 'bonuses:' and flat bonus fields", d.ID)
    }
    if hasBonusesField {
        for i := range d.Bonuses {
            d.Bonuses[i].Normalise()
        }
        return nil
    }
    // Synthesise untyped bonuses from flat fields.
    var out []effect.Bonus
    add := func(stat effect.Stat, value int) {
        if value != 0 {
            out = append(out, effect.Bonus{Stat: stat, Value: value, Type: effect.BonusTypeUntyped})
        }
    }
    add(effect.StatAttack, d.AttackBonus)
    add(effect.StatAttack, -d.AttackPenalty)
    add(effect.StatAC, d.ACBonus)
    add(effect.StatAC, -d.ACPenalty)
    add(effect.StatDamage, d.DamageBonus)
    add(effect.Stat("skill:stealth"), d.StealthBonus)
    add(effect.StatFlair, d.FlairBonus)
    add(effect.StatSkill, -d.SkillPenalty)
    for skillID, penalty := range d.SkillPenalties {
        add(effect.Stat("skill:"+skillID), -penalty)
    }
    d.Bonuses = out
    return nil
}
```

Call `SynthesiseBonuses()` in `LoadDirectory` after unmarshalling each def (in `definition.go`'s `LoadDirectory` function, after `Register(def)`).

- [ ] **Step 4: Add internal EffectSet to `ActiveSet` and sync in Apply/Remove/Tick**

In `internal/game/condition/active.go`, add the internal `*effect.EffectSet` to the `ActiveSet` struct and sync it in `Apply`, `ApplyTagged`, `ApplyTaggedWithExpiry`, `Remove`, `RemoveBySource`, `Tick`, and `TickCalendar`:

```go
// Add to ActiveSet struct:
effects *effect.EffectSet

// Add to NewActiveSet():
effects: effect.NewEffectSet(),

// Add Effects() method:
// Effects returns the internal EffectSet derived from all active conditions.
// Callers MUST NOT mutate the returned EffectSet directly.
func (s *ActiveSet) Effects() *effect.EffectSet {
    if s == nil {
        return nil
    }
    return s.effects
}

// In Apply (and ApplyTagged/ApplyTaggedWithExpiry), after setting the condition:
// Sync condition bonuses into the internal EffectSet.
s.syncConditionEffect("uid_placeholder", def, stacks)
// (Note: uid is passed in — use the actual uid parameter from the Apply signature)

// Add syncConditionEffect helper:
func (s *ActiveSet) syncConditionEffect(uid string, def *ConditionDef, stacks int) {
    if s.effects == nil {
        s.effects = effect.NewEffectSet()
    }
    // Scale bonuses by stack count.
    bonuses := make([]effect.Bonus, len(def.Bonuses))
    for i, b := range def.Bonuses {
        b.Value = b.Value * stacks
        bonuses[i] = b
    }
    s.effects.Apply(effect.Effect{
        EffectID:  def.ID,
        SourceID:  "condition:" + def.ID,
        CasterUID: uid,
        Bonuses:   bonuses,
        DurKind:   effect.DurationUntilRemove,
    })
}

// In Remove: after removing condition from the set:
s.effects.RemoveBySource("condition:" + id)

// In RemoveBySource: iterate conditions removed and call RemoveBySource on effects for each.
// In Tick: for each expired condition ID, call s.effects.RemoveBySource("condition:" + expiredID).
// In TickCalendar: same pattern.
// In ClearEncounter: s.effects.ClearEncounter()
// In ClearAll: s.effects = effect.NewEffectSet()
```

Note: The exact UID for the condition caster is `uid` passed into Apply — this is the bearer UID. In `activeSet` the `uid` parameter in `Apply(uid string, ...)` is the bearer's UID, used as CasterUID in condition effects (representing self-applied conditions).

- [ ] **Step 5: Rewrite `modifiers.go` as thin wrappers over `effect.Resolve`**

```go
// internal/game/condition/modifiers.go
package condition

import "github.com/cory-johannsen/mud/internal/game/effect"

// AttackBonus returns the net attack roll modifier from all active conditions.
// Postcondition: value may be negative (penalty) or positive (bonus).
func AttackBonus(s *ActiveSet) int {
    if s == nil {
        return 0
    }
    return effect.Resolve(s.Effects(), effect.StatAttack).Total
}

// ACBonus returns the net AC modifier from all active conditions.
func ACBonus(s *ActiveSet) int {
    if s == nil {
        return 0
    }
    return effect.Resolve(s.Effects(), effect.StatAC).Total
}

// DamageBonus returns the total damage bonus from all active conditions.
// Postcondition: value >= 0 (penalties on damage are not returned here per legacy contract).
func DamageBonus(s *ActiveSet) int {
    if s == nil {
        return 0
    }
    v := effect.Resolve(s.Effects(), effect.StatDamage).Total
    if v < 0 {
        return 0
    }
    return v
}

// SkillPenalty returns the flat all-skill penalty from active conditions.
// Postcondition: value >= 0.
func SkillPenalty(s *ActiveSet) int {
    if s == nil {
        return 0
    }
    v := effect.Resolve(s.Effects(), effect.StatSkill).Total
    if v >= 0 {
        return 0
    }
    return -v
}

// StealthBonus returns the stealth skill bonus from active conditions.
func StealthBonus(s *ActiveSet) int {
    if s == nil {
        return 0
    }
    v := effect.Resolve(s.Effects(), effect.Stat("skill:stealth")).Total
    if v < 0 {
        return 0
    }
    return v
}

// FlairBonus returns the Flair ability bonus from active conditions.
func FlairBonus(s *ActiveSet) int {
    if s == nil {
        return 0
    }
    v := effect.Resolve(s.Effects(), effect.StatFlair).Total
    if v < 0 {
        return 0
    }
    return v
}

// ReflexBonus is retained for API compatibility; returns 0 (Gunchete uses no Reflex saves).
func ReflexBonus(_ *ActiveSet) int { return 0 }

// ExtraWeaponDice, APReduction, SkipTurn, ForcedActionType, IsMovementPrevented,
// IsCommandsPrevented, IsTargetingPrevented, IsActionRestricted remain as-is
// (non-numeric-bonus conditions; not routed through EffectSet).
```

Keep the non-numeric modifier functions (`ExtraWeaponDice`, `APReduction`, `SkipTurn`, `ForcedActionType`, `IsMovementPrevented`, `IsCommandsPrevented`, `IsTargetingPrevented`, `IsActionRestricted`, `StunnedAPReduction`) unchanged — they don't route through the bonus pipeline.

- [ ] **Step 6: Run all condition tests including compat**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v 2>&1 | tail -30
```
Expected: all PASS.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```
Expected: no FAILs.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/condition/ && git commit -m "feat(condition): EffectSet integration; modifiers.go as effect.Resolve wrappers (#245)"
```

---

## Task 5: ClassFeature and TechnologyDef gain `PassiveBonuses`

**Files:**
- Modify: `internal/game/ruleset/class_feature.go`
- Modify: `internal/game/technology/model.go`

- [ ] **Step 1: Write failing tests**

```go
// Append to internal/game/ruleset/class_feature_test.go (or create new file)
// internal/game/ruleset/class_feature_passive_test.go
package ruleset_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestClassFeature_PassiveBonuses_ParsedFromYAML(t *testing.T) {
    yaml := `
class_features:
  - id: iron_will
    name: Iron Will
    passive: true
    passive_bonuses:
      - stat: grit
        value: 1
        type: status
`
    features, err := ruleset.LoadClassFeaturesFromBytes([]byte(yaml))
    assert.NoError(t, err)
    assert.Len(t, features, 1)
    assert.Len(t, features[0].PassiveBonuses, 1)
    assert.Equal(t, effect.StatGrit, features[0].PassiveBonuses[0].Stat)
    assert.Equal(t, 1, features[0].PassiveBonuses[0].Value)
    assert.Equal(t, effect.BonusTypeStatus, features[0].PassiveBonuses[0].Type)
}

func TestClassFeature_PassiveBonuses_EmptyByDefault(t *testing.T) {
    yaml := `
class_features:
  - id: toughness
    name: Toughness
    passive: true
`
    features, err := ruleset.LoadClassFeaturesFromBytes([]byte(yaml))
    assert.NoError(t, err)
    assert.Empty(t, features[0].PassiveBonuses)
}
```

```go
// internal/game/technology/model_passive_test.go
package technology_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/technology"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestTechnologyDef_PassiveBonuses_Field(t *testing.T) {
    def := &technology.TechnologyDef{
        ID: "neural_boost",
        PassiveBonuses: []effect.Bonus{
            {Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
        },
    }
    assert.Len(t, def.PassiveBonuses, 1)
    assert.Equal(t, effect.StatAttack, def.PassiveBonuses[0].Stat)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... ./internal/game/technology/... 2>&1 | grep -E "FAIL|undefined"
```

- [ ] **Step 3: Add `PassiveBonuses` to `ClassFeature`**

In `internal/game/ruleset/class_feature.go`, add the import and field:

```go
import "github.com/cory-johannsen/mud/internal/game/effect"

// In ClassFeature struct, add after existing fields:
// PassiveBonuses are always-on typed bonuses granted while this feature is active.
// Only meaningful when Active == false (passive features).
PassiveBonuses []effect.Bonus `yaml:"passive_bonuses,omitempty"`
```

- [ ] **Step 4: Add `PassiveBonuses` to `TechnologyDef`**

In `internal/game/technology/model.go`, add the import and field:

```go
import "github.com/cory-johannsen/mud/internal/game/effect"

// In TechnologyDef struct, add after existing fields:
// PassiveBonuses are always-on typed bonuses granted while this technology is active (Passive == true only).
PassiveBonuses []effect.Bonus `yaml:"passive_bonuses,omitempty"`
```

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... ./internal/game/technology/... -v 2>&1 | tail -20
```

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/ruleset/ internal/game/technology/ && git commit -m "feat(effect): ClassFeature and TechnologyDef gain PassiveBonuses field (#245)"
```

---

## Task 6: `Combatant.Effects` wiring and population at creation

**Files:**
- Modify: `internal/game/combat/combat.go` — add `Effects *effect.EffectSet` to `Combatant`
- Create: `internal/game/combat/combatant_effects.go` — `BuildCombatantEffects`, sync helpers
- Modify: `internal/gameserver/combat_handler.go` — call `BuildCombatantEffects` after creating `playerCbt` and `npcCbt`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/combat/combatant_effects_test.go
package combat_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/effect"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/technology"
)

func TestBuildCombatantEffects_ConditionEffectsIncluded(t *testing.T) {
    conds := condition.NewActiveSet()
    def := &condition.ConditionDef{ID: "inspired", Name: "Inspired", DurationType: "permanent",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}}}
    require.NoError(t, def.SynthesiseBonuses())
    require.NoError(t, conds.Apply("uid1", def, 1, -1))
    opts := combat.BuildEffectsOpts{
        BearerUID:     "uid1",
        Conditions:    conds,
    }
    es := combat.BuildCombatantEffects(opts)
    r := effect.Resolve(es, effect.StatAttack)
    assert.Equal(t, 1, r.Total)
}

func TestBuildCombatantEffects_FeatPassiveBonusIncluded(t *testing.T) {
    feats := []*ruleset.ClassFeature{
        {ID: "iron_will", Active: false,
         PassiveBonuses: []effect.Bonus{{Stat: effect.StatGrit, Value: 2, Type: effect.BonusTypeStatus}}},
    }
    opts := combat.BuildEffectsOpts{
        BearerUID:     "uid1",
        Conditions:    condition.NewActiveSet(),
        PassiveFeats:  feats,
    }
    es := combat.BuildCombatantEffects(opts)
    r := effect.Resolve(es, effect.StatGrit)
    assert.Equal(t, 2, r.Total)
}

func TestBuildCombatantEffects_TechPassiveBonusIncluded(t *testing.T) {
    techs := []*technology.TechnologyDef{
        {ID: "neural_boost", Passive: true,
         PassiveBonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeUntyped}}},
    }
    opts := combat.BuildEffectsOpts{
        BearerUID:     "uid1",
        Conditions:    condition.NewActiveSet(),
        PassiveTechs:  techs,
    }
    es := combat.BuildCombatantEffects(opts)
    r := effect.Resolve(es, effect.StatAttack)
    assert.Equal(t, 1, r.Total)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestBuildCombatantEffects" 2>&1 | head -20
```

- [ ] **Step 3: Add `Effects` field to `Combatant` in `combat.go`**

In `internal/game/combat/combat.go`, add to `Combatant`:

```go
import "github.com/cory-johannsen/mud/internal/game/effect"

// In Combatant struct, add after Conditions-adjacent fields:
// Effects is the unified typed-bonus set for this combatant.
// Populated at combatant creation from conditions + feat/tech passive bonuses + equipment bonuses.
Effects *effect.EffectSet
```

- [ ] **Step 4: Create `combatant_effects.go`**

```go
// internal/game/combat/combatant_effects.go
package combat

import (
    "fmt"

    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/effect"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/technology"
)

// BuildEffectsOpts carries all sources needed to build a Combatant's EffectSet.
type BuildEffectsOpts struct {
    BearerUID    string
    Conditions   *condition.ActiveSet
    PassiveFeats []*ruleset.ClassFeature  // nil = no feats
    PassiveTechs []*technology.TechnologyDef // nil = no techs
    // WeaponSourceID, WeaponBonusValue: for weapon item bonus (optional)
    WeaponSourceID   string
    WeaponBonusValue int
}

// BuildCombatantEffects constructs a fresh EffectSet from all effect sources.
// Precondition: opts.BearerUID must be non-empty.
// Postcondition: returns a non-nil EffectSet.
func BuildCombatantEffects(opts BuildEffectsOpts) *effect.EffectSet {
    es := effect.NewEffectSet()

    // 1. Condition effects from ActiveSet.
    if opts.Conditions != nil {
        for _, ac := range opts.Conditions.All() {
            if len(ac.Def.Bonuses) == 0 {
                continue
            }
            bonuses := make([]effect.Bonus, len(ac.Def.Bonuses))
            for i, b := range ac.Def.Bonuses {
                scaled := b
                scaled.Value = b.Value * ac.Stacks
                bonuses[i] = scaled
            }
            es.Apply(effect.Effect{
                EffectID:  ac.Def.ID,
                SourceID:  "condition:" + ac.Def.ID,
                CasterUID: opts.BearerUID,
                Bonuses:   bonuses,
                DurKind:   effect.DurationUntilRemove,
            })
        }
    }

    // 2. Feat passive bonuses.
    for _, f := range opts.PassiveFeats {
        if f.Active || len(f.PassiveBonuses) == 0 {
            continue
        }
        es.Apply(effect.Effect{
            EffectID:  f.ID,
            SourceID:  "feat:" + f.ID,
            CasterUID: opts.BearerUID,
            Bonuses:   f.PassiveBonuses,
            DurKind:   effect.DurationUntilRemove,
        })
    }

    // 3. Tech passive bonuses.
    for _, td := range opts.PassiveTechs {
        if !td.Passive || len(td.PassiveBonuses) == 0 {
            continue
        }
        es.Apply(effect.Effect{
            EffectID:  td.ID,
            SourceID:  "tech:" + td.ID,
            CasterUID: opts.BearerUID,
            Bonuses:   td.PassiveBonuses,
            DurKind:   effect.DurationUntilRemove,
        })
    }

    // 4. Weapon item bonus (if non-zero).
    if opts.WeaponSourceID != "" && opts.WeaponBonusValue != 0 {
        es.Apply(effect.Effect{
            EffectID:  opts.WeaponSourceID,
            SourceID:  "item:" + opts.WeaponSourceID,
            CasterUID: "",
            Bonuses: []effect.Bonus{
                {Stat: effect.StatAttack, Value: opts.WeaponBonusValue, Type: effect.BonusTypeItem},
                {Stat: effect.StatDamage, Value: opts.WeaponBonusValue, Type: effect.BonusTypeItem},
            },
            DurKind: effect.DurationUntilRemove,
        })
    }

    return es
}

// SyncConditionApply updates cbt.Effects when a condition is applied mid-combat.
// Call this immediately after cbt.Conditions.Apply succeeds.
func SyncConditionApply(cbt *Combatant, uid string, def *condition.ConditionDef, stacks int) {
    if cbt.Effects == nil {
        cbt.Effects = effect.NewEffectSet()
    }
    bonuses := make([]effect.Bonus, len(def.Bonuses))
    for i, b := range def.Bonuses {
        scaled := b
        scaled.Value = b.Value * stacks
        bonuses[i] = scaled
    }
    cbt.Effects.Apply(effect.Effect{
        EffectID:  def.ID,
        SourceID:  "condition:" + def.ID,
        CasterUID: uid,
        Bonuses:   bonuses,
        DurKind:   effect.DurationUntilRemove,
    })
}

// SyncConditionRemove updates cbt.Effects when a condition is removed mid-combat.
// Call this immediately after cbt.Conditions.Remove completes.
func SyncConditionRemove(cbt *Combatant, conditionID string) {
    if cbt.Effects == nil {
        return
    }
    cbt.Effects.RemoveBySource("condition:" + conditionID)
}

// SyncConditionsTick updates cbt.Effects for expired conditions after a round tick.
// expiredIDs is the list returned by condition.ActiveSet.Tick().
func SyncConditionsTick(cbt *Combatant, expiredIDs []string) {
    if cbt.Effects == nil {
        return
    }
    for _, id := range expiredIDs {
        cbt.Effects.RemoveBySource("condition:" + id)
    }
}

// OverrideNarrativeEvents computes which stats changed between active/overridden
// state before vs after an effect change, returning narrative strings for combat log.
func OverrideNarrativeEvents(before, after *effect.EffectSet, stats []effect.Stat) []string {
    var events []string
    for _, stat := range stats {
        rb := effect.Resolve(before, stat)
        ra := effect.Resolve(after, stat)
        // Detect newly-overridden contributions.
        for _, sup := range ra.Suppressed {
            wasContributing := false
            for _, c := range rb.Contributing {
                if c.EffectID == sup.EffectID {
                    wasContributing = true
                    break
                }
            }
            if wasContributing {
                events = append(events, fmt.Sprintf("[EFFECT] %s (%s) is overridden by %s.",
                    sup.SourceID, stat, sup.OverriddenBy.SourceID))
            }
        }
    }
    return events
}
```

- [ ] **Step 5: Populate `Effects` in `combat_handler.go` at playerCbt and npcCbt creation**

In `internal/gameserver/combat_handler.go`, after building `playerCbt` (around line 1846), add:

```go
// Populate the unified EffectSet.
// PassiveFeats and PassiveTechs are not wired in this PR (no registry lookup here);
// they default to nil; feat/tech passive bonuses are a follow-on wiring step.
playerCbt.Effects = combat.BuildCombatantEffects(combat.BuildEffectsOpts{
    BearerUID:        playerSess.UID,
    Conditions:       h.combatConditions(playerSess.UID, roomID), // see note below
    WeaponSourceID:   weaponSourceID,
    WeaponBonusValue: playerCbt.WeaponBonus,
})
```

Note: `h.combatConditions(uid, roomID)` is a helper you need to add — it returns `cbt.Conditions[uid]` for the active combat in roomID, or `nil` if not in combat. Add this private method to `CombatHandler`:

```go
func (h *CombatHandler) combatConditions(uid, roomID string) *condition.ActiveSet {
    h.mu.RLock()
    defer h.mu.RUnlock()
    cbt, ok := h.combats[roomID]
    if !ok {
        return nil
    }
    return cbt.Conditions[uid]
}
```

For `npcCbt`, pass `Conditions: cbt.Conditions[inst.ID]` (NPC combatants don't have passive feats/techs in v1, so PassiveFeats/PassiveTechs remain nil).

Also call this for the combatant construction in all three `&combat.Combatant{}` creation sites in `combat_handler.go` (lines ~1803, ~3008, ~3100). Identify them by the `playerCbt :=` and `npcCbt :=` variable names.

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestBuildCombatantEffects" -v 2>&1 | tail -20
```

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/ internal/gameserver/combat_handler.go && git commit -m "feat(effect): Combatant.Effects wired at creation; BuildCombatantEffects provider (#245)"
```

---

## Task 7: Combat resolver migration — replace `condition.AttackBonus` call sites in `round.go`

**Files:**
- Modify: `internal/game/combat/round.go`
- Create: `internal/game/combat/round_effect_test.go`

- [ ] **Step 1: Write failing end-to-end test**

```go
// internal/game/combat/round_effect_test.go
package combat_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/effect"
    "github.com/cory-johannsen/mud/internal/dice"
    "math/rand"
)

// TestResolveRound_EffectPipelineContributesToAttack verifies that an attack bonus
// in Combatant.Effects is factored into the attack roll total.
func TestResolveRound_EffectPipelineContributesToAttack(t *testing.T) {
    // Build minimal combat with a player and one NPC.
    src := dice.NewRandSource(rand.NewSource(42))

    attacker := &combat.Combatant{
        ID: "player", Kind: combat.KindPlayer,
        Name: "Test", Level: 1, MaxHP: 20, CurrentHP: 20,
        AC: 10, StrMod: 0,
        Effects: effect.NewEffectSet(),
    }
    // Apply a +2 status bonus to attack via Effects (not via condition.Apply).
    attacker.Effects.Apply(effect.Effect{
        EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "player",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove,
    })

    target := &combat.Combatant{
        ID: "npc1", Kind: combat.KindNPC,
        Name: "Goon", Level: 1, MaxHP: 10, CurrentHP: 10,
        AC: 100, // nearly impossible to hit — we'll check AttackTotal includes the +2
        Effects: effect.NewEffectSet(),
    }

    cbt := &combat.Combat{
        RoomID:      "room1",
        Combatants:  []*combat.Combatant{attacker, target},
        Conditions:  map[string]*condition.ActiveSet{"player": condition.NewActiveSet(), "npc1": condition.NewActiveSet()},
        ActionQueues: map[string]*combat.ActionQueue{
            "player": {Actions: []combat.QueuedAction{{Type: combat.ActionAttack, Target: "Goon"}}},
            "npc1":   {Actions: []combat.QueuedAction{{Type: combat.ActionPass}}},
        },
    }

    events := combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil)
    require.NotEmpty(t, events)

    // Find the attack event for the player.
    var atkEvent *combat.RoundEvent
    for i := range events {
        if events[i].ActorID == "player" && events[i].ActionType == combat.ActionAttack {
            atkEvent = &events[i]
            break
        }
    }
    require.NotNil(t, atkEvent)
    require.NotNil(t, atkEvent.AttackResult)
    // AttackTotal = d20 + StrMod(0) + proficiency + weapon bonus + effect bonus(+2)
    // With seed 42 and no proficiency, the +2 should be visible in AttackTotal.
    // We can't assert exact total without knowing the roll, so assert it's non-zero.
    assert.Greater(t, atkEvent.AttackResult.AttackTotal, 0)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestResolveRound_EffectPipeline" -v 2>&1 | tail -20
```

- [ ] **Step 3: Migrate `round.go` call sites**

Find all 10 `condition.AttackBonus`, `condition.ACBonus`, `condition.DamageBonus`, and `condition.ExtraWeaponDice` call sites in `round.go` (lines ~718-1330) and replace them.

Replacement pattern for attack bonus:
```go
// BEFORE:
atkBonus := condition.AttackBonus(cbt.Conditions[actor.ID])
// AFTER:
atkBonus := effect.Resolve(actor.Effects, effect.StatAttack).Total
```

Replacement pattern for AC bonus:
```go
// BEFORE:
acBonus := condition.ACBonus(cbt.Conditions[target.ID])
// AFTER:
acBonus := effect.Resolve(target.Effects, effect.StatAC).Total
```

Replacement pattern for damage bonus:
```go
// BEFORE:
dmg += condition.DamageBonus(cbt.Conditions[actor.ID])
// AFTER:
dmgBonus := effect.Resolve(actor.Effects, effect.StatDamage).Total
if dmgBonus > 0 { dmg += dmgBonus }
```

Keep `condition.ExtraWeaponDice(cbt.Conditions[actor.ID])` unchanged — `ExtraWeaponDice` is a non-numeric boolean-style mechanic not in the Stat enum.

Add the import: `"github.com/cory-johannsen/mud/internal/game/effect"` to `round.go`.

Make sure `actor.Effects` is non-nil before calling `effect.Resolve`. Add nil guard: if actor.Effects is nil, fall back to 0 (safe no-op since `effect.Resolve(nil, stat).Total == 0`).

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v 2>&1 | tail -30
```

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/round.go internal/game/combat/round_effect_test.go && git commit -m "feat(effect): migrate round.go to effect.Resolve pipeline for attack/AC/damage (#245)"
```

---

## Task 8: Override narrative events (DEDUP-14) and mid-combat condition sync

**Files:**
- Modify: `internal/game/combat/round.go` — call `SyncConditionsTick` after round tick; emit override narratives
- Modify: `internal/gameserver/combat_handler.go` — call `SyncConditionApply`/`SyncConditionRemove` when conditions change

- [ ] **Step 1: Write failing tests for override narrative**

```go
// Append to internal/game/combat/combatant_effects_test.go
func TestOverrideNarrativeEvents_DetectsNewSuppression(t *testing.T) {
    before := effect.NewEffectSet()
    before.Apply(effect.Effect{EffectID: "inspire", SourceID: "condition:inspire_courage", CasterUID: "xin",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})

    after := effect.NewEffectSet()
    after.Apply(effect.Effect{EffectID: "inspire", SourceID: "condition:inspire_courage", CasterUID: "xin",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    after.Apply(effect.Effect{EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "kira",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})

    events := combat.OverrideNarrativeEvents(before, after, []effect.Stat{effect.StatAttack})
    assert.Len(t, events, 1)
    assert.Contains(t, events[0], "overridden")
}

func TestOverrideNarrativeEvents_NoChangeNoEvents(t *testing.T) {
    s := effect.NewEffectSet()
    s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:bless", CasterUID: "",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove})
    events := combat.OverrideNarrativeEvents(s, s, []effect.Stat{effect.StatAttack})
    assert.Empty(t, events)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestOverrideNarrative" -v 2>&1 | tail -10
```

- [ ] **Step 3: Wire `SyncConditionsTick` into `round.go` StartRound**

In `internal/game/combat/engine.go` (or wherever `StartRoundWithSrc` calls `Tick`), after each combatant's condition tick, call `SyncConditionsTick(cbt_ptr, expiredIDs)`.

Find the Tick call pattern — it should be something like:
```go
expired := cbt.Conditions[c.ID].Tick(c.ID)
```

Change to:
```go
expired := cbt.Conditions[c.ID].Tick(c.ID)
SyncConditionsTick(c, expired)
```

- [ ] **Step 4: Wire `SyncConditionApply`/`SyncConditionRemove` in `combat_handler.go`**

Search for all `cbt.Conditions[uid].Apply(...)` and `cbt.Conditions[uid].Remove(...)` call sites in `combat_handler.go` and wrap them:

```go
// After a successful Apply:
if combatant := h.findCombatant(roomID, uid); combatant != nil {
    combat.SyncConditionApply(combatant, uid, def, stacks)
}

// After a Remove:
if combatant := h.findCombatant(roomID, uid); combatant != nil {
    combat.SyncConditionRemove(combatant, conditionID)
}
```

Add private helper `findCombatant(roomID, uid string) *combat.Combatant` to `CombatHandler`:

```go
func (h *CombatHandler) findCombatant(roomID, uid string) *combat.Combatant {
    h.mu.RLock()
    defer h.mu.RUnlock()
    cbt, ok := h.combats[roomID]
    if !ok {
        return nil
    }
    for _, c := range cbt.Combatants {
        if c.ID == uid {
            return c
        }
    }
    return nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... ./internal/gameserver/... -v -run "TestOverrideNarrative|TestSync" 2>&1 | tail -20
```

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/ internal/gameserver/combat_handler.go && git commit -m "feat(effect): mid-combat condition sync and override narrative events (DEDUP-14) (#245)"
```

---

## Task 9: Telnet `effects` command and rendering

**Files:**
- Modify: `internal/frontend/handlers/mode_combat.go` — handle `effects` and `effects detail <id>` commands
- Modify: `internal/frontend/handlers/text_renderer.go` — `renderEffectsBlock()` function

- [ ] **Step 1: Write failing tests**

```go
// internal/frontend/handlers/effects_render_test.go
package handlers_test

import (
    "testing"
    "strings"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/frontend/handlers"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestRenderEffectsBlock_Empty(t *testing.T) {
    es := effect.NewEffectSet()
    out := handlers.RenderEffectsBlock(es, nil, 80)
    assert.Contains(t, out, "No active effects")
}

func TestRenderEffectsBlock_SingleActive(t *testing.T) {
    es := effect.NewEffectSet()
    es.Apply(effect.Effect{
        EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "kira",
        Annotation: "Heroism",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove,
    })
    casterNames := map[string]string{"kira": "Kira"}
    out := handlers.RenderEffectsBlock(es, casterNames, 80)
    assert.Contains(t, out, "heroism")
    assert.Contains(t, out, "active")
    assert.Contains(t, out, "attack +1 status")
}

func TestRenderEffectsBlock_Suppressed(t *testing.T) {
    es := effect.NewEffectSet()
    es.Apply(effect.Effect{
        EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "",
        Annotation: "Heroism",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove,
    })
    es.Apply(effect.Effect{
        EffectID: "inspire", SourceID: "condition:inspire_courage", CasterUID: "",
        Annotation: "Inspire Courage",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove,
    })
    out := handlers.RenderEffectsBlock(es, nil, 80)
    assert.Contains(t, out, "overridden")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestRenderEffectsBlock" 2>&1 | head -20
```

- [ ] **Step 3: Implement `RenderEffectsBlock` in `text_renderer.go`**

Add `RenderEffectsBlock` as a package-level function in `internal/frontend/handlers/text_renderer.go`:

```go
// RenderEffectsBlock renders all active effects for a bearer as a text block.
// casterNames maps casterUID → display name; nil is safe (uses "item"/"self" labels).
// width is the terminal column width.
func RenderEffectsBlock(es *effect.EffectSet, casterNames map[string]string, width int) string {
    if es == nil || len(es.All()) == 0 {
        return "Effects:\n  No active effects.\n"
    }

    // Build per-stat resolved views for all stats we care about.
    statList := []effect.Stat{
        effect.StatAttack, effect.StatAC, effect.StatDamage, effect.StatSpeed,
        effect.StatGrit, effect.StatQuickness, effect.StatSavvy, effect.StatFlair,
        effect.StatSkill,
    }
    // Map effectID → list of contribution annotations.
    type annotation struct { line string; suppressed bool }
    annotMap := map[string][]annotation{}

    for _, stat := range statList {
        r := effect.Resolve(es, stat)
        for _, c := range r.Contributing {
            sign := "+"
            if c.Value < 0 { sign = "" }
            line := fmt.Sprintf("%s %s%d %s", stat, sign, c.Value, strings.ToLower(string(c.BonusType)))
            annotMap[c.EffectID] = append(annotMap[c.EffectID], annotation{line: line, suppressed: false})
        }
        for _, c := range r.Suppressed {
            sign := "+"
            if c.Value < 0 { sign = "" }
            overriddenBy := c.OverriddenBy.SourceID
            line := fmt.Sprintf("%s %s%d %s  (overridden by %s)", stat, sign, c.Value, strings.ToLower(string(c.BonusType)), overriddenBy)
            annotMap[c.EffectID] = append(annotMap[c.EffectID], annotation{line: line, suppressed: true})
        }
    }

    var sb strings.Builder
    sb.WriteString("Effects:\n")
    for _, e := range es.All() {
        casterLabel := "self"
        if e.CasterUID != "" {
            if name, ok := casterNames[e.CasterUID]; ok {
                casterLabel = "from " + name
            }
        }
        if strings.HasPrefix(e.SourceID, "item:") {
            casterLabel = "item"
        } else if strings.HasPrefix(e.SourceID, "feat:") {
            casterLabel = "feat"
        } else if strings.HasPrefix(e.SourceID, "tech:") {
            casterLabel = "tech"
        }
        displayName := e.Annotation
        if displayName == "" {
            displayName = e.SourceID
        }
        annots := annotMap[e.EffectID]
        if len(annots) == 0 {
            fmt.Fprintf(&sb, "  %-25s (%-12s)  [no stat bonuses]\n", displayName, casterLabel)
            continue
        }
        first := true
        for _, a := range annots {
            status := "(active)"
            if a.suppressed { status = "(overridden)" }
            if first {
                fmt.Fprintf(&sb, "  %-25s (%-12s)  %-35s %s\n", displayName, casterLabel, a.line, status)
                first = false
            } else {
                fmt.Fprintf(&sb, "  %-25s  %-12s   %-35s %s\n", "", "", a.line, status)
            }
        }
    }
    return sb.String()
}
```

Add the needed imports: `"fmt"`, `"strings"`, `"github.com/cory-johannsen/mud/internal/game/effect"`.

- [ ] **Step 4: Add `effects` command to `mode_combat.go`**

In the combat mode command handler, add:

```go
case "effects":
    if len(parts) >= 3 && parts[1] == "detail" {
        effectID := parts[2]
        // Show per-stat breakdown for one effect.
        // Get the effect from session's combat EffectSet.
        // (Implementation: call grpc StatusRequest or use session's cached EffectSet)
        h.writeConsole(fmt.Sprintf("Effect detail for '%s' not yet implemented in telnet.\n", effectID))
        return
    }
    // Print full effects block.
    es := h.getCombatantEffects() // returns current combatant's Effects EffectSet
    if es == nil {
        h.writeConsole("No effects active.\n")
        return
    }
    h.writeConsole(RenderEffectsBlock(es, h.getCasterNames(), h.screenWidth()))
```

Add a `getCombatantEffects()` method on the combat handler that returns the player's current `Combatant.Effects` from the active combat session (via the grpc bridge or session cache).

The simplest approach: add a `lastEffects *effect.EffectSet` field to the combat mode state, populated when the game bridge receives a `CharacterSheetView` update (which already carries `EffectsSummary`). Wire `getCombatantEffects()` to return that field.

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestRenderEffectsBlock" -v 2>&1 | tail -20
```

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/ && git commit -m "feat(effect): telnet effects command and RenderEffectsBlock (#245)"
```

---

## Task 10: Web Effects panel

**Files:**
- Create: `cmd/webclient/ui/src/game/drawers/EffectsDrawer.tsx`
- Modify: `cmd/webclient/ui/src/game/drawers/DrawerContainer.tsx` — add `'effects'` type and render

The proto `CharacterSheetView` already has an `EffectsSummary string` field (line 4622 of `game.pb.go`). The web client should receive this field and display it. For v1, a simple text display of the summary string (populated by the server-side formatter) is sufficient.

- [ ] **Step 1: Write failing component test**

```tsx
// cmd/webclient/ui/src/game/drawers/EffectsDrawer.test.tsx
import { render, screen } from '@testing-library/react'
import { EffectsDrawer } from './EffectsDrawer'
import { GameContext } from '../GameContext'

const mockContext = {
  state: {
    characterSheet: {
      effectsSummary: 'Heroism (from Kira)  attack +1 status  (active)\n'
    }
  },
  dispatch: jest.fn()
}

test('renders effects summary from character sheet', () => {
  render(
    <GameContext.Provider value={mockContext as any}>
      <EffectsDrawer onClose={() => {}} />
    </GameContext.Provider>
  )
  expect(screen.getByText(/Heroism/)).toBeInTheDocument()
})

test('shows empty state when no effects', () => {
  const emptyCtx = { ...mockContext, state: { characterSheet: { effectsSummary: '' } } }
  render(
    <GameContext.Provider value={emptyCtx as any}>
      <EffectsDrawer onClose={() => {}} />
    </GameContext.Provider>
  )
  expect(screen.getByText(/No active effects/)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm test -- --testPathPattern=EffectsDrawer --watchAll=false 2>&1 | tail -20
```

- [ ] **Step 3: Implement `EffectsDrawer.tsx`**

```tsx
// cmd/webclient/ui/src/game/drawers/EffectsDrawer.tsx
import React, { useContext } from 'react'
import { GameContext } from '../GameContext'

interface Props {
  onClose: () => void
}

export function EffectsDrawer({ onClose }: Props) {
  const { state } = useContext(GameContext)
  const summary = state.characterSheet?.effectsSummary ?? ''

  return (
    <div className="drawer-content effects-drawer">
      <div className="drawer-header">
        <h2>Active Effects</h2>
        <button onClick={onClose} aria-label="Close">✕</button>
      </div>
      <div className="drawer-body">
        {summary.trim() === '' ? (
          <p className="effects-empty">No active effects.</p>
        ) : (
          <pre className="effects-summary">{summary}</pre>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Update `DrawerContainer.tsx`**

```tsx
// Add to imports:
import { EffectsDrawer } from './EffectsDrawer'

// Change type:
export type DrawerType = 'inventory' | 'equipment' | 'skills' | 'feats' | 'stats' | 'technology' | 'job' | 'explore' | 'quests' | 'effects'

// Add to render:
{openDrawer === 'effects' && <EffectsDrawer onClose={onClose} />}
```

- [ ] **Step 5: Populate `effectsSummary` on the server side**

In the gameserver handler that builds `CharacterSheetView`, find where `EffectsSummary` is populated (or is currently empty) and set it using the text renderer:

```go
// In the handler that builds CharacterSheetView (search for "EffectsSummary" in combat_handler.go):
if cbt := h.findCombatantInRoom(roomID, uid); cbt != nil && cbt.Effects != nil {
    charSheet.EffectsSummary = handlers.RenderEffectsBlock(cbt.Effects, casterNamesForRoom(cbt, roomID, h), 80)
}
```

Note: `casterNamesForRoom` maps caster UIDs to their display names using `h.sessionGetter`.

- [ ] **Step 6: Run web tests**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm test -- --testPathPattern=EffectsDrawer --watchAll=false 2>&1 | tail -20
```

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -20
```

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/webclient/ui/src/game/drawers/ && git commit -m "feat(effect): web EffectsDrawer panel with effectsSummary display (#245)"
```

---

## Task 11: Character sheet — Effects block on telnet character sheet

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` — append effects block to `RenderCharacterSheet`

- [ ] **Step 1: Write failing test**

```go
// Append to internal/frontend/handlers/text_renderer_test.go (or create file)
// internal/frontend/handlers/text_renderer_effects_test.go
package handlers_test

import (
    "testing"
    "strings"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/frontend/handlers"
    "github.com/cory-johannsen/mud/internal/game/effect"
)

func TestRenderCharacterSheet_IncludesEffectsBlock(t *testing.T) {
    // Build a minimal CharacterSheetView-equivalent and an EffectSet.
    // The exact CharacterSheetView proto type is gamev1.CharacterSheetView.
    // Since RenderCharacterSheet takes that type, we need a valid instance.
    // This test only checks that the effects block appears in output.
    es := effect.NewEffectSet()
    es.Apply(effect.Effect{
        EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "",
        Annotation: "Heroism",
        Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
        DurKind: effect.DurationUntilRemove,
    })
    block := handlers.RenderEffectsBlock(es, nil, 80)
    assert.True(t, strings.Contains(block, "heroism") || strings.Contains(block, "Heroism"))
    assert.Contains(t, block, "attack +1 status")
}
```

This test validates `RenderEffectsBlock` in isolation (the `RenderCharacterSheet` function is tested via the existing property test suite).

- [ ] **Step 2: Run the test**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestRenderCharacterSheet_IncludesEffectsBlock" -v 2>&1 | tail -20
```

- [ ] **Step 3: Add effects block to `RenderCharacterSheet`**

In `internal/frontend/handlers/text_renderer.go`, find the `RenderCharacterSheet` function. At the end of the function, before the final return, append the effects block if an EffectSet is available. Since `RenderCharacterSheet` currently takes `(csv *gamev1.CharacterSheetView, width int)`, you have two options:

**Option A (preferred):** Use the existing `EffectsSummary` string field on `CharacterSheetView` — if it's non-empty, append it directly:

```go
// At the end of RenderCharacterSheet, before returning:
if csv.EffectsSummary != "" {
    sb.WriteString("\n")
    sb.WriteString("── Effects ──────────────────────────────────────────────\n")
    sb.WriteString(csv.EffectsSummary)
}
```

The server populates `EffectsSummary` in Task 10 Step 5. This approach requires no signature change.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -v 2>&1 | tail -20
```

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_effects_test.go && git commit -m "feat(effect): effects block in telnet character sheet (#245)"
```

---

## Task 12: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`
- Modify: `docs/architecture/CHARACTERS.md`

- [ ] **Step 1: Add "Bonus stacking" section to `docs/architecture/combat.md`**

Find the appropriate place in `combat.md` (after the round resolution section) and insert:

```markdown
## Bonus Stacking (DEDUP Requirements)

All numeric bonuses to combat stats flow through `internal/game/effect/EffectSet` and `effect.Resolve`. No subsystem computes bonus totals outside this pipeline (DEDUP-10).

### Bonus Types

| Type | Stacking Rule |
|------|--------------|
| `status` | Only the highest single positive bonus contributes per stat; only the worst single penalty contributes |
| `circumstance` | Same as status |
| `item` | Same as status |
| `untyped` | All values stack additively |

### Dedup Key

Effects are deduplicated by `(source_id, caster_uid)`. Re-applying the same key overwrites the previous entry (DEDUP-1).

### Resolve Algorithm

`effect.Resolve(set, stat)` is a pure function (DEDUP-7). Per-type: highest-bonus-wins, worst-penalty-wins. Untyped: sum all. Ties broken lexicographically by `(SourceID, CasterUID)` ascending (DEDUP-6).

### Stat Inheritance

A bonus to `skill` contributes to any `skill:<id>` query (prefix-on-colon). A bonus to `skill:stealth` does NOT contribute to `skill:savvy` (DEDUP-16).

### Effect Sources

| Source prefix | Origin |
|--------------|--------|
| `condition:<id>` | Applied via `condition.ActiveSet` |
| `feat:<id>` | Passive `ClassFeature.PassiveBonuses` |
| `tech:<id>` | Passive `TechnologyDef.PassiveBonuses` |
| `item:<instance>` | Equipped weapon/armor item bonus |

### Combatant Effects Lifecycle

1. At combatant creation: `combat.BuildCombatantEffects()` populates `Combatant.Effects` from all sources.
2. On condition apply mid-combat: `combat.SyncConditionApply()` updates `Combatant.Effects`.
3. On condition removal: `combat.SyncConditionRemove()` updates `Combatant.Effects`.
4. On round tick: `combat.SyncConditionsTick()` removes expired condition effects.
5. `combat.OverrideNarrativeEvents()` diffs before/after Resolve to emit suppression log lines (DEDUP-14).
```

- [ ] **Step 2: Update `docs/architecture/CHARACTERS.md`**

Add a section describing the `EffectSet` on `Combatant` and the provider pattern:

```markdown
## Effect Pipeline

In combat, each `Combatant` carries an `Effects *effect.EffectSet` (package `internal/game/effect`).
This is the single source of truth for all typed bonus calculations during a combat encounter.

### Provider Pattern

At combatant creation, `BuildCombatantEffects(BuildEffectsOpts)` merges:
1. Condition effects from `condition.ActiveSet`
2. `ClassFeature.PassiveBonuses` for each passive feat the player holds
3. `TechnologyDef.PassiveBonuses` for each passive tech the player holds
4. Weapon item bonus (from `WeaponDef.Bonus`) as `item:`-typed entries

Each source is keyed by `source_id` (e.g. `"condition:prone"`, `"feat:iron_will"`, `"item:my_sword_instance"`).

### Out-of-Combat Bonus Queries

`condition/modifiers.go` functions (`AttackBonus`, `ACBonus`, etc.) remain available for out-of-combat callers. They delegate to `effect.Resolve(activeSet.Effects(), stat).Total` internally.
```

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add docs/architecture/ && git commit -m "docs: bonus stacking architecture — DEDUP requirements, Resolve algorithm, provider pattern (#245)"
```

---

## Self-Review

**Spec coverage check:**

| Requirement | Covered by |
|-------------|-----------|
| DEDUP-1: dedup on (bearer, source, caster) | Task 2 `EffectSet.Apply` |
| DEDUP-2: BonusType with untyped default | Task 1 `Bonus.Normalise` |
| DEDUP-3: highest typed bonus wins | Task 3 `Resolve` / `pickHighest` |
| DEDUP-4: worst typed penalty wins | Task 3 `Resolve` / `pickLowest` |
| DEDUP-5: untyped always stacks | Task 3 `Resolve` |
| DEDUP-6: lex tiebreak | Task 3 `pickHighest`/`pickLowest` |
| DEDUP-7: Resolve is pure | Task 3 (no side effects) |
| DEDUP-8: monotonic version | Task 2 `Version()` |
| DEDUP-9: multi-stat bonuses | Task 3 (loop over `effect.Bonuses`) |
| DEDUP-10: no bonus outside effect.Resolve | Task 4 modifiers.go rewrite; Task 7 round.go |
| DEDUP-11: flat+bonuses mix is load error | Task 4 `SynthesiseBonuses()` |
| DEDUP-12: caster-linked removal | Task 2 `RemoveByCaster` |
| DEDUP-13: character sheet shows (active)/(overridden) | Task 9 `RenderEffectsBlock`; Task 11 `RenderCharacterSheet` |
| DEDUP-14: narrative on override transition | Task 8 `OverrideNarrativeEvents` |
| DEDUP-15: zero bonus rejected | Task 1 `Bonus.Validate`; Task 4 `SynthesiseBonuses` skips zero |
| DEDUP-16: prefix-on-colon stat matching | Task 1 `StatMatches` |

**Placeholder scan:** No TBDs, no "implement later" phrases, no vague requirements.

**Type consistency:** `BuildEffectsOpts` uses `[]*ruleset.ClassFeature` and `[]*technology.TechnologyDef` consistently across Tasks 5-7. `effect.Stat`, `effect.BonusType`, `effect.Bonus` types used identically in Tasks 1-4 and downstream.

**Spec §12 Open Questions resolved:**
- File locations: `internal/game/effect/`
- Provider pattern: pull-based (`BuildCombatantEffects` at creation)
- Equipment effects location: `internal/game/combat/combatant_effects.go`
- Telnet column widths: `%-25s %-12s %-35s` (tested against 80-column width)
