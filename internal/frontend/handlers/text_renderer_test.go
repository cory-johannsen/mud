package handlers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var testDT = gameserver.GameDateTime{Hour: 7, Day: 1, Month: 1}

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

	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)

	assert.Contains(t, stripped, "Test Room")
	assert.Contains(t, stripped, "dusty chamber")
	assert.Contains(t, stripped, "north")
	assert.Contains(t, stripped, "east*")
	assert.Contains(t, stripped, "Bob")
}

func TestRenderRoomView_ExitsLabelInline(t *testing.T) {
	rv := &gamev1.RoomView{
		Title: "Test Room",
		Exits: []*gamev1.ExitInfo{
			{Direction: "north", TargetTitle: "Alley"},
		},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	lines := strings.Split(strings.TrimRight(rendered, "\r\n"), "\r\n")
	var exitsLine string
	for _, l := range lines {
		if strings.Contains(telnet.StripANSI(l), "Exits:") {
			exitsLine = l
			break
		}
	}
	require.NotEmpty(t, exitsLine, "must have an Exits line")
	stripped := telnet.StripANSI(exitsLine)
	assert.Contains(t, stripped, "north", "first exit must be on same line as Exits label")
}

func TestRenderRoomView_ExitsFourPerRow(t *testing.T) {
	rv := &gamev1.RoomView{
		Title: "Test Room",
		Exits: []*gamev1.ExitInfo{
			{Direction: "north"}, {Direction: "south"},
			{Direction: "east"}, {Direction: "west"},
			{Direction: "up"},
		},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	// Count lines that contain exit directions after the label
	exitDirs := map[string]bool{"north": true, "south": true, "east": true, "west": true, "up": true}
	var exitRowCount int
	for _, l := range strings.Split(strings.TrimRight(rendered, "\r\n"), "\r\n") {
		stripped := telnet.StripANSI(l)
		if strings.Contains(stripped, "Exits:") {
			exitRowCount++
			continue
		}
		if exitRowCount > 0 {
			hasExit := false
			for dir := range exitDirs {
				if strings.Contains(stripped, dir) {
					hasExit = true
					break
				}
			}
			if hasExit {
				exitRowCount++
			} else {
				break
			}
		}
	}
	// 5 exits at 4/row: first row has label+"north south east west", second row has "up"
	// But since label is INLINE on first row, exitRowCount counts:
	// 1 for the "Exits: north south east west" line
	// 1 for the "       up" line
	// Total = 2
	assert.Equal(t, 2, exitRowCount, "5 exits should produce 2 rows (4+1)")
}

func TestRenderRoomView_NoExitsNoPlayers(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "empty",
		Title:       "Empty Room",
		Description: "Nothing here.",
	}

	rendered := RenderRoomView(rv, 80, 0, testDT, "")
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
	stripped := telnet.StripANSI(RenderMessage(msg, ""))
	assert.Contains(t, stripped, "Alice says: hello world")
}

func TestRenderMessage_Emote(t *testing.T) {
	msg := &gamev1.MessageEvent{
		Sender:  "Alice",
		Content: "waves",
		Type:    gamev1.MessageType_MESSAGE_TYPE_EMOTE,
	}
	stripped := telnet.StripANSI(RenderMessage(msg, ""))
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

func TestRenderCharacterSheet_ShowsGender(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:   "Ash",
		Job:    "gunslinger",
		Gender: "non-binary",
		Level:  1,
	}
	result := RenderCharacterSheet(csv, 80)
	if !strings.Contains(result, "Non-binary") {
		t.Errorf("expected 'Non-binary' in rendered sheet, got:\n%s", result)
	}
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
	// Use a wide width so the description is not wrapped and appears verbatim.
	dt := gameserver.GameDateTime{Hour: 17, Day: 1, Month: 1}
	rendered := RenderRoomView(rv, 200, 0, dt, "")
	stripped := telnet.StripANSI(rendered)
	if !strings.Contains(stripped, description) {
		t.Errorf("expected description in render, got %q", stripped)
	}
}

func TestRenderRoomView_DateTimeInHeader(t *testing.T) {
	rv := &gamev1.RoomView{
		Title:       "Test Room",
		Description: "A plain room.",
	}
	dt := gameserver.GameDateTime{Hour: 7, Day: 15, Month: 6}
	rendered := RenderRoomView(rv, 80, 0, dt, "")
	if !strings.Contains(rendered, "June 15th") {
		t.Errorf("room header must contain date, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Morning") {
		t.Errorf("room header must contain period, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "07:00") {
		t.Errorf("room header must contain hour, got:\n%s", rendered)
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
	result := RenderMap(nil, 80)
	require.Contains(t, result, "No map data")
}

func TestRenderMap_Empty_ReturnsNoMapData(t *testing.T) {
	result := RenderMap(&gamev1.MapResponse{}, 80)
	require.Contains(t, result, "No map data")
}

func TestRenderMap_SingleRoom_Current(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Start Room", X: 0, Y: 0, Current: true},
		},
	}
	result := RenderMap(resp, 80)
	require.Contains(t, result, "< 1>")
	require.Contains(t, result, "Start Room")
}

// TestRenderMap_WestStub_CrossZoneExit verifies that a room at the leftmost map
// column with a west exit (e.g. a cross-zone exit) displays a "<" stub to the left
// of the room cell (BUG-28).
func TestRenderMap_WestStub_CrossZoneExit(t *testing.T) {
	// A non-current room at x=0 with a west exit is used so we can look for "[" as the
	// room marker and a preceding "<" stub without confusion from the current-room "<>"
	// notation.
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "grinders_row", RoomName: "Grinder's Row", X: 0, Y: 0, Current: false,
				Exits: []string{"north", "east", "south", "west"}},
			{RoomId: "last_stand_lodge", RoomName: "Last Stand Lodge", X: 2, Y: 0, Current: true,
				Exits: []string{"west"}},
		},
	}
	result := RenderMap(resp, 80)
	// The west stub must appear on the same line as the grinders_row cell "[ 1]".
	// ANSI color codes may appear between the stub and the cell, so check index ordering.
	found := false
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "[ 1]") {
			// The "<" stub must appear before "[ 1]" on the same line.
			stubIdx := strings.Index(line, "<")
			cellIdx := strings.Index(line, "[ 1]")
			require.True(t, stubIdx >= 0 && stubIdx < cellIdx,
				"west stub '<' must appear before the room cell on line: %q", line)
			found = true
			break
		}
	}
	require.True(t, found, "room cell '[ 1]' not found in map output:\n%s", result)
}

