# Region Descriptions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace raw region ID strings with grammatically correct display names ("the Northeast") across character selection, in-game status, and examine-player surfaces.

**Architecture:** Add `Article` field to `Region` struct with `DisplayName()` helper. Propagate the resolved display name through three paths: (1) character selection uses `h.regions` lookup in `AuthHandler`; (2) in-game status/examine extend `JoinWorldRequest` proto to carry region/class/level so `PlayerSession` can store display values without DB lookups; (3) frontend renders `CharacterInfo` events (currently silently ignored).

**Tech Stack:** Go, protobuf/gRPC, YAML, `pgregory.net/rapid` for property tests.

---

### Task 1: Add Article field to Region struct and all YAML files

**Files:**
- Modify: `internal/game/ruleset/region.go`
- Modify: `content/regions/northeast.yaml` (and all 10 other region files)
- Test: `internal/game/ruleset/region_test.go`

**Step 1: Write the failing test**

```go
// internal/game/ruleset/region_test.go
package ruleset_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestRegion_DisplayName_WithArticle(t *testing.T) {
    r := &ruleset.Region{Name: "Northeast", Article: "the"}
    if got := r.DisplayName(); got != "the Northeast" {
        t.Errorf("DisplayName() = %q, want %q", got, "the Northeast")
    }
}

func TestRegion_DisplayName_WithoutArticle(t *testing.T) {
    r := &ruleset.Region{Name: "Gresham Outskirts", Article: ""}
    if got := r.DisplayName(); got != "Gresham Outskirts" {
        t.Errorf("DisplayName() = %q, want %q", got, "Gresham Outskirts")
    }
}

func TestProperty_Region_DisplayName_NonEmpty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        name := rapid.StringMatching(`[A-Za-z ]+`).Draw(t, "name")
        article := rapid.SampledFrom([]string{"", "the", "a"}).Draw(t, "article")
        r := &ruleset.Region{Name: name, Article: article}
        got := r.DisplayName()
        if got == "" {
            t.Fatalf("DisplayName() must not be empty for non-empty name %q", name)
        }
    })
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/game/ruleset/... -run TestRegion_DisplayName -v`
Expected: FAIL — `DisplayName` undefined

**Step 3: Add Article field and DisplayName() to Region struct**

In `internal/game/ruleset/region.go`, update `Region` struct and add `DisplayName()`:

```go
type Region struct {
    ID          string         `yaml:"id"`
    Name        string         `yaml:"name"`
    Article     string         `yaml:"article"`
    Description string         `yaml:"description"`
    Modifiers   map[string]int `yaml:"modifiers"`
    Traits      []string       `yaml:"traits"`
}

// DisplayName returns the human-readable region name with its grammatical article.
// If Article is empty, returns Name alone.
//
// Precondition: Name must be non-empty.
// Postcondition: Returns a non-empty string.
func (r *Region) DisplayName() string {
    if r.Article == "" {
        return r.Name
    }
    return r.Article + " " + r.Name
}
```

**Step 4: Run test to verify it passes**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/game/ruleset/... -v`
Expected: PASS

**Step 5: Add `article: "the"` to all 11 region YAML files**

Each of the following files in `content/regions/` needs `article: "the"` added after the `name:` line:
- `gresham_outskirts.yaml`
- `midwest.yaml`
- `mountain.yaml`
- `northeast.yaml`
- `north_portland.yaml`
- `old_town.yaml`
- `pacific_northwest.yaml`
- `pearl_district.yaml`
- `southeast_portland.yaml`
- `southern_california.yaml`
- `south.yaml`

**Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/ruleset/region.go internal/game/ruleset/region_test.go content/regions/
git commit -m "feat: add Article field and DisplayName() to Region"
```

---

### Task 2: Fix character selection display

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`
- Modify: `internal/frontend/handlers/character_flow_test.go`

**Step 1: Write failing tests**

In `character_flow_test.go`, update `TestFormatCharacterSummary`, `TestFormatCharacterStats`, and property tests to expect "from the Northeast" instead of "from northeast", and pass the region display string:

```go
func TestFormatCharacterSummary_WithRegionDisplay(t *testing.T) {
    c := &character.Character{
        Name: "TestChar", Region: "northeast", Class: "Gunner", Level: 3,
    }
    summary := handlers.FormatCharacterSummary(c, "the Northeast")
    if !strings.Contains(summary, "from the Northeast") {
        t.Errorf("expected 'from the Northeast' in %q", summary)
    }
}

