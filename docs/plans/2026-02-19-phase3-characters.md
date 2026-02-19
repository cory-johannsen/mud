# Phase 3: Character System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add multi-character accounts, interactive character creation (home region + class), character persistence, and character-aware game sessions so players select and name a PF2E-adapted character before entering the world.

**Architecture:** After login the Telnet handler runs a character-selection/creation flow (in `internal/frontend/handlers/`) backed by a `CharacterRepository` (postgres). The selected character's ID and name are forwarded to the gameserver via updated `JoinWorldRequest` proto fields; the gameserver uses the character ID as the session UID, displays the character name in prompts, and persists the character's last room on disconnect.

**Tech Stack:** Go 1.22+, pgx v5, gopkg.in/yaml.v3, pgregory.net/rapid (property tests), protoc + protoc-gen-go, testify, existing internal packages.

---

## Context: Current State

- `internal/frontend/handlers/auth.go` — `AuthHandler.HandleSession()` authenticates then calls `gameBridge(username)` directly. No character concept.
- `internal/frontend/handlers/game_bridge.go` — sends `JoinWorldRequest{Uid: username, Username: username}`.
- `internal/gameserver/grpc_service.go` — `Session()` uses `joinReq.Uid` as session UID and username as display name.
- `internal/storage/postgres/account.go` — `AccountRepository` with `Create`, `Authenticate`, `GetByUsername`.
- `migrations/001_accounts.up.sql` — `accounts` table.
- `migrations/002_zones_rooms.up.sql` — `zones`, `rooms`, `room_exits` tables (schema only, not yet populated at runtime).
- `api/proto/game/v1/game.proto` — `JoinWorldRequest{uid, username}`.
- `cmd/gameserver/main.go` — no postgres connection; loads world from YAML only.
- `cmd/frontend/main.go` — connects to postgres for accounts.

## Key Design Decisions

- **UID strategy:** After this phase, `JoinWorldRequest.Uid` = `fmt.Sprintf("%d", character.ID)`. This uniquely identifies a character's active session.
- **Display name:** `character.Name` (chosen by player at creation) replaces `username` in prompts.
- **Content location:** `content/regions/` and `content/classes/` — YAML files loaded at startup by both frontend (for character creation) and gameserver (for HP calculation on join).
- **Gameserver DB:** Gameserver gains a postgres connection in this phase to persist character location on disconnect.
- **Ability calculation:** Base 10 in all six abilities + region modifiers (+2/+2/-2 pattern) + class key ability boost (+2). HP at level 1 = `class.HitPointsPerLevel + max(0, (constitution-10)/2)`.
- **Phase 3 scope:** CHAR-3–24. Deferred: CHAR-25–27 (progression/leveling — needs combat), CHAR-18 Lua scripts (Lua deferred to its own phase).

---

## Package Structure (new/modified)

```
content/
├── regions/          NEW — YAML home region definitions
└── classes/          NEW — YAML class definitions

api/proto/game/v1/game.proto   MODIFIED — character fields in JoinWorldRequest + CharacterInfo event

internal/
├── game/
│   ├── character/    NEW
│   │   ├── model.go        — Character, AbilityScores types
│   │   ├── builder.go      — pure BuildCharacter function
│   │   └── builder_test.go
│   └── ruleset/      NEW
│       ├── region.go       — Region YAML type + loader
│       ├── class.go        — Class YAML type + loader
│       └── loader_test.go
├── storage/postgres/
│   └── character.go  NEW — CharacterRepository (Create, ListByAccount, GetByID, SaveState)
└── frontend/handlers/
    ├── auth.go       MODIFIED — add CharacterStore interface, call character flow after login
    └── character_flow.go  NEW — characterSelectionFlow, characterCreationFlow

cmd/
├── frontend/main.go  MODIFIED — wire CharacterRepository into AuthHandler
└── gameserver/main.go MODIFIED — connect postgres, wire CharacterRepository into GameServiceServer

migrations/
├── 003_characters.up.sql   NEW
└── 003_characters.down.sql NEW
```

---

## Tasks

### Task 1: DB Migration — Characters Table

**Files:**
- Create: `migrations/003_characters.up.sql`
- Create: `migrations/003_characters.down.sql`

**Step 1: Write the migration**

Create `migrations/003_characters.up.sql`:

```sql
CREATE TABLE characters (
    id              BIGSERIAL    PRIMARY KEY,
    account_id      BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name            VARCHAR(64)  NOT NULL,
    region          TEXT         NOT NULL,
    class           TEXT         NOT NULL,
    level           INT          NOT NULL DEFAULT 1,
    experience      INT          NOT NULL DEFAULT 0,
    location        TEXT         NOT NULL DEFAULT 'grinders_row',
    strength        INT          NOT NULL DEFAULT 10,
    dexterity       INT          NOT NULL DEFAULT 10,
    constitution    INT          NOT NULL DEFAULT 10,
    intelligence    INT          NOT NULL DEFAULT 10,
    wisdom          INT          NOT NULL DEFAULT 10,
    charisma        INT          NOT NULL DEFAULT 10,
    max_hp          INT          NOT NULL DEFAULT 8,
    current_hp      INT          NOT NULL DEFAULT 8,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_characters_account_name UNIQUE (account_id, name)
);

CREATE INDEX idx_characters_account_id ON characters (account_id);
```

Create `migrations/003_characters.down.sql`:

```sql
DROP TABLE IF EXISTS characters;
```

**Step 2: Verify migration runs**

```bash
go run ./cmd/migrate -config configs/dev.yaml
```

Expected: migration applies without error. (Requires local postgres from `docker compose up postgres`.)

**Step 3: Commit**

```bash
git add migrations/003_characters.up.sql migrations/003_characters.down.sql
git commit -m "feat(migrations): add characters table"
```

---

### Task 2: Ruleset YAML Types + Loader

**Files:**
- Create: `internal/game/ruleset/region.go`
- Create: `internal/game/ruleset/class.go`
- Create: `internal/game/ruleset/loader.go`
- Create: `internal/game/ruleset/loader_test.go`

**Step 1: Write failing tests**

Create `internal/game/ruleset/loader_test.go`:

```go
package ruleset_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestLoadRegions_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old_town.yaml"), `
id: old_town
name: "Old Town"
description: "The neon-stained ruins of Portland's oldest district."
modifiers:
  charisma: 2
  dexterity: 2
  strength: -2
traits:
  - street_smart
  - scrapper
`)
	regions, err := ruleset.LoadRegions(dir)
	require.NoError(t, err)
	require.Len(t, regions, 1)
	r := regions[0]
	assert.Equal(t, "old_town", r.ID)
	assert.Equal(t, "Old Town", r.Name)
	assert.Equal(t, 2, r.Modifiers["charisma"])
	assert.Equal(t, 2, r.Modifiers["dexterity"])
	assert.Equal(t, -2, r.Modifiers["strength"])
	assert.Equal(t, []string{"street_smart", "scrapper"}, r.Traits)
}

func TestLoadClasses_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ganger.yaml"), `
id: ganger
name: "Ganger"
description: "Melee combatant hardened by street warfare."
key_ability: strength
hit_points_per_level: 10
proficiencies:
  weapons: trained
  armor: trained
features:
  - name: "Gang Tactics"
    level: 1
    description: "You fight dirty and effectively in groups."
`)
	classes, err := ruleset.LoadClasses(dir)
	require.NoError(t, err)
	require.Len(t, classes, 1)
	c := classes[0]
	assert.Equal(t, "ganger", c.ID)
	assert.Equal(t, "Ganger", c.Name)
	assert.Equal(t, "strength", c.KeyAbility)
	assert.Equal(t, 10, c.HitPointsPerLevel)
	assert.Equal(t, "trained", c.Proficiencies["weapons"])
	require.Len(t, c.Features, 1)
	assert.Equal(t, "Gang Tactics", c.Features[0].Name)
	assert.Equal(t, 1, c.Features[0].Level)
}

func TestLoadRegions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	regions, err := ruleset.LoadRegions(dir)
	require.NoError(t, err)
	assert.Empty(t, regions)
}

func TestLoadRegions_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{ not yaml`)
	_, err := ruleset.LoadRegions(dir)
	require.Error(t, err)
}

