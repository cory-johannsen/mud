package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// TestTargetCategory_String ensures every named category renders its canonical name.
func TestTargetCategory_String(t *testing.T) {
	cases := []struct {
		c    combat.TargetCategory
		want string
	}{
		{combat.TargetUnknown, "unknown"},
		{combat.TargetSelf, "self"},
		{combat.TargetSingleAlly, "single_ally"},
		{combat.TargetSingleEnemy, "single_enemy"},
		{combat.TargetSingleAny, "single_any"},
		{combat.TargetAoEBurst, "aoe_burst"},
		{combat.TargetAoECone, "aoe_cone"},
		{combat.TargetAoELine, "aoe_line"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("TargetCategory(%d).String() = %q; want %q", tc.c, got, tc.want)
		}
	}
}

// TestTargetCategory_Predicates verifies IsAoE / IsSingle classification.
func TestTargetCategory_Predicates(t *testing.T) {
	if !combat.TargetAoEBurst.IsAoE() || !combat.TargetAoECone.IsAoE() || !combat.TargetAoELine.IsAoE() {
		t.Errorf("IsAoE missed an AoE category")
	}
	if combat.TargetSelf.IsAoE() || combat.TargetSingleEnemy.IsAoE() {
		t.Errorf("IsAoE returned true for non-AoE")
	}
	if !combat.TargetSingleAlly.IsSingle() || !combat.TargetSingleEnemy.IsSingle() || !combat.TargetSingleAny.IsSingle() {
		t.Errorf("IsSingle missed a single-target category")
	}
	if combat.TargetSelf.IsSingle() || combat.TargetAoEBurst.IsSingle() {
		t.Errorf("IsSingle returned true for non-single")
	}
}

// TestTargetingError_String ensures every error code renders its canonical name.
func TestTargetingError_String(t *testing.T) {
	cases := []struct {
		e    combat.TargetingError
		want string
	}{
		{combat.TargetOK, "ok"},
		{combat.ErrTargetMissing, "target_missing"},
		{combat.ErrTargetNotInCombat, "target_not_in_combat"},
		{combat.ErrTargetDead, "target_dead"},
		{combat.ErrOutOfRange, "out_of_range"},
		{combat.ErrLineOfFireBlocked, "line_of_fire_blocked"},
		{combat.ErrWrongCategory, "wrong_category"},
		{combat.ErrAoEShapeInvalid, "aoe_shape_invalid"},
	}
	for _, tc := range cases {
		if got := tc.e.String(); got != tc.want {
			t.Errorf("TargetingError(%d).String() = %q; want %q", tc.e, got, tc.want)
		}
	}
	if !combat.TargetOK.OK() {
		t.Errorf("TargetOK.OK() = false; want true")
	}
	if combat.ErrTargetMissing.OK() {
		t.Errorf("ErrTargetMissing.OK() = true; want false")
	}
}

// TestTargetingResult_ZeroValueOK confirms the zero value represents success.
func TestTargetingResult_ZeroValueOK(t *testing.T) {
	var r combat.TargetingResult
	if !r.OK() {
		t.Errorf("zero-value TargetingResult.OK() = false; want true")
	}
	if r.Err != combat.TargetOK {
		t.Errorf("zero-value Err = %v; want TargetOK", r.Err)
	}
}

// newTargetingCombat builds a small two-row combat for validation tests.
// Player p1 sits at (0,0); enemy n1 at (0,1) (5 ft away);
// enemy n2 at (5,5) (25 ft Chebyshev = 25 ft); ally NPC af1 at (1,0).
func newTargetingCombat(t *testing.T) *combat.Combat {
	t.Helper()
	cbt := &combat.Combat{
		RoomID:     "room",
		GridWidth:  10,
		GridHeight: 10,
	}
	p1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "P1", MaxHP: 10, CurrentHP: 10, GridX: 0, GridY: 0}
	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "P2", MaxHP: 10, CurrentHP: 10, GridX: 1, GridY: 0}
	n1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Goblin", MaxHP: 10, CurrentHP: 10, GridX: 0, GridY: 1, FactionID: "monsters"}
	n2 := &combat.Combatant{ID: "n2", Kind: combat.KindNPC, Name: "DistantOrc", MaxHP: 10, CurrentHP: 10, GridX: 5, GridY: 5, FactionID: "monsters"}
	dead := &combat.Combatant{ID: "n3", Kind: combat.KindNPC, Name: "Corpse", MaxHP: 10, CurrentHP: 0, GridX: 1, GridY: 1, FactionID: "monsters"}
	cbt.Combatants = []*combat.Combatant{p1, p2, n1, n2, dead}
	return cbt
}

// TestValidateSingleTarget_Table exercises every stage of the pipeline.
func TestValidateSingleTarget_Table(t *testing.T) {
	cbt := newTargetingCombat(t)
	actor := cbt.GetCombatant("p1")

	cases := []struct {
		name     string
		targetID string
		cat      combat.TargetCategory
		maxRange int
		want     combat.TargetingError
	}{
		{"self ok no uid", "", combat.TargetSelf, 0, combat.TargetOK},
		{"missing uid", "", combat.TargetSingleEnemy, 0, combat.ErrTargetMissing},
		{"not in combat", "ghost", combat.TargetSingleEnemy, 0, combat.ErrTargetNotInCombat},
		{"dead target", "n3", combat.TargetSingleEnemy, 0, combat.ErrTargetDead},
		{"single enemy ok", "n1", combat.TargetSingleEnemy, 0, combat.TargetOK},
		{"ally requested but enemy supplied", "n1", combat.TargetSingleAlly, 0, combat.ErrWrongCategory},
		{"single ally ok (player ally)", "p2", combat.TargetSingleAlly, 0, combat.TargetOK},
		{"single any (enemy)", "n1", combat.TargetSingleAny, 0, combat.TargetOK},
		{"single any (ally)", "p2", combat.TargetSingleAny, 0, combat.TargetOK},
		{"out of range", "n2", combat.TargetSingleEnemy, 10, combat.ErrOutOfRange},
		{"in range", "n2", combat.TargetSingleEnemy, 25, combat.TargetOK},
		{"unlimited range allows distant target", "n2", combat.TargetSingleEnemy, 0, combat.TargetOK},
		{"unsupported category routed here", "n1", combat.TargetAoEBurst, 0, combat.ErrWrongCategory},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := combat.ValidateSingleTarget(cbt, actor, tc.targetID, tc.cat, tc.maxRange, false)
			if got.Err != tc.want {
				t.Errorf("ValidateSingleTarget err = %s (%q); want %s", got.Err, got.Detail, tc.want)
			}
			if tc.want == combat.TargetOK && !got.OK() {
				t.Errorf("expected OK result, got %+v", got)
			}
			if tc.want != combat.TargetOK && got.OK() {
				t.Errorf("expected failure, got OK")
			}
			if tc.want != combat.TargetOK && got.Detail == "" {
				t.Errorf("expected non-empty Detail on failure, got empty")
			}
		})
	}
}

// TestValidateSingleTarget_NilGuards covers defensive nil checks.
func TestValidateSingleTarget_NilGuards(t *testing.T) {
	cbt := newTargetingCombat(t)
	actor := cbt.GetCombatant("p1")

	if r := combat.ValidateSingleTarget(nil, actor, "n1", combat.TargetSingleEnemy, 0, false); r.OK() {
		t.Errorf("expected failure with nil cbt")
	}
	if r := combat.ValidateSingleTarget(cbt, nil, "n1", combat.TargetSingleEnemy, 0, false); r.OK() {
		t.Errorf("expected failure with nil actor")
	}
}