func TestRenderMap_TwoRooms_DistinguishesCurrentFromDiscovered(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Room A", X: 0, Y: 0, Current: false},
			{RoomId: "r2", RoomName: "Room B", X: 2, Y: 0, Current: true, Exits: []string{}},
		},
	}
	result := RenderMap(resp, 80)
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
		result := RenderMap(resp, 80)
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
	result := RenderCharacterSheet(view, 80)
	assert.Contains(t, result, "Skills")
	assert.Contains(t, result, "Acrobatics")
	assert.Contains(t, result, "trnd")
	assert.Contains(t, result, "untr")
	assert.Contains(t, result, "expt")
	assert.Contains(t, result, "mstr")
	assert.Contains(t, result, "lgnd")
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
	result := RenderCharacterSheet(view, 80)
	stripped := telnet.StripANSI(result)
	assert.Contains(t, stripped, "Feats")
	assert.Contains(t, stripped, "Iron Will")
	assert.Contains(t, stripped, "Power Attack")
	assert.Contains(t, stripped, "[A]")
	// Only Power Attack is active — Iron Will must not be followed by [A]
	assert.NotContains(t, stripped, "Iron Will [A]")
}

// TestProperty_RenderCharacterSheet_Feats verifies two properties:
// 1. For any feat with Active=true, the rendered output contains "[A]".
// 2. For a sheet containing only inactive feats, the output does not contain "[A]".
func TestProperty_RenderCharacterSheet_Feats(t *testing.T) {
	// Property 1: at least one active feat => output contains "[A]".
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
		result := telnet.StripANSI(RenderCharacterSheet(view, 80))
		if !strings.Contains(result, "[A]") {
			rt.Fatalf("active feat %q: output does not contain [A]; got:\n%s", name, result)
		}
	})

	// Property 2: all inactive feats => output does not contain "[A]".
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
		result := telnet.StripANSI(RenderCharacterSheet(view, 80))
		if strings.Contains(result, "[A]") {
			rt.Fatalf("inactive feat %q: output must not contain [A]; got:\n%s", name, result)
		}
	})
}

