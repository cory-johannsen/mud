# AI Item Quest Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the AI Chainsaw and AI AK-47 to players through two quest chains (Signal in the Static → Field Test) that converge on the Cipher NPC in The Velvet Rope. Both items also drop at 5% from the Gangbang boss.

**Architecture:** Pure data addition — no Go source changes. New NPC YAML (Cipher), four new quest YAML files, updates to two existing NPC quest-giver YAMLs, one zone file (spawn), and one boss NPC file (loot drops).

**Tech Stack:** YAML content files, Go test suite, existing `QuestRegistry` and `QuestService`.

**Dependency:** This plan MUST NOT be executed until `docs/superpowers/plans/2026-04-20-ai-item-content.md` is fully implemented and merged.

---

## File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `content/npcs/cipher.yaml` | Cipher NPC definition |
| Modify | `content/zones/the_velvet_rope.yaml` | Add Cipher spawn to `the_velvet_rope_brothel` |
| Modify | `content/npcs/gangbang.yaml` | Add 5% loot drops for both AI items |
| Modify | `content/npcs/rustbucket_ridge_quest_giver.yaml` | Add `machete_signal_in_static` quest ID |
| Modify | `content/npcs/vantucky_quest_giver.yaml` | Add `gun_signal_in_static` quest ID |
| Create | `content/quests/machete_signal_in_static.yaml` | Machete team intro quest |
| Create | `content/quests/gun_signal_in_static.yaml` | Gun team intro quest |
| Create | `content/quests/machete_field_test.yaml` | Machete team reward quest (AI Chainsaw) |
| Create | `content/quests/gun_field_test.yaml` | Gun team reward quest (AI AK-47) |
| Create | `internal/game/quest/cipher_quest_test.go` | Quest registry and prerequisite tests |

---

### Task 1: Cipher NPC YAML

**Files:**
- Create: `content/npcs/cipher.yaml`

The Cipher NPC is a quest giver. It uses the standard quest_giver NPC pattern with `npc_type: quest_giver`, `npc_role: quest_giver`, and a `quest_giver.quest_ids` list.

- [ ] **Step 1: Write the failing test**

Create `internal/game/quest/cipher_quest_test.go`:

```go
package quest_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// cipherNPC is a minimal struct for validating the cipher NPC YAML shape.
type cipherNPC struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	NPCType  string `yaml:"npc_type"`
	NPCRole  string `yaml:"npc_role"`
	QuestGiver struct {
		QuestIDs []string `yaml:"quest_ids"`
	} `yaml:"quest_giver"`
}

func TestCipherNPC_LoadsWithExpectedFields(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/cipher.yaml")
	if err != nil {
		t.Fatalf("read cipher.yaml: %v", err)
	}
	var npc cipherNPC
	if err := yaml.Unmarshal(data, &npc); err != nil {
		t.Fatalf("unmarshal cipher.yaml: %v", err)
	}
	if npc.ID != "cipher" {
		t.Errorf("expected id cipher, got %q", npc.ID)
	}
	if npc.NPCType != "quest_giver" {
		t.Errorf("expected npc_type quest_giver, got %q", npc.NPCType)
	}
	if len(npc.QuestGiver.QuestIDs) < 2 {
		t.Errorf("expected at least 2 quest_ids, got %d", len(npc.QuestGiver.QuestIDs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestCipherNPC_LoadsWithExpectedFields -v
```
Expected: FAIL — file not found.

- [ ] **Step 3: Create `content/npcs/cipher.yaml`**

