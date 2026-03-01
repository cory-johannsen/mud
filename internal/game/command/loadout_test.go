package command_test

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// newTestSession returns a PlayerSession with a freshly initialised LoadoutSet and Equipment.
//
// Postcondition: sess.LoadoutSet != nil, len(Presets)==2, Active==0; sess.Equipment != nil.
func newTestSession() *session.PlayerSession {
	return &session.PlayerSession{
		UID:        "test-uid",
		CharName:   "Tester",
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
	}
}

// pistolWeaponDef returns a valid firearm WeaponDef for use in tests.
func pistolWeaponDef() *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:               "pistol-9mm",
		Name:             "9mm Pistol",
		DamageDice:       "2d6",
		DamageType:       "ballistic",
		RangeIncrement:   30,
		FiringModes:      []inventory.FiringMode{inventory.FiringModeSingle},
		MagazineCapacity: 15,
		Kind:             inventory.WeaponKindOneHanded,
	}
}

// TestHandleLoadout_NoArg_ShowsBothPresets verifies that calling with no argument
// displays both presets with the [active] marker on the active preset.
func TestHandleLoadout_NoArg_ShowsBothPresets(t *testing.T) {
	sess := newTestSession()
	result := command.HandleLoadout(sess, "")

	if !strings.Contains(result, "Preset 1") {
		t.Errorf("expected output to contain 'Preset 1', got:\n%s", result)
	}
	if !strings.Contains(result, "Preset 2") {
		t.Errorf("expected output to contain 'Preset 2', got:\n%s", result)
	}
	if !strings.Contains(result, "[active]") {
		t.Errorf("expected output to contain '[active]', got:\n%s", result)
	}
}

// TestHandleLoadout_NoArg_ActiveMarkerOnPreset1 verifies that Preset 1 is marked
// active when Active==0.
func TestHandleLoadout_NoArg_ActiveMarkerOnPreset1(t *testing.T) {
	sess := newTestSession()
	// Active defaults to 0 (Preset 1).
	result := command.HandleLoadout(sess, "")

	lines := strings.Split(result, "\n")
	var preset1Line string
	for _, l := range lines {
		if strings.Contains(l, "Preset 1") {
			preset1Line = l
			break
		}
	}
	if !strings.Contains(preset1Line, "[active]") {
		t.Errorf("expected Preset 1 line to contain '[active]', got: %q", preset1Line)
	}
}

// TestHandleLoadout_NoArg_EmptySlots verifies that empty main and off-hand slots
// display "empty".
func TestHandleLoadout_NoArg_EmptySlots(t *testing.T) {
	sess := newTestSession()
	result := command.HandleLoadout(sess, "")

	count := strings.Count(result, "empty")
	// Two presets x two slots = 4 occurrences of "empty".
	if count < 4 {
		t.Errorf("expected at least 4 occurrences of 'empty', got %d in:\n%s", count, result)
	}
}

// TestHandleLoadout_NoArg_WeaponWithAmmo verifies that an equipped firearm is
// displayed with its ammo count in [loaded/capacity] format.
func TestHandleLoadout_NoArg_WeaponWithAmmo(t *testing.T) {
	sess := newTestSession()
	def := pistolWeaponDef()
	if err := sess.LoadoutSet.Presets[0].EquipMainHand(def); err != nil {
		t.Fatalf("equip failed: %v", err)
	}
	// Consume 3 rounds so the display is non-trivial.
	sess.LoadoutSet.Presets[0].MainHand.Magazine.Loaded = 12

	result := command.HandleLoadout(sess, "")

	if !strings.Contains(result, "9mm Pistol") {
		t.Errorf("expected weapon name in output, got:\n%s", result)
	}
	if !strings.Contains(result, "[12/15]") {
		t.Errorf("expected ammo display [12/15] in output, got:\n%s", result)
	}
}

// TestHandleLoadout_SwapToPreset2 verifies that "loadout 2" swaps to preset index 1
// and returns a confirmation message.
func TestHandleLoadout_SwapToPreset2(t *testing.T) {
	sess := newTestSession()
	result := command.HandleLoadout(sess, "2")

	if sess.LoadoutSet.Active != 1 {
		t.Errorf("expected Active==1 after swap to preset 2, got %d", sess.LoadoutSet.Active)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("expected confirmation to mention preset 2, got: %q", result)
	}
}

