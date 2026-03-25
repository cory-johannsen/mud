package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// stubAffixRoller implements inventory.Roller for affix tests.
type stubAffixRoller struct {
	d20      int
	floatVal float64
}

func (s stubAffixRoller) Roll(_ string) int      { return s.d20 }
func (s stubAffixRoller) RollD20() int           { return s.d20 }
func (s stubAffixRoller) RollFloat() float64     { return s.floatVal }

// buildAffixTestRegistryRT constructs a registry with the minimal items needed for rapid property tests.
func buildAffixTestRegistryRT(rt *rapid.T) *inventory.Registry {
	reg := inventory.NewRegistry()
	for _, def := range []*inventory.ItemDef{
		{
			ID: "scrap_iron_street_grade", Name: "Scrap Iron (Street Grade)",
			Kind:         inventory.KindPreciousMaterial,
			MaterialID:   "scrap_iron", GradeID: "street_grade",
			MaterialName: "Scrap Iron", MaterialTier: "common",
			AppliesTo:    []string{"weapon"},
			MaxStack:     1,
		},
	} {
		if err := reg.RegisterItem(def); err != nil {
			rt.Fatalf("buildAffixTestRegistryRT: RegisterItem %s: %v", def.ID, err)
		}
		matDef, err := inventory.MaterialDefFromItemDef(def)
		if err != nil {
			rt.Fatalf("buildAffixTestRegistryRT: MaterialDefFromItemDef %s: %v", def.ID, err)
		}
		if err := reg.RegisterMaterial(matDef); err != nil {
			rt.Fatalf("buildAffixTestRegistryRT: RegisterMaterial %s: %v", def.ID, err)
		}
	}
	wd := &inventory.WeaponDef{
		ID: "test_pistol_mil_spec", Name: "Test Pistol Mil-Spec",
		DamageDice: "1d4", DamageType: "piercing",
		ProficiencyCategory: "simple_ranged",
		Rarity: "mil_spec", UpgradeSlots: rarityToSlots("mil_spec"),
	}
	if err := reg.RegisterWeapon(wd); err != nil {
		rt.Fatalf("buildAffixTestRegistryRT: RegisterWeapon %s: %v", wd.ID, err)
	}
	return reg
}

// buildAffixTestRegistry constructs a registry with the minimal items needed for affix tests.
func buildAffixTestRegistry(t testing.TB) *inventory.Registry {
	t.Helper()
	reg := inventory.NewRegistry()
	for _, def := range []*inventory.ItemDef{
		{
			ID: "scrap_iron_street_grade", Name: "Scrap Iron (Street Grade)",
			Kind:         inventory.KindPreciousMaterial,
			MaterialID:   "scrap_iron", GradeID: "street_grade",
			MaterialName: "Scrap Iron", MaterialTier: "common",
			AppliesTo:    []string{"weapon"},
			MaxStack:     1,
		},
		{
			ID: "carbon_weave_street_grade", Name: "Carbon Weave (Street Grade)",
			Kind:         inventory.KindPreciousMaterial,
			MaterialID:   "carbon_weave", GradeID: "street_grade",
			MaterialName: "Carbon Weave", MaterialTier: "common",
			AppliesTo:    []string{"armor"},
			MaxStack:     1,
		},
		{
			ID: "hollow_point_street_grade", Name: "Hollow Point (Street Grade)",
			Kind:         inventory.KindPreciousMaterial,
			MaterialID:   "hollow_point", GradeID: "street_grade",
			MaterialName: "Hollow Point", MaterialTier: "common",
			AppliesTo:    []string{"weapon"},
			MaxStack:     1,
		},
	} {
		if err := reg.RegisterItem(def); err != nil {
			t.Fatalf("buildAffixTestRegistry: RegisterItem %s: %v", def.ID, err)
		}
		matDef, err := inventory.MaterialDefFromItemDef(def)
		if err != nil {
			t.Fatalf("buildAffixTestRegistry: MaterialDefFromItemDef %s: %v", def.ID, err)
		}
		if err := reg.RegisterMaterial(matDef); err != nil {
			t.Fatalf("buildAffixTestRegistry: RegisterMaterial %s: %v", def.ID, err)
		}
	}
	for _, wd := range []*inventory.WeaponDef{
		{
			ID: "test_pistol_street", Name: "Test Pistol",
			DamageDice: "1d4", DamageType: "piercing",
			ProficiencyCategory: "simple_ranged",
			Rarity: "street", UpgradeSlots: rarityToSlots("street"),
		},
		{
			ID: "test_pistol_mil_spec", Name: "Test Pistol Mil-Spec",
			DamageDice: "1d4", DamageType: "piercing",
			ProficiencyCategory: "simple_ranged",
			Rarity: "mil_spec", UpgradeSlots: rarityToSlots("mil_spec"),
		},
	} {
		if err := reg.RegisterWeapon(wd); err != nil {
			t.Fatalf("buildAffixTestRegistry: RegisterWeapon %s: %v", wd.ID, err)
		}
	}
	return reg
}

