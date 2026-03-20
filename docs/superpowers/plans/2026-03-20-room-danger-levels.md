# Room Danger Levels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Room Danger Levels feature end-to-end: a typed `DangerLevel` enum, per-zone/room YAML fields, `CanInitiateCombat` enforcement, trap probability rolls, a WantedLevel system with DB persistence and calendar-driven decay, guard combat initiation, proto/map updates, ANSI color-coded map rendering, and an architecture doc.

**Architecture:** Functional `danger` package (pure functions, no state) → world model YAML fields → session in-memory cache → Postgres persistence (`character_wanted_levels`) → gRPC service wiring (`handleMove`, `handleMap`, `handleAttack`) → frontend ANSI rendering in `RenderMap`.

**Tech Stack:** Go 1.23+, pgx v5, protobuf/grpc, pgregory.net/rapid (property-based testing), ANSI escape codes, PostgreSQL migrations.

**Spec:** `docs/superpowers/specs/2026-03-20-room-danger-levels-design.md`
**Feature:** `docs/features/room-danger-levels.md`

---

## Task 1: `danger` package — DangerLevel type

**Files:**
- Create: `internal/game/danger/level.go`
- Create: `internal/game/danger/level_test.go`

### Steps

- [ ] Create `internal/game/danger/level.go`:

```go
package danger

// DangerLevel represents the threat level of a zone or room.
type DangerLevel string

const (
	Safe      DangerLevel = "safe"
	Sketchy   DangerLevel = "sketchy"
	Dangerous DangerLevel = "dangerous"
	AllOutWar DangerLevel = "all_out_war"
)

// EffectiveDangerLevel returns roomDanger if non-empty, else zoneDanger.
// Precondition: zoneDanger MUST be a valid DangerLevel string.
// Postcondition: returns the effective DangerLevel for the room.
func EffectiveDangerLevel(zoneDanger, roomDanger string) DangerLevel {
	if roomDanger != "" {
		return DangerLevel(roomDanger)
	}
	return DangerLevel(zoneDanger)
}
```

- [ ] Create `internal/game/danger/level_test.go` with table-driven tests covering:
  - Zone-only (roomDanger empty) → returns zone value
  - Room override (both non-empty) → returns room value
  - Both empty → returns empty DangerLevel
  - Room empty, zone non-empty → returns zone value

```go
package danger_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

func TestEffectiveDangerLevel(t *testing.T) {
	tests := []struct {
		name       string
		zoneDanger string
		roomDanger string
		want       danger.DangerLevel
	}{
		{
			name:       "zone only",
			zoneDanger: "dangerous",
			roomDanger: "",
			want:       danger.Dangerous,
		},
		{
			name:       "room overrides zone",
			zoneDanger: "dangerous",
			roomDanger: "safe",
			want:       danger.Safe,
		},
		{
			name:       "both empty",
			zoneDanger: "",
			roomDanger: "",
			want:       danger.DangerLevel(""),
		},
		{
			name:       "room empty zone set",
			zoneDanger: "all_out_war",
			roomDanger: "",
			want:       danger.AllOutWar,
		},
		{
			name:       "room set zone empty",
			zoneDanger: "",
			roomDanger: "sketchy",
			want:       danger.Sketchy,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := danger.EffectiveDangerLevel(tc.zoneDanger, tc.roomDanger)
			if got != tc.want {
				t.Errorf("EffectiveDangerLevel(%q, %q) = %q; want %q", tc.zoneDanger, tc.roomDanger, got, tc.want)
			}
		})
	}
}
```

- [ ] Run tests (must pass):

```
go test ./internal/game/danger/... -v -run TestEffectiveDangerLevel
```

Expected output: all 5 sub-tests PASS.

- [ ] Commit:

```
git add internal/game/danger/level.go internal/game/danger/level_test.go
git commit -m "feat(danger): add DangerLevel type and EffectiveDangerLevel"
```

---

## Task 2: `danger` package — CanInitiateCombat

**Files:**
- Create: `internal/game/danger/combat.go`
- Create: `internal/game/danger/combat_test.go`

### Steps

- [ ] Create `internal/game/danger/combat.go`:

```go
package danger

// CanInitiateCombat reports whether the given initiator may start combat
// at this danger level. initiator is "player" or "npc".
// Precondition: initiator is "player" or "npc".
// Postcondition: returns false for all initiators in Safe rooms;
//   NPCs cannot initiate in Sketchy rooms; both may initiate in Dangerous and AllOutWar.
// Note: guard enforcement via InitiateGuardCombat bypasses this function.
func CanInitiateCombat(level DangerLevel, initiator string) bool {
	switch level {
	case Safe:
		return false
	case Sketchy:
		return initiator == "player"
	case Dangerous, AllOutWar:
		return true
	default:
		return false
	}
}
```

- [ ] Create `internal/game/danger/combat_test.go` with table-driven unit tests and property-based tests using `pgregory.net/rapid`:

```go
package danger_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

func TestCanInitiateCombat_Table(t *testing.T) {
	tests := []struct {
		level     danger.DangerLevel
		initiator string
		want      bool
	}{
		{danger.Safe, "player", false},
		{danger.Safe, "npc", false},
		{danger.Sketchy, "player", true},
		{danger.Sketchy, "npc", false},
		{danger.Dangerous, "player", true},
		{danger.Dangerous, "npc", true},
		{danger.AllOutWar, "player", true},
		{danger.AllOutWar, "npc", true},
	}
	for _, tc := range tests {
		t.Run(string(tc.level)+"/"+tc.initiator, func(t *testing.T) {
			got := danger.CanInitiateCombat(tc.level, tc.initiator)
			if got != tc.want {
				t.Errorf("CanInitiateCombat(%q, %q) = %v; want %v", tc.level, tc.initiator, got, tc.want)
			}
		})
	}
}

func TestCanInitiateCombat_Property(t *testing.T) {
	levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
	initiators := []string{"player", "npc"}

	rapid.Check(t, func(t *rapid.T) {
		level := rapid.SampledFrom(levels).Draw(t, "level")
		initiator := rapid.SampledFrom(initiators).Draw(t, "initiator")
		result := danger.CanInitiateCombat(level, initiator)

		// Safe: no one may initiate
		if level == danger.Safe && result {
			t.Fatalf("CanInitiateCombat(Safe, %q) = true; want false", initiator)
		}
		// Sketchy: npc may never initiate
		if level == danger.Sketchy && initiator == "npc" && result {
			t.Fatalf("CanInitiateCombat(Sketchy, npc) = true; want false")
		}
		// Dangerous/AllOutWar: everyone may initiate
		if (level == danger.Dangerous || level == danger.AllOutWar) && !result {
			t.Fatalf("CanInitiateCombat(%q, %q) = false; want true", level, initiator)
		}
	})
}
```

- [ ] Run tests (must pass):

```
go test ./internal/game/danger/... -v -run TestCanInitiateCombat
```

Expected output: all 8 table sub-tests and the property test PASS.

- [ ] Commit:

```
git add internal/game/danger/combat.go internal/game/danger/combat_test.go
git commit -m "feat(danger): add CanInitiateCombat"
```

---

## Task 3: `danger` package — Trap rolls