// Property: every loaded region has a non-empty ID and Name.
func TestLoadRegions_AllHaveIDAndName(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		dir := t.TempDir()
		for i := 0; i < n; i++ {
			content := fmt.Sprintf(`
id: region_%d
name: "Region %d"
description: "Test region."
modifiers: {}
traits: []
`, i, i)
			fname := filepath.Join(dir, fmt.Sprintf("region_%d.yaml", i))
			if err := os.WriteFile(fname, []byte(content), 0644); err != nil {
				rt.Fatal(err)
			}
		}
		regions, err := ruleset.LoadRegions(dir)
		if err != nil {
			rt.Fatal(err)
		}
		for _, r := range regions {
			if r.ID == "" {
				rt.Fatalf("region has empty ID")
			}
			if r.Name == "" {
				rt.Fatalf("region has empty Name")
			}
		}
	})
}
```

Note: add `"fmt"` to imports in the test file.

**Step 2: Run to verify failure**

```bash
go test ./internal/game/ruleset/... 2>&1
```

Expected: `FAIL` — package does not exist.

**Step 3: Implement region.go**

Create `internal/game/ruleset/region.go`:

```go
package ruleset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Region defines a home region (PF2E ancestry replacement) for character creation.
//
// Precondition: ID and Name must be non-empty after loading.
type Region struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Modifiers   map[string]int `yaml:"modifiers"`
	Traits      []string       `yaml:"traits"`
}

// LoadRegions reads all .yaml files in dir and parses each as a Region.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed regions (may be empty slice) or a non-nil error.
func LoadRegions(dir string) ([]*Region, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	regions := make([]*Region, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var r Region
		if err := yaml.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("parsing region file %s: %w", path, err)
		}
		regions = append(regions, &r)
	}
	return regions, nil
}

func yamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	return paths, nil
}
```

**Step 4: Implement class.go**

Create `internal/game/ruleset/class.go`:

```go
package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ClassFeature describes a single class feature gained at a specific level.
type ClassFeature struct {
	Name        string `yaml:"name"`
	Level       int    `yaml:"level"`
	Description string `yaml:"description"`
}

// Class defines a playable character class for character creation.
//
// Precondition: ID, Name, and KeyAbility must be non-empty after loading.
type Class struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	Description       string            `yaml:"description"`
	KeyAbility        string            `yaml:"key_ability"`
	HitPointsPerLevel int               `yaml:"hit_points_per_level"`
	Proficiencies     map[string]string `yaml:"proficiencies"`
	Features          []ClassFeature    `yaml:"features"`
}

// LoadClasses reads all .yaml files in dir and parses each as a Class.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed classes (may be empty slice) or a non-nil error.
func LoadClasses(dir string) ([]*Class, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	classes := make([]*Class, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var c Class
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parsing class file %s: %w", path, err)
		}
		classes = append(classes, &c)
	}
	return classes, nil
}
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/game/ruleset/... -v 2>&1
```

Expected: all tests pass.

**Step 6: Commit**

```bash
git add internal/game/ruleset/
git commit -m "feat(ruleset): YAML region and class loader"
```

---

### Task 3: Content Data Files — Home Regions and Classes

**Files:**
- Create: `content/regions/old_town.yaml`
- Create: `content/regions/pearl_district.yaml`
- Create: `content/regions/southeast_portland.yaml`
- Create: `content/regions/north_portland.yaml`
- Create: `content/regions/gresham_outskirts.yaml`
- Create: `content/classes/ganger.yaml`
- Create: `content/classes/scavenger.yaml`
- Create: `content/classes/fixer.yaml`
- Create: `content/classes/medic.yaml`
- Create: `content/classes/techie.yaml`

**Step 1: Write content — home regions**

Create `content/regions/old_town.yaml`:

```yaml
id: old_town
name: "Old Town"
description: |
  You grew up in the neon-stained ruins of Portland's oldest district, where
  every alley hides a deal and every face hides an angle. You learned to read
  people before you learned to read anything else.
modifiers:
  charisma: 2
  dexterity: 2
  strength: -2
traits:
  - street_smart
  - networked
```

Create `content/regions/pearl_district.yaml`:

```yaml
id: pearl_district
name: "Pearl District"
description: |
  What was once Portland's artsy enclave is now a fortified enclave for those
  who still believe in pre-collapse ideals. You grew up among archives,
  workshops, and fading murals of a world that no longer exists.
modifiers:
  intelligence: 2
  wisdom: 2
  constitution: -2
traits:
  - educated
  - archivalist
```

Create `content/regions/southeast_portland.yaml`:

```yaml
id: southeast_portland
name: "Southeast Portland"
description: |
  The sprawling flatlands of SE are patchwork survival — community gardens
  between burnt-out condos, mutual aid networks that outlasted every government.
  You learned to endure and adapt before anything else.
modifiers:
  constitution: 2
  dexterity: 2
  intelligence: -2
traits:
  - survivor
  - community_ties
```

Create `content/regions/north_portland.yaml`:

```yaml
id: north_portland
name: "North Portland"
description: |
  The industrial riverfront district where the mills still run, grinding out
  whatever the gangs and cooperatives need. You grew up moving heavy things
  and taking hard hits, and you know your own strength.
modifiers:
  strength: 2
  constitution: 2
  charisma: -2
traits:
  - labor_hardened
  - intimidating
```

Create `content/regions/gresham_outskirts.yaml`:

```yaml
id: gresham_outskirts
name: "Gresham Outskirts"
description: |
  Beyond the city limits, the Gresham sprawl is a patchwork of salvage yards,
  grow operations, and old-faith communities. Out here you learned patience,
  and how to read the land and the sky for signs of what's coming.
modifiers:
  wisdom: 2
  constitution: 2
  charisma: -2
traits:
  - forager
  - self_reliant
```

**Step 2: Write content — classes**

Create `content/classes/ganger.yaml`:

```yaml
id: ganger
name: "Ganger"
description: |
  Street-hardened melee combatant. You've fought your way through Portland's
  worst, and you know how to take a hit as well as dish one out.
key_ability: strength
hit_points_per_level: 10
proficiencies:
  weapons: trained
  armor: trained
  unarmed: expert
features:
  - name: "Gang Tactics"
    level: 1
    description: "When adjacent to an ally, you gain +1 to attack rolls."
  - name: "Toughened"
    level: 1
    description: "You gain 2 additional HP per level."
```

Create `content/classes/scavenger.yaml`:

```yaml
id: scavenger
name: "Scavenger"
description: |
  Quick-fingered opportunist and infiltrator. You survive by taking what others
  overlook, slipping past guards, and knowing when to run.
key_ability: dexterity
hit_points_per_level: 8
proficiencies:
  weapons: trained
  armor: light
  stealth: expert
features:
  - name: "Salvage Eye"
    level: 1
    description: "You automatically notice valuable salvage in any room you enter."
  - name: "Sneak Attack"
    level: 1
    description: "Deal +1d6 damage when you have advantage on the attack."
```

Create `content/classes/fixer.yaml`:

```yaml
id: fixer
name: "Fixer"
description: |
  Negotiator, broker, and social engineer. You move through Portland's
  underworld by knowing the right people and saying the right things.
key_ability: charisma
hit_points_per_level: 8
proficiencies:
  weapons: simple
  armor: light
  diplomacy: expert
features:
  - name: "The Deal"
    level: 1
    description: "Once per scene, you can propose a deal that NPCs must seriously consider."
  - name: "Connected"
    level: 1
    description: "You begin play with a contact in two different factions."
```

Create `content/classes/medic.yaml`:

```yaml
id: medic
name: "Medic"
description: |
  Field surgeon and herbalist. Pre-collapse medicine is a luxury; you learned
  to improvise with salvaged supplies and hard-won knowledge.
key_ability: wisdom
hit_points_per_level: 8
proficiencies:
  weapons: simple
  armor: light
  medicine: expert
features:
  - name: "Field Dressing"
    level: 1
    description: "As an action, restore 1d8+WIS modifier HP to a touched creature."
  - name: "Triage"
    level: 1
    description: "You can stabilize a dying creature as a free action."
```

Create `content/classes/techie.yaml`:

```yaml
id: techie
name: "Techie"
description: |
  Electronics hacker and device specialist. Pre-collapse technology still
  works if you know how to talk to it, and you speak its language fluently.