func TestRenderSkillsResponse_GroupedByAbility(t *testing.T) {
	sr := &gamev1.SkillsResponse{
		Skills: []*gamev1.SkillEntry{
			{SkillId: "parkour", Name: "Parkour", Ability: "quickness", Proficiency: "trained", Bonus: 2, Description: "Movement through ruins and tight spaces."},
			{SkillId: "muscle", Name: "Muscle", Ability: "brutality", Proficiency: "untrained", Bonus: 0, Description: "Climbing, swimming, and breaking things."},
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
	if !strings.Contains(out, "+2") {
		t.Error("expected +2 bonus for trained skill")
	}
	if !strings.Contains(out, "+0") {
		t.Error("expected +0 bonus for untrained skill")
	}
	if !strings.Contains(out, "Movement through ruins") {
		t.Error("expected description for Parkour")
	}
	if !strings.Contains(out, "Climbing, swimming") {
		t.Error("expected description for Muscle")
	}
}

func TestRenderCharacterSheet_ClassFeatures(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:  "Test",
		Level: 1,
		ClassFeatures: []*gamev1.ClassFeatureEntry{
			{FeatureId: "brutal_surge", Name: "Brutal Surge", Archetype: "aggressor", Active: true, Description: "Enter a frenzy."},
			{FeatureId: "street_brawler", Name: "Street Brawler", Archetype: "aggressor", Active: false, Description: "Opportunity attacks."},
			{FeatureId: "guerilla_warfare", Name: "Guerilla Warfare", Job: "soldier", Active: false, Description: "Urban cover bonus."},
		},
	}
	result := RenderCharacterSheet(view, 80)
	assert.Contains(t, result, "Job Features")
	assert.Contains(t, result, "Brutal Surge")
	assert.Contains(t, result, "Street Brawler")
	assert.Contains(t, result, "Guerilla Warfare")
	assert.Contains(t, result, "[A]")
}

func TestRenderCharacterSheet_ClassFeatures_Property(t *testing.T) {
	// Property 1: features with Archetype != "" appear before features with Job != "".
	rapid.Check(t, func(rt *rapid.T) {
		archetypeName := rapid.StringMatching(`[a-zA-Z ]{1,20}`).Draw(rt, "archetypeName")
		jobName := rapid.StringMatching(`[a-zA-Z ]{1,20}`).Draw(rt, "jobName")
		active := rapid.Bool().Draw(rt, "active")
		view := &gamev1.CharacterSheetView{
			Name:  "Hero",
			Level: 1,
			ClassFeatures: []*gamev1.ClassFeatureEntry{
				{FeatureId: "arch_feat", Name: archetypeName, Archetype: "someArchetype", Active: active},
				{FeatureId: "job_feat", Name: jobName, Job: "someJob", Active: false},
			},
		}
		result := RenderCharacterSheet(view, 80)
		stripped := telnet.StripANSI(result)

		archIdx := strings.Index(stripped, archetypeName)
		jobIdx := strings.Index(stripped, jobName)
		if archIdx == -1 {
			rt.Fatalf("archetype feature %q not found in output:\n%s", archetypeName, stripped)
		}
		if jobIdx == -1 {
			rt.Fatalf("job feature %q not found in output:\n%s", jobName, stripped)
		}
		// Archetype section header precedes job section header (indented labels inside class features).
		archetypeSectionIdx := strings.Index(stripped, "  Archetype:")
		jobSectionIdx := strings.LastIndex(stripped, "  Job:")
		if archetypeSectionIdx == -1 || jobSectionIdx == -1 {
			rt.Fatalf("expected both '  Archetype:' and '  Job:' section headers in output:\n%s", stripped)
		}
		if archetypeSectionIdx >= jobSectionIdx {
			rt.Fatalf("archetype section must appear before job section; got:\n%s", stripped)
		}
		// Active archetype feature must produce [A].
		if active && !strings.Contains(stripped, "[A]") {
			rt.Fatalf("active archetype feature must produce [A] in output:\n%s", stripped)
		}
	})

	// Property 2: inactive features must not produce [A].
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z ]{1,20}`).Draw(rt, "name")
		view := &gamev1.CharacterSheetView{
			Name:  "Hero",
			Level: 1,
			ClassFeatures: []*gamev1.ClassFeatureEntry{
				{FeatureId: "feat", Name: name, Archetype: "arch", Active: false},
			},
		}
		result := telnet.StripANSI(RenderCharacterSheet(view, 80))
		if strings.Contains(result, "[A]") {
			rt.Fatalf("inactive feature %q must not produce [A]; got:\n%s", name, result)
		}
	})
}

// TestRenderCharacterSheet_Proficiencies verifies that a CharacterSheetView with
// proficiency entries renders a Proficiencies section with each entry listed.
func TestRenderCharacterSheet_Proficiencies(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:  "Test",
		Level: 1,
		Proficiencies: []*gamev1.ProficiencyEntry{
			{Category: "unarmored", Name: "Unarmored", Rank: "trained", Bonus: 3, Kind: "armor"},
			{Category: "light_armor", Name: "Light Armor", Rank: "untrained", Bonus: 0, Kind: "armor"},
			{Category: "simple_weapons", Name: "Simple Weapons", Rank: "trained", Bonus: 3, Kind: "weapon"},
		},
	}
	result := RenderCharacterSheet(view, 80)
	stripped := telnet.StripANSI(result)

	require.Contains(t, stripped, "Proficiencies")
	assert.Contains(t, stripped, "Unarmored")
	assert.Contains(t, stripped, "Light Armor")
	assert.Contains(t, stripped, "Simple Weapons")
	assert.Contains(t, stripped, "trained")
	assert.Contains(t, stripped, "untrained")
}

// TestRenderCharacterSheet_EmptyProficiencies verifies that a CharacterSheetView
// with no proficiency entries does not render a Proficiencies section.
func TestRenderCharacterSheet_EmptyProficiencies(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:          "Test",
		Level:         1,
		Proficiencies: nil,
	}
	result := RenderCharacterSheet(view, 80)
	stripped := telnet.StripANSI(result)
	assert.NotContains(t, stripped, "Proficiencies")
}

// TestProperty_RenderCharacterSheet_Proficiencies verifies that every proficiency
// entry name provided in the view appears in the rendered output.
func TestProperty_RenderCharacterSheet_Proficiencies(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z ]{2,15}`).Draw(rt, "name")
		rank := rapid.SampledFrom([]string{"untrained", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")
		kind := rapid.SampledFrom([]string{"armor", "weapon"}).Draw(rt, "kind")
		bonus := rapid.Int32Range(0, 30).Draw(rt, "bonus")
		view := &gamev1.CharacterSheetView{
			Name:  "Hero",
			Level: 1,
			Proficiencies: []*gamev1.ProficiencyEntry{
				{Category: "test_cat", Name: name, Rank: rank, Bonus: bonus, Kind: kind},
			},
		}
		result := telnet.StripANSI(RenderCharacterSheet(view, 80))
		if !strings.Contains(result, "Proficiencies") {
			rt.Fatalf("expected 'Proficiencies' section header in output:\n%s", result)
		}
		if !strings.Contains(result, name) {
			rt.Fatalf("expected proficiency name %q in output:\n%s", name, result)
		}
		if !strings.Contains(result, rank) {
			rt.Fatalf("expected rank %q in output:\n%s", rank, result)
		}
		bonusStr := fmt.Sprintf("+%d", bonus)
		if !strings.Contains(result, bonusStr) {
			rt.Fatalf("expected bonus %q in output:\n%s", bonusStr, result)
		}
	})
}