// TestHandleLoadout_SwapToPreset1 verifies that "loadout 1" after an initial swap
// back to preset index 0 works correctly.
func TestHandleLoadout_SwapToPreset1(t *testing.T) {
	sess := newTestSession()
	// Start at preset 2 so swapping to 1 is a real swap.
	sess.LoadoutSet.Active = 1
	result := command.HandleLoadout(sess, "1")

	if sess.LoadoutSet.Active != 0 {
		t.Errorf("expected Active==0 after swap to preset 1, got %d", sess.LoadoutSet.Active)
	}
	if !strings.Contains(result, "1") {
		t.Errorf("expected confirmation to mention preset 1, got: %q", result)
	}
}

// TestHandleLoadout_AlreadySwappedThisRound verifies that a second swap in the same
// round returns an error containing "already".
func TestHandleLoadout_AlreadySwappedThisRound(t *testing.T) {
	sess := newTestSession()
	// Perform first swap.
	sess.LoadoutSet.Active = 1
	sess.LoadoutSet.SwappedThisRound = true

	result := command.HandleLoadout(sess, "1")

	if !strings.Contains(strings.ToLower(result), "already") {
		t.Errorf("expected error containing 'already', got: %q", result)
	}
}

// TestHandleLoadout_InvalidIndex verifies that an out-of-range index returns
// an error containing "invalid".
func TestHandleLoadout_InvalidIndex(t *testing.T) {
	sess := newTestSession()
	result := command.HandleLoadout(sess, "9")

	if !strings.Contains(strings.ToLower(result), "invalid") {
		t.Errorf("expected error containing 'invalid', got: %q", result)
	}
}

// TestHandleLoadout_NonNumericArg verifies that a non-numeric argument returns
// an error containing "invalid".
func TestHandleLoadout_NonNumericArg(t *testing.T) {
	sess := newTestSession()
	result := command.HandleLoadout(sess, "foo")

	if !strings.Contains(strings.ToLower(result), "invalid") {
		t.Errorf("expected error containing 'invalid' for non-numeric arg, got: %q", result)
	}
}

// TestHandleLoadout_SameIndexNoOp verifies that selecting the already-active
// preset does not consume the swap action and returns a no-op message.
func TestHandleLoadout_SameIndexNoOp(t *testing.T) {
	sess := newTestSession()
	// Active is 0; selecting preset 1 (1-based) maps to idx 0 — no-op.
	result := command.HandleLoadout(sess, "1")

	if sess.LoadoutSet.SwappedThisRound {
		t.Error("expected SwappedThisRound==false after selecting already-active preset")
	}
	if !strings.Contains(strings.ToLower(result), "already active") {
		t.Errorf("expected message containing 'already active', got: %q", result)
	}
}

// TestHandleLoadout_ZeroIndexInvalid verifies that "0" is rejected as invalid.
func TestHandleLoadout_ZeroIndexInvalid(t *testing.T) {
	sess := newTestSession()
	result := command.HandleLoadout(sess, "0")

	if !strings.Contains(strings.ToLower(result), "invalid") {
		t.Errorf("expected error containing 'invalid' for arg '0', got: %q", result)
	}
}

// TestProperty_HandleLoadout_DisplayAlwaysContainsAllPresets is a property-based
// test that verifies the display output always contains one line per preset
// regardless of the active index.
func TestProperty_HandleLoadout_DisplayAlwaysContainsAllPresets(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := newTestSession()
		activeIdx := rapid.IntRange(0, len(sess.LoadoutSet.Presets)-1).Draw(rt, "activeIdx")
		sess.LoadoutSet.Active = activeIdx

		result := command.HandleLoadout(sess, "")

		for i := range sess.LoadoutSet.Presets {
			needle := fmt.Sprintf("Preset %d", i+1)
			if !strings.Contains(result, needle) {
				rt.Fatalf("output missing %q for activeIdx=%d:\n%s", needle, activeIdx, result)
			}
		}

		// Exactly one [active] marker must appear.
		count := strings.Count(result, "[active]")
		if count != 1 {
			rt.Fatalf("expected exactly 1 [active] marker, got %d", count)
		}
	})
}
