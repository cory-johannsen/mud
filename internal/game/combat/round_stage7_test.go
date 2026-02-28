package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// fixedSrc is a deterministic dice source that always returns the same value modulo n.
type fixedSrc7 struct{ val int }

func (f fixedSrc7) Intn(n int) int {
	v := f.val % n
	if v < 0 {
		v = 0
	}
	return v
}

func makeEngine7() *combat.Engine {
	return combat.NewEngine()
}

func makeCombatants7() (*combat.Combatant, *combat.Combatant) {
	player := &combat.Combatant{
		ID: "player1", Kind: combat.KindPlayer,
		Name: "Hero", MaxHP: 30, CurrentHP: 30,
		AC: 15, Level: 1, StrMod: 2, DexMod: 1, Initiative: 20,
	}
	npc := &combat.Combatant{
		ID: "npc1", Kind: combat.KindNPC,
		Name: "Goblin", MaxHP: 20, CurrentHP: 20,
		AC: 12, Level: 1, StrMod: 0, DexMod: 0, Initiative: 10,
	}
	return player, npc
}

// TestResolveRound_Reload_RestoresMagazine verifies that ActionReload restores a
// depleted magazine to full capacity.
func TestResolveRound_Reload_RestoresMagazine(t *testing.T) {
	player, npc := makeCombatants7()

	// Build loadout with a pistol (capacity=15).
	pistolDef := &inventory.WeaponDef{
		ID: "pistol", Name: "Pistol",
		DamageDice: "1d6", DamageType: "piercing",
		RangeIncrement: 30, ReloadActions: 1, MagazineCapacity: 15,
		FiringModes: []inventory.FiringMode{inventory.FiringModeSingle},
	}
	preset := inventory.NewWeaponPreset()
	if err := preset.EquipMainHand(pistolDef); err != nil {
		t.Fatalf("EquipMainHand failed: %v", err)
	}
	// Consume 10 rounds.
	eq := preset.MainHand
	for i := 0; i < 10; i++ {
		if err := eq.Magazine.Consume(1); err != nil {
			t.Fatalf("Consume failed: %v", err)
		}
	}
	if eq.Magazine.Loaded != 5 {
		t.Fatalf("expected 5 rounds after consuming 10, got %d", eq.Magazine.Loaded)
	}
	player.Loadout = preset

	condReg := condition.NewRegistry()
	eng := makeEngine7()
	cbt, err := eng.StartCombat("room1", []*combat.Combatant{player, npc}, condReg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	cbt.StartRound(3)

	if err := cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionReload, WeaponID: "pistol"}); err != nil {
		t.Fatalf("QueueAction reload: %v", err)
	}
	if err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction pass: %v", err)
	}

	src := fixedSrc7{val: 10}
	events := combat.ResolveRound(cbt, src, nil)
	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}

	// Magazine must be fully restored.
	if eq.Magazine.Loaded != 15 {
		t.Errorf("expected magazine.Loaded=15 after reload, got %d", eq.Magazine.Loaded)
	}
}

// TestResolveRound_FireBurst_ProducesTwoEvents verifies that ActionFireBurst emits
// at least 2 RoundEvents attributed to the acting player.
func TestResolveRound_FireBurst_ProducesTwoEvents(t *testing.T) {
	player, npc := makeCombatants7()

	shotgunDef := &inventory.WeaponDef{
		ID: "shotgun", Name: "Shotgun",
		DamageDice: "2d6", DamageType: "piercing",
		RangeIncrement: 20, ReloadActions: 1, MagazineCapacity: 8,
		FiringModes: []inventory.FiringMode{inventory.FiringModeBurst},
	}
	preset := inventory.NewWeaponPreset()
	if err := preset.EquipMainHand(shotgunDef); err != nil {
		t.Fatalf("EquipMainHand failed: %v", err)
	}
	player.Loadout = preset

	condReg := condition.NewRegistry()
	eng := makeEngine7()
	cbt, err := eng.StartCombat("room2", []*combat.Combatant{player, npc}, condReg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	cbt.StartRound(3)

	if err := cbt.QueueAction("player1", combat.QueuedAction{
		Type: combat.ActionFireBurst, Target: "Goblin", WeaponID: "shotgun",
	}); err != nil {
		t.Fatalf("QueueAction burst: %v", err)
	}
	if err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction pass: %v", err)
	}

	// Use a high roll so hits land and the goblin survives both shots.
	src := fixedSrc7{val: 15}
	events := combat.ResolveRound(cbt, src, nil)

	playerEvents := 0
	for _, e := range events {
		if e.ActorID == "player1" && e.ActionType == combat.ActionFireBurst {
			playerEvents++
		}
	}
	if playerEvents < 2 {
		t.Errorf("expected >= 2 burst events for player1, got %d (total events: %d)", playerEvents, len(events))
	}
}

// TestResolveRound_Throw_ProducesThrowEvents verifies that ActionThrow emits at
// least one RoundEvent with ActionType == ActionThrow.
func TestResolveRound_Throw_ProducesThrowEvents(t *testing.T) {
	player, npc := makeCombatants7()

	grenade := &inventory.ExplosiveDef{
		ID: "frag_grenade", Name: "Frag Grenade",
		DamageDice: "3d6", DamageType: "piercing",
		SaveDC: 15,
	}
	reg := inventory.NewRegistry()
	if err := reg.RegisterExplosive(grenade); err != nil {
		t.Fatalf("RegisterExplosive: %v", err)
	}

	condReg := condition.NewRegistry()
	eng := makeEngine7()
	cbt, err := eng.StartCombat("room3", []*combat.Combatant{player, npc}, condReg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.SetInventoryRegistry(reg)

	cbt.StartRound(3)

	if err := cbt.QueueAction("player1", combat.QueuedAction{
		Type: combat.ActionThrow, ExplosiveID: "frag_grenade",
	}); err != nil {
		t.Fatalf("QueueAction throw: %v", err)
	}
	if err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction pass: %v", err)
	}

	src := fixedSrc7{val: 10}
	events := combat.ResolveRound(cbt, src, nil)

	throwCount := 0
	for _, e := range events {
		if e.ActionType == combat.ActionThrow {
			throwCount++
		}
	}
	if throwCount < 1 {
		t.Errorf("expected >= 1 throw event, got %d", throwCount)
	}
}