func TestRenderRoomView_NPCsLabelled(t *testing.T) {
	rv := &gamev1.RoomView{
		Title: "Test Room",
		Npcs: []*gamev1.NpcInfo{
			{Name: "Ganger A", HealthDescription: "unharmed"},
		},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	assert.Contains(t, stripped, "NPCs:")
	for _, line := range strings.Split(stripped, "\r\n") {
		if strings.Contains(line, "NPCs:") {
			assert.Contains(t, line, "Ganger A", "first NPC must be on same line as NPCs: label")
			break
		}
	}
}

func TestRenderRoomView_NPCsTwoColumns(t *testing.T) {
	rv := &gamev1.RoomView{
		Title: "Test Room",
		Npcs: []*gamev1.NpcInfo{
			{Name: "Ganger A", HealthDescription: "unharmed"},
			{Name: "Ganger B", HealthDescription: "lightly wounded"},
			{Name: "Ganger C", HealthDescription: "unharmed"},
		},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	var npcLines []string
	for _, l := range strings.Split(strings.TrimRight(stripped, "\r\n"), "\r\n") {
		if strings.Contains(l, "Ganger") {
			npcLines = append(npcLines, l)
		}
	}
	require.Equal(t, 2, len(npcLines), "3 NPCs should span 2 rows at 2 per row")
	assert.Contains(t, npcLines[0], "Ganger A")
	assert.Contains(t, npcLines[0], "Ganger B")
	assert.Contains(t, npcLines[1], "Ganger C")
}

func TestRenderRoomView_NPCsFightingTarget(t *testing.T) {
	rv := &gamev1.RoomView{
		Title: "Test Room",
		Npcs: []*gamev1.NpcInfo{
			{Name: "Ganger A", HealthDescription: "unharmed", FightingTarget: "alice"},
		},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	assert.Contains(t, stripped, "fighting alice")
}

func TestRenderRoomView_EquipmentLabelled(t *testing.T) {
	rv := &gamev1.RoomView{
		Title:     "Test Room",
		Equipment: []*gamev1.RoomEquipmentItem{{Name: "Crate"}},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	for _, l := range strings.Split(stripped, "\r\n") {
		if strings.Contains(l, "Items:") {
			assert.Contains(t, l, "Crate", "first item must be on same line as Items: label")
			return
		}
	}
	t.Fatal("Items: label not found")
}

func TestRenderRoomView_EquipmentFourColumns(t *testing.T) {
	rv := &gamev1.RoomView{
		Title: "Test Room",
		Equipment: []*gamev1.RoomEquipmentItem{
			{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"},
		},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	var itemLines []string
	inItems := false
	for _, l := range strings.Split(strings.TrimRight(stripped, "\r\n"), "\r\n") {
		if strings.Contains(l, "Items:") {
			inItems = true
			itemLines = append(itemLines, l)
			continue
		}
		if inItems {
			trimmed := strings.TrimSpace(l)
			if trimmed == "" {
				break
			}
			itemLines = append(itemLines, l)
		}
	}
	assert.Equal(t, 2, len(itemLines), "5 items should produce 2 rows (4+1)")
}

func TestRenderRoomView_EquipmentAfterExitsBeforeNPCs(t *testing.T) {
	rv := &gamev1.RoomView{
		Title:     "Test Room",
		Exits:     []*gamev1.ExitInfo{{Direction: "north"}},
		Equipment: []*gamev1.RoomEquipmentItem{{Name: "Crate"}},
		Npcs:      []*gamev1.NpcInfo{{Name: "Ganger A", HealthDescription: "unharmed"}},
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	lines := strings.Split(strings.TrimRight(stripped, "\r\n"), "\r\n")
	exitIdx, equipIdx, npcIdx := -1, -1, -1
	for i, l := range lines {
		if strings.Contains(l, "Exits:") && exitIdx == -1   { exitIdx = i }
		if strings.Contains(l, "Items:") && equipIdx == -1  { equipIdx = i }
		if strings.Contains(l, "Ganger A") && npcIdx == -1  { npcIdx = i }
	}
	require.Greater(t, exitIdx, -1, "must have exits")
	require.Greater(t, equipIdx, -1, "must have items label")
	require.Greater(t, npcIdx, -1, "must have NPC")
	assert.Greater(t, equipIdx, exitIdx, "items must appear after exits")
	assert.Greater(t, npcIdx, equipIdx, "NPCs must appear after items")
}

func TestAbilityBonus_ModifierOnly(t *testing.T) {
	result := RenderCharacterSheet(&gamev1.CharacterSheetView{
		Name:      "Hero",
		Level:     1,
		Brutality:  14,
		Grit:       10,
		Quickness:  8,
	}, 80)
	stripped := telnet.StripANSI(result)
	assert.Contains(t, stripped, "Brutality:")
	assert.Contains(t, stripped, "+2")
	assert.Contains(t, stripped, "Grit:")
	assert.Contains(t, stripped, "+0")
	assert.Contains(t, stripped, "Quickness:")
	assert.Contains(t, stripped, "-1")
	assert.NotContains(t, stripped, "(14)")
	assert.NotContains(t, stripped, "(10)")
	assert.NotContains(t, stripped, "(8)")
}

func TestRenderCharacterSheet_Saves(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:          "Hero",
		Level:         1,
		ToughnessSave: 5,
		HustleSave:    3,
		CoolSave:      -1,
	}
	result := RenderCharacterSheet(view, 80)
	assert.Contains(t, result, "Saves")
	assert.Contains(t, result, "Toughness")
	assert.Contains(t, result, "+5")
	assert.Contains(t, result, "Hustle")
	assert.Contains(t, result, "+3")
	assert.Contains(t, result, "Cool")
	assert.Contains(t, result, "-1") // negative modifier, no + prefix
}

// TestRenderCharacterSheet_XPSection verifies that XP progress fields are rendered.
func TestRenderCharacterSheet_XPSection(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:          "Hero",
		Level:         3,
		Experience:    250,
		XpToNext:      650,
		PendingBoosts: 0,
	}
	result := telnet.StripANSI(RenderCharacterSheet(view, 80))
	assert.Contains(t, result, "Progress")
	assert.Contains(t, result, "250")
	assert.Contains(t, result, "650")
	assert.NotContains(t, result, "levelup") // no hint when boosts == 0
}

func TestRenderCharacterSheet_PendingSkillIncreases_Shown(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:                  "Test",
		Level:                 4,
		Experience:            1600,
		XpToNext:              900,
		PendingBoosts:         0,
		PendingSkillIncreases: 2,
	}
	result := RenderCharacterSheet(csv, 120)
	assert.Contains(t, result, "Pending Skill Increases: 2")
	assert.Contains(t, result, "trainskill")
}

func TestRenderCharacterSheet_PendingSkillIncreases_Zero_NotShown(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:                  "Test",
		Level:                 4,
		Experience:            1600,
		XpToNext:              900,
		PendingBoosts:         0,
		PendingSkillIncreases: 0,
	}
	result := RenderCharacterSheet(csv, 120)
	assert.NotContains(t, result, "Pending Skill Increases")
	assert.NotContains(t, result, "trainskill")
}

// TestRenderCharacterSheet_XPSection_PendingBoosts verifies the levelup hint appears when pending_boosts > 0.
func TestRenderCharacterSheet_XPSection_PendingBoosts(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:          "Hero",
		Level:         4,
		Experience:    1600,
		XpToNext:      900,
		PendingBoosts: 1,
	}
	result := telnet.StripANSI(RenderCharacterSheet(view, 80))
	assert.Contains(t, result, "Progress")
	assert.Contains(t, result, "Pending Boosts: 1")
	assert.Contains(t, result, "levelup")
}

func TestRenderCharacterSheet_ShowsAwareness(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:      "TestChar",
		Awareness: 15,
	}
	rendered := RenderCharacterSheet(csv, 80)
	assert.Contains(t, telnet.StripANSI(rendered), "Awareness: +15")
}

func TestRenderCharacterSheet_ShowsAwareness_NoBonus(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:      "TestChar",
		Awareness: 10,
	}
	rendered := RenderCharacterSheet(csv, 80)
	assert.Contains(t, telnet.StripANSI(rendered), "Awareness: +10")
}

