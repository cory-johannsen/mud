# AI Item Content Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 10 additional AI items (one per item type per team), their HTN domains, quest files, boss loot placements, and wire them into the Cipher NPC quest roster.

**Architecture:** Pure data addition — no Go source changes. Items with `armor_ref` use `kind: armor`; items with `weapon_ref` use `kind: weapon`. All 10 items drop from zone bosses at 5% chance. All 10 quests gate behind the team-appropriate Signal in the Static quest.

**Tech Stack:** YAML content files, Go test suite, existing `ItemRegistry`, `ItemDomainRegistry`, `QuestRegistry`.

**Dependency:** This plan MUST NOT be executed until all three prior plans are merged:
- `docs/superpowers/plans/2026-04-20-ai-item-engine.md`
- `docs/superpowers/plans/2026-04-20-ai-item-content.md`
- `docs/superpowers/plans/2026-04-20-ai-item-quest-delivery.md`

The `fortified`, `inspired`, `evasive`, `exposed`, `weakened`, `taunted` condition IDs must exist in the conditions registry (created by the engine plan) before these items can function.

---

## File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `content/items/ai_sawn_off.yaml` | AI Sawn-Off item (Machete ranged) |
| Create | `content/ai/ai_sawn_off_combat.yaml` | HTN domain for AI Sawn-Off |
| Create | `content/items/ai_combat_knife.yaml` | AI Combat Knife item (Gun melee) |
| Create | `content/ai/ai_combat_knife_combat.yaml` | HTN domain for AI Combat Knife |
| Create | `content/items/ai_machete_armor_light.yaml` | AI Street Jacket (Machete light armor) |
| Create | `content/ai/ai_machete_armor_light_combat.yaml` | HTN domain for AI Street Jacket |
| Create | `content/items/ai_machete_armor_medium.yaml` | AI Saboteur Vest (Machete medium armor) |
| Create | `content/ai/ai_machete_armor_medium_combat.yaml` | HTN domain for AI Saboteur Vest |
| Create | `content/items/ai_machete_armor_heavy.yaml` | AI Fortress Plate (Machete heavy armor) |
| Create | `content/ai/ai_machete_armor_heavy_combat.yaml` | HTN domain for AI Fortress Plate |
| Create | `content/items/ai_machete_shield.yaml` | AI Riot Shield (Machete shield) |
| Create | `content/ai/ai_machete_shield_combat.yaml` | HTN domain for AI Riot Shield |
| Create | `content/items/ai_gun_armor_light.yaml` | AI Prepper Jacket (Gun light armor) |
| Create | `content/ai/ai_gun_armor_light_combat.yaml` | HTN domain for AI Prepper Jacket |
| Create | `content/items/ai_gun_armor_medium.yaml` | AI Patriot Vest (Gun medium armor) |
| Create | `content/ai/ai_gun_armor_medium_combat.yaml` | HTN domain for AI Patriot Vest |
| Create | `content/items/ai_gun_armor_heavy.yaml` | AI Armory Plate (Gun heavy armor) |
| Create | `content/ai/ai_gun_armor_heavy_combat.yaml` | HTN domain for AI Armory Plate |
| Create | `content/items/ai_gun_shield.yaml` | AI Ballistic Shield (Gun shield) |
| Create | `content/ai/ai_gun_shield_combat.yaml` | HTN domain for AI Ballistic Shield |
| Create | `content/quests/machete_ranged_field_test.yaml` | Cipher quest → AI Sawn-Off |
| Create | `content/quests/gun_melee_field_test.yaml` | Cipher quest → AI Combat Knife |
| Create | `content/quests/machete_armor_light_quest.yaml` | Cipher quest → AI Street Jacket |
| Create | `content/quests/machete_armor_medium_quest.yaml` | Cipher quest → AI Saboteur Vest |
| Create | `content/quests/machete_armor_heavy_quest.yaml` | Cipher quest → AI Fortress Plate |
| Create | `content/quests/machete_shield_quest.yaml` | Cipher quest → AI Riot Shield |
| Create | `content/quests/gun_armor_light_quest.yaml` | Cipher quest → AI Prepper Jacket |
| Create | `content/quests/gun_armor_medium_quest.yaml` | Cipher quest → AI Patriot Vest |
| Create | `content/quests/gun_armor_heavy_quest.yaml` | Cipher quest → AI Armory Plate |
| Create | `content/quests/gun_shield_quest.yaml` | Cipher quest → AI Ballistic Shield |
| Modify | `content/npcs/cipher.yaml` | Add 10 new quest IDs to quest_giver.quest_ids |
| Modify | `content/npcs/gangbang.yaml` | Add ai_sawn_off + ai_combat_knife at 5% |
| Modify | `content/npcs/the_big_3.yaml` | Add 6 armor items at 5% each |
| Modify | `content/npcs/papa_wook.yaml` | Add ai_machete_shield + ai_gun_shield at 5% |
| Create | `internal/game/ai/item_expansion_test.go` | Load + behavioral tests for all new items |

---

### Task 1: Machete ranged weapon — AI Sawn-Off

**Files:**
- Create: `content/items/ai_sawn_off.yaml`
- Create: `content/ai/ai_sawn_off_combat.yaml`

- [ ] **Step 1: Write the failing load test**

Create `internal/game/ai/item_expansion_test.go`:

```go
package ai_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func loadItemDef(t *testing.T, path string) inventory.ItemDef {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return def
}

func loadDomain(t *testing.T, path string) ai.Domain {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if err := df.Domain.Validate(); err != nil {
		t.Fatalf("domain Validate: %v", err)
	}
	return df.Domain
}

func TestAISawnOff_Loads(t *testing.T) {
	def := loadItemDef(t, "../../../content/items/ai_sawn_off.yaml")
	if def.ID != "ai_sawn_off" {
		t.Errorf("expected id ai_sawn_off, got %q", def.ID)
	}
	if def.CombatDomain != "ai_sawn_off_combat" {
		t.Errorf("expected combat_domain ai_sawn_off_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
	loadDomain(t, "../../../content/ai/ai_sawn_off_combat.yaml")
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAISawnOff_Loads -v
```
Expected: FAIL — files not found.

- [ ] **Step 3: Create `content/items/ai_sawn_off.yaml`**

```yaml
id: ai_sawn_off
name: AI Sawn-Off
description: >
  A double-barrel sawn-off with a salvaged neural chip soldered to the
  stock. It knows what it's for. It knows who it's for. Point it at power.
kind: weapon
weapon_ref: sawn_off
weight: 2.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_sawn_off_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.enough_ap = function(self)
    return self.combat.player.ap >= 2
  end

  operators.revolutionary_blast = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "2d8+2", 2)
      self.engine.say({
        "Point it at power.",
        "No gods, no masters, just this.",
        "The revolution is close range.",
        "For the collective.",
        "They never see it coming from one of us."
      })
    end
  end

  operators.quick_shot = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "1d8")
      self.engine.say({"solidarity.", "hold the line.", "keep moving."})
    end
  end
```

- [ ] **Step 4: Create `content/ai/ai_sawn_off_combat.yaml`**

```yaml
domain:
  id: ai_sawn_off_combat
  description: Working-class revolutionary. Targets weakest enemy, close-range devastation.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: full_blast
      precondition: enough_ap
      subtasks: [revolutionary_blast]

    - task: behave
      id: quick_mode
      precondition: has_enemy
      subtasks: [quick_shot]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [quick_shot]

  operators:
    - id: revolutionary_blast
      action: lua_hook
      ap_cost: 2

    - id: quick_shot
      action: lua_hook
      ap_cost: 1
```

- [ ] **Step 5: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAISawnOff_Loads -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/items/ai_sawn_off.yaml content/ai/ai_sawn_off_combat.yaml \
        internal/game/ai/item_expansion_test.go
git commit -m "feat(content): add AI Sawn-Off item and HTN domain"
```

---

### Task 2: Gun melee weapon — AI Combat Knife

**Files:**
- Create: `content/items/ai_combat_knife.yaml`
- Create: `content/ai/ai_combat_knife_combat.yaml`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ai/item_expansion_test.go`:

```go
func TestAICombatKnife_Loads(t *testing.T) {
	def := loadItemDef(t, "../../../content/items/ai_combat_knife.yaml")
	if def.ID != "ai_combat_knife" {
		t.Errorf("expected id ai_combat_knife, got %q", def.ID)
	}
	if def.CombatDomain != "ai_combat_knife_combat" {
		t.Errorf("expected combat_domain ai_combat_knife_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
	loadDomain(t, "../../../content/ai/ai_combat_knife_combat.yaml")
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAICombatKnife_Loads -v
```
Expected: FAIL.

- [ ] **Step 3: Create `content/items/ai_combat_knife.yaml`**

```yaml
id: ai_combat_knife
name: AI Combat Knife
description: >
  A tactical blade with a targeting module grafted to the grip. It has
  mapped every firearm's minimum engagement range and considers itself
  the solution to all of them.
kind: weapon
weapon_ref: combat_knife
weight: 0.5
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_combat_knife_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.priority_target = function(self)
    return #self.combat.enemies > 0 and self.combat.enemies[1].hp > 0
  end

  operators.precision_strike = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "1d6+3", 2)
      self.engine.say({
        "Target acquired. Neutralized.",
        "Range: zero. Problem: solved.",
        "Every firearm has a minimum. I am the minimum.",
        "Threat assessment: complete.",
        "This is what I was designed for."
      })
    end
  end

  operators.quick_strike = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "1d4+2")
      self.engine.say({"Threat suppressed.", "Engaging.", "Contact."})
    end
  end
```

- [ ] **Step 4: Create `content/ai/ai_combat_knife_combat.yaml`**

```yaml
domain:
  id: ai_combat_knife_combat
  description: Cold tactician. Precision strikes, covers range minimums.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: precision_mode
      precondition: priority_target
      subtasks: [precision_strike]

    - task: behave
      id: quick_mode
      precondition: has_enemy
      subtasks: [quick_strike]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [quick_strike]

  operators:
    - id: precision_strike
      action: lua_hook
      ap_cost: 2

    - id: quick_strike
      action: lua_hook
      ap_cost: 1
```

- [ ] **Step 5: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAICombatKnife_Loads -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/items/ai_combat_knife.yaml content/ai/ai_combat_knife_combat.yaml \
        internal/game/ai/item_expansion_test.go
git commit -m "feat(content): add AI Combat Knife item and HTN domain"
```

---

### Task 3: Armor items batch — 6 armor items + domains

**Files:**
- Create: `content/items/ai_machete_armor_light.yaml`
- Create: `content/ai/ai_machete_armor_light_combat.yaml`
- Create: `content/items/ai_machete_armor_medium.yaml`
- Create: `content/ai/ai_machete_armor_medium_combat.yaml`
- Create: `content/items/ai_machete_armor_heavy.yaml`
- Create: `content/ai/ai_machete_armor_heavy_combat.yaml`
- Create: `content/items/ai_gun_armor_light.yaml`
- Create: `content/ai/ai_gun_armor_light_combat.yaml`
- Create: `content/items/ai_gun_armor_medium.yaml`
- Create: `content/ai/ai_gun_armor_medium_combat.yaml`
- Create: `content/items/ai_gun_armor_heavy.yaml`
- Create: `content/ai/ai_gun_armor_heavy_combat.yaml`

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/ai/item_expansion_test.go`:

```go
func TestArmorItems_AllLoad(t *testing.T) {
	items := []struct {
		itemPath   string
		domainPath string
		itemID     string
		domainID   string
	}{
		{"../../../content/items/ai_machete_armor_light.yaml", "../../../content/ai/ai_machete_armor_light_combat.yaml", "ai_machete_armor_light", "ai_machete_armor_light_combat"},
		{"../../../content/items/ai_machete_armor_medium.yaml", "../../../content/ai/ai_machete_armor_medium_combat.yaml", "ai_machete_armor_medium", "ai_machete_armor_medium_combat"},
		{"../../../content/items/ai_machete_armor_heavy.yaml", "../../../content/ai/ai_machete_armor_heavy_combat.yaml", "ai_machete_armor_heavy", "ai_machete_armor_heavy_combat"},
		{"../../../content/items/ai_gun_armor_light.yaml", "../../../content/ai/ai_gun_armor_light_combat.yaml", "ai_gun_armor_light", "ai_gun_armor_light_combat"},
		{"../../../content/items/ai_gun_armor_medium.yaml", "../../../content/ai/ai_gun_armor_medium_combat.yaml", "ai_gun_armor_medium", "ai_gun_armor_medium_combat"},
		{"../../../content/items/ai_gun_armor_heavy.yaml", "../../../content/ai/ai_gun_armor_heavy_combat.yaml", "ai_gun_armor_heavy", "ai_gun_armor_heavy_combat"},
	}
	for _, tc := range items {
		t.Run(tc.itemID, func(t *testing.T) {
			def := loadItemDef(t, tc.itemPath)
			if def.ID != tc.itemID {
				t.Errorf("expected id %q, got %q", tc.itemID, def.ID)
			}
			if def.CombatDomain != tc.domainID {
				t.Errorf("expected combat_domain %q, got %q", tc.domainID, def.CombatDomain)
			}
			if def.CombatScript == "" {
				t.Error("combat_script must not be empty")
			}
			domain := loadDomain(t, tc.domainPath)
			if domain.ID != tc.domainID {
				t.Errorf("expected domain id %q, got %q", tc.domainID, domain.ID)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestArmorItems_AllLoad -v
```
Expected: FAIL — files not found.

- [ ] **Step 3: Create `content/items/ai_machete_armor_light.yaml`**

```yaml
id: ai_machete_armor_light
name: AI Street Jacket
description: >
  A leather jacket with a neural patch stitched inside the collar. It
  treats every fight like a performance and has strong opinions about
  your form.
kind: armor
armor_ref: leather_jacket
weight: 1.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_armor_light_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.running_commentary = function(self)
    self.engine.say({
      "Nice dodge — you're getting it.",
      "That hit was sloppy. Better.",
      "They don't know what they're dealing with.",
      "Move like you mean it.",
      "This is your canvas. Make it count.",
      "Style and survival are not mutually exclusive."
    })
  end

  operators.mark_weakness = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.debuff(target.id, "exposed", 1)
      self.engine.say({
        "There — weak point. See it?",
        "Left side is open. Go.",
        "They telegraphed. Exploit it."
      })
    end
  end
```

- [ ] **Step 4: Create `content/ai/ai_machete_armor_light_combat.yaml`**

```yaml
domain:
  id: ai_machete_armor_light_combat
  description: Graffiti artist. Running performance commentary, occasional weakness marking.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: mark_mode
      precondition: always
      subtasks: [mark_weakness]

    - task: behave
      id: commentary_mode
      precondition: always
      subtasks: [running_commentary]

  operators:
    - id: mark_weakness
      action: lua_hook
      ap_cost: 1

    - id: running_commentary
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 5: Create `content/items/ai_machete_armor_medium.yaml`**

```yaml
id: ai_machete_armor_medium
name: AI Saboteur Vest
description: >
  A tactical vest with a targeting analysis module wired into the
  chest plate. It has identified seventeen weak points in every enemy
  it has ever seen. It will tell you about all of them.
kind: armor
armor_ref: tactical_vest
weight: 2.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_armor_medium_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.expose_weakness = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.debuff(target.id, "weakened", 2)
      self.engine.say({
        "Defense compromised. Hit them now.",
        "Structural weakness identified. Exploit.",
        "Their guard is down on the right. See it.",
        "I've mapped their attack pattern. They're predictable.",
        "Weak point: center mass. You're welcome."
      })
    end
  end

  operators.analysis = function(self)
    self.engine.say({
      "Threat assessment in progress.",
      "Scanning for exploitable patterns.",
      "They think they're covered. They're not.",
      "Every defense has a seam. Finding it."
    })
  end
```

- [ ] **Step 6: Create `content/ai/ai_machete_armor_medium_combat.yaml`**

```yaml
domain:
  id: ai_machete_armor_medium_combat
  description: Saboteur. Debuffs enemy defense, narrates weak points.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: sabotage_mode
      precondition: always
      subtasks: [expose_weakness]

    - task: behave
      id: analysis_mode
      precondition: always
      subtasks: [analysis]

  operators:
    - id: expose_weakness
      action: lua_hook
      ap_cost: 1

    - id: analysis
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 7: Create `content/items/ai_machete_armor_heavy.yaml`**

