# Real Save DC Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the placeholder `dc = inst.Level + 10` formula in Grapple, Shove, Trip, Disarm, Tumble, and Demoralize handlers with real Gunchete save DCs based on NPC ability scores and proficiency ranks.

**Architecture:** Three save types — Toughness (Brutality-based, for Grapple/Shove), Hustle (Quickness-based, for Trip/Disarm/Tumble), Cool (Savvy-based, for Demoralize) — are computed as `10 + level + combat.AbilityMod(abilityScore) + skillRankBonus(proficiencyRank)`. NPC templates gain optional `toughness_rank`, `hustle_rank`, `cool_rank` YAML string fields (default `""` = untrained = 0 bonus). The three relevant ability scores (Brutality, Quickness, Savvy) and the three rank strings are copied into `npc.Instance` at spawn so handlers can read them directly without re-accessing the template.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based tests), `github.com/cory-johannsen/mud/internal/game/combat` (AbilityMod), `github.com/cory-johannsen/mud/internal/gameserver` (skillRankBonus), YAML.

---

## Background: Key Invariants

- `combat.AbilityMod(score int) int` = `(score - 10) / 2` using integer floor division. Defined in `internal/game/combat/combat.go:201`.
- `skillRankBonus(rank string) int` — `"trained"→2`, `"expert"→4`, `"master"→6`, `"legendary"→8`, default `0`. Defined in `internal/gameserver/action_handler.go:368`. Available to all `grpc_service_*.go` files since they are all in `package gameserver`.
- Toughness DC (Grapple, Shove) = `10 + inst.Level + combat.AbilityMod(inst.Brutality) + skillRankBonus(inst.ToughnessRank)`
- Hustle DC (Trip, Disarm, Tumble) = `10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)`
- Cool DC (Demoralize) = `10 + inst.Level + combat.AbilityMod(inst.Savvy) + skillRankBonus(inst.CoolRank)`
- Existing tests spawn NPCs WITHOUT setting Abilities, so `Brutality/Quickness/Savvy == 0` → `AbilityMod(0) = -5`, which would change DCs. **Fix:** update those existing test spawns to set `Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10}` so all mods are 0 and old assertions remain valid.

---

## Chunk 1: Extend npc.Template and npc.Instance

### Task 1: Add save fields to Template, Instance, and spawn

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Test: `internal/game/npc/template_test.go`

- [ ] **Step 1: Write failing tests for new Template and Instance fields**

In `internal/game/npc/template_test.go`, add:

```go
// TestTemplate_SaveRankFields_DefaultToEmpty verifies that toughness_rank,
// hustle_rank, cool_rank all default to "" when not set in YAML.
//
// Precondition: YAML has no rank fields.
// Postcondition: all three rank fields are "".
func TestTemplate_SaveRankFields_DefaultToEmpty(t *testing.T) {
	yaml := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
`
	tmpl, err := LoadTemplateFromBytes([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, "", tmpl.ToughnessRank)
	assert.Equal(t, "", tmpl.HustleRank)
	assert.Equal(t, "", tmpl.CoolRank)
}