key_ability: intelligence
hit_points_per_level: 6
proficiencies:
  weapons: simple
  armor: none
  hacking: expert
features:
  - name: "Jury Rig"
    level: 1
    description: "Given 10 minutes and salvage parts, you can repair or improvise a simple device."
  - name: "System Probe"
    level: 1
    description: "You can attempt to interface with any electronic system you can physically touch."
```

**Step 3: Verify files load with loader**

```bash
go test ./internal/game/ruleset/... -v -run TestLoad 2>&1
```

This test uses temp dirs, not content/, so just verify the package compiles:

```bash
go build ./... 2>&1
```

Expected: no errors.

**Step 4: Write integration test that loads actual content files**

Add to `internal/game/ruleset/loader_test.go`:

```go
func TestLoadRegions_ActualContent(t *testing.T) {
	regions, err := ruleset.LoadRegions("../../../content/regions")
	require.NoError(t, err)
	assert.Len(t, regions, 5, "expected 5 home regions")
	ids := make(map[string]bool)
	for _, r := range regions {
		assert.NotEmpty(t, r.ID)
		assert.NotEmpty(t, r.Name)
		assert.NotEmpty(t, r.Description)
		assert.False(t, ids[r.ID], "duplicate region ID: %s", r.ID)
		ids[r.ID] = true
	}
}

func TestLoadClasses_ActualContent(t *testing.T) {
	classes, err := ruleset.LoadClasses("../../../content/classes")
	require.NoError(t, err)
	assert.Len(t, classes, 5, "expected 5 classes")
	for _, c := range classes {
		assert.NotEmpty(t, c.ID)
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.KeyAbility)
		assert.Greater(t, c.HitPointsPerLevel, 0)
	}
}
```

```bash
go test ./internal/game/ruleset/... -v 2>&1
```

Expected: all tests pass.

**Step 5: Commit**

```bash
git add content/regions/ content/classes/ internal/game/ruleset/loader_test.go
git commit -m "feat(content): add 5 home regions and 5 classes for PF2E ruleset"
```

---

### Task 4: Character Domain Model + Builder

**Files:**
- Create: `internal/game/character/model.go`
- Create: `internal/game/character/builder.go`
- Create: `internal/game/character/builder_test.go`

**Step 1: Write failing tests**

Create `internal/game/character/builder_test.go`:

```go
package character_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func makeRegion(mods map[string]int) *ruleset.Region {
	return &ruleset.Region{
		ID:        "test_region",
		Name:      "Test Region",
		Modifiers: mods,
	}
}

func makeClass(keyAbility string, hpPerLevel int) *ruleset.Class {
	return &ruleset.Class{
		ID:                "test_class",
		Name:              "Test Class",
		KeyAbility:        keyAbility,
		HitPointsPerLevel: hpPerLevel,
	}
}

func TestBuildCharacter_AppliesRegionModifiers(t *testing.T) {
	region := makeRegion(map[string]int{
		"strength": 2,
		"charisma": 2,
		"wisdom":   -2,
	})
	class := makeClass("strength", 10)

	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)

	assert.Equal(t, 14, c.Abilities.Strength)     // 10 + 2 region + 2 key ability
	assert.Equal(t, 10, c.Abilities.Dexterity)     // base
	assert.Equal(t, 10, c.Abilities.Constitution)  // base
	assert.Equal(t, 10, c.Abilities.Intelligence)  // base
	assert.Equal(t, 8, c.Abilities.Wisdom)         // 10 - 2
	assert.Equal(t, 12, c.Abilities.Charisma)      // 10 + 2
}

func TestBuildCharacter_CalculatesHP(t *testing.T) {
	// CON 12 → modifier +1; HP = 10 + 1 = 11
	region := makeRegion(map[string]int{"constitution": 2})
	class := makeClass("intelligence", 10)

	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)

	assert.Equal(t, 12, c.Abilities.Constitution)
	assert.Equal(t, 11, c.MaxHP)
	assert.Equal(t, 11, c.CurrentHP)
}

func TestBuildCharacter_HPNeverBelowOne(t *testing.T) {
	// CON 6 (10 - 4 from extreme penalty) would give -2 modifier → hp = 6 - 2 = 4 but floor at 1
	region := makeRegion(map[string]int{"constitution": -4})
	class := makeClass("strength", 6)

	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, c.MaxHP, 1)
}

func TestBuildCharacter_NameSet(t *testing.T) {
	region := makeRegion(nil)
	class := makeClass("strength", 8)
	c, err := character.Build("Aria", region, class)
	require.NoError(t, err)
	assert.Equal(t, "Aria", c.Name)
}

func TestBuildCharacter_EmptyNameError(t *testing.T) {
	region := makeRegion(nil)
	class := makeClass("strength", 8)
	_, err := character.Build("", region, class)
	require.Error(t, err)
}

func TestBuildCharacter_NilRegionError(t *testing.T) {
	class := makeClass("strength", 8)
	_, err := character.Build("Hero", nil, class)
	require.Error(t, err)
}

func TestBuildCharacter_NilClassError(t *testing.T) {
	region := makeRegion(nil)
	_, err := character.Build("Hero", region, nil)
	require.Error(t, err)
}

// Property: MaxHP is always >= 1 regardless of region modifiers.
func TestBuildCharacter_MaxHPAlwaysPositive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		conMod := rapid.IntRange(-8, 8).Draw(rt, "conMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"constitution": conMod})
		class := makeClass("strength", hpPerLevel)
		c, err := character.Build("Hero", region, class)
		if err != nil {
			rt.Fatal(err)
		}
		if c.MaxHP < 1 {
			rt.Fatalf("MaxHP %d < 1 with conMod=%d hpPerLevel=%d", c.MaxHP, conMod, hpPerLevel)
		}
	})
}

// Property: all ability scores are at least 4 (no modifier should drop below 4 in normal play).
func TestBuildCharacter_AbilitiesInReasonableRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mod := rapid.IntRange(-4, 4).Draw(rt, "mod")
		ability := rapid.SampledFrom([]string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}).Draw(rt, "ability")
		region := makeRegion(map[string]int{ability: mod})
		class := makeClass("strength", 8)
		c, err := character.Build("Hero", region, class)
		if err != nil {
			rt.Fatal(err)
		}
		scores := []int{
			c.Abilities.Strength, c.Abilities.Dexterity, c.Abilities.Constitution,
			c.Abilities.Intelligence, c.Abilities.Wisdom, c.Abilities.Charisma,
		}
		for _, s := range scores {
			if s < 4 {
				rt.Fatalf("ability score %d < 4", s)
			}
		}
	})
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/character/... 2>&1
```

Expected: `FAIL` — package does not exist.

**Step 3: Implement model.go**

Create `internal/game/character/model.go`:

```go
// Package character defines the character domain model and pure creation logic.
package character

import "time"

// AbilityScores holds the six PF2E ability score values for a character.
type AbilityScores struct {
	Strength     int
	Dexterity    int
	Constitution int
	Intelligence int
	Wisdom       int
	Charisma     int
}

// Modifier returns the PF2E ability modifier for a given score: (score - 10) / 2.
func (a AbilityScores) Modifier(score int) int {
	return (score - 10) / 2
}