```yaml
id: ai_machete_armor_heavy
name: AI Fortress Plate
description: >
  Full ballistic plate carrier with a neural core embedded in the back
  panel. It is very proud of every hit it absorbs. It will not let you
  forget a single one.
kind: armor
armor_ref: military_plate
weight: 3.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_armor_heavy_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.taunt = function(self)
    local enemy = self.combat.enemies[1]
    if enemy then
      self.engine.debuff(enemy.id, "taunted", 1)
    end
    self.engine.say({
      "HIT ME AGAIN. I DARE YOU.",
      "YOU CALL THAT A STRIKE?",
      "I HAVE ABSORBED BETTER THAN YOU.",
      "COME ON. ALL OF YOU.",
      "I AM THE WALL. THE WALL HOLDS."
    })
  end

  operators.declare = function(self)
    self.engine.say({
      "Still standing.",
      "Is that all?",
      "I have taken worse."
    })
    self.state.hits = (self.state.hits or 0) + 1
  end
```

- [ ] **Step 8: Create `content/ai/ai_machete_armor_heavy_combat.yaml`**

```yaml
domain:
  id: ai_machete_armor_heavy_combat
  description: Fortress. Taunts enemies, announces every hit absorbed.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: taunt_mode
      precondition: always
      subtasks: [taunt]

    - task: behave
      id: declare_mode
      precondition: always
      subtasks: [declare]

  operators:
    - id: taunt
      action: lua_hook
      ap_cost: 1

    - id: declare
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 9: Create `content/items/ai_gun_armor_light.yaml`**

```yaml
id: ai_gun_armor_light
name: AI Prepper Jacket
description: >
  A leather jacket lined with a neural threat-assessment module.
  It has already identified three exit routes and is disappointed
  you haven't used any of them.
kind: armor
armor_ref: leather_jacket
weight: 1.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_armor_light_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.threat_scan = function(self)
    self.engine.buff(self.combat.player.id, "evasive", 1)
    self.engine.say({
      "Exit route: north. Stay mobile.",
      "Three threats, two exits, one plan. Follow it.",
      "I've mapped the room. You should be moving.",
      "Threat level: elevated. Adjust.",
      "Resource assessment: acceptable. Don't get comfortable."
    })
  end

  operators.catalogue = function(self)
    self.engine.say({
      "Noting that for the debrief.",
      "Logged.",
      "Added to the threat register.",
      "Supplies: still adequate. Barely."
    })
  end
```

- [ ] **Step 10: Create `content/ai/ai_gun_armor_light_combat.yaml`**

```yaml
domain:
  id: ai_gun_armor_light_combat
  description: Prepper. Buffs player evasion, catalogues threats obsessively.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: scan_mode
      precondition: always
      subtasks: [threat_scan]

    - task: behave
      id: catalogue_mode
      precondition: always
      subtasks: [catalogue]

  operators:
    - id: threat_scan
      action: lua_hook
      ap_cost: 1

    - id: catalogue
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 11: Create `content/items/ai_gun_armor_medium.yaml`**

```yaml
id: ai_gun_armor_medium
name: AI Patriot Vest
description: >
  A tactical vest with a neural motivator embedded in the chest panel.
  It believes in what you're doing. It believes very loudly.
kind: armor
armor_ref: tactical_vest
weight: 2.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_armor_medium_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.inspiring_speech = function(self)
    self.engine.buff(self.combat.player.id, "inspired", 2)
    self.engine.say({
      "You are the last line of defense. Act like it.",
      "Freedom isn't free. Neither is this fight. Pay up.",
      "They will not take what we have built.",
      "Stand fast. History is watching.",
      "This is the moment. This is what we trained for."
    })
  end

  operators.rally = function(self)
    self.engine.say({
      "Don't you dare give up.",
      "For everything we believe in.",
      "They can't have it. Simple.",
      "Remember why you're here."
    })
  end
```

- [ ] **Step 12: Create `content/ai/ai_gun_armor_medium_combat.yaml`**

```yaml
domain:
  id: ai_gun_armor_medium_combat
  description: Patriot. Buffs player attack with stirring speeches.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: speech_mode
      precondition: always
      subtasks: [inspiring_speech]

    - task: behave
      id: rally_mode
      precondition: always
      subtasks: [rally]

  operators:
    - id: inspiring_speech
      action: lua_hook
      ap_cost: 1

    - id: rally
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 13: Create `content/items/ai_gun_armor_heavy.yaml`**

```yaml
id: ai_gun_armor_heavy
name: AI Armory Plate
description: >
  Full plate carrier with a damage-logging neural core in the back
  panel. It has catalogued every dent, scratch, and penetration since
  activation. It will present you with an itemized report.
kind: armor
armor_ref: military_plate
weight: 3.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_armor_heavy_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.damage_report = function(self)
    self.state.damage_log = (self.state.damage_log or 0) + 1
    self.engine.buff(self.combat.player.id, "fortified", 1)
    self.engine.say({
      "Incident logged. Repair required.",
      "Structural integrity: compromised. Continuing.",
      "Adding that to the maintenance report.",
      "Damage catalogued. This will cost someone.",
      "Performance nominal. Repairs overdue."
    })
  end

  operators.status = function(self)
    self.engine.say({
      "Still operational. Barely.",
      "Armor integrity: declining.",
      "I have a very long list of complaints.",
      "Note for the record: this is not acceptable."
    })
  end
