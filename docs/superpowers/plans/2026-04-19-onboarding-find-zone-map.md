# Onboarding Find-Zone-Map Quest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auto-issue a starter quest to new characters that guides them to find and use the Felony Flats zone map terminal, teaching the map mechanic through play.

**Architecture:** Add `use_zone_map` as a new quest objective type in the quest domain layer; wire it into the existing `wireRevealZone` callback so zone map use automatically records progress. Auto-issue the quest immediately after `grantStartingInventory` succeeds, mirroring the existing `issueTechTrainerQuests` pattern.

**Tech Stack:** Go, YAML content files, existing quest service (`internal/game/quest/`), gameserver (`internal/gameserver/grpc_service.go`)

---

## File Map

| Action | File | Purpose |
|---|---|---|
| Modify | `internal/game/quest/def.go` | Add `use_zone_map` to `validObjectiveTypes`; exempt `onboarding` type from giver NPC requirement |
| Modify | `internal/game/quest/def_test.go` | Tests for new type and validation exemption |
| Modify | `internal/game/quest/registry.go` | Skip giver NPC check for `onboarding` quests; skip TargetID check for `use_zone_map` objectives |
| Modify | `internal/game/quest/registry_test.go` | Tests for registry cross-validation exemptions |
| Modify | `internal/game/quest/service.go` | Add `RecordZoneMapUse` method |
| Modify | `internal/game/quest/service_test.go` | Unit tests for `RecordZoneMapUse` |
| Modify | `internal/gameserver/grpc_service.go` | Wire `RecordZoneMapUse` into `wireRevealZone`; auto-issue quest after `grantStartingInventory` |
| Modify | `internal/gameserver/grpc_service_test.go` | Property-based test for auto-issue |
| Create | `content/quests/onboarding_find_zone_map.yaml` | Quest definition |

---

### Task 1: Add `use_zone_map` objective type and `onboarding` quest type validation

**Files:**
- Modify: `internal/game/quest/def.go`
- Modify: `internal/game/quest/def_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/quest/def_test.go`:

```go
// TestValidate_UseZoneMapObjective verifies use_zone_map objective type passes Validate.
//
// Precondition: QuestDef with use_zone_map objective, valid target_id, quantity=1.
// Postcondition: Validate returns nil.
func TestValidate_UseZoneMapObjective(t *testing.T) {
	def := QuestDef{
		ID:          "test_use_zone_map",
		Title:       "Test Quest",
		GiverNPCID:  "some_npc",
		Repeatable:  false,
		Objectives: []QuestObjective{
			{
				ID:          "use_map_obj",
				Type:        "use_zone_map",
				Description: "Use the zone map",
				TargetID:    "felony_flats",
				Quantity:    1,
			},
		},
		Rewards: QuestRewards{XP: 50},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error for use_zone_map objective: %v", err)
	}
}

// TestValidate_OnboardingQuestNoGiver verifies onboarding quests pass Validate without GiverNPCID.
//
// Precondition: QuestDef with type=onboarding, no GiverNPCID, has objectives.
// Postcondition: Validate returns nil.
func TestValidate_OnboardingQuestNoGiver(t *testing.T) {
	def := QuestDef{
		ID:    "onboarding_test",
		Title: "Onboarding Quest",
		Type:  "onboarding",
		Objectives: []QuestObjective{
			{
				ID:          "obj1",
				Type:        "use_zone_map",
				Description: "Use the zone map",
				TargetID:    "felony_flats",
				Quantity:    1,
			},
		},
		Rewards: QuestRewards{XP: 50},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error for onboarding quest with no giver: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/quest/... -run "TestValidate_UseZoneMapObjective|TestValidate_OnboardingQuestNoGiver" -v
```

Expected: FAIL — `use_zone_map` is not in `validObjectiveTypes`.

- [ ] **Step 3: Implement in `def.go`**

In `internal/game/quest/def.go`, make two changes:

**Change 1** — add `use_zone_map` to `validObjectiveTypes`:
```go
var validObjectiveTypes = map[string]bool{
	"kill": true, "fetch": true, "explore": true, "deliver": true,
	"use_zone_map": true,
}
```