func TestFormatCharacterStats_WithRegionDisplay(t *testing.T) {
    c := &character.Character{
        Name: "TestChar", Region: "northeast", Class: "Gunner", Level: 2,
        CurrentHP: 10, MaxHP: 10,
        Abilities: character.AbilityScores{Brutality: 10, Quickness: 10, Grit: 10, Reasoning: 10, Savvy: 10, Flair: 10},
    }
    stats := handlers.FormatCharacterStats(c, "the Northeast")
    if !strings.Contains(stats, "the Northeast") {
        t.Errorf("expected 'the Northeast' in stats %q", stats)
    }
}

func TestProperty_FormatCharacterSummary_ContainsRegionDisplay(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        name := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "name")
        regionID := rapid.StringMatching(`[a-z_]+`).Draw(t, "regionID")
        display := rapid.StringMatching(`[A-Za-z ]+`).Draw(t, "display")
        c := &character.Character{Name: name, Region: regionID, Class: "Gunner", Level: 1}
        summary := handlers.FormatCharacterSummary(c, display)
        if !strings.Contains(summary, display) {
            t.Fatalf("FormatCharacterSummary must contain regionDisplay %q in %q", display, summary)
        }
    })
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -run TestFormatCharacter -v`
Expected: FAIL — wrong function signature

**Step 3: Update FormatCharacterSummary and FormatCharacterStats signatures**

In `character_flow.go`:

```go
// FormatCharacterSummary returns a one-line summary of a character for the selection list.
//
// Precondition: c must be non-nil; regionDisplay must be non-empty.
// Postcondition: Returns a non-empty human-readable string.
func FormatCharacterSummary(c *character.Character, regionDisplay string) string {
    return fmt.Sprintf("%s%s%s — Lvl %d %s from %s",
        telnet.BrightWhite, c.Name, telnet.Reset,
        c.Level, c.Class, regionDisplay)
}

// FormatCharacterStats returns a multi-line stats block for the character preview.
//
// Precondition: c must be non-nil; regionDisplay must be non-empty.
// Postcondition: Returns a formatted multi-line string with HP and all six ability scores.
func FormatCharacterStats(c *character.Character, regionDisplay string) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("  Name:   %s%s%s\r\n", telnet.BrightWhite, c.Name, telnet.Reset))
    sb.WriteString(fmt.Sprintf("  Region: %s   Class: %s   Level: %d\r\n", regionDisplay, c.Class, c.Level))
    sb.WriteString(fmt.Sprintf("  HP:     %d/%d\r\n", c.CurrentHP, c.MaxHP))
    sb.WriteString(fmt.Sprintf("  BRT:%2d  QCK:%2d  GRT:%2d  RSN:%2d  SAV:%2d  FLR:%2d\r\n",
        c.Abilities.Brutality, c.Abilities.Quickness, c.Abilities.Grit,
        c.Abilities.Reasoning, c.Abilities.Savvy, c.Abilities.Flair))
    return sb.String()
}
```

**Step 4: Add regionDisplayName helper to AuthHandler and update call sites**

Add a private helper to `character_flow.go`:

```go
// regionDisplayName returns the DisplayName for the region with id, or id itself if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns a non-empty string.
func (h *AuthHandler) regionDisplayName(id string) string {
    for _, r := range h.regions {
        if r.ID == id {
            return r.DisplayName()
        }
    }
    return id
}
```

Update call sites in `character_flow.go`:
- Line 114: `FormatCharacterSummary(c)` → `FormatCharacterSummary(c, h.regionDisplayName(c.Region))`
- Line 340: `FormatCharacterStats(newChar)` → `FormatCharacterStats(newChar, region.DisplayName())` (has the `*ruleset.Region` directly)

**Step 5: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -v`
Expected: PASS

**Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/frontend/handlers/character_flow.go internal/frontend/handlers/character_flow_test.go
git commit -m "feat: use region display name in character selection screens"
```

---

### Task 3: Propagate region/class/level through JoinWorldRequest to PlayerSession

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/game_bridge.go`
- Test: `internal/game/session/manager_test.go`
- Test: `internal/gameserver/grpc_service_login_test.go`

**Step 1: Write failing tests**

In `internal/game/session/manager_test.go`, add test that `PlayerSession` has `RegionDisplayName`, `Class`, `Level` fields set:

```go
func TestAddPlayer_StoresDisplayFields(t *testing.T) {
    m := session.NewManager()
    sess, err := m.AddPlayer("u1", "user", "Hero", 1, "room1", 10, "player",
        "the Northeast", "Gunner", 3)
    if err != nil {
        t.Fatalf("AddPlayer error: %v", err)
    }
    if sess.RegionDisplayName != "the Northeast" {
        t.Errorf("RegionDisplayName = %q, want %q", sess.RegionDisplayName, "the Northeast")
    }
    if sess.Class != "Gunner" {
        t.Errorf("Class = %q, want %q", sess.Class, "Gunner")
    }
    if sess.Level != 3 {
        t.Errorf("Level = %d, want 3", sess.Level)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/game/session/... -run TestAddPlayer_StoresDisplayFields -v`
Expected: FAIL — `AddPlayer` has wrong signature

**Step 3: Update proto — add region_display, class, level to JoinWorldRequest**

In `api/proto/game/v1/game.proto`, update `JoinWorldRequest`:

```proto
message JoinWorldRequest {
  string uid            = 1;
  string username       = 2;
  int64  character_id   = 3;
  string character_name = 4;
  int32  current_hp     = 5;
  string location       = 6;
  string role           = 7;
  string region_display = 8;
  string class          = 9;
  int32  level          = 10;
}
```

Run: `cd /home/cjohannsen/src/mud && make proto`

**Step 4: Add fields to PlayerSession and update AddPlayer**

In `internal/game/session/manager.go`:

Add fields to `PlayerSession`:
```go
// RegionDisplayName is the human-readable region display name (e.g. "the Northeast").
RegionDisplayName string
// Class is the character's job/class ID.
Class string
// Level is the character's current level.
Level int
```

Update `AddPlayer` signature to include the three new fields:
```go
func (m *Manager) AddPlayer(uid, username, charName string, characterID int64, roomID string, currentHP int, role string, regionDisplayName string, class string, level int) (*PlayerSession, error)
```

Populate in the `sess` construction:
```go
sess := &PlayerSession{
    // ... existing fields ...
    RegionDisplayName: regionDisplayName,
    Class:             class,
    Level:             level,
}
```

**Step 5: Update grpc_service.go Session() call to AddPlayer**

In `internal/gameserver/grpc_service.go`, update the `AddPlayer` call at ~line 182:

```go
sess, err := s.sessions.AddPlayer(
    uid, username, charName, characterID, spawnRoom.ID, currentHP, role,
    join.RegionDisplay, join.Class, int(join.Level),
)
```

**Step 6: Update game_bridge.go to send region_display/class/level in JoinWorldRequest**

In `internal/frontend/handlers/game_bridge.go`:

```go
// Resolve region display name using h.regions
regionDisplay := char.Region
for _, r := range h.regions {
    if r.ID == char.Region {
        regionDisplay = r.DisplayName()
        break
    }
}

if err := stream.Send(&gamev1.ClientMessage{
    RequestId: "join",
    Payload: &gamev1.ClientMessage_JoinWorld{
        JoinWorld: &gamev1.JoinWorldRequest{
            Uid:           uid,
            Username:      acct.Username,
            CharacterId:   char.ID,
            CharacterName: char.Name,
            CurrentHp:     int32(char.CurrentHP),
            Location:      char.Location,
            Role:          acct.Role,
            RegionDisplay: regionDisplay,
            Class:         char.Class,
            Level:         int32(char.Level),
        },
    },
}); err != nil {
    return fmt.Errorf("sending join request: %w", err)
}
```

**Step 7: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/game/session/... ./internal/gameserver/... ./internal/frontend/handlers/... -v`
Expected: PASS

**Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/game/v1/ internal/game/session/manager.go internal/gameserver/grpc_service.go internal/frontend/handlers/game_bridge.go
git commit -m "feat: propagate region display/class/level through JoinWorldRequest to PlayerSession"
```

---

### Task 4: Display CharacterInfo in status command

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/game_bridge.go`
- Test: `internal/frontend/handlers/text_renderer_test.go`
- Test: `internal/gameserver/grpc_service_status_test.go` (new)

**Step 1: Write failing tests**

In `internal/frontend/handlers/text_renderer_test.go`, add:

```go
func TestRenderCharacterInfo(t *testing.T) {
    ci := &gamev1.CharacterInfo{
        Name: "Hero", Region: "the Northeast", Class: "Gunner", Level: 3,
        CurrentHp: 15, MaxHp: 20,
    }
    got := handlers.RenderCharacterInfo(ci)
    if !strings.Contains(got, "the Northeast") {
        t.Errorf("expected 'the Northeast' in %q", got)
    }
    if !strings.Contains(got, "Hero") {
        t.Errorf("expected 'Hero' in %q", got)
    }
}

