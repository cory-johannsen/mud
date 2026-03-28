package handlers

import (
	"testing"

	"pgregory.net/rapid"
)

func TestNewCombatModeHandler_Mode(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	if h.Mode() != ModeCombat {
		t.Fatalf("expected ModeCombat (%d), got %d", ModeCombat, h.Mode())
	}
}

func TestCombatModeHandler_UpdateRoundStart_SetsRound(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(2, 3, []string{"Alice", "Goblin"})
	if got := h.Round(); got != 2 {
		t.Fatalf("expected round 2, got %d", got)
	}
}

func TestCombatModeHandler_UpdateRoundStart_ResetsCombatants(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	combatants := h.Combatants()
	if len(combatants) != 2 {
		t.Fatalf("expected 2 combatants, got %d", len(combatants))
	}
	// Verify AP was set.
	for _, c := range combatants {
		if c.AP != 3 {
			t.Errorf("combatant %s: expected AP 3, got %d", c.Name, c.AP)
		}
		if c.MaxAP != 3 {
			t.Errorf("combatant %s: expected MaxAP 3, got %d", c.Name, c.MaxAP)
		}
	}
	// Verify player flag.
	alice := h.CombatantByName("Alice")
	if alice == nil {
		t.Fatal("expected Alice combatant")
	}
	if !alice.IsPlayer {
		t.Error("expected Alice.IsPlayer == true")
	}
	goblin := h.CombatantByName("Goblin")
	if goblin == nil {
		t.Fatal("expected Goblin combatant")
	}
	if goblin.IsPlayer {
		t.Error("expected Goblin.IsPlayer == false")
	}
}

func TestCombatModeHandler_UpdateCombatEvent_UpdatesHP(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	h.UpdateCombatEvent("Alice", "Goblin", 5, 15, 20, "Alice hits Goblin for 5 damage.", 0)
	goblin := h.CombatantByName("Goblin")
	if goblin == nil {
		t.Fatal("expected Goblin combatant")
	}
	if goblin.HP != 15 {
		t.Fatalf("expected Goblin HP 15, got %d", goblin.HP)
	}
}

func TestCombatModeHandler_Prompt(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	prompt := h.Prompt()
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if prompt != "[combat]> " {
		t.Fatalf("expected '[combat]> ', got %q", prompt)
	}
}

func TestCombatModeHandler_Reset_ClearsCombatants(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	if len(h.Combatants()) == 0 {
		t.Fatal("expected combatants before reset")
	}
	h.Reset()
	if len(h.Combatants()) != 0 {
		t.Fatalf("expected 0 combatants after reset, got %d", len(h.Combatants()))
	}
	if h.Round() != 0 {
		t.Fatalf("expected round 0 after reset, got %d", h.Round())
	}
}

func TestCombatModeHandler_HPBar(t *testing.T) {
	got := hpBar(20, 30, 10)
	expected := "[######....] 20/30"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestCombatModeHandler_APDots(t *testing.T) {
	got := apDots(2, 4)
	expected := "●●○○"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestCombatModeHandler_UpdateRoundStart_MarksMissingAsDead(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	// Round 2: Goblin removed from turn order.
	h.UpdateRoundStart(2, 3, []string{"Alice"})
	goblin := h.CombatantByName("Goblin")
	if goblin == nil {
		t.Fatal("expected Goblin combatant to still exist")
	}
	if !goblin.IsDead {
		t.Error("expected Goblin.IsDead == true after removal from turn order")
	}
}

func TestCombatModeHandler_UpdateCombatEvent_AppendsLog(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	h.UpdateCombatEvent("Alice", "Goblin", 5, 15, 20, "Alice hits Goblin.", 0)
	h.UpdateCombatEvent("Goblin", "Alice", 3, 27, 30, "Goblin hits Alice.", 0)
	snap := h.SnapshotForRender()
	if len(snap.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(snap.Log))
	}
}

func TestCombatModeHandler_SetSummary(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.SetSummary("Victory!")
	if h.Summary() != "Victory!" {
		t.Fatalf("expected summary 'Victory!', got %q", h.Summary())
	}
}

func TestCombatModeHandler_IsMovementCommand(t *testing.T) {
	for _, dir := range []string{"n", "north", "s", "south", "e", "east", "w", "west", "ne", "nw", "se", "sw", "u", "up", "d", "down"} {
		if !isMovementCommand(dir) {
			t.Errorf("expected %q to be a movement command", dir)
		}
	}
	for _, cmd := range []string{"attack", "look", "cast", "q", ""} {
		if isMovementCommand(cmd) {
			t.Errorf("expected %q to NOT be a movement command", cmd)
		}
	}
}

func TestCombatModeHandler_MovementCommandsBlocked(t *testing.T) {
	blocked := []string{
		"north", "south", "east", "west", "up", "down",
		"n", "s", "e", "w", "u", "d",
		"ne", "nw", "se", "sw",
		"northeast", "northwest", "southeast", "southwest",
	}
	for _, cmd := range blocked {
		if !IsMovementCommand(cmd) {
			t.Errorf("expected %q to be a movement command", cmd)
		}
	}
}

func TestCombatModeHandler_NonMovementCommandsNotBlocked(t *testing.T) {
	allowed := []string{"attack", "look", "inventory", "say hello", "who", "flee"}
	for _, cmd := range allowed {
		if IsMovementCommand(cmd) {
			t.Errorf("expected %q NOT to be a movement command", cmd)
		}
	}
}

func TestIsMovementCommand_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cmd := rapid.StringMatching(`[a-z]{1,15}`).Draw(t, "cmd")
		_ = IsMovementCommand(cmd)
	})
}

func TestCombatModeHandler_UpdatePosition_UpdatesCombatantPosition(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	h.UpdatePosition("Goblin", 15)
	goblin := h.CombatantByName("Goblin")
	if goblin == nil {
		t.Fatal("expected Goblin combatant")
	}
	if goblin.Position != 15 {
		t.Fatalf("expected Goblin.Position == 15, got %d", goblin.Position)
	}
}

func TestCombatModeHandler_UpdatePosition_PlayerPosition(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	h.UpdatePosition("Alice", 25)
	alice := h.CombatantByName("Alice")
	if alice == nil {
		t.Fatal("expected Alice combatant")
	}
	if alice.Position != 25 {
		t.Fatalf("expected Alice.Position == 25, got %d", alice.Position)
	}
}

func TestCombatModeHandler_UpdatePosition_UnknownNameNoOp(t *testing.T) {
	h := NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice"})
	// Should not panic for unknown combatant.
	h.UpdatePosition("Unknown", 10)
}

func TestCombatModeHandler_UpdatePosition_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, "name")
		pos := rapid.IntRange(0, 100).Draw(t, "pos")
		h := NewCombatModeHandler(name, func() {})
		h.UpdateRoundStart(1, 3, []string{name, "NPC"})
		h.UpdatePosition(name, pos)
		c := h.CombatantByName(name)
		if c == nil {
			t.Fatal("expected combatant")
		}
		if c.Position != pos {
			t.Fatalf("expected position %d, got %d", pos, c.Position)
		}
	})
}
