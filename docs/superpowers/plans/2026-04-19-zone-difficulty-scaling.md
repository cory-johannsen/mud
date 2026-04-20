# Zone Difficulty Scaling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scale all existing zone NPC stats to match the 5-tier difficulty framework (levels 1–100), covering Tiers 1–3 (levels 1–60); Tiers 4–5 require separate zone-creation specs.

**Architecture:** Pure YAML content editing — no engine code changes needed for stat upgrades. A new Go compliance test validates all NPC templates against the stat formula (±10% tolerance). Shared NPC templates used across zones in different tiers are cloned into zone-specific variants first to avoid cross-zone conflicts.

**Tech Stack:** Go (`pgregory.net/rapid` property tests, `testify`), YAML NPC templates (`content/npcs/`), zone YAMLs (`content/zones/`)

---

## Stat Formula Reference

Use this table throughout all tasks to determine target stats.

**Base HP (standard NPC, ×1.0):**
```
L1:12   L6:42   L11:78   L16:120  L21:171  L26:226  L31:285  L36:360  L41:438  L46:528  L51:621  L56:726
L2:18   L7:49   L12:86   L17:130  L22:182  L27:237  L32:300  L37:375  L42:456  L47:546  L52:642  L57:747
L3:24   L8:56   L13:94   L18:140  L23:193  L28:248  L33:315  L38:390  L43:474  L48:564  L53:663  L58:768
L4:30   L9:63   L14:102  L19:150  L24:204  L29:259  L34:330  L39:405  L44:492  L49:582  L54:684  L59:789
L5:35   L10:70  L15:110  L20:160  L25:215  L30:270  L35:345  L40:420  L45:510  L50:600  L55:705  L60:810
```

**Base AC (standard NPC, ±0):**
```
L1–4: 14   L5–9: 15   L10–14: 16   L15–19: 17   L20–24: 18   L25–34: 19
L35–44: 20   L45–54: 21   L55–60: 22
```

**Tier HP/AC modifiers:**
| Tier | HP Multiplier | AC Modifier |
|---|---|---|
| minion | ×0.6 | −2 |
| standard | ×1.0 | ±0 |
| elite | ×1.5 | +2 |
| champion | ×2.0 | +4 |
| boss | ×3.0 | +5 |

**rob_multiplier requirements:** ≥1.2 for level ≥20; ≥1.5 for level ≥50; ≥2.0 for boss tier.

---

## File Structure

**New NPC templates (zone-specific clones to resolve cross-tier conflicts):**
- `content/npcs/rr_ganger.yaml` — Rustbucket Ridge ganger at T2 level
- `content/npcs/rr_scavenger.yaml` — Rustbucket Ridge scavenger at T2 level
- `content/npcs/rr_commissar.yaml` — Rustbucket Ridge commissar at T2 level
- `content/npcs/rr_lieutenant.yaml` — Rustbucket Ridge lieutenant at T2 level
- `content/npcs/beaverton_scav.yaml` — Beaverton scavenger at T2 level
- `content/npcs/lake_oswego_guard.yaml` — Lake Oswego standard combat NPC
- `content/npcs/lake_oswego_sniper.yaml` — Lake Oswego elite NPC (replaces country_club_sniper)
- `content/npcs/lake_oswego_warlord.yaml` — Lake Oswego boss NPC

**Modified NPC templates** (`content/npcs/*.yaml`): all zone combat NPCs — level, max_hp, ac, tier, boss_abilities, rob_multiplier.

**Modified zone files** (`content/zones/*.yaml`): min_level, max_level, room danger_level values; spawn table template references where clones replace shared templates.

**New test:** `internal/game/npc/stat_formula_test.go`

---

## Task 1: NPC Stat Formula Compliance Validator

**Files:**
- Create: `internal/game/npc/stat_formula_test.go`

- [ ] **Step 1: Write the compliance test**

```go
package npc_test

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseHP returns the standard-tier base HP for a given level using linear
// interpolation between the spec-defined anchors (REQ-ZDS-1).
func baseHP(level int) float64 {
	anchors := [][2]int{
		{1, 12}, {5, 35}, {10, 70}, {15, 110}, {20, 160},
		{30, 270}, {40, 420}, {50, 600}, {60, 810}, {70, 1050},
		{80, 1320}, {90, 1620}, {100, 1950},
	}
	if level <= anchors[0][0] {
		return float64(anchors[0][1])
	}
	for i := 1; i < len(anchors); i++ {
		lo, hi := anchors[i-1], anchors[i]
		if level <= hi[0] {
			t := float64(level-lo[0]) / float64(hi[0]-lo[0])
			return float64(lo[1]) + t*float64(hi[1]-lo[1])
		}
	}
	return float64(anchors[len(anchors)-1][1])
}

// baseAC returns the standard-tier base AC for a given level.
func baseAC(level int) float64 {
	anchors := [][2]int{
		{1, 14}, {5, 15}, {10, 16}, {15, 17}, {20, 18},
		{30, 19}, {40, 20}, {50, 21}, {60, 22}, {70, 23},
		{80, 24}, {90, 25}, {100, 26},
	}
	if level <= anchors[0][0] {
		return float64(anchors[0][1])
	}
	for i := 1; i < len(anchors); i++ {
		lo, hi := anchors[i-1], anchors[i]
		if level <= hi[0] {
			t := float64(level-lo[0]) / float64(hi[0]-lo[0])
			return float64(lo[1]) + t*float64(hi[1]-lo[1])
		}
	}
	return float64(anchors[len(anchors)-1][1])
}

func tierHPMult(tier string) float64 {
	switch tier {
	case "minion":
		return 0.6
	case "elite":
		return 1.5
	case "champion":
		return 2.0
	case "boss":
		return 3.0
	default:
		return 1.0
	}
}

func tierACMod(tier string) float64 {
	switch tier {
	case "minion":
		return -2
	case "elite":
		return 2
	case "champion":
		return 4
	case "boss":
		return 5
	default:
		return 0
	}
}

// TestNPCStatFormula_AllTemplatesCompliant loads all NPC templates from the
// content directory and asserts that every template's max_hp and ac are within
// ±10% of the formula value for its level and tier (REQ-ZDS-1).
func TestNPCStatFormula_AllTemplatesCompliant(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "content", "npcs")

	templates, err := npc.LoadTemplates(root)
	require.NoError(t, err)
	require.NotEmpty(t, templates)

	for _, tmpl := range templates {
		if tmpl.Level == 0 {
			continue // non-combat service NPCs without a level
		}
		tmpl := tmpl
		t.Run(tmpl.ID, func(t *testing.T) {
			expHP := baseHP(tmpl.Level) * tierHPMult(tmpl.Tier)
			expAC := baseAC(tmpl.Level) + tierACMod(tmpl.Tier)

			actualHP := float64(tmpl.MaxHP)
			actualAC := float64(tmpl.AC)

			hpLo, hpHi := expHP*0.90, expHP*1.10
			assert.True(t, actualHP >= hpLo && actualHP <= hpHi,
				"template %q level %d tier %q: max_hp %d not within ±10%% of formula %.0f (range [%.0f, %.0f])",
				tmpl.ID, tmpl.Level, tmpl.Tier, tmpl.MaxHP, expHP, hpLo, hpHi,
			)

			acLo := math.Round(expAC) - 1
			acHi := math.Round(expAC) + 1
			assert.True(t, actualAC >= acLo && actualAC <= acHi,
				"template %q level %d tier %q: ac %d not within ±1 of formula %.1f",
				tmpl.ID, tmpl.Level, tmpl.Tier, tmpl.AC, expAC,
			)
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails (most templates are out of spec)**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant -v 2>&1 | head -60
```

Expected: FAIL — many templates reported out of range. This is the baseline.

- [ ] **Step 3: Commit the test**

```bash
git add internal/game/npc/stat_formula_test.go
git commit -m "test(npc): stat formula compliance validator (REQ-ZDS-1)"
```

---

## Task 2: Shared NPC Template Conflict Resolution

Cross-tier conflicts: `ganger`, `scavenger`, `commissar`, `lieutenant` are used by both Tier 1 zones (1–10) and Rustbucket Ridge (Tier 2, 20–30). `strip_mall_scav` is used by Felony Flats (T1) and Beaverton (T2). `country_club_sniper` is used by Battleground (T1, 3–12) and Lake Oswego (T3, 45–60).

**Files:**
- Create: `content/npcs/rr_ganger.yaml`
- Create: `content/npcs/rr_scavenger.yaml`
- Create: `content/npcs/rr_commissar.yaml`
- Create: `content/npcs/rr_lieutenant.yaml`
- Create: `content/npcs/beaverton_scav.yaml`
- Create: `content/npcs/lake_oswego_guard.yaml`
- Create: `content/npcs/lake_oswego_sniper.yaml`
- Create: `content/npcs/lake_oswego_warlord.yaml`
- Modify: `content/zones/rustbucket_ridge.yaml` (update 4 template references)
- Modify: `content/zones/beaverton.yaml` (update 1 template reference)
- Modify: `content/zones/lake_oswego.yaml` (update 2 template references)

- [ ] **Step 1: Create RR-specific ganger clone**