func TestRenderCharacterSheet_ShowsInitiative_Positive(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:      "TestChar",
		Quickness: 16, // mod = (16-10)/2 = +3
	}
	rendered := RenderCharacterSheet(csv, 80)
	assert.Contains(t, telnet.StripANSI(rendered), "Initiative: +3")
}

func TestRenderCharacterSheet_ShowsInitiative_Zero(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:      "TestChar",
		Quickness: 10, // mod = (10-10)/2 = +0
	}
	rendered := RenderCharacterSheet(csv, 80)
	assert.Contains(t, telnet.StripANSI(rendered), "Initiative: +0")
}

func TestRenderCharacterSheet_ShowsInitiative_Negative(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:      "TestChar",
		Quickness: 8, // mod = (8-10)/2 = -1
	}
	rendered := RenderCharacterSheet(csv, 80)
	assert.Contains(t, telnet.StripANSI(rendered), "Initiative: -1")
}

func TestRenderNPCs_ConditionsShown(t *testing.T) {
	npcs := []*gamev1.NpcInfo{
		{
			Name:              "Goblin",
			HealthDescription: "lightly wounded",
			FightingTarget:    "Hero",
			Conditions:        []string{"prone", "grabbed"},
		},
	}
	lines := renderNPCs(npcs, 120)
	joined := strings.Join(lines, "\n")
	stripped := telnet.StripANSI(joined)
	if !strings.Contains(stripped, "prone") {
		t.Errorf("expected 'prone' in NPC display, got %q", stripped)
	}
	if !strings.Contains(stripped, "grabbed") {
		t.Errorf("expected 'grabbed' in NPC display, got %q", stripped)
	}
	if strings.Contains(stripped, "[prone") || strings.Contains(stripped, "grabbed]") {
		t.Errorf("conditions must be rendered without square brackets, got %q", stripped)
	}
}

func TestRenderNPCs_NoConditions(t *testing.T) {
	npcs := []*gamev1.NpcInfo{
		{
			Name:              "Goblin",
			HealthDescription: "unharmed",
			Conditions:        nil,
		},
	}
	lines := renderNPCs(npcs, 120)
	joined := strings.Join(lines, "\n")
	stripped := telnet.StripANSI(joined)
	if strings.Contains(stripped, "prone") || strings.Contains(stripped, "grabbed") {
		t.Errorf("unexpected condition text in %q", stripped)
	}
}

func TestProperty_RenderNPCs_AllConditionsPresent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 4).Draw(rt, "n")
		conds := make([]string, n)
		for i := range conds {
			conds[i] = rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, fmt.Sprintf("cond%d", i))
		}
		npc := &gamev1.NpcInfo{
			Name:              "Goblin",
			HealthDescription: "unharmed",
			Conditions:        conds,
		}
		lines := renderNPCs([]*gamev1.NpcInfo{npc}, 120)
		joined := strings.Join(lines, "\n")
		stripped := telnet.StripANSI(joined)
		for _, c := range conds {
			if !strings.Contains(stripped, c) {
				rt.Errorf("condition %q missing from NPC display %q", c, stripped)
			}
		}
	})
}

