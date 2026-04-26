package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/detection"
)

// TestStartCombat_DetectionStatesInitialised verifies StartCombat installs a
// non-nil DetectionStates map (#254 / DETECT-3).
func TestStartCombat_DetectionStatesInitialised(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "P", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "N", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1},
	}
	cbt, err := eng.StartCombat("room", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	if cbt.DetectionStates == nil {
		t.Fatalf("DetectionStates must be initialised at StartCombat")
	}
	// Absent pair → Observed.
	if got := cbt.DetectionStates.Get("p1", "n1"); got != detection.Observed {
		t.Errorf("absent pair Get = %v, want Observed", got)
	}
}

// TestStartCombat_LegacyHiddenFlagPopulatesMapSymmetrically verifies the
// DETECT-5 back-compat shim: a combatant with Hidden=true at StartCombat is
// projected into the per-pair map symmetrically against every other
// combatant.
func TestStartCombat_LegacyHiddenFlagPopulatesMapSymmetrically(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "P1", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1},
		{ID: "p2", Kind: combat.KindPlayer, Name: "P2", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "N1", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1, Hidden: true},
	}
	cbt, err := eng.StartCombat("room", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	if got := cbt.DetectionStates.Get("p1", "n1"); got != detection.Hidden {
		t.Errorf("p1→n1 = %v, want Hidden (legacy shim)", got)
	}
	if got := cbt.DetectionStates.Get("p2", "n1"); got != detection.Hidden {
		t.Errorf("p2→n1 = %v, want Hidden (legacy shim)", got)
	}
	// Non-hidden combatants are untouched.
	if got := cbt.DetectionStates.Get("n1", "p1"); got != detection.Observed {
		t.Errorf("n1→p1 = %v, want Observed (asymmetric)", got)
	}
}

// TestStartRound_ResetsMadeSoundThisRound verifies that
// MadeSoundThisRound is cleared at the start of each round (DETECT-19).
func TestStartRound_ResetsMadeSoundThisRound(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "P", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "N", MaxHP: 10, CurrentHP: 10, AC: 10, Level: 1},
	}
	cbt, err := eng.StartCombat("room", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.Combatants[0].MadeSoundThisRound = true
	cbt.Combatants[1].MadeSoundThisRound = true
	_ = cbt.StartRound(3)
	for _, c := range cbt.Combatants {
		if c.MadeSoundThisRound {
			t.Errorf("%s: MadeSoundThisRound must reset at StartRound", c.Name)
		}
	}
}