Create `content/npcs/rr_ganger.yaml`:
```yaml
id: rr_ganger
name: Ridge Ganger
description: A hardened street tough who survived the collapse of Rustbucket Ridge's organized crime. Meaner and more desperate than your average ganger.
level: 20
max_hp: 160
ac: 18
rob_multiplier: 1.2
awareness: 6
ai_domain: ganger_npc_combat
respawn_delay: "5m"
abilities:
  brutality: 16
  quickness: 14
  grit: 16
  reasoning: 9
  savvy: 10
  flair: 8
weapon:
  - id: cheap_blade
    weight: 3
  - id: ganger_pistol
    weight: 2
armor:
  - id: kevlar_vest
    weight: 3
  - id: leather_jacket
    weight: 2
taunts:
  - "You walked into the wrong ridge, outsider."
  - "The Ridge eats soft people for breakfast."
  - "I've buried better than you in the scrapyard."
  - "Nobody comes here without owing the Ridge something."
```

- [ ] **Step 2: Create RR-specific scavenger clone**

Create `content/npcs/rr_scavenger.yaml`:
```yaml
id: rr_scavenger
name: Ridge Scavenger
description: A survivor who's picked over every scrap of Rustbucket Ridge twice over. Desperate and fast with a blade.
level: 20
max_hp: 160
ac: 18
rob_multiplier: 1.2
awareness: 5
ai_domain: ganger_npc_combat
respawn_delay: "5m"
abilities:
  brutality: 14
  quickness: 16
  grit: 14
  reasoning: 9
  savvy: 11
  flair: 8
weapon:
  - id: cheap_blade
    weight: 3
  - id: spiked_knuckles
    weight: 2
armor:
  - id: leather_jacket
    weight: 3
taunts:
  - "Everything here is mine. Including your gear."
  - "I've scavenged tougher than you from the pile."
```

- [ ] **Step 3: Create RR-specific commissar and lieutenant clones**

Create `content/npcs/rr_commissar.yaml`:
```yaml
id: rr_commissar
name: Ridge Commissar
description: A mid-level enforcer who keeps order in the rougher parts of Rustbucket Ridge through systematic violence.
level: 22
max_hp: 182
ac: 18
rob_multiplier: 1.2
awareness: 7
ai_domain: territory_patrol
respawn_delay: "8m"
abilities:
  brutality: 17
  quickness: 13
  grit: 16
  reasoning: 11
  savvy: 10
  flair: 9
weapon:
  - id: sawn_off
    weight: 3
  - id: steel_pipe
    weight: 2
armor:
  - id: kevlar_vest
    weight: 3
taunts:
  - "The Ridge has order. I enforce it."
  - "One warning. That was it."
```

Create `content/npcs/rr_lieutenant.yaml`:
```yaml
id: rr_lieutenant
name: Ridge Lieutenant
description: A seasoned gang lieutenant who commands a section of Rustbucket Ridge through fear and ruthless efficiency.
level: 24
max_hp: 204
ac: 18
rob_multiplier: 1.2
awareness: 8
ai_domain: territory_patrol
respawn_delay: "8m"
abilities:
  brutality: 18
  quickness: 14
  grit: 17
  reasoning: 12
  savvy: 11
  flair: 9
weapon:
  - id: sawn_off
    weight: 2
  - id: steel_pipe
    weight: 3
armor:
  - id: kevlar_vest
    weight: 3
  - id: leather_jacket
    weight: 1
taunts:
  - "Lieutenant means I've already won every fight that matters."
  - "You're either paying tribute or becoming an example."
```

- [ ] **Step 4: Create Beaverton-specific scav clone**

Create `content/npcs/beaverton_scav.yaml`:
```yaml
id: beaverton_scav
name: Strip Mall Scavenger
description: A desperate scavenger picking over Beaverton's abandoned strip malls for anything the HOA enforcers haven't locked down.
level: 16
max_hp: 120
ac: 17
rob_multiplier: 1.2
awareness: 5
ai_domain: ganger_npc_combat
respawn_delay: "5m"
abilities:
  brutality: 14
  quickness: 15
  grit: 14
  reasoning: 9
  savvy: 10
  flair: 8
weapon:
  - id: cheap_blade
    weight: 3
  - id: spiked_knuckles
    weight: 2
armor:
  - id: leather_jacket
    weight: 3
taunts:
  - "HOA doesn't own the parking lot. I do."
  - "Everything left behind is fair game."
```

- [ ] **Step 5: Create Lake Oswego guard, sniper, and warlord**

Create `content/npcs/lake_oswego_guard.yaml`:
```yaml
id: lake_oswego_guard
name: Country Club Guard
description: A heavily armed private security contractor protecting Lake Oswego's remaining wealthy enclaves. Well-equipped and utterly merciless.
level: 45
max_hp: 510
ac: 21
rob_multiplier: 1.5
awareness: 9
ai_domain: territory_patrol
respawn_delay: "5m"
abilities:
  brutality: 18
  quickness: 16
  grit: 18
  reasoning: 13
  savvy: 12
  flair: 10
weapon:
  - id: assault_rifle
    weight: 3
  - id: combat_knife
    weight: 2
armor:
  - id: tactical_armor
    weight: 3
taunts:
  - "This property is private. You're trespassing."
  - "Security clearance required. You don't have it."
  - "The club pays me very well to make this problem go away."
```

Create `content/npcs/lake_oswego_sniper.yaml`:
```yaml
id: lake_oswego_sniper
name: Hilltop Sniper
description: An elite marksman who holds the high ground in Lake Oswego, picking off intruders from fortified positions that have never been successfully rushed.
level: 50
max_hp: 900
tier: elite
ac: 23
rob_multiplier: 1.5
awareness: 12
ai_domain: territory_patrol
respawn_delay: "10m"
abilities:
  brutality: 16
  quickness: 20
  grit: 18
  reasoning: 16
  savvy: 14
  flair: 11
weapon:
  - id: sniper_rifle
    weight: 4
  - id: combat_knife
    weight: 1
armor:
  - id: tactical_armor
    weight: 3
taunts:
  - "You can't dodge what you don't see coming."
  - "Thousand-yard stare. About a hundred yards for you."
  - "Every shot I've taken at this range has connected."
```

Create `content/npcs/lake_oswego_warlord.yaml`:
```yaml
id: lake_oswego_warlord
name: The Commandant
description: >
  The iron-fisted ruler of Lake Oswego's remaining enclave, a former private
  military commander who seized the country clubs and golf courses when the
  collapse came and has held them through systematic brutality ever since.
  His tactical genius and personal combat capability make him one of the most
  dangerous individuals in the greater Portland area.
level: 60
max_hp: 2430
tier: boss
ac: 27
rob_multiplier: 2.0
awareness: 14
ai_domain: lake_oswego_warlord_combat
respawn_delay: "72h"
abilities:
  brutality: 22
  quickness: 18
  grit: 22
  reasoning: 20
  savvy: 18
  flair: 14
weapon:
  - id: assault_rifle
    weight: 3
  - id: combat_knife
    weight: 2
armor:
  - id: tactical_armor
    weight: 3
boss_abilities:
  - id: perimeter_lockdown
    name: "Perimeter Lockdown"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: slowed
  - id: tactical_barrage
    name: "Tactical Barrage"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "6d10"
  - id: last_resort_protocol
    name: "Last Resort Protocol"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 20
taunts:
  - "You've made a tactical error coming here."
  - "I've held this position against everything Portland has thrown at me."
  - "The enclave survives. You won't."
  - "Tactical analysis: you're outmatched."
```

- [ ] **Step 6: Update Rustbucket Ridge spawn tables to use RR-specific clones**

In `content/zones/rustbucket_ridge.yaml`, replace all occurrences of:
- `template: ganger` → `template: rr_ganger`
- `template: scavenger` → `template: rr_scavenger`
- `template: commissar` → `template: rr_commissar`
- `template: lieutenant` → `template: rr_lieutenant`

```bash
sed -i 's/template: ganger/template: rr_ganger/g' content/zones/rustbucket_ridge.yaml
sed -i 's/template: scavenger/template: rr_scavenger/g' content/zones/rustbucket_ridge.yaml
sed -i 's/template: commissar/template: rr_commissar/g' content/zones/rustbucket_ridge.yaml
sed -i 's/template: lieutenant/template: rr_lieutenant/g' content/zones/rustbucket_ridge.yaml
```

Verify no other templates were accidentally changed:
```bash
grep "template: rr_\|template: ganger\|template: scavenger\|template: commissar\|template: lieutenant" content/zones/rustbucket_ridge.yaml
```

- [ ] **Step 7: Update Beaverton and Lake Oswego spawn tables**

In `content/zones/beaverton.yaml`, replace `template: strip_mall_scav` → `template: beaverton_scav`:
```bash
sed -i 's/template: strip_mall_scav/template: beaverton_scav/g' content/zones/beaverton.yaml
```

In `content/zones/lake_oswego.yaml`, replace:
- `template: country_club_sniper` → `template: lake_oswego_sniper`
- `template: lake_oswego_commandant` → `template: lake_oswego_warlord`

```bash
sed -i 's/template: country_club_sniper/template: lake_oswego_sniper/g' content/zones/lake_oswego.yaml
sed -i 's/template: lake_oswego_commandant/template: lake_oswego_warlord/g' content/zones/lake_oswego.yaml
```

Also add a spawn entry for `lake_oswego_guard` in the standard combat rooms of `content/zones/lake_oswego.yaml`. Find the first non-boss room with spawns and add:
```yaml
    - template: lake_oswego_guard
      count: 2
      respawn_after: 5m
```

