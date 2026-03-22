package gameserver_test

import (
	"math/rand"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/gameserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDurabilityRoller is a deterministic Roller for durability tests.
type stubDurabilityRoller struct {
	floatResult float64
	rollResult  int
}

func (s *stubDurabilityRoller) Roll(_ string) int    { return s.rollResult }
func (s *stubDurabilityRoller) RollD20() int         { return 10 }
func (s *stubDurabilityRoller) RollFloat() float64   { return s.floatResult }

// makeSwordWeapon creates a WeaponDef with street rarity.
func makeSwordWeapon() *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID: "sword", Name: "Sword",
		DamageDice: "1d6", DamageType: "slashing",
		ProficiencyCategory: "martial_melee",
		Rarity:              "street",
	}
}

// makeAttackEvent creates a RoundEvent for an attack action.
func makeAttackEvent(actorID, targetID string, outcome combat.Outcome) combat.RoundEvent {
	return combat.RoundEvent{
		ActionType: combat.ActionAttack,
		ActorID:    actorID,
		TargetID:   targetID,
		AttackResult: &combat.AttackResult{
			AttackerID: actorID,
			TargetID:   targetID,
			Outcome:    outcome,
			BaseDamage: 5,
		},
	}
}

// makeEquippedWeapon creates an EquippedWeapon with the given durability.
func makeEquippedWeapon(durability int) *inventory.EquippedWeapon {
	return &inventory.EquippedWeapon{
		Def:        makeSwordWeapon(),
		Durability: durability,
		InstanceID: "inst-weapon-1",
	}
}

// makeEquipmentWithArmor creates an Equipment with one armor slot occupied.
func makeEquipmentWithArmor(durability int, rarity string) *inventory.Equipment {
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "chest_armor",
		Name:       "Chest Armor",
		InstanceID: "inst-armor-1",
		Durability: durability,
		Rarity:     rarity,
	}
	return eq
}

// TestApplyRoundDurability_WeaponDurabilityDeducted verifies REQ-EM-5: weapon
// durability decreases by 1 after an attack action.
func TestApplyRoundDurability_WeaponDurabilityDeducted(t *testing.T) {
	ew := makeEquippedWeapon(10)
	events := []combat.RoundEvent{makeAttackEvent("player1", "npc1", combat.Success)}
	rng := &stubDurabilityRoller{floatResult: 0.99}
	rnd := rand.New(rand.NewSource(42))

	var notifications []string
	gameserver.ApplyRoundDurability(
		events,
		func(id string) *inventory.EquippedWeapon { if id == "player1" { return ew }; return nil },
		func(id string) *inventory.Equipment { return nil },
		func(_ string, _ inventory.ArmorSlot) {},
		func(_, msg string) { notifications = append(notifications, msg) },
		rng,
		rnd,
	)

	assert.Equal(t, 9, ew.Durability, "weapon durability should decrease by 1")
	assert.Empty(t, notifications, "no notification when durability > 0")
}

// TestApplyRoundDurability_WeaponDestroyed verifies REQ-EM-10: notifies when weapon
// durability reaches 0 and destruction roll succeeds.
func TestApplyRoundDurability_WeaponDestroyed(t *testing.T) {
	ew := makeEquippedWeapon(1) // one hit away from 0
	events := []combat.RoundEvent{makeAttackEvent("player1", "npc1", combat.Success)}
	// street rarity: DestructionChance=0.30; roll 0.10 < 0.30 → destroyed
	rng := &stubDurabilityRoller{floatResult: 0.10}
	rnd := rand.New(rand.NewSource(42))

	var destroyedFor string
	var msgs []string
	gameserver.ApplyRoundDurability(
		events,
		func(id string) *inventory.EquippedWeapon { if id == "player1" { return ew }; return nil },
		func(id string) *inventory.Equipment { return nil },
		func(_ string, _ inventory.ArmorSlot) {},
		func(actorID, msg string) { destroyedFor = actorID; msgs = append(msgs, msg) },
		rng,
		rnd,
	)

	assert.Equal(t, 0, ew.Durability)
	assert.Equal(t, "player1", destroyedFor)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "Sword")
	assert.Contains(t, msgs[0], "destroyed")
}

