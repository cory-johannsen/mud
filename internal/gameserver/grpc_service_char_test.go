package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"pgregory.net/rapid"
)

func TestHandleChar_HappyPath_ReturnsCharacterSheetView(t *testing.T) {
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            "room1",
		CurrentHP:         15,
		MaxHP:             20,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the North",
		Class:             "Gunner",
		Level:             3,
	})

	s := &GameServiceServer{sessions: mgr}
	result, err := s.handleChar("uid1")
	if err != nil {
		t.Fatalf("handleChar returned unexpected error: %v", err)
	}
	cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
	if !ok {
		t.Fatalf("expected ServerEvent_CharacterSheet payload, got %T", result.Payload)
	}
	if cs.CharacterSheet.Name != "Hero" {
		t.Errorf("Name = %q, want %q", cs.CharacterSheet.Name, "Hero")
	}
	if cs.CharacterSheet.Level != 3 {
		t.Errorf("Level = %d, want 3", cs.CharacterSheet.Level)
	}
	if cs.CharacterSheet.CurrentHp != 15 {
		t.Errorf("CurrentHp = %d, want 15", cs.CharacterSheet.CurrentHp)
	}
	if cs.CharacterSheet.MaxHp != 20 {
		t.Errorf("MaxHp = %d, want 20", cs.CharacterSheet.MaxHp)
	}
}

func TestHandleChar_SessionNotFound_ReturnsErrorEvent(t *testing.T) {
	mgr := session.NewManager()
	s := &GameServiceServer{sessions: mgr}
	result, err := s.handleChar("nonexistent-uid")
	if err != nil {
		t.Fatalf("handleChar returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for missing session")
	}
	if _, ok := result.Payload.(*gamev1.ServerEvent_Error); !ok {
		t.Fatalf("expected ServerEvent_Error payload for missing session, got %T", result.Payload)
	}
}

func TestHandleChar_NilJobRegistry_DoesNotPanic_ReturnsJobAsClass(t *testing.T) {
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Ranger",
		CharacterID:       1,
		RoomID:            "room1",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the West",
		Class:             "Scout",
		Level:             1,
	})

	// jobRegistry is explicitly nil — must not panic.
	s := &GameServiceServer{sessions: mgr, jobRegistry: nil}
	result, err := s.handleChar("uid1")
	if err != nil {
		t.Fatalf("handleChar returned unexpected error: %v", err)
	}
	cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
	if !ok {
		t.Fatalf("expected ServerEvent_CharacterSheet payload, got %T", result.Payload)
	}
	if cs.CharacterSheet.Job != "Scout" {
		t.Errorf("Job = %q, want %q (class fallback)", cs.CharacterSheet.Job, "Scout")
	}
}

func TestProperty_HandleChar_NilJobRegistry_AlwaysReturnsClassAsJob(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		className := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "className")
		charName := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "charName")

		mgr := session.NewManager()
		_, _ = mgr.AddPlayer(session.AddPlayerOptions{
			UID:               "uid1",
			Username:          "user1",
			CharName:          charName,
			CharacterID:       1,
			RoomID:            "room1",
			CurrentHP:         10,
			MaxHP:             10,
			Abilities:         character.AbilityScores{},
			Role:              "player",
			RegionDisplayName: "the East",
			Class:             className,
			Level:             1,
		})

		s := &GameServiceServer{sessions: mgr, jobRegistry: nil}
		result, err := s.handleChar("uid1")
		if err != nil {
			t.Fatalf("handleChar error: %v", err)
		}
		cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
		if !ok {
			t.Fatalf("expected CharacterSheet, got %T", result.Payload)
		}
		if cs.CharacterSheet.Job != className {
			t.Fatalf("Job = %q, want %q", cs.CharacterSheet.Job, className)
		}
		if cs.CharacterSheet.Name != charName {
			t.Fatalf("Name = %q, want %q", cs.CharacterSheet.Name, charName)
		}
	})
}