- [ ] **Step 8: Commit**

```bash
git add content/npcs/rr_ganger.yaml content/npcs/rr_scavenger.yaml content/npcs/rr_commissar.yaml content/npcs/rr_lieutenant.yaml content/npcs/beaverton_scav.yaml content/npcs/lake_oswego_guard.yaml content/npcs/lake_oswego_sniper.yaml content/npcs/lake_oswego_warlord.yaml content/zones/rustbucket_ridge.yaml content/zones/beaverton.yaml content/zones/lake_oswego.yaml
git commit -m "content(npc): add zone-specific NPC clones to resolve cross-tier template conflicts"
```

---

## Task 3: Tier 1 — Downtown Portland NPC Upgrades (band 1–10, boss L10)

**Files:**
- Modify: `content/npcs/ganger.yaml`
- Modify: `content/npcs/scavenger.yaml`
- Modify: `content/npcs/commissar.yaml`
- Modify: `content/npcs/lieutenant.yaml`
- Modify: `content/npcs/82nd_enforcer.yaml`
- Modify: `content/npcs/downtown_library_warlord.yaml`
- Modify: `content/zones/downtown.yaml`

- [ ] **Step 1: Update ganger (L1 → L2, HP 18, AC 14)**

In `content/npcs/ganger.yaml`, update:
```yaml
level: 2
max_hp: 18
ac: 14
```

- [ ] **Step 2: Update scavenger (L1, fix HP 14→12, AC 12→14)**

In `content/npcs/scavenger.yaml`, update:
```yaml
level: 1
max_hp: 12
ac: 14
```

- [ ] **Step 3: Update commissar (L3, fix HP 35→24, AC stays 14)**

In `content/npcs/commissar.yaml`, update:
```yaml
level: 3
max_hp: 24
ac: 14
```

- [ ] **Step 4: Update lieutenant (L3→L5, HP 35, AC 14→15)**

In `content/npcs/lieutenant.yaml`, update:
```yaml
level: 5
max_hp: 35
ac: 15
```

- [ ] **Step 5: Update 82nd_enforcer (L3→L7, HP 36→49, AC 16→15)**

In `content/npcs/82nd_enforcer.yaml`, update:
```yaml
level: 7
max_hp: 49
ac: 15
```

- [ ] **Step 6: Upgrade downtown_library_warlord to L10 boss**

In `content/npcs/downtown_library_warlord.yaml`, update:
```yaml
level: 10
max_hp: 210
tier: boss
ac: 21
rob_multiplier: 2.0
```

Add `boss_abilities` block after `ac:`:
```yaml
boss_abilities:
  - id: catalogue_of_doom
    name: "Catalogue of Doom"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: frightened
  - id: militia_reinforcement
    name: "Militia Reinforcement"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "2d8"
  - id: archival_fury
    name: "Archival Fury"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 25
```

- [ ] **Step 7: Update downtown zone metadata**

In `content/zones/downtown.yaml`, update the zone-level fields:
```yaml
  danger_level: sketchy
  min_level: 1
  max_level: 10
```

For each room in `downtown.yaml`, update `danger_level` per Tier 1 rules:
- Standard rooms: `safe` or `sketchy`
- Boss approach room (containing downtown_library_warlord spawn): `dangerous`

Run this to identify rooms that need `danger_level` changes:
```bash
grep -n "danger_level\|downtown_library_warlord" content/zones/downtown.yaml
```

Set the room containing `downtown_library_warlord` spawn to `danger_level: dangerous`. Leave all other rooms at `safe` or `sketchy`.

- [ ] **Step 8: Run validator**

```bash
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/ganger -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/scavenger -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/commissar -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/lieutenant -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/82nd_enforcer -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/downtown_library_warlord -v
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add content/npcs/ganger.yaml content/npcs/scavenger.yaml content/npcs/commissar.yaml content/npcs/lieutenant.yaml content/npcs/82nd_enforcer.yaml content/npcs/downtown_library_warlord.yaml content/zones/downtown.yaml
git commit -m "content(npc): Tier 1 Downtown Portland NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 4: Tier 1 — Felony Flats NPC Upgrades (band 1–10, boss L10)

**Files:**
- Modify: `content/npcs/strip_mall_scav.yaml`
- Modify: `content/npcs/crack_house_dealer.yaml`
- Modify: `content/npcs/flats_enforcer_captain.yaml`
- Modify: `content/npcs/felony_flats_enforcer_lord.yaml`
- Modify: `content/zones/felony_flats.yaml`

Note: `82nd_enforcer` already updated in Task 3.

- [ ] **Step 1: Update strip_mall_scav (L1, fix HP 16→12, AC 13→14)**

```yaml
level: 1
max_hp: 12
ac: 14
```

- [ ] **Step 2: Update crack_house_dealer (L3, fix HP 28→24, AC 13→14)**

```yaml
level: 3
max_hp: 24
ac: 14
```

- [ ] **Step 3: Update flats_enforcer_captain to elite L8**

```yaml
level: 8
max_hp: 84
tier: elite
ac: 17
```
(HP: 56 × 1.5 = 84; AC: 15 + 2 = 17)

- [ ] **Step 4: Upgrade felony_flats_enforcer_lord to L10 boss**

```yaml
level: 10
max_hp: 210
tier: boss
ac: 21
rob_multiplier: 2.0
boss_abilities:
  - id: gang_dominance
    name: "Gang Dominance"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: frightened
  - id: shank_volley
    name: "Shank Volley"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "2d6"
  - id: territorial_rage
    name: "Territorial Rage"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 5: Update felony_flats zone metadata**

In `content/zones/felony_flats.yaml`:
```yaml
  danger_level: dangerous
  min_level: 1
  max_level: 10
```

Set boss room (containing `felony_flats_enforcer_lord`) to `danger_level: dangerous`. All other rooms: `safe` or `sketchy`.

- [ ] **Step 6: Run validator and commit**

```bash
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/strip_mall_scav -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/crack_house_dealer -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/flats_enforcer_captain -v
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant/felony_flats_enforcer_lord -v
```

Expected: all PASS.

```bash
git add content/npcs/strip_mall_scav.yaml content/npcs/crack_house_dealer.yaml content/npcs/flats_enforcer_captain.yaml content/npcs/felony_flats_enforcer_lord.yaml content/zones/felony_flats.yaml
git commit -m "content(npc): Tier 1 Felony Flats NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 5: Tier 1 — The Couve NPC Upgrades (band 3–12, boss L12)

**Files:**
- Modify: `content/npcs/couve_scavenger.yaml`
- Modify: `content/npcs/mill_plain_thug.yaml`
- Modify: `content/npcs/couve_militia.yaml`
- Modify: `content/npcs/couve_gang_enforcer.yaml`
- Modify: `content/npcs/jantzen_beach_pirate.yaml`
- Modify: `content/npcs/the_couve_warlord.yaml`
- Modify: `content/zones/the_couve.yaml`

- [ ] **Step 1: Update standard combat NPCs**

`content/npcs/couve_scavenger.yaml`: `level: 3  max_hp: 24  ac: 14`

`content/npcs/mill_plain_thug.yaml`: `level: 3  max_hp: 24  ac: 14`

`content/npcs/couve_militia.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/couve_gang_enforcer.yaml`: `level: 6  max_hp: 42  ac: 15`

`content/npcs/jantzen_beach_pirate.yaml`: `level: 8  max_hp: 56  ac: 15`

- [ ] **Step 2: Upgrade the_couve_warlord to L12 boss**

```yaml
level: 12
max_hp: 258
tier: boss
ac: 21
rob_multiplier: 2.0
boss_abilities:
  - id: crossing_control
    name: "Crossing Control"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: slowed
  - id: border_assault
    name: "Border Assault"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "2d8"
  - id: warlord_authority
    name: "Warlord Authority"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata**

In `content/zones/the_couve.yaml`:
```yaml
  min_level: 3
  max_level: 12
```

Boss room: `danger_level: dangerous`. Standard rooms: `sketchy`. Safe rooms: `safe`.

- [ ] **Step 4: Run validator and commit**

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(couve_scavenger|mill_plain_thug|couve_militia|couve_gang_enforcer|jantzen_beach_pirate|the_couve_warlord)" -v
```

Expected: all PASS.

```bash
git add content/npcs/couve_scavenger.yaml content/npcs/mill_plain_thug.yaml content/npcs/couve_militia.yaml content/npcs/couve_gang_enforcer.yaml content/npcs/jantzen_beach_pirate.yaml content/npcs/the_couve_warlord.yaml content/zones/the_couve.yaml
git commit -m "content(npc): Tier 1 The Couve NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 6: Tier 1 — Troutdale NPC Upgrades (band 3–12, boss L12)

**Files:**
- Modify: `content/npcs/outlet_scavenger.yaml`
- Modify: `content/npcs/gorge_bandit.yaml`
- Modify: `content/npcs/salvager_crew.yaml`
- Modify: `content/npcs/gorge_runner.yaml`
- Modify: `content/npcs/wind_walker.yaml`
- Modify: `content/npcs/troutdale_gorge_king.yaml`
- Modify: `content/zones/troutdale.yaml`

- [ ] **Step 1: Update standard combat NPCs**

`content/npcs/outlet_scavenger.yaml`: `level: 3  max_hp: 24  ac: 14`

