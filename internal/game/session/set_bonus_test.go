package session

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// ── REQ-EM-29: RecomputeSetBonuses ───────────────────────────────────────────

func TestRecomputeSetBonuses_NilSetReg_NoOp(t *testing.T) {
	sess := &PlayerSession{
		Equipment:  inventory.NewEquipment(),
		LoadoutSet: inventory.NewLoadoutSet(),
	}
	// Must not panic and SetBonusSummary remains zero.
	RecomputeSetBonuses(sess, nil)
	if sess.SetBonusSummary.ACBonus != 0 {
		t.Errorf("expected ACBonus=0 with nil setReg, got %d", sess.SetBonusSummary.ACBonus)
	}
}

func TestRecomputeSetBonuses_NilSession_NoOp(t *testing.T) {
	setReg := &inventory.SetRegistry{}
	// Must not panic.
	RecomputeSetBonuses(nil, setReg)
}

func TestRecomputeSetBonuses_EmptyEquipment_ZeroSummary(t *testing.T) {
	sess := &PlayerSession{
		Equipment:  inventory.NewEquipment(),
		LoadoutSet: inventory.NewLoadoutSet(),
	}
	setReg := &inventory.SetRegistry{}
	RecomputeSetBonuses(sess, setReg)
	if sess.SetBonusSummary.ACBonus != 0 {
		t.Errorf("expected ACBonus=0 for empty equipment, got %d", sess.SetBonusSummary.ACBonus)
	}
}

func TestRecomputeSetBonuses_ActiveBonusApplied(t *testing.T) {
	// Build a SetRegistry with one set whose pieces are all equipped.
	setReg := inventorySetRegWithACBonus("street_set", []string{"chest_item", "leg_item"}, 5)

	sess := &PlayerSession{
		Equipment:  inventory.NewEquipment(),
		LoadoutSet: inventory.NewLoadoutSet(),
	}
	// Equip both pieces.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "chest_item",
	}
	sess.Equipment.Armor[inventory.SlotLeftLeg] = &inventory.SlottedItem{
		ItemDefID: "leg_item",
	}

	RecomputeSetBonuses(sess, setReg)
	if sess.SetBonusSummary.ACBonus != 5 {
		t.Errorf("expected ACBonus=5 when full set equipped, got %d", sess.SetBonusSummary.ACBonus)
	}
}

func TestRecomputeSetBonuses_PartialSetNoBonusApplied(t *testing.T) {
	// Set requires both pieces but only one is equipped.
	setReg := inventorySetRegWithACBonus("street_set", []string{"chest_item", "leg_item"}, 5)

	sess := &PlayerSession{
		Equipment:  inventory.NewEquipment(),
		LoadoutSet: inventory.NewLoadoutSet(),
	}
	// Only equip chest.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "chest_item",
	}

	RecomputeSetBonuses(sess, setReg)
	if sess.SetBonusSummary.ACBonus != 0 {
		t.Errorf("expected ACBonus=0 for partial set, got %d", sess.SetBonusSummary.ACBonus)
	}
}

// inventorySetRegWithACBonus creates a SetRegistry with one set that has a single full-set
// AC bonus of acBonus. pieces is the list of item_def_ids required.
func inventorySetRegWithACBonus(setID string, pieces []string, acBonus int) *inventory.SetRegistry {
	reg := inventory.NewSetRegistry()
	setpieces := make([]inventory.SetPiece, len(pieces))
	for i, p := range pieces {
		setpieces[i] = inventory.SetPiece{ItemDefID: p}
	}
	set := &inventory.SetDef{
		ID:     setID,
		Name:   setID,
		Pieces: setpieces,
		Bonuses: []inventory.SetBonus{
			{
				Threshold:   inventory.SetThreshold{Count: len(pieces), IsFull: true},
				Description: "Full set AC bonus",
				Effect: inventory.SetEffect{
					Type:   "ac_bonus",
					Amount: acBonus,
				},
			},
		},
	}
	reg.Register(set)
	return reg
}