func rarityToSlots(rarity string) int {
	switch rarity {
	case "street":
		return 1
	case "mil_spec":
		return 2
	default:
		return 0
	}
}

func makeAffixSession() *session.PlayerSession {
	sess := &session.PlayerSession{Status: 1}
	sess.Backpack = inventory.NewBackpack(20, 100.0)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	sess.Equipment = inventory.NewEquipment()
	sess.MaterialState = inventory.MaterialSessionState{
		CombatUsed: make(map[string]bool),
		DailyUsed:  make(map[string]int),
	}
	sess.Abilities = character.AbilityScores{
		Brutality: 10, Grit: 10, Quickness: 10,
		Reasoning: 10, Savvy: 10, Flair: 10,
	}
	return sess
}

func addMaterialToAffixBackpack(t testing.TB, sess *session.PlayerSession, reg *inventory.Registry, itemDefID string) {
	t.Helper()
	_, err := sess.Backpack.Add(itemDefID, 1, reg)
	require.NoError(t, err)
}

func equipAffixWeapon(t testing.TB, sess *session.PlayerSession, reg *inventory.Registry, weaponDefID string) {
	t.Helper()
	wd := reg.Weapon(weaponDefID)
	if wd == nil {
		t.Fatalf("equipAffixWeapon: weapon %q not in registry", weaponDefID)
	}
	preset := inventory.NewWeaponPreset()
	require.NoError(t, preset.EquipMainHand(wd))
	preset.MainHand.Durability = 10 // test durability
	sess.LoadoutSet.Presets[0] = preset
}

func addMaterialToAffixBackpackRT(rt *rapid.T, sess *session.PlayerSession, reg *inventory.Registry, itemDefID string) {
	_, err := sess.Backpack.Add(itemDefID, 1, reg)
	if err != nil {
		rt.Fatalf("addMaterialToAffixBackpackRT: %v", err)
	}
}

func equipAffixWeaponRT(rt *rapid.T, sess *session.PlayerSession, reg *inventory.Registry, weaponDefID string) {
	wd := reg.Weapon(weaponDefID)
	if wd == nil {
		rt.Fatalf("equipAffixWeaponRT: weapon %q not in registry", weaponDefID)
	}
	preset := inventory.NewWeaponPreset()
	if err := preset.EquipMainHand(wd); err != nil {
		rt.Fatalf("equipAffixWeaponRT: EquipMainHand: %v", err)
	}
	preset.MainHand.Durability = 10 // test durability
	sess.LoadoutSet.Presets[0] = preset
}

func TestHandleAffix_InCombat(t *testing.T) {
	sess := &session.PlayerSession{Status: 2} // InCombat
	sess.Backpack = inventory.NewBackpack(20, 100.0)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	as := &command.AffixSession{Session: sess}
	reg := inventory.NewRegistry()
	result := command.HandleAffix(as, reg, "carbide_alloy_street_grade", "pistol", stubAffixRoller{})
	assert.Contains(t, result.Message, "cannot affix materials during combat")
	assert.Equal(t, command.AffixOutcomeUnspecified, result.Outcome)
}

