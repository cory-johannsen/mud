# Non-Combat NPCs Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the shared infrastructure that all non-combat NPC types depend on: type-specific config structs, Template/Instance fields, `Validate()` enforcement, combat exclusion (REQ-NPC-1 through REQ-NPC-4), and flee/cower behavior when combat starts in a non-combat NPC's room.

**Architecture:** New config structs live in `internal/game/npc/noncombat.go`. Template gains `NPCType`, `Personality`, and eight config struct pointer fields; `Validate()` enforces the type/config contract (REQ-NPC-1, REQ-NPC-2). Instance gains `NPCType`, `Personality`, and `Cowering` fields. The combat handler rejects attacks on non-combat NPCs (REQ-NPC-4) and triggers flee/cower when combat starts in their room.

**Tech Stack:** Go, yaml.v3, testify/assert, rapid (property-based testing), existing `internal/game/npc`, `internal/gameserver`, `internal/game/world`, `internal/game/danger` packages.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/game/npc/noncombat.go` | **Create** | All type-specific config structs and runtime state stubs |
| `internal/game/npc/noncombat_test.go` | **Create** | Tests for config struct validation helpers |
| `internal/game/npc/template.go` | **Modify** | Add NPCType, Personality, 8 config struct pointer fields; extend Validate() |
| `internal/game/npc/template_test.go` | **Modify** | Add tests for new Validate() rules |
| `internal/game/npc/instance.go` | **Modify** | Add NPCType, Personality, Cowering fields; update NewInstanceWithResolver |
| `internal/game/npc/instance_test.go` | **Modify** | Add tests for NPCType/Personality/Cowering propagation (file already exists) |
| `internal/gameserver/combat_handler.go` | **Modify** | Block non-combat attack targets in Attack(); add flee/cower hook in startCombatLocked; clear Cowering on combat end |
| `internal/gameserver/combat_handler_noncombat_test.go` | **Create** | Tests for attack exclusion and flee/cower |
| `docs/features/non-combat-npcs.md` | **Modify** | Mark foundation requirements complete |

---

## Task 1: Non-Combat Config Struct Definitions

**Files:**
- Create: `internal/game/npc/noncombat.go`
- Create: `internal/game/npc/noncombat_test.go`

These are pure data structs. No business logic lives here — validation belongs in `Template.Validate()` (Task 2). Keep structs minimal; do not add methods beyond what the spec defines.

- [ ] **Step 1: Write failing tests**

Create `internal/game/npc/noncombat_test.go`:

```go
package npc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestReplenishConfig_Valid(t *testing.T) {
	c := ReplenishConfig{MinHours: 2, MaxHours: 6, StockRefill: 1, BudgetRefill: 100}
	assert.NoError(t, c.Validate(), "valid ReplenishConfig must not error")
}

func TestReplenishConfig_MinZero(t *testing.T) {
	c := ReplenishConfig{MinHours: 0, MaxHours: 6}
	assert.Error(t, c.Validate(), "MinHours == 0 must error")
}

func TestReplenishConfig_MinGtMax(t *testing.T) {
	c := ReplenishConfig{MinHours: 8, MaxHours: 4}
	assert.Error(t, c.Validate(), "MinHours > MaxHours must error")
}

func TestReplenishConfig_MaxGt24(t *testing.T) {
	c := ReplenishConfig{MinHours: 1, MaxHours: 25}
	assert.Error(t, c.Validate(), "MaxHours > 24 must error")
}

func TestQuestGiverConfig_EmptyDialog(t *testing.T) {
	c := QuestGiverConfig{PlaceholderDialog: nil}
	assert.Error(t, c.Validate(), "empty PlaceholderDialog must error")
}

func TestQuestGiverConfig_NonEmptyDialog(t *testing.T) {
	c := QuestGiverConfig{PlaceholderDialog: []string{"Hello, stranger."}}
	assert.NoError(t, c.Validate(), "non-empty PlaceholderDialog must not error")
}

// TestProperty_ReplenishConfig_ValidRangeNeverErrors verifies that any
// ReplenishConfig with 0 < MinHours <= MaxHours <= 24 passes Validate().
func TestProperty_ReplenishConfig_ValidRangeNeverErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(1, 24).Draw(rt, "min")
		max := rapid.IntRange(min, 24).Draw(rt, "max")
		c := ReplenishConfig{MinHours: min, MaxHours: max}
		if err := c.Validate(); err != nil {
			rt.Fatalf("valid ReplenishConfig{min:%d, max:%d} must not error: %v", min, max, err)
		}
	})
}

