package handlers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestRenderRoomView(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "test_room",
		Title:       "Test Room",
		Description: "A dusty chamber.",
		Exits: []*gamev1.ExitInfo{
			{Direction: "north", TargetRoomId: "other"},
			{Direction: "east", TargetRoomId: "another", Locked: true},
		},
		Players: []string{"Bob"},
	}

	rendered := RenderRoomView(rv)
	stripped := telnet.StripANSI(rendered)

	assert.Contains(t, stripped, "Test Room")
	assert.Contains(t, stripped, "dusty chamber")
	assert.Contains(t, stripped, "north")
	assert.Contains(t, stripped, "east (locked)")
	assert.Contains(t, stripped, "Bob")
}

func TestRenderRoomView_NoExitsNoPlayers(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "empty",
		Title:       "Empty Room",
		Description: "Nothing here.",
	}

	rendered := RenderRoomView(rv)
	stripped := telnet.StripANSI(rendered)

	assert.Contains(t, stripped, "Empty Room")
	assert.NotContains(t, stripped, "Exits")
	assert.NotContains(t, stripped, "Also here")
}

func TestRenderMessage_Say(t *testing.T) {
	msg := &gamev1.MessageEvent{
		Sender:  "Alice",
		Content: "hello world",
		Type:    gamev1.MessageType_MESSAGE_TYPE_SAY,
	}
	stripped := telnet.StripANSI(RenderMessage(msg))
	assert.Contains(t, stripped, "Alice says: hello world")
}

func TestRenderMessage_Emote(t *testing.T) {
	msg := &gamev1.MessageEvent{
		Sender:  "Alice",
		Content: "waves",
		Type:    gamev1.MessageType_MESSAGE_TYPE_EMOTE,
	}
	stripped := telnet.StripANSI(RenderMessage(msg))
	assert.Contains(t, stripped, "Alice waves")
}

func TestRenderRoomEvent_Arrive(t *testing.T) {
	evt := &gamev1.RoomEvent{
		Player:    "Bob",
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
		Direction: "south",
	}
	stripped := telnet.StripANSI(RenderRoomEvent(evt))
	assert.Contains(t, stripped, "Bob arrived from the south")
}

func TestRenderRoomEvent_Depart(t *testing.T) {
	evt := &gamev1.RoomEvent{
		Player:    "Bob",
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
		Direction: "north",
	}
	stripped := telnet.StripANSI(RenderRoomEvent(evt))
	assert.Contains(t, stripped, "Bob left to the north")
}

func TestRenderRoomEvent_ArriveNoDirection(t *testing.T) {
	evt := &gamev1.RoomEvent{
		Player: "Bob",
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	}
	stripped := telnet.StripANSI(RenderRoomEvent(evt))
	assert.Contains(t, stripped, "Bob has arrived")
}

func TestRenderPlayerList(t *testing.T) {
	pl := &gamev1.PlayerList{
		Players: []*gamev1.PlayerInfo{
			{Name: "Alice", Level: 2, Job: "Striker (Melee)", HealthLabel: "Healthy", Status: gamev1.CombatStatus_COMBAT_STATUS_IDLE},
			{Name: "Bob", Level: 5, Job: "Boot (Gun)", HealthLabel: "Wounded", Status: gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT},
		},
	}
	stripped := telnet.StripANSI(RenderPlayerList(pl))
	assert.Contains(t, stripped, "Alice")
	assert.Contains(t, stripped, "Bob")
}

func TestRenderPlayerList_Empty(t *testing.T) {
	pl := &gamev1.PlayerList{}
	stripped := telnet.StripANSI(RenderPlayerList(pl))
	assert.Contains(t, stripped, "Nobody else")
}

func TestRenderPlayerList_EmptyList(t *testing.T) {
	pl := &gamev1.PlayerList{RoomTitle: "room1", Players: nil}
	result := RenderPlayerList(pl)
	assert.Contains(t, result, "Nobody")
}