func TestHandleAffix_MaterialNotInBackpack(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := inventory.NewRegistry()
	result := command.HandleAffix(as, reg, "carbide_alloy_street_grade", "pistol", stubAffixRoller{})
	assert.Contains(t, result.Message, "carbide_alloy_street_grade")
	assert.Contains(t, result.Message, "pack")
}

func TestHandleAffix_TargetNotEquipped(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "nonexistent_weapon", stubAffixRoller{})
	assert.Contains(t, result.Message, "not equipped")
}

func TestHandleAffix_BrokenTarget(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_street")
	sess.LoadoutSet.ActivePreset().MainHand.Durability = 0
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_street", stubAffixRoller{})
	assert.Contains(t, result.Message, "broken")
}

func TestHandleAffix_WrongAppliesTo(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "carbon_weave_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_street")
	result := command.HandleAffix(as, reg, "carbon_weave_street_grade", "test_pistol_street", stubAffixRoller{})
	assert.Contains(t, result.Message, "cannot be affixed to weapons")
}

func TestHandleAffix_DuplicateMaterial(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_street")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_street", stubAffixRoller{})
	assert.Contains(t, result.Message, "already has")
}

func TestHandleAffix_NoSlotsRemaining(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_street") // UpgradeSlots == 1
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"hollow_point:street_grade"}
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_street", stubAffixRoller{})
	assert.Contains(t, result.Message, "no upgrade slots remaining")
}

func TestHandleAffix_CriticalSuccess_ReturnsMaterial(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubAffixRoller{d20: 20})
	assert.Equal(t, command.AffixOutcomeCriticalSuccess, result.Outcome)
	assert.True(t, result.MaterialReturned)
	assert.Contains(t, result.Message, "returned intact")
	assert.Len(t, sess.Backpack.FindByItemDefID("scrap_iron_street_grade"), 1)
	assert.Contains(t, sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials, "scrap_iron:street_grade")
}

func TestHandleAffix_Success_ConsumesMaterial(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubAffixRoller{d20: 16})
	assert.Equal(t, command.AffixOutcomeSuccess, result.Outcome)
	assert.True(t, result.MaterialConsumed)
	assert.Len(t, sess.Backpack.FindByItemDefID("scrap_iron_street_grade"), 0)
}

func TestHandleAffix_CriticalFailure_DestroysMaterial(t *testing.T) {
	sess := makeAffixSession()
	as := &command.AffixSession{Session: sess}
	reg := buildAffixTestRegistry(t)
	addMaterialToAffixBackpack(t, sess, reg, "scrap_iron_street_grade")
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubAffixRoller{d20: 1})
	assert.Equal(t, command.AffixOutcomeCriticalFailure, result.Outcome)
	assert.Len(t, sess.Backpack.FindByItemDefID("scrap_iron_street_grade"), 0)
	assert.Contains(t, result.Message, "destroyed")
}

func TestHandleAffix_Property_OutcomeMatchesDCBounds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := makeAffixSession()
		as := &command.AffixSession{Session: sess}
		reg := buildAffixTestRegistryRT(rt)
		addMaterialToAffixBackpackRT(rt, sess, reg, "scrap_iron_street_grade")
		equipAffixWeaponRT(rt, sess, reg, "test_pistol_mil_spec")
		d20Roll := rapid.IntRange(1, 20).Draw(rt, "roll")
		result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubAffixRoller{d20: d20Roll})
		validOutcomes := map[command.AffixOutcome]bool{
			command.AffixOutcomeCriticalFailure: true,
			command.AffixOutcomeFailure:         true,
			command.AffixOutcomeSuccess:         true,
			command.AffixOutcomeCriticalSuccess: true,
		}
		assert.True(rt, validOutcomes[result.Outcome], "unexpected outcome %d", result.Outcome)
	})
}