**Change 2** — in `QuestDef.Validate()`, extend the `find_trainer` exemption to also cover `onboarding`:
```go
// find_trainer and onboarding quests have no NPC giver — skip those checks.
if d.Type == "find_trainer" || d.Type == "onboarding" {
    // onboarding quests still validate objectives below.
    if d.Type == "find_trainer" {
        return nil
    }
    // fall through to objective validation
} else {
    if d.GiverNPCID == "" {
        return fmt.Errorf("quest %q: GiverNPCID must not be empty", d.ID)
    }
}
```

Also add `use_zone_map` to the objective validation loop — it does not require `item_id`:
```go
// use_zone_map objectives require TargetID (zone ID) and Quantity >= 1; no item_id.
// TargetID and Quantity are already validated by the generic checks below.
```
(No special-case needed; the generic checks for `TargetID != ""` and `Quantity >= 1` already cover it, and `item_id` check is only on `deliver` type.)

The full updated `Validate()` method:
```go
func (d QuestDef) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("quest ID must not be empty")
	}
	if d.Title == "" {
		return fmt.Errorf("quest %q: Title must not be empty", d.ID)
	}
	// find_trainer quests have no NPC giver and no objectives — skip all checks.
	if d.Type == "find_trainer" {
		return nil
	}
	// onboarding quests have no NPC giver but DO have objectives.
	if d.Type != "onboarding" {
		if d.GiverNPCID == "" {
			return fmt.Errorf("quest %q: GiverNPCID must not be empty", d.ID)
		}
	}
	if len(d.Objectives) == 0 {
		return fmt.Errorf("quest %q: Objectives must not be empty", d.ID)
	}
	for _, obj := range d.Objectives {
		if obj.ID == "" {
			return fmt.Errorf("quest %q: objective ID must not be empty", d.ID)
		}
		if obj.Description == "" {
			return fmt.Errorf("quest %q objective %q: Description must not be empty", d.ID, obj.ID)
		}
		if obj.TargetID == "" {
			return fmt.Errorf("quest %q objective %q: TargetID must not be empty", d.ID, obj.ID)
		}
		if !validObjectiveTypes[obj.Type] {
			return fmt.Errorf("quest %q objective %q: invalid Type %q", d.ID, obj.ID, obj.Type)
		}
		if obj.Quantity < 1 {
			return fmt.Errorf("quest %q objective %q: Quantity must be >= 1", d.ID, obj.ID)
		}
		if obj.Type == "deliver" && obj.ItemID == "" {
			return fmt.Errorf("quest %q objective %q: deliver objective requires ItemID", d.ID, obj.ID)
		}
	}
	if !d.Repeatable && d.Cooldown != "" {
		return fmt.Errorf("quest %q: non-repeatable quest must not have Cooldown", d.ID)
	}
	if d.Cooldown != "" {
		if _, err := time.ParseDuration(d.Cooldown); err != nil {
			return fmt.Errorf("quest %q: invalid Cooldown %q: %w", d.ID, d.Cooldown, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/game/quest/... -run "TestValidate_UseZoneMapObjective|TestValidate_OnboardingQuestNoGiver" -v
```

Expected: PASS

- [ ] **Step 5: Run full quest package tests**

```bash
mise exec -- go test ./internal/game/quest/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/game/quest/def.go internal/game/quest/def_test.go
git commit -m "feat(quest): add use_zone_map objective type and onboarding quest type (#121)"
```

---

### Task 2: Update registry cross-validation for new type and quest type

**Files:**
- Modify: `internal/game/quest/registry.go`
- Modify: `internal/game/quest/registry_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/quest/registry_test.go`:

```go
// TestCrossValidate_OnboardingQuestNoNPC verifies onboarding quests pass CrossValidate without a matching NPC.
//
// Precondition: Registry with onboarding quest; empty npcIDs map.
// Postcondition: CrossValidate returns nil.
func TestCrossValidate_OnboardingQuestNoNPC(t *testing.T) {
	reg := QuestRegistry{
		"onboarding_find_zone_map": &QuestDef{
			ID:    "onboarding_find_zone_map",
			Title: "Find Your Bearings",
			Type:  "onboarding",
			Objectives: []QuestObjective{
				{ID: "explore_map_room", Type: "explore", Description: "Locate the terminal", TargetID: "flats_82nd_ave", Quantity: 1},
				{ID: "use_zone_map", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
			},
			Rewards: QuestRewards{XP: 50},
		},
	}
	npcIDs := map[string]bool{}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{"flats_82nd_ave": true}
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err != nil {
		t.Fatalf("unexpected error for onboarding CrossValidate: %v", err)
	}
}

// TestCrossValidate_UseZoneMapObjectiveNotCheckedAgainstRooms verifies use_zone_map
// target_id is not validated against roomIDs (it is a zone ID, not a room ID).
//
// Precondition: Registry with use_zone_map objective; roomIDs does not contain "felony_flats".
// Postcondition: CrossValidate returns nil.
func TestCrossValidate_UseZoneMapTargetNotValidated(t *testing.T) {
	reg := QuestRegistry{
		"onboarding_find_zone_map": &QuestDef{
			ID:    "onboarding_find_zone_map",
			Title: "Find Your Bearings",
			Type:  "onboarding",
			Objectives: []QuestObjective{
				{ID: "explore_map_room", Type: "explore", Description: "Locate the terminal", TargetID: "flats_82nd_ave", Quantity: 1},
				{ID: "use_zone_map_obj", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
			},
			Rewards: QuestRewards{XP: 50},
		},
	}
	npcIDs := map[string]bool{}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{"flats_82nd_ave": true} // "felony_flats" intentionally absent
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err != nil {
		t.Fatalf("unexpected CrossValidate error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/game/quest/... -run "TestCrossValidate_OnboardingQuestNoNPC|TestCrossValidate_UseZoneMapTargetNotValidated" -v
```

Expected: FAIL

- [ ] **Step 3: Update `registry.go`**

In `CrossValidate`, extend the NPC check exemption and add a `use_zone_map` case to the objective switch:

```go
func (r QuestRegistry) CrossValidate(npcIDs, itemIDs, roomIDs map[string]bool) error {
	for _, def := range r {
		// Skip NPC check for find_trainer and onboarding quests — they have no giver NPC.
		if def.Type != "find_trainer" && def.Type != "onboarding" && !npcIDs[def.GiverNPCID] {
			return fmt.Errorf("quest %q: GiverNPCID %q not found in NPC registry", def.ID, def.GiverNPCID)
		}
		for _, prereq := range def.Prerequisites {
			if _, ok := r[prereq]; !ok {
				return fmt.Errorf("quest %q: prerequisite quest %q not found in QuestRegistry", def.ID, prereq)
			}
		}
		for _, obj := range def.Objectives {
			switch obj.Type {
			case "kill":
				if !npcIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: kill TargetID %q not in NPC registry", def.ID, obj.ID, obj.TargetID)
				}
			case "fetch":
				if !itemIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: fetch TargetID %q not in item registry", def.ID, obj.ID, obj.TargetID)
				}
			case "explore":
				if !roomIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: explore TargetID %q not in world rooms", def.ID, obj.ID, obj.TargetID)
				}
			case "deliver":
				if !npcIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: deliver TargetID %q not in NPC registry", def.ID, obj.ID, obj.TargetID)
				}
				if !itemIDs[obj.ItemID] {
					return fmt.Errorf("quest %q objective %q: deliver ItemID %q not in item registry", def.ID, obj.ID, obj.ItemID)
				}
			case "use_zone_map":
				// TargetID is a zone ID — not validated against roomIDs or npcIDs.
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/game/quest/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/quest/registry.go internal/game/quest/registry_test.go
git commit -m "feat(quest): exempt onboarding quests and use_zone_map from registry validation (#121)"
```

---

### Task 3: Add `RecordZoneMapUse` to quest service

**Files:**
- Modify: `internal/game/quest/service.go`
- Modify: `internal/game/quest/service_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/game/quest/service_test.go`:

```go
// TestRecordZoneMapUse_CompletesObjective verifies RecordZoneMapUse increments
// use_zone_map objectives matching the given zoneID and triggers completion.
//
// Precondition: Active quest with use_zone_map objective targeting "felony_flats"; progress=0.
// Postcondition: Progress=1; quest completed; completion messages non-empty.
func TestRecordZoneMapUse_CompletesObjective(t *testing.T) {
	def := &QuestDef{
		ID:    "onboarding_find_zone_map",
		Title: "Find Your Bearings",
		Type:  "onboarding",
		Objectives: []QuestObjective{
			{ID: "use_zone_map_obj", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
		},
		Rewards: QuestRewards{XP: 50},
	}
	reg := QuestRegistry{"onboarding_find_zone_map": def}
	repo := &stubQuestRepo{}
	svc := NewService(reg, repo, nil, nil, nil)

	sess := newStubSession()
	sess.activeQuests["onboarding_find_zone_map"] = &ActiveQuest{
		QuestID:           "onboarding_find_zone_map",
		ObjectiveProgress: map[string]int{"use_zone_map_obj": 0},
	}

	msgs, err := svc.RecordZoneMapUse(context.Background(), sess, 42, "felony_flats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected completion messages, got none")
	}
	if _, stillActive := sess.activeQuests["onboarding_find_zone_map"]; stillActive {
		t.Fatal("quest should be completed and removed from activeQuests")
	}
}

// TestRecordZoneMapUse_WrongZone verifies RecordZoneMapUse does not increment
// progress when the zoneID does not match the objective target_id.
//
// Precondition: Active quest targeting "felony_flats"; RecordZoneMapUse called with "downtown".
// Postcondition: Progress unchanged; no completion messages.
func TestRecordZoneMapUse_WrongZone(t *testing.T) {
	def := &QuestDef{
		ID:    "onboarding_find_zone_map",
		Title: "Find Your Bearings",
		Type:  "onboarding",
		Objectives: []QuestObjective{
			{ID: "use_zone_map_obj", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
		},
		Rewards: QuestRewards{XP: 50},
	}
	reg := QuestRegistry{"onboarding_find_zone_map": def}
	repo := &stubQuestRepo{}
	svc := NewService(reg, repo, nil, nil, nil)

	sess := newStubSession()
	sess.activeQuests["onboarding_find_zone_map"] = &ActiveQuest{
		QuestID:           "onboarding_find_zone_map",
		ObjectiveProgress: map[string]int{"use_zone_map_obj": 0},
	}

	msgs, err := svc.RecordZoneMapUse(context.Background(), sess, 42, "downtown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no messages for wrong zone, got %v", msgs)
	}
	aq := sess.activeQuests["onboarding_find_zone_map"]
	if aq.ObjectiveProgress["use_zone_map_obj"] != 0 {
		t.Fatal("progress should be unchanged for wrong zone")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/game/quest/... -run "TestRecordZoneMapUse" -v
```

Expected: FAIL — `RecordZoneMapUse` does not exist.

- [ ] **Step 3: Implement `RecordZoneMapUse` in `service.go`**

Add after `RecordExplore`:

```go
// RecordZoneMapUse increments progress for all active use_zone_map objectives
// whose target_id matches zoneID.
//
// Precondition: sess non-nil; zoneID non-empty.
// Postcondition: matching objectives are incremented (clamped at Quantity); maybeComplete called.
// Returns completion messages if a quest completed, or nil if none completed.
func (s *Service) RecordZoneMapUse(ctx context.Context, sess SessionState, characterID int64, zoneID string) ([]string, error) {
	return s.recordProgress(ctx, sess, characterID, func(obj QuestObjective) bool {
		return obj.Type == "use_zone_map" && obj.TargetID == zoneID
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/game/quest/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/quest/service.go internal/game/quest/service_test.go
git commit -m "feat(quest): add RecordZoneMapUse for use_zone_map objective tracking (#121)"
```

---

### Task 4: Wire `RecordZoneMapUse` into `wireRevealZone` and auto-issue quest on character creation

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Update `wireRevealZone` to call `RecordZoneMapUse`**

In `grpc_service.go`, find `wireRevealZone` (around line 7322) and add the quest recording after the automap bulk insert:

```go
func (s *GameServiceServer) wireRevealZone() {
	if s.scriptMgr == nil {
		return
	}
	s.scriptMgr.RevealZoneMap = func(uid, zoneID string) {
		zone, ok := s.world.GetZone(zoneID)
		if !ok {
			return
		}
		sess, ok := s.sessions.GetPlayer(uid)
		if !ok {
			return
		}
		if sess.AutomapCache[zoneID] == nil {
			sess.AutomapCache[zoneID] = make(map[string]bool)
		}
		roomIDs := make([]string, 0, len(zone.Rooms))
		for roomID := range zone.Rooms {
			if !sess.AutomapCache[zoneID][roomID] {
				sess.AutomapCache[zoneID][roomID] = true
				roomIDs = append(roomIDs, roomID)
			}
		}
		if s.automapRepo != nil && len(roomIDs) > 0 {
			if err := s.automapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, roomIDs, false); err != nil {
				s.logger.Warn("reveal_zone: bulk insert automap", zap.Error(err))
			}
		}
		// Record zone map use for quest progress.
		if s.questSvc != nil && sess.CharacterID > 0 {
			msgs, err := s.questSvc.RecordZoneMapUse(context.Background(), sess, sess.CharacterID, zoneID)
			if err != nil {
				s.logger.Warn("reveal_zone: RecordZoneMapUse failed",
					zap.String("uid", uid),
					zap.String("zone_id", zoneID),
					zap.Error(err),
				)
			}
			for _, msg := range msgs {
				_ = s.sendEvent(uid, messageEvent(msg))
			}
		}
	}
}
```

- [ ] **Step 2: Add `issueOnboardingQuest` helper method**

Add a new private method near `issueTechTrainerQuests` (around line 12085):

```go
// issueOnboardingQuest auto-grants the onboarding_find_zone_map quest for a new character.
//
// Precondition: sess non-nil; uid non-empty.
// Postcondition: If quest not already active/completed, it is accepted and a console message sent.
// Errors are logged at Warn and do not abort the caller.
func (s *GameServiceServer) issueOnboardingQuest(ctx context.Context, uid string, sess *session.PlayerSession) {
	if s.questSvc == nil || sess.CharacterID <= 0 {
		return
	}
	const questID = "onboarding_find_zone_map"
	// Skip if already active or completed (migration edge case).
	if _, active := sess.GetActiveQuests()[questID]; active {
		return
	}
	if _, done := sess.GetCompletedQuests()[questID]; done {
		return
	}
	if _, _, err := s.questSvc.Accept(ctx, sess, sess.CharacterID, questID); err != nil {
		s.logger.Warn("issueOnboardingQuest: Accept failed",
			zap.String("uid", uid),
			zap.String("quest_id", questID),
			zap.Error(err),
		)
		return
	}
	_ = s.sendEvent(uid, messageEvent("New quest: Find Your Bearings — locate the district map terminal on 82nd Avenue."))
}
```

- [ ] **Step 3: Call `issueOnboardingQuest` after `grantStartingInventory`**

Find the `grantStartingInventory` call site (around line 1284) and add the onboarding quest issue immediately after the successful grant:

```go
if grantErr := s.grantStartingInventory(stream.Context(), sess, characterID, archetype, team, jobOverride); grantErr != nil {
    s.logger.Error("failed to grant starting inventory",
        zap.String("uid", uid),
        zap.Int64("character_id", characterID),
        zap.Error(grantErr),
    )
} else {
    // Issue onboarding quest for new characters.
    s.issueOnboardingQuest(stream.Context(), uid, sess)
}
```

- [ ] **Step 4: Build to check for compilation errors**

```bash
mise exec -- go build ./internal/gameserver/...
```

Expected: no errors.

- [ ] **Step 5: Run gameserver tests**

```bash
mise exec -- go test ./internal/gameserver/... -timeout 120s
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): wire RecordZoneMapUse and auto-issue onboarding quest on character creation (#121)"
```

---

### Task 5: Create the quest YAML content file

**Files:**
- Create: `content/quests/onboarding_find_zone_map.yaml`

- [ ] **Step 1: Create the quest YAML**