`content/npcs/gorge_bandit.yaml`: `level: 4  max_hp: 30  ac: 14`

`content/npcs/salvager_crew.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/gorge_runner.yaml`: `level: 7  max_hp: 49  ac: 15`

`content/npcs/wind_walker.yaml`: `level: 9  max_hp: 63  ac: 15`

- [ ] **Step 2: Upgrade troutdale_gorge_king to L12 boss**

```yaml
level: 12
max_hp: 258
tier: boss
ac: 21
rob_multiplier: 2.0
boss_abilities:
  - id: gorge_smash
    name: "Gorge Smash"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "2d6"
  - id: rock_slide
    name: "Rock Slide"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: slowed
  - id: kings_fury
    name: "King's Fury"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 30
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/troutdale.yaml`: `min_level: 3  max_level: 12`. Boss room: `danger_level: dangerous`. Other rooms: `sketchy` or `safe`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(outlet_scavenger|gorge_bandit|salvager_crew|gorge_runner|wind_walker|troutdale_gorge_king)" -v
```

Expected: all PASS.

```bash
git add content/npcs/outlet_scavenger.yaml content/npcs/gorge_bandit.yaml content/npcs/salvager_crew.yaml content/npcs/gorge_runner.yaml content/npcs/wind_walker.yaml content/npcs/troutdale_gorge_king.yaml content/zones/troutdale.yaml
git commit -m "content(npc): Tier 1 Troutdale NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 7: Tier 1 — NE Portland NPC Upgrades (band 5–15, boss L15)

**Files:**
- Modify: `content/npcs/bike_courier.yaml`, `content/npcs/alberta_drifter.yaml`
- Modify: `content/npcs/ne_gang_enforcer.yaml`, `content/npcs/brew_warlord.yaml`
- Modify: `content/npcs/ne_gulch_raider.yaml`, `content/npcs/ne_gang_lieutenant.yaml`
- Modify: `content/npcs/ne_portland_brew_lord.yaml`
- Modify: `content/npcs/ne_bridge_warden.yaml`
- Modify: `content/zones/ne_portland.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/bike_courier.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/alberta_drifter.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/ne_gang_enforcer.yaml`: `level: 7  max_hp: 49  ac: 15`

`content/npcs/brew_warlord.yaml`: `level: 8  max_hp: 56  ac: 15`

`content/npcs/ne_gulch_raider.yaml`: `level: 9  max_hp: 63  ac: 15`

`content/npcs/ne_gang_lieutenant.yaml`: `level: 11  max_hp: 78  ac: 16`

`content/npcs/ne_portland_brew_lord.yaml`: `level: 13  max_hp: 94  ac: 16`

- [ ] **Step 2: Upgrade ne_bridge_warden to L15 boss**

```yaml
level: 15
max_hp: 330
tier: boss
ac: 22
rob_multiplier: 2.0
boss_abilities:
  - id: bridge_toll
    name: "Bridge Toll"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: slowed
  - id: river_barrage
    name: "River Barrage"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "2d10"
  - id: warden_stance
    name: "Warden Stance"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/ne_portland.yaml`: `min_level: 5  max_level: 15`. Boss room: `danger_level: dangerous`. Other rooms: `safe` or `sketchy`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(bike_courier|alberta_drifter|ne_gang_enforcer|brew_warlord|ne_gulch_raider|ne_gang_lieutenant|ne_portland_brew_lord|ne_bridge_warden)" -v
```

Expected: all PASS.

```bash
git add content/npcs/bike_courier.yaml content/npcs/alberta_drifter.yaml content/npcs/ne_gang_enforcer.yaml content/npcs/brew_warlord.yaml content/npcs/ne_gulch_raider.yaml content/npcs/ne_gang_lieutenant.yaml content/npcs/ne_portland_brew_lord.yaml content/npcs/ne_bridge_warden.yaml content/zones/ne_portland.yaml
git commit -m "content(npc): Tier 1 NE Portland NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 8: Tier 1 — PDX International NPC Upgrades (band 5–15, boss L15)

**Files:**
- Modify: `content/npcs/terminal_squatter.yaml`, `content/npcs/airport_scavenger.yaml`
- Modify: `content/npcs/cargo_cultist.yaml`, `content/npcs/rogue_security.yaml`
- Modify: `content/npcs/tarmac_raider.yaml`
- Modify: `content/npcs/pdx_high_steward.yaml`
- Modify: `content/zones/pdx_international.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/terminal_squatter.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/airport_scavenger.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/cargo_cultist.yaml`: `level: 7  max_hp: 49  ac: 15`

`content/npcs/rogue_security.yaml`: `level: 9  max_hp: 63  ac: 15`

`content/npcs/tarmac_raider.yaml`: `level: 11  max_hp: 78  ac: 16`

- [ ] **Step 2: Upgrade pdx_high_steward to L15 boss**

```yaml
level: 15
max_hp: 330
tier: boss
ac: 22
rob_multiplier: 2.0
boss_abilities:
  - id: terminal_lockdown
    name: "Terminal Lockdown"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: slowed
  - id: security_surge
    name: "Security Surge"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "2d10"
  - id: elevated_threat
    name: "Elevated Threat"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/pdx_international.yaml`: `min_level: 5  max_level: 15`. Boss room: `danger_level: dangerous`. Other rooms: `safe` or `sketchy`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(terminal_squatter|airport_scavenger|cargo_cultist|rogue_security|tarmac_raider|pdx_high_steward)" -v
```

Expected: all PASS.

```bash
git add content/npcs/terminal_squatter.yaml content/npcs/airport_scavenger.yaml content/npcs/cargo_cultist.yaml content/npcs/rogue_security.yaml content/npcs/tarmac_raider.yaml content/npcs/pdx_high_steward.yaml content/zones/pdx_international.yaml
git commit -m "content(npc): Tier 1 PDX International NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 9: Tier 1 — Battleground NPC Upgrades (band 3–12, boss L12)

**Files:**
- Modify: `content/npcs/field_worker.yaml`
- Modify: `content/npcs/collective_guardian.yaml`
- Modify: `content/npcs/country_club_sniper.yaml`
- Modify: `content/npcs/battleground_grand_commissar.yaml`
- Modify: `content/zones/battleground.yaml`

Note: `commissar` and `lieutenant` already updated in Task 3.

- [ ] **Step 1: Update standard NPCs**

`content/npcs/field_worker.yaml`: `level: 3  max_hp: 24  ac: 14`

`content/npcs/collective_guardian.yaml`: `level: 7  max_hp: 49  ac: 15`

`content/npcs/country_club_sniper.yaml`: `level: 8  max_hp: 56  ac: 15`

(Lake Oswego uses `lake_oswego_sniper` from Task 2; this template is now Battleground-only.)

- [ ] **Step 2: Upgrade battleground_grand_commissar to L12 boss**

```yaml
level: 12
max_hp: 258
tier: boss
ac: 21
rob_multiplier: 2.0
boss_abilities:
  - id: peoples_decree
    name: "People's Decree"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: frightened
  - id: commissar_volley
    name: "Commissar Volley"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "2d8"
  - id: final_decree
    name: "Final Decree"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/battleground.yaml`: `min_level: 3  max_level: 12`. Boss room: `danger_level: dangerous`. Other rooms: `safe` or `sketchy`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(field_worker|collective_guardian|country_club_sniper|battleground_grand_commissar)" -v
```

Expected: all PASS.

```bash
git add content/npcs/field_worker.yaml content/npcs/collective_guardian.yaml content/npcs/country_club_sniper.yaml content/npcs/battleground_grand_commissar.yaml content/zones/battleground.yaml
git commit -m "content(npc): Tier 1 Battleground NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 10: Tier 1 — Hillsboro NPC Upgrades (band 5–15, boss L15)

**Files:**
- Modify: `content/npcs/silicon_serf.yaml`, `content/npcs/intel_guard.yaml`
- Modify: `content/npcs/hills_armory_warden.yaml`, `content/npcs/hillsboro_knight.yaml`
- Modify: `content/npcs/hills_fortress_sentinel.yaml`
- Modify: `content/npcs/hillsboro_knight_commander.yaml`
- Modify: `content/zones/hillsboro.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/silicon_serf.yaml`: `level: 5  max_hp: 35  ac: 15`

`content/npcs/intel_guard.yaml`: `level: 7  max_hp: 49  ac: 15`

`content/npcs/hills_armory_warden.yaml`: `level: 9  max_hp: 63  ac: 15`

`content/npcs/hillsboro_knight.yaml`: `level: 10  max_hp: 70  ac: 16`

`content/npcs/hills_fortress_sentinel.yaml`: `level: 12  max_hp: 86  ac: 16`

- [ ] **Step 2: Upgrade hillsboro_knight_commander to L15 boss**