```yaml
id: cipher
name: Cipher
npc_type: quest_giver
npc_role: quest_giver
type: human
description: >
  A slight figure hunched over a workbench covered in circuit boards and
  stripped weapon receivers. They don't look up when you enter. Whatever
  they're soldering, it's more interesting than you are. For now.
level: 0
max_hp: 22
ac: 10
awareness: 5
disposition: neutral
personality: neutral
quest_giver:
  placeholder_dialog:
    - "The modification takes weeks. The components aren't cheap. Prove yourself first."
    - "I don't do walk-ins. You were referred. That means something. Barely."
    - "I've been waiting for someone worth the components. Maybe that's you."
  quest_ids:
    - machete_field_test
    - gun_field_test
loot:
  currency:
    min: 0
    max: 0
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestCipherNPC_LoadsWithExpectedFields -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/npcs/cipher.yaml internal/game/quest/cipher_quest_test.go
git commit -m "feat(content): add Cipher NPC YAML and load test"
```

---

### Task 2: Add Cipher spawn to the_velvet_rope.yaml

**Files:**
- Modify: `content/zones/the_velvet_rope.yaml`

The `the_velvet_rope_brothel` room already has four spawns. Add Cipher as a fifth.

- [ ] **Step 1: Write the failing test**

Add to `internal/game/quest/cipher_quest_test.go`:

```go
// zoneRoom is a minimal struct for checking spawn lists.
type zoneRoom struct {
	ID     string `yaml:"id"`
	Spawns []struct {
		Template string `yaml:"template"`
	} `yaml:"spawns"`
}

type zoneFile struct {
	Zone struct {
		Rooms []zoneRoom `yaml:"rooms"`
	} `yaml:"zone"`
}

func TestVelvetRopeBrothel_HasCipherSpawn(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/the_velvet_rope.yaml")
	if err != nil {
		t.Fatalf("read the_velvet_rope.yaml: %v", err)
	}
	var zf zoneFile
	if err := yaml.Unmarshal(data, &zf); err != nil {
		t.Fatalf("unmarshal zone: %v", err)
	}
	for _, room := range zf.Zone.Rooms {
		if room.ID != "the_velvet_rope_brothel" {
			continue
		}
		for _, spawn := range room.Spawns {
			if spawn.Template == "cipher" {
				return // found
			}
		}
		t.Fatal("the_velvet_rope_brothel has no cipher spawn")
	}
	t.Fatal("room the_velvet_rope_brothel not found in zone")
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestVelvetRopeBrothel_HasCipherSpawn -v
```
Expected: FAIL — cipher spawn not present.

- [ ] **Step 3: Edit `content/zones/the_velvet_rope.yaml`**

The `the_velvet_rope_brothel` room's `spawns` block currently ends at:

```yaml
    - template: the_velvet_rope_quest_giver
      count: 1
      respawn_after: 0s
```

Add immediately after that entry (before `equipment:`):

```yaml
    - template: cipher
      count: 1
      respawn_after: 0s
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestVelvetRopeBrothel_HasCipherSpawn -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/zones/the_velvet_rope.yaml internal/game/quest/cipher_quest_test.go
git commit -m "feat(content): add Cipher spawn to VIP Lounge in The Velvet Rope"
```

---

### Task 3: Add boss loot drops to gangbang NPC

**Files:**
- Modify: `content/npcs/gangbang.yaml`

The gangbang NPC's `loot:` block currently contains only `currency`. Add items for both AI items at 5% drop chance each.

- [ ] **Step 1: Write the failing test**

Add to `internal/game/quest/cipher_quest_test.go`:

```go
type npcLoot struct {
	Loot struct {
		Items []struct {
			ItemID string  `yaml:"item"`
			Chance float64 `yaml:"chance"`
		} `yaml:"items"`
	} `yaml:"loot"`
}

func TestGangbang_HasAIItemDrops(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/gangbang.yaml")
	if err != nil {
		t.Fatalf("read gangbang.yaml: %v", err)
	}
	var npc npcLoot
	if err := yaml.Unmarshal(data, &npc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantItems := map[string]bool{
		"ai_chainsaw": false,
		"ai_ak47":     false,
	}
	for _, item := range npc.Loot.Items {
		if _, ok := wantItems[item.ItemID]; ok {
			wantItems[item.ItemID] = true
			if item.Chance != 0.05 {
				t.Errorf("item %q: expected chance 0.05, got %f", item.ItemID, item.Chance)
			}
		}
	}
	for itemID, found := range wantItems {
		if !found {
			t.Errorf("gangbang loot missing item %q", itemID)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestGangbang_HasAIItemDrops -v
```
Expected: FAIL — no items in gangbang loot.