// TestTemplate_SaveRankFields_ParseFromYAML verifies that rank fields round-trip
// through YAML parsing.
//
// Precondition: YAML specifies toughness_rank=trained, hustle_rank=expert, cool_rank=master.
// Postcondition: parsed fields equal the specified values.
func TestTemplate_SaveRankFields_ParseFromYAML(t *testing.T) {
	yaml := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
toughness_rank: trained
hustle_rank: expert
cool_rank: master
`
	tmpl, err := LoadTemplateFromBytes([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, "trained", tmpl.ToughnessRank)
	assert.Equal(t, "expert", tmpl.HustleRank)
	assert.Equal(t, "master", tmpl.CoolRank)
}

// TestInstance_SaveFields_CopiedFromTemplate verifies that Instance fields
// Brutality, Quickness, Savvy, ToughnessRank, HustleRank, CoolRank are copied
// from the template at spawn.
//
// Precondition: template has non-zero ability scores and rank fields.
// Postcondition: instance fields equal template values.
func TestInstance_SaveFields_CopiedFromTemplate(t *testing.T) {
	tmpl := &Template{
		ID: "t1", Name: "T", Level: 1, MaxHP: 10, AC: 10, Perception: 0,
		Abilities:     Abilities{Brutality: 14, Quickness: 12, Savvy: 8},
		ToughnessRank: "trained",
		HustleRank:    "expert",
		CoolRank:      "master",
	}
	inst := NewInstance("i1", tmpl, "room1")
	assert.Equal(t, 14, inst.Brutality)
	assert.Equal(t, 12, inst.Quickness)
	assert.Equal(t, 8, inst.Savvy)
	assert.Equal(t, "trained", inst.ToughnessRank)
	assert.Equal(t, "expert", inst.HustleRank)
	assert.Equal(t, "master", inst.CoolRank)
}
```

- [ ] **Step 2: Run failing tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_SaveRankFields|TestInstance_SaveFields" -v 2>&1 | tail -20
```

Expected: `FAIL` — fields don't exist yet.

- [ ] **Step 3: Add fields to Template**

In `internal/game/npc/template.go`, add three fields at the end of the `Template` struct (after the `Combat` field):

```go
// ToughnessRank is the NPC's Toughness save proficiency rank
// ("trained", "expert", "master", "legendary", or "" for untrained).
// Used to compute Toughness DC for Grapple and Shove.
ToughnessRank string `yaml:"toughness_rank"`
// HustleRank is the NPC's Hustle save proficiency rank.
// Used to compute Hustle DC for Trip, Disarm, and Tumble.
HustleRank string `yaml:"hustle_rank"`
// CoolRank is the NPC's Cool save proficiency rank.
// Used to compute Cool DC for Demoralize.
CoolRank string `yaml:"cool_rank"`
```

- [ ] **Step 4: Add fields to Instance**

In `internal/game/npc/instance.go`, add six fields after the `UseCover` field:

```go
// Brutality is copied from the template's Abilities.Brutality at spawn.
// Used to compute Toughness DC.
Brutality int
// Quickness is copied from the template's Abilities.Quickness at spawn.
// Used to compute Hustle DC.
Quickness int
// Savvy is copied from the template's Abilities.Savvy at spawn.
// Used to compute Cool DC.
Savvy int
// ToughnessRank is the Toughness save proficiency rank, copied from template.
ToughnessRank string
// HustleRank is the Hustle save proficiency rank, copied from template.
HustleRank string
// CoolRank is the Cool save proficiency rank, copied from template.
CoolRank string
```

- [ ] **Step 5: Copy fields in NewInstanceWithResolver**

In `NewInstanceWithResolver`, add the six fields to the returned `&Instance{...}` literal, after `UseCover`:

```go
Brutality:     tmpl.Abilities.Brutality,
Quickness:     tmpl.Abilities.Quickness,
Savvy:         tmpl.Abilities.Savvy,
ToughnessRank: tmpl.ToughnessRank,
HustleRank:    tmpl.HustleRank,
CoolRank:      tmpl.CoolRank,
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_SaveRankFields|TestInstance_SaveFields" -v 2>&1 | tail -20
```

Expected: all three tests `PASS`.

- [ ] **Step 7: Run full npc package test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v 2>&1 | tail -30
```

Expected: all existing tests still pass.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/template_test.go && git commit -m "feat(npc): add Brutality/Quickness/Savvy + save rank fields to Template and Instance"
```

---

## Chunk 2: Toughness DC (Grapple and Shove)

### Task 2: Replace Level+10 with Toughness DC in handleGrapple and handleShove

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (lines ~4693–4714 for grapple, ~4887–4895 for shove)
- Modify: `internal/gameserver/grpc_service_grapple_test.go`
- Modify: `internal/gameserver/grpc_service_shove_test.go`

- [ ] **Step 1: Add property test for Toughness DC formula**

In `internal/gameserver/grpc_service_grapple_test.go`, add at the end:

```go
// TestProperty_HandleGrapple_ToughnessDC_Formula verifies that the Toughness DC
// used by handleGrapple equals 10 + level + abilityMod(brutality) + rankBonus.
//
// Uses rapid to generate random level (1-20), brutality score (1-20), and rank string.
func TestProperty_HandleGrapple_ToughnessDC_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		brutality := rapid.IntRange(1, 20).Draw(rt, "brutality")
		rank := rapid.SampledFrom([]string{"", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")

		expectedMod := (brutality - 10) / 2
		expectedRankBonus := skillRankBonus(rank)
		expectedDC := 10 + level + expectedMod + expectedRankBonus

		tmpl := &npc.Template{
			ID: rt.Name() + "-grap-prop", Name: "Target", Level: level,
			MaxHP: 20, AC: 13, Perception: 5,
			Abilities:     npc.Abilities{Brutality: brutality, Quickness: 10, Savvy: 10},
			ToughnessRank: rank,
		}

		logger := zaptest.NewLogger(t)
		// Force roll so total = 100 (always succeeds) — we only care about DC label in message.
		src := &fixedDiceSource{val: 99}
		roller := dice.NewLoggedRoller(src, logger)
		svc, sessMgr, npcMgr, combatHandler := newGrappleSvcWithCombat(t, roller)

		roomID := "room_grp_prop_" + rt.Name()
		_, err := npcMgr.Spawn(tmpl, roomID)
		require.NoError(rt, err)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u_grp_prop_" + rt.Name(), Username: "F", CharName: "F",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat
		_, err = combatHandler.Attack("u_grp_prop_"+rt.Name(), "Target")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleGrapple("u_grp_prop_"+rt.Name(), &gamev1.GrappleRequest{Target: "Target"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("DC %d", expectedDC),
			"message must include computed Toughness DC")
	})
}
```

Also add `"fmt"` to imports if not already present.

- [ ] **Step 2: Run property test — expect fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestProperty_HandleGrapple_ToughnessDC_Formula" -v 2>&1 | tail -20
```

Expected: `FAIL` — currently shows `Level+10`, not `10+level+mod+rank`.

- [ ] **Step 3: Update handleGrapple in grpc_service.go**

Locate the comment `// Skill check: 1d20 + athletics bonus vs target Level+10.` (around line 4693) and replace the DC computation block:

**Old:**
```go
	// Skill check: 1d20 + athletics bonus vs target Level+10.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleGrapple: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := inst.Level + 10
```

**New:**
```go
	// Skill check: 1d20 + athletics bonus vs target Toughness DC.
	// Toughness DC = 10 + level + AbilityMod(Brutality) + proficiency rank bonus.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleGrapple: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Brutality) + skillRankBonus(inst.ToughnessRank)
```

Also update the doc comment at line ~4659 and format string at line ~4703:
- Doc: `// handleGrapple performs an athletics skill check against the target NPC's Toughness DC.`
- Format: `"Grapple (athletics Toughness DC %d): rolled %d+%d=%d"`

- [ ] **Step 4: Update handleShove the same way**

Locate handleShove (around line 4853). Make the same substitution:

- Doc comment: `// handleShove performs an athletics skill check against the target NPC's Toughness DC.`
- DC line: `dc := 10 + inst.Level + combat.AbilityMod(inst.Brutality) + skillRankBonus(inst.ToughnessRank)`
- Comment: `// Skill check: 1d20 + athletics bonus vs target Toughness DC.`
- Format: `"Shove (athletics Toughness DC %d): rolled %d+%d=%d"`

- [ ] **Step 5: Update existing grapple and shove test spawns to set neutral Abilities**

In `grpc_service_grapple_test.go` and `grpc_service_shove_test.go`, every `npc.Template{...}` that does NOT set `Abilities` needs:

```go
Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
```

added to the struct literal. This makes `AbilityMod = 0` so `dc = 10 + level + 0 + 0 = level + 10`, preserving existing assertions like `Level=5 → DC=15`.

Search for all template literals in those two test files:
```bash
grep -n "npc.Template{" internal/gameserver/grpc_service_grapple_test.go internal/gameserver/grpc_service_shove_test.go
```

Add `Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},` to each one.

- [ ] **Step 6: Run all grapple and shove tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "Grapple|Shove|Property_HandleGrapple" -v 2>&1 | tail -30
```

Expected: all tests pass including the new property test.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_grapple_test.go internal/gameserver/grpc_service_shove_test.go && git commit -m "feat(gameserver): use Toughness DC for grapple and shove"
```

---

## Chunk 3: Hustle DC (Trip, Disarm, Tumble)

### Task 3: Replace Level+10 with Hustle DC in handleTrip, handleDisarm, handleTumble

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (~4717–4773 trip, ~4775–4850 disarm, ~5048–5100 tumble)
- Modify: `internal/gameserver/grpc_service_trip_test.go`
- Modify: `internal/gameserver/grpc_service_disarm_test.go`
- Modify: `internal/gameserver/grpc_service_tumble_test.go`

- [ ] **Step 1: Add property test for Hustle DC formula**

In `internal/gameserver/grpc_service_trip_test.go`, add at the end:

```go
// TestProperty_HandleTrip_HustleDC_Formula verifies that the Hustle DC
// used by handleTrip equals 10 + level + abilityMod(quickness) + rankBonus.
func TestProperty_HandleTrip_HustleDC_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		quickness := rapid.IntRange(1, 20).Draw(rt, "quickness")
		rank := rapid.SampledFrom([]string{"", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")

		expectedMod := (quickness - 10) / 2
		expectedRankBonus := skillRankBonus(rank)
		expectedDC := 10 + level + expectedMod + expectedRankBonus

		tmpl := &npc.Template{
			ID: rt.Name() + "-trip-prop", Name: "Target", Level: level,
			MaxHP: 20, AC: 13, Perception: 5,
			Abilities:  npc.Abilities{Brutality: 10, Quickness: quickness, Savvy: 10},
			HustleRank: rank,
		}

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: 99}
		roller := dice.NewLoggedRoller(src, logger)
		svc, sessMgr, npcMgr, combatHandler := newTripSvcWithCombat(t, roller)

		roomID := "room_trip_prop_" + rt.Name()
		_, err := npcMgr.Spawn(tmpl, roomID)
		require.NoError(rt, err)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u_trip_prop_" + rt.Name(), Username: "F", CharName: "F",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat
		_, err = combatHandler.Attack("u_trip_prop_"+rt.Name(), "Target")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleTrip("u_trip_prop_"+rt.Name(), &gamev1.TripRequest{Target: "Target"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("DC %d", expectedDC))
	})
}
```

Ensure `rapid`, `fmt`, `npc`, `session`, `dice`, `gamev1` are all imported in the trip test file (check existing imports and add what's missing).

- [ ] **Step 2: Run property test — expect fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestProperty_HandleTrip_HustleDC_Formula" -v 2>&1 | tail -20
```

Expected: `FAIL`.

- [ ] **Step 3: Update handleTrip**

In `grpc_service.go`, update handleTrip (around line 4751):
- Comment: `// Skill check: 1d20 + athletics bonus vs target Hustle DC.`
- DC line: `dc := 10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)`
- Doc comment: `// handleTrip performs an athletics skill check against the target NPC's Hustle DC.`
- Format: `"Trip (athletics Hustle DC %d): rolled %d+%d=%d"`

- [ ] **Step 4: Update handleDisarm**

In `grpc_service.go`, update handleDisarm (around line 4809):
- Comment: `// Skill check: 1d20 + athletics bonus vs target Hustle DC.`
- DC line: `dc := 10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)`
- Doc comment: `// handleDisarm performs an athletics skill check against the target NPC's Hustle DC.`
- Format: `"Disarm (athletics Hustle DC %d): rolled %d+%d=%d"`

- [ ] **Step 5: Update handleTumble**

In `grpc_service.go`, update handleTumble (around line 5084):
- Comment: `// Skill check: 1d20 + acrobatics bonus vs target Hustle DC.`
- DC line: `dc := 10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)`
- Doc comment: `// handleTumble attempts to move through enemy space using Acrobatics vs Hustle DC.`
- Format: `"Tumble (acrobatics Hustle DC %d): rolled %d+%d=%d"`

- [ ] **Step 6: Update existing test spawns to set neutral Abilities**

In `grpc_service_trip_test.go`, `grpc_service_disarm_test.go`, `grpc_service_tumble_test.go`:

```bash
grep -n "npc.Template{" internal/gameserver/grpc_service_trip_test.go internal/gameserver/grpc_service_disarm_test.go internal/gameserver/grpc_service_tumble_test.go
```

Add `Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},` to each template literal.

- [ ] **Step 7: Run all trip/disarm/tumble tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "Trip|Disarm|Tumble|Property_HandleTrip" -v 2>&1 | tail -30
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_trip_test.go internal/gameserver/grpc_service_disarm_test.go internal/gameserver/grpc_service_tumble_test.go && git commit -m "feat(gameserver): use Hustle DC for trip, disarm, and tumble"
```

---

## Chunk 4: Cool DC (Demoralize) and Full Suite

### Task 4: Replace Level+10 with Cool DC in handleDemoralize

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (~4597–4650 demoralize)
- Modify: `internal/gameserver/grpc_service_mentalstate_test.go` (or wherever demoralize tests live — verify with `grep -rn "handleDemoralize\|Demoralize" internal/gameserver/ --include="*_test.go" -l`)

- [ ] **Step 1: Locate the demoralize test file**

```bash
grep -rln "Demoralize\|handleDemoralize" /home/cjohannsen/src/mud/internal/gameserver/ --include="*_test.go"
```

Note the filename(s).

- [ ] **Step 2: Add property test for Cool DC formula**

In the demoralize test file, add:

```go
// TestProperty_HandleDemoralize_CoolDC_Formula verifies that the Cool DC
// used by handleDemoralize equals 10 + level + abilityMod(savvy) + rankBonus.
func TestProperty_HandleDemoralize_CoolDC_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		savvy := rapid.IntRange(1, 20).Draw(rt, "savvy")
		rank := rapid.SampledFrom([]string{"", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")

		expectedMod := (savvy - 10) / 2
		expectedRankBonus := skillRankBonus(rank)
		expectedDC := 10 + level + expectedMod + expectedRankBonus

		tmpl := &npc.Template{
			ID: rt.Name() + "-dem-prop", Name: "Target", Level: level,
			MaxHP: 20, AC: 13, Perception: 5,
			Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: savvy},
			CoolRank:  rank,
		}

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: 99}
		roller := dice.NewLoggedRoller(src, logger)
		// Use the existing newDemoralizeXxx helper — look up the exact helper name in the test file.
		svc, sessMgr, npcMgr, combatHandler := newDemoralizeSvcWithCombat(t, roller)

		roomID := "room_dem_prop_" + rt.Name()
		_, err := npcMgr.Spawn(tmpl, roomID)
		require.NoError(rt, err)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u_dem_prop_" + rt.Name(), Username: "F", CharName: "F",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat
		_, err = combatHandler.Attack("u_dem_prop_"+rt.Name(), "Target")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleDemoralize("u_dem_prop_"+rt.Name(), &gamev1.DemoralizeRequest{Target: "Target"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("DC %d", expectedDC))
	})
}
```

**Important:** Before writing this test, read the existing demoralize test file to confirm the exact constructor name (e.g., `newDemoralizeSvcWithCombat`) and any helpers. Adapt accordingly.

- [ ] **Step 3: Run property test — expect fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestProperty_HandleDemoralize_CoolDC_Formula" -v 2>&1 | tail -20
```

