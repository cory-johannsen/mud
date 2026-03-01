package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestArmorDef_Validate_Valid(t *testing.T) {
	def := &inventory.ArmorDef{
		ID: "test_armor", Name: "Test Armor", Slot: inventory.SlotTorso,
		ACBonus: 2, DexCap: 3, CheckPenalty: -1, SpeedPenalty: 0,
		StrengthReq: 12, Bulk: 2, Group: "composite",
	}
	assert.NoError(t, def.Validate())
}

func TestArmorDef_Validate_MissingID(t *testing.T) {
	def := &inventory.ArmorDef{Name: "Test", Slot: inventory.SlotTorso, Group: "composite"}
	assert.ErrorContains(t, def.Validate(), "id")
}

func TestArmorDef_Validate_InvalidSlot(t *testing.T) {
	def := &inventory.ArmorDef{ID: "test", Name: "Test", Slot: inventory.ArmorSlot("bad"), Group: "composite"}
	assert.ErrorContains(t, def.Validate(), "slot")
}

func TestArmorDef_Validate_NegativeACBonus(t *testing.T) {
	def := &inventory.ArmorDef{ID: "test", Name: "Test", Slot: inventory.SlotTorso, Group: "composite", ACBonus: -1}
	assert.ErrorContains(t, def.Validate(), "ac_bonus")
}

func TestArmorDef_Validate_MissingName(t *testing.T) {
	def := &inventory.ArmorDef{ID: "test", Slot: inventory.SlotTorso, Group: "leather"}
	assert.ErrorContains(t, def.Validate(), "name")
}

func TestArmorDef_Validate_PositiveCheckPenalty(t *testing.T) {
	def := &inventory.ArmorDef{ID: "test", Name: "Test", Slot: inventory.SlotTorso, Group: "leather", CheckPenalty: 1}
	assert.ErrorContains(t, def.Validate(), "check_penalty")
}

func TestArmorDef_Validate_NegativeSpeedPenalty(t *testing.T) {
	def := &inventory.ArmorDef{ID: "test", Name: "Test", Slot: inventory.SlotTorso, Group: "leather", SpeedPenalty: -5}
	assert.ErrorContains(t, def.Validate(), "speed_penalty")
}

func TestArmorDef_Validate_MissingGroup(t *testing.T) {
	def := &inventory.ArmorDef{ID: "test", Name: "Test", Slot: inventory.SlotTorso}
	assert.ErrorContains(t, def.Validate(), "group")
}

func TestArmorDef_Validate_CrossTeamEffect_InvalidKind(t *testing.T) {
	def := &inventory.ArmorDef{
		ID: "test", Name: "Test", Slot: inventory.SlotTorso, Group: "leather",
		CrossTeamEffect: &inventory.CrossTeamEffect{Kind: "bad", Value: "clumsy-1"},
	}
	assert.ErrorContains(t, def.Validate(), "kind")
}

func TestArmorDef_Validate_CrossTeamEffect_EmptyValue(t *testing.T) {
	def := &inventory.ArmorDef{
		ID: "test", Name: "Test", Slot: inventory.SlotTorso, Group: "leather",
		CrossTeamEffect: &inventory.CrossTeamEffect{Kind: "condition", Value: ""},
	}
	assert.ErrorContains(t, def.Validate(), "value")
}

func TestLoadArmors_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: arm_guards
name: Arm Guards
slot: left_arm
ac_bonus: 1
dex_cap: 4
check_penalty: 0
speed_penalty: 0
strength_req: 10
bulk: 1
group: leather
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "arm_guards.yaml"), []byte(yaml), 0644))
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	require.Len(t, armors, 1)
	assert.Equal(t, "arm_guards", armors[0].ID)
	assert.Equal(t, inventory.SlotLeftArm, armors[0].Slot)
	assert.Equal(t, 1, armors[0].ACBonus)
	assert.Equal(t, 4, armors[0].DexCap)
}

func TestLoadArmors_TeamAffinityAndCrossEffect(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: tactical_vest
name: Tactical Vest
slot: torso
ac_bonus: 3
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 14
bulk: 2
group: composite
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tactical_vest.yaml"), []byte(yaml), 0644))
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	require.Len(t, armors, 1)
	assert.Equal(t, "gun", armors[0].TeamAffinity)
	require.NotNil(t, armors[0].CrossTeamEffect)
	assert.Equal(t, "condition", armors[0].CrossTeamEffect.Kind)
	assert.Equal(t, "clumsy-1", armors[0].CrossTeamEffect.Value)
}

func TestLoadArmors_EmptyDirReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	assert.Empty(t, armors)
}

func TestLoadArmors_InvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::invalid"), 0644))
	_, err := inventory.LoadArmors(dir)
	assert.ErrorContains(t, err, "cannot parse")
}

func TestProperty_ArmorSlot_AllConstantsAreValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slot := rapid.SampledFrom([]inventory.ArmorSlot{
			inventory.SlotHead, inventory.SlotTorso, inventory.SlotLeftArm, inventory.SlotRightArm,
			inventory.SlotHands, inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet,
		}).Draw(rt, "slot")
		def := &inventory.ArmorDef{ID: "test", Name: "Test", Slot: slot, Group: "leather"}
		assert.NoError(t, def.Validate())
	})
}