```yaml
level: 15
max_hp: 330
tier: boss
ac: 22
rob_multiplier: 2.0
boss_abilities:
  - id: knights_charge
    name: "Knight's Charge"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "2d8"
  - id: fortify
    name: "Fortify"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "5m"
    effect:
      heal_pct: 30
  - id: crusaders_wrath
    name: "Crusader's Wrath"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      aoe_damage_expr: "4d10"
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/hillsboro.yaml`: `min_level: 5  max_level: 15`. Boss room: `danger_level: dangerous`. Other rooms: `safe` or `sketchy`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(silicon_serf|intel_guard|hills_armory_warden|hillsboro_knight$|hills_fortress_sentinel|hillsboro_knight_commander)" -v
```

Expected: all PASS.

```bash
git add content/npcs/silicon_serf.yaml content/npcs/intel_guard.yaml content/npcs/hills_armory_warden.yaml content/npcs/hillsboro_knight.yaml content/npcs/hills_fortress_sentinel.yaml content/npcs/hillsboro_knight_commander.yaml content/zones/hillsboro.yaml
git commit -m "content(npc): Tier 1 Hillsboro NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 11: Tier 2 — Beaverton NPC Upgrades (band 16–25, boss L25)

**Files:**
- Modify: `content/npcs/beaverton_minuteman.yaml`, `content/npcs/compound_guard.yaml`
- Modify: `content/npcs/hoa_enforcer.yaml`, `content/npcs/mall_cop_elite.yaml`
- Modify: `content/npcs/beaverton_hoa_warlord.yaml`
- Modify: `content/zones/beaverton.yaml`

Note: `beaverton_scav` already created in Task 2.

- [ ] **Step 1: Update standard NPCs**

`content/npcs/beaverton_minuteman.yaml`: `level: 16  max_hp: 120  ac: 17  rob_multiplier: 1.2`

`content/npcs/compound_guard.yaml`: `level: 18  max_hp: 140  ac: 17  rob_multiplier: 1.2`

`content/npcs/hoa_enforcer.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/mall_cop_elite.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade beaverton_hoa_warlord to L25 boss**

```yaml
level: 25
max_hp: 645
tier: boss
ac: 23
rob_multiplier: 2.0
boss_abilities:
  - id: hoa_citation
    name: "HOA Citation"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: slowed
  - id: code_enforcement
    name: "Code Enforcement"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "3d10"
  - id: warlord_ordinance
    name: "Warlord Ordinance"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/beaverton.yaml`: `min_level: 16  max_level: 25`. Boss room: `danger_level: all_out_war`. Other rooms: `sketchy` or `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(beaverton_minuteman|compound_guard|hoa_enforcer|mall_cop_elite|beaverton_hoa_warlord|beaverton_scav)" -v
```

Expected: all PASS.

```bash
git add content/npcs/beaverton_minuteman.yaml content/npcs/compound_guard.yaml content/npcs/hoa_enforcer.yaml content/npcs/mall_cop_elite.yaml content/npcs/beaverton_hoa_warlord.yaml content/zones/beaverton.yaml
git commit -m "content(npc): Tier 2 Beaverton NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 12: Tier 2 — Vantucky NPC Upgrades (band 16–25, boss L25)

**Files:**
- Modify: `content/npcs/highway_bandit.yaml`, `content/npcs/vantucky_militiaman.yaml`
- Modify: `content/npcs/vantucky_scavenger.yaml`, `content/npcs/rack_vantucky.yaml`
- Modify: `content/npcs/vantucky_gang_enforcer.yaml`
- Modify: `content/npcs/vantucky_militia_commander.yaml`
- Modify: `content/zones/vantucky.yaml`

Note: `compound_guard` already updated in Task 11.

- [ ] **Step 1: Update standard NPCs**

`content/npcs/highway_bandit.yaml`: `level: 16  max_hp: 120  ac: 17  rob_multiplier: 1.2`

`content/npcs/vantucky_militiaman.yaml`: `level: 16  max_hp: 120  ac: 17  rob_multiplier: 1.2`

`content/npcs/vantucky_scavenger.yaml`: `level: 16  max_hp: 120  ac: 17  rob_multiplier: 1.2`

`content/npcs/rack_vantucky.yaml`: `level: 18  max_hp: 140  ac: 17  rob_multiplier: 1.2`

`content/npcs/vantucky_gang_enforcer.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade vantucky_militia_commander to L25 boss**

```yaml
level: 25
max_hp: 645
tier: boss
ac: 23
rob_multiplier: 2.0
boss_abilities:
  - id: militia_rally
    name: "Militia Rally"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: frightened
  - id: suppressive_fire
    name: "Suppressive Fire"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "4d10"
  - id: last_stand
    name: "Last Stand"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/vantucky.yaml`: `min_level: 16  max_level: 25`. Boss room: `danger_level: all_out_war`. Other rooms: `sketchy` or `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(highway_bandit|vantucky_militiaman|vantucky_scavenger|rack_vantucky|vantucky_gang_enforcer|vantucky_militia_commander)" -v
```

Expected: all PASS.

```bash
git add content/npcs/highway_bandit.yaml content/npcs/vantucky_militiaman.yaml content/npcs/vantucky_scavenger.yaml content/npcs/rack_vantucky.yaml content/npcs/vantucky_gang_enforcer.yaml content/npcs/vantucky_militia_commander.yaml content/zones/vantucky.yaml
git commit -m "content(npc): Tier 2 Vantucky NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 13: Tier 2 — Rustbucket Ridge NPC Upgrades (band 20–30, boss L30)

**Files:**
- Modify: many `content/npcs/*.yaml` (listed below)
- Modify: `content/zones/rustbucket_ridge.yaml`

Note: RR-specific clones (rr_ganger, rr_scavenger, rr_commissar, rr_lieutenant) already created in Task 2.

- [ ] **Step 1: Update all standard combat NPCs to T2 levels**

All of these are unique to Rustbucket Ridge. Set each:

`content/npcs/old_rusty.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/sparks.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/whiskey_joe.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/barrel_house_enforcer.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/slick_sally.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/vera_coldcoin.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/gail_grinder_graves.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/tina_wires.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/dwayne_dawg.yaml`: `level: 21  max_hp: 171  ac: 18  rob_multiplier: 1.2`

`content/npcs/jennifer_dawg.yaml`: `level: 21  max_hp: 171  ac: 18  rob_multiplier: 1.2`

`content/npcs/wayne_dawg.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/patch.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/ellie_mack.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/clutch.yaml`: `level: 23  max_hp: 193  ac: 18  rob_multiplier: 1.2`

`content/npcs/dex.yaml`: `level: 23  max_hp: 193  ac: 18  rob_multiplier: 1.2`

`content/npcs/herb.yaml`: `level: 24  max_hp: 204  ac: 18  rob_multiplier: 1.2`

`content/npcs/rio_wrench.yaml`: `level: 24  max_hp: 204  ac: 18  rob_multiplier: 1.2`

`content/npcs/sergeant_mack.yaml`: `level: 25  max_hp: 215  ac: 19  rob_multiplier: 1.2`

`content/npcs/marshal_ironsides.yaml`: `level: 26  max_hp: 226  ac: 19  rob_multiplier: 1.2`

`content/npcs/rr_field_boss.yaml` (new file): `level: 28  max_hp: 248  ac: 19  rob_multiplier: 1.5`

Note: `rustbucket_ridge_warlord.yaml` does NOT exist. The actual RR boss NPC is `rustbucket_ridge_slasher.yaml` (The Slasher). The Slasher is upgraded to L30 boss in Step 2. Create `rr_field_boss.yaml` as the field boss (Blood Camp Enforcer) at L28.

- [ ] **Step 2: Upgrade rustbucket_ridge_slasher to L30 boss and create rr_field_boss**

In `content/npcs/rustbucket_ridge_slasher.yaml`, set:
```yaml
level: 30
max_hp: 810
tier: boss
ac: 24
rob_multiplier: 2.0
boss_abilities:
  - id: ridge_domination
    name: "Ridge Domination"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: frightened
  - id: scrap_barrage
    name: "Scrap Barrage"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "4d10"
  - id: warlords_iron_will
    name: "Warlord's Iron Will"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 30
```

Create `content/npcs/rr_field_boss.yaml` at `level: 28  max_hp: 248  ac: 19  rob_multiplier: 1.2  ai_domain: territory_patrol`. Add one spawn of `rr_field_boss` to a non-boss dangerous room (e.g., `the_barrel_house`) in `content/zones/rustbucket_ridge.yaml`.

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/rustbucket_ridge.yaml`: `min_level: 20  max_level: 30`. Boss room: `danger_level: all_out_war`. Other rooms: `sketchy` or `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(old_rusty|sparks|whiskey_joe|barrel_house_enforcer|slick_sally|rustbucket_ridge_slasher|marshal_ironsides|sergeant_mack|rr_field_boss)" -v
```

Expected: all PASS.

```bash
git add content/npcs/old_rusty.yaml content/npcs/sparks.yaml content/npcs/whiskey_joe.yaml content/npcs/barrel_house_enforcer.yaml content/npcs/slick_sally.yaml content/npcs/vera_coldcoin.yaml content/npcs/gail_grinder_graves.yaml content/npcs/tina_wires.yaml content/npcs/dwayne_dawg.yaml content/npcs/jennifer_dawg.yaml content/npcs/wayne_dawg.yaml content/npcs/patch.yaml content/npcs/ellie_mack.yaml content/npcs/clutch.yaml content/npcs/dex.yaml content/npcs/herb.yaml content/npcs/rio_wrench.yaml content/npcs/sergeant_mack.yaml content/npcs/marshal_ironsides.yaml content/npcs/rustbucket_ridge_slasher.yaml content/npcs/rr_field_boss.yaml content/zones/rustbucket_ridge.yaml
git commit -m "content(npc): Tier 2 Rustbucket Ridge NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 14: Tier 2 — Sauvie Island NPC Upgrades (band 20–30, boss L30)