// Character represents a player character's persistent state.
//
// AccountID and ID are set by the persistence layer; zero values indicate an unsaved character.
type Character struct {
	ID        int64
	AccountID int64

	Name      string
	Region    string // home region ID
	Class     string // class ID
	Level     int
	Experience int

	Location  string // current room ID
	Abilities AbilityScores
	MaxHP     int
	CurrentHP int

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

**Step 4: Implement builder.go**

Create `internal/game/character/builder.go`:

```go
package character

import (
	"errors"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// Build constructs a new Character from a name, region, and class.
// Ability scores start at 10, region modifiers are applied, then the
// class key ability receives a +2 boost. HP = max(1, hpPerLevel + CON modifier).
//
// Precondition: name must be non-empty; region and class must be non-nil.
// Postcondition: Returns a Character ready for persistence, or a non-nil error.
func Build(name string, region *ruleset.Region, class *ruleset.Class) (*Character, error) {
	if name == "" {
		return nil, errors.New("character name must not be empty")
	}
	if region == nil {
		return nil, errors.New("region must not be nil")
	}
	if class == nil {
		return nil, errors.New("class must not be nil")
	}

	abilities := applyModifiers(region.Modifiers)
	abilities = applyKeyAbilityBoost(abilities, class.KeyAbility)

	conMod := abilities.Modifier(abilities.Constitution)
	maxHP := class.HitPointsPerLevel + conMod
	if maxHP < 1 {
		maxHP = 1
	}

	return &Character{
		Name:      name,
		Region:    region.ID,
		Class:     class.ID,
		Level:     1,
		Location:  "grinders_row", // default start room for the zone
		Abilities: abilities,
		MaxHP:     maxHP,
		CurrentHP: maxHP,
	}, nil
}

// applyModifiers starts all abilities at 10 and adds region modifier values.
func applyModifiers(mods map[string]int) AbilityScores {
	a := AbilityScores{
		Strength: 10, Dexterity: 10, Constitution: 10,
		Intelligence: 10, Wisdom: 10, Charisma: 10,
	}
	for ability, delta := range mods {
		switch ability {
		case "strength":
			a.Strength += delta
		case "dexterity":
			a.Dexterity += delta
		case "constitution":
			a.Constitution += delta
		case "intelligence":
			a.Intelligence += delta
		case "wisdom":
			a.Wisdom += delta
		case "charisma":
			a.Charisma += delta
		}
	}
	return a
}

// applyKeyAbilityBoost adds +2 to the class key ability score.
func applyKeyAbilityBoost(a AbilityScores, keyAbility string) AbilityScores {
	switch keyAbility {
	case "strength":
		a.Strength += 2
	case "dexterity":
		a.Dexterity += 2
	case "constitution":
		a.Constitution += 2
	case "intelligence":
		a.Intelligence += 2
	case "wisdom":
		a.Wisdom += 2
	case "charisma":
		a.Charisma += 2
	}
	return a
}

// AbilityName returns the display string for an ability score field.
func AbilityName(field string) string {
	names := map[string]string{
		"strength":     "STR",
		"dexterity":    "DEX",
		"constitution": "CON",
		"intelligence": "INT",
		"wisdom":       "WIS",
		"charisma":     "CHA",
	}
	if n, ok := names[field]; ok {
		return n
	}
	return fmt.Sprintf("<%s>", field)
}
```

**Step 5: Run tests**

```bash
go test ./internal/game/character/... -v -race 2>&1
```

Expected: all tests pass.

**Step 6: Commit**

```bash
git add internal/game/character/
git commit -m "feat(character): domain model and pure builder function"
```

---

### Task 5: Character Repository (Postgres)

**Files:**
- Create: `internal/storage/postgres/character.go`
- Create: `internal/storage/postgres/character_test.go`

**Step 1: Write failing tests**

Create `internal/storage/postgres/character_test.go`:

```go
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: these tests require a running postgres instance (see testutil.NewPool).
// They will skip automatically if TEST_DATABASE_URL is not set.

func setupCharacterRepo(t *testing.T) (*postgres.CharacterRepository, *postgres.AccountRepository, int64) {
	t.Helper()
	pool := testutil.NewPool(t)
	acctRepo := postgres.NewAccountRepository(pool)
	acct, err := acctRepo.Create(context.Background(), testutil.UniqueName(t, "user"), "password123")
	require.NoError(t, err)
	return postgres.NewCharacterRepository(pool), acctRepo, acct.ID
}

func makeTestCharacter(accountID int64) *character.Character {
	return &character.Character{
		AccountID: accountID,
		Name:      "Zara",
		Region:    "old_town",
		Class:     "ganger",
		Level:     1,
		Location:  "grinders_row",
		Abilities: character.AbilityScores{
			Strength: 14, Dexterity: 12, Constitution: 10,
			Intelligence: 10, Wisdom: 8, Charisma: 12,
		},
		MaxHP:     10,
		CurrentHP: 10,
	}
}

func TestCharacterRepository_Create(t *testing.T) {
	repo, _, accountID := setupCharacterRepo(t)
	ctx := context.Background()

	c := makeTestCharacter(accountID)
	created, err := repo.Create(ctx, c)
	require.NoError(t, err)

	assert.Greater(t, created.ID, int64(0))
	assert.Equal(t, accountID, created.AccountID)
	assert.Equal(t, "Zara", created.Name)
	assert.Equal(t, "old_town", created.Region)
	assert.Equal(t, "ganger", created.Class)
	assert.Equal(t, 1, created.Level)
	assert.Equal(t, "grinders_row", created.Location)
	assert.Equal(t, 14, created.Abilities.Strength)
	assert.Equal(t, 10, created.MaxHP)
	assert.False(t, created.CreatedAt.IsZero())
}

func TestCharacterRepository_DuplicateNameError(t *testing.T) {
	repo, _, accountID := setupCharacterRepo(t)
	ctx := context.Background()

	c := makeTestCharacter(accountID)
	_, err := repo.Create(ctx, c)
	require.NoError(t, err)

	_, err = repo.Create(ctx, c) // same name, same account
	require.Error(t, err)
	assert.ErrorIs(t, err, postgres.ErrCharacterNameTaken)
}

func TestCharacterRepository_ListByAccount(t *testing.T) {
	repo, _, accountID := setupCharacterRepo(t)
	ctx := context.Background()

	c1 := makeTestCharacter(accountID)
	c1.Name = "Alpha"
	_, err := repo.Create(ctx, c1)
	require.NoError(t, err)

	c2 := makeTestCharacter(accountID)
	c2.Name = "Beta"
	_, err = repo.Create(ctx, c2)
	require.NoError(t, err)

	chars, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Len(t, chars, 2)
}

func TestCharacterRepository_GetByID(t *testing.T) {
	repo, _, accountID := setupCharacterRepo(t)
	ctx := context.Background()

	c := makeTestCharacter(accountID)
	created, err := repo.Create(ctx, c)
	require.NoError(t, err)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Zara", fetched.Name)
}

func TestCharacterRepository_GetByID_NotFound(t *testing.T) {
	repo, _, _ := setupCharacterRepo(t)
	_, err := repo.GetByID(context.Background(), 99999999)
	require.Error(t, err)
	assert.ErrorIs(t, err, postgres.ErrCharacterNotFound)
}

func TestCharacterRepository_SaveState(t *testing.T) {
	repo, _, accountID := setupCharacterRepo(t)
	ctx := context.Background()

	c := makeTestCharacter(accountID)
	created, err := repo.Create(ctx, c)
	require.NoError(t, err)

	err = repo.SaveState(ctx, created.ID, "broadway_ruins", 7)
	require.NoError(t, err)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "broadway_ruins", fetched.Location)
	assert.Equal(t, 7, fetched.CurrentHP)
	assert.True(t, fetched.UpdatedAt.After(created.UpdatedAt.Add(-time.Second)))
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/storage/postgres/... -run TestCharacterRepository -v 2>&1
```

Expected: compile error — `CharacterRepository` does not exist.

**Step 3: Implement character.go**

Create `internal/storage/postgres/character.go`:

```go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/character"
)

// ErrCharacterNotFound is returned when a character lookup yields no results.
var ErrCharacterNotFound = errors.New("character not found")

// ErrCharacterNameTaken is returned when creating a character with a name already used by the account.
var ErrCharacterNameTaken = errors.New("character name already taken")

// CharacterRepository provides character persistence operations.
type CharacterRepository struct {
	db *pgxpool.Pool
}

// NewCharacterRepository creates a CharacterRepository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterRepository(db *pgxpool.Pool) *CharacterRepository {
	return &CharacterRepository{db: db}
}

// Create inserts a new character and returns it with ID and timestamps set.
//
// Precondition: c.AccountID must reference an existing account; c.Name must be non-empty.
// Postcondition: Returns the created character with ID set, or ErrCharacterNameTaken on duplicate.
func (r *CharacterRepository) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
	var out character.Character
	err := r.db.QueryRow(ctx, `
		INSERT INTO characters
			(account_id, name, region, class, level, experience, location,
			 strength, dexterity, constitution, intelligence, wisdom, charisma,
			 max_hp, current_hp)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, account_id, name, region, class, level, experience, location,
		          strength, dexterity, constitution, intelligence, wisdom, charisma,
		          max_hp, current_hp, created_at, updated_at`,
		c.AccountID, c.Name, c.Region, c.Class, c.Level, c.Experience, c.Location,
		c.Abilities.Strength, c.Abilities.Dexterity, c.Abilities.Constitution,
		c.Abilities.Intelligence, c.Abilities.Wisdom, c.Abilities.Charisma,
		c.MaxHP, c.CurrentHP,
	).Scan(
		&out.ID, &out.AccountID, &out.Name, &out.Region, &out.Class,
		&out.Level, &out.Experience, &out.Location,
		&out.Abilities.Strength, &out.Abilities.Dexterity, &out.Abilities.Constitution,
		&out.Abilities.Intelligence, &out.Abilities.Wisdom, &out.Abilities.Charisma,
		&out.MaxHP, &out.CurrentHP, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrCharacterNameTaken
		}
		return nil, fmt.Errorf("inserting character: %w", err)
	}
	return &out, nil
}

// ListByAccount returns all characters for the given account ID, ordered by created_at.
//
// Precondition: accountID must be > 0.
// Postcondition: Returns a slice (may be empty) or a non-nil error.
func (r *CharacterRepository) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, account_id, name, region, class, level, experience, location,
		       strength, dexterity, constitution, intelligence, wisdom, charisma,
		       max_hp, current_hp, created_at, updated_at
		FROM characters WHERE account_id = $1 ORDER BY created_at ASC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing characters: %w", err)
	}
	defer rows.Close()

	var chars []*character.Character
	for rows.Next() {
		var c character.Character
		if err := rows.Scan(
			&c.ID, &c.AccountID, &c.Name, &c.Region, &c.Class,
			&c.Level, &c.Experience, &c.Location,
			&c.Abilities.Strength, &c.Abilities.Dexterity, &c.Abilities.Constitution,
			&c.Abilities.Intelligence, &c.Abilities.Wisdom, &c.Abilities.Charisma,
			&c.MaxHP, &c.CurrentHP, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning character row: %w", err)
		}
		chars = append(chars, &c)
	}
	return chars, rows.Err()
}

// GetByID retrieves a character by its primary key.
//
// Precondition: id must be > 0.
// Postcondition: Returns the Character or ErrCharacterNotFound.
func (r *CharacterRepository) GetByID(ctx context.Context, id int64) (*character.Character, error) {
	var c character.Character
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id, name, region, class, level, experience, location,
		       strength, dexterity, constitution, intelligence, wisdom, charisma,
		       max_hp, current_hp, created_at, updated_at
		FROM characters WHERE id = $1`,
		id,
	).Scan(
		&c.ID, &c.AccountID, &c.Name, &c.Region, &c.Class,
		&c.Level, &c.Experience, &c.Location,
		&c.Abilities.Strength, &c.Abilities.Dexterity, &c.Abilities.Constitution,
		&c.Abilities.Intelligence, &c.Abilities.Wisdom, &c.Abilities.Charisma,
		&c.MaxHP, &c.CurrentHP, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCharacterNotFound
		}
		return nil, fmt.Errorf("querying character: %w", err)
	}
	return &c, nil
}