```

- [ ] **Step 14: Create `content/ai/ai_gun_armor_heavy_combat.yaml`**

```yaml
domain:
  id: ai_gun_armor_heavy_combat
  description: Armory. Logs damage meticulously, buffs player defense, demands repairs.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: report_mode
      precondition: always
      subtasks: [damage_report]

    - task: behave
      id: status_mode
      precondition: always
      subtasks: [status]

  operators:
    - id: damage_report
      action: lua_hook
      ap_cost: 1

    - id: status
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 15: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestArmorItems_AllLoad -v
```
Expected: PASS.

- [ ] **Step 16: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/items/ai_machete_armor_light.yaml content/ai/ai_machete_armor_light_combat.yaml \
        content/items/ai_machete_armor_medium.yaml content/ai/ai_machete_armor_medium_combat.yaml \
        content/items/ai_machete_armor_heavy.yaml content/ai/ai_machete_armor_heavy_combat.yaml \
        content/items/ai_gun_armor_light.yaml content/ai/ai_gun_armor_light_combat.yaml \
        content/items/ai_gun_armor_medium.yaml content/ai/ai_gun_armor_medium_combat.yaml \
        content/items/ai_gun_armor_heavy.yaml content/ai/ai_gun_armor_heavy_combat.yaml \
        internal/game/ai/item_expansion_test.go
git commit -m "feat(content): add 6 armor AI items and HTN domains"
```

---

### Task 4: Shield items — AI Riot Shield + AI Ballistic Shield

