package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

func TestTemplate_AIDomain_LoadedFromYAML(t *testing.T) {
	tmpl := &npc.Template{
		ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 18, AC: 14,
		AIDomain: "ganger_combat",
	}
	if tmpl.AIDomain != "ganger_combat" {
		t.Fatalf("expected ganger_combat, got %q", tmpl.AIDomain)
	}
}

func TestInstance_AIDomain_CopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 18, AC: 14,
		AIDomain: "ganger_combat",
	}
	inst := npc.NewInstance("g1", tmpl, "pioneer_square")
	if inst.AIDomain != "ganger_combat" {
		t.Fatalf("expected AIDomain copied, got %q", inst.AIDomain)
	}
}

func TestManager_Move_UpdatesRoomID(t *testing.T) {
	mgr := npc.NewManager()
	tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
	inst, _ := mgr.Spawn(tmpl, "room_a")
	if err := mgr.Move(inst.ID, "room_b"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	updated, _ := mgr.Get(inst.ID)
	if updated.RoomID != "room_b" {
		t.Fatalf("expected room_b, got %q", updated.RoomID)
	}
}