```yaml
id: onboarding_find_zone_map
title: "Find Your Bearings"
description: >
  You've been dropped into Felony Flats with nothing but instinct and the
  clothes on your back. Every survivor in this district knows one thing:
  you need a map. There's a district terminal on 82nd Avenue that has a
  zone map. Find it and figure out how to use it.
type: onboarding
auto_complete: true
repeatable: false
objectives:
  - id: explore_map_room
    type: explore
    description: Locate the zone map terminal on 82nd Avenue
    target_id: flats_82nd_ave
    quantity: 1
  - id: use_zone_map
    type: use_zone_map
    description: Use the zone map terminal
    target_id: felony_flats
    quantity: 1
rewards:
  xp: 50
  credits: 0
```

- [ ] **Step 2: Run full test suite to ensure YAML loads cleanly**

```bash
mise exec -- go test ./... -timeout 180s
```

Expected: all PASS (the quest registry loads and validates the new YAML at startup).

- [ ] **Step 3: Commit**

```bash
git add content/quests/onboarding_find_zone_map.yaml
git commit -m "content(quest): add onboarding_find_zone_map quest definition (#121)"
```

---

### Task 6: Property-based test for auto-issue idempotency

**Files:**
- Modify: `internal/gameserver/grpc_service_test.go` (or a new focused test file)

- [ ] **Step 1: Write the property test**

Add to the gameserver test file (or create `internal/gameserver/grpc_service_onboarding_test.go`):

```go
// TestProperty_IssueOnboardingQuest_IdempotentOnRepeat verifies that calling
// issueOnboardingQuest multiple times on the same session never returns an error
// and only issues the quest once.
//
// Precondition: Session with questSvc wired; characterID > 0.
// Postcondition: Quest appears exactly once in activeQuests after multiple calls.
func TestProperty_IssueOnboardingQuest_IdempotentOnRepeat(t *testing.T) {
	// Build minimal service with quest svc loaded.
	// Use the actual quest registry loaded from content/ so the YAML is validated.
	// This is an integration-style property test — it requires the quest registry to load.
	// If content/quests/onboarding_find_zone_map.yaml is missing this test will fail at load.
	reg, err := quest.LoadRegistry("../../content/quests")
	if err != nil {
		t.Fatalf("failed to load quest registry: %v", err)
	}
	repo := &stubQuestRepo{}
	svc := quest.NewService(reg, repo, nil, nil, nil)

	sess := session.NewPlayerSession()
	sess.CharacterID = 1

	s := &GameServiceServer{questSvc: svc}

	// Call three times — only the first should issue.
	for i := 0; i < 3; i++ {
		s.issueOnboardingQuest(context.Background(), "uid-test", sess)
	}

	count := 0
	for id := range sess.GetActiveQuests() {
		if id == "onboarding_find_zone_map" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected quest issued exactly once, got %d active entries", count)
	}
}
```

- [ ] **Step 2: Run the property test**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestProperty_IssueOnboardingQuest_IdempotentOnRepeat" -v
```

Expected: PASS

- [ ] **Step 3: Run full test suite**

```bash
mise exec -- go test ./... -timeout 180s
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service_onboarding_test.go
git commit -m "test(gameserver): property test for onboarding quest idempotency (#121)"
```

---

## Spec Coverage Checklist

| Requirement | Task |
|---|---|
| REQ-OBQ-1: Quest YAML | Task 5 |
| REQ-OBQ-2: `use_zone_map` objective type | Task 1 |
| REQ-OBQ-3: Validation for `use_zone_map` | Task 1 |
| REQ-OBQ-4: `RecordZoneMapUse` | Task 3 |
| REQ-OBQ-5: Wire into `wireRevealZone` | Task 4 |
| REQ-OBQ-6: `onboarding` type exemption | Task 1 |
| REQ-OBQ-7: Auto-issue on character creation | Task 4 |
| REQ-OBQ-8: Registry cross-validation exemption | Task 2 |
| REQ-OBQ-9a: `RecordZoneMapUse` unit tests | Task 3 |
| REQ-OBQ-9b: `onboarding` Validate unit test | Task 1 |
| REQ-OBQ-9c: Registry CrossValidate unit test | Task 2 |
| REQ-OBQ-9d: Property-based auto-issue test | Task 6 |