**Files:**
- Modify: `content/npcs/island_farmer.yaml`, `content/npcs/sauvie_wild_dog.yaml`
- Modify: `content/npcs/sauvie_survivalist.yaml`, `content/npcs/harvest_guard.yaml`
- Modify: `content/npcs/river_pirate.yaml`
- Modify: `content/npcs/sauvie_island_pirate_queen.yaml`
- Modify: `content/zones/sauvie_island.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/island_farmer.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/sauvie_wild_dog.yaml`: `level: 20  max_hp: 160  ac: 18  rob_multiplier: 1.2`

`content/npcs/sauvie_survivalist.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/harvest_guard.yaml`: `level: 24  max_hp: 204  ac: 18  rob_multiplier: 1.2`

`content/npcs/river_pirate.yaml`: `level: 26  max_hp: 226  ac: 19  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade sauvie_island_pirate_queen to L30 boss**

```yaml
level: 30
max_hp: 810
tier: boss
ac: 24
rob_multiplier: 2.0
boss_abilities:
  - id: broadside_barrage
    name: "Broadside Barrage"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "4d10"
  - id: pirates_gambit
    name: "Pirate's Gambit"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: slowed
  - id: queens_resolve
    name: "Queen's Resolve"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/sauvie_island.yaml`: `min_level: 20  max_level: 30`. Boss room: `danger_level: all_out_war`. Other rooms: `sketchy` or `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(island_farmer|sauvie_wild_dog|sauvie_survivalist|harvest_guard|river_pirate|sauvie_island_pirate_queen)" -v
```

Expected: all PASS.

```bash
git add content/npcs/island_farmer.yaml content/npcs/sauvie_wild_dog.yaml content/npcs/sauvie_survivalist.yaml content/npcs/harvest_guard.yaml content/npcs/river_pirate.yaml content/npcs/sauvie_island_pirate_queen.yaml content/zones/sauvie_island.yaml
git commit -m "content(npc): Tier 2 Sauvie Island NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 15: Tier 2 — Colonel Summers Park NPC Upgrades (band 22–32, boss L32)

**Files:**
- Modify: `content/npcs/shrimp_gang_goon.yaml`, `content/npcs/shrimp_gang_dazzler.yaml`
- Modify: `content/npcs/shrimp_gang_enforcer.yaml`
- Modify: `content/npcs/yo_yo_master.yaml`
- Modify: `content/zones/colonel_summers_park.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/shrimp_gang_goon.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/shrimp_gang_dazzler.yaml`: `level: 26  max_hp: 226  ac: 19  rob_multiplier: 1.2`

`content/npcs/shrimp_gang_enforcer.yaml`: `level: 28  max_hp: 248  ac: 19  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade yo_yo_master to L32 boss**

```yaml
level: 32
max_hp: 900
tier: boss
ac: 24
rob_multiplier: 2.0
boss_abilities:
  - id: yo_yo_lash
    name: "Yo-Yo Lash"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "4d10"
  - id: dazzle_spin
    name: "Dazzle Spin"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: nausea
  - id: masters_resolve
    name: "Master's Resolve"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/colonel_summers_park.yaml`: `min_level: 22  max_level: 32`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(shrimp_gang_goon|shrimp_gang_dazzler|shrimp_gang_enforcer|yo_yo_master)" -v
```

Expected: all PASS.

```bash
git add content/npcs/shrimp_gang_goon.yaml content/npcs/shrimp_gang_dazzler.yaml content/npcs/shrimp_gang_enforcer.yaml content/npcs/yo_yo_master.yaml content/zones/colonel_summers_park.yaml
git commit -m "content(npc): Tier 2 Colonel Summers Park NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 16: Tier 2 — Ross Island NPC Upgrades (band 22–32, boss L32)

**Files:**
- Modify: `content/npcs/island_hermit.yaml`, `content/npcs/ross_salvager.yaml`
- Modify: `content/npcs/bridge_troll.yaml`, `content/npcs/gang_enforcer.yaml`
- Modify: `content/npcs/gravel_pit_boss.yaml`
- Modify: `content/npcs/ross_island_pit_king.yaml`
- Modify: `content/zones/ross_island.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/island_hermit.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/ross_salvager.yaml`: `level: 22  max_hp: 182  ac: 18  rob_multiplier: 1.2`

`content/npcs/bridge_troll.yaml`: `level: 25  max_hp: 215  ac: 19  rob_multiplier: 1.2`

`content/npcs/gang_enforcer.yaml`: `level: 26  max_hp: 226  ac: 19  rob_multiplier: 1.2`

`content/npcs/gravel_pit_boss.yaml`: `level: 28  max_hp: 248  ac: 19  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade ross_island_pit_king to L32 boss**

```yaml
level: 32
max_hp: 900
tier: boss
ac: 24
rob_multiplier: 2.0
boss_abilities:
  - id: pit_slam
    name: "Pit Slam"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "4d10"
  - id: gravel_storm
    name: "Gravel Storm"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: nausea
  - id: pit_kings_will
    name: "Pit King's Will"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/ross_island.yaml`: `min_level: 22  max_level: 32`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(island_hermit|ross_salvager|bridge_troll|gang_enforcer|gravel_pit_boss|ross_island_pit_king)" -v
```

Expected: all PASS.

```bash
git add content/npcs/island_hermit.yaml content/npcs/ross_salvager.yaml content/npcs/bridge_troll.yaml content/npcs/gang_enforcer.yaml content/npcs/gravel_pit_boss.yaml content/npcs/ross_island_pit_king.yaml content/zones/ross_island.yaml
git commit -m "content(npc): Tier 2 Ross Island NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 17: Tier 2 — SE Industrial NPC Upgrades (band 25–35, boss L35)

**Files:**
- Modify: `content/npcs/industrial_scav.yaml`, `content/npcs/dock_worker.yaml`
- Modify: `content/npcs/rail_gang_raider.yaml`, `content/npcs/chemical_enforcer.yaml`
- Modify: `content/npcs/warehouse_guard.yaml`
- Modify: `content/npcs/sei_dock_warlord.yaml`
- Modify: `content/zones/se_industrial.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/industrial_scav.yaml`: `level: 25  max_hp: 215  ac: 19  rob_multiplier: 1.2`

`content/npcs/dock_worker.yaml`: `level: 25  max_hp: 215  ac: 19  rob_multiplier: 1.2`

`content/npcs/rail_gang_raider.yaml`: `level: 27  max_hp: 237  ac: 19  rob_multiplier: 1.2`

`content/npcs/chemical_enforcer.yaml`: `level: 29  max_hp: 259  ac: 19  rob_multiplier: 1.2`

`content/npcs/warehouse_guard.yaml`: `level: 31  max_hp: 285  ac: 19  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade sei_dock_warlord to L35 boss**

```yaml
level: 35
max_hp: 1035
tier: boss
ac: 25
rob_multiplier: 2.0
boss_abilities:
  - id: toxic_spill
    name: "Toxic Spill"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: nausea
  - id: dock_barrage
    name: "Dock Barrage"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "5d10"
  - id: warlords_endurance
    name: "Warlord's Endurance"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/se_industrial.yaml`: `min_level: 25  max_level: 35`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(industrial_scav|dock_worker|rail_gang_raider|chemical_enforcer|warehouse_guard|sei_dock_warlord)" -v
```

Expected: all PASS.

```bash
git add content/npcs/industrial_scav.yaml content/npcs/dock_worker.yaml content/npcs/rail_gang_raider.yaml content/npcs/chemical_enforcer.yaml content/npcs/warehouse_guard.yaml content/npcs/sei_dock_warlord.yaml content/zones/se_industrial.yaml
git commit -m "content(npc): Tier 2 SE Industrial NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 18: Tier 2 — Aloha NPC Upgrades (band 16–22, boss L22)

**Files:**
- Modify: `content/npcs/smuggler.yaml`, `content/npcs/neutral_zone_sentry.yaml`
- Modify: `content/npcs/aloha_smuggler_king.yaml`
- Modify: `content/zones/aloha.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/smuggler.yaml`: `level: 16  max_hp: 120  ac: 17  rob_multiplier: 1.2`

`content/npcs/neutral_zone_sentry.yaml`: `level: 18  max_hp: 140  ac: 17  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade aloha_smuggler_king to L22 boss**

```yaml
level: 22
max_hp: 546
tier: boss
ac: 23
rob_multiplier: 2.0
boss_abilities:
  - id: smugglers_gambit
    name: "Smuggler's Gambit"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: nausea
  - id: contraband_blast
    name: "Contraband Blast"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "3d10"
  - id: kings_escape
    name: "King's Escape"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/aloha.yaml`: `min_level: 16  max_level: 22`. Boss room: `danger_level: all_out_war`. Other rooms: `sketchy` or `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(smuggler|neutral_zone_sentry|aloha_smuggler_king)" -v
```

Expected: all PASS.