func TestRenderPlayerList_ShowsLevelAndJob(t *testing.T) {
	pl := &gamev1.PlayerList{
		RoomTitle: "room1",
		Players: []*gamev1.PlayerInfo{
			{Name: "Raze", Level: 3, Job: "Striker (Gun)", HealthLabel: "Wounded", Status: gamev1.CombatStatus_COMBAT_STATUS_IDLE},
		},
	}
	result := RenderPlayerList(pl)
	assert.Contains(t, result, "Raze")
	assert.Contains(t, result, "Lvl 3")
	assert.Contains(t, result, "Striker (Gun)")
}

func TestRenderPlayerList_ShowsHealthLabel(t *testing.T) {
	pl := &gamev1.PlayerList{
		RoomTitle: "room1",
		Players: []*gamev1.PlayerInfo{
			{Name: "Ash", Level: 1, Job: "Boot (Gun)", HealthLabel: "Near Death", Status: gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT},
		},
	}
	result := RenderPlayerList(pl)
	assert.Contains(t, result, "Near Death")
	assert.Contains(t, result, "In Combat")
}

func TestRenderExitList(t *testing.T) {
	el := &gamev1.ExitList{
		Exits: []*gamev1.ExitInfo{
			{Direction: "north"},
			{Direction: "east", Locked: true},
		},
	}
	stripped := telnet.StripANSI(RenderExitList(el))
	assert.Contains(t, stripped, "north")
	assert.Contains(t, stripped, "east")
	assert.Contains(t, stripped, "(locked)")
}

func TestRenderExitList_Empty(t *testing.T) {
	el := &gamev1.ExitList{}
	stripped := telnet.StripANSI(RenderExitList(el))
	assert.Contains(t, stripped, "no obvious exits")
}

func TestRenderError(t *testing.T) {
	ee := &gamev1.ErrorEvent{Message: "something went wrong"}
	stripped := telnet.StripANSI(RenderError(ee))
	assert.Equal(t, "something went wrong", stripped)
}

func TestRenderRoundStartEvent(t *testing.T) {
	evt := &gamev1.RoundStartEvent{
		Round: 1, ActionsPerTurn: 3, DurationMs: 6000,
		TurnOrder: []string{"Alice", "Ganger"},
	}
	result := RenderRoundStartEvent(evt)
	assert.Contains(t, result, "Round 1")
	assert.Contains(t, result, "Actions: 3")
	assert.Contains(t, result, "6s")
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Ganger")
}

func TestRenderRoundEndEvent(t *testing.T) {
	evt := &gamev1.RoundEndEvent{Round: 2}
	result := RenderRoundEndEvent(evt)
	assert.Contains(t, result, "Round 2")
	assert.Contains(t, result, "resolved")
}

func TestRenderConditionEvent_Applied(t *testing.T) {
	ce := &gamev1.ConditionEvent{
		TargetName:    "Alice",
		ConditionName: "Prone",
		ConditionId:   "prone",
		Stacks:        1,
		Applied:       true,
	}
	result := RenderConditionEvent(ce)
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Prone")
	assert.Contains(t, result, "CONDITION")
}

func TestRenderConditionEvent_Removed(t *testing.T) {
	ce := &gamev1.ConditionEvent{
		TargetName:    "Alice",
		ConditionName: "Frightened",
		ConditionId:   "frightened",
		Stacks:        0,
		Applied:       false,
	}
	result := RenderConditionEvent(ce)
	assert.Contains(t, result, "fades")
	assert.Contains(t, result, "Alice")
}