// TestProperty_RenderCharacterSheet_XPSection verifies XP values always appear in output.
func TestProperty_RenderCharacterSheet_XPSection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		xp := rapid.Int32Range(0, 1000000).Draw(rt, "experience")
		xpToNext := rapid.Int32Range(0, 1000000).Draw(rt, "xp_to_next")
		boosts := rapid.Int32Range(0, 10).Draw(rt, "pending_boosts")
		view := &gamev1.CharacterSheetView{
			Experience:    xp,
			XpToNext:      xpToNext,
			PendingBoosts: boosts,
		}
		result := telnet.StripANSI(RenderCharacterSheet(view, 80))
		if !strings.Contains(result, "Progress") {
			rt.Fatalf("expected 'Progress' section in output:\n%s", result)
		}
		if boosts > 0 && !strings.Contains(result, "levelup") {
			rt.Fatalf("expected 'levelup' hint when pending_boosts=%d:\n%s", boosts, result)
		}
		if boosts == 0 && strings.Contains(result, "levelup") {
			rt.Fatalf("unexpected 'levelup' hint when pending_boosts=0:\n%s", result)
		}
	})
}

func TestRenderMap_WideLayout_HasSeparator(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Start", X: 0, Y: 0, Current: true, Exits: []string{"east"}},
			{RoomId: "r2", RoomName: "East Room", X: 1, Y: 0, Current: false, Exits: []string{"west"}},
		},
	}
	out := RenderMap(resp, 120)
	if !strings.Contains(out, "│") {
		t.Errorf("expected '│' separator in wide map layout, got:\n%s", out)
	}
	if !strings.Contains(out, "Start") {
		t.Errorf("expected 'Start' in legend, got:\n%s", out)
	}
	if !strings.Contains(out, "East Room") {
		t.Errorf("expected 'East Room' in legend, got:\n%s", out)
	}
}

// TestRenderMap_BoundaryWide verifies the exact threshold: width=100 triggers 2-column layout (REQ-T9).
func TestRenderMap_BoundaryWide(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Threshold", X: 0, Y: 0, Current: true, Exits: []string{}},
		},
	}
	out := RenderMap(resp, 100)
	if !strings.Contains(out, "│") {
		t.Errorf("expected '│' separator at exact threshold width=100, got:\n%s", out)
	}
	if !strings.Contains(out, "Threshold") {
		t.Errorf("expected 'Threshold' in legend at width=100, got:\n%s", out)
	}
}

func TestRenderMap_NarrowLayout_NoSeparator(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Start", X: 0, Y: 0, Current: true, Exits: []string{}},
		},
	}
	out := RenderMap(resp, 80)
	if strings.Contains(out, "│") {
		t.Errorf("unexpected '│' separator in narrow map layout (width=80), got:\n%s", out)
	}
	if !strings.Contains(out, "Legend:") {
		t.Errorf("expected 'Legend:' in stacked layout, got:\n%s", out)
	}
}

// TestRenderMap_BoundaryNarrow verifies the exact threshold: width=99 keeps stacked layout (REQ-T10).
func TestRenderMap_BoundaryNarrow(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Threshold", X: 0, Y: 0, Current: true, Exits: []string{}},
		},
	}
	out := RenderMap(resp, 99)
	if strings.Contains(out, "│") {
		t.Errorf("unexpected '│' separator at width=99 (one below threshold), got:\n%s", out)
	}
	if !strings.Contains(out, "Legend:") {
		t.Errorf("expected 'Legend:' in stacked layout at width=99, got:\n%s", out)
	}
}

func TestProperty_RenderMap_AllLegendEntriesPresent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		tiles := make([]*gamev1.MapTile, n)
		for i := range tiles {
			tiles[i] = &gamev1.MapTile{
				RoomId:   fmt.Sprintf("r%d", i),
				RoomName: fmt.Sprintf("Room%d", i),
				X:        int32(i),
				Y:        0,
				Current:  i == 0,
				Exits:    []string{},
			}
		}
		width := rapid.IntRange(100, 200).Draw(rt, "width")
		resp := &gamev1.MapResponse{Tiles: tiles}
		out := RenderMap(resp, width)
		for _, tile := range tiles {
			count := strings.Count(out, tile.RoomName)
			if count != 1 {
				rt.Errorf("legend entry %q appears %d times (want 1) in wide map output", tile.RoomName, count)
			}
		}
		if !strings.Contains(out, "│") {
			rt.Errorf("expected '│' separator at width=%d", width)
		}
	})
}

func TestProperty_RenderMap_NarrowUnchanged(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		tiles := make([]*gamev1.MapTile, n)
		for i := range tiles {
			tiles[i] = &gamev1.MapTile{
				RoomId:   fmt.Sprintf("r%d", i),
				RoomName: fmt.Sprintf("Room%d", i),
				X:        int32(i),
				Y:        0,
				Current:  i == 0,
			}
		}
		width := rapid.IntRange(1, 99).Draw(rt, "width")
		resp := &gamev1.MapResponse{Tiles: tiles}
		out := RenderMap(resp, width)
		if strings.Contains(out, "│") {
			rt.Errorf("unexpected '│' separator at narrow width=%d", width)
		}
		if !strings.Contains(out, "Legend:") {
			rt.Errorf("expected 'Legend:' in stacked output at width=%d", width)
		}
	})
}

func TestRenderRoomView_ZoneName(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "test_room",
		Title:       "Cully Road",
		Description: "A busy road.",
		ZoneName:    "Northeast Portland",
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	assert.Contains(t, stripped, "[Northeast Portland]", "zone name must appear in header")
	assert.Contains(t, stripped, "Cully Road", "room title must appear in header")
}