- [ ] **Step 3: Edit `content/npcs/gangbang.yaml`**

The current `loot:` block is:

```yaml
loot:
  currency:
    min: 70
    max: 180
```

Replace with:

```yaml
loot:
  currency:
    min: 70
    max: 180
  items:
    - item: ai_chainsaw
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_ak47
      chance: 0.05
      min_qty: 1
      max_qty: 1
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestGangbang_HasAIItemDrops -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/npcs/gangbang.yaml internal/game/quest/cipher_quest_test.go
git commit -m "feat(content): add AI item boss drops to gangbang NPC"
```

---

### Task 4: Create all four quest YAML files

**Files:**
- Create: `content/quests/machete_signal_in_static.yaml`
- Create: `content/quests/gun_signal_in_static.yaml`
- Create: `content/quests/machete_field_test.yaml`
- Create: `content/quests/gun_field_test.yaml`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/quest/cipher_quest_test.go`:

```go
func TestQuestRegistry_CipherQuestsValid(t *testing.T) {
	questFiles := []string{
		"../../../content/quests/machete_signal_in_static.yaml",
		"../../../content/quests/gun_signal_in_static.yaml",
		"../../../content/quests/machete_field_test.yaml",
		"../../../content/quests/gun_field_test.yaml",
	}
	for _, path := range questFiles {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var def quest.QuestDef
			if err := yaml.Unmarshal(data, &def); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}
			if err := def.Validate(); err != nil {
				t.Errorf("Validate %s: %v", path, err)
			}
		})
	}
}
```

Note: add `"github.com/cory-johannsen/mud/internal/game/quest"` to imports.

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestQuestRegistry_CipherQuestsValid -v
```
Expected: FAIL — files not found.

- [ ] **Step 3: Create the four quest YAML files**

`content/quests/machete_signal_in_static.yaml`:
```yaml
id: machete_signal_in_static
title: Signal in the Static
description: >
  Word's going around that someone in The Velvet Rope has been pulling
  neural chips out of salvage and wiring them into weapons. Nobody knows
  who. Nobody knows how to find them. That's the point. Prove you're
  serious and the signal gets clearer.
giver_npc_id: rustbucket_ridge_quest_giver
repeatable: false
prerequisites:
  - rustbucket_ridge_slasher_takedown
objectives:
  - id: find_cipher
    type: explore
    description: Find the rogue AI technician in The Velvet Rope
    target_id: the_velvet_rope_brothel
    quantity: 1
rewards:
  xp: 800
  credits: 0
```

`content/quests/gun_signal_in_static.yaml`:
```yaml
id: gun_signal_in_static
title: Signal in the Static
description: >
  Word's going around that someone in The Velvet Rope has been pulling
  neural chips out of salvage and wiring them into weapons. Nobody knows
  who. Nobody knows how to find them. That's the point. Prove you're
  serious and the signal gets clearer.
giver_npc_id: vantucky_quest_giver
repeatable: false
prerequisites:
  - vantucky_militia_commander_takedown
objectives:
  - id: find_cipher
    type: explore
    description: Find the rogue AI technician in The Velvet Rope
    target_id: the_velvet_rope_brothel
    quantity: 1
rewards:
  xp: 800
  credits: 0
```

`content/quests/machete_field_test.yaml`:
```yaml
id: machete_field_test
title: Field Test
description: >
  Cipher doesn't give these things away. The modification takes weeks and
  the components aren't cheap. But they have a standing arrangement: prove
  you can operate at this tier, and the next one is yours. The VIP Chamber
  is where you prove it.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_vip_boss
    type: kill
    description: Kill the VIP in the VIP Chamber
    target_id: gangbang
    quantity: 1
rewards:
  xp: 1200
  credits: 500
  items:
    - item_id: ai_chainsaw
      quantity: 1
```