// TestApplyRoundDurability_ArmorDurabilityDeducted verifies REQ-EM-6: armor
// durability decreases by 1 when a hit lands.
func TestApplyRoundDurability_ArmorDurabilityDeducted(t *testing.T) {
	eq := makeEquipmentWithArmor(10, "street")
	events := []combat.RoundEvent{makeAttackEvent("npc1", "player1", combat.Success)}
	rng := &stubDurabilityRoller{floatResult: 0.99}
	rnd := rand.New(rand.NewSource(42))

	gameserver.ApplyRoundDurability(
		events,
		func(_ string) *inventory.EquippedWeapon { return nil },
		func(id string) *inventory.Equipment { if id == "player1" { return eq }; return nil },
		func(_ string, _ inventory.ArmorSlot) {},
		func(_, _ string) {},
		rng,
		rnd,
	)

	si := eq.Armor[inventory.SlotTorso]
	require.NotNil(t, si)
	assert.Equal(t, 9, si.Durability, "armor durability should decrease by 1 on hit")
}

// TestApplyRoundDurability_ArmorDestroyedAndSlotCleared verifies REQ-EM-10:
// when armor is destroyed, the slot is cleared and player is notified.
func TestApplyRoundDurability_ArmorDestroyedAndSlotCleared(t *testing.T) {
	eq := makeEquipmentWithArmor(1, "street")
	events := []combat.RoundEvent{makeAttackEvent("npc1", "player1", combat.Success)}
	// street: DestructionChance=0.30; roll 0.10 < 0.30 → destroyed
	rng := &stubDurabilityRoller{floatResult: 0.10}
	rnd := rand.New(rand.NewSource(42))

	var removedSlot inventory.ArmorSlot
	var removedFor string
	var msgs []string
	gameserver.ApplyRoundDurability(
		events,
		func(_ string) *inventory.EquippedWeapon { return nil },
		func(id string) *inventory.Equipment { if id == "player1" { return eq }; return nil },
		func(targetID string, slot inventory.ArmorSlot) { removedFor = targetID; removedSlot = slot; eq.Armor[slot] = nil },
		func(_, msg string) { msgs = append(msgs, msg) },
		rng,
		rnd,
	)

	assert.Equal(t, "player1", removedFor)
	assert.Equal(t, inventory.SlotTorso, removedSlot)
	assert.Nil(t, eq.Armor[inventory.SlotTorso], "slot should be cleared after destruction")
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "Chest Armor")
	assert.Contains(t, msgs[0], "destroyed")
}

// TestApplyRoundDurability_MissDoesNotDeductArmor verifies REQ-EM-6: misses do not
// deduct from armor durability.
func TestApplyRoundDurability_MissDoesNotDeductArmor(t *testing.T) {
	eq := makeEquipmentWithArmor(10, "street")
	events := []combat.RoundEvent{makeAttackEvent("npc1", "player1", combat.Failure)}
	rng := &stubDurabilityRoller{floatResult: 0.99}
	rnd := rand.New(rand.NewSource(42))

	gameserver.ApplyRoundDurability(
		events,
		func(_ string) *inventory.EquippedWeapon { return nil },
		func(id string) *inventory.Equipment { if id == "player1" { return eq }; return nil },
		func(_ string, _ inventory.ArmorSlot) {},
		func(_, _ string) {},
		rng,
		rnd,
	)

	si := eq.Armor[inventory.SlotTorso]
	require.NotNil(t, si)
	assert.Equal(t, 10, si.Durability, "armor durability should not change on miss")
}

// TestApplyRoundDurability_NoWeaponNoOp verifies no panic when actor has no weapon.
func TestApplyRoundDurability_NoWeaponNoOp(t *testing.T) {
	events := []combat.RoundEvent{makeAttackEvent("player1", "npc1", combat.Success)}
	rng := &stubDurabilityRoller{floatResult: 0.99}
	rnd := rand.New(rand.NewSource(42))

	gameserver.ApplyRoundDurability(
		events,
		func(_ string) *inventory.EquippedWeapon { return nil },
		func(_ string) *inventory.Equipment { return nil },
		func(_ string, _ inventory.ArmorSlot) {},
		func(_, _ string) {},
		rng,
		rnd,
	)
	// No panic = pass.
}