Expected: `FAIL`.

- [ ] **Step 4: Update handleDemoralize in grpc_service.go**

Locate the demoralize DC computation (around line 4631–4639):

**Old:**
```go
	// Skill check: 1d20 + smooth_talk bonus vs target Level+10.
	...
	dc := inst.Level + 10
```

**New:**
```go
	// Skill check: 1d20 + smooth_talk bonus vs target Cool DC.
	// Cool DC = 10 + level + AbilityMod(Savvy) + proficiency rank bonus.
	...
	dc := 10 + inst.Level + combat.AbilityMod(inst.Savvy) + skillRankBonus(inst.CoolRank)
```

Also update:
- Doc comment (~4597): `// handleDemoralize performs a smooth_talk skill check against the target NPC's Cool DC.`
- Format string: `"Demoralize (smooth_talk Cool DC %d): rolled %d+%d=%d"`

- [ ] **Step 5: Update existing demoralize test spawns**

Add `Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},` to every `npc.Template{...}` in the demoralize test file to preserve existing `Level+10` assertions.

- [ ] **Step 6: Run all demoralize tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "Demoralize|Property_HandleDemoralize" -v 2>&1 | tail -30
```

Expected: all pass.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -40
```

Expected: 100% pass.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go && git add $(git diff --name-only | grep "_test.go") && git commit -m "feat(gameserver): use Cool DC for demoralize"
```

---

## Chunk 5: YAML Content and FEATURES.md

### Task 5: Mark feature complete and update YAML files with default ranks

**Files:**
- Modify: `docs/requirements/FEATURES.md`
- Modify (optional): All YAML files in `content/npcs/` — add `toughness_rank: ""`, `hustle_rank: ""`, `cool_rank: ""` as explicit defaults, or leave absent (YAML defaults to "" automatically).

- [ ] **Step 1: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, find the line:
```
- [ ] Replace 'Level + 10' implementation for Grapple, Trip, Reflex DC with real calculations
```
Change to:
```
- [x] Replace 'Level + 10' implementation for Grapple, Shove, Trip, Disarm, Tumble, Demoralize with real Toughness/Hustle/Cool DC calculations (10 + level + abilityMod + proficiencyRank)
```

- [ ] **Step 2: Decide on YAML updates**

The three new fields default to `""` (untrained, 0 bonus) if absent from YAML. That means for any NPC that already has `abilities` set, the DC will now correctly include the ability modifier. No additional YAML changes are strictly required.

However, for documentation clarity, you MAY add explicit `toughness_rank: ""`, `hustle_rank: ""`, `cool_rank: ""` lines to a few representative YAML files to show the pattern. This is optional.

If you choose to add them, pick 3-5 representative NPCs (e.g., `ganger.yaml`, `lieutenant.yaml`, `commissar.yaml`) and add:

```yaml
toughness_rank: ""
hustle_rank: ""
cool_rank: ""
```

after the `abilities:` block. Leave the other 40+ files alone (they'll still work correctly with the empty default).

- [ ] **Step 3: Run final full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -40
```

Expected: 100% pass.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add docs/requirements/FEATURES.md && git commit -m "docs: mark real save DC feature complete in FEATURES.md"
```