`content/quests/gun_field_test.yaml`:
```yaml
id: gun_field_test
title: Field Test
description: >
  Cipher doesn't give these things away. The modification takes weeks and
  the components aren't cheap. But they have a standing arrangement: prove
  you can operate at this tier, and the next one is yours. The VIP Chamber
  is where you prove it.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_vip_boss
    type: kill
    description: Kill the VIP in the VIP Chamber
    target_id: gangbang
    quantity: 1
rewards:
  xp: 1200
  credits: 500
  items:
    - item_id: ai_ak47
      quantity: 1
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run TestQuestRegistry_CipherQuestsValid -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/quests/machete_signal_in_static.yaml content/quests/gun_signal_in_static.yaml \
        content/quests/machete_field_test.yaml content/quests/gun_field_test.yaml \
        internal/game/quest/cipher_quest_test.go
git commit -m "feat(content): add Signal in the Static and Field Test quests for AI items"
```

---

### Task 5: Register quests with their giver NPCs

**Files:**
- Modify: `content/npcs/rustbucket_ridge_quest_giver.yaml`
- Modify: `content/npcs/vantucky_quest_giver.yaml`

Quest giver NPCs list their quest IDs explicitly. Signal in the Static quests must appear on their respective home-zone quest givers.

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/quest/cipher_quest_test.go`:

```go
func TestRustbucketRidgeQuestGiver_HasMacheteSignal(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/rustbucket_ridge_quest_giver.yaml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var npc struct {
		QuestGiver struct {
			QuestIDs []string `yaml:"quest_ids"`
		} `yaml:"quest_giver"`
	}
	yaml.Unmarshal(data, &npc)
	for _, id := range npc.QuestGiver.QuestIDs {
		if id == "machete_signal_in_static" {
			return
		}
	}
	t.Error("rustbucket_ridge_quest_giver missing machete_signal_in_static quest_id")
}

func TestVantuckyQuestGiver_HasGunSignal(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/vantucky_quest_giver.yaml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var npc struct {
		QuestGiver struct {
			QuestIDs []string `yaml:"quest_ids"`
		} `yaml:"quest_giver"`
	}
	yaml.Unmarshal(data, &npc)
	for _, id := range npc.QuestGiver.QuestIDs {
		if id == "gun_signal_in_static" {
			return
		}
	}
	t.Error("vantucky_quest_giver missing gun_signal_in_static quest_id")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run "TestRustbucketRidgeQuestGiver_HasMacheteSignal|TestVantuckyQuestGiver_HasGunSignal" -v
```
Expected: FAIL — quest IDs not present.

- [ ] **Step 3: Edit `content/npcs/rustbucket_ridge_quest_giver.yaml`**

Append `machete_signal_in_static` to the `quest_ids` list:

```yaml
  quest_ids:
    - rrq_scavenger_sweep
    - rrq_rail_gang_bounty
    - rrq_barrel_house_cleanup
    - rrq_take_down_big_grizz
    - machete_signal_in_static
```

- [ ] **Step 4: Edit `content/npcs/vantucky_quest_giver.yaml`**

Append `gun_signal_in_static` to the `quest_ids` list:

```yaml
  quest_ids:
    - vtq_militia_patrol
    - vtq_scavenger_drive
    - vtq_bandit_bounty
    - vtq_gang_enforcer_takedown
    - gun_signal_in_static
```

- [ ] **Step 5: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/quest/... -run "TestRustbucketRidgeQuestGiver_HasMacheteSignal|TestVantuckyQuestGiver_HasGunSignal" -v
```
Expected: PASS.

- [ ] **Step 6: Run full test suite**

```
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```
Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/npcs/rustbucket_ridge_quest_giver.yaml content/npcs/vantucky_quest_giver.yaml \
        internal/game/quest/cipher_quest_test.go
git commit -m "feat(content): register Signal in the Static quests with home-zone quest givers"
```
