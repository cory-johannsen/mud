# Advanced Enemies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Implementation Order for `npc/template.go`:** map-poi → non-human-npcs → npc-behaviors → **advanced-enemies** → factions → curse-removal.

**Goal:** Add 5 enemy difficulty tiers, NPC tag/feat systems with allow/deny lists, boss rooms with hazards/abilities/minions, tier-scaled XP/loot, and rename special_abilities.

**Architecture:** Add Tier field to NPC Template; add Tags []string and Feats []string with deny/allow lists; add BossRoomConfig with hazards/minions/abilities; XP/loot scale formula by tier; rename special_abilities → abilities throughout.

**Tech Stack:** Go, YAML, existing NPC/combat packages

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/game/xp/config.go` | Modify | Add `TierMultiplier` struct and `TierMultipliers` map to `XPConfig`; add `BossKillBonusXP` to `Awards` |
| `internal/game/xp/config_test.go` | Modify | Tests for tier multiplier loading and validation |
| `content/xp_config.yaml` | Modify | Add `tier_multipliers` block and `boss_kill_bonus_xp` |
| `internal/game/npc/template.go` | Modify | Add `Tier`, `Tags`, `Feats`, `BossAbilities`, `SenseAbilities` fields; rename `SpecialAbilities`; update `Validate()`; add `ValidateWithRegistry()` |
| `internal/game/npc/template_test.go` | Modify/Create | Tests for new fields in Validate and ValidateWithRegistry |
| `internal/game/npc/instance.go` | Modify | Add `Tags`, `Feats`, `Tier`, `AbilityCooldowns map[string]time.Time` to `Instance`; add `HasTag()` method; rename `SpecialAbilities` → `SenseAbilities`; update `NewInstanceWithResolver` |
| `internal/game/npc/loot.go` | Modify | Add `TierScale bool` to `LootTable`; update `GenerateLoot` to accept tier loot multiplier |
| `internal/game/npc/loot_test.go` | Modify | Tests for tier-scaled loot generation |
| `internal/game/npc/boss_ability.go` | Create | `BossAbility` and `BossAbilityEffect` struct definitions |
| `internal/game/ruleset/feat.go` | Modify | Add `AllowNPC bool` and `TargetTags []string` to `Feat` struct |
| `internal/game/ruleset/feat_test.go` | Modify | Tests for AllowNPC loading |
| `internal/game/world/model.go` | Modify | Add `BossRoom bool` and `Hazards []HazardDef` to `Room`; define `HazardDef` with `Validate()` |
| `internal/game/world/model_test.go` | Modify | Tests for HazardDef.Validate() and Room fields |
| `internal/game/npc/respawn.go` | Modify | Add coordinated boss respawn logic |
| `internal/game/npc/respawn_test.go` | Modify | Tests for coordinated boss respawn |
| `internal/gameserver/combat_handler.go` | Modify | Apply NPC feat bonuses per round; evaluate boss abilities; fire hazards on room entry and round start; award boss kill bonus XP |
| `internal/frontend/handlers/text_renderer.go` | Modify | Render boss room tiles as `<BB>` |
| `internal/frontend/handlers/text_renderer_test.go` | Modify | Tests for boss room tile rendering |
| `api/proto/game/v1/game.proto` | Modify | Add `bool boss_room = 9` to `MapTile` |
| `internal/gameserver/gamev1/game.pb.go` | Regenerate | Protobuf regeneration |
| `content/feats.yaml` | Modify | Add four NPC feats: `tough`, `brutal_strike`, `evasive`, `pack_tactics` |
| `content/npcs/*.yaml` | Modify | Rename `special_abilities:` → `sense_abilities:` in all NPC YAML files |

---

## Task 1: Rename SpecialAbilities → SenseAbilities (REQ-AE-39)

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/game/npc/npc_model_extensions_test.go`
- Modify: `content/npcs/*.yaml` (all files using `special_abilities:`)

- [ ] **Step 1: Write failing test for SenseAbilities field name**

```go
// internal/game/npc/template_test.go (add to existing test file)
func TestTemplate_SenseAbilitiesField(t *testing.T) {
    data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
sense_abilities:
  - detect_lies
  - read_aura
`)
    tmpl, err := LoadTemplateFromBytes(data)
    require.NoError(t, err)
    assert.Equal(t, []string{"detect_lies", "read_aura"}, tmpl.SenseAbilities)
}

func TestTemplate_SpecialAbilitiesYAMLKeyRejected(t *testing.T) {
    // old key must not silently map — strict decode will ignore it
    data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
special_abilities:
  - detect_lies
`)
    tmpl, err := LoadTemplateFromBytes(data)
    require.NoError(t, err)
    // old key is not aliased — SenseAbilities must be empty
    assert.Empty(t, tmpl.SenseAbilities)
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestTemplate_SenseAbilitiesField -v`
Expected: FAIL (field does not exist yet)

- [ ] **Step 2: Rename SpecialAbilities → SenseAbilities in template.go**

In `internal/game/npc/template.go`, change:
```go
// BEFORE
SpecialAbilities []string `yaml:"special_abilities"`

// AFTER
SenseAbilities []string `yaml:"sense_abilities"`
```

Update the comment above it to read: `// SenseAbilities lists named special abilities for sense motive reveal.`

- [ ] **Step 3: Rename in instance.go**

In `internal/game/npc/instance.go`:
- Change field `SpecialAbilities []string` to `SenseAbilities []string`
- Change the comment from `// SpecialAbilities...` to `// SenseAbilities lists named special abilities copied from the template at spawn.`
- In `NewInstanceWithResolver`: change `SpecialAbilities: append([]string(nil), tmpl.SpecialAbilities...)` to `SenseAbilities: append([]string(nil), tmpl.SenseAbilities...)`

- [ ] **Step 4: Update all other code references**

Run: `grep -rn "SpecialAbilities\|special_abilities" /home/cjohannsen/src/mud/internal/ --include="*.go"`

Update each reference found (expected: `grpc_service.go`, `npc_model_extensions_test.go`). Change each `.SpecialAbilities` field access to `.SenseAbilities`.

- [ ] **Step 5: Migrate NPC YAML files**

Run: `grep -rl "special_abilities:" /home/cjohannsen/src/mud/content/npcs/`

For each file found, replace `special_abilities:` with `sense_abilities:`.

- [ ] **Step 6: Run tests and verify they pass**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1`
Expected: all tests PASS (or only pre-existing failures)

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/template.go internal/game/npc/instance.go internal/gameserver/grpc_service.go internal/game/npc/npc_model_extensions_test.go content/npcs/
git commit -m "refactor(npc): rename SpecialAbilities → SenseAbilities (REQ-AE-39)"
```

---

## Task 2: Add TierMultiplier to XPConfig (REQ-AE-3, REQ-AE-4, REQ-AE-21)

**Files:**
- Modify: `internal/game/xp/config.go`
- Modify: `internal/game/xp/config_test.go`
- Modify: `content/xp_config.yaml`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/xp/config_test.go (add)
func TestXPConfig_TierMultipliers_LoadedFromYAML(t *testing.T) {
    data := []byte(`
base_xp: 100
hp_per_level: 5
boost_interval: 5
level_cap: 100
job_level_cap: 20
awards:
  kill_xp_per_npc_level: 50
  new_room_xp: 10
  skill_check_success_xp: 10
  skill_check_crit_success_xp: 25
  skill_check_dc_multiplier: 2
  boss_kill_bonus_xp: 200
tier_multipliers:
  minion:    { xp: 0.5, loot: 0.5, hp: 0.75 }
  standard:  { xp: 1.0, loot: 1.0, hp: 1.0  }
  elite:     { xp: 2.0, loot: 1.5, hp: 1.5  }
  champion:  { xp: 3.0, loot: 2.0, hp: 2.0  }
  boss:      { xp: 5.0, loot: 3.0, hp: 3.0  }
`)
    var cfg XPConfig
    err := yaml.Unmarshal(data, &cfg)
    require.NoError(t, err)
    require.Len(t, cfg.TierMultipliers, 5)
    assert.InDelta(t, 5.0, cfg.TierMultipliers["boss"].XP, 1e-9)
    assert.InDelta(t, 3.0, cfg.TierMultipliers["boss"].Loot, 1e-9)
    assert.InDelta(t, 3.0, cfg.TierMultipliers["boss"].HP, 1e-9)
    assert.Equal(t, 200, cfg.Awards.BossKillBonusXP)
}

func TestXPConfig_ValidateTiers_MissingTierFatal(t *testing.T) {
    cfg := &XPConfig{
        TierMultipliers: map[string]TierMultiplier{
            "minion": {XP: 0.5, Loot: 0.5, HP: 0.75},
            // missing standard, elite, champion, boss
        },
    }
    err := cfg.ValidateTiers()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "standard")
}

func TestXPConfig_ValidateTiers_AllPresent(t *testing.T) {
    cfg := &XPConfig{
        TierMultipliers: map[string]TierMultiplier{
            "minion":   {XP: 0.5, Loot: 0.5, HP: 0.75},
            "standard": {XP: 1.0, Loot: 1.0, HP: 1.0},
            "elite":    {XP: 2.0, Loot: 1.5, HP: 1.5},
            "champion": {XP: 3.0, Loot: 2.0, HP: 2.0},
            "boss":     {XP: 5.0, Loot: 3.0, HP: 3.0},
        },
    }
    require.NoError(t, cfg.ValidateTiers())
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/xp/... -run TestXPConfig_Tier -v`
Expected: FAIL (TierMultiplier not defined)

- [ ] **Step 2: Add TierMultiplier struct and fields to config.go**

In `internal/game/xp/config.go`, add after the `Awards` struct:

```go
// TierMultiplier holds per-tier scaling coefficients for XP, loot, and HP.
type TierMultiplier struct {
    // XP is the multiplier applied to the base XP award for kills.
    XP float64 `yaml:"xp"`
    // Loot is the multiplier applied to item quantity ranges and credits.
    Loot float64 `yaml:"loot"`
    // HP is the multiplier applied to the template's MaxHP at spawn.
    HP float64 `yaml:"hp"`
}

// CanonicalTiers lists the five tier names that must be present in TierMultipliers.
var CanonicalTiers = []string{"minion", "standard", "elite", "champion", "boss"}
```

Add `BossKillBonusXP int` to `Awards`:
```go
// BossKillBonusXP is the flat XP bonus awarded to each player in the room when a boss dies.
BossKillBonusXP int `yaml:"boss_kill_bonus_xp"`
```

Add `TierMultipliers map[string]TierMultiplier` to `XPConfig`:
```go
// TierMultipliers maps tier name → scaling coefficients.
// Must contain entries for all five canonical tiers (validated at startup).
TierMultipliers map[string]TierMultiplier `yaml:"tier_multipliers"`
```

Add `ValidateTiers()` method to `XPConfig`:
```go
// ValidateTiers checks that TierMultipliers contains all five canonical tier entries.
//
// Precondition: cfg must not be nil.
// Postcondition: Returns nil iff all canonical tiers are present; returns an error
// listing the first missing tier otherwise.
func (cfg *XPConfig) ValidateTiers() error {
    for _, tier := range CanonicalTiers {
        if _, ok := cfg.TierMultipliers[tier]; !ok {
            return fmt.Errorf("xp_config: missing tier_multipliers entry for %q", tier)
        }
    }
    return nil
}
```

- [ ] **Step 3: Update content/xp_config.yaml**

```yaml
# XP curve: xp_to_reach_level(n) = n² × base_xp
base_xp: 100
# Max HP increase per level-up
hp_per_level: 5
# Ability boost granted every N levels
boost_interval: 5
# Hard cap on character level
level_cap: 100
# Hard cap on any single job's level (reserved for job advancement)
job_level_cap: 20

awards:
  # XP per NPC level for a kill (total = npc_level × this)
  kill_xp_per_npc_level: 50
  # Flat XP for discovering a new room
  new_room_xp: 10
  # Base XP for a skill check success outcome
  skill_check_success_xp: 10
  # Base XP for a skill check crit_success outcome
  skill_check_crit_success_xp: 25
  # Multiplied by DC and added to skill check XP (total = base + DC × this)
  skill_check_dc_multiplier: 2
  # Flat bonus XP awarded to each player in the room when a boss NPC dies
  boss_kill_bonus_xp: 200

# Per-tier scaling multipliers for XP, loot quantity, and HP pool.
# All five canonical tiers must be present; missing entries cause fatal startup.
tier_multipliers:
  minion:   { xp: 0.5, loot: 0.5, hp: 0.75 }
  standard: { xp: 1.0, loot: 1.0, hp: 1.0  }
  elite:    { xp: 2.0, loot: 1.5, hp: 1.5  }
  champion: { xp: 3.0, loot: 2.0, hp: 2.0  }
  boss:     { xp: 5.0, loot: 3.0, hp: 3.0  }
```

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/xp/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/xp/config.go internal/game/xp/config_test.go content/xp_config.yaml
git commit -m "feat(xp): add TierMultiplier, TierMultipliers, BossKillBonusXP, ValidateTiers (REQ-AE-3, REQ-AE-4, REQ-AE-21)"
```

---

## Task 3: Add Tier field to NPC Template (REQ-AE-1, REQ-AE-2)

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/template_test.go` (or create if absent)

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/template_test.go (add)
func TestTemplate_Tier_DefaultsToStandardAtValidate(t *testing.T) {
    data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
`)
    tmpl, err := LoadTemplateFromBytes(data)
    require.NoError(t, err)
    // Tier is empty in YAML — Validate normalizes to ""
    // actual tier resolution to "standard" happens at usage time
    assert.Equal(t, "", tmpl.Tier)
}

func TestTemplate_Tier_ValidValues(t *testing.T) {
    for _, tier := range []string{"minion", "standard", "elite", "champion", "boss"} {
        data := []byte(fmt.Sprintf(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
tier: %s
`, tier))
        tmpl, err := LoadTemplateFromBytes(data)
        require.NoError(t, err, "tier %q should be valid", tier)
        assert.Equal(t, tier, tmpl.Tier)
    }
}

func TestTemplate_Tier_InvalidRejected(t *testing.T) {
    data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
tier: legendary
`)
    _, err := LoadTemplateFromBytes(data)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "tier")
}

func TestProperty_Template_TierValidation(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        tier := rapid.StringN(1, 20, -1).Draw(t, "tier")
        valid := map[string]bool{
            "minion": true, "standard": true, "elite": true,
            "champion": true, "boss": true, "": true,
        }
        data := []byte(fmt.Sprintf(`
id: npc_%s
name: NPC %s
level: 1
max_hp: 10
ac: 10
tier: %s
`, tier, tier, tier))
        _, err := LoadTemplateFromBytes(data)
        if valid[tier] {
            assert.NoError(t, err)
        } else {
            assert.Error(t, err)
        }
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestTemplate_Tier -v`
Expected: FAIL (Tier field does not exist)

- [ ] **Step 2: Add Tier field to Template struct in template.go**

Add after the `Disposition string` field (before the `NPCType` field):
```go
// Tier sets the difficulty tier for this NPC. Valid values: "minion", "standard",
// "elite", "champion", "boss". Empty means "standard" is assumed at runtime.
Tier string `yaml:"tier"`
```

- [ ] **Step 3: Add tier validation in Template.Validate()**

In `Template.Validate()`, add after the existing `RobMultiplier` check:
```go
validTiers := map[string]bool{
    "": true, "minion": true, "standard": true,
    "elite": true, "champion": true, "boss": true,
}
if !validTiers[t.Tier] {
    return fmt.Errorf("npc template %q: unknown tier %q", t.ID, t.Tier)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/template.go internal/game/npc/template_test.go
git commit -m "feat(npc): add Tier field with validation to Template (REQ-AE-1, REQ-AE-2)"
```

---

## Task 4: Add Tags to NPC Template and Instance (REQ-AE-7, REQ-AE-8, REQ-AE-9, REQ-AE-10)

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/template_test.go`
- Modify: `internal/game/npc/instance_test.go` (create if absent)

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/template_test.go (add)
func TestTemplate_Tags_PropagatedToInstance(t *testing.T) {
    data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
tags:
  - undead
  - flying
`)
    tmpl, err := LoadTemplateFromBytes(data)
    require.NoError(t, err)
    assert.Equal(t, []string{"undead", "flying"}, tmpl.Tags)
}

// internal/game/npc/instance_test.go (create/add)
func TestInstance_HasTag_True(t *testing.T) {
    inst := &Instance{Tags: []string{"undead", "flying"}}
    assert.True(t, inst.HasTag("undead"))
    assert.True(t, inst.HasTag("flying"))
}

func TestInstance_HasTag_False(t *testing.T) {
    inst := &Instance{Tags: []string{"undead"}}
    assert.False(t, inst.HasTag("robot"))
}

func TestInstance_HasTag_Empty(t *testing.T) {
    inst := &Instance{}
    assert.False(t, inst.HasTag("anything"))
}

func TestProperty_Instance_HasTag_Reflexive(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        tags := rapid.SliceOf(rapid.StringN(1, 10, -1)).Draw(t, "tags")
        inst := &Instance{Tags: tags}
        for _, tag := range tags {
            assert.True(t, inst.HasTag(tag), "HasTag must return true for any tag in Tags")
        }
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_Tags|TestInstance_HasTag" -v`
Expected: FAIL

- [ ] **Step 2: Add Tags field to Template struct**

In `internal/game/npc/template.go`, add after `Tier string`:
```go
// Tags is a list of free-form content labels. Not code-enforced.
Tags []string `yaml:"tags"`
```

- [ ] **Step 3: Add Tags field and HasTag method to Instance**

In `internal/game/npc/instance.go`, add after the `Disposition` field:
```go
// Tags is the list of content labels propagated from the template at spawn.
Tags []string
```

Add method:
```go
// HasTag reports whether the given tag is present in the instance's tag list.
//
// Precondition: tag must be non-empty for a meaningful result.
// Postcondition: Returns true iff tag is present in Tags; false otherwise.
func (i *Instance) HasTag(tag string) bool {
    for _, t := range i.Tags {
        if t == tag {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: Propagate Tags in NewInstanceWithResolver**

In `NewInstanceWithResolver`, add to the returned `Instance`:
```go
Tags:             append([]string(nil), tmpl.Tags...),
```

- [ ] **Step 5: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/template_test.go internal/game/npc/instance_test.go
git commit -m "feat(npc): add Tags field to Template/Instance; add HasTag method (REQ-AE-7–10)"
```

---

## Task 5: Add AllowNPC and TargetTags to Feat (REQ-AE-11, REQ-AE-12)

**Files:**
- Modify: `internal/game/ruleset/feat.go`
- Modify: `internal/game/ruleset/feat_test.go`
- Modify: `content/feats.yaml`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/ruleset/feat_test.go (add)
func TestFeat_AllowNPC_DefaultFalse(t *testing.T) {
    data := []byte(`
feats:
  - id: test_feat
    name: Test Feat
    category: general
    active: false
    description: "A test feat."
`)
    feats, err := LoadFeatsFromBytes(data)
    require.NoError(t, err)
    require.Len(t, feats, 1)
    assert.False(t, feats[0].AllowNPC)
}

func TestFeat_AllowNPC_TrueWhenSet(t *testing.T) {
    data := []byte(`
feats:
  - id: brutal_strike
    name: Brutal Strike
    category: general
    active: false
    allow_npc: true
    description: "+2 damage."
`)
    feats, err := LoadFeatsFromBytes(data)
    require.NoError(t, err)
    assert.True(t, feats[0].AllowNPC)
}

func TestFeat_TargetTags_Loaded(t *testing.T) {
    data := []byte(`
feats:
  - id: hunter_feat
    name: Hunter Feat
    category: general
    active: false
    target_tags:
      - undead
      - mutant
    description: "Bonus vs tagged targets."
`)
    feats, err := LoadFeatsFromBytes(data)
    require.NoError(t, err)
    assert.Equal(t, []string{"undead", "mutant"}, feats[0].TargetTags)
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestFeat_AllowNPC -v`
Expected: FAIL

- [ ] **Step 2: Add AllowNPC and TargetTags to Feat struct**

In `internal/game/ruleset/feat.go`, add to `Feat` struct:
```go
// AllowNPC when true allows this feat to be assigned to NPC templates.
// Default false. Only feats with AllowNPC == true may appear in Template.Feats.
AllowNPC bool `yaml:"allow_npc"`
// TargetTags is an optional list of NPC tags; when non-empty, the feat bonus
// applies only when the combat target has at least one matching tag.
TargetTags []string `yaml:"target_tags"`
```

- [ ] **Step 3: Add four NPC feats to content/feats.yaml**

At the end of the general feats section, add:
```yaml
  - id: tough
    name: Tough
    category: general
    pf2e: toughness
    active: false
    allow_npc: true
    activate_text: ""
    description: "Flat +5 max HP bonus, applied at spawn after tier multiplier."

  - id: brutal_strike
    name: Brutal Strike
    category: general
    pf2e: ""
    active: false
    allow_npc: true
    activate_text: ""
    description: "+2 damage on all attacks."

  - id: evasive
    name: Evasive
    category: general
    pf2e: ""
    active: false
    allow_npc: true
    activate_text: ""
    description: "+2 AC."

  - id: pack_tactics
    name: Pack Tactics
    category: general
    pf2e: ""
    active: false
    allow_npc: true
    activate_text: ""
    description: "+2 attack bonus when at least one ally NPC in the room targets the same player."
```

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/ruleset/feat.go internal/game/ruleset/feat_test.go content/feats.yaml
git commit -m "feat(ruleset): add AllowNPC, TargetTags to Feat; add four NPC feats (REQ-AE-11, REQ-AE-12, REQ-AE-17)"
```

---

## Task 6: Add NPC Feats to Template/Instance (REQ-AE-13, REQ-AE-14, REQ-AE-15)

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/template_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/template_test.go (add)
func TestTemplate_ValidateWithRegistry_UnknownFeat(t *testing.T) {
    tmpl := &Template{
        ID: "test", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
        Feats: []string{"nonexistent_feat"},
    }
    registry := ruleset.NewFeatRegistry([]*ruleset.Feat{})
    err := tmpl.ValidateWithRegistry(registry)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "nonexistent_feat")
}

func TestTemplate_ValidateWithRegistry_FeatNotAllowNPC(t *testing.T) {
    tmpl := &Template{
        ID: "test", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
        Feats: []string{"player_only_feat"},
    }
    feats := []*ruleset.Feat{{ID: "player_only_feat", Name: "PO Feat", AllowNPC: false}}
    registry := ruleset.NewFeatRegistry(feats)
    err := tmpl.ValidateWithRegistry(registry)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "player_only_feat")
}

func TestTemplate_ValidateWithRegistry_ValidFeats(t *testing.T) {
    tmpl := &Template{
        ID: "test", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
        Feats: []string{"tough", "brutal_strike"},
    }
    feats := []*ruleset.Feat{
        {ID: "tough", Name: "Tough", AllowNPC: true},
        {ID: "brutal_strike", Name: "Brutal Strike", AllowNPC: true},
    }
    registry := ruleset.NewFeatRegistry(feats)
    err := tmpl.ValidateWithRegistry(registry)
    require.NoError(t, err)
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestTemplate_ValidateWithRegistry -v`
Expected: FAIL

- [ ] **Step 2: Add Feats field to Template**

In `internal/game/npc/template.go`, add after `Tags []string`:
```go
// Feats is a list of feat IDs assigned to this NPC. Each must be an NPC-valid feat
// (AllowNPC == true). Validated by ValidateWithRegistry at startup.
Feats []string `yaml:"feats"`
```

- [ ] **Step 3: Add ValidateWithRegistry to template.go**

Add after `ValidateWithSkills`:
```go
// ValidateWithRegistry verifies that all feat IDs in Feats exist in registry
// and have AllowNPC == true.
//
// Precondition: t must not be nil; registry must not be nil.
// Postcondition: Returns nil iff all feats are valid for NPC use; error otherwise.
func (t *Template) ValidateWithRegistry(registry *ruleset.FeatRegistry) error {
    for _, featID := range t.Feats {
        f, ok := registry.Feat(featID)
        if !ok {
            return fmt.Errorf("npc template %q: feat %q not found in registry", t.ID, featID)
        }
        if !f.AllowNPC {
            return fmt.Errorf("npc template %q: feat %q does not have allow_npc: true", t.ID, featID)
        }
    }
    return nil
}
```

Add the required import in `template.go`:
```go
"github.com/cory-johannsen/mud/internal/game/ruleset"
```

- [ ] **Step 4: Add Feats field to Instance; propagate in NewInstanceWithResolver**

In `internal/game/npc/instance.go`, add after `Tags []string`:
```go
// Feats is the list of feat IDs propagated from the template at spawn.
Feats []string
// Tier is the difficulty tier propagated from the template at spawn.
// Empty string means "standard" is assumed.
Tier string
```

In `NewInstanceWithResolver`, add:
```go
Feats: append([]string(nil), tmpl.Feats...),
Tier:  tmpl.Tier,
```

- [ ] **Step 5: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/template_test.go
git commit -m "feat(npc): add Feats/Tier fields; add ValidateWithRegistry (REQ-AE-13–15)"
```

---

## Task 7: Define BossAbility types; add to Template; update Validate (REQ-AE-31, REQ-AE-32, REQ-AE-33, REQ-AE-34)

**Files:**
- Create: `internal/game/npc/boss_ability.go`
- Create: `internal/game/npc/boss_ability_test.go`
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/boss_ability_test.go (create)
package npc_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/npc"
)

func TestBossAbilityEffect_Validate_ExactlyOneField(t *testing.T) {
    // Zero fields — invalid
    eff := npc.BossAbilityEffect{}
    assert.Error(t, eff.Validate())

    // Exactly one field — valid
    assert.NoError(t, npc.BossAbilityEffect{AoeCondition: "poisoned"}.Validate())
    assert.NoError(t, npc.BossAbilityEffect{AoeDamageExpr: "3d6"}.Validate())
    assert.NoError(t, npc.BossAbilityEffect{HealPct: 25}.Validate())

    // Two fields — invalid
    assert.Error(t, npc.BossAbilityEffect{AoeCondition: "poisoned", HealPct: 25}.Validate())
    assert.Error(t, npc.BossAbilityEffect{AoeDamageExpr: "2d6", HealPct: 10}.Validate())
}

func TestBossAbility_ValidateTrigger(t *testing.T) {
    base := npc.BossAbility{
        ID: "test", Name: "Test",
        Effect: npc.BossAbilityEffect{AoeDamageExpr: "2d6"},
    }

    for _, trigger := range []string{"hp_pct_below", "round_start", "on_damage_taken"} {
        a := base
        a.Trigger = trigger
        assert.NoError(t, a.Validate(), "trigger %q should be valid", trigger)
    }

    base.Trigger = "on_death"
    assert.Error(t, base.Validate())
}

func TestBossAbility_ValidateCooldown(t *testing.T) {
    a := npc.BossAbility{
        ID: "test", Name: "Test", Trigger: "round_start",
        Effect:   npc.BossAbilityEffect{AoeDamageExpr: "1d6"},
        Cooldown: "not_a_duration",
    }
    assert.Error(t, a.Validate())

    a.Cooldown = "30s"
    assert.NoError(t, a.Validate())

    a.Cooldown = ""
    assert.NoError(t, a.Validate())
}

func TestBossAbility_OnDamageTaken_TriggerValueMustBeZero(t *testing.T) {
    a := npc.BossAbility{
        ID: "test", Name: "Test", Trigger: "on_damage_taken",
        TriggerValue: 50,
        Effect:       npc.BossAbilityEffect{AoeDamageExpr: "2d6"},
    }
    assert.Error(t, a.Validate())

    a.TriggerValue = 0
    assert.NoError(t, a.Validate())
}

func TestProperty_BossAbilityEffect_ExactlyOneSet(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cond := rapid.StringN(0, 20, -1).Draw(t, "cond")
        dmg := rapid.StringN(0, 20, -1).Draw(t, "dmg")
        heal := rapid.IntRange(-100, 100).Draw(t, "heal")

        eff := npc.BossAbilityEffect{
            AoeCondition:  cond,
            AoeDamageExpr: dmg,
            HealPct:       heal,
        }
        setCount := 0
        if cond != "" {
            setCount++
        }
        if dmg != "" {
            setCount++
        }
        if heal != 0 {
            setCount++
        }
        err := eff.Validate()
        if setCount == 1 {
            assert.NoError(t, err)
        } else {
            assert.Error(t, err)
        }
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestBossAbility -v`
Expected: FAIL (BossAbility not defined)

- [ ] **Step 2: Create internal/game/npc/boss_ability.go**

```go
// Package npc — BossAbility and BossAbilityEffect type definitions.
package npc

import (
    "fmt"
    "time"
)

// BossAbilityEffect holds the mechanical outcome of a boss ability.
// Exactly one field must be set (non-zero / non-empty).
type BossAbilityEffect struct {
    // AoeCondition is the condition ID to apply to all players in the room.
    AoeCondition string `yaml:"aoe_condition"`
    // AoeDamageExpr is a dice expression for AoE damage applied to all players.
    AoeDamageExpr string `yaml:"aoe_damage_expr"`
    // HealPct is the percentage of MaxHP to restore to the boss (non-zero).
    HealPct int `yaml:"heal_pct"`
}

// Validate checks that exactly one field is set.
//
// Precondition: none.
// Postcondition: returns nil iff exactly one field is non-zero/non-empty.
func (e BossAbilityEffect) Validate() error {
    set := 0
    if e.AoeCondition != "" {
        set++
    }
    if e.AoeDamageExpr != "" {
        set++
    }
    if e.HealPct != 0 {
        set++
    }
    if set != 1 {
        return fmt.Errorf("boss_ability_effect: exactly one field must be set, got %d", set)
    }
    return nil
}

// BossAbility defines a special ability that a boss NPC can use during combat.
type BossAbility struct {
    // ID is a unique identifier for this ability within the template.
    ID string `yaml:"id"`
    // Name is the player-visible display name of the ability.
    Name string `yaml:"name"`
    // Trigger determines when this ability fires.
    // Valid values: "hp_pct_below", "round_start", "on_damage_taken".
    Trigger string `yaml:"trigger"`
    // TriggerValue holds the threshold for trigger evaluation.
    // For "hp_pct_below": HP percentage (e.g. 50 = fires below 50% HP).
    // For "round_start": round number (0 = every round).
    // For "on_damage_taken": must be 0 (unused).
    TriggerValue int `yaml:"trigger_value"`
    // Cooldown is a Go duration string (e.g. "30s"). Empty means no cooldown.
    Cooldown string `yaml:"cooldown"`
    // Effect is the mechanical outcome when this ability fires.
    Effect BossAbilityEffect `yaml:"effect"`
}

// Validate checks the ability definition for correctness.
//
// Precondition: none.
// Postcondition: returns nil iff all fields are valid per REQ-AE-33.
func (a BossAbility) Validate() error {
    if a.ID == "" {
        return fmt.Errorf("boss_ability: id must not be empty")
    }
    if a.Name == "" {
        return fmt.Errorf("boss_ability %q: name must not be empty", a.ID)
    }
    validTriggers := map[string]bool{
        "hp_pct_below": true, "round_start": true, "on_damage_taken": true,
    }
    if !validTriggers[a.Trigger] {
        return fmt.Errorf("boss_ability %q: unknown trigger %q", a.ID, a.Trigger)
    }
    if a.Trigger == "on_damage_taken" && a.TriggerValue != 0 {
        return fmt.Errorf("boss_ability %q: trigger_value must be 0 for on_damage_taken", a.ID)
    }
    if a.Cooldown != "" {
        if _, err := time.ParseDuration(a.Cooldown); err != nil {
            return fmt.Errorf("boss_ability %q: cooldown %q is not a valid duration: %w", a.ID, a.Cooldown, err)
        }
    }
    if err := a.Effect.Validate(); err != nil {
        return fmt.Errorf("boss_ability %q: %w", a.ID, err)
    }
    return nil
}
```

- [ ] **Step 3: Add BossAbilities to Template; update Validate()**

In `internal/game/npc/template.go`, add after `SenseAbilities`:
```go
// BossAbilities defines the set of special abilities for boss-tier NPCs.
// Validated by Template.Validate().
BossAbilities []BossAbility `yaml:"boss_abilities"`
```

In `Template.Validate()`, add after the tier validation:
```go
for _, ability := range t.BossAbilities {
    if err := ability.Validate(); err != nil {
        return fmt.Errorf("npc template %q: %w", t.ID, err)
    }
}
```

- [ ] **Step 4: Add AbilityCooldowns to Instance; initialize at spawn**

In `internal/game/npc/instance.go`, add after `Tier string`:
```go
// AbilityCooldowns maps boss ability ID → time after which it may fire again.
// Initialized to an empty non-nil map at spawn. Nil-safe check is not required.
AbilityCooldowns map[string]time.Time
```

In `NewInstanceWithResolver`, add:
```go
AbilityCooldowns: make(map[string]time.Time),
```

- [ ] **Step 5: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/boss_ability.go internal/game/npc/boss_ability_test.go internal/game/npc/template.go internal/game/npc/instance.go
git commit -m "feat(npc): add BossAbility types, Template.BossAbilities, Instance.AbilityCooldowns (REQ-AE-31–34)"
```

---

## Task 8: Tier HP scaling at spawn; tough feat bonus (REQ-AE-5, REQ-AE-18)

**Files:**
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/instance_test.go`

The `NewInstanceWithResolver` signature must change to accept `*xp.XPConfig` and `*ruleset.FeatRegistry`. All callers must be updated.

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/instance_test.go (add)
func TestNewInstanceWithResolver_TierHPScaling(t *testing.T) {
    cfg := &xp.XPConfig{
        TierMultipliers: map[string]xp.TierMultiplier{
            "minion":   {HP: 0.75}, "standard": {HP: 1.0},
            "elite":    {HP: 1.5},  "champion": {HP: 2.0},
            "boss":     {HP: 3.0},
        },
    }
    tmpl := &npc.Template{
        ID: "t", Name: "T", Level: 1, MaxHP: 20, AC: 10, Tier: "elite",
    }
    inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, cfg, nil)
    // ceil(20 * 1.5) = 30
    assert.Equal(t, 30, inst.MaxHP)
    assert.Equal(t, 30, inst.CurrentHP)
}

func TestNewInstanceWithResolver_ToughFeatAddsFiveHP(t *testing.T) {
    cfg := &xp.XPConfig{
        TierMultipliers: map[string]xp.TierMultiplier{
            "standard": {HP: 1.0},
        },
    }
    toughFeat := &ruleset.Feat{ID: "tough", Name: "Tough", AllowNPC: true}
    registry := ruleset.NewFeatRegistry([]*ruleset.Feat{toughFeat})
    tmpl := &npc.Template{
        ID: "t", Name: "T", Level: 1, MaxHP: 20, AC: 10,
        Tier: "standard", Feats: []string{"tough"},
    }
    inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, cfg, registry)
    // ceil(20 * 1.0) = 20, then + 5 from tough = 25
    assert.Equal(t, 25, inst.MaxHP)
    assert.Equal(t, 25, inst.CurrentHP)
}

func TestNewInstanceWithResolver_EmptyTierDefaultsToStandard(t *testing.T) {
    cfg := &xp.XPConfig{
        TierMultipliers: map[string]xp.TierMultiplier{
            "standard": {HP: 1.0},
        },
    }
    tmpl := &npc.Template{
        ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10,
        // Tier is empty — should default to "standard"
    }
    inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, cfg, nil)
    assert.Equal(t, 10, inst.MaxHP)
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestNewInstanceWithResolver_Tier -v`
Expected: FAIL (signature mismatch)

- [ ] **Step 2: Update NewInstanceWithResolver signature**

Change the signature to:
```go
func NewInstanceWithResolver(id string, tmpl *Template, roomID string, armorACBonus func(string) int, xpCfg *xp.XPConfig, featRegistry *ruleset.FeatRegistry) *Instance
```

Add `"math"` to imports in `instance.go`.

In the function body, compute tier-scaled MaxHP:
```go
tier := tmpl.Tier
if tier == "" {
    tier = "standard"
}

maxHP := tmpl.MaxHP
if xpCfg != nil {
    if mult, ok := xpCfg.TierMultipliers[tier]; ok {
        maxHP = int(math.Ceil(float64(tmpl.MaxHP) * mult.HP))
    }
}

// Apply tough feat bonus (+5 HP) after tier multiplier.
if featRegistry != nil {
    for _, featID := range tmpl.Feats {
        if featID == "tough" {
            if f, ok := featRegistry.Feat("tough"); ok && f.AllowNPC {
                maxHP += 5
            }
        }
    }
}
```

Update the returned `Instance` to use `maxHP`:
```go
CurrentHP: maxHP,
MaxHP:     maxHP,
```

- [ ] **Step 3: Find and update all callers of NewInstanceWithResolver**

Run: `grep -rn "NewInstanceWithResolver" /home/cjohannsen/src/mud --include="*.go"`

Update every call site to pass `nil, nil` for the new parameters, or the real `xpCfg` and `featRegistry` where available (typically in `npc.Manager.Spawn` or similar).

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/instance.go internal/game/npc/instance_test.go
git commit -m "feat(npc): apply tier HP multiplier and tough feat bonus at spawn (REQ-AE-5, REQ-AE-18)"
```

---

## Task 9: AwardKill tier XP scaling (REQ-AE-6)

**Files:**
- Modify: `internal/game/xp/service.go`
- Modify: `internal/game/xp/service_test.go`
- Modify: `internal/gameserver/combat_handler.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/xp/service_test.go (add)
func TestService_AwardKill_TierScaling_Elite(t *testing.T) {
    cfg := &xp.XPConfig{
        BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, LevelCap: 100,
        Awards: xp.Awards{KillXPPerNPCLevel: 50},
        TierMultipliers: map[string]xp.TierMultiplier{
            "standard": {XP: 1.0}, "elite": {XP: 2.0},
        },
    }
    saver := &mockSaver{}
    svc := xp.NewService(cfg, saver)
    sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}

    // NPC level 3 elite: base = 3*50 = 150, then *2.0 = 300
    _, err := svc.AwardKill(context.Background(), sess, 3, 1, "elite")
    require.NoError(t, err)
    assert.Equal(t, 300, sess.Experience)
}

func TestService_AwardKill_EmptyTierDefaultsToStandard(t *testing.T) {
    cfg := &xp.XPConfig{
        BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, LevelCap: 100,
        Awards: xp.Awards{KillXPPerNPCLevel: 50},
        TierMultipliers: map[string]xp.TierMultiplier{
            "standard": {XP: 1.0},
        },
    }
    saver := &mockSaver{}
    svc := xp.NewService(cfg, saver)
    sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}

    _, err := svc.AwardKill(context.Background(), sess, 2, 1, "")
    require.NoError(t, err)
    assert.Equal(t, 100, sess.Experience) // 2*50*1.0
}

func TestProperty_AwardKill_TierMultipliesMonotonically(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        level := rapid.IntRange(1, 20).Draw(t, "level")
        mults := map[string]xp.TierMultiplier{
            "minion": {XP: 0.5}, "standard": {XP: 1.0},
            "elite": {XP: 2.0}, "champion": {XP: 3.0}, "boss": {XP: 5.0},
        }
        cfg := &xp.XPConfig{
            BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, LevelCap: 100,
            Awards: xp.Awards{KillXPPerNPCLevel: 50},
            TierMultipliers: mults,
        }
        saver := &mockSaver{}

        tierOrder := []string{"minion", "standard", "elite", "champion", "boss"}
        prevXP := 0
        for _, tier := range tierOrder {
            sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}
            _, err := xp.NewService(cfg, saver).AwardKill(context.Background(), sess, level, 1, tier)
            require.NoError(t, err)
            assert.GreaterOrEqual(t, sess.Experience, prevXP,
                "XP for tier %q must be >= tier below it", tier)
            prevXP = sess.Experience
        }
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/xp/... -run TestService_AwardKill_Tier -v`
Expected: FAIL

- [ ] **Step 2: Update AwardKill signature and implementation**

In `internal/game/xp/service.go`, change:
```go
// BEFORE
func (s *Service) AwardKill(ctx context.Context, sess *session.PlayerSession, npcLevel int, characterID int64) ([]string, error) {
    return s.award(ctx, sess, characterID, npcLevel*s.cfg.Awards.KillXPPerNPCLevel)
}

// AFTER
func (s *Service) AwardKill(ctx context.Context, sess *session.PlayerSession, npcLevel int, characterID int64, tier string) ([]string, error) {
    if tier == "" {
        tier = "standard"
    }
    base := npcLevel * s.cfg.Awards.KillXPPerNPCLevel
    if mult, ok := s.cfg.TierMultipliers[tier]; ok {
        base = int(math.Ceil(float64(base) * mult.XP))
    }
    return s.award(ctx, sess, characterID, base)
}
```

Add `"math"` to imports in `service.go`.

- [ ] **Step 3: Update all AwardKill callers**

Run: `grep -rn "\.AwardKill(" /home/cjohannsen/src/mud --include="*.go"`

In `internal/gameserver/combat_handler.go`, find the XP award section (around line 2833) and update the raw `KillXPPerNPCLevel` calculation — this now flows through `AwardKill`. Pass the instance's tier:

```go
// The tier is now passed to AwardKill, which handles multiplier internally.
// Replace the raw totalXP calculation with AwardKill calls directly.
// inst.Tier may be empty — AwardKill defaults to "standard".
```

The combat handler currently computes `totalXP = inst.Level * cfg.Awards.KillXPPerNPCLevel` and then calls `AwardXPAmount`. Update it to call `AwardKill(ctx, p, inst.Level, p.CharacterID, inst.Tier)` for each participant directly, OR compute the tier-multiplied total via `AwardKill` for first participant and replicate the share logic. The cleanest approach is to compute the total XP once with the tier multiplier then split:

```go
totalXP := inst.Level * cfg.Awards.KillXPPerNPCLevel
if mult, ok := cfg.TierMultipliers[effectiveTier]; ok {
    totalXP = int(math.Ceil(float64(totalXP) * mult.XP))
}
// ... rest of split/share logic unchanged, using AwardXPAmount
```

where `effectiveTier` is `inst.Tier` (with empty → `"standard"` fallback).

Update all other existing `AwardKill` call sites to pass the tier parameter (pass `""` for places where tier is unknown or not applicable).

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/xp/service.go internal/game/xp/service_test.go internal/gameserver/combat_handler.go
git commit -m "feat(xp): tier-scaled AwardKill; combat handler uses tier multiplier (REQ-AE-6)"
```

---

## Task 10: TierScale loot (REQ-AE-19, REQ-AE-20)

**Files:**
- Modify: `internal/game/npc/loot.go`
- Modify: `internal/game/npc/loot_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/loot_test.go (add)
func TestGenerateLoot_TierScale_QuantityScaled(t *testing.T) {
    lt := npc.LootTable{
        TierScale: true,
        Items: []npc.ItemDrop{
            {ItemID: "ammo", Chance: 1.0, MinQty: 2, MaxQty: 4},
        },
    }
    // loot multiplier 2.0: MinQty = ceil(2*2.0)=4, MaxQty = ceil(4*2.0)=8
    result := npc.GenerateLootWithTier(lt, 2.0)
    require.Len(t, result.Items, 1)
    assert.GreaterOrEqual(t, result.Items[0].Quantity, 4)
    assert.LessOrEqual(t, result.Items[0].Quantity, 8)
}

func TestGenerateLoot_TierScale_CreditsScaled(t *testing.T) {
    lt := npc.LootTable{
        TierScale: true,
        Currency: &npc.CurrencyDrop{
            CreditsMin: 10, CreditsMax: 20,
        },
    }
    // loot multiplier 3.0: min=30, max=60
    result := npc.GenerateLootWithTier(lt, 3.0)
    assert.GreaterOrEqual(t, result.Currency, 30)
    assert.LessOrEqual(t, result.Currency, 60)
}

func TestGenerateLoot_NoTierScale_Unchanged(t *testing.T) {
    lt := npc.LootTable{
        TierScale: false,
        Items: []npc.ItemDrop{
            {ItemID: "ammo", Chance: 1.0, MinQty: 2, MaxQty: 4},
        },
    }
    result := npc.GenerateLootWithTier(lt, 5.0) // scale ignored
    require.Len(t, result.Items, 1)
    assert.GreaterOrEqual(t, result.Items[0].Quantity, 2)
    assert.LessOrEqual(t, result.Items[0].Quantity, 4)
}

func TestProperty_TierScale_MinimumOne(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        mult := rapid.Float64Range(0.01, 0.9).Draw(t, "mult")
        lt := npc.LootTable{
            TierScale: true,
            Items: []npc.ItemDrop{
                {ItemID: "item", Chance: 1.0, MinQty: 1, MaxQty: 1},
            },
        }
        result := npc.GenerateLootWithTier(lt, mult)
        require.Len(t, result.Items, 1)
        assert.GreaterOrEqual(t, result.Items[0].Quantity, 1, "quantity must be at least 1 after scaling")
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestGenerateLoot_TierScale -v`
Expected: FAIL

- [ ] **Step 2: Add TierScale bool to LootTable; add CreditsMin/CreditsMax to CurrencyDrop; add GenerateLootWithTier**

Current `CurrencyDrop` uses `Min`/`Max`. The spec requires `credits_min`/`credits_max` YAML keys (REQ-AE-20). Add new exported fields with YAML tags — but first check the existing YAML content. Looking at current `loot.go`, `CurrencyDrop` has `Min`/`Max`. To avoid breaking existing YAML, add `CreditsMin`/`CreditsMax` as the primary fields OR rename Min/Max with YAML tag `credits_min`/`credits_max`. Per REQ-AE-20, which references `credits_min`/`credits_max` as scaling targets, simply ensure the YAML tags on the existing `Min`/`Max` fields include these aliases. The cleanest approach without breaking existing NPC files is to keep `Min`/`Max` fields but add `yaml:"credits_min"` and `yaml:"credits_max"` as the YAML tags (replacing the current `min`/`max` tags) and update all NPC YAML files. Do this consistently.

Alternatively: add `TierScale bool` to `LootTable`, and in `GenerateLootWithTier` scale the existing `Min`/`Max` by the tier loot multiplier. Update all NPC YAML currency blocks from `min:`/`max:` to `credits_min:`/`credits_max:`.

In `internal/game/npc/loot.go`:
```go
// Add to LootTable:
// TierScale when true causes quantity ranges and credits to be scaled
// by the NPC tier's Loot multiplier at generation time.
TierScale bool `yaml:"tier_scale"`

// Update CurrencyDrop YAML tags:
type CurrencyDrop struct {
    Min int `yaml:"credits_min"`
    Max int `yaml:"credits_max"`
}
```

Add `GenerateLootWithTier`:
```go
// GenerateLootWithTier rolls loot from lt, applying lootMult when TierScale is true.
// lootMult is the tier's Loot multiplier (e.g. 1.5 for elite).
// When TierScale is false, lootMult is ignored. Minimum quantity after scaling is 1.
//
// Precondition: lt must have passed Validate(); lootMult > 0.
// Postcondition: Returns a LootResult with scaled quantities when TierScale is true.
func GenerateLootWithTier(lt LootTable, lootMult float64) LootResult {
    var result LootResult

    if lt.Currency != nil && lt.Currency.Max > 0 {
        cMin, cMax := lt.Currency.Min, lt.Currency.Max
        if lt.TierScale {
            cMin = max1(int(math.Ceil(float64(cMin) * lootMult)))
            cMax = max1(int(math.Ceil(float64(cMax) * lootMult)))
        }
        spread := cMax - cMin
        if spread == 0 {
            result.Currency = cMin
        } else {
            result.Currency = cMin + rand.Intn(spread+1)
        }
    }

    for _, item := range lt.Items {
        if rand.Float64() < item.Chance {
            minQ, maxQ := item.MinQty, item.MaxQty
            if lt.TierScale {
                minQ = max1(int(math.Ceil(float64(minQ) * lootMult)))
                maxQ = max1(int(math.Ceil(float64(maxQ) * lootMult)))
            }
            qty := minQ
            spread := maxQ - minQ
            if spread > 0 {
                qty += rand.Intn(spread + 1)
            }
            result.Items = append(result.Items, LootItem{
                ItemDefID:  item.ItemID,
                InstanceID: uuid.New().String(),
                Quantity:   qty,
            })
        }
    }
    return result
}

// max1 returns n if n >= 1, otherwise 1. Used to enforce minimum post-scale quantity.
func max1(n int) int {
    if n < 1 {
        return 1
    }
    return n
}
```

Add `"math"` to imports.

Make `GenerateLoot` delegate to `GenerateLootWithTier` with `lootMult=1.0`:
```go
func GenerateLoot(lt LootTable) LootResult {
    return GenerateLootWithTier(lt, 1.0)
}
```

- [ ] **Step 3: Update NPC YAML currency blocks**

Run: `grep -rl "currency:" /home/cjohannsen/src/mud/content/npcs/ --include="*.yaml"`

In each file using `min:` / `max:` under `currency:`, rename to `credits_min:` / `credits_max:`.

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/loot.go internal/game/npc/loot_test.go content/npcs/
git commit -m "feat(npc): add TierScale loot; GenerateLootWithTier; credits_min/credits_max YAML keys (REQ-AE-19, REQ-AE-20)"
```

---

## Task 11: Add BossRoom and HazardDef to world.Room (REQ-AE-23, REQ-AE-27)

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/model_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/world/model_test.go (add)
func TestHazardDef_Validate_ValidOnEnter(t *testing.T) {
    h := world.HazardDef{
        ID: "acid_pool", Trigger: "on_enter", DamageExpr: "1d6",
        DamageType: "acid", Message: "You step into acid!",
    }
    assert.NoError(t, h.Validate())
}

func TestHazardDef_Validate_ValidRoundStart(t *testing.T) {
    h := world.HazardDef{
        ID: "gas_cloud", Trigger: "round_start", DamageExpr: "2d4",
        DamageType: "poison", ConditionID: "poisoned",
        Message: "Poison gas burns your lungs.",
    }
    assert.NoError(t, h.Validate())
}

func TestHazardDef_Validate_InvalidTrigger(t *testing.T) {
    h := world.HazardDef{
        ID: "trap", Trigger: "on_death", DamageExpr: "1d6",
    }
    assert.Error(t, h.Validate())
}

func TestHazardDef_Validate_EmptyDamageExpr(t *testing.T) {
    h := world.HazardDef{ID: "trap", Trigger: "on_enter"}
    assert.Error(t, h.Validate())
}

func TestProperty_HazardDef_ValidateConsistency(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        trigger := rapid.SampledFrom([]string{"on_enter", "round_start", "on_attack", ""}).Draw(t, "trigger")
        dmgExpr := rapid.StringN(0, 10, -1).Draw(t, "dmg")

        h := world.HazardDef{ID: "h1", Trigger: trigger, DamageExpr: dmgExpr}
        valid := (trigger == "on_enter" || trigger == "round_start") && dmgExpr != ""
        if valid {
            assert.NoError(t, h.Validate())
        } else {
            assert.Error(t, h.Validate())
        }
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run TestHazardDef -v`
Expected: FAIL

- [ ] **Step 2: Add HazardDef and BossRoom to model.go**

In `internal/game/world/model.go`, before the `Room` struct, add:

```go
// HazardDef declares a room hazard that fires on player entry or combat round start.
type HazardDef struct {
    // ID is a unique identifier for this hazard within the room.
    ID string `yaml:"id"`
    // Trigger determines when the hazard fires.
    // Valid values: "on_enter" (player enters room), "round_start" (each combat round).
    Trigger string `yaml:"trigger"`
    // DamageExpr is a dice expression for damage (e.g. "2d6"). Must not be empty.
    DamageExpr string `yaml:"damage_expr"`
    // DamageType is an optional damage type label (e.g. "fire", "acid").
    DamageType string `yaml:"damage_type"`
    // ConditionID is an optional condition to apply to the player when the hazard fires.
    ConditionID string `yaml:"condition_id"`
    // Message is sent to each affected player via conn.WriteConsole.
    Message string `yaml:"message"`
}

// Validate checks that the hazard definition is valid.
//
// Precondition: none.
// Postcondition: Returns nil iff Trigger is valid and DamageExpr is non-empty.
func (h HazardDef) Validate() error {
    switch h.Trigger {
    case "on_enter", "round_start":
        // valid
    default:
        return fmt.Errorf("hazard %q: unknown trigger %q (must be \"on_enter\" or \"round_start\")", h.ID, h.Trigger)
    }
    if h.DamageExpr == "" {
        return fmt.Errorf("hazard %q: damage_expr must not be empty", h.ID)
    }
    return nil
}
```

In the `Room` struct, add two new fields after `Traps`:
```go
// BossRoom when true marks this room as a boss arena.
// Boss respawn triggers coordinated room respawn; map renderer uses <BB> tile.
BossRoom bool `yaml:"boss_room,omitempty"`
// Hazards lists environmental hazards that fire on room entry or combat rounds.
Hazards []HazardDef `yaml:"hazards,omitempty"`
```

- [ ] **Step 3: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -v`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/world/model.go internal/game/world/model_test.go
git commit -m "feat(world): add BossRoom, HazardDef to Room (REQ-AE-23, REQ-AE-27)"
```

---

## Task 12: Add boss_room to MapTile proto; update renderer (REQ-AE-24)

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/frontend/handlers/text_renderer_test.go (add)
func TestRenderMap_BossRoom_RendersAngleBrackets(t *testing.T) {
    resp := &gamev1.MapResponse{
        Tiles: []*gamev1.MapTile{
            {RoomId: "boss1", X: 0, Y: 0, Current: false, BossRoom: true},
        },
    }
    result := RenderMap(resp, 80)
    assert.Contains(t, result, "<", "boss room tile must use angle brackets")
    assert.NotContains(t, result, "[", "boss room tile must not use square brackets")
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestRenderMap_BossRoom -v`
Expected: FAIL (BossRoom field not in MapTile)

- [ ] **Step 2: Add boss_room to MapTile proto**

In `api/proto/game/v1/game.proto`, update the `MapTile` message:
```proto
message MapTile {
    string room_id   = 1;
    string room_name = 2;
    int32  x         = 3;
    int32  y         = 4;
    bool   current   = 5;
    repeated string exits = 6;
    string danger_level  = 7;
    // Field 8 is reserved for map-poi's repeated string pois = 8
    bool   boss_room     = 9;
}
```

- [ ] **Step 3: Regenerate protobuf**

Run: `cd /home/cjohannsen/src/mud && make proto` (or the existing proto generation target)
If no Makefile target exists: `cd /home/cjohannsen/src/mud && protoc --go_out=. --go-grpc_out=. api/proto/game/v1/game.proto`

- [ ] **Step 4: Update RenderMap in text_renderer.go**

In `internal/frontend/handlers/text_renderer.go`, find the room tile rendering inside `RenderMap` (around line 1107):

```go
// BEFORE
if t.Current {
    sb.WriteString(fmt.Sprintf("%s<%2d>%s", color, num, ansiReset))
} else {
    sb.WriteString(fmt.Sprintf("%s[%2d]%s", color, num, ansiReset))
}

// AFTER
switch {
case t.Current:
    sb.WriteString(fmt.Sprintf("%s<%2d>%s", color, num, ansiReset))
case t.BossRoom:
    sb.WriteString(fmt.Sprintf("%s<BB>%s", color, ansiReset))
default:
    sb.WriteString(fmt.Sprintf("%s[%2d]%s", color, num, ansiReset))
}
```

Note: Boss room tiles use `<BB>` and do not show the room number. This is consistent with the spec (REQ-AE-24).

- [ ] **Step 5: Update game_bridge.go or any place that builds MapTile**

Run: `grep -rn "MapTile{" /home/cjohannsen/src/mud/internal --include="*.go"`

Find where MapTile is constructed and propagate `BossRoom: room.BossRoom` when populating the tile from a `world.Room`.

- [ ] **Step 6: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat(map): add boss_room to MapTile proto; render boss rooms as <BB> (REQ-AE-24)"
```

---

## Task 13: Coordinated boss respawn (REQ-AE-25, REQ-AE-26)

**Files:**
- Modify: `internal/game/npc/respawn.go`
- Modify: `internal/game/npc/respawn_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/respawn_test.go (add)
func TestRespawnManager_BossRespawn_CoordinatedRespawn(t *testing.T) {
    // Setup: a boss room with two templates (boss + minion)
    tmplBoss := &npc.Template{ID: "boss_npc", Name: "Boss", Level: 5, MaxHP: 100, AC: 15, Tier: "boss", RespawnDelay: "1m"}
    tmplMinion := &npc.Template{ID: "minion_npc", Name: "Minion", Level: 1, MaxHP: 10, AC: 10, RespawnDelay: "5m"}

    templates := map[string]*npc.Template{
        "boss_npc":   tmplBoss,
        "minion_npc": tmplMinion,
    }
    spawns := map[string][]npc.RoomSpawn{
        "boss_room": {
            {TemplateID: "boss_npc", Max: 1, RespawnDelay: time.Minute},
            {TemplateID: "minion_npc", Max: 3, RespawnDelay: 5 * time.Minute},
        },
    }
    rm := npc.NewRespawnManager(spawns, templates)

    // Schedule pending minion respawn far in the future
    rm.Schedule("minion_npc", "boss_room", time.Now(), 10*time.Minute)
    pendingBefore := rm.PendingCount("boss_room")
    assert.Equal(t, 1, pendingBefore, "should have one pending minion")

    // Trigger coordinated boss respawn
    mgr := npc.NewManager()
    rm.CoordinatedBossRespawn("boss_room", mgr)

    // All pending timers for boss_room must be cancelled
    assert.Equal(t, 0, rm.PendingCount("boss_room"), "coordinated respawn must cancel pending timers")
}
```

Note: `PendingCount` and `CoordinatedBossRespawn` are new methods that must be added to `RespawnManager`. The test creates an `npc.Manager` with `npc.NewManager()` — verify this constructor exists, or use `npc.NewManager(nil)` etc. as appropriate.

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestRespawnManager_Boss -v`
Expected: FAIL

- [ ] **Step 2: Add CoordinatedBossRespawn and PendingCount to RespawnManager**

In `internal/game/npc/respawn.go`:

```go
// PendingCount returns the number of pending respawn entries for the given roomID.
// Used for testing. Safe for concurrent use.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns the count of pending entries whose roomID matches.
func (r *RespawnManager) PendingCount(roomID string) int {
    r.mu.RLock()
    defer r.mu.RUnlock()
    count := 0
    for _, e := range r.pending {
        if e.roomID == roomID {
            count++
        }
    }
    return count
}

// CoordinatedBossRespawn cancels all pending individual respawn timers for roomID
// and immediately triggers PopulateRoom for all spawn configs in that room.
// Called when a boss-tier NPC respawns in a boss_room.
//
// Precondition: roomID must be non-empty; mgr must not be nil.
// Postcondition: All pending timers for roomID are removed; all spawns repopulated.
func (r *RespawnManager) CoordinatedBossRespawn(roomID string, mgr *Manager) {
    r.mu.Lock()
    filtered := r.pending[:0]
    for _, e := range r.pending {
        if e.roomID != roomID {
            filtered = append(filtered, e)
        }
    }
    r.pending = filtered
    r.mu.Unlock()

    r.PopulateRoom(roomID, mgr)
}
```

- [ ] **Step 3: Wire CoordinatedBossRespawn into the respawn tick path**

In `RespawnManager.Tick` (or the existing respawn execution code), when a respawn fires for a template with `tier == "boss"` and the room has `BossRoom == true`, call `CoordinatedBossRespawn` instead of the normal `PopulateRoom`.

This requires `RespawnManager` to know which rooms are boss rooms. Add a `bossRooms map[string]bool` field to `RespawnManager` and a `NewRespawnManagerWithBossRooms` constructor (or extend `NewRespawnManager` to accept a `bossRooms` set).

Add to `RespawnManager`:
```go
bossRooms map[string]bool // set of roomIDs marked as boss_room
```

Add constructor:
```go
// NewRespawnManagerWithBossRooms creates a RespawnManager that knows which rooms are boss rooms.
//
// Precondition: spawns, templates, and bossRooms may be nil (treated as empty maps).
// Postcondition: Returns a non-nil RespawnManager.
func NewRespawnManagerWithBossRooms(spawns map[string][]RoomSpawn, templates map[string]*Template, bossRooms map[string]bool) *RespawnManager {
    if spawns == nil {
        spawns = make(map[string][]RoomSpawn)
    }
    if templates == nil {
        templates = make(map[string]*Template)
    }
    if bossRooms == nil {
        bossRooms = make(map[string]bool)
    }
    return &RespawnManager{
        spawns:    spawns,
        templates: templates,
        bossRooms: bossRooms,
        pending:   nil,
    }
}
```

In `Tick` (or wherever a respawn entry fires), add the boss-room check:
```go
// If the respawning template is a boss tier and the room is a boss room,
// use coordinated respawn instead of individual spawn.
if tmpl.Tier == "boss" && r.bossRooms[entry.roomID] {
    r.CoordinatedBossRespawn(entry.roomID, mgr)
    continue
}
```

Update callers of `NewRespawnManager` in the server wiring to pass the boss rooms set (derived from `world.Manager` rooms at startup).

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/respawn.go internal/game/npc/respawn_test.go
git commit -m "feat(respawn): coordinated boss room respawn on boss death (REQ-AE-25, REQ-AE-26)"
```

---

## Task 14: NPC passive feat bonuses in combat; boss abilities; room hazards (REQ-AE-16, REQ-AE-17, REQ-AE-22, REQ-AE-28–38)

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_boss_test.go`
- Modify: `internal/gameserver/grpc_service.go`

This is the largest task. Break it into named sub-steps.

- [ ] **Step 1: Write failing test for NPC feat bonuses**

```go
// internal/gameserver/combat_handler_boss_test.go (create)
package gameserver_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/gameserver"
)

func TestNPCEffectiveStats_BrutalStrike_AddsDamage(t *testing.T) {
    feats := []*ruleset.Feat{
        {ID: "brutal_strike", Name: "Brutal Strike", AllowNPC: true},
    }
    registry := ruleset.NewFeatRegistry(feats)
    inst := &npc.Instance{Feats: []string{"brutal_strike"}}

    stats := gameserver.ComputeNPCEffectiveStats(inst, registry, nil)
    assert.Equal(t, 2, stats.DamageBonus)
}

func TestNPCEffectiveStats_Evasive_AddsAC(t *testing.T) {
    feats := []*ruleset.Feat{
        {ID: "evasive", Name: "Evasive", AllowNPC: true},
    }
    registry := ruleset.NewFeatRegistry(feats)
    inst := &npc.Instance{AC: 14, Feats: []string{"evasive"}}

    stats := gameserver.ComputeNPCEffectiveStats(inst, registry, nil)
    assert.Equal(t, 2, stats.ACBonus)
}

func TestNPCEffectiveStats_PackTactics_AllyPresent(t *testing.T) {
    feats := []*ruleset.Feat{
        {ID: "pack_tactics", Name: "Pack Tactics", AllowNPC: true},
    }
    registry := ruleset.NewFeatRegistry(feats)
    ally := &npc.Instance{ID: "ally1", Feats: nil}
    attacker := &npc.Instance{ID: "attacker", Feats: []string{"pack_tactics"}}
    // roomNPCs includes the ally (different from attacker)
    roomNPCs := []*npc.Instance{attacker, ally}

    stats := gameserver.ComputeNPCEffectiveStats(attacker, registry, roomNPCs)
    assert.Equal(t, 2, stats.AttackBonus)
}

func TestNPCEffectiveStats_PackTactics_NoAlly(t *testing.T) {
    feats := []*ruleset.Feat{
        {ID: "pack_tactics", Name: "Pack Tactics", AllowNPC: true},
    }
    registry := ruleset.NewFeatRegistry(feats)
    attacker := &npc.Instance{ID: "solo", Feats: []string{"pack_tactics"}}
    roomNPCs := []*npc.Instance{attacker} // alone

    stats := gameserver.ComputeNPCEffectiveStats(attacker, registry, roomNPCs)
    assert.Equal(t, 0, stats.AttackBonus)
}

func TestProperty_NPCEffectiveStats_NeverNegativeWithValidFeats(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        featIDs := rapid.SliceOfDistinct(
            rapid.SampledFrom([]string{"brutal_strike", "evasive"}),
            func(s string) string { return s },
        ).Draw(t, "feats")

        feats := []*ruleset.Feat{
            {ID: "brutal_strike", AllowNPC: true},
            {ID: "evasive", AllowNPC: true},
        }
        registry := ruleset.NewFeatRegistry(feats)
        inst := &npc.Instance{Feats: featIDs, AC: 12}

        stats := gameserver.ComputeNPCEffectiveStats(inst, registry, nil)
        assert.GreaterOrEqual(t, stats.DamageBonus, 0)
        assert.GreaterOrEqual(t, stats.ACBonus, 0)
        assert.GreaterOrEqual(t, stats.AttackBonus, 0)
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestNPCEffectiveStats -v`
Expected: FAIL (ComputeNPCEffectiveStats not exported)

- [ ] **Step 2: Add NPCEffectiveStats and ComputeNPCEffectiveStats to combat_handler.go**

Add a new exported type and function (can be in a new file `internal/gameserver/npc_feat_bonuses.go`):

```go
// NPCEffectiveStats holds combat bonuses computed from an NPC's passive feats.
type NPCEffectiveStats struct {
    // AttackBonus is the bonus to attack rolls from passive feats.
    AttackBonus int
    // DamageBonus is the bonus to damage rolls from passive feats.
    DamageBonus int
    // ACBonus is the bonus to AC from passive feats.
    ACBonus int
}

// ComputeNPCEffectiveStats returns the combat stat bonuses from the NPC's passive feats.
// roomNPCs is the list of all NPC instances in the room (used for pack_tactics evaluation).
// Pass nil for roomNPCs when not in combat or when the list is unavailable.
//
// Precondition: inst must not be nil; registry must not be nil.
// Postcondition: Returns an NPCEffectiveStats with summed bonuses from all passive feats.
func ComputeNPCEffectiveStats(inst *npc.Instance, registry *ruleset.FeatRegistry, roomNPCs []*npc.Instance) NPCEffectiveStats {
    var stats NPCEffectiveStats
    for _, featID := range inst.Feats {
        f, ok := registry.Feat(featID)
        if !ok || !f.AllowNPC {
            continue
        }
        switch featID {
        case "brutal_strike":
            stats.DamageBonus += 2
        case "evasive":
            stats.ACBonus += 2
        case "pack_tactics":
            for _, ally := range roomNPCs {
                if ally.ID != inst.ID {
                    stats.AttackBonus += 2
                    break
                }
            }
        }
        // "tough" is applied at spawn, not at round resolution — skip here.
    }
    return stats
}
```

- [ ] **Step 3: Wire feat bonuses into NPC attack resolution**

In `combat_handler.go`, find the NPC attack roll and damage computation (search for the section that resolves NPC attack). Before computing the NPC attack roll, call:

```go
// Compute NPC feat bonuses for this attacker.
var npcStats gameserver.NPCEffectiveStats
if h.engine != nil && /* featRegistry available */ {
    roomInstances := h.npcMgr.InRoom(inst.RoomID)
    npcStats = ComputeNPCEffectiveStats(inst, featRegistry, roomInstances)
}
```

Add the `featRegistry *ruleset.FeatRegistry` field to `CombatHandler` and `NewCombatHandler`. Apply `npcStats.AttackBonus` to the attack roll modifier and `npcStats.DamageBonus` to the damage roll, and `npcStats.ACBonus` to the NPC's effective AC when being targeted.

- [ ] **Step 4: Write failing test for room hazard on_enter**

```go
// internal/gameserver/combat_handler_boss_test.go (add)
func TestApplyHazards_OnEnter_DealsDirectDamage(t *testing.T) {
    // This test verifies that on_enter hazards deal damage to the player session.
    // The test uses a stub that calls the internal hazard-apply helper directly.
    // Full integration test validates the hook fires on Move command.
    // See: TestApplyRoomHazards_OnEnter in grpc_service_test.go
    t.Skip("integration test in grpc_service_test.go")
}
```

The main logic is in `grpc_service.go`'s room entry handler. Add hazard evaluation there:

In `internal/gameserver/grpc_service.go`, find `applyRoomSkillChecks` (around line 2010). After the skill check evaluation and after Lua `on_enter` hooks, add:
```go
// Fire on_enter hazards for the new room (REQ-AE-28).
s.applyRoomHazards(ctx, newRoom, sess, conn, "on_enter")
```

Add the `applyRoomHazards` method:
```go
// applyRoomHazards fires all hazards matching the given trigger for the player.
// For "on_enter": fires when the player enters the room.
// For "round_start": called at the start of each combat round.
//
// Precondition: room and sess must not be nil; conn must not be nil.
// Postcondition: each matching hazard deals damage and/or applies a condition.
func (s *GameServiceServer) applyRoomHazards(
    ctx context.Context,
    room *world.Room,
    sess *session.PlayerSession,
    conn PlayerConn,
    trigger string,
) {
    for _, hazard := range room.Hazards {
        if hazard.Trigger != trigger {
            continue
        }
        dmg, err := s.dice.RollExpr(hazard.DamageExpr)
        if err != nil {
            if s.logger != nil {
                s.logger.Warn("hazard dice roll failed", zap.String("expr", hazard.DamageExpr), zap.Error(err))
            }
            continue
        }
        sess.CurrentHP -= dmg
        if hazard.Message != "" {
            conn.WriteConsole(hazard.Message + "\r\n")
        }
        if hazard.ConditionID != "" {
            if cond, ok := s.condRegistry.Get(hazard.ConditionID); ok {
                _ = sess.Conditions.Apply(sess.UID, cond, 1, -1)
            }
        }
    }
}
```

- [ ] **Step 5: Add round_start hazard firing in combat_handler.go**

In `CombatHandler`, find `StartRound` processing or the per-round tick. Add a call to apply `round_start` hazards for all players in the room via the session manager and `applyRoomHazards`. The combat handler already has access to `worldMgr`, `sessions`, and the player connection via the session manager.

Since `applyRoomHazards` lives in `grpc_service.go`, consider moving it to a shared helper in `internal/gameserver/hazards.go` so that `combat_handler.go` can also call it. Create `internal/gameserver/hazards.go`:

```go
package gameserver