func TestProperty_RenderCharacterInfo_NonEmpty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        name := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "name")
        region := rapid.StringMatching(`[A-Za-z ]+`).Draw(t, "region")
        ci := &gamev1.CharacterInfo{Name: name, Region: region, Class: "Gunner", Level: 1, CurrentHp: 10, MaxHp: 10}
        got := handlers.RenderCharacterInfo(ci)
        if got == "" {
            t.Fatal("RenderCharacterInfo must not return empty string")
        }
    })
}
```

In `internal/gameserver/grpc_service_status_test.go` (new file), add test that `handleStatus` sends a `CharacterInfo` event as the first event:

```go
package gameserver_test

import (
    "testing"
    gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"
    "github.com/cory-johannsen/mud/internal/game/session"
)

func TestHandleStatus_SendsCharacterInfoFirst(t *testing.T) {
    // Set up a mock session with region/class/level populated
    // This test verifies the contract: CharacterInfo is the first event sent
    // Use the existing test infrastructure pattern from grpc_service_login_test.go
    // ...
    // Assert first event is CharacterInfo with the correct region display name
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -run TestRenderCharacterInfo -v`
Expected: FAIL — `RenderCharacterInfo` undefined

**Step 3: Add RenderCharacterInfo to text_renderer.go**

In `internal/frontend/handlers/text_renderer.go`:

```go
// RenderCharacterInfo formats a CharacterInfo event as a multi-line Telnet stats block.
//
// Precondition: ci must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderCharacterInfo(ci *gamev1.CharacterInfo) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("  %s%s%s\r\n", telnet.BrightWhite, ci.Name, telnet.Reset))
    sb.WriteString(fmt.Sprintf("  Region: %s   Class: %s   Level: %d\r\n", ci.Region, ci.Class, ci.Level))
    sb.WriteString(fmt.Sprintf("  HP: %d/%d\r\n", ci.CurrentHp, ci.MaxHp))
    sb.WriteString(fmt.Sprintf("  BRT:%2d  QCK:%2d  GRT:%2d  RSN:%2d  SAV:%2d  FLR:%2d\r\n",
        ci.Brutality, ci.Quickness, ci.Grit, ci.Reasoning, ci.Savvy, ci.Flair))
    return sb.String()
}
```

**Step 4: Update frontend event loop to render CharacterInfo**

In `internal/frontend/handlers/game_bridge.go`, change the `CharacterInfo` case from silently ignoring to rendering:

```go
case *gamev1.ServerEvent_CharacterInfo:
    text = RenderCharacterInfo(p.CharacterInfo)
```

**Step 5: Update handleStatus to send CharacterInfo first**

In `internal/gameserver/grpc_service.go`, update `handleStatus` to send a `CharacterInfo` event before conditions:

```go
func (s *GameServiceServer) handleStatus(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return fmt.Errorf("player %q not found", uid)
    }

    // Send character info first.
    if err := stream.Send(&gamev1.ServerEvent{
        RequestId: requestID,
        Payload: &gamev1.ServerEvent_CharacterInfo{
            CharacterInfo: &gamev1.CharacterInfo{
                CharacterId: sess.CharacterID,
                Name:        sess.CharName,
                Region:      sess.RegionDisplayName,
                Class:       sess.Class,
                Level:       int32(sess.Level),
                CurrentHp:   int32(sess.CurrentHP),
                // MaxHP, abilities not stored in session — omit (zero values)
            },
        },
    }); err != nil {
        return fmt.Errorf("sending character info: %w", err)
    }

    // Then send conditions.
    conds, err := s.combatH.Status(uid)
    if err != nil {
        return err
    }
    if len(conds) == 0 {
        return stream.Send(&gamev1.ServerEvent{
            RequestId: requestID,
            Payload: &gamev1.ServerEvent_ConditionEvent{
                ConditionEvent: &gamev1.ConditionEvent{},
            },
        })
    }
    for _, ac := range conds {
        if err := stream.Send(&gamev1.ServerEvent{
            RequestId: requestID,
            Payload: &gamev1.ServerEvent_ConditionEvent{
                ConditionEvent: &gamev1.ConditionEvent{
                    TargetUid:     uid,
                    TargetName:    sess.CharName,
                    ConditionId:   ac.Def.ID,
                    ConditionName: ac.Def.Name,
                    Stacks:        int32(ac.Stacks),
                    Applied:       true,
                },
            },
        }); err != nil {
            return fmt.Errorf("sending condition event: %w", err)
        }
    }
    return nil
}
```

**Step 6: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... ./internal/gameserver/... -v`
Expected: PASS

**Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go internal/frontend/handlers/text_renderer.go internal/frontend/handlers/game_bridge.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: show character info with region display in status command"
```

---

### Task 5: Extend examine to support player targets

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/grpc_service_examine_test.go` (new)

**Step 1: Write failing tests**

In `internal/gameserver/grpc_service_examine_test.go` (new file):

```go
package gameserver_test

// TestHandleExamine_PlayerTarget verifies that examine on a player in the same room
// returns a CharacterInfo event (not NpcView) with the player's region display name.
func TestHandleExamine_PlayerTarget(t *testing.T) {
    // Setup: two players in same room
    // Call handleExamine with target = other player's CharName
    // Assert: result is *gamev1.ServerEvent_CharacterInfo, not *gamev1.ServerEvent_NpcView
    // Assert: CharacterInfo.Region == sess.RegionDisplayName of target
}

func TestProperty_HandleExamine_PlayerTargetAlwaysReturnsCharacterInfo(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Property: for any valid player name in same room, result is CharacterInfo
    })
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/gameserver/... -run TestHandleExamine_Player -v`
Expected: FAIL — examine returns NpcView, not CharacterInfo for player targets

**Step 3: Update handleExamine to check for player targets first**

In `internal/gameserver/grpc_service.go`, update `handleExamine`:

```go
// handleExamine returns detailed information about a named target.
// It first checks for a player with the given name in the same room;
// if found, returns CharacterInfo. Otherwise falls back to NPC examine.
//
// Precondition: uid must be a valid connected player; req.Target must be non-empty.
// Postcondition: Returns CharacterInfo for player targets, NpcView for NPC targets, or error if not found.
func (s *GameServiceServer) handleExamine(uid string, req *gamev1.ExamineRequest) (*gamev1.ServerEvent, error) {
    // Check if target is a player in the same room.
    examiner, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }
    target, ok := s.sessions.GetPlayerByCharName(req.Target)
    if ok && target.RoomID == examiner.RoomID {
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_CharacterInfo{
                CharacterInfo: &gamev1.CharacterInfo{
                    CharacterId: target.CharacterID,
                    Name:        target.CharName,
                    Region:      target.RegionDisplayName,
                    Class:       target.Class,
                    Level:       int32(target.Level),
                    CurrentHp:   int32(target.CurrentHP),
                },
            },
        }, nil
    }

    // Fall back to NPC examine.
    view, err := s.npcH.Examine(uid, req.Target)
    if err != nil {
        return nil, err
    }
    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_NpcView{NpcView: view},
    }, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && mise run go test ./internal/gameserver/... -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_examine_test.go
git commit -m "feat: examine command supports player targets with region display"
```

---

### Task 6: Final build verification and deploy

**Step 1: Run full test suite (excluding postgres race detector deadlock)**

```bash
cd /home/cjohannsen/src/mud && mise run go list ./... | grep -v storage/postgres | xargs mise run go test -race -v 2>&1 | tail -20
```
Expected: all PASS, no failures

**Step 2: Build all binaries**

```bash
cd /home/cjohannsen/src/mud && mise run go build ./... 2>&1
```
Expected: no output (success)

**Step 3: Deploy to k8s**

```bash
cd /home/cjohannsen/src/mud && make k8s-build-push
helm upgrade mud ./deploy/mud --namespace mud --set db.password=mud
kubectl rollout status deployment/mud-gameserver -n mud
kubectl rollout status deployment/mud-devserver -n mud
```
Expected: rollout complete

**Step 4: Smoke test**

Connect via telnet and verify:
1. Character selection list shows "from the Northeast" (not "from northeast")
2. `status` command shows region with article ("the Northeast")
3. `examine <player>` returns character info with region display

**Step 5: Update FEATURES.md**

Mark region descriptions feature as complete in `docs/requirements/FEATURES.md`:
- Change `- [ ] Region descriptions instead of ID strings in character descriptions` to `- [x] Region descriptions instead of ID strings in character descriptions`

**Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add docs/requirements/FEATURES.md
git commit -m "docs: mark region descriptions feature complete"
```
