# Exploration Quests — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Light up exploration quests on top of the already-shipped `explore` objective primitive. Add three new objective shapes (`room_set`, `any_of`, `landmark_id`) alongside the existing single-room `target_id`. Add `is_landmark`/`landmark_id` to the `Room` model and a per-room landmark UI surface. Scale XP/Credits rewards by zone tier with author opt-out. Emit discovery narrative on advancement and on first landmark visit. Author three exemplar quests exercising each shape.

**Spec:** [docs/superpowers/specs/2026-04-25-exploration-quests.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-exploration-quests.md) (PR [#283](https://github.com/cory-johannsen/mud/pull/283))

**Architecture:** All four shapes ride on the existing `RecordExplore(charID, roomID)` dispatch in `quest.Service`. The function gains a switch on `QuestObjective` shape, persists per-room progress for `room_set` objectives in a new table `character_quest_explore_progress`, and emits a narrative line whenever progress materially changes. Landmarks are flagged on `world.Room` and surfaced via `world.Landmarks(zoneID)` plus a derived `is_landmark` column on the existing `character_map_rooms` table (so the per-character map can render landmark icons without joining). Reward scaling is a tiny pure helper `quest.ScaleReward(baseline, tier)` in the zone-difficulty package; the resolver consults it only when `QuestRewards.Scale` is true (default true). Telnet renders the new shapes via the existing `quest log`; the web client gets a `QuestLogPanel.tsx` component that mirrors. A new `landmarks` telnet command lists landmark discovery state.

**Tech Stack:** Go (`internal/game/quest/`, `internal/game/world/`, `internal/storage/postgres/`, `internal/gameserver/`), Postgres migrations, telnet, React/TypeScript (`cmd/webclient/ui/src/game/quests/`).

**Prerequisite:** None hard. The 2026-04-19 zone-difficulty-scaling spec defines the tier model — `quest.ScaleReward` lives in that package. The 2026-04-13 RRQ/VTQ spec defines the existing reward shape; this plan keeps those rewards stable by leaving `Scale` defaulted true and authoring `Scale: false` on legacy entries that should not change.

**Note on spec PR**: Spec is on PR #283, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/quest/def.go` (`RoomSet`, `AnyOf`, `LandmarkID`, `QuestRewards.Scale`) |
| Modify | `internal/game/quest/def_test.go` |
| Modify | `internal/game/quest/service.go` (extended `RecordExplore` dispatch) |
| Modify | `internal/game/quest/service_test.go` |
| Create | `internal/game/quest/reward.go` (`ScaleReward`) |
| Create | `internal/game/quest/reward_test.go` |
| Create | `internal/game/quest/store_explore_progress.go` |
| Create | `internal/storage/postgres/quest_explore_progress.go` |
| Create | `internal/storage/postgres/quest_explore_progress_test.go` |
| Modify | `internal/game/world/model.go` (`Room.IsLandmark`, `Room.LandmarkID`; loader validation) |
| Modify | `internal/game/world/landmarks.go` (`Landmarks(zoneID)`) |
| Modify | `internal/storage/postgres/automap.go` (derive `is_landmark` on map_rooms upsert) |
| Modify | `internal/gameserver/grpc_service.go` (discovery narrative emission) |
| Modify | `internal/gameserver/grpc_service_quests.go` (log render for new shapes) |
| Create | `internal/gameserver/grpc_service_landmarks.go` (`landmarks` telnet command) |
| Modify | `cmd/webclient/ui/src/game/quests/QuestLogPanel.tsx` |
| Create | `cmd/webclient/ui/src/game/quests/QuestLogPanel.test.tsx` |
| Create | `migrations/NNN_character_quest_explore_progress.up.sql`, `.down.sql` |
| Create | `migrations/NNN_character_map_rooms_is_landmark.up.sql`, `.down.sql` |
| Create | `content/quests/exq_discover_pdx_underbelly.yaml`, `exq_find_clown_camp.yaml`, `exq_landmark_radio_tower.yaml` |
| Modify | `content/zones/<chosen-zones>.yaml` (tag chosen rooms with `is_landmark` + `landmark_id`) |
| Modify | `docs/architecture/quests.md` |

---

### Task 1: Objective shape extensions + loader validation

**Files:**
- Modify: `internal/game/quest/def.go`
- Modify: `internal/game/quest/def_test.go`

- [ ] **Step 1: Failing tests** (EXQ-1, EXQ-2):

```go
func TestQuestObjective_RejectsMultipleTargetShapes(t *testing.T) {
    _, err := quest.LoadObjective(`
type: explore
target_id: room_a
room_set: [room_b, room_c]
`)
    require.Error(t, err)
    require.Contains(t, err.Error(), "exactly one of target_id, room_set, any_of, landmark_id")
}

func TestQuestObjective_RoomSetQuantityDefaultsToLength(t *testing.T) {
    obj, err := quest.LoadObjective(`type: explore
room_set: [a, b, c]
`)
    require.NoError(t, err)
    require.Equal(t, 3, obj.Quantity)
}

func TestQuestObjective_AnyOfQuantityIsOne(t *testing.T) {
    obj, _ := quest.LoadObjective(`type: explore
any_of: [a, b, c]
`)
    require.Equal(t, 1, obj.Quantity)
}

func TestQuestObjective_LandmarkIDQuantityIsOne(t *testing.T) {
    obj, _ := quest.LoadObjective(`type: explore
landmark_id: radio_tower
`)
    require.Equal(t, 1, obj.Quantity)
}

func TestQuestObjective_RejectsZeroTargetShapes(t *testing.T) {
    _, err := quest.LoadObjective(`type: explore`)
    require.Error(t, err)
}
```

- [ ] **Step 2: Implement**:

```go
type QuestObjective struct {
    Type       ObjectiveType
    TargetID   string   // existing
    Text       string   // existing
    Quantity   int      // existing
    RoomSet    []string // NEW
    AnyOf      []string // NEW
    LandmarkID string   // NEW
}

type QuestRewards struct {
    XP      int
    Credits int
    Items   []ItemReward
    Scale   *bool       // NEW; pointer so `omit` and `false` are distinguishable; nil → default true
}
```

Loader validation: count populated of `{TargetID != "", len(RoomSet) > 0, len(AnyOf) > 0, LandmarkID != ""}` MUST equal exactly 1 for `Type == ObjectiveExplore`.

- [ ] **Step 3:** All five tests pass.

---

### Task 2: Per-room set progress persistence

**Files:**
- Create: `migrations/NNN_character_quest_explore_progress.up.sql`, `.down.sql`
- Create: `internal/storage/postgres/quest_explore_progress.go`
- Create: `internal/storage/postgres/quest_explore_progress_test.go`
- Create: `internal/game/quest/store_explore_progress.go`

- [ ] **Step 1: Author migration** (EXQ-4, EXQ-24):

```sql
-- migrations/NNN_character_quest_explore_progress.up.sql
CREATE TABLE character_quest_explore_progress (
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    quest_id     TEXT NOT NULL,
    objective_id TEXT NOT NULL,
    room_id      TEXT NOT NULL,
    visited_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, quest_id, objective_id, room_id)
);
CREATE INDEX cqep_by_char_quest ON character_quest_explore_progress(character_id, quest_id);
```

- [ ] **Step 2: Failing tests** for the repository:

```go
func TestExploreProgress_Idempotent(t *testing.T) {
    s := newPGStore(t)
    require.NoError(t, s.RecordVisit("c1", "q1", "obj1", "rA"))
    require.NoError(t, s.RecordVisit("c1", "q1", "obj1", "rA"), "duplicate write must not error")
    rooms, _ := s.VisitedRooms("c1", "q1", "obj1")
    require.Equal(t, []string{"rA"}, rooms)
}

func TestExploreProgress_DistinctRoomsCount(t *testing.T) {
    s := newPGStore(t)
    s.RecordVisit("c1", "q1", "obj1", "rA")
    s.RecordVisit("c1", "q1", "obj1", "rB")
    s.RecordVisit("c1", "q1", "obj1", "rA") // duplicate
    n, _ := s.VisitedCount("c1", "q1", "obj1")
    require.Equal(t, 2, n)
}
```

- [ ] **Step 3: Implement** the repository (`RecordVisit` uses `INSERT ... ON CONFLICT DO NOTHING`).

- [ ] **Step 4: Migration applies and rolls back cleanly.**

---

### Task 3: Extended `RecordExplore` dispatch

**Files:**
- Modify: `internal/game/quest/service.go`
- Modify: `internal/game/quest/service_test.go`

- [ ] **Step 1: Failing tests** (EXQ-3, EXQ-22):

```go
func TestRecordExplore_TargetID_IncrementsByOne(t *testing.T) {
    svc := newSvc(t, withQuest(singleRoomQuest))
    svc.RecordExplore("c1", "room_a")
    require.Equal(t, 1, svc.Progress("c1", "single_quest", "obj1"))
}

func TestRecordExplore_RoomSet_IdempotentPerRoom(t *testing.T) {
    svc := newSvc(t, withQuest(roomSetQuest))
    svc.RecordExplore("c1", "room_a")
    svc.RecordExplore("c1", "room_a") // re-entry
    svc.RecordExplore("c1", "room_b")
    require.Equal(t, 2, svc.Progress("c1", "set_quest", "obj1"), "EXQ-3 idempotent contribution")
}

func TestRecordExplore_RoomSet_NeverExceedsTotal(t *testing.T) {
    svc := newSvc(t, withQuest(roomSetQuestSize2))
    svc.RecordExplore("c1", "room_a")
    svc.RecordExplore("c1", "room_b")
    svc.RecordExplore("c1", "room_c") // not in set; ignored
    require.Equal(t, 2, svc.Progress("c1", "set_quest", "obj1"))
}

func TestRecordExplore_AnyOf_CompletesOnFirstMatch(t *testing.T) {
    svc := newSvc(t, withQuest(anyOfQuest))
    svc.RecordExplore("c1", "room_a")
    require.True(t, svc.Completed("c1", "any_quest"))
}

func TestRecordExplore_LandmarkID_CompletesOnLandmarkVisit(t *testing.T) {
    svc := newSvc(t, withQuest(landmarkQuest), withRoom("radio_tower_room", withLandmark("radio_tower")))
    svc.RecordExplore("c1", "radio_tower_room")
    require.True(t, svc.Completed("c1", "landmark_quest"))
}
```

- [ ] **Step 2: Implement** the dispatch:

```go
func (s *Service) RecordExplore(charID, roomID string) {
    room := s.world.Room(roomID)
    for _, q := range s.activeQuests(charID) {
        for _, obj := range q.Def.Objectives {
            if obj.Type != ObjectiveExplore { continue }
            advanced := false
            switch {
            case obj.TargetID == roomID:
                advanced = s.incrementProgress(charID, q.ID, obj.ID, 1)
            case contains(obj.RoomSet, roomID):
                if s.exploreStore.RecordVisit(charID, q.ID, obj.ID, roomID) == nil {
                    n, _ := s.exploreStore.VisitedCount(charID, q.ID, obj.ID)
                    advanced = s.setProgress(charID, q.ID, obj.ID, min(n, len(obj.RoomSet)))
                }
            case contains(obj.AnyOf, roomID):
                advanced = s.setProgress(charID, q.ID, obj.ID, 1)
            case obj.LandmarkID != "" && room != nil && room.LandmarkID == obj.LandmarkID:
                advanced = s.setProgress(charID, q.ID, obj.ID, 1)
            }
            if advanced {
                s.emitNarrative(charID, q, obj)
                s.maybeComplete(charID, q.ID)
            }
        }
    }
}
```

- [ ] **Step 3: Test the existing single-room behaviour stays unchanged** (EXQ-G6) — re-run the existing quest tests; nothing changes.

---

### Task 4: Reward scaling helper

**Files:**
- Create: `internal/game/quest/reward.go`
- Create: `internal/game/quest/reward_test.go`

- [ ] **Step 1: Failing tests** (EXQ-9, EXQ-10, EXQ-11, EXQ-12):

```go
func TestScaleReward_TierMultipliers(t *testing.T) {
    cases := map[zone.Tier]int{
        zone.DesperateStreets: 100,
        zone.ArmedDangerous:   200,
        zone.Warlord:          400,
        zone.ApexPredator:     800,
        zone.EndTimes:         1600,
    }
    for tier, want := range cases {
        require.Equal(t, want, quest.ScaleReward(100, tier), "tier %v", tier)
    }
}

func TestApplyRewards_ScaleFalseSkipsScaling(t *testing.T) {
    granted := quest.ApplyRewards(rewards{XP: 100, Credits: 50, Scale: ptr(false)}, zone.EndTimes)
    require.Equal(t, 100, granted.XP)
    require.Equal(t, 50, granted.Credits)
}

func TestApplyRewards_ScaleTrueScalesXPAndCreditsButNotItems(t *testing.T) {
    granted := quest.ApplyRewards(rewards{
        XP: 100, Credits: 50,
        Items: []ItemReward{{ID: "lockpick", Quantity: 1}},
        Scale: ptr(true),
    }, zone.Warlord)
    require.Equal(t, 400, granted.XP)
    require.Equal(t, 200, granted.Credits)
    require.Equal(t, 1, granted.Items[0].Quantity, "items not scaled (EXQ-9)")
}

func TestApplyRewards_MultiTierRoomSetUsesHighestTier(t *testing.T) {
    rooms := []zone.Tier{zone.DesperateStreets, zone.ApexPredator, zone.ArmedDangerous}
    require.Equal(t, zone.ApexPredator, quest.HighestTier(rooms))
}
```

- [ ] **Step 2: Implement** in the zone-difficulty package (importable as `quest.ScaleReward` via re-export, or directly in quest pkg):

```go
var tierMultipliers = map[zone.Tier]float64{
    zone.DesperateStreets: 1.0,
    zone.ArmedDangerous:   2.0,
    zone.Warlord:          4.0,
    zone.ApexPredator:     8.0,
    zone.EndTimes:         16.0,
}

func ScaleReward(baseline int, tier zone.Tier) int {
    return int(math.Round(float64(baseline) * tierMultipliers[tier]))
}

func HighestTier(tiers []zone.Tier) zone.Tier {
    out := tiers[0]
    for _, t := range tiers[1:] { if t > out { out = t } }
    return out
}
```

- [ ] **Step 3:** Resolver hook in `quest.maybeComplete`: when granting rewards, gather the tiers of all relevant rooms (`obj.TargetID`, `obj.RoomSet`, `obj.AnyOf`, or the landmark's room), pick the highest, scale XP and Credits if `Scale != false`.

---

### Task 5: Landmarks — `Room.IsLandmark`, validation, helper, map column

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader_test.go`
- Create: `internal/game/world/landmarks.go`
- Create: `migrations/NNN_character_map_rooms_is_landmark.up.sql`, `.down.sql`
- Modify: `internal/storage/postgres/automap.go`

- [ ] **Step 1: Failing tests** (EXQ-5, EXQ-6, EXQ-7, EXQ-8):

```go
func TestLoader_RejectsMissingLandmarkIDWhenIsLandmarkTrue(t *testing.T) {
    _, err := world.LoadRoom(`is_landmark: true`)
    require.Error(t, err)
    require.Contains(t, err.Error(), "landmark_id required when is_landmark is true")
}

func TestLoader_RejectsDuplicateLandmarkIDAcrossRooms(t *testing.T) {
    _, err := world.LoadAll([]roomYAML{
        {ID: "a", IsLandmark: true, LandmarkID: "tower"},
        {ID: "b", IsLandmark: true, LandmarkID: "tower"},
    })
    require.Error(t, err)
    require.Contains(t, err.Error(), "duplicate landmark_id 'tower'")
}

func TestLandmarks_OrderedByLandmarkID(t *testing.T) {
    w := worldWithLandmarks(t, []string{"radio_tower", "abandoned_subway", "factory"})
    got := w.Landmarks("portland_zone")
    ids := make([]string, len(got))
    for i, r := range got { ids[i] = r.LandmarkID }
    require.Equal(t, []string{"abandoned_subway", "factory", "radio_tower"}, ids)
}

func TestAutomap_IsLandmarkPopulatedOnUpsert(t *testing.T) {
    db := newPGStore(t)
    db.UpsertVisit("c1", "radio_tower_room", true /*isLandmark*/)
    rows := db.MapRowsForCharacter("c1")
    require.True(t, rows[0].IsLandmark)
}
```

- [ ] **Step 2: Add fields to `world.Room`**:

```go
type Room struct {
    // ... existing ...
    IsLandmark bool
    LandmarkID string
}
```

- [ ] **Step 3: Loader validation** — every quest objective `LandmarkID` resolves to a real landmark; no two rooms share `LandmarkID`.

- [ ] **Step 4: Migration**:

```sql
-- migrations/NNN_character_map_rooms_is_landmark.up.sql
ALTER TABLE character_map_rooms ADD COLUMN is_landmark BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX character_map_rooms_landmarks ON character_map_rooms(character_id) WHERE is_landmark;
```

- [ ] **Step 5: `Landmarks(zoneID)`** returns a sorted slice; `automap.UpsertVisit(...)` is extended to take `isLandmark bool` and stores it.

---

### Task 6: Discovery narrative emission

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_test.go`

- [ ] **Step 1: Failing tests** (EXQ-13, EXQ-14, EXQ-15):

```go
func TestNarrative_EmitsOnRoomSetAdvancement(t *testing.T) {
    sess := startSession(t)
    enterRoom(sess, "room_a")
    enterRoom(sess, "room_b")
    out := sess.LastNarrative()
    require.Contains(t, out, "Quest progress")
    require.Contains(t, out, "2/3 rooms")
}

func TestNarrative_NoSpamForCorridorReentry(t *testing.T) {
    sess := startSession(t)
    enterRoom(sess, "room_corridor") // not in any quest
    enterRoom(sess, "room_corridor") // re-entry
    require.Equal(t, "", sess.LastNarrative(), "no narrative on plain corridor (EXQ-14)")
}

func TestNarrative_LandmarkFirstDiscoveryAlwaysEmits(t *testing.T) {
    sess := startSession(t)
    enterRoom(sess, "radio_tower_room")
    require.Contains(t, sess.LastNarrative(), "Discovery: Radio Tower")
}

func TestNarrative_LandmarkRevisitDoesNotEmit(t *testing.T) {
    sess := startSession(t)
    enterRoom(sess, "radio_tower_room")
    sess.ClearNarrative()
    enterRoom(sess, "radio_tower_room")
    require.Equal(t, "", sess.LastNarrative())
}
```

- [ ] **Step 2: Implement** the narrative emit at the call site of `RecordExplore` in the room-enter pipeline. The quest service exposes a callback `OnAdvanced(charID, quest, obj, oldProgress, newProgress)` that the gameserver wires to `sess.Send(narrative)`.

- [ ] **Step 3: Landmark first-discovery** is a separate emit driven from the `automap` upsert path: when the upsert returns `(inserted=true, isLandmark=true)`, emit `"Discovery: <landmark name>"`.

---

### Task 7: Telnet quest log + `landmarks` command

**Files:**
- Modify: `internal/gameserver/grpc_service_quests.go`
- Create: `internal/gameserver/grpc_service_landmarks.go`
- Modify: `internal/gameserver/grpc_service_quests_test.go`

- [ ] **Step 1: Failing tests** (EXQ-16, EXQ-18):

```go
func TestQuestLog_RoomSetRendering(t *testing.T) {
    out := renderQuestLog(t, withRoomSetQuest(visited: 2, total: 3))
    require.Contains(t, out, "2/3 rooms discovered")
}

func TestQuestLog_AnyOfRendering(t *testing.T) {
    out := renderQuestLog(t, withAnyOfQuest("entry_1", "entry_2", "entry_3"))
    require.Contains(t, out, "discover any of: entry_1, entry_2, entry_3")
}

func TestQuestLog_LandmarkRendering(t *testing.T) {
    out := renderQuestLog(t, withLandmarkQuest("radio_tower"))
    require.Contains(t, out, `discover landmark "Radio Tower"`)
}

func TestLandmarksCommand_ListsZoneLandmarksWithDiscoveryState(t *testing.T) {
    out := runLandmarks(t, withDiscovered("radio_tower"))
    require.Contains(t, out, "radio_tower — Radio Tower [discovered: yes]")
    require.Contains(t, out, "factory — Old Factory [discovered: no]")
}
```

- [ ] **Step 2: Implement** the new render branches in `grpc_service_quests.go` and the `landmarks` command in a new file. The `landmarks` command consults `world.Landmarks(zone)` and the per-character map table for discovery state.

---

### Task 8: Web `QuestLogPanel`

**Files:**
- Modify: `cmd/webclient/ui/src/game/quests/QuestLogPanel.tsx`
- Create: `cmd/webclient/ui/src/game/quests/QuestLogPanel.test.tsx`

- [ ] **Step 1: Failing component tests** (EXQ-17):

```ts
test("renders room_set objective with x/y progress", () => {
  render(<QuestLogPanel quests={[{ objectives: [{ kind: "room_set", visited: 2, total: 3, text: "Discover the underbelly" }] }]} />);
  expect(screen.getByText("Discover the underbelly")).toBeVisible();
  expect(screen.getByText("2/3 rooms discovered")).toBeVisible();
});

test("renders any_of objective", () => {
  render(<QuestLogPanel quests={[{ objectives: [{ kind: "any_of", anyOf: ["entry_1", "entry_2"], completed: false }] }]} />);
  expect(screen.getByText(/Discover any of/)).toBeVisible();
});

test("renders landmark objective with name", () => {
  render(<QuestLogPanel quests={[{ objectives: [{ kind: "landmark", landmarkName: "Radio Tower", completed: true }] }]} />);
  expect(screen.getByText(/Radio Tower/)).toBeVisible();
});

test("clicking objective row links to zone map filtered to its targets", async () => {
  const onMapFilter = jest.fn();
  render(<QuestLogPanel onMapFilter={onMapFilter} quests={...} />);
  fireEvent.click(screen.getByText("Discover the underbelly"));
  expect(onMapFilter).toHaveBeenCalledWith({ rooms: ["room_a", "room_b", "room_c"] });
});

test("collapsible objective row toggles details", () => {
  const { container } = render(<QuestLogPanel quests={[{ objectives: [{ kind: "room_set", visited: 1, total: 3, rooms: ["a","b","c"] }] }]} />);
  expect(container.querySelector(".objective-details")).toHaveAttribute("hidden");
  fireEvent.click(screen.getByText(/Discover/));
  expect(container.querySelector(".objective-details")).not.toHaveAttribute("hidden");
});
```

- [ ] **Step 2: Implement** the component with collapsible objective rows. Map filter dispatch uses the existing zone-map slice's `setRoomFilter` action.

- [ ] **Step 3:** Visual parity with the telnet rendering — same numerator/denominator, same landmark name format.

---

### Task 9: Exemplar quests + content tagging + docs

**Files:**
- Create: `content/quests/exq_discover_pdx_underbelly.yaml`
- Create: `content/quests/exq_find_clown_camp.yaml`
- Create: `content/quests/exq_landmark_radio_tower.yaml`
- Modify: `content/zones/<chosen>.yaml` (add `is_landmark: true` + `landmark_id` to selected rooms)
- Modify: `docs/architecture/quests.md`

- [ ] **Step 1: Checkpoint (EXQ-20).** Confirm with user the rooms/landmarks for each exemplar:
  - `exq_discover_pdx_underbelly`: three Portland-area rooms.
  - `exq_find_clown_camp`: several entry points to clown-camp territory.
  - `exq_landmark_radio_tower`: one landmark room.

- [ ] **Step 2: Author the three quest YAML files**:

```yaml
# content/quests/exq_discover_pdx_underbelly.yaml
id: exq_discover_pdx_underbelly
title: The Portland Underbelly
description: Word on the street: there are three places nobody talks about. Find them.
objectives:
  - id: obj1
    type: explore
    text: Discover the abandoned subway, the bone yard, and the rust market.
    room_set:
      - subway_entrance
      - bone_yard
      - rust_market
rewards:
  xp: 100
  credits: 200
  scale: true
```

(Plus the `any_of` and `landmark_id` exemplars.)

- [ ] **Step 3: Tag landmark rooms** in their zone YAML:

```yaml
# content/zones/portland.yaml
rooms:
  - id: radio_tower_room
    name: Radio Tower
    is_landmark: true
    landmark_id: radio_tower
    # ... existing fields ...
```

- [ ] **Step 4: Architecture doc** — section explaining the four objective shapes, when to use each, the reward-scaling rule, the landmark concept, and the discovery narrative semantics. Cross-link spec, plan, exemplars.

---

### Task 10: Integration test — end-to-end exemplars

**Files:**
- Create: `internal/gameserver/grpc_service_exq_integration_test.go`

- [ ] **Step 1: Failing test** (EXQ-23):

```go
func TestExqIntegration_RoomSetCompletesWithScaledRewards(t *testing.T) {
    s := newServer(t, withExemplars())
    char := newCharacter(t, s)
    enterRoom(s, char, "subway_entrance")
    enterRoom(s, char, "bone_yard")
    enterRoom(s, char, "rust_market")
    progress := s.QuestProgress(char.ID, "exq_discover_pdx_underbelly")
    require.True(t, progress.Completed)
    // Highest tier among the three rooms determines scaling.
    require.Equal(t, 200 /*baseline*/ * mul(highestTier()), char.GrantedCredits)
}
```

- [ ] **Step 2:** Similar end-to-end runs for `any_of` and `landmark_id` exemplars.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
make migrate-up && make migrate-down  # both directions clean
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff.
- Smoke test on telnet: pick up `exq_find_clown_camp`, enter one of the listed rooms, verify completion narrative + reward grant; `quest log` shows the right shape; `landmarks` lists the radio tower with discovery state.
- Smoke test on web: same scenarios in `QuestLogPanel`; clicking an objective row filters the zone map to its targets.

---

## Rollout / Open Questions Resolved at Plan Time

- **EXQ-Q1**: Narrative emits on every increment. Players want visible progress.
- **EXQ-Q2**: Reward multipliers default to spec values (×1, ×2, ×4, ×8, ×16) but `Scale: false` is the per-quest opt-out. User can re-balance globally without code change by editing the multiplier constants in `quest/reward.go`.
- **EXQ-Q3**: `any_of` tracks the satisfying room name in the completion narrative only; not in serialized state.
- **EXQ-Q4**: Undiscovered landmarks render as `???` greyed icons on the zone map; named after first discovery. Aligns with #254's progressive-reveal pattern.
- **EXQ-Q5**: Fast-travel unlocks deferred. Out of scope.

## Non-Goals Reaffirmed

Per spec §2.2:

- No replacement of existing `explore` objective semantics.
- No cross-zone objectives.
- No time-limited "race" exploration.
- No party-shared exploration credit.
- No procedural quest generation.
- Reward types stay XP/Credits/Items.
