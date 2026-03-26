package npc_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func loadNamedNPCTemplate(t *testing.T, filename string) *npc.Template {
	t.Helper()
	data, err := os.ReadFile("../../../content/npcs/" + filename)
	require.NoError(t, err, "content/npcs/%s must exist", filename)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err, "content/npcs/%s must parse and validate", filename)
	return tmpl
}

// yamlDomainFile mirrors the YAML top-level key used by the ai package loader.
type yamlDomainFile struct {
	Domain *ai.Domain `yaml:"domain"`
}

func loadAIDomainForNPC(t *testing.T, domainID string) *ai.Domain {
	t.Helper()
	data, err := os.ReadFile("../../../content/ai/" + domainID + ".yaml")
	require.NoError(t, err, "content/ai/%s.yaml must exist (REQ-NN-16)", domainID)
	var f yamlDomainFile
	err = yaml.Unmarshal(data, &f)
	require.NoError(t, err, "content/ai/%s.yaml must parse", domainID)
	require.NotNil(t, f.Domain, "content/ai/%s.yaml must have a top-level domain: key", domainID)
	return f.Domain
}

func assertDomainHasSayOperator(t *testing.T, domain *ai.Domain) {
	t.Helper()
	for _, op := range domain.Operators {
		if op.Action == "say" && len(op.Strings) > 0 {
			return
		}
	}
	t.Errorf("ai domain %q must have at least one operator with action: say and non-empty strings (REQ-NN-16)", domain.ID)
}

func TestNamedNPC_WayneDawg_LoadsAndValidates(t *testing.T) {
	tmpl := loadNamedNPCTemplate(t, "wayne_dawg.yaml")
	assert.Equal(t, "wayne_dawg", tmpl.ID)
	assert.Equal(t, "Wayne Dawg", tmpl.Name)
	assert.Equal(t, "human", tmpl.Type)
	assert.Equal(t, "friendly", tmpl.Disposition)
	assert.Equal(t, "0s", tmpl.RespawnDelay)
	assert.Empty(t, tmpl.Weapon, "weapon must be absent (REQ-NN-4)")
	assert.Empty(t, tmpl.Armor, "armor must be absent (REQ-NN-4)")
	require.NotNil(t, tmpl.Loot, "loot must be set (REQ-NN-6)")
	assert.Empty(t, tmpl.Loot.Items, "loot must be credits-only (REQ-NN-6)")
	assert.NotNil(t, tmpl.Loot.Currency, "loot.currency must be set (REQ-NN-6)")
	require.NotEmpty(t, tmpl.AIDomain, "ai_domain must be set (REQ-NN-16)")
	domain := loadAIDomainForNPC(t, tmpl.AIDomain)
	assertDomainHasSayOperator(t, domain)
}

func TestNamedNPC_JenniferDawg_LoadsAndValidates(t *testing.T) {
	tmpl := loadNamedNPCTemplate(t, "jennifer_dawg.yaml")
	assert.Equal(t, "jennifer_dawg", tmpl.ID)
	assert.Equal(t, "Jennifer Dawg", tmpl.Name)
	assert.Equal(t, "human", tmpl.Type)
	assert.Equal(t, "friendly", tmpl.Disposition)
	assert.Equal(t, "0s", tmpl.RespawnDelay)
	assert.Empty(t, tmpl.Weapon, "weapon must be absent (REQ-NN-4)")
	assert.Empty(t, tmpl.Armor, "armor must be absent (REQ-NN-4)")
	require.NotNil(t, tmpl.Loot, "loot must be set (REQ-NN-6)")
	assert.Empty(t, tmpl.Loot.Items, "loot must be credits-only (REQ-NN-6)")
	assert.NotNil(t, tmpl.Loot.Currency, "loot.currency must be set (REQ-NN-6)")
	require.NotEmpty(t, tmpl.AIDomain, "ai_domain must be set (REQ-NN-16)")
	domain := loadAIDomainForNPC(t, tmpl.AIDomain)
	assertDomainHasSayOperator(t, domain)
}

func TestNamedNPC_DwayneDawg_LoadsAndValidates(t *testing.T) {
	tmpl := loadNamedNPCTemplate(t, "dwayne_dawg.yaml")
	assert.Equal(t, "dwayne_dawg", tmpl.ID)
	assert.Equal(t, "Dwayne Dawg", tmpl.Name)
	assert.Equal(t, "human", tmpl.Type)
	assert.Equal(t, "friendly", tmpl.Disposition)
	assert.Equal(t, "0s", tmpl.RespawnDelay)
	assert.Empty(t, tmpl.Weapon, "weapon must be absent (REQ-NN-4)")
	assert.Empty(t, tmpl.Armor, "armor must be absent (REQ-NN-4)")
	require.NotNil(t, tmpl.Loot, "loot must be set (REQ-NN-6)")
	assert.Empty(t, tmpl.Loot.Items, "loot must be credits-only (REQ-NN-6)")
	assert.NotNil(t, tmpl.Loot.Currency, "loot.currency must be set (REQ-NN-6)")
	require.NotEmpty(t, tmpl.AIDomain, "ai_domain must be set (REQ-NN-16)")
	domain := loadAIDomainForNPC(t, tmpl.AIDomain)
	assertDomainHasSayOperator(t, domain)
}

func TestNamedNPC_AllThree_UniqueIDs(t *testing.T) {
	wayne := loadNamedNPCTemplate(t, "wayne_dawg.yaml")
	jennifer := loadNamedNPCTemplate(t, "jennifer_dawg.yaml")
	dwayne := loadNamedNPCTemplate(t, "dwayne_dawg.yaml")
	ids := map[string]bool{wayne.ID: true, jennifer.ID: true, dwayne.ID: true}
	assert.Len(t, ids, 3, "all three named NPCs must have unique IDs (REQ-NN-15)")
}

func TestProperty_NamedNPCs_NpcRoleIsMerchant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		filename := rapid.SampledFrom([]string{
			"wayne_dawg.yaml",
			"jennifer_dawg.yaml",
			"dwayne_dawg.yaml",
		}).Draw(rt, "filename")
		tmpl := loadNamedNPCTemplate(t, filename)
		assert.Equal(rt, "merchant", tmpl.NpcRole,
			"all named NPCs must have npc_role: merchant (REQ-NN-2)")
	})
}

func TestNamedNPC_AllThree_HaveQuestGiverComment(t *testing.T) {
	for _, filename := range []string{"wayne_dawg.yaml", "jennifer_dawg.yaml", "dwayne_dawg.yaml"} {
		data, err := os.ReadFile("../../../content/npcs/" + filename)
		require.NoError(t, err)
		assert.True(t, bytes.Contains(data, []byte("# quest_giver: pending quests feature")),
			"content/npcs/%s must contain '# quest_giver: pending quests feature' (REQ-NN-5)", filename)
	}
}