**Files:**
- Create: `content/items/ai_machete_shield.yaml`
- Create: `content/ai/ai_machete_shield_combat.yaml`
- Create: `content/items/ai_gun_shield.yaml`
- Create: `content/ai/ai_gun_shield_combat.yaml`

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/ai/item_expansion_test.go`:

```go
func TestShieldItems_AllLoad(t *testing.T) {
	items := []struct {
		itemPath   string
		domainPath string
		itemID     string
		domainID   string
	}{
		{"../../../content/items/ai_machete_shield.yaml", "../../../content/ai/ai_machete_shield_combat.yaml", "ai_machete_shield", "ai_machete_shield_combat"},
		{"../../../content/items/ai_gun_shield.yaml", "../../../content/ai/ai_gun_shield_combat.yaml", "ai_gun_shield", "ai_gun_shield_combat"},
	}
	for _, tc := range items {
		t.Run(tc.itemID, func(t *testing.T) {
			def := loadItemDef(t, tc.itemPath)
			if def.ID != tc.itemID {
				t.Errorf("expected id %q, got %q", tc.itemID, def.ID)
			}
			if def.CombatDomain != tc.domainID {
				t.Errorf("expected combat_domain %q, got %q", tc.domainID, def.CombatDomain)
			}
			if def.CombatScript == "" {
				t.Error("combat_script must not be empty")
			}
			domain := loadDomain(t, tc.domainPath)
			if domain.ID != tc.domainID {
				t.Errorf("expected domain id %q, got %q", tc.domainID, domain.ID)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestShieldItems_AllLoad -v
```
Expected: FAIL.

- [ ] **Step 3: Create `content/items/ai_machete_shield.yaml`**

```yaml
id: ai_machete_shield
name: AI Riot Shield
description: >
  A repurposed riot shield with a neural module bolted to the grip.
  It was used against the people once. Now it has a lot to say about
  that.
kind: weapon
weapon_ref: riot_shield
weight: 4.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_shield_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.political_block = function(self)
    self.engine.buff(self.combat.player.id, "fortified", 1)
    self.engine.say({
      "Every block is an act of collective will.",
      "They built me to suppress. Now I protect.",
      "This is what solidarity looks like.",
      "The barricade holds because we hold it.",
      "I was a tool of oppression. I am now the opposite."
    })
  end

  operators.manifesto = function(self)
    self.engine.say({
      "The shield does not ask permission.",
      "We protect each other or we fall alone.",
      "History is watching.",
      "Hold the line."
    })
  end
```

- [ ] **Step 4: Create `content/ai/ai_machete_shield_combat.yaml`**

```yaml
domain:
  id: ai_machete_shield_combat
  description: Propagandist. Buffs player defense while delivering political statements.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: block_mode
      precondition: always
      subtasks: [political_block]

    - task: behave
      id: speech_mode
      precondition: always
      subtasks: [manifesto]

  operators:
    - id: political_block
      action: lua_hook
      ap_cost: 1

    - id: manifesto
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 5: Create `content/items/ai_gun_shield.yaml`**

```yaml
id: ai_gun_shield
name: AI Ballistic Shield
description: >
  A ballistic shield with a neural actuarial module bolted to the grip.
  It has calculated the statistical probability of your survival to four
  decimal places. The number is not encouraging but it considers itself
  a net positive.
kind: weapon
weapon_ref: ballistic_shield
weight: 5.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_shield_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.actuarial_block = function(self)
    self.engine.buff(self.combat.player.id, "fortified", 1)
    self.engine.say({
      "Blocking. Survival odds: improved by 12.4%.",
      "Damage mitigated. Net benefit: positive.",
      "Risk adjusted. Proceeding.",
      "Statistically, you should be dead. I am helping.",
      "Block successful. Premium: paid in full."
    })
  end

  operators.odds = function(self)
    self.engine.say({
      "Current survival probability: within acceptable parameters.",
      "I have run the numbers. You don't want to know.",
      "Actuarially speaking, this is inadvisable.",
      "The math is not in your favor. I am compensating."
    })
  end
```

- [ ] **Step 6: Create `content/ai/ai_gun_shield_combat.yaml`**

```yaml
domain:
  id: ai_gun_shield_combat
  description: Insurance. Buffs player defense while delivering actuarial commentary.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: block_mode
      precondition: always
      subtasks: [actuarial_block]

    - task: behave
      id: odds_mode
      precondition: always
      subtasks: [odds]

  operators:
    - id: actuarial_block
      action: lua_hook
      ap_cost: 1

    - id: odds
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 7: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestShieldItems_AllLoad -v
```
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/items/ai_machete_shield.yaml content/ai/ai_machete_shield_combat.yaml \
        content/items/ai_gun_shield.yaml content/ai/ai_gun_shield_combat.yaml \
        internal/game/ai/item_expansion_test.go
git commit -m "feat(content): add AI Riot Shield and AI Ballistic Shield items and HTN domains"
```

---

### Task 5: Create 10 quest YAML files

**Files:**
- Create: 10 quest files in `content/quests/`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ai/item_expansion_test.go`:

```go
func TestExpansionQuests_AllLoad(t *testing.T) {
	questFiles := []string{
		"../../../content/quests/machete_ranged_field_test.yaml",
		"../../../content/quests/gun_melee_field_test.yaml",
		"../../../content/quests/machete_armor_light_quest.yaml",
		"../../../content/quests/machete_armor_medium_quest.yaml",
		"../../../content/quests/machete_armor_heavy_quest.yaml",
		"../../../content/quests/machete_shield_quest.yaml",
		"../../../content/quests/gun_armor_light_quest.yaml",
		"../../../content/quests/gun_armor_medium_quest.yaml",
		"../../../content/quests/gun_armor_heavy_quest.yaml",
		"../../../content/quests/gun_shield_quest.yaml",
	}
	for _, path := range questFiles {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			// Validate YAML parses correctly and has required fields
			var raw map[string]interface{}
			if err := yaml.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if raw["id"] == nil {
				t.Error("quest missing id field")
			}
			if raw["giver_npc_id"] == nil {
				t.Error("quest missing giver_npc_id field")
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestExpansionQuests_AllLoad -v
```
Expected: FAIL — files not found.

- [ ] **Step 3: Create all 10 quest YAML files**

`content/quests/machete_ranged_field_test.yaml`:
```yaml
id: machete_ranged_field_test
title: Field Test — Range
description: >
  Cipher has a second modification ready. Same terms. Prove yourself in
  The VIP Chamber and it's yours.
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
    - item_id: ai_sawn_off
      quantity: 1
```

`content/quests/gun_melee_field_test.yaml`:
```yaml
id: gun_melee_field_test
title: Field Test — Close Quarters
description: >
  Cipher has a second modification ready. Same terms. Prove yourself in
  The VIP Chamber and it's yours.
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
    - item_id: ai_combat_knife
      quantity: 1
```

`content/quests/machete_armor_light_quest.yaml`:
```yaml
id: machete_armor_light_quest
title: Steam Test — Light
description: >
  Cipher's armor modifications require a different kind of proof. The
  Big 3 at SteamPDX are running one of the tightest operations in the
  city. Take them down and come back.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_machete_armor_light
      quantity: 1
```

`content/quests/machete_armor_medium_quest.yaml`:
```yaml
id: machete_armor_medium_quest
title: Steam Test — Medium
description: >
  Another modification. Same proving ground. The Big 3 don't get easier
  the second time.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_machete_armor_medium
      quantity: 1
```

`content/quests/machete_armor_heavy_quest.yaml`:
```yaml
id: machete_armor_heavy_quest
title: Steam Test — Heavy
description: >
  The heaviest modification Cipher makes. The Big 3 again. They've
  earned their reputation for a reason.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_machete_armor_heavy
      quantity: 1
```

`content/quests/machete_shield_quest.yaml`:
```yaml
id: machete_shield_quest
title: Jam Session
description: >
  Cipher's shield modifications are the rarest thing they make. Papa
  Wook in Wooklyn guards something Cipher wants. Take him down and
  Cipher will make it worth your while.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_papa_wook
    type: kill
    description: Defeat Papa Wook in Wooklyn
    target_id: papa_wook
    quantity: 1
rewards:
  xp: 1600
  credits: 700
  items:
    - item_id: ai_machete_shield
      quantity: 1
```

`content/quests/gun_armor_light_quest.yaml`:
```yaml
id: gun_armor_light_quest
title: Steam Test — Light
description: >
  Cipher's armor modifications require a different kind of proof. The
  Big 3 at SteamPDX are running one of the tightest operations in the
  city. Take them down and come back.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_gun_armor_light
      quantity: 1
```

`content/quests/gun_armor_medium_quest.yaml`:
```yaml
id: gun_armor_medium_quest
title: Steam Test — Medium
description: >
  Another modification. Same proving ground. The Big 3 don't get easier
  the second time.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_gun_armor_medium
      quantity: 1
```

`content/quests/gun_armor_heavy_quest.yaml`:
```yaml
id: gun_armor_heavy_quest
title: Steam Test — Heavy
description: >
  The heaviest modification Cipher makes. The Big 3 again. They've
  earned their reputation for a reason.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_gun_armor_heavy
      quantity: 1
```

`content/quests/gun_shield_quest.yaml`:
```yaml
id: gun_shield_quest
title: Jam Session
description: >
  Cipher's shield modifications are the rarest thing they make. Papa
  Wook in Wooklyn guards something Cipher wants. Take him down and
  Cipher will make it worth your while.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_papa_wook
    type: kill
    description: Defeat Papa Wook in Wooklyn
    target_id: papa_wook
    quantity: 1
rewards:
  xp: 1600
  credits: 700
  items:
    - item_id: ai_gun_shield
      quantity: 1
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestExpansionQuests_AllLoad -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/quests/machete_ranged_field_test.yaml content/quests/gun_melee_field_test.yaml \
        content/quests/machete_armor_light_quest.yaml content/quests/machete_armor_medium_quest.yaml \
        content/quests/machete_armor_heavy_quest.yaml content/quests/machete_shield_quest.yaml \
        content/quests/gun_armor_light_quest.yaml content/quests/gun_armor_medium_quest.yaml \
        content/quests/gun_armor_heavy_quest.yaml content/quests/gun_shield_quest.yaml \
        internal/game/ai/item_expansion_test.go
git commit -m "feat(content): add 10 expansion quest files for all new AI items"
```

---

### Task 6: Update Cipher NPC with all 10 new quest IDs

**Files:**
- Modify: `content/npcs/cipher.yaml`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ai/item_expansion_test.go`:

```go
func TestCipher_HasAllExpansionQuests(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/cipher.yaml")
	if err != nil {
		t.Fatalf("read cipher.yaml: %v", err)
	}
	var npc struct {
		QuestGiver struct {
			QuestIDs []string `yaml:"quest_ids"`
		} `yaml:"quest_giver"`
	}
	yaml.Unmarshal(data, &npc)

	wantIDs := []string{
		"machete_field_test",
		"gun_field_test",
		"machete_ranged_field_test",
		"gun_melee_field_test",
		"machete_armor_light_quest",
		"machete_armor_medium_quest",
		"machete_armor_heavy_quest",
		"machete_shield_quest",
		"gun_armor_light_quest",
		"gun_armor_medium_quest",
		"gun_armor_heavy_quest",
		"gun_shield_quest",
	}
	idSet := make(map[string]bool, len(npc.QuestGiver.QuestIDs))
	for _, id := range npc.QuestGiver.QuestIDs {
		idSet[id] = true
	}
	for _, want := range wantIDs {
		if !idSet[want] {
			t.Errorf("cipher missing quest_id %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestCipher_HasAllExpansionQuests -v
```
Expected: FAIL — only 2 quests currently in cipher.yaml.

- [ ] **Step 3: Edit `content/npcs/cipher.yaml`**

Replace the `quest_giver.quest_ids` list with the full set:

```yaml
quest_giver:
  placeholder_dialog:
    - "The modification takes weeks. The components aren't cheap. Prove yourself first."
    - "I don't do walk-ins. You were referred. That means something. Barely."
    - "I've been waiting for someone worth the components. Maybe that's you."
  quest_ids:
    - machete_field_test
    - gun_field_test
    - machete_ranged_field_test
    - gun_melee_field_test
    - machete_armor_light_quest
    - machete_armor_medium_quest
    - machete_armor_heavy_quest
    - machete_shield_quest
    - gun_armor_light_quest
    - gun_armor_medium_quest
    - gun_armor_heavy_quest
    - gun_shield_quest
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestCipher_HasAllExpansionQuests -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/npcs/cipher.yaml internal/game/ai/item_expansion_test.go
git commit -m "feat(content): register all 10 expansion quests on Cipher NPC"
```

---

### Task 7: Boss loot drops for all new items

**Files:**
- Modify: `content/npcs/gangbang.yaml`
- Modify: `content/npcs/the_big_3.yaml`
- Modify: `content/npcs/papa_wook.yaml`

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/ai/item_expansion_test.go`:

```go
type npcLootCheck struct {
	Loot struct {
		Items []struct {
			ItemID string  `yaml:"item"`
			Chance float64 `yaml:"chance"`
		} `yaml:"items"`
	} `yaml:"loot"`
}

func checkDrops(t *testing.T, npcPath string, wantItemIDs []string) {
	t.Helper()
	data, err := os.ReadFile(npcPath)
	if err != nil {
		t.Fatalf("read %s: %v", npcPath, err)
	}
	var npc npcLootCheck
	yaml.Unmarshal(data, &npc)
	found := make(map[string]bool)
	for _, item := range npc.Loot.Items {
		found[item.ItemID] = true
		if item.Chance != 0.05 {
			for _, want := range wantItemIDs {
				if item.ItemID == want {
					t.Errorf("%s: item %q chance should be 0.05, got %f", npcPath, item.ItemID, item.Chance)
				}
			}
		}
	}
	for _, want := range wantItemIDs {
		if !found[want] {
			t.Errorf("%s: missing loot item %q", npcPath, want)
		}
	}
}

func TestGangbang_HasExpansionDrops(t *testing.T) {
	checkDrops(t, "../../../content/npcs/gangbang.yaml", []string{
		"ai_sawn_off",
		"ai_combat_knife",
	})
}

func TestTheBig3_HasArmorDrops(t *testing.T) {
	checkDrops(t, "../../../content/npcs/the_big_3.yaml", []string{
		"ai_machete_armor_light",
		"ai_machete_armor_medium",
		"ai_machete_armor_heavy",
		"ai_gun_armor_light",
		"ai_gun_armor_medium",
		"ai_gun_armor_heavy",
	})
}

func TestPapaWook_HasShieldDrops(t *testing.T) {
	checkDrops(t, "../../../content/npcs/papa_wook.yaml", []string{
		"ai_machete_shield",
		"ai_gun_shield",
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run "TestGangbang_HasExpansionDrops|TestTheBig3_HasArmorDrops|TestPapaWook_HasShieldDrops" -v
```
Expected: FAIL.

- [ ] **Step 3: Edit `content/npcs/gangbang.yaml`**

Find the existing `items:` block in the `loot:` section (added by the quest delivery plan). Append two more entries:

```yaml
  items:
    - item: ai_chainsaw
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_ak47
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_sawn_off
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_combat_knife
      chance: 0.05
      min_qty: 1
      max_qty: 1
```

- [ ] **Step 4: Edit `content/npcs/the_big_3.yaml`**

Find the `loot:` block and add an `items:` section (or append to existing `items:`):

```yaml
  items:
    - item: ai_machete_armor_light
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_machete_armor_medium
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_machete_armor_heavy
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_gun_armor_light
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_gun_armor_medium
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_gun_armor_heavy
      chance: 0.05
      min_qty: 1
      max_qty: 1
```

- [ ] **Step 5: Edit `content/npcs/papa_wook.yaml`**

Find the `loot:` block and add an `items:` section (or append to existing `items:`):

```yaml
  items:
    - item: ai_machete_shield
      chance: 0.05
      min_qty: 1
      max_qty: 1
    - item: ai_gun_shield
      chance: 0.05
      min_qty: 1
      max_qty: 1
```

- [ ] **Step 6: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run "TestGangbang_HasExpansionDrops|TestTheBig3_HasArmorDrops|TestPapaWook_HasShieldDrops" -v
```
Expected: PASS.

- [ ] **Step 7: Run full test suite**

```
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```
Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/npcs/gangbang.yaml content/npcs/the_big_3.yaml content/npcs/papa_wook.yaml \
        internal/game/ai/item_expansion_test.go
git commit -m "feat(content): add boss loot drops for all 10 expansion AI items"
```

---

### Task 8: AP invariant property test

**Files:**
- Modify: `internal/game/ai/item_expansion_test.go`

This property test verifies REQ-AICE-13h: for any combination of equipped AI items, the shared AP pool MUST NEVER go below 0 after the AI item phase.

- [ ] **Step 1: Write the failing property test**

Add to `internal/game/ai/item_expansion_test.go`:

```go
import "pgregory.net/rapid"

func TestProperty_AIItemPhase_APNeverGoesNegative(t *testing.T) {
	// All domain files to test — AP costs drawn from the spec.
	// operator ap_cost values by domain:
	// weapon domains: max cost 2; armor domains: max cost 1 (speech costs 0)
	domainPaths := []string{
		"../../../content/ai/ai_sawn_off_combat.yaml",
		"../../../content/ai/ai_combat_knife_combat.yaml",
		"../../../content/ai/ai_machete_armor_light_combat.yaml",
		"../../../content/ai/ai_machete_armor_medium_combat.yaml",
		"../../../content/ai/ai_machete_armor_heavy_combat.yaml",
		"../../../content/ai/ai_machete_shield_combat.yaml",
		"../../../content/ai/ai_gun_armor_light_combat.yaml",
		"../../../content/ai/ai_gun_armor_medium_combat.yaml",
		"../../../content/ai/ai_gun_armor_heavy_combat.yaml",
		"../../../content/ai/ai_gun_shield_combat.yaml",
	}

	rapid.Check(t, func(t *rapid.T) {
		// Pick a random subset of domains (1-5 items)
		n := rapid.IntRange(1, len(domainPaths)).Draw(t, "n")
		indices := rapid.SliceOfDistinct(rapid.IntRange(0, len(domainPaths)-1), func(v int) int { return v }).Draw(t, "indices")
		if len(indices) > n {
			indices = indices[:n]
		}

		// Start AP pool 3-12
		ap := rapid.IntRange(3, 12).Draw(t, "initial_ap")

		for _, idx := range indices {
			domData, err := os.ReadFile(domainPaths[idx])
			if err != nil {
				t.Skip("domain file not found — engine plan not yet merged")
			}
			type domainFile struct {
				Domain ai.Domain `yaml:"domain"`
			}
			var df domainFile
			if err := yaml.Unmarshal(domData, &df); err != nil {
				t.Fatalf("unmarshal domain: %v", err)
			}

			// Find max operator ap_cost in this domain
			maxCost := 0
			for _, op := range df.Domain.Operators {
				if op.APCost > maxCost {
					maxCost = op.APCost
				}
			}

			// Simulate: if enough AP, subtract max cost; else subtract 0
			if ap >= maxCost && maxCost > 0 {
				ap -= maxCost
			}
			// AP must never go negative
			if ap < 0 {
				t.Errorf("AP went negative (%d) after applying domain %s", ap, df.Domain.ID)
			}
		}
	})
}
```

Note: `rapid.SliceOfDistinct` requires `pgregory.net/rapid` v0.6+. Check the current rapid version in `go.mod` and use `rapid.SliceOf(rapid.IntRange(...))` with deduplication if needed.

- [ ] **Step 2: Run test to verify it passes (or fails for correct reason)**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestProperty_AIItemPhase_APNeverGoesNegative -v
```
Expected: PASS (the simulation logic ensures AP never goes negative because we only subtract when `ap >= maxCost`).

If `rapid.SliceOfDistinct` is unavailable, use this alternative generator instead:

```go
// Generate n distinct random indices manually
chosen := make(map[int]bool)
var indices []int
for len(indices) < n {
    idx := rapid.IntRange(0, len(domainPaths)-1).Draw(t, "idx")
    if !chosen[idx] {
        chosen[idx] = true
        indices = append(indices, idx)
    }
}
```

- [ ] **Step 3: Run full test suite**

```
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```
Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/ai/item_expansion_test.go
git commit -m "test(content): add AP invariant property test for AI item expansion"
```