// TestProperty_RenderConditionEvent_Applied verifies that for any non-empty target name
// and condition name, RenderConditionEvent with Applied=true returns a non-empty string
// containing the target name.
func TestProperty_RenderConditionEvent_Applied(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		target := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "target")
		condition := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "condition")
		stacks := rapid.Int32Range(1, 10).Draw(rt, "stacks")

		ce := &gamev1.ConditionEvent{
			TargetName:    target,
			ConditionName: condition,
			ConditionId:   "test",
			Stacks:        stacks,
			Applied:       true,
		}
		result := RenderConditionEvent(ce)
		assert.NotEmpty(rt, result)
		assert.Contains(rt, telnet.StripANSI(result), target)
	})
}

// TestProperty_RenderConditionEvent_Removed verifies that for any non-empty target name
// and condition name, RenderConditionEvent with Applied=false returns a string containing "fades".
func TestProperty_RenderConditionEvent_Removed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		target := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "target")
		condition := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "condition")

		ce := &gamev1.ConditionEvent{
			TargetName:    target,
			ConditionName: condition,
			ConditionId:   "test",
			Stacks:        0,
			Applied:       false,
		}
		result := RenderConditionEvent(ce)
		assert.Contains(rt, telnet.StripANSI(result), "fades")
	})
}

func TestRenderCharacterInfo(t *testing.T) {
	ci := &gamev1.CharacterInfo{
		Name: "Hero", Region: "the Northeast", Class: "Gunner", Level: 3,
		CurrentHp: 15, MaxHp: 20,
	}
	got := RenderCharacterInfo(ci)
	if !strings.Contains(got, "the Northeast") {
		t.Errorf("expected 'the Northeast' in %q", got)
	}
	if !strings.Contains(got, "Hero") {
		t.Errorf("expected 'Hero' in %q", got)
	}
	if !strings.Contains(got, "Gunner") {
		t.Errorf("expected 'Gunner' in %q", got)
	}
}

func TestRenderRoomView_WithTimeFields_DescriptionPreserved(t *testing.T) {
	description := "A wide open field. The sky burns orange and red as the sun sinks toward the horizon."
	rv := &gamev1.RoomView{
		RoomId:      "room1",
		Title:       "Open Field",
		Description: description,
		Period:      "Dusk",
		Hour:        17,
	}
	rendered := RenderRoomView(rv)
	if !strings.Contains(rendered, description) {
		t.Errorf("expected description in render, got %q", rendered)
	}
}

func TestRenderRoomView_TimeFields_NoExtraOutput(t *testing.T) {
	// Verify that Hour and Period fields don't add unexpected content
	rv1 := &gamev1.RoomView{
		RoomId:      "room1",
		Title:       "A Room",
		Description: "A description.",
	}
	rv2 := &gamev1.RoomView{
		RoomId:      "room1",
		Title:       "A Room",
		Description: "A description.",
		Period:      "Dusk",
		Hour:        17,
	}
	// RenderRoomView should produce the same output regardless of Hour/Period
	if RenderRoomView(rv1) != RenderRoomView(rv2) {
		t.Error("RenderRoomView should not include Hour/Period fields in output")
	}
}

func TestProperty_RenderPlayerList_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		statusVal := rapid.SampledFrom([]gamev1.CombatStatus{
			gamev1.CombatStatus_COMBAT_STATUS_UNSPECIFIED,
			gamev1.CombatStatus_COMBAT_STATUS_IDLE,
			gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT,
			gamev1.CombatStatus_COMBAT_STATUS_RESTING,
			gamev1.CombatStatus_COMBAT_STATUS_UNCONSCIOUS,
		}).Draw(rt, "status")
		players := rapid.SliceOf(rapid.Custom(func(rt *rapid.T) *gamev1.PlayerInfo {
			return &gamev1.PlayerInfo{
				Name:        rapid.String().Draw(rt, "name"),
				Level:       rapid.Int32().Draw(rt, "level"),
				Job:         rapid.String().Draw(rt, "job"),
				HealthLabel: rapid.String().Draw(rt, "healthLabel"),
				Status:      statusVal,
			}
		})).Draw(rt, "players")
		pl := &gamev1.PlayerList{RoomTitle: "room", Players: players}
		result := RenderPlayerList(pl)
		if len(players) == 0 {
			assert.Contains(rt, result, "Nobody")
		} else {
			assert.NotEmpty(rt, result)
		}
	})
}