**Files:**
- Create: `internal/game/danger/trap.go`
- Create: `internal/game/danger/trap_test.go`

### Steps

- [ ] Create `internal/game/danger/trap.go`:

```go
package danger

// Roller is a source of random integers.
// Roll(max) returns a value in [0, max).
type Roller interface {
	Roll(max int) int
}

// defaultTrapPcts maps danger level to [roomPct, coverPct].
var defaultTrapPcts = map[DangerLevel][2]int{
	Safe:      {0, 0},
	Sketchy:   {0, 15},
	Dangerous: {35, 50},
	AllOutWar: {60, 75},
}

// RollRoomTrap returns true if a room trap should trigger.
// override is nil to use the danger level default; non-nil uses the explicit value (including 0).
// Precondition: rng MUST NOT be nil.
func RollRoomTrap(level DangerLevel, override *int, rng Roller) bool {
	pct := defaultTrapPcts[level][0]
	if override != nil {
		pct = *override
	}
	if pct <= 0 {
		return false
	}
	return rng.Roll(100) < pct
}

// RollCoverTrap returns true if a cover trap should trigger.
// override is nil to use the danger level default; non-nil uses the explicit value (including 0).
// Precondition: rng MUST NOT be nil.
func RollCoverTrap(level DangerLevel, override *int, rng Roller) bool {
	pct := defaultTrapPcts[level][1]
	if override != nil {
		pct = *override
	}
	if pct <= 0 {
		return false
	}
	return rng.Roll(100) < pct
}
```

- [ ] Create `internal/game/danger/trap_test.go` with unit and property-based tests:

```go
package danger_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

// alwaysRoller returns a fixed value for all Roll calls.
type alwaysRoller struct{ val int }

func (r alwaysRoller) Roll(_ int) int { return r.val }

func TestRollRoomTrap_SafeAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		if danger.RollRoomTrap(danger.Safe, nil, rng) {
			t.Fatal("RollRoomTrap(Safe) returned true; want false always")
		}
	})
}

func TestRollCoverTrap_SafeAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		if danger.RollCoverTrap(danger.Safe, nil, rng) {
			t.Fatal("RollCoverTrap(Safe) returned true; want false always")
		}
	})
}

func TestRollRoomTrap_ZeroRollTriggersWhenPctPositive(t *testing.T) {
	// Roll(100)=0 < any positive pct → true
	rng := alwaysRoller{val: 0}
	levels := []danger.DangerLevel{danger.Dangerous, danger.AllOutWar}
	for _, lvl := range levels {
		if !danger.RollRoomTrap(lvl, nil, rng) {
			t.Errorf("RollRoomTrap(%q, nil, roll=0): want true (pct>0), got false", lvl)
		}
	}
}

func TestRollCoverTrap_ZeroRollTriggersWhenPctPositive(t *testing.T) {
	rng := alwaysRoller{val: 0}
	levels := []danger.DangerLevel{danger.Sketchy, danger.Dangerous, danger.AllOutWar}
	for _, lvl := range levels {
		if !danger.RollCoverTrap(lvl, nil, rng) {
			t.Errorf("RollCoverTrap(%q, nil, roll=0): want true (pct>0), got false", lvl)
		}
	}
}

func TestRollRoomTrap_HighRollNeverTriggers(t *testing.T) {
	// Roll(100)=99 >= any pct < 100 → false
	rng := alwaysRoller{val: 99}
	levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
	for _, lvl := range levels {
		if danger.RollRoomTrap(lvl, nil, rng) {
			t.Errorf("RollRoomTrap(%q, nil, roll=99): want false (no pct==100), got true", lvl)
		}
	}
}

func TestRollRoomTrap_OverrideZeroAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		override := 0
		levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
		lvl := rapid.SampledFrom(levels).Draw(t, "level")
		if danger.RollRoomTrap(lvl, &override, rng) {
			t.Fatalf("RollRoomTrap(%q, &0, roll=%d): want false always, got true", lvl, roll)
		}
	})
}

func TestRollCoverTrap_OverrideZeroAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		override := 0
		levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
		lvl := rapid.SampledFrom(levels).Draw(t, "level")
		if danger.RollCoverTrap(lvl, &override, rng) {
			t.Fatalf("RollCoverTrap(%q, &0, roll=%d): want false always, got true", lvl, roll)
		}
	})
}

func TestRollRoomTrap_OverrideNonNilUsed(t *testing.T) {
	// Override of 50: roll=49 → true, roll=50 → false
	rng49 := alwaysRoller{val: 49}
	rng50 := alwaysRoller{val: 50}
	override := 50
	// Use Safe level (default 0%) to confirm override is used
	if !danger.RollRoomTrap(danger.Safe, &override, rng49) {
		t.Error("RollRoomTrap(Safe, &50, roll=49): want true (override used)")
	}
	if danger.RollRoomTrap(danger.Safe, &override, rng50) {
		t.Error("RollRoomTrap(Safe, &50, roll=50): want false")
	}
}
```

- [ ] Run tests (must pass):

```
go test ./internal/game/danger/... -v -run TestRoll
```

Expected output: all trap tests PASS.

- [ ] Create `internal/game/danger/dice_adapter.go`:

```go
package danger

import "math/rand"

// RandRoller wraps math/rand as a Roller.
// Use danger.RandRoller{} wherever a production Roller is required.
// This avoids importing the internal dice package from within the danger package.
type RandRoller struct{}

func (RandRoller) Roll(max int) int { return rand.Intn(max) }
```

- [ ] Commit:

```
git add internal/game/danger/trap.go internal/game/danger/trap_test.go internal/game/danger/dice_adapter.go
git commit -m "feat(danger): add RollRoomTrap, RollCoverTrap, and RandRoller adapter"
```

---

## Task 4: World model — DangerLevel + trap fields

**Files:**
- Modify: `internal/game/world/model.go`
- Create: `internal/game/world/model_danger_test.go`

### Steps

- [ ] In `internal/game/world/model.go`, add to the `Zone` struct (after `ScriptInstructionLimit`):

```go
DangerLevel     string `yaml:"danger_level"`
RoomTrapChance  *int   `yaml:"room_trap_chance,omitempty"`
CoverTrapChance *int   `yaml:"cover_trap_chance,omitempty"`
```

- [ ] In the `Room` struct (after `Terrain`), add:

```go
DangerLevel     string `yaml:"danger_level,omitempty"`
RoomTrapChance  *int   `yaml:"room_trap_chance,omitempty"`
CoverTrapChance *int   `yaml:"cover_trap_chance,omitempty"`
```

- [ ] In the `RoomEquipmentConfig` struct (after `CoverHP`), add:

```go
CoverTier string `yaml:"cover_tier,omitempty"`
```

- [ ] Create `internal/game/world/model_danger_test.go`:

```go
package world_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/world"
)

func TestZoneDangerLevelYAMLRoundTrip(t *testing.T) {
	trapChance := 25
	input := `
