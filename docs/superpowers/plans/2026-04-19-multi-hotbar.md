# Multi-Row Hotbar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow players to create and switch between up to 4 independent 10-slot hotbars, with UI controls and Ctrl+Up/Down keyboard shortcuts, persisted per character.

**Architecture:** Expand `PlayerSession.Hotbar [10]HotbarSlot` to `Hotbars [][10]HotbarSlot` + `ActiveHotbarIndex int`. Add a new DB migration with `hotbars TEXT` and `active_hotbar_idx` columns. Update the proto to carry multi-bar metadata. Add `create`/`switch` server actions. Update the React hotbar panel with controls and global key handler.

**Tech Stack:** Go (domain, persistence, gameserver), PostgreSQL (migration), protobuf (proto regeneration), React/TypeScript (HotbarPanel, GameContext)

---

## File Map

| Action | File | Purpose |
|---|---|---|
| Modify | `internal/config/config.go` | Add `HotbarConfig{MaxHotbars int}` to root `Config` |
| Modify | `internal/game/session/manager.go` | `Hotbar → Hotbars [][10]HotbarSlot`; add `ActiveHotbarIndex int` |
| Create | `migrations/065_multi_hotbar.up.sql` | Add `hotbars TEXT`, `active_hotbar_idx INTEGER` columns |
| Create | `migrations/065_multi_hotbar.down.sql` | Drop new columns |
| Modify | `internal/storage/postgres/character_hotbar.go` | Add `LoadHotbars`, `SaveHotbars`, `MarshalHotbars`, `UnmarshalHotbars`; keep `LoadHotbar`/`SaveHotbar` as internal legacy helpers |
| Modify | `internal/storage/postgres/character_hotbar_test.go` | Tests for new marshal/load/save functions |
| Modify | `internal/gameserver/grpc_service.go` | Update `CharacterSaver` interface; update load and event-push call sites |
| Modify | `internal/gameserver/grpc_service_hotbar.go` | Add `create`/`switch` actions; update all `sess.Hotbar` → `sess.Hotbars[sess.ActiveHotbarIndex]`; update `hotbarUpdateEvent` |
| Modify | `internal/gameserver/grpc_service_hotbar_test.go` | Tests for create/switch; update existing tests |
| Modify | `api/proto/game/v1/game.proto` | Add `hotbar_index` to `HotbarRequest`; add 3 fields to `HotbarUpdateEvent` |
| Modify | `cmd/webclient/ui/src/game/GameContext.tsx` | Add `activeHotbarIndex`, `hotbarCount`, `maxHotbars` to state |
| Modify | `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx` | Add ▲/▼ controls, indicator, "+ New Hotbar" button |

---

### Task 1: Add `HotbarConfig` to server config

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add `HotbarConfig` struct and wire it into `Config`**

In `internal/config/config.go`, locate the `Config` struct and add:

```go
type HotbarConfig struct {
	MaxHotbars int `mapstructure:"max_hotbars"`
}
```

Add to the `Config` struct:
```go
Hotbar HotbarConfig `mapstructure:"hotbar"`
```

In `setDefaults()` (or wherever other defaults are set), add:
```go
v.SetDefault("hotbar.max_hotbars", 4)
```

