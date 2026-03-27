# Named NPCs (Wayne/Dwayne/Jennifer Dawg) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Dependency:** This plan MUST be implemented AFTER `npc-behaviors`, which replaces the static `Taunts` field with the HTN `say` operator system.

**Goal:** Add 3 named NPC YAML entries (wayne_dawg, dwayne_dawg, jennifer_dawg) with lore-appropriate placement in Rustbucket Ridge.

**Architecture:** YAML-only content feature — no Go code changes required; named NPCs use a `name` field override on the NPC Template to display a fixed name instead of a random one.

**Tech Stack:** YAML content files, existing NPC template loader

---

## Prerequisite Check

Before executing any task, verify that `non-combat-npcs` feature has been implemented:

- [ ] **Step: Confirm `NpcRole` field exists on `npc.Template`**

  Open `internal/game/npc/template.go` and confirm that `Template` has the field:
  ```go
  NpcRole string `yaml:"npc_role"`
  ```
  If it is absent, STOP — this plan MUST NOT proceed until `non-combat-npcs` adds that field (REQ-NN-0).

- [ ] **Step: Confirm `Taunts` field does NOT exist on `npc.Template`**

  Open `internal/game/npc/template.go` and verify that `Template` does NOT have a `Taunts`, `TauntChance`, or `TauntCooldown` field. If any of these fields are still present, STOP — this plan MUST NOT proceed until `npc-behaviors` has been implemented (those fields are removed and replaced by the HTN `say` operator system).

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `content/npcs/wayne_dawg.yaml` | Create | Wayne Dawg NPC template (REQ-NN-1 through REQ-NN-6, REQ-NN-14, REQ-NN-15, REQ-NN-16) |
| `content/npcs/jennifer_dawg.yaml` | Create | Jennifer Dawg NPC template (REQ-NN-1 through REQ-NN-6, REQ-NN-14, REQ-NN-15, REQ-NN-16) |
| `content/npcs/dwayne_dawg.yaml` | Create | Dwayne Dawg NPC template (REQ-NN-1 through REQ-NN-6, REQ-NN-14, REQ-NN-15, REQ-NN-16) |
| `content/ai/wayne_dawg_idle.yaml` | Create | Wayne Dawg AI domain with `say` operator dialog lines (REQ-NN-16) |
| `content/ai/jennifer_dawg_idle.yaml` | Create | Jennifer Dawg AI domain with `say` operator dialog lines (REQ-NN-16) |
| `content/ai/dwayne_dawg_idle.yaml` | Create | Dwayne Dawg AI domain with `say` operator dialog lines (REQ-NN-16) |
| `content/zones/rustbucket_ridge.yaml` | Modify | Update `wayne_dawgs_trailer`; add `dwayne_dawgs_trailer` room (REQ-NN-7 through REQ-NN-13) |
| `internal/game/npc/npc_named_test.go` | Create | Tests for all three named NPC templates loading and validating correctly |
| `internal/game/world/zone_named_npcs_test.go` | Create | Tests for rustbucket_ridge zone changes |
| `docs/features/npcs-named.md` | Modify | Mark all REQ checkboxes complete |

---

## Task 1: Named NPC Template Tests (TDD — write tests first)

**Files:**
- Create: `internal/game/npc/npc_named_test.go`

- [ ] **Step 1.1: Write failing tests for all three named NPC templates**

  Create `internal/game/npc/npc_named_test.go` with the following tests. These tests will fail because the YAML files do not yet exist.

  ```go
  package npc_test

  import (
      "os"
      "testing"

      "github.com/cory-johannsen/mud/internal/game/ai"
      "github.com/cory-johannsen/mud/internal/game/npc"
      "github.com/stretchr/testify/assert"
      "github.com/stretchr/testify/require"
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

  func loadAIDomainForNPC(t *testing.T, domainID string) *ai.Domain {
      t.Helper()
      data, err := os.ReadFile("../../../content/ai/" + domainID + ".yaml")
      require.NoError(t, err, "content/ai/%s.yaml must exist (REQ-NN-16)", domainID)
      domain, err := ai.LoadDomainFromBytes(data)
      require.NoError(t, err, "content/ai/%s.yaml must parse", domainID)
      return domain
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
  ```