id: test_zone
name: Test Zone
description: A test zone
danger_level: dangerous
room_trap_chance: 25
rooms: []
`
	var z world.Zone
	if err := yaml.Unmarshal([]byte(input), &z); err != nil {
		t.Fatalf("unmarshal Zone: %v", err)
	}
	if z.DangerLevel != "dangerous" {
		t.Errorf("Zone.DangerLevel = %q; want %q", z.DangerLevel, "dangerous")
	}
	if z.RoomTrapChance == nil {
		t.Fatal("Zone.RoomTrapChance is nil; want non-nil")
	}
	if *z.RoomTrapChance != trapChance {
		t.Errorf("Zone.RoomTrapChance = %d; want %d", *z.RoomTrapChance, trapChance)
	}

	out, err := yaml.Marshal(z)
	if err != nil {
		t.Fatalf("marshal Zone: %v", err)
	}
	var z2 world.Zone
	if err := yaml.Unmarshal(out, &z2); err != nil {
		t.Fatalf("re-unmarshal Zone: %v", err)
	}
	if z2.DangerLevel != "dangerous" {
		t.Errorf("round-trip Zone.DangerLevel = %q; want %q", z2.DangerLevel, "dangerous")
	}
}

func TestRoomDangerLevelOverride(t *testing.T) {
	input := `
id: test_room
zone_id: test_zone
title: Test Room
description: A test room
danger_level: safe
`
	var r world.Room
	if err := yaml.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal Room: %v", err)
	}
	if r.DangerLevel != "safe" {
		t.Errorf("Room.DangerLevel = %q; want %q", r.DangerLevel, "safe")
	}
}

func TestRoomEquipmentConfigCoverTier(t *testing.T) {
	input := `
item_id: barrel
description: A heavy barrel
cover_tier: heavy
`
	var rec world.RoomEquipmentConfig
	if err := yaml.Unmarshal([]byte(input), &rec); err != nil {
		t.Fatalf("unmarshal RoomEquipmentConfig: %v", err)
	}
	if rec.CoverTier != "heavy" {
		t.Errorf("RoomEquipmentConfig.CoverTier = %q; want %q", rec.CoverTier, "heavy")
	}
}
```

- [ ] Run tests (must pass):

```
go test ./internal/game/world/... -v -run TestZoneDangerLevelYAMLRoundTrip
go test ./internal/game/world/... -v -run TestRoomDangerLevelOverride
go test ./internal/game/world/... -v -run TestRoomEquipmentConfigCoverTier
```

Expected output: all 3 tests PASS.

- [ ] Commit:

```
git add internal/game/world/model.go internal/game/world/model_danger_test.go
git commit -m "feat(world): add DangerLevel, trap override fields, and CoverTier to world model"
```

---

## Task 5: Session model — WantedLevel fields

**Files:**
- Modify: `internal/game/session/manager.go`

### Steps

- [ ] Locate the `PlayerSession` struct in `internal/game/session/manager.go`. Add the following three fields after `LastViolationDay` (or after `ZoneEffectCooldowns` if `LastViolationDay` does not exist yet):

```go
WantedLevel      map[string]int // zone_id → wanted level (0–4)
SafeViolations   map[string]int // zone_id → violation count in current WantedLevel cycle
LastViolationDay map[string]int // zone_id → in-game day of last violation
```

- [ ] Find the session creation/reset path (wherever `AutomapCache` is initialized) and add initialization for the three new maps:

```go
WantedLevel:      make(map[string]int),
SafeViolations:   make(map[string]int),
LastViolationDay: make(map[string]int),
```

- [ ] Add `AllPlayers() []*PlayerSession` method to the session manager (needed by Task 10):

```go
// AllPlayers returns a snapshot of all currently active player sessions.
// Postcondition: the returned slice MUST NOT be nil.
func (m *Manager) AllPlayers() []*PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*PlayerSession, 0, len(m.players))
	for _, sess := range m.players {
		result = append(result, sess)
	}
	return result
}
```

Note: Replace `m.players` and `m.mu` with the actual field names used in the manager struct.

Note (`LastViolationDay` persistence): `LastViolationDay` is intentionally in-memory only and is NOT persisted to the database. Since the decay hook (`decayWantedLevels`) only runs for online players, a player who logs out and back in will have `LastViolationDay` reset to its zero value (map entry absent, treated as day 0). On the next decay tick, `currentDay > 0` will be true, so the decay hook will treat the player as "no violation today" and decrement their `WantedLevel`. This behavior is acceptable and by design — a player who logs out forfeits the protection of their last-violation timestamp and may experience accelerated decay on re-login. This tradeoff is documented here and MUST NOT be treated as a bug.

- [ ] Run the existing session tests to confirm no regressions:

```
go test ./internal/game/session/... -v
```

Expected output: all existing tests PASS.

- [ ] Commit:

```
git add internal/game/session/manager.go
git commit -m "feat(session): add WantedLevel, SafeViolations, LastViolationDay fields and AllPlayers method"
```

---

## Task 6: WantedRepository — DB persistence

**Files:**
- Create: `migrations/031_character_wanted_levels.up.sql`
- Create: `migrations/031_character_wanted_levels.down.sql`
- Create: `internal/storage/postgres/wanted.go`
- Create: `internal/storage/postgres/wanted_test.go`

### Steps

- [ ] Create `migrations/031_character_wanted_levels.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS character_wanted_levels (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    zone_id      VARCHAR(64) NOT NULL,
    wanted_level INTEGER NOT NULL CHECK (wanted_level BETWEEN 1 AND 4),
    PRIMARY KEY (character_id, zone_id)
);
```

- [ ] Create `migrations/031_character_wanted_levels.down.sql`:

```sql
DROP TABLE IF EXISTS character_wanted_levels;
```

- [ ] Create `internal/storage/postgres/wanted.go`:

```go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WantedRepository persists per-player per-zone WantedLevel values.
type WantedRepository struct {
	db *pgxpool.Pool
}

// NewWantedRepository constructs a WantedRepository backed by the given pool.
func NewWantedRepository(db *pgxpool.Pool) *WantedRepository {
	return &WantedRepository{db: db}
}

// Load returns all non-zero wanted levels for the character.
// Rows with wanted_level=0 are never stored; absent rows imply level 0.
// Postcondition: the returned map MUST NOT be nil.
func (r *WantedRepository) Load(ctx context.Context, characterID int64) (map[string]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT zone_id, wanted_level FROM character_wanted_levels WHERE character_id = $1`,
		characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var zoneID string
		var level int
		if err := rows.Scan(&zoneID, &level); err != nil {
			return nil, err
		}
		result[zoneID] = level
	}
	return result, rows.Err()
}