// TestProperty_ReplenishConfig_InvalidRangeAlwaysErrors verifies that any
// ReplenishConfig with MinHours <= 0 or MinHours > MaxHours always errors.
func TestProperty_ReplenishConfig_InvalidRangeAlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(-10, 0).Draw(rt, "min_le_zero")
		max := rapid.IntRange(1, 24).Draw(rt, "max")
		c := ReplenishConfig{MinHours: min, MaxHours: max}
		if err := c.Validate(); err == nil {
			rt.Fatalf("invalid ReplenishConfig{min:%d, max:%d} must error", min, max)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestReplenishConfig|TestQuestGiverConfig|TestProperty_Replenish" -v 2>&1 | head -20
```

Expected: compilation error (types not defined yet).

- [ ] **Step 3: Create `internal/game/npc/noncombat.go`**

```go
// Package npc — non-combat NPC type-specific config structs and runtime state.
package npc

import (
	"fmt"
	"time"
)

// ---- Merchant ----

// MerchantConfig holds the static configuration for a merchant NPC.
type MerchantConfig struct {
	MerchantType  string          `yaml:"merchant_type"` // weapons|armor|rings_neck|consumables|maps|technology|drugs
	Inventory     []MerchantItem  `yaml:"inventory"`
	SellMargin    float64         `yaml:"sell_margin"`   // markup multiplier on base price for player purchases
	BuyMargin     float64         `yaml:"buy_margin"`    // fraction of base price paid to player on sale
	Budget        int             `yaml:"budget"`        // max credits available to buy from players
	ReplenishRate ReplenishConfig `yaml:"replenish_rate"`
}

// MerchantItem is one entry in a merchant's static inventory.
type MerchantItem struct {
	ItemID    string `yaml:"item_id"`
	BasePrice int    `yaml:"base_price"`
	InitStock int    `yaml:"init_stock"` // quantity loaded at first zone init
	MaxStock  int    `yaml:"max_stock"`  // cap for replenishment
}

// ReplenishConfig controls how often a merchant's stock and budget reset.
// REQ-NPC-13: 0 < MinHours <= MaxHours <= 24.
type ReplenishConfig struct {
	MinHours     int `yaml:"min_hours"`
	MaxHours     int `yaml:"max_hours"`
	StockRefill  int `yaml:"stock_refill"`  // units added per item per cycle; 0 = full reset to MaxStock
	BudgetRefill int `yaml:"budget_refill"` // credits added per cycle; 0 = full reset
}

// Validate checks REQ-NPC-13: 0 < MinHours <= MaxHours <= 24.
func (r ReplenishConfig) Validate() error {
	if r.MinHours <= 0 {
		return fmt.Errorf("replenish_rate: min_hours must be > 0, got %d", r.MinHours)
	}
	if r.MaxHours < r.MinHours {
		return fmt.Errorf("replenish_rate: max_hours (%d) must be >= min_hours (%d)", r.MaxHours, r.MinHours)
	}
	if r.MaxHours > 24 {
		return fmt.Errorf("replenish_rate: max_hours must be <= 24, got %d", r.MaxHours)
	}
	return nil
}

// MerchantRuntimeState holds the mutable runtime state of a merchant, persisted to DB.
type MerchantRuntimeState struct {
	Stock           map[string]int // itemID → current quantity
	CurrentBudget   int
	NextReplenishAt time.Time // zero = not yet scheduled
}

// ---- Guard ----

// GuardConfig holds the static configuration for a guard NPC.
type GuardConfig struct {
	// WantedThreshold is the minimum WantedLevel that triggers engagement. Default: 2.
	WantedThreshold int    `yaml:"wanted_threshold"`
	// PatrolRoom is the room ID this guard patrols; empty = stationary.
	PatrolRoom      string `yaml:"patrol_room,omitempty"`
}

// ---- Healer ----

// HealerConfig holds the static configuration for a healer NPC.
type HealerConfig struct {
	PricePerHP    int `yaml:"price_per_hp"`   // credits per HP restored
	DailyCapacity int `yaml:"daily_capacity"` // max total HP restorable per in-game day
}

// HealerRuntimeState holds the mutable runtime state of a healer, persisted to DB.
type HealerRuntimeState struct {
	CapacityUsed int // reset to 0 on daily calendar tick (REQ-NPC-16)
}

// ---- Quest Giver ----

// QuestGiverConfig holds the static configuration for a quest giver NPC.
// REQ-NPC-18: PlaceholderDialog must contain at least one entry.
type QuestGiverConfig struct {
	PlaceholderDialog []string `yaml:"placeholder_dialog"`
	QuestIDs          []string `yaml:"quest_ids,omitempty"` // populated when Quest system lands
}

// Validate checks REQ-NPC-18: PlaceholderDialog must not be empty.
func (q QuestGiverConfig) Validate() error {
	if len(q.PlaceholderDialog) == 0 {
		return fmt.Errorf("quest_giver: placeholder_dialog must contain at least one entry")
	}
	return nil
}

// ---- Hireling ----

// HirelingConfig holds the static configuration for a hireling NPC.
type HirelingConfig struct {
	DailyCost      int    `yaml:"daily_cost"`       // credits per in-game day while hired
	CombatRole     string `yaml:"combat_role"`       // "melee" | "ranged" | "support"
	MaxFollowZones int    `yaml:"max_follow_zones"`  // 0 = unlimited
}

// HirelingRuntimeState holds the mutable runtime state of a hireling, persisted to DB.
type HirelingRuntimeState struct {
	HiredByPlayerID string // empty if not currently hired
	ZonesFollowed   int    // count of zone transitions since hire
}

// ---- Banker ----

// BankerConfig holds the static configuration for a banker NPC.
type BankerConfig struct {
	ZoneID       string  `yaml:"zone_id"`
	BaseRate     float64 `yaml:"base_rate"`     // baseline exchange rate; 1.0 = no fee
	RateVariance float64 `yaml:"rate_variance"` // max daily variance; e.g. 0.05 = ±5%
}

// ---- Job Trainer ----

// JobTrainerConfig holds the static configuration for a job trainer NPC.
type JobTrainerConfig struct {
	OfferedJobs []TrainableJob `yaml:"offered_jobs"`
}

// TrainableJob describes a job that this trainer can teach.
type TrainableJob struct {
	JobID         string           `yaml:"job_id"`
	TrainingCost  int              `yaml:"training_cost"`
	Prerequisites JobPrerequisites `yaml:"prerequisites"`
}

// JobPrerequisites enumerates all gatekeeping conditions for training a job.
type JobPrerequisites struct {
	MinLevel      int               `yaml:"min_level,omitempty"`
	MinJobLevel   map[string]int    `yaml:"min_job_level,omitempty"`
	MinAttributes map[string]int    `yaml:"min_attributes,omitempty"`
	MinSkillRanks map[string]string `yaml:"min_skill_ranks,omitempty"`
	RequiredJobs  []string          `yaml:"required_jobs,omitempty"`
}

// ---- Crafter (stub) ----

// CrafterConfig is intentionally empty until the crafting feature spec is written.
// A YAML block `crafter: {}` MUST be present for npc_type: crafter (REQ-NPC-2).
type CrafterConfig struct{}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestReplenishConfig|TestQuestGiverConfig|TestProperty_Replenish" -v 2>&1
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/noncombat.go internal/game/npc/noncombat_test.go
git commit -m "feat: add non-combat NPC type-specific config structs"
```

---

## Task 2: Template NPCType/Personality Fields + Validate()

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/template_test.go`

Add `NPCType string`, `Personality string`, and the eight config struct pointer fields to `Template`. Update `Validate()` to enforce REQ-NPC-1 (default to "combat"), REQ-NPC-2 (config sub-struct must be present), REQ-NPC-13 (ReplenishConfig), and REQ-NPC-18 (QuestGiver dialog). Do NOT add REQ-NPC-2a (skill ID validation) — that requires the skills registry and belongs in a later sub-project.

Valid `NPCType` values: `"combat"`, `"merchant"`, `"guard"`, `"healer"`, `"quest_giver"`, `"hireling"`, `"banker"`, `"job_trainer"`, `"crafter"`.

Valid `Personality` values: `"cowardly"`, `"brave"`, `"neutral"`, `"opportunistic"` (or empty — defaults to type behavior).

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/template_test.go`. Do not remove any existing tests — append only.

```go
func TestTemplate_DefaultNPCType(t *testing.T) {
	data := []byte(`id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 12
`)
	tmpl, err := LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "combat", tmpl.NPCType, "missing npc_type must default to 'combat'")
}