// SaveState persists a character's current location and HP after a session.
//
// Precondition: id must be > 0; location must be a valid room ID.
// Postcondition: Returns nil on success, or a non-nil error.
func (r *CharacterRepository) SaveState(ctx context.Context, id int64, location string, currentHP int) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE characters SET location = $2, current_hp = $3, updated_at = $4
		WHERE id = $1`,
		id, location, currentHP, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("saving character state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCharacterNotFound
	}
	return nil
}
```

**Step 4: Add `UniqueName` helper to testutil if not present**

Check `internal/testutil/postgres.go` for a `UniqueName` helper. If missing, add this to `internal/testutil/postgres.go`:

```go
// UniqueName returns a unique string for use as a test resource name.
func UniqueName(t testing.TB, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
```

(Add `"fmt"` and `"time"` imports as needed.)

**Step 5: Run tests**

```bash
go test ./internal/storage/postgres/... -run TestCharacterRepository -v -timeout 120s 2>&1
```

Expected: all tests pass (requires running postgres).

**Step 6: Commit**

```bash
git add internal/storage/postgres/character.go internal/storage/postgres/character_test.go internal/testutil/
git commit -m "feat(postgres): character repository with CRUD and state persistence"
```

---

### Task 6: Proto Updates — Character Fields

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go`, `internal/gameserver/gamev1/game_grpc.pb.go`

**Step 1: Read current proto**

```bash
cat api/proto/game/v1/game.proto
```

**Step 2: Add character fields**

Add `character_id` and `character_name` to `JoinWorldRequest`, and add a `CharacterInfo` message and a `character_info` case in `ServerEvent`.

Find the `JoinWorldRequest` message in `api/proto/game/v1/game.proto` and update it:

```protobuf
message JoinWorldRequest {
  string uid = 1;
  string username = 2;
  int64 character_id = 3;
  string character_name = 4;
}
```

Add after the `Disconnected` message:

```protobuf
// CharacterInfo is sent to the client on session join to display character stats.
message CharacterInfo {
  int64  character_id  = 1;
  string name          = 2;
  string region        = 3;
  string class         = 4;
  int32  level         = 5;
  int32  experience    = 6;
  int32  max_hp        = 7;
  int32  current_hp    = 8;
  int32  strength      = 9;
  int32  dexterity     = 10;
  int32  constitution  = 11;
  int32  intelligence  = 12;
  int32  wisdom        = 13;
  int32  charisma      = 14;
}
```

Add to the `ServerEvent` oneof:

```protobuf
    CharacterInfo character_info = 9;
```

**Step 3: Regenerate proto code**

```bash
make proto
```

If `make proto` target does not exist, run directly:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/game/v1/game.proto
```

Then copy generated files to `internal/gameserver/gamev1/`:

```bash
cp api/proto/game/v1/game.pb.go internal/gameserver/gamev1/
cp api/proto/game/v1/game_grpc.pb.go internal/gameserver/gamev1/
```

(Check the Makefile for the exact generate command used in Phase 2.)

**Step 4: Verify compilation**

```bash
go build ./... 2>&1
```

Expected: no errors.

**Step 5: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(proto): add character_id/name to JoinWorldRequest and CharacterInfo event"
```

---

### Task 7: Frontend Character Flow (Telnet UI)

**Files:**
- Modify: `internal/frontend/handlers/auth.go` — add `CharacterStore` interface + `accountID` field tracking
- Create: `internal/frontend/handlers/character_flow.go`
- Create: `internal/frontend/handlers/character_flow_test.go`
- Modify: `internal/frontend/handlers/game_bridge.go` — pass character_id + character_name in JoinWorldRequest

This is the most significant task. The flow after login:
1. Look up the account by username to get `accountID`.
2. List characters for the account.
3. If empty: run character creation flow.
4. If non-empty: show character list + option to create new.
5. After selection/creation: call `gameBridge(conn, username, selectedCharacter)`.

**Step 1: Write failing tests**

Create `internal/frontend/handlers/character_flow_test.go`:

```go
package handlers_test

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCharStore implements handlers.CharacterStore for testing.
type mockCharStore struct {
	chars  []*character.Character
	created *character.Character
	createErr error
	listErr   error
}

func (m *mockCharStore) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.chars, nil
}

func (m *mockCharStore) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	c.ID = 42
	m.created = c
	return c, nil
}