func TestProperty_RenderCharacterInfo_NonEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "name")
		region := rapid.StringMatching(`[A-Za-z ]+`).Draw(t, "region")
		ci := &gamev1.CharacterInfo{
			Name: name, Region: region, Class: "Gunner", Level: 1, CurrentHp: 10, MaxHp: 10,
		}
		got := RenderCharacterInfo(ci)
		if got == "" {
			t.Fatal("RenderCharacterInfo must not return empty string")
		}
		if !strings.Contains(got, region) {
			t.Fatalf("RenderCharacterInfo must contain region %q in %q", region, got)
		}
	})
}

func TestRenderMap_Nil_ReturnsNoMapData(t *testing.T) {
	result := RenderMap(nil)
	require.Contains(t, result, "No map data")
}

func TestRenderMap_Empty_ReturnsNoMapData(t *testing.T) {
	result := RenderMap(&gamev1.MapResponse{})
	require.Contains(t, result, "No map data")
}

func TestRenderMap_SingleRoom_Current(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Start Room", X: 0, Y: 0, Current: true},
		},
	}
	result := RenderMap(resp)
	require.Contains(t, result, "< 1>")
	require.Contains(t, result, "Start Room")
}

func TestRenderMap_TwoRooms_DistinguishesCurrentFromDiscovered(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Room A", X: 0, Y: 0, Current: false},
			{RoomId: "r2", RoomName: "Room B", X: 2, Y: 0, Current: true, Exits: []string{}},
		},
	}
	result := RenderMap(resp)
	// Current room uses angle brackets, discovered uses square brackets.
	require.Contains(t, result, "< 2>")
	require.Contains(t, result, "[ 1]")
}

func TestProperty_RenderMap_NeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(t, "n")
		tiles := make([]*gamev1.MapTile, n)
		for i := range tiles {
			tiles[i] = &gamev1.MapTile{
				RoomId:   fmt.Sprintf("r%d", i),
				RoomName: fmt.Sprintf("Room %d", i),
				X:        int32(rapid.IntRange(-10, 10).Draw(t, fmt.Sprintf("x%d", i))),
				Y:        int32(rapid.IntRange(-10, 10).Draw(t, fmt.Sprintf("y%d", i))),
				Current:  i == 0 && n > 0,
			}
		}
		resp := &gamev1.MapResponse{Tiles: tiles}
		result := RenderMap(resp)
		if result == "" {
			t.Fatal("RenderMap returned empty string")
		}
		if n > 0 {
			// Must contain at least one room marker (angle or square brackets with number).
			hasMarker := strings.Contains(result, "<") || strings.Contains(result, "[")
			if !hasMarker {
				t.Fatal("RenderMap with tiles must contain a room marker")
			}
		}
		// Only assert current marker appears when the current tile has unique coordinates —
		// if another tile shares the same (x,y), byCoord will retain only one entry
		// and the current marker may be displaced.
		if n > 0 && tiles[0].Current {
			currentUnique := true
			for i := 1; i < n; i++ {
				if tiles[i].X == tiles[0].X && tiles[i].Y == tiles[0].Y {
					currentUnique = false
					break
				}
			}
			if currentUnique && !strings.Contains(result, "<") {
				t.Fatal("current tile must use angle-bracket marker")
			}
		}
	})
}