- [ ] **Step 1.2: Run tests — confirm failure**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestNamedNPC|TestProperty_NamedNPCs" -v 2>&1 | tail -20
  ```

  Expected: FAIL (YAML files not found).

---

## Task 2: Zone Change Tests (TDD — write tests first)

**Files:**
- Create: `internal/game/world/zone_named_npcs_test.go`

- [ ] **Step 2.1: Write failing tests for rustbucket_ridge zone changes**

  Create `internal/game/world/zone_named_npcs_test.go`:

  ```go
  package world_test

  import (
      "testing"

      "github.com/cory-johannsen/mud/internal/game/world"
      "github.com/stretchr/testify/assert"
      "github.com/stretchr/testify/require"
  )

  func loadRustbucketRidge(t *testing.T) *world.Zone {
      t.Helper()
      zone, err := world.LoadZoneFromFile("../../../content/zones/rustbucket_ridge.yaml")
      require.NoError(t, err)
      return zone
  }

  func TestRustbucketRidge_WayneDawgsTrailer_DangerLevelSafe(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room, ok := zone.Rooms["wayne_dawgs_trailer"]
      require.True(t, ok, "wayne_dawgs_trailer must exist")
      assert.Equal(t, "safe", room.DangerLevel, "REQ-NN-7: danger_level must be safe")
  }

  func TestRustbucketRidge_WayneDawgsTrailer_UpdatedDescription(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room := zone.Rooms["wayne_dawgs_trailer"]
      assert.Contains(t, room.Description, "Wayne and Jennifer Dawg",
          "REQ-NN-8: description must reference both Wayne and Jennifer Dawg")
      assert.Contains(t, room.Description, "makeshift lab",
          "REQ-NN-8: description must mention makeshift lab")
  }

  func TestRustbucketRidge_WayneDawgsTrailer_HasWestExitToDwayne(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room := zone.Rooms["wayne_dawgs_trailer"]
      exit, ok := room.ExitForDirection(world.West)
      require.True(t, ok, "REQ-NN-10: wayne_dawgs_trailer must have a west exit")
      assert.Equal(t, "dwayne_dawgs_trailer", exit.Target,
          "REQ-NN-10: west exit must target dwayne_dawgs_trailer")
  }

  func TestRustbucketRidge_WayneDawgsTrailer_HasSpawnsForWayneAndJennifer(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room := zone.Rooms["wayne_dawgs_trailer"]
      templates := make(map[string]bool)
      for _, sp := range room.Spawns {
          templates[sp.Template] = true
      }
      assert.True(t, templates["wayne_dawg"],
          "REQ-NN-9: wayne_dawgs_trailer must have spawn for wayne_dawg")
      assert.True(t, templates["jennifer_dawg"],
          "REQ-NN-9: wayne_dawgs_trailer must have spawn for jennifer_dawg")
  }

  func TestRustbucketRidge_DwayneDawgsTrailer_Exists(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room, ok := zone.Rooms["dwayne_dawgs_trailer"]
      require.True(t, ok, "REQ-NN-11: dwayne_dawgs_trailer must exist")
      assert.Equal(t, "safe", room.DangerLevel, "REQ-NN-11: danger_level must be safe")
      assert.Equal(t, -4, room.MapX, "REQ-NN-11/REQ-NN-13: map_x must be -4")
      assert.Equal(t, 4, room.MapY, "REQ-NN-11/REQ-NN-13: map_y must be 4")
  }

  func TestRustbucketRidge_DwayneDawgsTrailer_HasEastExitToWayne(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room := zone.Rooms["dwayne_dawgs_trailer"]
      exit, ok := room.ExitForDirection(world.East)
      require.True(t, ok, "REQ-NN-12: dwayne_dawgs_trailer must have an east exit")
      assert.Equal(t, "wayne_dawgs_trailer", exit.Target,
          "REQ-NN-12: east exit must target wayne_dawgs_trailer")
  }

  func TestRustbucketRidge_DwayneDawgsTrailer_HasSpawnForDwayne(t *testing.T) {
      zone := loadRustbucketRidge(t)
      room := zone.Rooms["dwayne_dawgs_trailer"]
      templates := make(map[string]bool)
      for _, sp := range room.Spawns {
          templates[sp.Template] = true
      }
      assert.True(t, templates["dwayne_dawg"],
          "REQ-NN-12: dwayne_dawgs_trailer must have spawn for dwayne_dawg")
  }

  func TestRustbucketRidge_MapCoordinates_NoOverlap(t *testing.T) {
      zone := loadRustbucketRidge(t)
      type coord struct{ x, y int }
      seen := make(map[coord]string)
      for id, room := range zone.Rooms {
          c := coord{room.MapX, room.MapY}
          if existing, ok := seen[c]; ok {
              assert.Failf(t, "duplicate map coordinates",
                  "rooms %q and %q both have map_x=%d map_y=%d (REQ-NN-13)",
                  existing, id, room.MapX, room.MapY)
          }
          seen[c] = id
      }
  }
  ```

- [ ] **Step 2.2: Run tests — confirm failure**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestRustbucketRidge" -v 2>&1 | tail -20
  ```

  Expected: FAIL (room not found, missing fields).