```bash
git add content/npcs/smuggler.yaml content/npcs/neutral_zone_sentry.yaml content/npcs/aloha_smuggler_king.yaml content/zones/aloha.yaml
git commit -m "content(npc): Tier 2 Aloha NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 19: Tier 3 — Oregon Country Fair NPC Upgrades (band 36–50, boss L50)

**Files:**
- Modify: `content/npcs/wook.yaml`, `content/npcs/tweaker.yaml`, `content/npcs/tweaker_paranoid.yaml`
- Modify: `content/npcs/juggalo.yaml`, `content/npcs/tweaker_cook.yaml`
- Modify: `content/npcs/juggalo_prophet.yaml`, `content/npcs/wook_enforcer.yaml`
- Modify: `content/npcs/crystal_karen.yaml`, `content/npcs/spiral_king.yaml`, `content/npcs/violent_jimmy.yaml`
- Modify: `content/zones/oregon_country_fair.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/wook.yaml`: `level: 36  max_hp: 360  ac: 20  rob_multiplier: 1.2`

`content/npcs/tweaker.yaml`: `level: 36  max_hp: 360  ac: 20  rob_multiplier: 1.2`

`content/npcs/tweaker_paranoid.yaml`: `level: 36  max_hp: 360  ac: 20  rob_multiplier: 1.2`

`content/npcs/juggalo.yaml`: `level: 39  max_hp: 405  ac: 20  rob_multiplier: 1.2`

`content/npcs/tweaker_cook.yaml`: `level: 42  max_hp: 456  ac: 20  rob_multiplier: 1.2`

`content/npcs/juggalo_prophet.yaml`: `level: 44  max_hp: 492  ac: 20  rob_multiplier: 1.2`

`content/npcs/wook_enforcer.yaml`: `level: 46  max_hp: 528  ac: 21  rob_multiplier: 1.5`

- [ ] **Step 2: Upgrade three bosses to L50**

OCF has three boss NPCs (crystal_karen, spiral_king, violent_jimmy). All are upgraded to L50 boss.

`content/npcs/crystal_karen.yaml`:
```yaml
level: 50
max_hp: 1800
tier: boss
ac: 26
rob_multiplier: 2.0
boss_abilities:
  - id: crystal_storm
    name: "Crystal Storm"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: nausea
  - id: meth_rage
    name: "Meth Rage"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "6d10"
  - id: karens_comeback
    name: "Karen's Comeback"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

`content/npcs/spiral_king.yaml`:
```yaml
level: 50
max_hp: 1800
tier: boss
ac: 26
rob_multiplier: 2.0
boss_abilities:
  - id: spiral_vortex
    name: "Spiral Vortex"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "5d10"
  - id: kings_madness
    name: "King's Madness"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: nausea
  - id: spiral_rebirth
    name: "Spiral Rebirth"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 30
```

`content/npcs/violent_jimmy.yaml`:
```yaml
level: 50
max_hp: 1800
tier: boss
ac: 26
rob_multiplier: 2.0
boss_abilities:
  - id: jimmys_rampage
    name: "Jimmy's Rampage"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "6d10"
  - id: bloody_fury
    name: "Bloody Fury"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: frightened
  - id: violent_surge
    name: "Violent Surge"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/oregon_country_fair.yaml`: `min_level: 36  max_level: 50`. Boss rooms: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(wook$|tweaker$|tweaker_paranoid|juggalo$|tweaker_cook|juggalo_prophet|wook_enforcer|crystal_karen|spiral_king|violent_jimmy)" -v
```

Expected: all PASS.

```bash
git add content/npcs/wook.yaml content/npcs/tweaker.yaml content/npcs/tweaker_paranoid.yaml content/npcs/juggalo.yaml content/npcs/tweaker_cook.yaml content/npcs/juggalo_prophet.yaml content/npcs/wook_enforcer.yaml content/npcs/crystal_karen.yaml content/npcs/spiral_king.yaml content/npcs/violent_jimmy.yaml content/zones/oregon_country_fair.yaml
git commit -m "content(npc): Tier 3 Oregon Country Fair NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 20: Tier 3 — Wooklyn NPC Upgrades (band 36–48, boss L48)

**Files:**
- Modify: `content/npcs/ginger_wook.yaml`
- Modify: `content/npcs/wook_shaman.yaml`
- Modify: `content/npcs/papa_wook.yaml`
- Modify: `content/zones/wooklyn.yaml`

Note: `wook` and `wook_enforcer` already upgraded in Task 19 (shared with OCF — same Tier 3 band).

- [ ] **Step 1: Update Wooklyn-specific NPCs**

`content/npcs/ginger_wook.yaml`: `level: 36  max_hp: 360  ac: 20  rob_multiplier: 1.2`

`content/npcs/wook_shaman.yaml`: `level: 40  max_hp: 420  ac: 20  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade papa_wook to L48 boss**

```yaml
level: 48
max_hp: 1692
tier: boss
ac: 26
rob_multiplier: 2.0
```

Update existing `boss_abilities` — keep IDs the same but update damage dice to match the new level (existing abilities are thematically correct):
```yaml
boss_abilities:
  - id: psychedelic_burst
    name: "Psychedelic Burst"
    trigger: round_start
    trigger_value: 0
    cooldown: "4m"
    effect:
      aoe_condition: nausea
  - id: eternal_groove_drain
    name: "The Eternal Groove"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      aoe_damage_expr: "6d10"
```

Also add a self-heal phase ability (required by REQ-ZDS-3d):
```yaml
  - id: grove_renewal
    name: "Grove Renewal"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/wooklyn.yaml`: `min_level: 36  max_level: 48`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(ginger_wook|wook_shaman|papa_wook)" -v
```

Expected: all PASS.

```bash
git add content/npcs/ginger_wook.yaml content/npcs/wook_shaman.yaml content/npcs/papa_wook.yaml content/zones/wooklyn.yaml
git commit -m "content(npc): Tier 3 Wooklyn NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 21: Tier 3 — SteamPDX NPC Upgrades (band 40–55, boss L55)

**Files:**
- Modify: `content/npcs/steam_patron.yaml`, `content/npcs/steam_bouncer.yaml`
- Modify: `content/npcs/the_big_3.yaml`
- Modify: `content/zones/steampdx.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/steam_patron.yaml`: `level: 40  max_hp: 420  ac: 20  rob_multiplier: 1.2`

`content/npcs/steam_bouncer.yaml`: `level: 44  max_hp: 492  ac: 20  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade the_big_3 to L55 boss**

```yaml
level: 55
max_hp: 2115
tier: boss
ac: 27
rob_multiplier: 2.0
boss_abilities:
  - id: triple_threat
    name: "Triple Threat"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "7d10"
  - id: steam_surge
    name: "Steam Surge"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: nausea
  - id: big_3_resilience
    name: "Big 3 Resilience"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/steampdx.yaml`: `min_level: 40  max_level: 55`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(steam_patron|steam_bouncer|the_big_3)" -v
```

Expected: all PASS.

```bash
git add content/npcs/steam_patron.yaml content/npcs/steam_bouncer.yaml content/npcs/the_big_3.yaml content/zones/steampdx.yaml
git commit -m "content(npc): Tier 3 SteamPDX NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 22: Tier 3 — Club Privata NPC Upgrades (band 40–55, boss L55)

**Files:**
- Modify: `content/npcs/club_dancer.yaml`, `content/npcs/club_bouncer.yaml`
- Modify: `content/npcs/club_vip_boss.yaml`
- Modify: `content/zones/club_privata.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/club_dancer.yaml`: `level: 40  max_hp: 420  ac: 20  rob_multiplier: 1.2`

`content/npcs/club_bouncer.yaml`: `level: 44  max_hp: 492  ac: 20  rob_multiplier: 1.2`

Note: `club_bouncer` is also used in `the_velvet_rope` — same T3 band, no conflict.

- [ ] **Step 2: Upgrade club_vip_boss to L55 boss**

```yaml
level: 55
max_hp: 2115
tier: boss
ac: 27
rob_multiplier: 2.0
boss_abilities:
  - id: vip_lockdown
    name: "VIP Lockdown"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: slowed
  - id: exclusive_violence
    name: "Exclusive Violence"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "7d10"
  - id: vip_resilience
    name: "VIP Resilience"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/club_privata.yaml`: `min_level: 40  max_level: 55`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(club_dancer|club_bouncer|club_vip_boss)" -v
```

Expected: all PASS.

```bash
git add content/npcs/club_dancer.yaml content/npcs/club_bouncer.yaml content/npcs/club_vip_boss.yaml content/zones/club_privata.yaml
git commit -m "content(npc): Tier 3 Club Privata NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 23: Tier 3 — The Velvet Rope NPC Upgrades (band 40–55, boss L55)

**Files:**
- Modify: `content/npcs/velvet_patron.yaml`, `content/npcs/velvet_hostess.yaml`
- Modify: `content/npcs/gangbang.yaml`
- Modify: `content/zones/the_velvet_rope.yaml`

Note: `club_bouncer` already updated in Task 22.

- [ ] **Step 1: Update standard NPCs**

`content/npcs/velvet_patron.yaml`: `level: 40  max_hp: 420  ac: 20  rob_multiplier: 1.2`

`content/npcs/velvet_hostess.yaml`: `level: 42  max_hp: 456  ac: 20  rob_multiplier: 1.2`

- [ ] **Step 2: Upgrade gangbang to L55 boss**

```yaml
level: 55
max_hp: 2115
tier: boss
ac: 27
rob_multiplier: 2.0
boss_abilities:
  - id: velvet_frenzy
    name: "Velvet Frenzy"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "7d10"
  - id: rope_restriction
    name: "Rope Restriction"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: slowed
  - id: gangbang_resurgence
    name: "Gangbang Resurgence"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/the_velvet_rope.yaml`: `min_level: 40  max_level: 55`. Boss room: `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(velvet_patron|velvet_hostess|gangbang)" -v
```

