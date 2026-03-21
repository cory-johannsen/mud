package world_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/world"
)

func TestZoneDangerLevelYAMLRoundTrip(t *testing.T) {
	trapChance := 25
	input := `
id: test_zone
name: Test Zone
description: A test zone
danger_level: dangerous
room_trap_chance: 25
`
	var z world.Zone
	if err := yaml.Unmarshal([]byte(input), &z); err != nil {
		t.Fatalf("unmarshal Zone: %v", err)
	}
	if z.DangerLevel != "dangerous" {
		t.Errorf("Zone.DangerLevel = %q; want %q", z.DangerLevel, "dangerous")
	}
	if z.RoomTrapChance == nil {
		t.Fatal("Zone.RoomTrapChance is nil; want non-nil")
	}
	if *z.RoomTrapChance != trapChance {
		t.Errorf("Zone.RoomTrapChance = %d; want %d", *z.RoomTrapChance, trapChance)
	}

	out, err := yaml.Marshal(z)
	if err != nil {
		t.Fatalf("marshal Zone: %v", err)
	}
	var z2 world.Zone
	if err := yaml.Unmarshal(out, &z2); err != nil {
		t.Fatalf("re-unmarshal Zone: %v", err)
	}
	if z2.DangerLevel != "dangerous" {
		t.Errorf("round-trip Zone.DangerLevel = %q; want %q", z2.DangerLevel, "dangerous")
	}
}

func TestRoomDangerLevelOverride(t *testing.T) {
	input := `
id: test_room
zone_id: test_zone
title: Test Room
description: A test room
danger_level: safe
`
	var r world.Room
	if err := yaml.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal Room: %v", err)
	}
	if r.DangerLevel != "safe" {
		t.Errorf("Room.DangerLevel = %q; want %q", r.DangerLevel, "safe")
	}
}

func TestRoomEquipmentConfigCoverTier(t *testing.T) {
	input := `
item_id: barrel
description: A heavy barrel
cover_tier: heavy
`
	var rec world.RoomEquipmentConfig
	if err := yaml.Unmarshal([]byte(input), &rec); err != nil {
		t.Fatalf("unmarshal RoomEquipmentConfig: %v", err)
	}
	if rec.CoverTier != "heavy" {
		t.Errorf("RoomEquipmentConfig.CoverTier = %q; want %q", rec.CoverTier, "heavy")
	}
}

func TestZoneCoverTrapChanceYAMLRoundTrip(t *testing.T) {
	coverChance := 30
	input := `
id: test_zone2
name: Test Zone 2
description: Another test zone
danger_level: all_out_war
cover_trap_chance: 30
`
	var z world.Zone
	if err := yaml.Unmarshal([]byte(input), &z); err != nil {
		t.Fatalf("unmarshal Zone: %v", err)
	}
	if z.CoverTrapChance == nil {
		t.Fatal("Zone.CoverTrapChance is nil; want non-nil")
	}
	if *z.CoverTrapChance != coverChance {
		t.Errorf("Zone.CoverTrapChance = %d; want %d", *z.CoverTrapChance, coverChance)
	}
}