---

## Task 3: Create Named NPC YAML Templates

**Files:**
- Create: `content/npcs/wayne_dawg.yaml`
- Create: `content/npcs/jennifer_dawg.yaml`
- Create: `content/npcs/dwayne_dawg.yaml`

- [ ] **Step 3.1: Create `content/npcs/wayne_dawg.yaml`**

  ```yaml
  # quest_giver: pending quests feature
  id: wayne_dawg
  name: "Wayne Dawg"
  description: "A grizzled old man with oil-stained hands and a knowing squint. He's been on the Ridge longer than most and has the deals to prove it."
  type: human
  npc_role: merchant
  disposition: friendly
  level: 3
  max_hp: 30
  ac: 10
  awareness: 8
  respawn_delay: "0s"
  ai_domain: wayne_dawg_idle
  abilities:
    brutality: 8
    quickness: 10
    grit: 12
    reasoning: 12
    savvy: 14
    flair: 10
  loot:
    currency:
      min: 20
      max: 80
  ```

- [ ] **Step 3.1b: Create `content/ai/wayne_dawg_idle.yaml`**

  ```yaml
  id: wayne_dawg_idle
  tasks:
    - id: behave
      methods:
        - id: m1
          subtasks:
            - say_op
  operators:
    - id: say_op
      action: say
      cooldown: "45s"
      strings:
        - "You need somethin', or you just here to look?"
        - "Jennifer! We got company!"
        - "Best deals on the Ridge, and that ain't braggin'."
        - "Don't let the place fool ya. I got good stock."
  ```

- [ ] **Step 3.2: Create `content/npcs/jennifer_dawg.yaml`**

  ```yaml
  # quest_giver: pending quests feature
  id: jennifer_dawg
  name: "Jennifer Dawg"
  description: "A sharp-eyed woman who keeps the trailer running while Wayne keeps it interesting. She sizes you up before Wayne's finished his sentence."
  type: human
  npc_role: merchant
  disposition: friendly
  level: 3
  max_hp: 28
  ac: 10
  awareness: 10
  respawn_delay: "0s"
  ai_domain: jennifer_dawg_idle
  abilities:
    brutality: 8
    quickness: 11
    grit: 11
    reasoning: 14
    savvy: 15
    flair: 11
  loot:
    currency:
      min: 20
      max: 80
  ```

- [ ] **Step 3.2b: Create `content/ai/jennifer_dawg_idle.yaml`**

  ```yaml
  id: jennifer_dawg_idle
  tasks:
    - id: behave
      methods:
        - id: m1
          subtasks:
            - say_op
  operators:
    - id: say_op
      action: say
      cooldown: "45s"
      strings:
        - "Wayne, let me handle this one."
        - "You look like you could use something useful."
        - "I know what you need before you do. That's just how it is."
        - "Fair prices. No nonsense. That's the Dawg way."
  ```

- [ ] **Step 3.3: Create `content/npcs/dwayne_dawg.yaml`**

  ```yaml
  # quest_giver: pending quests feature
  id: dwayne_dawg
  name: "Dwayne Dawg"
  description: "A big man in a small trailer with a lot of ideas. He talks like every deal is the deal of a lifetime. Sometimes he's right."
  type: human
  npc_role: merchant
  disposition: friendly
  level: 3
  max_hp: 32
  ac: 10
  awareness: 7
  respawn_delay: "0s"
  ai_domain: dwayne_dawg_idle
  abilities:
    brutality: 12
    quickness: 9
    grit: 13
    reasoning: 10
    savvy: 13
    flair: 13
  loot:
    currency:
      min: 20
      max: 80
  ```

- [ ] **Step 3.3b: Create `content/ai/dwayne_dawg_idle.yaml`**

  ```yaml
  id: dwayne_dawg_idle
  tasks:
    - id: behave
      methods:
        - id: m1
          subtasks:
            - say_op
  operators:
    - id: say_op
      action: say
      cooldown: "45s"
      strings:
        - "Dwayne Dawg, entrepreneur. At your service."
        - "Wayne's good, but I'm better. Don't tell him I said that."
        - "I got things. You need things. This works out."
        - "Pull up a chair. This won't take long."
  ```