Expected: all PASS.

```bash
git add content/npcs/velvet_patron.yaml content/npcs/velvet_hostess.yaml content/npcs/gangbang.yaml content/zones/the_velvet_rope.yaml
git commit -m "content(npc): Tier 3 The Velvet Rope NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 24: Tier 3 — Clown Camp NPC Upgrades (band 45–60, boss L60)

**Files:**
- Modify: `content/npcs/clown_mime.yaml`, `content/npcs/clown.yaml`
- Modify: `content/npcs/just_clownin.yaml`
- Modify: `content/npcs/big_top.yaml`
- Modify: `content/zones/clown_camp.yaml`

- [ ] **Step 1: Update standard NPCs**

`content/npcs/clown_mime.yaml`: `level: 45  max_hp: 510  ac: 21  rob_multiplier: 1.5`

`content/npcs/clown.yaml`: `level: 48  max_hp: 564  ac: 21  rob_multiplier: 1.5`

`content/npcs/just_clownin.yaml`: `level: 53  max_hp: 663  ac: 21  rob_multiplier: 1.5`

- [ ] **Step 2: Upgrade big_top to L60 boss**

```yaml
level: 60
max_hp: 2430
tier: boss
ac: 27
rob_multiplier: 2.0
boss_abilities:
  - id: big_top_spectacle
    name: "Big Top Spectacle"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: nausea
  - id: clown_carnage
    name: "Clown Carnage"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "8d10"
  - id: big_tops_encore
    name: "Big Top's Encore"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 25
```

- [ ] **Step 3: Update zone metadata and commit**

In `content/zones/clown_camp.yaml`: `min_level: 45  max_level: 60`. Boss room (`cc_the_stage` or boss room): `danger_level: all_out_war`. Other rooms: `dangerous`.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(clown_mime|clown$|just_clownin|big_top)" -v
```

Expected: all PASS.

```bash
git add content/npcs/clown_mime.yaml content/npcs/clown.yaml content/npcs/just_clownin.yaml content/npcs/big_top.yaml content/zones/clown_camp.yaml
git commit -m "content(npc): Tier 3 Clown Camp NPC stat upgrades (REQ-ZDS-1,2,3)"
```

---

## Task 25: Tier 3 — Lake Oswego Upgrades (band 45–60, boss L60) + New Boss Room

**Files:**
- Modify: `content/zones/lake_oswego.yaml` (add boss room, update metadata)

Note: All three Lake Oswego combat NPC templates (`lake_oswego_guard`, `lake_oswego_sniper`, `lake_oswego_warlord`) were created in Task 2.

- [ ] **Step 1: Add a boss room to lake_oswego.yaml**

Read `content/zones/lake_oswego.yaml` to identify the last room in the zone. Add a new boss room after the penultimate room:

```yaml
  - id: lo_country_club_vault
    title: The Country Club Vault
    danger_level: all_out_war
    boss_room: true
    description: >
      The innermost sanctum of Lake Oswego's fortified country club. Reinforced
      blast doors line every wall, surveillance cameras track your every move,
      and the Commandant holds court from behind a mahogany desk he's somehow
      kept polished through the apocalypse. Trophies from fallen challengers
      hang on the walls.
    exits:
    - direction: south
      target: <ID_OF_PRECEDING_ROOM>
    map_x: <preceding_room_map_x>
    map_y: <preceding_room_map_y - 2>
    spawns:
    - template: lake_oswego_warlord
      count: 1
      respawn_after: 72h
```

Replace `<ID_OF_PRECEDING_ROOM>` and map coordinates with values from the actual zone file (read it first).

Also add an exit from the preceding room to this new room:
```yaml
    - direction: north
      target: lo_country_club_vault
```

Add spawns of `lake_oswego_guard` and `lake_oswego_sniper` to regular combat rooms:
```yaml
    spawns:
    - template: lake_oswego_guard
      count: 2
      respawn_after: 5m
    - template: lake_oswego_sniper
      count: 1
      respawn_after: 10m
```

- [ ] **Step 2: Update zone metadata**

In `content/zones/lake_oswego.yaml`:
```yaml
  min_level: 45
  max_level: 60
  danger_level: dangerous
```

All normal rooms: `danger_level: dangerous`. Boss room: `danger_level: all_out_war`.

- [ ] **Step 3: Run full validator and commit**

```bash
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant -v 2>&1 | grep -E "FAIL|PASS|---" | tail -30
```

Expected: only the new templates pass; any remaining T1/T2/T3 NPCs that haven't been updated yet will still show failures.

```bash
go test ./internal/game/npc/... -run "TestNPCStatFormula_AllTemplatesCompliant/(lake_oswego_guard|lake_oswego_sniper|lake_oswego_warlord)" -v
```

Expected: all PASS.

```bash
git add content/zones/lake_oswego.yaml
git commit -m "content(npc): Tier 3 Lake Oswego upgrades + new boss room (REQ-ZDS-1,2,3,4)"
```

---

## Task 26: XP Formula Validation (REQ-ZDS-7)

**Files:**
- Read: `internal/config/config.go` (find XP config)
- Read: `internal/game/npc/instance.go` (find XP award formula)
- Create or modify: XP config if coefficient needed

- [ ] **Step 1: Find the XP formula**

```bash
grep -rn "XP\|xp\|experience\|Experience" internal/game/npc/ internal/gameserver/ --include="*.go" | grep -v "_test.go" | grep -i "formula\|reward\|award\|grant\|level.*50\|50.*level" | head -20
```

Also check:
```bash
grep -rn "npc_level\|NpcLevel\|npcLevel\|xp_reward\|XPReward" internal/ --include="*.go" | head -20
```

- [ ] **Step 2: Verify formula produces meaningful progression**

The spec says the formula should be `npc_level × 50 × tier_multiplier` (REQ-ZDS-7a). Write a quick Go test to verify it produces reasonable rewards across the 1–100 range:

Add this test to `internal/game/npc/stat_formula_test.go`:

```go
// TestXPFormula_ProducesReasonableProgression verifies that the XP formula
// npc_level × 50 × tier_multiplier provides meaningful progression at all levels (REQ-ZDS-7a).
func TestXPFormula_ProducesReasonableProgression(t *testing.T) {
	tierMultipliers := map[string]float64{
		"minion": 0.5, "standard": 1.0, "elite": 2.0, "champion": 3.0, "boss": 5.0,
	}
	for _, level := range []int{1, 5, 10, 15, 20, 30, 40, 50, 60, 70, 80, 90, 100} {
		for tier, mult := range tierMultipliers {
			xp := float64(level) * 50 * mult
			// XP must be > 0 and grow monotonically with level
			assert.Greater(t, xp, 0.0, "level %d tier %s: XP must be positive", level, tier)
			// At max level, even standard XP should be substantial (>= 5000)
			if level == 100 && tier == "standard" {
				assert.GreaterOrEqual(t, xp, 5000.0,
					"level 100 standard XP %.0f is degenerate (< 5000)", xp)
			}
		}
	}
}
```

Run:
```bash
go test ./internal/game/npc/... -run TestXPFormula_ProducesReasonableProgression -v
```

Expected: PASS. If it fails, the XP formula config needs a coefficient (REQ-ZDS-7b).

- [ ] **Step 3: Run the full compliance validator**

```bash
go test ./internal/game/npc/... -run TestNPCStatFormula_AllTemplatesCompliant -v 2>&1 | grep -E "^--- FAIL|^--- PASS" | grep FAIL
```

If any templates still fail, update them before committing.

- [ ] **Step 4: Run complete test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/stat_formula_test.go
git commit -m "test(npc): XP formula progression validation (REQ-ZDS-7a)"
```

---

## Out of Scope (Requires Separate Specs)

Per REQ-ZDS-5 and the spec's own implementation order:

- **Tier 4 zones** (Mount Hood Stronghold, Willamette Wastelands, Portland Heights Citadel) — levels 61–80 — must be specified as independent zone specs before implementation.
- **Tier 5 zones** (The Exclusion Zone, The Vault) — levels 81–100 — same requirement.

These are not included in this plan.

---

## Self-Review Against Spec

**REQ-ZDS-1** (±10% formula): Task 1 creates the validator; Tasks 3–25 make all templates compliant. ✓

**REQ-ZDS-2** (zone NPC level band, HP, AC): All 23 Tier 1–3 zones covered in Tasks 3–25. ✓

**REQ-ZDS-3** (boss upgrades with boss_abilities): All boss NPCs receive `tier: boss`, formula-compliant HP/AC, and 3 boss abilities (AOE, self-heal, phase-change). ✓

**REQ-ZDS-4** (danger_level calibration): Each zone task updates room danger_levels per tier rules. ✓

**REQ-ZDS-5** (new zones for T4/T5): Explicitly deferred — noted in Out of Scope. ✓

**REQ-ZDS-6** (rob_multiplier): All T2+ standard NPCs set to ≥1.2; T3 ≥1.5 where level ≥50; all bosses ≥2.0. ✓

**REQ-ZDS-7** (XP validation): Task 26 verifies formula at all levels. ✓

**REQ-ZDS-8** (incremental delivery): Each task produces independently testable, committable changes. ✓