// Upsert sets the wanted level for the character in the given zone.
// If level is 0, the row is deleted (level 0 means no record).
// Precondition: level MUST be in [0, 4].
// Postcondition: when level==0, no row exists for (characterID, zoneID).
func (r *WantedRepository) Upsert(ctx context.Context, characterID int64, zoneID string, level int) error {
	if level == 0 {
		_, err := r.db.Exec(ctx,
			`DELETE FROM character_wanted_levels WHERE character_id = $1 AND zone_id = $2`,
			characterID, zoneID)
		return err
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_wanted_levels (character_id, zone_id, wanted_level)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (character_id, zone_id) DO UPDATE SET wanted_level = EXCLUDED.wanted_level`,
		characterID, zoneID, level)
	return err
}
```

- [ ] Create `internal/storage/postgres/wanted_test.go` (integration test, uses the test DB established in `main_test.go`):

```go
package postgres_test

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestWantedRepository(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB available")
	}
	repo := postgres.NewWantedRepository(testDB)
	ctx := context.Background()

	// Use a character_id that exists in the test DB or insert a temp one.
	// Adjust characterID to match a valid character in the test database.
	const characterID int64 = 1
	const zoneID = "test_zone_wanted"

	// Precondition: clean state
	_ = repo.Upsert(ctx, characterID, zoneID, 0)

	// Load returns empty map for new character
	levels, err := repo.Load(ctx, characterID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(levels) != 0 {
		t.Errorf("Load (initial): want empty map, got %v", levels)
	}

	// Upsert level=2 stores it
	if err := repo.Upsert(ctx, characterID, zoneID, 2); err != nil {
		t.Fatalf("Upsert(2): %v", err)
	}
	levels, err = repo.Load(ctx, characterID)
	if err != nil {
		t.Fatalf("Load after Upsert(2): %v", err)
	}
	if levels[zoneID] != 2 {
		t.Errorf("Load after Upsert(2): want {%q:2}, got %v", zoneID, levels)
	}

	// Upsert level=0 deletes the row
	if err := repo.Upsert(ctx, characterID, zoneID, 0); err != nil {
		t.Fatalf("Upsert(0): %v", err)
	}
	levels, err = repo.Load(ctx, characterID)
	if err != nil {
		t.Fatalf("Load after Upsert(0): %v", err)
	}
	if len(levels) != 0 {
		t.Errorf("Load after Upsert(0): want empty map, got %v", levels)
	}
}
```

- [ ] Wire `WantedRepository` into `GameServiceServer` in `internal/gameserver/grpc_service.go`. Add the following field to the `GameServiceServer` struct:

```go
wantedRepo *postgres.WantedRepository
```

Add the parameter to `NewGameServiceServer` constructor (in addition to any existing parameters):

```go
func NewGameServiceServer(
    // ... existing parameters ...
    wantedRepo *postgres.WantedRepository,
    // ... remaining parameters ...
) *GameServiceServer {
```

And assign it in the constructor body:

```go
s.wantedRepo = wantedRepo
```

Note: The `WantedSaver` interface (defined in `enforcement.go`) is satisfied by `*postgres.WantedRepository` because it has an `Upsert(ctx, characterID, zoneID, level)` method. The server struct field uses the concrete type; the `enforcement.go` functions accept the `WantedSaver` interface. Pass `s.wantedRepo` (which implicitly satisfies `WantedSaver`) when calling `CheckSafeViolation` and `StartWantedDecay`.

- [ ] Run the migration (apply to dev DB), then run the test:

```
go test ./internal/storage/postgres/... -run TestWantedRepository -v
```

Expected output: `TestWantedRepository` PASS.

- [ ] Commit:

```
git add migrations/031_character_wanted_levels.up.sql migrations/031_character_wanted_levels.down.sql internal/storage/postgres/wanted.go internal/storage/postgres/wanted_test.go
git commit -m "feat(storage): add WantedRepository and migration 031"
```

---

## Task 7: Proto — danger_level on MapTile

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go` (via `make proto`)

### Steps

- [ ] In `api/proto/game/v1/game.proto`, locate the `MapTile` message and add field 7:

```proto
message MapTile {
  string room_id     = 1;
  string room_name   = 2;
  int32  x           = 3;
  int32  y           = 4;
  bool   current     = 5;
  repeated string exits = 6;
  string danger_level = 7;
}
```

- [ ] Regenerate the Go protobuf bindings:

```
make proto
```

- [ ] Verify the generated file contains `DangerLevel`:

```
grep -n "DangerLevel" internal/gameserver/gamev1/game.pb.go
```

Expected output: at least one line showing `DangerLevel string`.

- [ ] Confirm compilation:

```
go build ./...
```

Expected output: no errors.

- [ ] Commit:

```
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat(proto): add danger_level field to MapTile"
```

---

## Task 8: InitiateGuardCombat on CombatHandler

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_guard_test.go`

### Steps

- [ ] Write the failing test first. Create `internal/gameserver/combat_handler_guard_test.go`.

Note: All existing combat handler test files use `package gameserver` (not `package gameserver_test`), giving access to internal helpers `makeCombatHandler`, `spawnTestNPC`, and `addTestPlayerNamed`. This test MUST use the same package.

Note: `npc.Instance` does NOT have a `Tags` field — it has a `Type` field (string). The `InitiateGuardCombat` implementation MUST use `n.Type == "guard"` to identify guard NPCs, NOT `n.Tags["guard"]`. Similarly, the NPC instance identifier field is `ID`, NOT `InstanceID`.

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestInitiateGuardCombat_NoGuards_NoOp(t *testing.T) {
	broadcastCalled := false
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-guard-1"
	const playerUID = "player-guard-1"

	// Spawn a non-guard NPC (Type is empty, not "guard")
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, playerUID, roomID)

	h.InitiateGuardCombat(playerUID, "zone-1", 2)

	if broadcastCalled {
		t.Error("broadcastFn was called when no guards present; want no-op")
	}
}

func TestInitiateGuardCombat_WithGuards_BroadcastsAndAttacks(t *testing.T) {
	var broadcastedRoomID string
	var broadcastedEvents []*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		broadcastedRoomID = roomID
		broadcastedEvents = events
	}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-guard-2"
	const playerUID = "player-guard-2"

	// Spawn a guard NPC (Type = "guard")
	guardTmpl := &npc.Template{
		ID:    "guard_tmpl",
		Name:  "Guard",
		Type:  "guard",
		Level: 1, MaxHP: 20, AC: 14, Perception: 3,
	}
	_, err := h.npcMgr.Spawn(guardTmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn guard NPC: %v", err)
	}
	addTestPlayer(t, h.sessions, playerUID, roomID)

	h.InitiateGuardCombat(playerUID, "zone-1", 2)

	if broadcastedRoomID != roomID {
		t.Errorf("broadcastFn roomID = %q; want %q", broadcastedRoomID, roomID)
	}
	if len(broadcastedEvents) == 0 {
		t.Error("broadcastFn events is empty; want at least one combat event")
	}
	if broadcastedEvents[0].Narrative == "" {
		t.Error("broadcast narrative is empty; want non-empty message")
	}
}
```

Note: `addTestPlayer` may not yet exist as a helper in the same package. If `addTestPlayerNamed` is the helper (from `combat_handler_aid_test.go`), use it with a placeholder char name. Adjust the helper call accordingly at implementation time.

- [ ] Add `InitiateGuardCombat` to `internal/gameserver/combat_handler.go`:

```go
// InitiateGuardCombat finds guard NPCs in the player's current room and starts
// combat against the player. wantedLevel distinguishes detain (2) from kill (3-4).
// If no guard NPCs are present in the room, this is a no-op.
// Precondition: uid MUST be a valid player UID; wantedLevel MUST be in [2, 4].
func (h *CombatHandler) InitiateGuardCombat(uid, zoneID string, wantedLevel int) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	// Use InstancesInRoom — npc.Instance identifies guards via the Type field (not a Tags map).
	npcs := h.npcMgr.InstancesInRoom(sess.RoomID)
	var guardIDs []string
	for _, n := range npcs {
		if n.Type == "guard" {
			guardIDs = append(guardIDs, n.ID)
		}
	}
	if len(guardIDs) == 0 {
		return
	}
	var narrative string
	if wantedLevel >= 3 {
		narrative = "The guards attack on sight!"
	} else {
		narrative = "Guards shout: Drop your weapon and surrender!"
	}
	h.broadcastFn(sess.RoomID, []*gamev1.CombatEvent{
		{Type: gamev1.CombatEventType_COMBAT_EVENT_TYPE_MESSAGE, Narrative: narrative},
	})
	for _, guardID := range guardIDs {
		_, _ = h.Attack(guardID, uid)
	}
}
```

Note: `npc.Instance` uses field `ID` (not `InstanceID`) and `Type` (not `Tags`). The method on `npc.Manager` is `InstancesInRoom` (not `NPCsInRoom`). These names are confirmed from the actual codebase.

- [ ] Run guard combat tests (must pass after implementing stubs):

```
go test ./internal/gameserver/... -run TestInitiateGuardCombat -v
```

Expected output: both guard tests PASS.

- [ ] Run full gameserver tests:

```
go test ./internal/gameserver/... 2>&1 | tail -20
```

Expected output: all tests PASS, no failures.

- [ ] Commit:

```
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_guard_test.go
git commit -m "feat(combat): add InitiateGuardCombat to CombatHandler"
```

---

## Task 9: enforcement.go — CheckSafeViolation

**Files:**
- Create: `internal/gameserver/enforcement.go`
- Create: `internal/gameserver/enforcement_test.go`

### Steps

- [ ] Write the failing tests first. Create `internal/gameserver/enforcement_test.go`.

Note: Use `package gameserver` (same-package, not `gameserver_test`) to access the unexported `CheckSafeViolation` function directly without a package qualifier. This is consistent with all other test files in this directory which also use `package gameserver`.

```go
package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// mockWantedSaver records Upsert calls without hitting a DB.
type mockWantedSaver struct {
	upserted map[string]int // zoneID → level
}

