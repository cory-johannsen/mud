package world_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
)

const trapZoneYAML = `
zone:
  id: test_zone
  name: Test Zone
  description: A zone with a trap.
  start_room: room1
  danger_level: dangerous
  trap_probabilities:
    room_trap_chance: 0.4
    cover_trap_chance: 0.6
    trap_pool:
      - template: mine
        weight: 3
      - template: bear_trap
        weight: 2
  rooms:
    - id: room1
      title: Test Room
      description: A room with a static trap.
      map_x: 0
      map_y: 0
      traps:
        - template: mine
          position: room
      equipment:
        - item_id: metal_cabinet
          description: Metal Cabinet
          max_count: 1
          immovable: true
          cover_tier: standard
          trap_template: bear_trap
`

func TestLoadZone_WithTraps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zone.yaml"), []byte(trapZoneYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	zone, err := world.LoadZone(filepath.Join(dir, "zone.yaml"))
	if err != nil {
		t.Fatalf("LoadZone: %v", err)
	}
	room, ok := zone.Rooms["room1"]
	if !ok {
		t.Fatal("room1 not found")
	}
	if len(room.Traps) != 1 {
		t.Fatalf("expected 1 trap, got %d", len(room.Traps))
	}
	if room.Traps[0].TemplateID != "mine" {
		t.Errorf("TemplateID: got %q, want mine", room.Traps[0].TemplateID)
	}
	if room.Traps[0].Position != "room" {
		t.Errorf("Position: got %q, want room", room.Traps[0].Position)
	}
	if zone.TrapProbabilities == nil {
		t.Fatal("TrapProbabilities is nil")
	}
	if zone.TrapProbabilities.RoomTrapChance == nil || *zone.TrapProbabilities.RoomTrapChance != 0.4 {
		t.Errorf("RoomTrapChance: expected 0.4")
	}
	if len(zone.TrapProbabilities.TrapPool) != 2 {
		t.Errorf("TrapPool: expected 2 entries, got %d", len(zone.TrapProbabilities.TrapPool))
	}
}

func TestLoadZone_EquipmentTrapTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zone.yaml"), []byte(trapZoneYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	zone, err := world.LoadZone(filepath.Join(dir, "zone.yaml"))
	if err != nil {
		t.Fatalf("LoadZone: %v", err)
	}
	room := zone.Rooms["room1"]
	if len(room.Equipment) == 0 {
		t.Fatal("expected at least 1 equipment item")
	}
	eq := room.Equipment[0]
	if eq.TrapTemplate != "bear_trap" {
		t.Errorf("TrapTemplate: got %q, want bear_trap", eq.TrapTemplate)
	}
}
