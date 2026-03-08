package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver"
)

const statusIdle = int32(0)
const statusInCombat = int32(2)

func TestAvailableActions_CombatContext(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "surge", Active: true, Contexts: []string{"combat"}},
		{ID: "patch", Active: true, Contexts: []string{"exploration"}},
		{ID: "passive", Active: false},
	}
	reg := ruleset.NewClassFeatureRegistry(features)
	sess := &session.PlayerSession{
		Status:       statusInCombat,
		PassiveFeats: map[string]bool{"surge": true, "patch": true, "passive": true},
	}
	actions := gameserver.AvailableActions(sess, reg, "combat")
	if len(actions) != 1 {
		t.Fatalf("expected 1 action in combat, got %d", len(actions))
	}
	if actions[0].ID != "surge" {
		t.Errorf("wrong action: %s", actions[0].ID)
	}
}

func TestAvailableActions_ExplorationContext(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "surge", Active: true, Contexts: []string{"combat"}},
		{ID: "patch", Active: true, Contexts: []string{"exploration"}},
	}
	reg := ruleset.NewClassFeatureRegistry(features)
	sess := &session.PlayerSession{
		Status:       statusIdle,
		PassiveFeats: map[string]bool{"surge": true, "patch": true},
	}
	actions := gameserver.AvailableActions(sess, reg, "exploration")
	if len(actions) != 1 {
		t.Fatalf("expected 1 action in exploration, got %d", len(actions))
	}
	if actions[0].ID != "patch" {
		t.Errorf("wrong action: %s", actions[0].ID)
	}
}

func TestAvailableActions_UnownedFeature(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "surge", Active: true, Contexts: []string{"combat"}},
	}
	reg := ruleset.NewClassFeatureRegistry(features)
	sess := &session.PlayerSession{
		Status:       statusInCombat,
		PassiveFeats: map[string]bool{},
	}
	actions := gameserver.AvailableActions(sess, reg, "combat")
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestContextForSession_InCombat(t *testing.T) {
	sess := &session.PlayerSession{Status: statusInCombat}
	ctx := gameserver.ContextForSession(sess)
	if ctx != "combat" {
		t.Errorf("got %q, want %q", ctx, "combat")
	}
}

func TestContextForSession_Idle(t *testing.T) {
	sess := &session.PlayerSession{Status: statusIdle}
	ctx := gameserver.ContextForSession(sess)
	if ctx != "exploration" {
		t.Errorf("got %q, want %q", ctx, "exploration")
	}
}