func (m *mockWantedSaver) Upsert(_ context.Context, _ int64, zoneID string, level int) error {
	if m.upserted == nil {
		m.upserted = make(map[string]int)
	}
	m.upserted[zoneID] = level
	return nil
}

func newTestSession() *session.PlayerSession {
	return &session.PlayerSession{
		UID:             "test-uid",
		CharacterID:     42,
		WantedLevel:     make(map[string]int),
		SafeViolations:  make(map[string]int),
		LastViolationDay: make(map[string]int),
	}
}

func TestCheckSafeViolation_FirstViolation_Warning(t *testing.T) {
	sess := newTestSession()
	saver := &mockWantedSaver{}
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Safe), "", 5, nil, saver, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 warning event, got %d", len(events))
	}
	if events[0].Type != gamev1.CombatEventType_COMBAT_EVENT_TYPE_MESSAGE {
		t.Errorf("event type = %v; want MESSAGE", events[0].Type)
	}
	if sess.SafeViolations[zoneID] != 1 {
		t.Errorf("SafeViolations[%q] = %d; want 1", zoneID, sess.SafeViolations[zoneID])
	}
	if len(saver.upserted) != 0 {
		t.Errorf("Upsert called on first violation; want no DB write")
	}
}

func TestCheckSafeViolation_SecondViolation_IncrementsWanted(t *testing.T) {
	sess := newTestSession()
	sess.SafeViolations["test_zone"] = 1 // simulate first violation already recorded
	saver := &mockWantedSaver{}
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Safe), "", 5, nil, saver, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("want no events on second violation, got %d", len(events))
	}
	if sess.WantedLevel[zoneID] != 1 {
		t.Errorf("WantedLevel[%q] = %d; want 1", zoneID, sess.WantedLevel[zoneID])
	}
	if sess.SafeViolations[zoneID] != 0 {
		t.Errorf("SafeViolations[%q] = %d; want 0 (reset)", zoneID, sess.SafeViolations[zoneID])
	}
	if saver.upserted[zoneID] != 1 {
		t.Errorf("Upsert zoneID=%q level=%d; want 1", zoneID, saver.upserted[zoneID])
	}
}

func TestCheckSafeViolation_NonSafeRoom_NoOp(t *testing.T) {
	sess := newTestSession()
	saver := &mockWantedSaver{}
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Sketchy), "", 5, nil, saver, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("want no events for non-safe room, got %d", len(events))
	}
	if sess.SafeViolations[zoneID] != 0 {
		t.Errorf("SafeViolations should not change for non-safe room")
	}
}
```

- [ ] Create `internal/gameserver/enforcement.go`:

```go
package gameserver

import (
	"context"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// WantedSaver is the persistence interface for WantedLevel.
type WantedSaver interface {
	Upsert(ctx context.Context, characterID int64, zoneID string, level int) error
}

// CheckSafeViolation enforces safe-room combat rules for the given player.
// It is called when a player attempts to initiate combat.
// If the room is not Safe, this is a no-op (returns nil, nil).
// First violation: emits warning event and returns it (caller MUST NOT proceed with attack).
// Second+ violation: increments WantedLevel, resets SafeViolations, calls InitiateGuardCombat.
// Precondition: zoneLevel and roomLevel MUST be valid DangerLevel strings (may be empty for room).
// Precondition: currentDay MUST be the current in-game day number.
// Postcondition: returns non-nil events if a warning was issued; returns error on persistence failure.
func CheckSafeViolation(
	sess *session.PlayerSession,
	zoneID string,
	zoneLevel, roomLevel string,
	currentDay int,
	combatH *CombatHandler,
	wantedRepo WantedSaver,
	broadcastFn func(roomID string, events []*gamev1.CombatEvent),
) ([]*gamev1.CombatEvent, error) {
	level := danger.EffectiveDangerLevel(zoneLevel, roomLevel)
	if level != danger.Safe {
		return nil, nil
	}
	sess.SafeViolations[zoneID]++
	sess.LastViolationDay[zoneID] = currentDay
	if sess.SafeViolations[zoneID] == 1 {
		return []*gamev1.CombatEvent{
			{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_MESSAGE,
				Narrative: "Warning: combat is not permitted in this area.",
			},
		}, nil
	}
	// Second+ violation: increment WantedLevel (cap at 4)
	newLevel := sess.WantedLevel[zoneID] + 1
	if newLevel > 4 {
		newLevel = 4
	}
	sess.WantedLevel[zoneID] = newLevel
	sess.SafeViolations[zoneID] = 0
	if err := wantedRepo.Upsert(context.Background(), sess.CharacterID, zoneID, newLevel); err != nil {
		return nil, err
	}
	if combatH != nil {
		combatH.InitiateGuardCombat(sess.UID, zoneID, newLevel)
	}
	return nil, nil
}
```

- [ ] Run enforcement tests (must pass):

```
go test ./internal/gameserver/... -run TestCheckSafeViolation -v
```

Expected output: all 3 enforcement tests PASS.

- [ ] Commit:

```
git add internal/gameserver/enforcement.go internal/gameserver/enforcement_test.go
git commit -m "feat(gameserver): add CheckSafeViolation enforcement"
```

---

## Task 10: Calendar decay hook

**Files:**
- Create: `internal/gameserver/wanted_decay.go`

### Steps

- [ ] Create `internal/gameserver/wanted_decay.go`:

```go
package gameserver

import (
	"context"

	"github.com/cory-johannsen/mud/internal/game/session"
	"go.uber.org/zap"
)

// SessionLister provides read access to all online player sessions.
type SessionLister interface {
	AllPlayers() []*session.PlayerSession
}

// StartWantedDecay subscribes to the calendar and decrements WantedLevel
// for all online players once per in-game day.
// It MUST be called after GameServiceServer is fully initialized.
// Precondition: cal MUST NOT be nil.
// Returns a stop function; call it to unsubscribe and stop the goroutine.
func StartWantedDecay(cal *GameCalendar, sessions SessionLister, wantedRepo WantedSaver, logger *zap.Logger) func() {
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	var lastDecayDay int
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case dt := <-ch:
				if dt.Day == lastDecayDay {
					continue
				}
				lastDecayDay = dt.Day
				decayWantedLevels(sessions, wantedRepo, dt.Day, logger)
			case <-stop:
				cal.Unsubscribe(ch)
				return
			}
		}
	}()
	return func() { close(stop) }
}