func TestRenderCharacterSheet_Skills(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:  "Test",
		Level: 1,
		Skills: []*gamev1.SkillEntry{
			{SkillId: "acrobatics", Name: "Acrobatics", Ability: "QCK", Proficiency: "trained"},
			{SkillId: "athletics", Name: "Athletics", Ability: "BRT", Proficiency: "untrained"},
			{SkillId: "stealth", Name: "Stealth", Ability: "QCK", Proficiency: "expert"},
			{SkillId: "deception", Name: "Deception", Ability: "FLR", Proficiency: "master"},
			{SkillId: "diplomacy", Name: "Diplomacy", Ability: "FLR", Proficiency: "legendary"},
		},
	}
	result := RenderCharacterSheet(view)
	assert.Contains(t, result, "Skills")
	assert.Contains(t, result, "Acrobatics")
	assert.Contains(t, result, "trained")
	assert.Contains(t, result, "untrained")
	assert.Contains(t, result, "expert")
	assert.Contains(t, result, "master")
	assert.Contains(t, result, "legendary")
}

// TestProficiencyColor_KnownRanks verifies that each known rank is wrapped in ANSI escape codes
// and that the rank string is preserved in the output.
func TestProficiencyColor_KnownRanks(t *testing.T) {
	knownRanks := []string{"legendary", "master", "expert", "trained"}
	for _, rank := range knownRanks {
		result := proficiencyColor(rank)
		assert.Contains(t, result, rank, "output must contain rank string for %q", rank)
		assert.Contains(t, result, "\033[", "output must contain ANSI escape for known rank %q", rank)
	}
}

// TestProficiencyColor_UnknownRanks verifies that unknown ranks (including untrained and empty)
// are returned unchanged with no ANSI codes.
func TestProficiencyColor_UnknownRanks(t *testing.T) {
	unknownRanks := []string{"untrained", "", "novice", "journeyman", "random"}
	for _, rank := range unknownRanks {
		result := proficiencyColor(rank)
		assert.Equal(t, rank, result, "unknown rank %q must be returned unchanged", rank)
		assert.NotContains(t, result, "\033[", "unknown rank %q must not contain ANSI codes", rank)
	}
}

// TestProperty_ProficiencyColor verifies three properties:
// 1. For any input not in {legendary, master, expert, trained}, the function returns the raw input unchanged.
// 2. For each known rank, the output contains the rank string and starts with an ANSI escape sequence.
// 3. Case-insensitive: "TRAINED", "trained", and "Trained" all produce the same output.
func TestProperty_ProficiencyColor(t *testing.T) {
	knownRanks := map[string]bool{
		"legendary": true,
		"master":    true,
		"expert":    true,
		"trained":   true,
	}

	// Property 1: unknown inputs are returned unchanged (no ANSI codes).
	rapid.Check(t, func(rt *rapid.T) {
		// Generate strings that are NOT one of the known ranks (case-insensitively).
		input := rapid.StringMatching(`[a-zA-Z0-9 _-]{0,20}`).Draw(rt, "input")
		if knownRanks[strings.ToLower(input)] {
			return // skip known ranks in this property
		}
		result := proficiencyColor(input)
		if result != input {
			rt.Fatalf("proficiencyColor(%q) = %q; want input unchanged", input, result)
		}
		if strings.Contains(result, "\033[") {
			rt.Fatalf("proficiencyColor(%q) contains ANSI escape; expected none for unknown rank", input)
		}
	})

	// Property 2: known ranks produce ANSI-wrapped output containing the rank string.
	rapid.Check(t, func(rt *rapid.T) {
		rank := rapid.SampledFrom([]string{"legendary", "master", "expert", "trained"}).Draw(rt, "rank")
		result := proficiencyColor(rank)
		if !strings.Contains(result, rank) {
			rt.Fatalf("proficiencyColor(%q) = %q; does not contain rank string", rank, result)
		}
		if !strings.Contains(result, "\033[") {
			rt.Fatalf("proficiencyColor(%q) = %q; expected ANSI escape sequence", rank, result)
		}
	})

	// Property 3: case-insensitive — mixed-case variants of known ranks produce identical output.
	rapid.Check(t, func(rt *rapid.T) {
		rank := rapid.SampledFrom([]string{"legendary", "master", "expert", "trained"}).Draw(rt, "rank")
		upper := strings.ToUpper(rank)
		title := strings.ToUpper(rank[:1]) + rank[1:]
		lower := rank

		resultLower := proficiencyColor(lower)
		resultUpper := proficiencyColor(upper)
		resultTitle := proficiencyColor(title)

		if resultLower != resultUpper {
			rt.Fatalf("proficiencyColor(%q) != proficiencyColor(%q): %q vs %q", lower, upper, resultLower, resultUpper)
		}
		if resultLower != resultTitle {
			rt.Fatalf("proficiencyColor(%q) != proficiencyColor(%q): %q vs %q", lower, title, resultLower, resultTitle)
		}
	})
}