- [ ] **Step 3.4: Run NPC template tests — confirm pass**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestNamedNPC|TestProperty_NamedNPCs" -v 2>&1 | tail -20
  ```

  Expected: PASS.

- [ ] **Step 3.5: Run full NPC test suite — confirm no regressions**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -30
  ```

  Expected: all pass.

- [ ] **Step 3.6: Commit NPC templates + tests**

  ```
  git add content/npcs/wayne_dawg.yaml content/npcs/jennifer_dawg.yaml content/npcs/dwayne_dawg.yaml content/ai/wayne_dawg_idle.yaml content/ai/jennifer_dawg_idle.yaml content/ai/dwayne_dawg_idle.yaml internal/game/npc/npc_named_test.go
  git commit -m "feat(npcs-named): add wayne_dawg, jennifer_dawg, dwayne_dawg NPC templates with HTN say domain files"
  ```

---

## Task 4: Update rustbucket_ridge.yaml

**Files:**
- Modify: `content/zones/rustbucket_ridge.yaml`

- [ ] **Step 4.1: Update `wayne_dawgs_trailer` room**

  In `content/zones/rustbucket_ridge.yaml`, locate the `wayne_dawgs_trailer` room entry and apply these changes:

  - Add `danger_level: safe` (REQ-NN-7)
  - Replace `description:` with: `"Wayne and Jennifer Dawg have made this rusted trailer into something almost livable. There's a makeshift lab bolted to the side wall and a card table by the door where deals get done. It smells like solder and something fried."` (REQ-NN-8)
  - Add a west exit targeting `dwayne_dawgs_trailer` to the `exits:` list (REQ-NN-10)
  - Add `spawns:` block with two entries: `wayne_dawg` (count: 1, respawn_after: 0s) and `jennifer_dawg` (count: 1, respawn_after: 0s) (REQ-NN-9)

  The updated room section must look like:

  ```yaml
  - id: wayne_dawgs_trailer
    title: Wayne Dawg's Trailer
    description: "Wayne and Jennifer Dawg have made this rusted trailer into something almost livable. There's a makeshift lab bolted to the side wall and a card table by the door where deals get done. It smells like solder and something fried."
    danger_level: safe
    exits:
    - direction: north
      target: the_green_hell
    - direction: west
      target: dwayne_dawgs_trailer
    map_x: -2
    map_y: 4
    spawns:
    - template: wayne_dawg
      count: 1
      respawn_after: 0s
    - template: jennifer_dawg
      count: 1
      respawn_after: 0s
  ```

- [ ] **Step 4.2: Add `dwayne_dawgs_trailer` room**

  Append a new room entry to the `rooms:` list in `content/zones/rustbucket_ridge.yaml`. Verify that map coordinates `-4, 4` are not already used (REQ-NN-13):

  ```yaml
  - id: dwayne_dawgs_trailer
    title: Dwayne Dawg's Trailer
    description: "A battered single-wide pressed up against the fence line. Dwayne has strung lights along the eaves and put out a folding chair like he's expecting company. He usually is."
    danger_level: safe
    exits:
    - direction: east
      target: wayne_dawgs_trailer
    map_x: -4
    map_y: 4
    spawns:
    - template: dwayne_dawg
      count: 1
      respawn_after: 0s
  ```

- [ ] **Step 4.3: Run zone tests — confirm pass**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestRustbucketRidge" -v 2>&1 | tail -30
  ```

  Expected: all pass.

- [ ] **Step 4.4: Run full world test suite — confirm no regressions**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -v 2>&1 | tail -30
  ```

  Expected: all pass.

- [ ] **Step 4.5: Commit zone changes + zone tests**

  ```
  git add content/zones/rustbucket_ridge.yaml internal/game/world/zone_named_npcs_test.go
  git commit -m "feat(npcs-named): update wayne_dawgs_trailer; add dwayne_dawgs_trailer to rustbucket_ridge"
  ```

---

## Task 5: Full Test Suite + Feature Completion

- [ ] **Step 5.1: Run full test suite**

  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -40
  ```

  Expected: all pass, zero failures.

- [ ] **Step 5.2: Mark feature complete in `docs/features/npcs-named.md`**

  Update `docs/features/npcs-named.md`: change all `- [ ]` checkboxes to `- [x]` for REQ-NN-0 through REQ-NN-16.

- [ ] **Step 5.3: Commit feature completion**

  ```
  git add docs/features/npcs-named.md
  git commit -m "chore(npcs-named): mark all requirements complete"
  ```