func TestTemplate_MerchantRequiresMerchantConfig(t *testing.T) {
	data := []byte(`id: test_merchant
name: Test Merchant
level: 1
max_hp: 10
ac: 12
npc_type: merchant
`)
	_, err := LoadTemplateFromBytes(data)
	assert.Error(t, err, "merchant npc_type without merchant config must error")
}

func TestTemplate_MerchantWithConfigLoads(t *testing.T) {
	data := []byte(`id: test_merchant
name: Test Merchant
level: 1
max_hp: 10
ac: 12
npc_type: merchant
merchant:
  merchant_type: consumables
  sell_margin: 1.2
  buy_margin: 0.6
  budget: 500
  replenish_rate:
    min_hours: 4
    max_hours: 8
    stock_refill: 1
    budget_refill: 200
`)
	tmpl, err := LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "merchant", tmpl.NPCType)
	require.NotNil(t, tmpl.Merchant)
	assert.Equal(t, "consumables", tmpl.Merchant.MerchantType)
}

func TestTemplate_QuestGiverEmptyDialogErrors(t *testing.T) {
	data := []byte(`id: test_qg
name: Test Quest Giver
level: 1
max_hp: 10
ac: 12
npc_type: quest_giver
quest_giver:
  placeholder_dialog: []
`)
	_, err := LoadTemplateFromBytes(data)
	assert.Error(t, err, "quest_giver with empty placeholder_dialog must error")
}

func TestTemplate_CrafterRequiresExplicitConfig(t *testing.T) {
	data := []byte(`id: test_crafter
name: Test Crafter
level: 1
max_hp: 10
ac: 12
npc_type: crafter
`)
	_, err := LoadTemplateFromBytes(data)
	assert.Error(t, err, "crafter npc_type without explicit crafter: {} must error")
}

func TestTemplate_CrafterWithEmptyBlockLoads(t *testing.T) {
	data := []byte(`id: test_crafter
name: Test Crafter
level: 1
max_hp: 10
ac: 12
npc_type: crafter
crafter: {}
`)
	tmpl, err := LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "crafter", tmpl.NPCType)
	require.NotNil(t, tmpl.Crafter)
}

func TestTemplate_UnknownNPCTypeErrors(t *testing.T) {
	data := []byte(`id: test_bad
name: Bad NPC
level: 1
max_hp: 10
ac: 12
npc_type: wizard
`)
	_, err := LoadTemplateFromBytes(data)
	assert.Error(t, err, "unknown npc_type must error")
}

func TestTemplate_PersonalityPreserved(t *testing.T) {
	data := []byte(`id: test_guard
name: Test Guard
level: 2
max_hp: 20
ac: 14
npc_type: guard
personality: brave
guard:
  wanted_threshold: 2
`)
	tmpl, err := LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "brave", tmpl.Personality, "personality must be preserved from YAML")
}