func TestRenderCharacterSheet_Feats(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:  "Test",
		Level: 1,
		Feats: []*gamev1.FeatEntry{
			{FeatId: "iron_will", Name: "Iron Will", Active: false, Description: "Bonus to Will saves."},
			{FeatId: "power_attack", Name: "Power Attack", Active: true, ActivateText: "Strike hard.", Description: "Deal extra damage."},
		},
	}
	result := RenderCharacterSheet(view)
	stripped := telnet.StripANSI(result)
	assert.Contains(t, stripped, "Feats")
	assert.Contains(t, stripped, "Iron Will")
	assert.Contains(t, stripped, "Power Attack")
	assert.Contains(t, stripped, "[active]")
	// Only Power Attack is active — Iron Will must not be followed by [active]
	assert.NotContains(t, stripped, "Iron Will [active]")
}

// TestProperty_RenderCharacterSheet_Feats verifies two properties:
// 1. For any feat with Active=true, the rendered output contains "[active]".
// 2. For a sheet containing only inactive feats, the output does not contain "[active]".
func TestProperty_RenderCharacterSheet_Feats(t *testing.T) {
	// Property 1: at least one active feat => output contains "[active]".
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z ]{1,20}`).Draw(rt, "name")
		feat := &gamev1.FeatEntry{
			FeatId: "feat_id",
			Name:   name,
			Active: true,
		}
		view := &gamev1.CharacterSheetView{
			Name:  "Hero",
			Level: 1,
			Feats: []*gamev1.FeatEntry{feat},
		}
		result := telnet.StripANSI(RenderCharacterSheet(view))
		if !strings.Contains(result, "[active]") {
			rt.Fatalf("active feat %q: output does not contain [active]; got:\n%s", name, result)
		}
	})

	// Property 2: all inactive feats => output does not contain "[active]".
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z ]{1,20}`).Draw(rt, "name")
		feat := &gamev1.FeatEntry{
			FeatId: "feat_id",
			Name:   name,
			Active: false,
		}
		view := &gamev1.CharacterSheetView{
			Name:  "Hero",
			Level: 1,
			Feats: []*gamev1.FeatEntry{feat},
		}
		result := telnet.StripANSI(RenderCharacterSheet(view))
		if strings.Contains(result, "[active]") {
			rt.Fatalf("inactive feat %q: output must not contain [active]; got:\n%s", name, result)
		}
	})
}

func TestRenderSkillsResponse_GroupedByAbility(t *testing.T) {
	sr := &gamev1.SkillsResponse{
		Skills: []*gamev1.SkillEntry{
			{SkillId: "parkour", Name: "Parkour", Ability: "quickness", Proficiency: "trained"},
			{SkillId: "muscle", Name: "Muscle", Ability: "brutality", Proficiency: "untrained"},
		},
	}
	out := RenderSkillsResponse(sr)
	if !strings.Contains(out, "Quickness") {
		t.Error("expected Quickness section")
	}
	if !strings.Contains(out, "Parkour") {
		t.Error("expected Parkour skill")
	}
	if !strings.Contains(out, "Brutality") {
		t.Error("expected Brutality section")
	}
}