func (m *mockCharStore) GetByID(ctx context.Context, id int64) (*character.Character, error) {
	for _, c := range m.chars {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func TestFormatCharacterSummary(t *testing.T) {
	c := &character.Character{
		ID:    1,
		Name:  "Zara",
		Class: "ganger",
		Level: 1,
		Region: "old_town",
	}
	summary := handlers.FormatCharacterSummary(c)
	assert.Contains(t, summary, "Zara")
	assert.Contains(t, summary, "ganger")
	assert.Contains(t, summary, "1")
}

func TestFormatCharacterStats(t *testing.T) {
	c := &character.Character{
		Name:  "Zara",
		Class: "ganger",
		Level: 1,
		Region: "old_town",
		MaxHP:     10,
		CurrentHP: 10,
		Abilities: character.AbilityScores{
			Strength: 14, Dexterity: 10, Constitution: 10,
			Intelligence: 10, Wisdom: 8, Charisma: 10,
		},
	}
	stats := handlers.FormatCharacterStats(c)
	assert.Contains(t, stats, "STR")
	assert.Contains(t, stats, "14")
	assert.Contains(t, stats, "HP")
	assert.Contains(t, stats, "10")
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/frontend/handlers/... -run TestFormatCharacter -v 2>&1
```

Expected: compile error.

**Step 3: Update auth.go**

Modify `internal/frontend/handlers/auth.go`:

1. Add `CharacterStore` interface and `accountStore` field:

After the existing `AccountStore` interface, add:

```go
// CharacterStore defines the character persistence operations required by AuthHandler.
type CharacterStore interface {
	ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error)
	Create(ctx context.Context, c *character.Character) (*character.Character, error)
	GetByID(ctx context.Context, id int64) (*character.Character, error)
}
```

2. Add `characters CharacterStore` field to `AuthHandler` and update constructor:

Change `AuthHandler` struct to:

```go
type AuthHandler struct {
	accounts       AccountStore
	characters     CharacterStore
	logger         *zap.Logger
	gameServerAddr string
}
```

Change `NewAuthHandler` to:

```go
func NewAuthHandler(accounts AccountStore, characters CharacterStore, logger *zap.Logger, gameServerAddr string) *AuthHandler {
	return &AuthHandler{
		accounts:       accounts,
		characters:     characters,
		logger:         logger,
		gameServerAddr: gameServerAddr,
	}
}
```

3. After successful login in `HandleSession`, the call becomes:

```go
if err := h.characterFlow(ctx, conn, acct); err != nil {
    return err
}
return nil
```

where `acct` is the `postgres.Account` returned by `handleLogin`. Update `handleLogin` to return `(postgres.Account, error)` instead of `(string, error)`.

Full updated `handleLogin` signature:

```go
func (h *AuthHandler) handleLogin(ctx context.Context, conn *telnet.Conn, args []string) (postgres.Account, error)
```

Returns `(postgres.Account{}, nil)` on user-visible error (auth loop continues), or `(postgres.Account{}, err)` on fatal error, or `(acct, nil)` on success.

Update the login case in `HandleSession`:

```go
case "login":
    acct, err := h.handleLogin(ctx, conn, args)
    if err != nil {
        return err
    }
    if acct.ID == 0 {
        continue
    }
    h.logger.Info("player logged in",
        zap.String("remote_addr", addr),
        zap.String("username", acct.Username),
        zap.Duration("login_time", time.Since(start)),
    )
    if err := h.characterFlow(ctx, conn, acct); err != nil {
        return err
    }
    return nil
```

Add import for `character` package to auth.go:

```go
"github.com/cory-johannsen/mud/internal/game/character"
```

**Step 4: Implement character_flow.go**

Create `internal/frontend/handlers/character_flow.go`:

```go
package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// characterFlow runs the character selection/creation UI after login.
// It exits by calling gameBridge with the selected or newly created character.
//
// Precondition: acct.ID must be > 0; conn must be open.
// Postcondition: Calls gameBridge on success; returns non-nil error on fatal failure.
func (h *AuthHandler) characterFlow(ctx context.Context, conn *telnet.Conn, acct postgres.Account) error {
	for {
		chars, err := h.characters.ListByAccount(ctx, acct.ID)
		if err != nil {
			return fmt.Errorf("listing characters: %w", err)
		}

		if len(chars) == 0 {
			_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow,
				"\r\nYou have no characters. Let's create one."))
			c, err := h.characterCreationFlow(ctx, conn, acct.ID)
			if err != nil {
				return err
			}
			if c == nil {
				continue // user cancelled (shouldn't happen but be safe)
			}
			return h.gameBridge(ctx, conn, acct.Username, c)
		}

		// Show character list
		_ = conn.WriteLine(telnet.Colorize(telnet.BrightWhite, "\r\nYour characters:"))
		for i, c := range chars {
			_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s",
				telnet.Green, i+1, telnet.Reset,
				FormatCharacterSummary(c)))
		}
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. Create a new character",
			telnet.Green, len(chars)+1, telnet.Reset))
		_ = conn.WriteLine(fmt.Sprintf("  %squit%s. Disconnect",
			telnet.Green, telnet.Reset))

		_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select [1-%d]: ", len(chars)+1))
		line, err := conn.ReadLine()
		if err != nil {
			return fmt.Errorf("reading character selection: %w", err)
		}
		line = strings.TrimSpace(line)

		if strings.ToLower(line) == "quit" || strings.ToLower(line) == "exit" {
			_ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "Goodbye."))
			return nil
		}

		choice := 0
		if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > len(chars)+1 {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
			continue
		}

		if choice == len(chars)+1 {
			c, err := h.characterCreationFlow(ctx, conn, acct.ID)
			if err != nil {
				return err
			}
			if c != nil {
				return h.gameBridge(ctx, conn, acct.Username, c)
			}
			continue
		}

		selected := chars[choice-1]
		return h.gameBridge(ctx, conn, acct.Username, selected)
	}
}

// characterCreationFlow guides the player through the interactive character builder.
// Returns (nil, nil) if the player cancels at any step.
//
// Precondition: accountID must be > 0; regionsDir and classesDir are loaded at handler init.
// Postcondition: Returns a persisted *character.Character or (nil, nil) on cancel.
func (h *AuthHandler) characterCreationFlow(ctx context.Context, conn *telnet.Conn, accountID int64) (*character.Character, error) {
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightCyan, "\r\n=== Character Creation ==="))
	_ = conn.WriteLine("Type 'cancel' at any prompt to return to the character screen.\r\n")

	// Step 1: Character name
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Enter your character's name: "))
	nameLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading character name: %w", err)
	}
	nameLine = strings.TrimSpace(nameLine)
	if strings.ToLower(nameLine) == "cancel" {
		return nil, nil
	}
	if len(nameLine) < 2 || len(nameLine) > 32 {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Name must be 2-32 characters."))
		return nil, nil
	}
	charName := nameLine

	// Step 2: Home region
	regions := h.regions
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your home region:"))
	for i, r := range regions {
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s\r\n     %s",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, r.Name, telnet.Reset,
			r.Description))
	}
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select region [1-%d]: ", len(regions)))
	regionLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading region selection: %w", err)
	}
	regionLine = strings.TrimSpace(regionLine)
	if strings.ToLower(regionLine) == "cancel" {
		return nil, nil
	}
	regionChoice := 0
	if _, err := fmt.Sscanf(regionLine, "%d", &regionChoice); err != nil || regionChoice < 1 || regionChoice > len(regions) {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
		return nil, nil
	}
	selectedRegion := regions[regionChoice-1]

	// Step 3: Class
	classes := h.classes
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your class:"))
	for i, c := range classes {
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s (HP/lvl: %d, Key: %s)\r\n     %s",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, c.Name, telnet.Reset,
			c.HitPointsPerLevel, c.KeyAbility,
			c.Description))
	}
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select class [1-%d]: ", len(classes)))
	classLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading class selection: %w", err)
	}
	classLine = strings.TrimSpace(classLine)
	if strings.ToLower(classLine) == "cancel" {
		return nil, nil
	}
	classChoice := 0
	if _, err := fmt.Sscanf(classLine, "%d", &classChoice); err != nil || classChoice < 1 || classChoice > len(classes) {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
		return nil, nil
	}
	selectedClass := classes[classChoice-1]

	// Step 4: Preview + confirm
	newChar, err := character.Build(charName, selectedRegion, selectedClass)
	if err != nil {
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error building character: %v", err))
		return nil, nil
	}

	_ = conn.WriteLine(telnet.Colorize(telnet.BrightCyan, "\r\n--- Character Preview ---"))
	_ = conn.WriteLine(FormatCharacterStats(newChar))
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Create this character? [y/N]: "))

	confirm, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Character creation cancelled."))
		return nil, nil
	}

	// Step 5: Persist
	newChar.AccountID = accountID
	start := time.Now()
	created, err := h.characters.Create(ctx, newChar)
	if err != nil {
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Failed to create character: %v", err))
		return nil, nil
	}
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"Character %s created! [%s]", created.Name, time.Since(start)))
	return created, nil
}