import (
    "context"

    "go.uber.org/zap"

    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/world"
)

// PlayerConn is the interface for sending messages to a player connection.
// Defined here to avoid circular imports; implemented by the telnet frontend conn.
type PlayerConn interface {
    WriteConsole(msg string)
}

// ApplyHazards fires all hazards matching trigger for the given player session.
//
// Precondition: room, sess, conn must not be nil; trigger must be "on_enter" or "round_start".
// Postcondition: each matching hazard deals direct damage and/or applies a condition to sess.
func ApplyHazards(
    ctx context.Context,
    room *world.Room,
    sess *session.PlayerSession,
    conn PlayerConn,
    trigger string,
    diceRoller *dice.Roller,
    condRegistry *condition.Registry,
    logger *zap.Logger,
) {
    for _, hazard := range room.Hazards {
        if hazard.Trigger != trigger {
            continue
        }
        dmg, err := diceRoller.RollExpr(hazard.DamageExpr)
        if err != nil {
            if logger != nil {
                logger.Warn("hazard dice roll failed",
                    zap.String("hazard_id", hazard.ID),
                    zap.String("expr", hazard.DamageExpr),
                    zap.Error(err),
                )
            }
            continue
        }
        sess.CurrentHP -= dmg
        if hazard.Message != "" {
            conn.WriteConsole(hazard.Message + "\r\n")
        }
        if hazard.ConditionID != "" {
            if cond, ok := condRegistry.Get(hazard.ConditionID); ok {
                _ = sess.Conditions.Apply(sess.UID, cond, 1, -1)
            }
        }
    }
}
```

Then call `ApplyHazards` from both `grpc_service.go` (on_enter) and the combat round tick in `combat_handler.go` (round_start).

- [ ] **Step 6: Add boss kill bonus XP (REQ-AE-22)**

In `combat_handler.go`, in the NPC death handler section (around line 2860), after the normal per-participant XP award, add:

```go
// Award boss kill bonus XP to all players in the room (REQ-AE-22).
effectiveTier := inst.Tier
if effectiveTier == "" {
    effectiveTier = "standard"
}
if effectiveTier == "boss" && h.xpSvc != nil {
    bonusXP := h.xpSvc.Config().Awards.BossKillBonusXP
    if bonusXP > 0 {
        roomPlayers := h.sessions.InRoom(inst.RoomID)
        for _, p := range roomPlayers {
            xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), p, p.CharacterID, bonusXP)
            if xpErr == nil {
                h.pushXPMessages(p, xpMsgs, bonusXP, "boss kill bonus")
            } else if h.logger != nil {
                h.logger.Warn("boss bonus XP failed", zap.String("uid", p.UID), zap.Error(xpErr))
            }
        }
    }
}
```

Note: `h.sessions.InRoom` may not exist yet — check `session.Manager` for a method that returns all sessions in a room, and add one if needed.

- [ ] **Step 7: Add boss ability evaluation per round (REQ-AE-35–38)**

Add helper to `combat_handler.go`:

```go
// evaluateBossAbilities checks and fires all eligible boss abilities for inst.
// Called at the start of each combat round, before normal NPC attack resolution.
//
// Precondition: inst must not be nil; roomID must be non-empty.
// Postcondition: eligible abilities are fired; cooldowns recorded.
func (h *CombatHandler) evaluateBossAbilitiesLocked(
    ctx context.Context,
    inst *npc.Instance,
    roomID string,
    round int,
    tookDamageThisRound bool,
) {
    if len(inst.AbilityCooldowns) == 0 {
        // AbilityCooldowns must be non-nil (initialized at spawn); skip if nil template
    }
    tmpl := h.npcMgr.TemplateByID(inst.TemplateID)
    if tmpl == nil {
        return
    }
    now := time.Now()
    for _, ability := range tmpl.BossAbilities {
        // Check cooldown
        if deadline, hasCooldown := inst.AbilityCooldowns[ability.ID]; hasCooldown {
            if now.Before(deadline) {
                continue
            }
        }
        // Check trigger condition
        fires := false
        switch ability.Trigger {
        case "hp_pct_below":
            pct := 0
            if inst.MaxHP > 0 {
                pct = int(100 * inst.CurrentHP / inst.MaxHP)
            }
            fires = pct < ability.TriggerValue
        case "round_start":
            fires = ability.TriggerValue == 0 || round == ability.TriggerValue
        case "on_damage_taken":
            fires = tookDamageThisRound
        }
        if !fires {
            continue
        }

        // Announce
        msg := fmt.Sprintf("%s uses %s!", inst.Name(), ability.Name)
        h.broadcastFn(roomID, []*gamev1.CombatEvent{{
            Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_MESSAGE,
            Narrative: msg,
        }})

        // Execute effect
        switch {
        case ability.Effect.AoeDamageExpr != "":
            dmg, err := h.dice.RollExpr(ability.Effect.AoeDamageExpr)
            if err == nil {
                // Apply same damage to all players in room — not mitigated by AC (REQ-AE-37)
                players := h.sessions.PlayersInRoomDetails(roomID)
                for _, p := range players {
                    p.CurrentHP -= dmg
                }
            }
        case ability.Effect.AoeCondition != "":
            players := h.sessions.PlayersInRoomDetails(roomID)
            for _, p := range players {
                if cond, ok := h.condRegistry.Get(ability.Effect.AoeCondition); ok {
                    _ = p.Conditions.Apply(p.UID, cond, 1, -1)
                }
            }
        case ability.Effect.HealPct != 0:
            heal := int(math.Ceil(float64(inst.MaxHP) * float64(ability.Effect.HealPct) / 100.0))
            inst.CurrentHP += heal
            if inst.CurrentHP > inst.MaxHP {
                inst.CurrentHP = inst.MaxHP
            }
        }

        // Record cooldown
        if ability.Cooldown != "" {
            if dur, err := time.ParseDuration(ability.Cooldown); err == nil {
                inst.AbilityCooldowns[ability.ID] = now.Add(dur)
            }
        }
    }
}
```

Call `evaluateBossAbilitiesLocked` at the start of each NPC's round, before normal attack resolution. Find the per-NPC round processing in `combat_handler.go` and add the call.

- [ ] **Step 8: Run all tests**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1`
Expected: all PASS (or only pre-existing failures)

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_boss_test.go internal/gameserver/hazards.go internal/gameserver/npc_feat_bonuses.go internal/gameserver/grpc_service.go
git commit -m "feat(combat): NPC feat bonuses; boss abilities; room hazards; boss kill bonus XP (REQ-AE-16–22, REQ-AE-28–38)"
```

---

## Task 15: Final wiring — ValidateWithRegistry at startup; run full test suite (REQ-AE-4, REQ-AE-14)

**Files:**
- Modify: server startup (likely `cmd/gameserver/main.go` or the wire-generated `Initialize()`)
- Modify: `internal/gameserver/grpc_service.go` (if startup validation lives there)

- [ ] **Step 1: Find startup validation location**

Run: `grep -rn "LoadTemplates\|ValidateTiers\|ValidateWithSkills" /home/cjohannsen/src/mud/internal --include="*.go" | head -20`

Identify where NPC templates are loaded and validated at startup.

- [ ] **Step 2: Add ValidateTiers to startup**

After loading `XPConfig`, add:
```go
if err := xpCfg.ValidateTiers(); err != nil {
    log.Fatalf("fatal: %v", err)
}
```

- [ ] **Step 3: Add ValidateWithRegistry for all templates at startup**

After loading the feat registry and NPC templates, add:
```go
for _, tmpl := range templates {
    if err := tmpl.ValidateWithRegistry(featRegistry); err != nil {
        log.Fatalf("fatal: NPC template validation failed: %v", err)
    }
}
```

- [ ] **Step 4: Pass XPConfig and FeatRegistry through to RespawnManager and SpawnInstance**

Ensure `NewRespawnManagerWithBossRooms` is called with the boss rooms set (derived from loaded world rooms: any room with `BossRoom == true`).

Ensure `NewInstanceWithResolver` is called with the real `xpCfg` and `featRegistry` everywhere instances are spawned (in `Manager.Spawn` or wherever templates are instantiated).

- [ ] **Step 5: Run full test suite**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1 -timeout 5m`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add .
git commit -m "feat(startup): ValidateTiers and ValidateWithRegistry at startup; full wiring (REQ-AE-4, REQ-AE-14)"
```

---

## Quick Reference: Requirement → Task Mapping

| REQ | Task |
|-----|------|
| REQ-AE-1, REQ-AE-2 | Task 3 |
| REQ-AE-3, REQ-AE-4 | Task 2 |
| REQ-AE-5, REQ-AE-18 | Task 8 |
| REQ-AE-6 | Task 9 |
| REQ-AE-7, REQ-AE-8, REQ-AE-9, REQ-AE-10 | Task 4 |
| REQ-AE-11, REQ-AE-12 | Task 5 |
| REQ-AE-13, REQ-AE-14, REQ-AE-15 | Task 6 |
| REQ-AE-16, REQ-AE-17 | Task 14 |
| REQ-AE-19, REQ-AE-20 | Task 10 |
| REQ-AE-21, REQ-AE-22 | Tasks 2 and 14 |
| REQ-AE-23 | Task 11 |
| REQ-AE-24 | Task 12 |
| REQ-AE-25, REQ-AE-26 | Task 13 |
| REQ-AE-27 | Task 11 |
| REQ-AE-28, REQ-AE-29, REQ-AE-30 | Task 14 |
| REQ-AE-31, REQ-AE-32, REQ-AE-33, REQ-AE-34 | Task 7 |
| REQ-AE-35, REQ-AE-36, REQ-AE-37, REQ-AE-38 | Task 14 |
| REQ-AE-39 | Task 1 |