func decayWantedLevels(sessions SessionLister, wantedRepo WantedSaver, currentDay int, logger *zap.Logger) {
	for _, sess := range sessions.AllPlayers() {
		for zoneID, level := range sess.WantedLevel {
			if level <= 0 {
				continue
			}
			if sess.LastViolationDay[zoneID] >= currentDay {
				continue // violated today or in the future; no decay
			}
			newLevel := level - 1
			sess.WantedLevel[zoneID] = newLevel
			if err := wantedRepo.Upsert(context.Background(), sess.CharacterID, zoneID, newLevel); err != nil {
				logger.Warn("failed to persist wanted decay",
					zap.String("uid", sess.UID),
					zap.String("zone", zoneID),
					zap.Error(err),
				)
			}
		}
	}
}
```

- [ ] Write tests for `decayWantedLevels` first. Create `internal/gameserver/wanted_decay_test.go`.

Note: `decayWantedLevels` is an unexported function, so these tests use `package gameserver` (same-package). They are pure function tests — no DB or calendar required.

```go
package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"go.uber.org/zap"
)

// mockDecaySaver records all Upsert calls.
type mockDecaySaver struct {
	calls []struct {
		characterID int64
		zoneID      string
		level       int
	}
}

func (m *mockDecaySaver) Upsert(_ context.Context, characterID int64, zoneID string, level int) error {
	m.calls = append(m.calls, struct {
		characterID int64
		zoneID      string
		level       int
	}{characterID, zoneID, level})
	return nil
}

// mockSessionLister returns a fixed list of sessions.
type mockSessionLister struct {
	players []*session.PlayerSession
}

func (m *mockSessionLister) AllPlayers() []*session.PlayerSession {
	return m.players
}

// TestDecayWantedLevels_DecaysWhenNoViolationToday verifies that a player
// with WantedLevel=2 and LastViolationDay < currentDay has their level decremented.
func TestDecayWantedLevels_DecaysWhenNoViolationToday(t *testing.T) {
	const zoneID = "zone-decay-1"
	sess := &session.PlayerSession{
		UID:              "uid-decay-1",
		CharacterID:      10,
		WantedLevel:      map[string]int{zoneID: 2},
		SafeViolations:   make(map[string]int),
		LastViolationDay: map[string]int{zoneID: 3}, // violated on day 3
	}
	lister := &mockSessionLister{players: []*session.PlayerSession{sess}}
	saver := &mockDecaySaver{}
	logger := zap.NewNop()

	decayWantedLevels(lister, saver, 4, logger) // currentDay=4 > lastViolationDay=3

	if sess.WantedLevel[zoneID] != 1 {
		t.Errorf("WantedLevel[%q] = %d; want 1 (decremented)", zoneID, sess.WantedLevel[zoneID])
	}
	if len(saver.calls) != 1 {
		t.Fatalf("Upsert called %d times; want 1", len(saver.calls))
	}
	if saver.calls[0].level != 1 {
		t.Errorf("Upsert level = %d; want 1", saver.calls[0].level)
	}
}