// TestProperty_AllExistingNPCTemplatesStillLoad verifies that adding NPCType/Validate changes
// does not break any existing NPC YAML file. Reads all *.yaml in content/npcs/.
func TestProperty_AllExistingNPCTemplatesStillLoad(t *testing.T) {
	templates, err := LoadTemplates("../../../content/npcs")
	require.NoError(t, err, "all existing NPC templates must still load after Validate() changes")
	assert.NotEmpty(t, templates, "expected at least one template in content/npcs/")
	for _, tmpl := range templates {
		assert.Equal(t, "combat", tmpl.NPCType,
			"existing NPC %q must default to npc_type 'combat'", tmpl.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestTemplate_Default|TestTemplate_Merchant|TestTemplate_QuestGiver|TestTemplate_Crafter|TestTemplate_Unknown|TestTemplate_Personality|TestProperty_AllExisting" -v 2>&1 | head -30
```

Expected: FAIL (fields don't exist yet).

- [ ] **Step 3: Add fields to Template struct in `template.go`**

After the existing `Disposition string` field (line 94), add:

```go
// NPCType classifies the NPC's role.
// Valid values: "combat", "merchant", "guard", "healer", "quest_giver",
// "hireling", "banker", "job_trainer", "crafter".
// Defaults to "combat" at load time if absent (REQ-NPC-1).
NPCType string `yaml:"npc_type"`

// Personality names the HTN preset governing non-combat flee/cower behavior.
// Valid values: "cowardly" (always flee), "brave" (always cower),
// "neutral" (use type default), "opportunistic" (use type default).
// Empty string also falls through to the type default.
Personality string `yaml:"personality"`

// Type-specific config — at most one is non-nil for a given NPC.
Merchant   *MerchantConfig   `yaml:"merchant,omitempty"`
Guard      *GuardConfig      `yaml:"guard,omitempty"`
Healer     *HealerConfig     `yaml:"healer,omitempty"`
QuestGiver *QuestGiverConfig `yaml:"quest_giver,omitempty"`
Hireling   *HirelingConfig   `yaml:"hireling,omitempty"`
Banker     *BankerConfig     `yaml:"banker,omitempty"`
JobTrainer *JobTrainerConfig `yaml:"job_trainer,omitempty"`
Crafter    *CrafterConfig    `yaml:"crafter,omitempty"`
```

- [ ] **Step 4: Update `Validate()` in `template.go`**

Replace the existing final `return nil` in `Validate()` with:

```go
// REQ-NPC-1: default NPCType to "combat".
if t.NPCType == "" {
    t.NPCType = "combat"
}

// Validate NPCType value and corresponding config struct (REQ-NPC-2).
validTypes := map[string]bool{
    "combat": true, "merchant": true, "guard": true, "healer": true,
    "quest_giver": true, "hireling": true, "banker": true,
    "job_trainer": true, "crafter": true,
}
if !validTypes[t.NPCType] {
    return fmt.Errorf("npc template %q: unknown npc_type %q", t.ID, t.NPCType)
}

switch t.NPCType {
case "combat":
    // no config struct required
case "merchant":
    if t.Merchant == nil {
        return fmt.Errorf("npc template %q: npc_type 'merchant' requires a merchant: config block", t.ID)
    }
    if err := t.Merchant.ReplenishRate.Validate(); err != nil {
        return fmt.Errorf("npc template %q: %w", t.ID, err)
    }
case "guard":
    if t.Guard == nil {
        return fmt.Errorf("npc template %q: npc_type 'guard' requires a guard: config block", t.ID)
    }
case "healer":
    if t.Healer == nil {
        return fmt.Errorf("npc template %q: npc_type 'healer' requires a healer: config block", t.ID)
    }
case "quest_giver":
    if t.QuestGiver == nil {
        return fmt.Errorf("npc template %q: npc_type 'quest_giver' requires a quest_giver: config block", t.ID)
    }
    if err := t.QuestGiver.Validate(); err != nil {
        return fmt.Errorf("npc template %q: %w", t.ID, err)
    }
case "hireling":
    if t.Hireling == nil {
        return fmt.Errorf("npc template %q: npc_type 'hireling' requires a hireling: config block", t.ID)
    }
case "banker":
    if t.Banker == nil {
        return fmt.Errorf("npc template %q: npc_type 'banker' requires a banker: config block", t.ID)
    }
case "job_trainer":
    if t.JobTrainer == nil {
        return fmt.Errorf("npc template %q: npc_type 'job_trainer' requires a job_trainer: config block", t.ID)
    }
case "crafter":
    if t.Crafter == nil {
        return fmt.Errorf("npc template %q: npc_type 'crafter' requires an explicit crafter: {} config block", t.ID)
    }
}
return nil
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -v 2>&1 | tail -20
```

Expected: all tests PASS including the new ones.

- [ ] **Step 6: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/template_test.go
git commit -m "feat: add NPCType/Personality/config fields to Template; extend Validate() with REQ-NPC-1/2"
```

---

## Task 3: Instance NPCType, Personality, and Cowering Fields

**Files:**
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/instance_test.go` (file already exists — append tests, do not delete existing ones)

Add `NPCType string`, `Personality string`, and `Cowering bool` to `Instance`. Update `NewInstanceWithResolver` to copy all three from the template. `Cowering` initializes to false.

- [ ] **Step 1: Write failing tests**

Append to the existing `internal/game/npc/instance_test.go` (after the last existing test):

```go
func TestInstance_NPCTypeFromTemplate(t *testing.T) {
	tmpl := &Template{
		ID: "test_npc", Name: "Test NPC", Level: 1, MaxHP: 10, AC: 12,
		NPCType: "merchant",
		Merchant: &MerchantConfig{ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 4}},
	}
	inst := NewInstance("inst-1", tmpl, "room-1")
	assert.Equal(t, "merchant", inst.NPCType, "NPCType must be copied from template")
}

func TestInstance_PersonalityFromTemplate(t *testing.T) {
	tmpl := &Template{
		ID: "test_guard", Name: "Guard", Level: 2, MaxHP: 20, AC: 14,
		NPCType: "guard", Personality: "brave",
		Guard: &GuardConfig{WantedThreshold: 2},
	}
	inst := NewInstance("inst-2", tmpl, "room-1")
	assert.Equal(t, "brave", inst.Personality, "Personality must be copied from template")
}

func TestInstance_CombatNPCType(t *testing.T) {
	tmpl := &Template{ID: "bandit", Name: "Bandit", Level: 1, MaxHP: 20, AC: 12, NPCType: "combat"}
	inst := NewInstance("inst-3", tmpl, "room-1")
	assert.Equal(t, "combat", inst.NPCType, "combat NPCType must propagate")
}

func TestInstance_CoweringDefaultsFalse(t *testing.T) {
	tmpl := &Template{ID: "test_npc", Name: "NPC", Level: 1, MaxHP: 10, AC: 12, NPCType: "combat"}
	inst := NewInstance("inst-4", tmpl, "room-1")
	assert.False(t, inst.Cowering, "Cowering must default to false at spawn")
}

func TestManager_SpawnPropagatesNPCType(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{
		ID: "healer_npc", Name: "Healer", Level: 1, MaxHP: 10, AC: 10,
		NPCType: "healer",
		Healer:  &HealerConfig{PricePerHP: 5, DailyCapacity: 200},
	}
	inst, err := mgr.Spawn(tmpl, "room-heal")
	require.NoError(t, err)
	assert.Equal(t, "healer", inst.NPCType, "Manager.Spawn must propagate NPCType")
}

// TestProperty_Instance_NPCTypeAlwaysPropagates checks that spawning any NPC
// template always produces an instance with the same NPCType as the template.
func TestProperty_Instance_NPCTypeAlwaysPropagates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		npcType := rapid.SampledFrom([]string{
			"combat", "merchant", "guard", "healer",
			"quest_giver", "hireling", "banker", "job_trainer", "crafter",
		}).Draw(rt, "npc_type")
		tmpl := &Template{
			ID:      "prop_npc",
			Name:    "Prop NPC",
			Level:   1,
			MaxHP:   10,
			AC:      12,
			NPCType: npcType,
		}
		inst := NewInstance("prop-inst", tmpl, "prop-room")
		if inst.NPCType != npcType {
			rt.Fatalf("expected NPCType %q, got %q", npcType, inst.NPCType)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestInstance_NPCType|TestInstance_Personality|TestInstance_Cowering|TestManager_SpawnPropagates|TestProperty_Instance" -v 2>&1 | head -20
```

Expected: FAIL (fields don't exist on Instance yet).

- [ ] **Step 3: Add fields to Instance struct in `instance.go`**

After the existing `MotiveBonus int` field, add:

```go
// NPCType is copied from the template at spawn.
// "combat" = participates in normal combat; other values = non-combat NPC.
NPCType string
// Personality is copied from the template at spawn; drives flee/cower behavior.
// See combat_handler.go defaultCombatResponse for how it's interpreted.
Personality string
// Cowering is true when this NPC is in a cower state because combat started
// in their room. While Cowering == true, the NPC does not respond to commands.
// Cleared when combat in their room ends.
Cowering bool
```

- [ ] **Step 4: Update `NewInstanceWithResolver` in `instance.go`**

In the large struct literal returned by `NewInstanceWithResolver`, add these three lines after `MotiveBonus: 0,`:

```go
NPCType:    tmpl.NPCType,
Personality: tmpl.Personality,
// Cowering defaults to false (zero value).
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -v 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/game/npc/instance.go internal/game/npc/instance_test.go
git commit -m "feat: add NPCType, Personality, Cowering fields to NPC Instance"
```

---

## Task 4: Combat Exclusion (REQ-NPC-4)

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_noncombat_test.go`

Block non-combat NPCs from being targeted by `Attack()`. Guards and hirelings are excluded from this block — guards enter combat via their own behavior logic (sub-project 4); hirelings join as combat participants.

The check goes in `Attack()` immediately after the existing `inst.IsDead()` check, before `h.combatMu.Lock()`.

- [ ] **Step 1: Write failing tests**

Before writing, read one existing test file that creates a `CombatHandler` directly, e.g., `internal/gameserver/grpc_service_swim_test.go` lines 120–160. Match its exact factory function pattern (`NewCombatHandler(...)`, `sessMgr.AddPlayer(...)`, etc.).

Create `internal/gameserver/combat_handler_noncombat_test.go` following this pattern:

```go
package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newNonCombatTestWorld builds a minimal zone with one room containing
// one combat NPC and one merchant NPC.
func newNonCombatTestWorld(t *testing.T) (*world.Manager, *session.Manager, *npc.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:        "nc_zone",
		Name:      "NonCombatZone",
		StartRoom: "room1",
		Rooms: map[string]*world.Room{
			"room1": {
				ID: "room1", ZoneID: "nc_zone", Title: "Test Room",
				DangerLevel: "risky",
				Properties:  map[string]string{},
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room2"}},
			},
			"room2": {
				ID: "room2", ZoneID: "nc_zone", Title: "Next Room",
				DangerLevel: "risky",
				Properties:  map[string]string{},
				Exits:       []world.Exit{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sm := session.NewManager()
	nm := npc.NewManager()

	// Spawn a combat NPC.
	combatTmpl := &npc.Template{
		ID: "bandit", Name: "Bandit", Level: 1, MaxHP: 20, AC: 12, NPCType: "combat",
	}
	_, err = nm.Spawn(combatTmpl, "room1")
	require.NoError(t, err)

	// Spawn a merchant (non-combat NPC).
	merchantTmpl := &npc.Template{
		ID:      "shopkeep",
		Name:    "Shopkeeper",
		Level:   1,
		MaxHP:   10,
		AC:      10,
		NPCType: "merchant",
		Merchant: &npc.MerchantConfig{
			MerchantType:  "consumables",
			ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4},
		},
	}
	_, err = nm.Spawn(merchantTmpl, "room1")
	require.NoError(t, err)

	return wm, sm, nm
}

// newNonCombatCombatHandler builds a CombatHandler for non-combat NPC tests.
func newNonCombatCombatHandler(t *testing.T, wm *world.Manager, sm *session.Manager, nm *npc.Manager) *CombatHandler {
	t.Helper()
	condReg := makeTestConditionRegistry()
	roller := dice.NewRoller()
	return NewCombatHandler(
		combat.NewEngine(), nm, sm, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, wm, nil, nil, nil, nil, nil, nil,
	)
}

func TestAttack_BlocksNonCombatNPC(t *testing.T) {
	wm, sm, nm := newNonCombatTestWorld(t)
	h := newNonCombatCombatHandler(t, wm, sm, nm)

	_, err := sm.AddPlayer(session.AddPlayerOptions{
		UID: "player1", Username: "Alice", CharName: "Alice",
		Role: "player", RoomID: "room1", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)

	_, err = h.Attack("player1", "Shopkeeper")
	assert.Error(t, err, "attacking a non-combat NPC must return an error")
	assert.Contains(t, err.Error(), "not a valid combat target",
		"error must indicate the NPC is not a valid combat target")
}

func TestAttack_AllowsCombatNPC(t *testing.T) {
	wm, sm, nm := newNonCombatTestWorld(t)
	h := newNonCombatCombatHandler(t, wm, sm, nm)

	_, err := sm.AddPlayer(session.AddPlayerOptions{
		UID: "player1", Username: "Alice", CharName: "Alice",
		Role: "player", RoomID: "room1", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)

	events, err := h.Attack("player1", "Bandit")
	assert.NoError(t, err, "attacking a combat NPC must succeed")
	assert.NotEmpty(t, events, "attack must return combat events")
}
```

**Note on `NewCombatHandler` signature:** Check `internal/gameserver/combat_handler.go` for the exact signature of `NewCombatHandler` before writing. Match the exact parameter list used in other test files. The `nil` parameters above are placeholders for optional managers — use the same nil-passing pattern as `newSwimSvcWithCombat` in the swim test file.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestAttack_Blocks|TestAttack_Allows" -v 2>&1 | head -20
```

Expected: FAIL (check doesn't exist yet in Attack()).

- [ ] **Step 3: Add NPCType check to `Attack()` in `combat_handler.go`**

Find the `Attack()` method. After the `inst.IsDead()` check and before the `h.combatMu.Lock()` call, add:

```go
// REQ-NPC-4: non-combat NPCs cannot be attacked directly.
// Guards enter combat via their own engage behavior (sub-project 4).
// Hirelings are combat participants (sub-project 4).
if inst.NPCType != "" && inst.NPCType != "combat" && inst.NPCType != "guard" && inst.NPCType != "hireling" {
    return nil, fmt.Errorf("%s is not a valid combat target", inst.Name())
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestAttack_Blocks|TestAttack_Allows" -v 2>&1
go test ./... 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_noncombat_test.go
git commit -m "feat: block non-combat NPCs as attack targets (REQ-NPC-4)"
```

---

## Task 5: Flee/Cower on Combat Start

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/combat_handler_noncombat_test.go`

When combat starts in a room, non-combat NPCs in that room must either flee to an adjacent room or cower in place. This logic runs inside `startCombatLocked` after combat is initialized.

### Flee/cower routing

Add `defaultCombatResponse` to `combat_handler.go`:

```go
// defaultCombatResponse returns "flee", "cower", or "engage" for a non-combat NPC.
//
// Personality "cowardly" always maps to "flee"; "brave" always maps to "cower".
// "neutral", "opportunistic", and empty all fall through to the type-specific default.
//
// Type defaults:
//   merchant, quest_giver, job_trainer → "cower"
//   healer, banker, crafter           → "flee"
//   guard                             → "engage" (enters combat; wired in sub-project 4)
//   hireling                          → "engage" (combat participant; wired in sub-project 4)
func defaultCombatResponse(npcType, personality string) string {
	switch personality {
	case "cowardly":
		return "flee"
	case "brave":
		return "cower"
	// "neutral", "opportunistic", and "" all fall through to type default.
	}
	switch npcType {
	case "merchant", "quest_giver", "job_trainer":
		return "cower"
	case "healer", "banker", "crafter":
		return "flee"
	case "guard":
		return "engage"
	case "hireling":
		return "engage"
	default:
		return "cower"
	}
}
```

### applyCombatStartBehaviorsLocked

Add to `CombatHandler`:

```go
// applyCombatStartBehaviorsLocked fires flee/cower for all non-combat NPCs in roomID
// when combat starts. Must be called with h.combatMu already held.
//
// Guards and hirelings are skipped (they have their own combat-start behavior).
// Precondition: roomID is non-empty; h.worldMgr is non-nil.
func (h *CombatHandler) applyCombatStartBehaviorsLocked(roomID string) {
	room, ok := h.worldMgr.GetRoom(roomID)
	if !ok {
		return
	}
	for _, inst := range h.npcMgr.InstancesInRoom(roomID) {
		if inst.NPCType == "" || inst.NPCType == "combat" || inst.NPCType == "guard" || inst.NPCType == "hireling" {
			continue
		}
		switch defaultCombatResponse(inst.NPCType, inst.Personality) {
		case "flee":
			h.fleeNPCLocked(inst, room)
		default: // "cower"
			inst.Cowering = true
		}
	}
}

// fleeNPCLocked moves inst to a random valid adjacent room.
// A valid exit is non-hidden, non-locked, and does not lead to an All Out War room.
// Falls back to cower if no valid exits exist.
// Must be called with h.combatMu held.
func (h *CombatHandler) fleeNPCLocked(inst *npc.Instance, room *world.Room) {
	var validExits []world.Exit
	for _, exit := range room.Exits {
		if exit.Hidden || exit.Locked {
			continue
		}
		// REQ: do not flee into an All Out War room.
		if dest, ok := h.worldMgr.GetRoom(exit.TargetRoom); ok {
			dl := danger.EffectiveDangerLevel(dest.ZoneID, dest.DangerLevel)
			if dl == danger.AllOutWar {
				continue
			}
		}
		validExits = append(validExits, exit)
	}
	if len(validExits) == 0 {
		inst.Cowering = true
		return
	}
	target := validExits[rand.Intn(len(validExits))]
	_ = h.npcMgr.Move(inst.ID, target.TargetRoom)
}
```

**Note on `danger.EffectiveDangerLevel` call:** The Room struct has `DangerLevel string` (not a `danger.DangerLevel`). The zone danger level needs to come from the zone, not the room directly. Check how `EffectiveDangerLevel` is called elsewhere in the codebase (e.g., `enforcement.go`) to pass the correct zone danger level. If the CombatHandler doesn't have access to the zone danger level, you can use `dest.DangerLevel` directly and compare `dest.DangerLevel == string(danger.AllOutWar)` instead.

### clearCoweringNPCsLocked

Add to `CombatHandler`:

```go
// clearCoweringNPCsLocked resets the Cowering flag for all NPCs in roomID.
// Must be called when combat in roomID ends.
func (h *CombatHandler) clearCoweringNPCsLocked(roomID string) {
	for _, inst := range h.npcMgr.InstancesInRoom(roomID) {
		if inst.Cowering {
			inst.Cowering = false
		}
	}
}
```

### Wire into startCombatLocked

1. At the end of `startCombatLocked`, after `h.startTimerLocked(sess.RoomID)`, add:
   ```go
   h.applyCombatStartBehaviorsLocked(sess.RoomID)
   ```

2. Find the combat-end path in `combat_handler.go` (search for `h.engine.RemoveCombat` or `endCombat` or where the "you won" event is broadcast). After the combat is removed/ended, add:
   ```go
   h.clearCoweringNPCsLocked(roomID)
   ```

### Required imports

Add to `combat_handler.go` imports if not already present:
- `"math/rand"`
- `"github.com/cory-johannsen/mud/internal/game/danger"`

- [ ] **Step 1: Write failing tests**

Append to `combat_handler_noncombat_test.go`:

```go
func TestCombatStart_MerchantCowers(t *testing.T) {
	wm, sm, nm := newNonCombatTestWorld(t)
	h := newNonCombatCombatHandler(t, wm, sm, nm)

	_, err := sm.AddPlayer(session.AddPlayerOptions{
		UID: "player1", Username: "Alice", CharName: "Alice",
		Role: "player", RoomID: "room1", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)

	// Start combat with the combat NPC.
	_, err = h.Attack("player1", "Bandit")
	require.NoError(t, err, "attack must succeed to start combat")

	// Merchant must be cowering (not fled — room1 has exits but merchant default is cower).
	var merchant *npc.Instance
	for _, n := range nm.InstancesInRoom("room1") {
		if n.NPCType == "merchant" {
			merchant = n
		}
	}
	require.NotNil(t, merchant, "merchant must still be in room1 (cower behavior)")
	assert.True(t, merchant.Cowering, "merchant must be cowering after combat starts")
}

func TestDefaultCombatResponse_TypeDefaults(t *testing.T) {
	cases := []struct {
		npcType  string
		expected string
	}{
		{"merchant", "cower"},
		{"quest_giver", "cower"},
		{"job_trainer", "cower"},
		{"healer", "flee"},
		{"banker", "flee"},
		{"crafter", "flee"},
		{"guard", "engage"},
		{"hireling", "engage"},
	}
	for _, tc := range cases {
		t.Run(tc.npcType, func(t *testing.T) {
			got := defaultCombatResponse(tc.npcType, "")
			assert.Equal(t, tc.expected, got,
				"npc_type %q with empty personality must default to %q", tc.npcType, tc.expected)
		})
	}
}

func TestDefaultCombatResponse_PersonalityOverrides(t *testing.T) {
	assert.Equal(t, "flee", defaultCombatResponse("merchant", "cowardly"),
		"cowardly personality always flees regardless of type default")
	assert.Equal(t, "cower", defaultCombatResponse("healer", "brave"),
		"brave personality always cowers regardless of type default")
	assert.Equal(t, "flee", defaultCombatResponse("healer", "neutral"),
		"neutral personality falls through to healer type default (flee)")
	assert.Equal(t, "cower", defaultCombatResponse("merchant", "opportunistic"),
		"opportunistic personality falls through to merchant type default (cower)")
}

// TestProperty_DefaultCombatResponse_NeverPanics verifies that defaultCombatResponse
// never panics for arbitrary npcType and personality strings.
func TestProperty_DefaultCombatResponse_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		npcType := rapid.String().Draw(rt, "npc_type")
		personality := rapid.String().Draw(rt, "personality")
		result := defaultCombatResponse(npcType, personality)
		validResponses := map[string]bool{"flee": true, "cower": true, "engage": true}
		if !validResponses[result] {
			rt.Fatalf("defaultCombatResponse(%q, %q) returned unexpected %q", npcType, personality, result)
		}
	})
}
```

Add `"pgregory.net/rapid"` to imports in `combat_handler_noncombat_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestCombatStart_|TestDefaultCombatResponse_|TestProperty_DefaultCombatResponse" -v 2>&1 | head -20
```

Expected: FAIL (functions don't exist yet).

- [ ] **Step 3: Implement in `combat_handler.go`**

Add `defaultCombatResponse`, `applyCombatStartBehaviorsLocked`, `fleeNPCLocked`, `clearCoweringNPCsLocked` as defined above. Wire into `startCombatLocked` and combat-end path as described.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestCombatStart_|TestDefaultCombatResponse_|TestProperty_DefaultCombatResponse" -v 2>&1
go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/combat_handler.go \
        internal/gameserver/combat_handler_noncombat_test.go \
        internal/game/npc/instance.go
git commit -m "feat: flee/cower behavior for non-combat NPCs on combat start"
```

---

## Task 6: Documentation Update

**Files:**
- Modify: `docs/features/non-combat-npcs.md`

Mark the foundation requirements as complete.

- [ ] **Step 1: Update `docs/features/non-combat-npcs.md`**

Add or replace the Requirements section with:

```markdown
## Requirements

### Foundation (sub-project 1) — complete

- [x] REQ-NPC-1: NPCs with no `npc_type` MUST default to `"combat"` at load time.
- [x] REQ-NPC-2: The type-specific config sub-struct for the declared `npc_type` MUST be non-nil at load time; mismatch MUST be a fatal load error. For `npc_type: "crafter"`, an explicit `crafter: {}` YAML block MUST be present.
- [ ] REQ-NPC-2a: `Template.Validate()` MUST verify all referenced skill IDs exist in the skill registry. *(Deferred to sub-project 3: Service NPCs, where skills are first used)*
- [x] REQ-NPC-3: Non-combat NPCs MUST NOT be added to the combat initiative order (satisfied structurally — only the attacked NPC joins combat; guard engage behavior wired in sub-project 4).
- [x] REQ-NPC-4: Non-combat NPCs MUST NOT be valid attack targets (except engaging guards — enabled in sub-project 4).
- [x] REQ-NPC-13: `ReplenishConfig` MUST satisfy `0 < MinHours <= MaxHours <= 24`; fatal load error on violation.
- [x] REQ-NPC-18: `QuestGiverConfig.PlaceholderDialog` MUST contain at least one entry; fatal load error otherwise.
```

- [ ] **Step 2: Commit**

```bash
git add docs/features/non-combat-npcs.md
git commit -m "docs: mark foundation requirements complete in non-combat-npcs.md"
```

---

## Completion Checklist

- [ ] `internal/game/npc/noncombat.go` created with all 8 config structs + runtime state stubs
- [ ] `Template` has `NPCType`, `Personality`, and 8 config struct pointer fields
- [ ] `Template.Validate()` enforces REQ-NPC-1, REQ-NPC-2, REQ-NPC-13, REQ-NPC-18
- [ ] `Instance` has `NPCType`, `Personality`, `Cowering` fields propagated from template at spawn
- [ ] `CombatHandler.Attack()` rejects non-combat NPCs with "not a valid combat target"
- [ ] `applyCombatStartBehaviorsLocked` fires flee/cower for non-combat NPCs when combat starts
- [ ] `fleeNPCLocked` skips All Out War destination rooms
- [ ] `clearCoweringNPCsLocked` resets Cowering when combat ends
- [ ] All tests pass: `go test ./...`
