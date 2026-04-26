package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// TestCombatTargeting_ZeroValueSafe verifies nil and zero-value safety.
func TestCombatTargeting_ZeroValueSafe(t *testing.T) {
	var s *combat.CombatTargeting
	if got := s.TargetID(); got != "" {
		t.Errorf("nil.TargetID() = %q; want empty", got)
	}
	s.Set("anything")
	s.Clear()
	s.OnCombatStart(nil, nil)
	s.OnTargetDeath(nil, nil, "x")

	s2 := combat.NewCombatTargeting()
	if s2.TargetID() != "" {
		t.Errorf("new targeting has non-empty target")
	}
}

// TestCombatTargeting_SingleEnemyAutoStick verifies the auto-target rule:
// at combat start with exactly one living enemy, that enemy becomes sticky.
func TestCombatTargeting_SingleEnemyAutoStick(t *testing.T) {
	cbt := newTargetingCombat(t)
	// remove n2 so only n1 is alive on the enemy side; n3 is already dead.
	cbt.Combatants = []*combat.Combatant{
		cbt.GetCombatant("p1"),
		cbt.GetCombatant("n1"),
	}
	actor := cbt.GetCombatant("p1")

	s := combat.NewCombatTargeting()
	s.OnCombatStart(cbt, actor)
	if got := s.TargetID(); got != "n1" {
		t.Errorf("auto-stick TargetID = %q; want n1", got)
	}
}

// TestCombatTargeting_MultipleEnemiesNoAutoStick verifies that when more
// than one enemy is alive the sticky selection is left empty.
func TestCombatTargeting_MultipleEnemiesNoAutoStick(t *testing.T) {
	cbt := newTargetingCombat(t)
	actor := cbt.GetCombatant("p1")

	s := combat.NewCombatTargeting()
	s.OnCombatStart(cbt, actor)
	if got := s.TargetID(); got != "" {
		t.Errorf("auto-stick with 2 enemies = %q; want empty", got)
	}
}

// TestCombatTargeting_TargetDeathReselect verifies that when the sticky
// target dies and exactly one living enemy remains, the survivor is selected.
func TestCombatTargeting_TargetDeathReselect(t *testing.T) {
	cbt := newTargetingCombat(t)
	actor := cbt.GetCombatant("p1")
	s := combat.NewCombatTargeting()
	s.Set("n1")

	// kill n1, leaving only n2 alive among enemies.
	cbt.GetCombatant("n1").CurrentHP = 0
	s.OnTargetDeath(cbt, actor, "n1")
	if got := s.TargetID(); got != "n2" {
		t.Errorf("post-death reselect TargetID = %q; want n2", got)
	}
}

// TestCombatTargeting_TargetDeathOtherIgnored verifies that the death of a
// non-target combatant does not affect the sticky selection.
func TestCombatTargeting_TargetDeathOtherIgnored(t *testing.T) {
	cbt := newTargetingCombat(t)
	actor := cbt.GetCombatant("p1")
	s := combat.NewCombatTargeting()
	s.Set("n1")

	cbt.GetCombatant("n2").CurrentHP = 0
	s.OnTargetDeath(cbt, actor, "n2")
	if got := s.TargetID(); got != "n1" {
		t.Errorf("unrelated death changed TargetID = %q; want n1", got)
	}
}

// TestCombatTargeting_ClearReselect verifies Clear() drops the selection
// without re-evaluating.
func TestCombatTargeting_ClearReselect(t *testing.T) {
	s := combat.NewCombatTargeting()
	s.Set("n1")
	s.Clear()
	if got := s.TargetID(); got != "" {
		t.Errorf("Clear left TargetID = %q; want empty", got)
	}
}

// TestCombatTargeting_ResolveAndValidate covers inline override + sticky
// fallback + sticky update on success.
func TestCombatTargeting_ResolveAndValidate(t *testing.T) {
	cbt := newTargetingCombat(t)
	actor := cbt.GetCombatant("p1")
	s := combat.NewCombatTargeting()

	// no sticky, no inline → ErrTargetMissing.
	uid, res := s.ResolveAndValidate(cbt, actor, combat.TargetSingleEnemy, 0, "")
	if res.OK() || res.Err != combat.ErrTargetMissing {
		t.Errorf("expected ErrTargetMissing, got %v", res)
	}
	if uid != "" {
		t.Errorf("expected empty uid, got %q", uid)
	}

	// inline override succeeds → sticky updated.
	uid, res = s.ResolveAndValidate(cbt, actor, combat.TargetSingleEnemy, 0, "n1")
	if !res.OK() {
		t.Fatalf("expected OK, got %v", res)
	}
	if uid != "n1" {
		t.Errorf("uid = %q; want n1", uid)
	}
	if s.TargetID() != "n1" {
		t.Errorf("sticky not updated after success: TargetID = %q", s.TargetID())
	}

	// no inline → sticky reused.
	uid, res = s.ResolveAndValidate(cbt, actor, combat.TargetSingleEnemy, 0, "")
	if !res.OK() || uid != "n1" {
		t.Errorf("sticky reuse failed: uid=%q res=%v", uid, res)
	}

	// inline override to a dead target → failure, sticky NOT clobbered.
	_, res = s.ResolveAndValidate(cbt, actor, combat.TargetSingleEnemy, 0, "n3")
	if res.OK() || res.Err != combat.ErrTargetDead {
		t.Errorf("expected ErrTargetDead, got %v", res)
	}
	if s.TargetID() != "n1" {
		t.Errorf("sticky clobbered on failure: %q", s.TargetID())
	}
}