// TestDecayWantedLevels_NoDecayWhenViolatedToday verifies that a player
// with LastViolationDay == currentDay does NOT have their level decremented.
func TestDecayWantedLevels_NoDecayWhenViolatedToday(t *testing.T) {
	const zoneID = "zone-decay-2"
	sess := &session.PlayerSession{
		UID:              "uid-decay-2",
		CharacterID:      11,
		WantedLevel:      map[string]int{zoneID: 2},
		SafeViolations:   make(map[string]int),
		LastViolationDay: map[string]int{zoneID: 5}, // violated on day 5 (same as currentDay)
	}
	lister := &mockSessionLister{players: []*session.PlayerSession{sess}}
	saver := &mockDecaySaver{}
	logger := zap.NewNop()

	decayWantedLevels(lister, saver, 5, logger) // currentDay=5 == lastViolationDay=5

	if sess.WantedLevel[zoneID] != 2 {
		t.Errorf("WantedLevel[%q] = %d; want 2 (no decay)", zoneID, sess.WantedLevel[zoneID])
	}
	if len(saver.calls) != 0 {
		t.Errorf("Upsert called %d times; want 0 (no decay)", len(saver.calls))
	}
}
```

- [ ] Run the decay tests (must fail before implementation, pass after):

```
go test ./internal/gameserver/... -run TestDecayWantedLevels -v
```

Expected output after implementation: both tests PASS.

- [ ] Verify the code compiles (no test file required for this task; the enforcement tests already exercise the WantedSaver interface):

```
go build ./internal/gameserver/...
```

Expected output: no errors.

- [ ] Wire `StartWantedDecay` into the server startup in `internal/gameserver/grpc_service.go`.

Add a `stopWantedDecay func()` field to `GameServiceServer`:

```go
stopWantedDecay func()
```

In the `Serve` method (or equivalent server-start lifecycle function — NOT in `NewGameServiceServer` and NOT per-session), after the calendar subscription is confirmed available, add:

```go
if s.calendar != nil && s.wantedRepo != nil {
    s.stopWantedDecay = StartWantedDecay(s.calendar, s.sessions, s.wantedRepo, s.logger)
}
```

In the server shutdown/stop path (wherever other resources are cleaned up), call:

```go
if s.stopWantedDecay != nil {
    s.stopWantedDecay()
}
```

Note: `StartWantedDecay` subscribes to the calendar and spawns a goroutine. It MUST be called only once at server startup, not once per player session. The `s.sessions` field must implement `SessionLister` — this is guaranteed by Task 5 which adds the `AllPlayers()` method to the session manager. Verify that `s.sessions` is assigned as `*session.Manager` (not an interface) so the compiler can confirm interface satisfaction.

- [ ] In the session-start path (in `handleLogin` or wherever `AutomapCache` is loaded), load WantedLevel from the DB immediately after the session is created:

```go
wantedLevels, err := s.wantedRepo.Load(ctx, sess.CharacterID)
if err != nil {
    s.logger.Warnw("failed to load wanted levels", "characterID", sess.CharacterID, "err", err)
} else {
    sess.WantedLevel = wantedLevels
}
```

Note: If `wantedRepo` is nil (e.g., in tests), guard with `if s.wantedRepo != nil`. Adjust `sess.CharacterID` to the actual field name on the session struct.

- [ ] Commit:

```
git add internal/gameserver/wanted_decay.go internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): add calendar-driven WantedLevel decay"
```

---

## Task 11: Wire enforcement into handleMove + handleMap

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

Note (TDD): Before modifying `grpc_service.go`, write or extend integration tests (e.g., in `grpc_service_move_test.go` or a new `grpc_service_danger_test.go`) that assert the expected behavior: trap events on `handleMove`, `DangerLevel` populated on `MapTile`, and `CheckSafeViolation` blocking attack. Verify those tests fail, then implement.

### Steps

- [ ] In `handleMap` (~line 4298), inside the discovered rooms loop when building each `MapTile`, add the `DangerLevel` field. Locate the tile construction and add:

```go
// Compute effective danger level for this room
effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, r.DangerLevel)
tiles = append(tiles, &gamev1.MapTile{
    RoomId:      r.ID,
    RoomName:    r.Title,
    X:           int32(r.MapX),
    Y:           int32(r.MapY),
    Current:     r.ID == sess.RoomID,
    Exits:       exits,
    DangerLevel: string(effectiveLevel),
})
```

Note: `zone` must be in scope when building each tile. If the zone is not currently fetched, add `zone, _ := s.world.GetZone(r.ZoneID)` before the tile construction.

- [ ] In `handleMove` (~line 1500), after the player enters the new room (after `s.automapRepo.Insert`), add the room trap roll:

```go
newZone, zErr := s.world.GetZone(newRoom.ZoneID)
if zErr == nil {
    effectiveLevel := danger.EffectiveDangerLevel(newZone.DangerLevel, newRoom.DangerLevel)
    if danger.RollRoomTrap(effectiveLevel, newRoom.RoomTrapChance, s.dice) {
        s.broadcastToPlayer(uid, messageEvent("You trigger a trap!"))
        s.logger.Infow("room trap triggered", "uid", uid, "room", newRoom.ID)
    }
}
```

Note: Verify that `s.dice` implements `danger.Roller` (has `Roll(max int) int`). If the dice type uses a different signature, create a thin adapter type in `internal/gameserver/grpc_service.go`:

```go
type diceRoller struct{ d DiceRoller }
func (dr diceRoller) Roll(max int) int { return dr.d.Roll(max) }
```

Replace `s.dice` with `diceRoller{s.dice}` in the `RollRoomTrap` call if an adapter is needed.

- [ ] In `handleMove`, after the trap roll, add the WantedLevel guard check:

```go
wantedLevel := sess.WantedLevel[newRoom.ZoneID]
if wantedLevel >= 2 {
    s.combatH.InitiateGuardCombat(uid, newRoom.ZoneID, wantedLevel)
}
```

- [ ] Find the attack initiation handler (`handleAttack` or equivalent). Add safe-room enforcement at the top of the player-initiated attack path. The logic MUST follow this order:

  1. Call `CheckSafeViolation` first. If it returns a warning event (first violation in a Safe room), return it immediately and block the attack. `CheckSafeViolation` also handles the second-violation WantedLevel increment and guard initiation internally.
  2. After `CheckSafeViolation` returns nil events, compute `effectiveLevel`. If `CanInitiateCombat(effectiveLevel, "player")` returns false (i.e., the room is not Safe but still blocks combat — currently only Safe rooms do this, but this path is future-proof), return a blocking message.

```go
// Enforce safe-room combat rules (first/second violation flow)
zone, _ := s.world.GetZone(sess.RoomZoneID) // adjust field name as needed
room, _ := s.world.GetRoom(sess.RoomID)      // adjust as needed
currentDay := s.calendar.CurrentDay()         // adjust to actual calendar API
safeEvents, err := CheckSafeViolation(sess, zone.ID, zone.DangerLevel, room.DangerLevel, currentDay, s.combatH, s.wantedRepo, s.broadcastFn)
if err != nil {
    return nil, err
}
if len(safeEvents) > 0 {
    // First violation warning — block the attack
    return &gamev1.AttackResponse{Events: safeEvents}, nil
}
// Non-safe-violation check: block combat in any room where CanInitiateCombat returns false
// (CheckSafeViolation already handled the Safe case above; this covers future level types)
effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, room.DangerLevel)
if !danger.CanInitiateCombat(effectiveLevel, "player") {
    return &gamev1.AttackResponse{Events: []*gamev1.CombatEvent{
        {Type: gamev1.CombatEventType_COMBAT_EVENT_TYPE_MESSAGE, Narrative: "You cannot initiate combat here."},
    }}, nil
}
```

Note: Adjust field names (`sess.RoomZoneID`, `s.broadcastFn`, etc.) to match actual struct fields. If `GameCalendar` does not expose `CurrentDay()`, read the day from the most recently received `GameDateTime` stored on the server struct.

Note: `danger.RandRoller{}` is the production Roller to pass to `RollRoomTrap` (from Task 3). Do NOT pass a dice package type directly into the danger package. Use `danger.RandRoller{}` or a thin local adapter struct.

- [ ] Build to confirm no compile errors:

```
go build ./...
```

- [ ] Run full test suite:

```
go test ./... 2>&1 | tail -20
```

Expected output: all tests PASS.

- [ ] Commit:

```
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): wire danger enforcement into handleMove and handleMap"
```

---

## Task 12: Map color coding in RenderMap

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Create: `internal/frontend/handlers/text_renderer_danger_test.go`

### Steps

- [ ] Write the failing test first. Create `internal/frontend/handlers/text_renderer_danger_test.go`.

Note: `DangerColor` is exported (capital D) so it is callable from `package handlers_test`. The test uses `package handlers_test` which is correct for an exported function test.

```go
package handlers_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
)

func TestDangerColor(t *testing.T) {
	cases := []struct {
		level      string
		wantPrefix string
	}{
		{"safe", "\033[32m"},
		{"sketchy", "\033[33m"},
		{"dangerous", "\033[38;5;208m"},
		{"all_out_war", "\033[31m"},
		{"", "\033[37m"},
		{"unknown", "\033[37m"},
	}
	for _, c := range cases {
		got := handlers.DangerColor(c.level)
		if got != c.wantPrefix {
			t.Errorf("DangerColor(%q) = %q, want %q", c.level, got, c.wantPrefix)
		}
	}
}
```

Note: `TestRenderMap_ColorCodedCells` is omitted — a real ANSI output integration test is deferred to a follow-up task once the `MapResponse` shape is confirmed. The `TestDangerColor` unit test provides sufficient coverage of the color-mapping logic.

- [ ] Add `DangerColor` helper and `ansiReset` constant to `internal/frontend/handlers/text_renderer.go` (exported so the test can call it):

```go
// DangerColor returns the ANSI color escape for a danger level.
// Unexplored rooms (empty or unknown danger level) return light gray.
func DangerColor(dangerLevel string) string {
	switch dangerLevel {
	case "safe":
		return "\033[32m" // green
	case "sketchy":
		return "\033[33m" // yellow
	case "dangerous":
		return "\033[38;5;208m" // orange
	case "all_out_war":
		return "\033[31m" // red
	default:
		return "\033[37m" // light gray
	}
}