func TestRenderRoomView_ZoneNameEmpty(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "test_room",
		Title:       "Cully Road",
		Description: "A busy road.",
		ZoneName:    "",
	}
	rendered := RenderRoomView(rv, 80, 0, testDT, "")
	stripped := telnet.StripANSI(rendered)
	assert.NotContains(t, stripped, "[", "no zone bracket when ZoneName is empty")
	assert.Contains(t, stripped, "Cully Road", "room title must appear in header")
}

func TestRenderMap_POISuffixRow_AfterRoomRow(t *testing.T) {
	// Single explored room with a merchant POI: suffix row must appear between
	// room row and south connector row.
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Market", X: 0, Y: 0, Current: true, Pois: []string{"merchant"}},
		},
	}
	out := RenderMap(resp, 80)
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\r\n")
	// lines[0] is blank from leading \r\n; lines[1] is room row; lines[2] is POI suffix row.
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %v", len(lines), lines)
	}
	// Room row must contain angle brackets for current room.
	if !strings.Contains(lines[1], "<") {
		t.Errorf("line[1] should be room row with < >, got %q", lines[1])
	}
	// POI suffix row must contain the merchant symbol $.
	if !strings.Contains(lines[2], "$") {
		t.Errorf("line[2] should be POI suffix row containing $, got %q", lines[2])
	}
}

func TestRenderMap_POISuffixRow_UnexploredBlank(t *testing.T) {
	// Two rooms; tile at (1,0) has Pois set but tile at (0,0) has empty Pois.
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Empty", X: 0, Y: 0, Current: false, Pois: nil},
			{RoomId: "r2", RoomName: "Market", X: 1, Y: 0, Current: true, Pois: []string{"merchant"}},
		},
	}
	out := RenderMap(resp, 80)
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\r\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	suffix := lines[2]
	// First 4 chars (cell for x=0) must be blank.
	if len(suffix) < 4 {
		t.Fatalf("suffix row too short: %q", suffix)
	}
	// Strip ANSI from first cell.
	cell0 := suffix[:4]
	if strings.TrimSpace(cell0) != "" {
		t.Errorf("x=0 suffix cell should be spaces, got %q", cell0)
	}
}

func TestRenderMap_POISuffixRow_FiveTypes_Ellipsis(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Hub", X: 0, Y: 0, Current: true,
				Pois: []string{"merchant", "healer", "trainer", "guard", "npc"}},
		},
	}
	out := RenderMap(resp, 80)
	if !strings.Contains(out, "…") {
		t.Errorf("RenderMap with 5 POI types should contain ellipsis, got:\n%s", out)
	}
}

func TestRenderMap_TwoColumnLayout_POISuffixRows(t *testing.T) {
	// Width >= 100 triggers two-column layout. Verify legend lines still align
	// (no panic, all grid lines interleaved with legend lines correctly).
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Market", X: 0, Y: 0, Current: true, Pois: []string{"merchant"}},
			{RoomId: "r2", RoomName: "Clinic", X: 1, Y: 0, Current: false, Pois: []string{"healer"}},
		},
	}
	out := RenderMap(resp, 120)
	if out == "" {
		t.Fatal("RenderMap returned empty string")
	}
	// Legend must contain room numbers.
	if !strings.Contains(out, "1.") || !strings.Contains(out, "2.") {
		t.Errorf("legend room numbers missing from:\n%s", out)
	}
}

func TestRenderMap_Legend_POISection_AfterHeader(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Market", X: 0, Y: 0, Current: true, Pois: []string{"merchant", "equipment"}},
		},
	}
	out := RenderMap(resp, 80)
	// Must contain "Points of Interest" section.
	if !strings.Contains(out, "Merchant") {
		t.Errorf("legend should contain Merchant label, got:\n%s", out)
	}
	if !strings.Contains(out, "Equipment") {
		t.Errorf("legend should contain Equipment label, got:\n%s", out)
	}
}

func TestRenderMap_Legend_POISection_OnlyPresentTypes(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Market", X: 0, Y: 0, Current: true, Pois: []string{"merchant"}},
		},
	}
	out := RenderMap(resp, 80)
	if strings.Contains(out, "Healer") {
		t.Errorf("legend should NOT contain Healer when not present, got:\n%s", out)
	}
}

func TestRenderMap_Legend_POISection_TableOrder(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Hub", X: 0, Y: 0, Current: true,
				Pois: []string{"equipment", "merchant"}},
		},
	}
	out := RenderMap(resp, 80)
	merchantIdx := strings.Index(out, "Merchant")
	equipmentIdx := strings.Index(out, "Equipment")
	if merchantIdx < 0 || equipmentIdx < 0 {
		t.Fatalf("legend missing Merchant or Equipment, got:\n%s", out)
	}
	if merchantIdx > equipmentIdx {
		t.Errorf("Merchant should appear before Equipment in legend (table order), got:\n%s", out)
	}
}

func TestRenderMap_Legend_POISection_NoPOIs(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "r1", RoomName: "Room", X: 0, Y: 0, Current: true, Pois: nil},
		},
	}
	out := RenderMap(resp, 80)
	// No POI section header when no POIs present.
	if strings.Contains(out, "Points of Interest") {
		t.Errorf("legend should NOT contain POI section when no POIs, got:\n%s", out)
	}
}

func TestRenderMap_BossRoom_RendersAngleBrackets(t *testing.T) {
	resp := &gamev1.MapResponse{
		Tiles: []*gamev1.MapTile{
			{RoomId: "boss1", X: 0, Y: 0, Current: false, BossRoom: true},
		},
	}
	result := RenderMap(resp, 80)
	// Boss room tile must render as <BB>.
	assert.Contains(t, result, "<BB>", "boss room tile must render as <BB>")
	// Boss room tile must not be rendered as a numbered square-bracket tile like "[ 1]".
	assert.NotContains(t, result, "[ 1]", "boss room grid tile must not use numbered square brackets")
}