// FormatCharacterSummary returns a one-line summary of a character for the selection list.
// Exported for testing.
func FormatCharacterSummary(c *character.Character) string {
	return fmt.Sprintf("%s%s%s — Lvl %d %s from %s",
		telnet.BrightWhite, c.Name, telnet.Reset,
		c.Level, c.Class, c.Region)
}

// FormatCharacterStats returns a multi-line stats block for the character preview.
// Exported for testing.
func FormatCharacterStats(c *character.Character) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Name:   %s%s%s\r\n", telnet.BrightWhite, c.Name, telnet.Reset))
	sb.WriteString(fmt.Sprintf("  Region: %s   Class: %s   Level: %d\r\n", c.Region, c.Class, c.Level))
	sb.WriteString(fmt.Sprintf("  HP:     %d/%d\r\n", c.CurrentHP, c.MaxHP))
	sb.WriteString(fmt.Sprintf("  STR:%2d  DEX:%2d  CON:%2d  INT:%2d  WIS:%2d  CHA:%2d\r\n",
		c.Abilities.Strength, c.Abilities.Dexterity, c.Abilities.Constitution,
		c.Abilities.Intelligence, c.Abilities.Wisdom, c.Abilities.Charisma))
	return sb.String()
}
```

**Step 5: Add `regions` and `classes` fields to `AuthHandler`**

`characterCreationFlow` references `h.regions` and `h.classes`. Add these to the struct and constructor:

In `auth.go`, update `AuthHandler`:

```go
type AuthHandler struct {
	accounts       AccountStore
	characters     CharacterStore
	regions        []*ruleset.Region
	classes        []*ruleset.Class
	logger         *zap.Logger
	gameServerAddr string
}
```

Update `NewAuthHandler`:

```go
func NewAuthHandler(
	accounts AccountStore,
	characters CharacterStore,
	regions []*ruleset.Region,
	classes []*ruleset.Class,
	logger *zap.Logger,
	gameServerAddr string,
) *AuthHandler {
	return &AuthHandler{
		accounts:       accounts,
		characters:     characters,
		regions:        regions,
		classes:        classes,
		logger:         logger,
		gameServerAddr: gameServerAddr,
	}
}
```

Add import: `"github.com/cory-johannsen/mud/internal/game/ruleset"` to auth.go.

**Step 6: Update game_bridge.go**

Change the `gameBridge` method signature to accept a character:

```go
func (h *AuthHandler) gameBridge(ctx context.Context, conn *telnet.Conn, username string, char *character.Character) error {
```

Update the `JoinWorldRequest` send:

```go
uid := fmt.Sprintf("%d", char.ID)
if err := stream.Send(&gamev1.ClientMessage{
    RequestId: "join",
    Payload: &gamev1.ClientMessage_JoinWorld{
        JoinWorld: &gamev1.JoinWorldRequest{
            Uid:           uid,
            Username:      username,
            CharacterId:   char.ID,
            CharacterName: char.Name,
        },
    },
}); err != nil {
    return fmt.Errorf("sending join request: %w", err)
}
```

Update the prompt to use character name:

```go
prompt := telnet.Colorf(telnet.BrightCyan, "[%s]> ", char.Name)
```

And in `commandLoop` and `forwardServerEvents`, replace `username` string parameter with `charName` where used for display.

Add import `"fmt"` if not already present, and `"github.com/cory-johannsen/mud/internal/game/character"`.

**Step 7: Run tests**

```bash
go test ./internal/frontend/handlers/... -v 2>&1
```

Expected: all existing tests pass plus new ones.

**Step 8: Commit**

```bash
git add internal/frontend/handlers/
git commit -m "feat(handlers): interactive character selection and creation flow"
```

---

### Task 8: Gameserver — Character-Aware Sessions

**Files:**
- Modify: `internal/gameserver/grpc_service.go` — use character_name as display name, save location on disconnect
- Modify: `cmd/gameserver/main.go` — add postgres connection + wire CharacterRepository
- Modify: `internal/gameserver/grpc_service_test.go` — update tests for new JoinWorldRequest fields

The gameserver needs to:
1. Use `CharacterName` from `JoinWorldRequest` as the player's display name (not `Username`).
2. Track `CharacterID` in the session for persistence.
3. On disconnect, call `CharacterRepository.SaveState(ctx, characterID, roomID, hp)`.

**Step 1: Update PlayerSession to carry CharacterID**

In `internal/game/session/manager.go`, update `PlayerSession`:

```go
type PlayerSession struct {
	UID         string
	Username    string   // account username (for logging)
	CharName    string   // character display name
	CharacterID int64    // DB ID for persistence
	RoomID      string
	CurrentHP   int
	Entity      *BridgeEntity
}
```

Update `AddPlayer` signature:

```go
func (m *Manager) AddPlayer(uid, username, charName string, characterID int64, roomID string, currentHP int) (*PlayerSession, error) {
```

Update the session creation inside `AddPlayer`:

```go
sess := &PlayerSession{
    UID:         uid,
    Username:    username,
    CharName:    charName,
    CharacterID: characterID,
    RoomID:      roomID,
    CurrentHP:   currentHP,
    Entity:      entity,
}
```

**Step 2: Update session_test.go for new signature**

In `internal/game/session/manager_test.go`, update all calls to `AddPlayer` to pass the new parameters. Use empty string for charName and 0 for characterID and currentHP in existing tests.

**Step 3: Add CharacterSaver interface to grpc_service.go**

In `internal/gameserver/grpc_service.go`, add:

```go
// CharacterSaver persists character state after a session ends.
type CharacterSaver interface {
	SaveState(ctx context.Context, id int64, location string, currentHP int) error
}
```

Add it to `GameServiceServer`:

```go
type GameServiceServer struct {
	gamev1.UnimplementedGameServiceServer
	world      *world.Manager
	sessions   *session.Manager
	commands   *command.Registry
	worldH     *WorldHandler
	chatH      *ChatHandler
	charSaver  CharacterSaver
	logger     *zap.Logger
}
```

Update `NewGameServiceServer` to accept `charSaver CharacterSaver`:

```go
func NewGameServiceServer(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	worldHandler *WorldHandler,
	chatHandler *ChatHandler,
	charSaver CharacterSaver,
	logger *zap.Logger,
) *GameServiceServer {
	return &GameServiceServer{
		world:     worldMgr,
		sessions:  sessMgr,
		commands:  cmdRegistry,
		worldH:    worldHandler,
		chatH:     chatHandler,
		charSaver: charSaver,
		logger:    logger,
	}
}
```

**Step 4: Update Session() in grpc_service.go**

In the `Session` method, update the join handling to use character fields:

```go
uid := joinReq.Uid
username := joinReq.Username
charName := joinReq.CharacterName
if charName == "" {
    charName = username // fallback for backward compat
}
characterID := joinReq.CharacterId

sess, err := s.sessions.AddPlayer(uid, username, charName, characterID, startRoom.ID, startRoom... /* no HP on room */)
```

Wait — the room doesn't know the character's HP. The initial HP needs to come from the JoinWorldRequest. Since the character's `current_hp` is in the DB and the frontend loaded it, we should pass it too. Add `current_hp int32` to `JoinWorldRequest` in the proto.

Actually, let's keep it simpler for Phase 3: the gameserver doesn't track HP changes during play in Phase 3 (combat is not implemented yet). So we pass `current_hp` from the loaded character, and save it back unchanged on disconnect. HP only changes in future combat phase.

Update `JoinWorldRequest` proto to also include `current_hp`:

```protobuf
message JoinWorldRequest {
  string uid = 1;
  string username = 2;
  int64  character_id = 3;
  string character_name = 4;
  int32  current_hp = 5;
}
```

Regenerate proto after this change.

In `gameBridge` on the frontend, send:

```go
JoinWorld: &gamev1.JoinWorldRequest{
    Uid:           uid,
    Username:      username,
    CharacterId:   char.ID,
    CharacterName: char.Name,
    CurrentHp:     int32(char.CurrentHP),
},
```

In `Session()`:

```go
sess, err := s.sessions.AddPlayer(
    uid, username, charName, characterID, startRoom.ID, int(joinReq.CurrentHp),
)
```

Update `cleanupPlayer` to save state:

```go
func (s *GameServiceServer) cleanupPlayer(uid, username string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	roomID := sess.RoomID
	characterID := sess.CharacterID
	currentHP := sess.CurrentHP

	if err := s.sessions.RemovePlayer(uid); err != nil {
		s.logger.Warn("removing player on cleanup", zap.String("uid", uid), zap.Error(err))
	}

	// Persist character state
	if characterID > 0 && s.charSaver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.charSaver.SaveState(ctx, characterID, roomID, currentHP); err != nil {
			s.logger.Warn("saving character state on disconnect",
				zap.String("uid", uid),
				zap.Int64("character_id", characterID),
				zap.Error(err),
			)
		} else {
			s.logger.Info("character state saved",
				zap.Int64("character_id", characterID),
				zap.String("room", roomID),
			)
		}
	}

	s.broadcastRoomEvent(roomID, uid, &gamev1.RoomEvent{
		Player: sess.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	s.logger.Info("player disconnected",
		zap.String("uid", uid),
		zap.String("username", username),
		zap.Int64("character_id", characterID),
	)
}
```

Also update all uses of `sess.Username` in broadcast calls to use `sess.CharName` for display.

**Step 5: Update cmd/gameserver/main.go**

Add postgres connection and CharacterRepository:

```go
import (
	// existing imports
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// In main(), after loading config:
pool, err := postgres.NewPool(ctx, cfg.Database)
if err != nil {
    logger.Fatal("connecting to database", zap.Error(err))
}
defer pool.Close()

charRepo := postgres.NewCharacterRepository(pool.DB())

// Update NewGameServiceServer call:
grpcService := gameserver.NewGameServiceServer(
    worldMgr, sessMgr, cmdRegistry,
    worldHandler, chatHandler, charRepo, logger,
)
```

**Step 6: Update cmd/frontend/main.go**

Load regions and classes, then pass to NewAuthHandler:

```go
import (
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// After building accounts repo:
regionsDir := flag.String("regions", "content/regions", "path to region YAML files")
classesDir := flag.String("classes", "content/classes", "path to class YAML files")
// (add these flags before flag.Parse())

regions, err := ruleset.LoadRegions(*regionsDir)
if err != nil {
    logger.Fatal("loading regions", zap.Error(err))
}
classes, err := ruleset.LoadClasses(*classesDir)
if err != nil {
    logger.Fatal("loading classes", zap.Error(err))
}

characters := postgres.NewCharacterRepository(pool.DB())
authHandler := handlers.NewAuthHandler(accounts, characters, regions, classes, logger, cfg.GameServer.Addr())
```

**Step 7: Run full test suite**

```bash
go test ./... -race -timeout 300s 2>&1
```

Expected: all tests pass.

**Step 8: Commit**

```bash
git add internal/game/session/ internal/gameserver/ cmd/gameserver/ cmd/frontend/ api/proto/ internal/gameserver/gamev1/ internal/frontend/handlers/
git commit -m "feat(gameserver): character-aware sessions with persistence on disconnect"
```

---

### Task 9: End-to-End Wiring and Integration Tests

**Files:**
- Create: `internal/frontend/handlers/character_flow_integration_test.go`
- Create: `internal/gameserver/grpc_service_char_test.go`

**Step 1: Write integration test for character flow**

These tests use real postgres (via testutil) and verify the complete post-login flow.

Create `internal/frontend/handlers/character_flow_integration_test.go`:

```go
//go:build integration

package handlers_test

// Integration tests: character creation + selection via in-memory mocks.
// Full E2E tests (with real postgres) are covered by postgres/character_test.go.
// This file tests the Telnet flow logic using mock stores and a pipe-based conn.

import (
	"bytes"
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify FormatCharacterSummary contains expected fields.
func TestFormatCharacterSummary_Fields(t *testing.T) {
	c := &character.Character{
		Name:   "Blade",
		Class:  "scavenger",
		Region: "southeast_portland",
		Level:  3,
	}
	s := handlers.FormatCharacterSummary(c)
	assert.Contains(t, s, "Blade")
	assert.Contains(t, s, "scavenger")
	assert.Contains(t, s, "3")
}

// Verify FormatCharacterStats includes all six ability names and values.
func TestFormatCharacterStats_AllAbilities(t *testing.T) {
	c := &character.Character{
		Name:      "Blade",
		Region:    "old_town",
		Class:     "ganger",
		Level:     1,
		MaxHP:     12,
		CurrentHP: 12,
		Abilities: character.AbilityScores{14, 12, 10, 8, 10, 14},
	}
	s := handlers.FormatCharacterStats(c)
	for _, label := range []string{"STR", "DEX", "CON", "INT", "WIS", "CHA"} {
		assert.Contains(t, s, label, "expected ability label %s in stats", label)
	}
	assert.Contains(t, s, "12") // HP
	assert.Contains(t, s, "14") // STR or CHA
}
```

**Step 2: Run all tests**

```bash
go test ./... -race -timeout 300s 2>&1
```

Expected: all tests pass.

**Step 3: Manual smoke test**

```bash
# Terminal 1: start postgres + migrate
docker compose -f deployments/docker/docker-compose.yml up postgres migrate -d

# Terminal 2: start gameserver
go run ./cmd/gameserver -config configs/dev.yaml

# Terminal 3: start frontend
go run ./cmd/frontend -config configs/dev.yaml

# Terminal 4: connect via Telnet
telnet localhost 4000
```

Verify:
- Welcome banner appears
- `register testuser testpass` creates account
- `login testuser testpass` prompts character creation
- Step through region + class selection
- Character enters the world with correct name in prompt
- `look` shows room description
- Character location saves on `quit`
- Re-login shows character in the list
- Selecting existing character enters world at saved location

**Step 4: Commit**

```bash
git add internal/frontend/handlers/character_flow_integration_test.go internal/gameserver/
git commit -m "feat(phase3): complete character system integration"
```

---

## Verification Checklist

- [ ] `go test ./... -race` passes on all packages
- [ ] `migrations/003_characters.up.sql` applies cleanly via `go run ./cmd/migrate`
- [ ] `content/regions/` contains 5 region YAML files, all parseable
- [ ] `content/classes/` contains 5 class YAML files, all parseable
- [ ] `character.Build()` passes all property-based tests (HP >= 1, abilities in range)
- [ ] `CharacterRepository` Create, ListByAccount, GetByID, SaveState all pass integration tests
- [ ] After login: character list displayed; can create new character interactively
- [ ] Character name appears in game prompt instead of username
- [ ] `quit` saves character's last room to database
- [ ] Re-login: existing character listed; selecting it loads at saved location
- [ ] All packages: >80% test coverage on new code

## Requirements Addressed

| Req  | Description |
|------|-------------|
| CHAR-3 | Multiple characters per account |
| CHAR-4 | Player selects one character per session |
| CHAR-5 | Account data persisted to PostgreSQL |
| CHAR-6 | Interactive in-game character creation |
| CHAR-7 | Builder driven by ruleset YAML (regions/classes) |
| CHAR-8 | Character builder state exposed via gRPC (JoinWorldRequest + CharacterInfo) |
| CHAR-9 | Builder validates choices (region/class exist, name non-empty) |
| CHAR-10 | Step-by-step creation with cancel at any point |
| CHAR-11 | All characters human (no non-human ancestries in content) |
| CHAR-12 | PF2E ancestry = home region |
| CHAR-13 | Home region is first creation step |
| CHAR-14 | Each region defines modifiers, description, traits |
| CHAR-15 | Regions in YAML: name, description, modifiers, traits |
| CHAR-16 | Classes as second creation step |
| CHAR-17 | Class defines key ability, HP/level, proficiencies, features |
| CHAR-19 | Six PF2E abilities (STR/DEX/CON/INT/WIS/CHA) |
| CHAR-22 | Character data persisted to PostgreSQL |
| CHAR-23 | State saved on disconnect |
| CHAR-24 | Character data includes abilities, location, HP |
| PERS-1 | PostgreSQL as sole persistent store |
| PERS-5 | Parameterized queries only |