const ansiReset = "\033[0m"
```

- [ ] In `RenderMap()` (lines 1016-1292), locate the room cell rendering path for discovered rooms. Wrap the cell string with color:

```go
color := DangerColor(tile.DangerLevel)
cell := fmt.Sprintf("%s[%d]%s", color, tileNum, ansiReset)
```

Note: `tile` here refers to the `MapTile` proto struct. Adjust the variable name to match the actual loop variable in `RenderMap`. The `DangerLevel` field is the one added in Task 7.

- [ ] Run tests:

```
go test ./internal/frontend/handlers/... -run TestDangerColor -v
```

Expected output: all 6 `TestDangerColor` sub-tests PASS.

- [ ] Commit:

```
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_danger_test.go
git commit -m "feat(frontend): color-code map cells by danger level"
```

---

## Task 13: Zone YAML updates

**Files:**
- Modify all 16 zone YAML files in `content/zones/`

### Steps

- [ ] Add `danger_level` under the `zone:` top-level key for each zone. Required value per zone:

| Zone file | danger_level |
|-----------|-------------|
| `aloha` | `safe` |
| `battleground` | `all_out_war` |
| `beaverton` | `sketchy` |
| `downtown` | `sketchy` |
| `felony_flats` | `dangerous` |
| `hillsboro` | `sketchy` |
| `lake_oswego` | `safe` |
| `ne_portland` | `sketchy` |
| `pdx_international` | `sketchy` |
| `ross_island` | `dangerous` |
| `rustbucket_ridge` | `dangerous` |
| `sauvie_island` | `sketchy` |
| `se_industrial` | `dangerous` |
| `the_couve` | `sketchy` |
| `troutdale` | `sketchy` |
| `vantucky` | `dangerous` |

- [ ] For each zone, scan room titles and add `danger_level: safe` to any rooms that should be safe enclaves inside a dangerous zone (examples: hospital rooms, refugee shelters, trading posts within dangerous zones). Use best judgment based on room titles. If no rooms warrant a safe override, no room-level additions are needed for that zone.

- [ ] Run world loading tests:

```
go test ./internal/game/world/... -v
```

Expected output: all tests PASS.

- [ ] Run the full test suite:

```
go test ./... 2>&1 | tail -20
```

Expected output: all tests PASS.

- [ ] Commit:

```
git add content/zones/
git commit -m "content(zones): add danger_level to all 16 zone YAML files"
```

---

## Task 14: Architecture documentation

**Files:**
- Create: `docs/architecture/map-system.md`

### Steps

- [ ] Create `docs/architecture/map-system.md` with the following content:

```markdown
# Map System Architecture

## Overview

The map system tracks which rooms each player has explored and renders an ASCII automap grid
color-coded by danger level. Discovery is per-character and persisted to PostgreSQL.

---

## Database Schema

### `character_map_rooms`

```sql
CREATE TABLE IF NOT EXISTS character_map_rooms (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    zone_id      VARCHAR(64) NOT NULL,
    room_id      VARCHAR(64) NOT NULL,
    PRIMARY KEY (character_id, zone_id, room_id)
);
```

Writes use INSERT ... ON CONFLICT DO NOTHING (idempotent). There is no DELETE path;
rooms once explored remain explored permanently.

---

## AutomapRepository

**File:** `internal/storage/postgres/automap.go`

| Method | Signature | Description |
|--------|-----------|-------------|
| Insert | `Insert(ctx, characterID int64, zoneID, roomID string) error` | Idempotent insert of a discovered room. |
| LoadAll | `LoadAll(ctx, characterID int64) (map[string]map[string]bool, error)` | Returns all explored rooms keyed by zoneID → roomID → true. |

---

## PlayerSession.AutomapCache

**Type:** `map[string]map[string]bool` (zoneID → roomID → explored)

Populated at session load via `AutomapRepository.LoadAll`. Updated in memory on each move
before the DB write. The cache is the authoritative in-memory source for `handleMap`.

---

## MapTile Proto Fields

**Message:** `MapTile` in `api/proto/game/v1/game.proto`

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| room_id | 1 | string | Room identifier |
| room_name | 2 | string | Display name |
| x | 3 | int32 | Grid X coordinate |
| y | 4 | int32 | Grid Y coordinate |
| current | 5 | bool | True if this is the player's current room |
| exits | 6 | repeated string | Available exit directions |
| danger_level | 7 | string | Effective danger level string (safe/sketchy/dangerous/all_out_war) |

---

## Discovery Flow

```
handleMove
  └─ move player to new room
  └─ AutomapRepository.Insert(characterID, zoneID, roomID)   ← idempotent DB write
  └─ sess.AutomapCache[zoneID][roomID] = true                ← in-memory update
  └─ XP award for new discovery (if first time)
  └─ RollRoomTrap(effectiveLevel, room.RoomTrapChance, dice)  ← trap roll
  └─ InitiateGuardCombat if WantedLevel >= 2                  ← guard enforcement

handleMap
  └─ iterate sess.AutomapCache[zoneID]                        ← explored rooms only
  └─ for each explored room: build MapTile with DangerLevel
  └─ return MapResponse{Tiles: tiles}
```

---

## RenderMap Rendering Pipeline

**Function:** `RenderMap(resp *gamev1.MapResponse, width int) string`
**File:** `internal/frontend/handlers/text_renderer.go`

1. Build a coordinate grid from `resp.Tiles` (x, y → tile index).
2. Find bounding box (min/max x, y).
3. For each cell in the bounding box:
   - If the cell has a tile (explored): render `[N]` wrapped in ANSI color from `DangerColor(tile.DangerLevel)`.
   - If no tile: render empty space.
4. Mark the current room with a special indicator (e.g., `[*]`).
5. Append a legend showing the exit directions.

---

## Danger Level Color Coding

| Danger Level | Color | ANSI Escape |
|--------------|-------|-------------|
| `safe` | Green | `\033[32m` |
| `sketchy` | Yellow | `\033[33m` |
| `dangerous` | Orange | `\033[38;5;208m` |
| `all_out_war` | Red | `\033[31m` |
| unexplored / unknown | Light Gray | `\033[37m` |

Unexplored rooms (not in `AutomapCache`) are not rendered in the current implementation.
If future requirements call for rendering unexplored rooms (e.g., from zone metadata),
they MUST use the light gray color regardless of their actual danger level (REQ-DL-10).

---

## Explored vs. Unexplored

- A room is **explored** if it appears in `sess.AutomapCache[zoneID][roomID]`.
- Only explored rooms are included in `handleMap`'s tile list.
- Unexplored rooms are not rendered; they simply do not appear on the map.
- This invariant is enforced at the `handleMap` layer, not the renderer.
```

- [ ] Commit:

```
git add docs/architecture/map-system.md
git commit -m "docs: add map system architecture doc"
```

---

## Final Verification

- [ ] Run the full test suite — must pass 100%:

```
go test ./... 2>&1 | tail -20
```

Expected output: `ok` for every package, zero failures.

- [ ] Verify all zone YAMLs load without errors:

```
go test ./internal/game/world/... -v -run TestZone
```

- [ ] Verify proto generation is clean:

```
make proto && go build ./...
```

Expected output: no errors.