func TestRenderCharacterSheet_ShowsFocusPoints_WhenMaxGTZero(t *testing.T) {
	csv := &gamev1.CharacterSheetView{FocusPoints: 2, MaxFocusPoints: 3}
	result := telnet.StripANSI(RenderCharacterSheet(csv, 80))
	assert.Contains(t, result, "Focus Points: 2 / 3")
}

func TestRenderCharacterSheet_OmitsFocusPoints_WhenMaxIsZero(t *testing.T) {
	csv := &gamev1.CharacterSheetView{MaxFocusPoints: 0}
	result := telnet.StripANSI(RenderCharacterSheet(csv, 80))
	assert.NotContains(t, result, "Focus Points")
}

func TestRenderNPCs_NonCombatTypeIndicator(t *testing.T) {
	// Non-combat NPC should show type tag but NOT health description
	npcs := []*gamev1.NpcInfo{
		{Name: "Sal", HealthDescription: "unharmed", NpcType: "merchant"},
		{Name: "Doc Mira", HealthDescription: "unharmed", NpcType: "healer"},
	}
	rows := renderNPCs(npcs, 80)
	combined := strings.Join(rows, "\n")
	stripped := telnet.StripANSI(combined)
	assert.Contains(t, stripped, "[shop]")
	assert.Contains(t, stripped, "[healer]")
	assert.NotContains(t, stripped, "(unharmed)", "non-combat NPCs must not show health")
}

func TestRenderNPCs_CombatNPCNoTypeIndicator(t *testing.T) {
	// Combat NPC should NOT show a type tag
	npcs := []*gamev1.NpcInfo{
		{Name: "Raider", HealthDescription: "unharmed", NpcType: "combat"},
	}
	rows := renderNPCs(npcs, 80)
	combined := strings.Join(rows, "\n")
	stripped := telnet.StripANSI(combined)
	assert.NotContains(t, stripped, "[")
}

// TestRenderNPCs_MotelKeeperShowsRoleTag verifies that motel_keeper NPCs display
// the [motel] role tag instead of a health status in parentheses (BUG-63).
func TestRenderNPCs_MotelKeeperShowsRoleTag(t *testing.T) {
	npcs := []*gamev1.NpcInfo{
		{Name: "Scrap Inn Clerk", HealthDescription: "unharmed", NpcType: "motel_keeper"},
	}
	rows := renderNPCs(npcs, 80)
	combined := strings.Join(rows, "\n")
	stripped := telnet.StripANSI(combined)
	assert.Contains(t, stripped, "Scrap Inn Clerk", "motel_keeper NPC name must appear in output")
	assert.Contains(t, stripped, "[motel]", "motel_keeper NPC must show [motel] role tag")
	assert.NotContains(t, stripped, "(unharmed)", "motel_keeper NPC must not show health status")
}

// TestRenderNPCs_ChipDocShowsRoleTag verifies that chip_doc NPCs display
// the [chip doc] role tag instead of a health status in parentheses.
func TestRenderNPCs_ChipDocShowsRoleTag(t *testing.T) {
	npcs := []*gamev1.NpcInfo{
		{Name: "Doc Reyes", HealthDescription: "unharmed", NpcType: "chip_doc"},
	}
	rows := renderNPCs(npcs, 80)
	combined := strings.Join(rows, "\n")
	stripped := telnet.StripANSI(combined)
	assert.Contains(t, stripped, "Doc Reyes", "chip_doc NPC name must appear in output")
	assert.Contains(t, stripped, "[chip doc]", "chip_doc NPC must show [chip doc] role tag")
	assert.NotContains(t, stripped, "(unharmed)", "chip_doc NPC must not show health status")
}

// TestRenderNPCs_CrafterShowsRoleTag verifies that crafter NPCs display
// the [crafter] role tag instead of a health status in parentheses.
func TestRenderNPCs_CrafterShowsRoleTag(t *testing.T) {
	npcs := []*gamev1.NpcInfo{
		{Name: "Mika the Tinker", HealthDescription: "unharmed", NpcType: "crafter"},
	}
	rows := renderNPCs(npcs, 80)
	combined := strings.Join(rows, "\n")
	stripped := telnet.StripANSI(combined)
	assert.Contains(t, stripped, "Mika the Tinker", "crafter NPC name must appear in output")
	assert.Contains(t, stripped, "[crafter]", "crafter NPC must show [crafter] role tag")
	assert.NotContains(t, stripped, "(unharmed)", "crafter NPC must not show health status")
}

// TestRenderNPCs_TechTrainerShowsRoleTag verifies that tech_trainer NPCs display
// the [tech trainer] role tag instead of a health status in parentheses.
func TestRenderNPCs_TechTrainerShowsRoleTag(t *testing.T) {
	npcs := []*gamev1.NpcInfo{
		{Name: "Grinder", HealthDescription: "unharmed", NpcType: "tech_trainer"},
	}
	rows := renderNPCs(npcs, 80)
	combined := strings.Join(rows, "\n")
	stripped := telnet.StripANSI(combined)
	assert.Contains(t, stripped, "Grinder", "tech_trainer NPC name must appear in output")
	assert.Contains(t, stripped, "[tech trainer]", "tech_trainer NPC must show [tech trainer] role tag")
	assert.NotContains(t, stripped, "(unharmed)", "tech_trainer NPC must not show health status")
}