- [ ] **Step 2: Build to verify no compilation errors**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./internal/config/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add HotbarConfig.MaxHotbars (default 4) (#192)"
```

---

### Task 2: Update domain model — `PlayerSession`

**Files:**
- Modify: `internal/game/session/manager.go`

- [ ] **Step 1: Replace `Hotbar` with `Hotbars` + `ActiveHotbarIndex`**

In `internal/game/session/manager.go`, find the `PlayerSession` struct.

Replace:
```go
// Hotbar holds the player's 10 persistent hotbar slot assignments.
// Index 0 = key "1", Index 9 = key "0".
Hotbar [10]HotbarSlot
```

With:
```go
// Hotbars holds the player's persistent hotbar bars (up to MaxHotbars).
// Each bar has 10 slots. Index 0 = key "1", Index 9 = key "0".
// Always contains at least 1 bar.
Hotbars           [][10]HotbarSlot
// ActiveHotbarIndex is the 0-based index of the currently displayed bar.
ActiveHotbarIndex int
```

- [ ] **Step 2: Initialize with one empty bar in `NewPlayerSession` (or wherever sessions are created)**

Find where `PlayerSession` is initialized (look for `PlayerSession{` or `&PlayerSession{}`). Add initialization:

```go
Hotbars:           [][10]HotbarSlot{{}},
ActiveHotbarIndex: 0,
```

- [ ] **Step 3: Build to find all `sess.Hotbar` references that need updating**

```bash
mise exec -- go build ./... 2>&1 | grep "Hotbar"
```

Expected: compilation errors listing every file that still uses the old `Hotbar` field. Note these files — they are all updated in subsequent tasks.

- [ ] **Step 4: Commit the domain change (compilation will fail until other tasks complete)**

```bash
git add internal/game/session/manager.go
git commit -m "feat(session): expand Hotbar to Hotbars [][10]HotbarSlot + ActiveHotbarIndex (#192)"
```

---

### Task 3: Database migration

**Files:**
- Create: `migrations/065_multi_hotbar.up.sql`
- Create: `migrations/065_multi_hotbar.down.sql`

- [ ] **Step 1: Create the up migration**

```sql
-- migrations/065_multi_hotbar.up.sql
ALTER TABLE characters
  ADD COLUMN IF NOT EXISTS hotbars          TEXT,
  ADD COLUMN IF NOT EXISTS active_hotbar_idx INTEGER NOT NULL DEFAULT 0;
```

- [ ] **Step 2: Create the down migration**

```sql
-- migrations/065_multi_hotbar.down.sql
ALTER TABLE characters
  DROP COLUMN IF EXISTS hotbars,
  DROP COLUMN IF EXISTS active_hotbar_idx;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/065_multi_hotbar.up.sql migrations/065_multi_hotbar.down.sql
git commit -m "feat(db): add hotbars and active_hotbar_idx columns (#192)"
```

---

### Task 4: Persistence — `LoadHotbars` / `SaveHotbars`

**Files:**
- Modify: `internal/storage/postgres/character_hotbar.go`
- Modify: `internal/storage/postgres/character_hotbar_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/storage/postgres/character_hotbar_test.go`:

```go
// TestMarshalUnmarshalHotbars verifies round-trip marshal/unmarshal of multiple bars.
//
// Precondition: Two bars; bar 0 has slot 0 set to a command slot; bar 1 is empty.
// Postcondition: Unmarshal returns identical bars.
func TestMarshalUnmarshalHotbars(t *testing.T) {
	bars := [][10]session.HotbarSlot{
		{session.CommandSlot("look")},
		{},
	}
	data, err := MarshalHotbars(bars)
	if err != nil {
		t.Fatalf("MarshalHotbars: %v", err)
	}
	got, err := UnmarshalHotbars(data)
	if err != nil {
		t.Fatalf("UnmarshalHotbars: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 bars, got %d", len(got))
	}
	if got[0][0].Ref != "look" {
		t.Fatalf("bar 0 slot 0: expected 'look', got %q", got[0][0].Ref)
	}
}

// TestLoadHotbars_LegacyMigration verifies that LoadHotbars falls back to the
// legacy hotbar column when hotbars is NULL.
//
// Precondition: characters.hotbar has data; characters.hotbars is NULL.
// Postcondition: LoadHotbars returns single-bar result with legacy data in bar 0.
// NOTE: This is an integration test requiring a test DB. Skip if DB unavailable.
func TestLoadHotbars_LegacyMigration(t *testing.T) {
	// This test is marked for integration; skipped in unit mode.
	t.Skip("integration test: requires test database")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/storage/postgres/... -run "TestMarshalUnmarshalHotbars" -v
```

Expected: FAIL — `MarshalHotbars` does not exist.

- [ ] **Step 3: Implement `MarshalHotbars` and `UnmarshalHotbars`**

In `internal/storage/postgres/character_hotbar.go`, add after the existing marshal helpers:

```go
// MarshalHotbars JSON-encodes a slice of 10-slot bars.
// Returns nil if all bars are empty (stored as NULL).
//
// Precondition: bars must be non-nil; each bar has exactly 10 slots.
// Postcondition: Returns nil bytes for all-empty bars.
func MarshalHotbars(bars [][10]session.HotbarSlot) ([]byte, error) {
	allEmpty := true
	for _, bar := range bars {
		for _, sl := range bar {
			if !sl.IsEmpty() {
				allEmpty = false
				break
			}
		}
		if !allEmpty {
			break
		}
	}
	if allEmpty {
		return nil, nil
	}
	type barJSON = [10]hotbarSlotJSON
	out := make([]barJSON, len(bars))
	for bi, bar := range bars {
		for si, sl := range bar {
			out[bi][si] = hotbarSlotJSON{Kind: sl.Kind, Ref: sl.Ref}
		}
	}
	return json.Marshal(out)
}

// UnmarshalHotbars decodes a JSON byte slice into a slice of 10-slot bars.
//
// Precondition: data is valid JSON produced by MarshalHotbars.
// Postcondition: Returns decoded bars with exactly 10 slots per bar.
func UnmarshalHotbars(data []byte) ([][10]session.HotbarSlot, error) {
	type barJSON = [10]hotbarSlotJSON
	var raw []barJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	bars := make([][10]session.HotbarSlot, len(raw))
	for bi, bar := range raw {
		for si, sl := range bar {
			bars[bi][si] = session.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		}
	}
	return bars, nil
}
```

- [ ] **Step 4: Implement `LoadHotbars` and `SaveHotbars`**

```go
// LoadHotbars retrieves all hotbars and the active index for a character.
// Falls back to the legacy hotbar column if hotbars is NULL.
// Returns one empty bar with index 0 if both columns are NULL.
//
// Precondition: characterID > 0.
// Postcondition: Returns at least 1 bar; activeIdx is within [0, len(bars)-1].
func (r *CharacterRepository) LoadHotbars(ctx context.Context, characterID int64) ([][10]session.HotbarSlot, int, error) {
	if characterID <= 0 {
		return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: characterID must be > 0, got %d", characterID)
	}
	var hotbarsJSON, legacyHotbarJSON *string
	var activeIdx int
	err := r.db.QueryRowContext(ctx,
		`SELECT hotbars, hotbar, active_hotbar_idx FROM characters WHERE id = $1`, characterID,
	).Scan(&hotbarsJSON, &legacyHotbarJSON, &activeIdx)
	if err == sql.ErrNoRows {
		return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: %w", ErrCharacterNotFound)
	}
	if err != nil {
		return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: %w", err)
	}
	// Use new hotbars column if present.
	if hotbarsJSON != nil && *hotbarsJSON != "" {
		bars, err := UnmarshalHotbars([]byte(*hotbarsJSON))
		if err != nil {
			return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: unmarshal: %w", err)
		}
		if len(bars) == 0 {
			bars = [][10]session.HotbarSlot{{}}
		}
		if activeIdx >= len(bars) {
			activeIdx = 0
		}
		return bars, activeIdx, nil
	}
	// Legacy migration: unmarshal old hotbar column as bar 0.
	if legacyHotbarJSON != nil && *legacyHotbarJSON != "" {
		bar, err := UnmarshalHotbarSlots([]byte(*legacyHotbarJSON))
		if err != nil {
			return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: legacy unmarshal: %w", err)
		}
		return [][10]session.HotbarSlot{bar}, 0, nil
	}
	return [][10]session.HotbarSlot{{}}, 0, nil
}

// SaveHotbars persists all hotbars and the active index for a character.
// Stores NULL for hotbars if all bars are empty.
//
// Precondition: characterID > 0; bars non-nil; activeIdx in [0, len(bars)-1].
// Postcondition: characters.hotbars and active_hotbar_idx updated.
func (r *CharacterRepository) SaveHotbars(ctx context.Context, characterID int64, bars [][10]session.HotbarSlot, activeIdx int) error {
	if characterID <= 0 {
		return fmt.Errorf("SaveHotbars: characterID must be > 0, got %d", characterID)
	}
	data, err := MarshalHotbars(bars)
	if err != nil {
		return fmt.Errorf("SaveHotbars: marshal: %w", err)
	}
	var hotbarsValue interface{}
	if data != nil {
		hotbarsValue = string(data)
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE characters SET hotbars = $2, active_hotbar_idx = $3 WHERE id = $1`,
		characterID, hotbarsValue, activeIdx,
	)
	if err != nil {
		return fmt.Errorf("SaveHotbars: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("SaveHotbars: %w", ErrCharacterNotFound)
	}
	return nil
}
```

- [ ] **Step 5: Run persistence tests**

```bash
mise exec -- go test ./internal/storage/postgres/... -run "TestMarshalUnmarshalHotbars" -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage/postgres/character_hotbar.go internal/storage/postgres/character_hotbar_test.go
git commit -m "feat(db): add LoadHotbars/SaveHotbars with legacy migration (#192)"
```

---

### Task 5: Update `CharacterSaver` interface and call sites in gameserver

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Update `CharacterSaver` interface**

In `grpc_service.go` around line 98, find the `CharacterSaver` interface. Replace:
```go
SaveHotbar(ctx context.Context, characterID int64, slots [10]session.HotbarSlot) error
LoadHotbar(ctx context.Context, characterID int64) ([10]session.HotbarSlot, error)
```
With:
```go
SaveHotbars(ctx context.Context, characterID int64, bars [][10]session.HotbarSlot, activeIdx int) error
LoadHotbars(ctx context.Context, characterID int64) ([][10]session.HotbarSlot, int, error)
```

- [ ] **Step 2: Update the load call site (around line 1496)**

Replace:
```go
hotbarSlots, hotbarErr := s.charSaver.LoadHotbar(stream.Context(), characterID)
if hotbarErr != nil {
    s.logger.Warn("loading hotbar", zap.Int64("character_id", characterID), zap.Error(hotbarErr))
}
sess.Hotbar = hotbarSlots
```
With:
```go
hotbarBars, hotbarActiveIdx, hotbarErr := s.charSaver.LoadHotbars(stream.Context(), characterID)
if hotbarErr != nil {
    s.logger.Warn("loading hotbars", zap.Int64("character_id", characterID), zap.Error(hotbarErr))
}
sess.Hotbars = hotbarBars
sess.ActiveHotbarIndex = hotbarActiveIdx
```

- [ ] **Step 3: Add `maxHotbars int` field to `GameServiceServer`**

In the `GameServiceServer` struct, add:
```go
maxHotbars int
```

Wire it from config where the server is constructed (search for `GameServiceServer{` or `NewGameServiceServer`), adding:
```go
maxHotbars: cfg.Hotbar.MaxHotbars,
```
If `maxHotbars` is 0 (not configured), default to 4:
```go
if s.maxHotbars <= 0 {
    s.maxHotbars = 4
}
```

- [ ] **Step 4: Build to see remaining errors**

```bash
mise exec -- go build ./internal/gameserver/... 2>&1
```

Expected: errors in `grpc_service_hotbar.go` for `sess.Hotbar` references — fixed in next task.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): update CharacterSaver to LoadHotbars/SaveHotbars; wire maxHotbars (#192)"
```

---

### Task 6: Update proto — `HotbarRequest` and `HotbarUpdateEvent`

**Files:**
- Modify: `api/proto/game/v1/game.proto`

- [ ] **Step 1: Add fields to `HotbarRequest`**

Find `message HotbarRequest` and add field 9:
```protobuf
message HotbarRequest {
  string action = 1;
  int32  slot   = 2;
  string text   = 3;
  string kind   = 4;
  string ref    = 5;
  // ... existing fields 6-8 unchanged ...
  int32  hotbar_index = 9;  // 1-based target bar; 0 = current active (default)
}
```

- [ ] **Step 2: Add fields to `HotbarUpdateEvent`**

Find `message HotbarUpdateEvent` and add fields 2-4:
```protobuf
message HotbarUpdateEvent {
  repeated HotbarSlot slots               = 1;
  int32               active_hotbar_index = 2;
  int32               hotbar_count        = 3;
  int32               max_hotbars         = 4;
}
```

- [ ] **Step 3: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
mise exec -- make proto
```

Expected: regenerated Go and TypeScript proto files. If `make proto` doesn't exist, run:
```bash
mise exec -- protoc --go_out=. --go-grpc_out=. api/proto/game/v1/game.proto
```

- [ ] **Step 4: Build to verify generated code compiles**

```bash
mise exec -- go build ./...
```

Expected: no errors (new fields are additive).

- [ ] **Step 5: Commit**

```bash
git add api/proto/game/v1/game.proto
git add internal/gameserver/gamev1/
git add cmd/webclient/ui/src/proto/
git commit -m "feat(proto): add hotbar_index to HotbarRequest; add multi-bar fields to HotbarUpdateEvent (#192)"
```

---

### Task 7: Rewrite `grpc_service_hotbar.go` for multi-bar

**Files:**
- Modify: `internal/gameserver/grpc_service_hotbar.go`
- Modify: `internal/gameserver/grpc_service_hotbar_test.go`

- [ ] **Step 1: Write failing tests for `create` and `switch`**

Add to `internal/gameserver/grpc_service_hotbar_test.go`:

```go
// TestHandleHotbar_Create_AddsBar verifies "create" appends a new empty bar and switches to it.
//
// Precondition: Session with 1 bar; maxHotbars=4.
// Postcondition: sess.Hotbars has 2 bars; sess.ActiveHotbarIndex == 1.
func TestHandleHotbar_Create_AddsBar(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 4)
	// Initial state: 1 bar
	if len(sess.Hotbars) != 1 {
		t.Fatalf("expected 1 bar initially, got %d", len(sess.Hotbars))
	}
	ev, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "create"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = ev
	if len(sess.Hotbars) != 2 {
		t.Fatalf("expected 2 bars after create, got %d", len(sess.Hotbars))
	}
	if sess.ActiveHotbarIndex != 1 {
		t.Fatalf("expected ActiveHotbarIndex=1 after create, got %d", sess.ActiveHotbarIndex)
	}
}

// TestHandleHotbar_Create_EnforcesLimit verifies "create" returns an error message at max.
//
// Precondition: Session with maxHotbars bars already.
// Postcondition: No new bar added; MessageEvent returned with limit message.
func TestHandleHotbar_Create_EnforcesLimit(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 2)
	sess.Hotbars = [][10]session.HotbarSlot{{}, {}} // already at limit
	ev, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "create"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sess.Hotbars) != 2 {
		t.Fatalf("expected bars unchanged at limit, got %d", len(sess.Hotbars))
	}
	// ev should be a MessageEvent containing "limit"
	msg := extractMessage(ev)
	if !strings.Contains(msg, "limit") && !strings.Contains(msg, "Hotbar limit") {
		t.Fatalf("expected limit message, got %q", msg)
	}
}

// TestHandleHotbar_Switch_ChangesActiveIndex verifies "switch" updates ActiveHotbarIndex.
//
// Precondition: Session with 2 bars; ActiveHotbarIndex=0.
// Postcondition: ActiveHotbarIndex=1 after switch to hotbar_index=2.
func TestHandleHotbar_Switch_ChangesIndex(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 4)
	sess.Hotbars = [][10]session.HotbarSlot{{}, {}}
	sess.ActiveHotbarIndex = 0
	_, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ActiveHotbarIndex != 1 {
		t.Fatalf("expected ActiveHotbarIndex=1, got %d", sess.ActiveHotbarIndex)
	}
}

// TestHandleHotbar_Switch_OutOfRange returns error message and does not change index.
//
// Precondition: 1 bar; switch to hotbar_index=5.
// Postcondition: ActiveHotbarIndex unchanged; MessageEvent with "Invalid" returned.
func TestHandleHotbar_Switch_OutOfRange(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 4)
	_, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ActiveHotbarIndex != 0 {
		t.Fatalf("expected ActiveHotbarIndex unchanged at 0, got %d", sess.ActiveHotbarIndex)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestHandleHotbar_Create|TestHandleHotbar_Switch" -v
```

Expected: FAIL (new actions not yet implemented).

- [ ] **Step 3: Update `handleHotbar` — replace all `sess.Hotbar` references**

In `grpc_service_hotbar.go`, update every access of `sess.Hotbar` to `sess.Hotbars[sess.ActiveHotbarIndex]`, and update every `SaveHotbar` call to `SaveHotbars`:

```go
func (s *GameServiceServer) handleHotbar(uid string, req *gamev1.HotbarRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}

	switch req.Action {
	case "set":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		slot := buildHotbarSlot(req)
		if slot.IsEmpty() {
			return messageEvent("Nothing to set: provide text or kind+ref."), nil
		}
		sess.Hotbars[sess.ActiveHotbarIndex][idx] = slot
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return messageEvent(fmt.Sprintf("Slot %d set.", req.Slot)), nil

	case "clear":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		sess.Hotbars[sess.ActiveHotbarIndex][idx] = session.HotbarSlot{}
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return messageEvent(fmt.Sprintf("Slot %d cleared.", req.Slot)), nil

	case "show":
		activeBar := sess.Hotbars[sess.ActiveHotbarIndex]
		for i := 0; i < 10; i++ {
			slotNum := i + 1
			display := "---"
			if cmd := activeBar[i].ActivationCommand(); cmd != "" {
				display = cmd
			}
			s.pushMessageToUID(uid, fmt.Sprintf("[%d] %s", slotNum, display))
		}
		return nil, nil

	case "create":
		maxHotbars := s.maxHotbars
		if maxHotbars <= 0 {
			maxHotbars = 4
		}
		if len(sess.Hotbars) >= maxHotbars {
			return messageEvent(fmt.Sprintf("Hotbar limit reached (max %d).", maxHotbars)), nil
		}
		sess.Hotbars = append(sess.Hotbars, [10]session.HotbarSlot{})
		sess.ActiveHotbarIndex = len(sess.Hotbars) - 1
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return nil, nil

	case "switch":
		targetIdx := int(req.HotbarIndex) - 1 // HotbarIndex is 1-based
		if targetIdx < 0 || targetIdx >= len(sess.Hotbars) {
			return messageEvent("Invalid hotbar index."), nil
		}
		sess.ActiveHotbarIndex = targetIdx
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return nil, nil

	default:
		return messageEvent(fmt.Sprintf("Unknown hotbar action '%s'.", req.Action)), nil
	}
}
```

- [ ] **Step 4: Update `hotbarUpdateEvent` to include multi-bar metadata**

```go
func (s *GameServiceServer) hotbarUpdateEvent(sess *session.PlayerSession) *gamev1.ServerEvent {
	activeBar := sess.Hotbars[sess.ActiveHotbarIndex]
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, sl := range activeBar {
		ps := &gamev1.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		if !sl.IsEmpty() {
			ps.DisplayName, ps.Description = s.resolveHotbarSlotDisplay(sl)
			ps.UsesRemaining, ps.MaxUses, ps.RechargeCondition = s.resolveHotbarSlotUseState(sess, sl)
			ps.ApCost, ps.DamageSummary = s.resolveHotbarSlotTechInfo(sl)
		}
		protoSlots[i] = ps
	}
	maxHotbars := int32(s.maxHotbars)
	if maxHotbars <= 0 {
		maxHotbars = 4
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{
				Slots:              protoSlots,
				ActiveHotbarIndex:  int32(sess.ActiveHotbarIndex + 1), // 1-based for client
				HotbarCount:        int32(len(sess.Hotbars)),
				MaxHotbars:         maxHotbars,
			},
		},
	}
}
```

- [ ] **Step 5: Update existing tests** 

In the existing tests, replace `sess.Hotbar[N]` with `sess.Hotbars[0][N]` and `sess.Hotbar` with `sess.Hotbars[0]`.

- [ ] **Step 6: Run hotbar tests**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestHandleHotbar" -v
```

Expected: all PASS

- [ ] **Step 7: Run full test suite**

```bash
mise exec -- go test ./... -timeout 180s
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/grpc_service_hotbar.go internal/gameserver/grpc_service_hotbar_test.go
git commit -m "feat(gameserver): multi-bar hotbar — create/switch actions, update all Hotbar refs (#192)"
```

---

### Task 8: Frontend — `GameContext.tsx` state update

**Files:**
- Modify: `cmd/webclient/ui/src/game/GameContext.tsx`

- [ ] **Step 1: Extend state interface**

In `GameContext.tsx`, find the game state interface and add three fields:

```typescript
hotbarSlots: HotbarSlot[]
activeHotbarIndex: number   // 1-based
hotbarCount: number
maxHotbars: number
```

- [ ] **Step 2: Add `SET_HOTBAR` action fields**

In the action union type, update `SET_HOTBAR`:

```typescript
| { type: 'SET_HOTBAR'; slots: HotbarSlot[]; activeHotbarIndex: number; hotbarCount: number; maxHotbars: number }
```

- [ ] **Step 3: Update the reducer**

```typescript
case 'SET_HOTBAR':
  return {
    ...state,
    hotbarSlots: action.slots,
    activeHotbarIndex: action.activeHotbarIndex,
    hotbarCount: action.hotbarCount,
    maxHotbars: action.maxHotbars,
  }
```

- [ ] **Step 4: Update initial state**

```typescript
hotbarSlots: Array(10).fill({ kind: 'command', ref: '' }) as HotbarSlot[],
activeHotbarIndex: 1,
hotbarCount: 1,
maxHotbars: 4,
```

- [ ] **Step 5: Update the `HotbarUpdate` handler**

Find the `case 'HotbarUpdate':` block (around line 591) and update:

```typescript
case 'HotbarUpdate': {
  const hu = payload as {
    slots?: HotbarSlot[]
    active_hotbar_index?: number
    hotbar_count?: number
    max_hotbars?: number
  }
  const slots = hu.slots && hu.slots.length > 0
    ? hu.slots
    : (Array(10).fill({ kind: 'command', ref: '' }) as HotbarSlot[])
  dispatch({
    type: 'SET_HOTBAR',
    slots,
    activeHotbarIndex: hu.active_hotbar_index ?? 1,
    hotbarCount: hu.hotbar_count ?? 1,
    maxHotbars: hu.max_hotbars ?? 4,
  })
  break
}
```

- [ ] **Step 6: TypeScript build check**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui
npm run build 2>&1 | tail -20
```

Expected: no TypeScript errors.

- [ ] **Step 7: Commit**

```bash
git add cmd/webclient/ui/src/game/GameContext.tsx
git commit -m "feat(ui): extend GameContext for multi-bar hotbar state (#192)"
```

---

### Task 9: Frontend — `HotbarPanel.tsx` controls and keyboard shortcuts

**Files:**
- Modify: `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx`

- [ ] **Step 1: Add `useGame` state destructuring for new fields**

At the top of the `HotbarPanel` component, add the new state fields:

```typescript
const { hotbarSlots, activeHotbarIndex, hotbarCount, maxHotbars, sendMessage } = useGame()
```

- [ ] **Step 2: Add global keyboard handler for Ctrl+Up / Ctrl+Down**

Add a `useEffect` in the component:

```typescript
useEffect(() => {
  const handleKeyDown = (e: KeyboardEvent) => {
    if (!e.ctrlKey) return
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
    if (hotbarCount <= 1) return
    e.preventDefault()
    e.stopPropagation()
    if (e.key === 'ArrowUp') {
      // wrap: if at index 1, go to hotbarCount
      const target = activeHotbarIndex === 1 ? hotbarCount : activeHotbarIndex - 1
      sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
    } else {
      // wrap: if at hotbarCount, go to 1
      const target = activeHotbarIndex === hotbarCount ? 1 : activeHotbarIndex + 1
      sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
    }
  }
  window.addEventListener('keydown', handleKeyDown, { capture: true })
  return () => window.removeEventListener('keydown', handleKeyDown, { capture: true })
}, [activeHotbarIndex, hotbarCount, sendMessage])
```

- [ ] **Step 3: Add ▲/▼ switch controls, indicator, and "+ New Hotbar" button to the render**

In the JSX returned by `HotbarPanel`, wrap the existing slots with the new controls. The layout (left to right) is: `▲` `▼` `<N>/<total>` `[slot 1–10]` `[+ New Hotbar]`.

```tsx
const switchUp = () => {
  if (hotbarCount <= 1) return
  const target = activeHotbarIndex === 1 ? hotbarCount : activeHotbarIndex - 1
  sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
}
const switchDown = () => {
  if (hotbarCount <= 1) return
  const target = activeHotbarIndex === hotbarCount ? 1 : activeHotbarIndex + 1
  sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
}
const createHotbar = () => {
  sendMessage('HotbarRequest', { action: 'create' })
}

return (
  <div className="hotbar-panel">
    <button
      className="hotbar-switch-btn"
      onClick={switchUp}
      disabled={hotbarCount <= 1}
      title="Previous hotbar (Ctrl+Up)"
    >▲</button>
    <button
      className="hotbar-switch-btn"
      onClick={switchDown}
      disabled={hotbarCount <= 1}
      title="Next hotbar (Ctrl+Down)"
    >▼</button>
    <span className="hotbar-indicator">{activeHotbarIndex}/{hotbarCount}</span>

    {/* existing 10 slot buttons — unchanged */}
    {KEYS.map((key, idx) => (
      <HotbarSlotButton key={key} slotKey={key} slot={hotbarSlots[idx]} />
    ))}

    {hotbarCount < maxHotbars && (
      <button
        className="hotbar-new-btn"
        onClick={createHotbar}
        title="Create new hotbar"
      >+ New Hotbar</button>
    )}
  </div>
)
```

- [ ] **Step 4: TypeScript build check**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui
npm run build 2>&1 | tail -20
```

Expected: no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/webclient/ui/src/game/panels/HotbarPanel.tsx
git commit -m "feat(ui): add multi-bar hotbar controls — ▲▼ switch, indicator, new hotbar button, Ctrl+Up/Down (#192)"
```

---

### Task 10: Property-based test — `activeHotbarIndex` invariant

**Files:**
- Modify: `internal/gameserver/grpc_service_hotbar_test.go`

- [ ] **Step 1: Write the property test**

```go
// TestProperty_Hotbar_ActiveIndexAlwaysValid verifies that after any sequence of
// create and switch operations, ActiveHotbarIndex is always within [0, len(Hotbars)-1].
//
// Precondition: fresh session; up to 20 random operations.
// Postcondition: ActiveHotbarIndex always in valid range.
func TestProperty_Hotbar_ActiveIndexAlwaysValid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		svc, sess, uid := newTestHotbarServer(t, 4)
		ops := rapid.SliceOfN(rapid.IntRange(0, 5), 1, 20).Draw(t, "ops").([]int)
		for _, op := range ops {
			switch op % 3 {
			case 0: // create
				_, _ = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "create"})
			case 1: // switch up
				target := sess.ActiveHotbarIndex // 0-based; send 1-based
				if len(sess.Hotbars) > 1 {
					target = (target + 1) % len(sess.Hotbars)
				}
				_, _ = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: int32(target + 1)})
			case 2: // switch to random index
				targetIdx := rapid.IntRange(0, 10).Draw(t, "idx").(int)
				_, _ = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: int32(targetIdx)})
			}
			// Invariant: ActiveHotbarIndex always in valid range
			if sess.ActiveHotbarIndex < 0 || sess.ActiveHotbarIndex >= len(sess.Hotbars) {
				t.Fatalf("ActiveHotbarIndex %d out of range [0, %d)", sess.ActiveHotbarIndex, len(sess.Hotbars))
			}
		}
	})
}
```

- [ ] **Step 2: Run property test**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestProperty_Hotbar_ActiveIndexAlwaysValid" -v
```

Expected: PASS (100 rapid iterations)

- [ ] **Step 3: Run full test suite**

```bash
mise exec -- go test ./... -timeout 180s
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service_hotbar_test.go
git commit -m "test(gameserver): property test — ActiveHotbarIndex always in valid range (#192)"
```

---

## Spec Coverage Checklist

| Requirement | Task |
|---|---|
| REQ-MHB-1: `HotbarConfig` + `maxHotbars` | Task 1, Task 5 |
| REQ-MHB-2: Domain model `Hotbars` / `ActiveHotbarIndex` | Task 2 |
| REQ-MHB-3: DB migration | Task 3 |
| REQ-MHB-4: `LoadHotbars` with legacy migration | Task 4 |
| REQ-MHB-5: `SaveHotbars` | Task 4 |
| REQ-MHB-6: Proto fields | Task 6 |
| REQ-MHB-7: `create` and `switch` server actions | Task 7 |
| REQ-MHB-8: Existing `set`/`clear`/`show` on active bar | Task 7 |
| REQ-MHB-9: Frontend state | Task 8 |
| REQ-MHB-10: HotbarPanel controls | Task 9 |
| REQ-MHB-11: Ctrl+Up/Down keyboard shortcuts | Task 9 |
| REQ-MHB-12: HotbarSlotPicker unchanged | (no task needed) |
| REQ-MHB-13: Backward compatibility | Task 4 (legacy), Task 6 (proto additive) |
| REQ-MHB-14a: `LoadHotbars` legacy migration test | Task 4 |
| REQ-MHB-14b: `SaveHotbars` round-trip test | Task 4 |
| REQ-MHB-14c: `create` test | Task 7 |
| REQ-MHB-14d: `switch` test | Task 7 |
| REQ-MHB-14e: Property-based index invariant test | Task 10 |