// TestHandleChar_ArmorProficiencyBonus_AppliedOnce verifies that ProficiencyAcBonus and
// EffectiveArmorCategory are populated on the CharacterSheetView, that AcBonus contains
// only item AC contributions (proficiency excluded for UI breakdown), and that TotalAc is
// the correct sum: 10 + Dex + (item AC + proficiency AC).
//
// Setup: level 8 character, Quickness=14 (dexMod=2), trained medium_armor, 7 slots × AC+1.
// Proficiency bonus for trained at level 8 = level + 2 = 10.
// Expected: ACBonus (full) = 7 + 10 = 17, TotalAc = 10 + 2 + 17 = 29.
// view.AcBonus (items only) = 7, view.ProficiencyAcBonus = 10, view.TotalAc = 29.
func TestHandleChar_ArmorProficiencyBonus_AppliedOnce(t *testing.T) {
	// Build an inventory registry with 7 medium armor pieces (AC+1 each).
	reg := inventory.NewRegistry()
	slots := []inventory.ArmorSlot{
		inventory.SlotHead,
		inventory.SlotLeftArm,
		inventory.SlotRightArm,
		inventory.SlotTorso,
		inventory.SlotHands,
		inventory.SlotLeftLeg,
		inventory.SlotRightLeg,
	}
	for i, slot := range slots {
		id := "med_piece_" + string(slot)
		def := &inventory.ArmorDef{
			ID:                  id,
			Name:                "Med " + string(slot),
			Slot:                slot,
			Group:               "composite",
			ACBonus:             1,
			DexCap:              10,
			ProficiencyCategory: "medium_armor",
			Rarity:              "street",
		}
		_ = i
		if err := reg.RegisterArmor(def); err != nil {
			t.Fatalf("RegisterArmor(%q): %v", id, err)
		}
	}

	mgr := session.NewManager()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid_prof",
		Username:          "prof_user",
		CharName:          "Armored",
		CharacterID:       42,
		RoomID:            "room1",
		CurrentHP:         100,
		MaxHP:             100,
		Abilities:         character.AbilityScores{Quickness: 14},
		Role:              "player",
		RegionDisplayName: "the Range",
		Class:             "Fighter",
		Level:             8,
	})
	if err != nil {
		t.Fatalf("AddPlayer: %v", err)
	}

	// Equip all 7 medium armor pieces.
	for _, slot := range slots {
		sess.Equipment.Armor[slot] = &inventory.SlottedItem{
			ItemDefID:  "med_piece_" + string(slot),
			Name:       "Med " + string(slot),
			Durability: 10,
		}
	}
	// Set proficiencies: trained in medium_armor.
	sess.Proficiencies = map[string]string{
		"light_armor":  "trained",
		"medium_armor": "trained",
	}

	s := &GameServiceServer{
		sessions:    mgr,
		invRegistry: reg,
	}
	result, err := s.handleChar("uid_prof")
	if err != nil {
		t.Fatalf("handleChar: %v", err)
	}
	cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
	if !ok {
		t.Fatalf("expected ServerEvent_CharacterSheet, got %T", result.Payload)
	}
	view := cs.CharacterSheet

	// Proficiency bonus for trained at level 8 = 8 + 2 = 10.
	// 7 items × AC+1 = 7 item AC. Full ACBonus = 7 + 10 = 17.
	// TotalAc = 10 + 2(dex) + 17 = 29.
	// view.AcBonus = items only = 7.
	// view.ProficiencyAcBonus = 10.
	if view.TotalAc != 29 {
		t.Errorf("TotalAc = %d, want 29", view.TotalAc)
	}
	if view.AcBonus != 7 {
		t.Errorf("AcBonus (items only) = %d, want 7", view.AcBonus)
	}
	if view.ProficiencyAcBonus != 10 {
		t.Errorf("ProficiencyAcBonus = %d, want 10", view.ProficiencyAcBonus)
	}
	if view.EffectiveArmorCategory != "medium_armor" {
		t.Errorf("EffectiveArmorCategory = %q, want %q", view.EffectiveArmorCategory, "medium_armor")
	}
}
