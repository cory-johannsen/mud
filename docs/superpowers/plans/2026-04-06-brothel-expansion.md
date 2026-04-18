# Brothel Expansion — Implementation Plan

**Branch:** `feature/brothel-expansion`
**Worktree:** `.worktrees/brothel-expansion`
**Spec:** `docs/superpowers/specs/2026-04-04-brothel-expansion-design.md`

## Context

This plan adds a `brothel_keeper` NPC type and a brothel room to every zone. Brothels offer
cheaper rest than motels with risk/reward trade-offs: 15% disease chance (10 new diseases),
20% robbery chance (5% crypto + backpack items), and a 1-day +1 Flair bonus. Each brothel
also houses a black market merchant and the zone's Fixer (relocated from its current room).

## Key Patterns Reference

**Working directory:** `/home/cjohannsen/src/mud/.worktrees/brothel-expansion`
**Run tests:** `mise exec -- go test ./...`

### Existing motel rest flow (to model brothel on):
- `handleRest` in `grpc_service.go` — routes based on danger level + NPC scan
- `handleMotelRest(uid, sess, motelNPC, sendMsg, requestID, stream)` — checks cost, deducts currency, calls `applyFullLongRestCtx`
- `applyFullLongRestCtx(uid, sess, ctx, sendMsg, promptFn)` — does HP/tech restore

### Key types:
- `npc.MotelConfig` in `internal/game/npc/noncombat.go` — model `BrothelConfig` on this
- `npc.Template.Motel *MotelConfig` in `internal/game/npc/template.go` — model `Brothel` field similarly
- `session.PlayerSession.Currency int` — deduct directly, save via `s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency)`
- `session.PlayerSession.Backpack *inventory.Backpack` — use `.Items()`, `.Remove(instanceID, qty)`, save via `s.charSaver.SaveInventory(ctx, sess.CharacterID, sess.Backpack)`
- `s.ApplySubstanceByID(uid, substanceID string) error` — apply disease substance
- Combat condition application: `cbt.ApplyCondition(uid, condID, stacks, durationSeconds)` or the condition registry directly

### POI pattern:
- `internal/game/maputil/poi.go` — `POITypes` slice and `NpcRoleToPOIID` switch

---

## Tasks

### Task 1 — BrothelConfig type + brothel_keeper NPC support

**Files:** `internal/game/npc/noncombat.go`, `internal/game/npc/template.go`, `internal/game/npc/template_test.go`

Add `BrothelConfig` struct and `Validate()` to `noncombat.go`:

```go
type BrothelConfig struct {
    RestCost         int      `yaml:"rest_cost"`
    DiseaseChance    float64  `yaml:"disease_chance"`
    RobberyChance    float64  `yaml:"robbery_chance"`
    DiseasePool      []string `yaml:"disease_pool"`
    FlairBonusDur    string   `yaml:"flair_bonus_duration"`
}

func (c *BrothelConfig) Validate() error {
    if c.RestCost <= 0 {
        return fmt.Errorf("brothel: rest_cost must be > 0")
    }
    if c.DiseaseChance < 0 || c.DiseaseChance > 1 {
        return fmt.Errorf("brothel: disease_chance must be in [0.0, 1.0]")
    }
    if c.RobberyChance < 0 || c.RobberyChance > 1 {
        return fmt.Errorf("brothel: robbery_chance must be in [0.0, 1.0]")
    }
    if len(c.DiseasePool) == 0 {
        return fmt.Errorf("brothel: disease_pool must not be empty")
    }
    if _, err := time.ParseDuration(c.FlairBonusDur); err != nil {
        return fmt.Errorf("brothel: flair_bonus_duration %q is not a valid Go duration: %w", c.FlairBonusDur, err)
    }
    return nil
}
```

In `template.go`:
- Add `Brothel *BrothelConfig \`yaml:"brothel,omitempty"\`` field to `Template` struct
- Add `"brothel_keeper": true` to the `validTypes` map
- Add validation case for `"brothel_keeper"`:
  ```go
  case "brothel_keeper":
      if t.Brothel == nil {
          return fmt.Errorf("npc template %q: npc_type 'brothel_keeper' requires a brothel: config block", t.ID)
      }
      if err := t.Brothel.Validate(); err != nil {
          return fmt.Errorf("npc template %q: %w", t.ID, err)
      }
  ```

**Tests** (property-based per SWENG-5a):
- `BrothelConfig.Validate()` passes for valid config
- Fails for `rest_cost <= 0`, `disease_chance` or `robbery_chance` outside `[0,1]`, empty `disease_pool`, invalid `flair_bonus_duration`
- Use `pgregory.net/rapid` for property tests on boundary values

**Commit** when tests pass.

---

### Task 2 — Map POI: motel and brothel symbols

**Files:** `internal/game/maputil/poi.go`, `internal/game/maputil/poi_test.go`

In `POITypes`, insert before the `"npc"` entry:
```go
{ID: "motel",   Symbol: 'R', Color: "\033[95m", Label: "Motel"},
{ID: "brothel", Symbol: 'B', Color: "\033[91m", Label: "Brothel"},
```

In `NpcRoleToPOIID`:
```go
case "motel_keeper":
    return "motel"
case "brothel_keeper":
    return "brothel"
```

**Tests:**
- `NpcRoleToPOIID("motel_keeper")` → `"motel"` (REQ-BR-T7)
- `NpcRoleToPOIID("brothel_keeper")` → `"brothel"` (REQ-BR-T8)
- Update `TestSortPOIs_KnownOrder` to include `"motel"` and `"brothel"` in the expected order
- Update `TestSortPOIs_DoesNotMutateInput` known IDs list
- The property-based test in `TestNpcRoleToPOIID_EmptyAlwaysEmpty` must exclude `"motel_keeper"` and `"brothel_keeper"` from the "maps to npc" filter

**Note:** `"motel_keeper"` previously fell through to `"npc"` — the new `"motel"` mapping is a behavior change. The existing test that expected `"npc"` for `"motel_keeper"` does NOT exist (check before assuming); if it does, update it.

**Commit** when tests pass.

---

### Task 3 — Content: disease substances, flair_bonus_1 condition, black_market merchant type

**Part A — 10 disease substance YAMLs** in `content/substances/`:

All have `category: disease`, `addictive: false`, `addiction_chance: 0.0`.

| ID | Name | Effects |
|----|------|---------|
| `street_fever` | Street Fever | `-1 grit` condition 4h; HP drain 1d4 per tick 1h |
| `crotch_rot` | Crotch Rot | `enfeebled` condition stacks 1, duration 8h |
| `swamp_itch` | Swamp Itch | `-1 savvy` condition 6h; `sickened` stacks 1, duration 2h |
| `track_rash` | Track Rash | `sickened` stacks 1, duration 4h; HP drain 1 per tick 30m |
| `gutter_flu` | Gutter Flu | `-1 grit` condition 6h; `-1 flair` condition 6h |
| `rust_pox` | Rust Pox | `enfeebled` stacks 2, duration 2h |
| `neon_blight` | Neon Blight | `-1 savvy` condition 8h; `-1 grit` condition 8h |
| `wet_lung` | Wet Lung | HP drain 2 per tick 2h; `sickened` stacks 1, duration 1h |
| `chrome_mange` | Chrome Mange | `-1 flair` condition 12h |
| `black_tongue` | Black Tongue | `sickened` stacks 2, duration 3h |

Model these on existing substances like `cheap_whiskey.yaml` and `tweaker_crystal.yaml`. Use `apply_condition` effect type for conditions. For HP drain, use the same pattern as other drain substances. Set `onset_delay: "0s"`, appropriate `duration`, `recovery_duration: "0s"`, `overdose_threshold: 0`.

**Part B — flair_bonus_1 condition** in `content/conditions/flair_bonus_1.yaml`:

Look at existing condition YAML files (e.g., `content/conditions/drunk.yaml`, `content/conditions/charmed.yaml`) to understand all fields.
The condition gives +1 Flair modifier. Set `duration_type: timed`. Duration is applied at runtime (not in YAML).

```yaml
id: flair_bonus_1
name: Flair Bonus
description: "You feel unusually confident."
duration_type: timed
max_stacks: 1
flair_bonus: 1
```

Check how other conditions express attribute bonuses (look at `content/conditions/` files that have attack_bonus, damage_bonus, etc.). Use whichever field name the condition system uses for Flair bonuses.

**Part C — black_market merchant type** in `internal/game/npc/noncombat.go`:

Find where merchant type is validated (look for `"general"`, `"specialty"`, `"fence"` or similar in `noncombat.go`). Add `"black_market"` to the valid merchant type list. No new behavior required — just make it a valid value.

**Tests:**
- REQ-BR-T9: All 10 disease substance YAML files load without errors. Write a table-driven test that loads each by ID from the substance registry and asserts no error.
- Validate `flair_bonus_1` condition YAML parses correctly (add to an existing condition load test if one exists, or create a simple test).

**Commit** when tests pass.

---

### Task 4 — Rest handler refactor + handleBrothelRest

**File:** `internal/gameserver/grpc_service.go`, `internal/gameserver/grpc_service_rest_test.go` (create or extend)

**Step 1 — Extract `applyLongRestEffects`:**

From `handleMotelRest`, extract the long-rest restoration logic into:
```go
func (s *GameServiceServer) applyLongRestEffects(uid string, sess *session.PlayerSession, ctx context.Context, sendMsg func(string) error, requestID string, stream gamev1.GameService_SessionServer) error
```

This function performs HP restoration, tech pool restore, and prepared tech selection (the parts currently in `applyFullLongRestCtx`). It should internally build the `promptFn` and call `applyFullLongRestCtx`.

Update `handleMotelRest` to call `applyLongRestEffects` instead of directly calling `applyFullLongRestCtx`.

**Step 2 — Add `handleBrothelRest`:**

```go
func (s *GameServiceServer) handleBrothelRest(uid string, sess *session.PlayerSession, brothelNPC *npc.Instance, sendMsg func(string) error, requestID string, stream gamev1.GameService_SessionServer) error
```

Implementation order per spec:
1. Check `sess.Currency < brothelNPC.BrothelConfig.RestCost` → block with message (REQ-BR-6)
2. Deduct crypto + save currency (REQ-BR-7)
3. Send confirmation message, call `applyLongRestEffects` (REQ-BR-7)
4. Apply `flair_bonus_1` condition for `FlairBonusDur` duration (REQ-BR-8). Message: `"You feel unusually confident. (+1 Flair)"`
5. Roll disease: `rand.Float64() < brothelNPC.BrothelConfig.DiseaseChance` → pick random pool entry, call `s.ApplySubstanceByID(uid, diseaseID)`. On error, log warning, do not block (REQ-BR-9, REQ-BR-13). Send disease message.
6. Roll robbery: `rand.Float64() < brothelNPC.BrothelConfig.RobberyChance` → steal crypto + items (REQ-BR-10). Send message: `"You wake to find someone has gone through your belongings."`. Save inventory.

**Robbery logic:**
```go
if sess.Currency > 0 {
    stolenCrypto = max(1, sess.Currency * 5 / 100)  // integer floor
    sess.Currency -= stolenCrypto
    // save currency
}
backpackItems := sess.Backpack.Items()
numToSteal := len(backpackItems) * 5 / 100  // floor
// randomly select numToSteal items; for each: Remove(item.InstanceID, 1) 
// (removes one stack or the whole item)
// save inventory: s.charSaver.SaveInventory(ctx, sess.CharacterID, sess.Backpack)
```

For condition application, look at how `handleMotelRest` or nearby code in the file applies conditions at runtime. The `condRegistry` is on `s.condRegistry`. Check `grpc_service.go` for `ApplyCondition` or `condRegistry` usage to find the right call pattern. Duration in seconds = parse `brothelNPC.BrothelConfig.FlairBonusDur` via `time.ParseDuration`.

**Step 3 — Update `handleRest` dispatch:**

In `handleRest`'s `case danger.Safe:` block, after the motel NPC scan, add a second scan for `brothel_keeper`:
```go
// REQ-BR-4: safe room + brothel NPC → brothel rest.
for _, npcInst := range s.npcMgr.InstancesInRoom(sess.RoomID) {
    if npcInst.NPCType == "brothel_keeper" {
        return s.handleBrothelRest(uid, sess, npcInst, sendMsg, requestID, stream)
    }
}
```
Place this AFTER the motel scan so motel takes priority when both are present (shouldn't happen, but safe default).

**Note on BrothelConfig on Instance:** The `npc.Instance` currently has `RestCost` promoted from `MotelConfig`. Check how `Instance` is built from `Template` (look for `RestCost` assignment in `npc/instance.go` or similar). Add equivalent promotion for `BrothelConfig` fields — either embed `*BrothelConfig` directly on `Instance`, or promote individual fields. Match whatever pattern `RestCost` uses.

**Tests** (REQ-BR-T2 through REQ-BR-T6):
- Create `internal/gameserver/grpc_service_brothel_rest_test.go`
- Test insufficient credits → blocks rest (REQ-BR-T2)
- Test sufficient credits → full restoration + Flair bonus (REQ-BR-T3)
- Test disease roll at 100% → `ApplySubstanceByID` called with pool member (REQ-BR-T4)
- Test robbery roll at 100% → crypto deducted + backpack item removed (REQ-BR-T5)
- Test disease + robbery at 0% → state unchanged (REQ-BR-T6)

Use `pgregory.net/rapid` for property tests where applicable.

**Commit** when all tests pass.

---

### Task 5 — Zone content: brothel rooms, NPC templates, Fixer relocation

**For every zone YAML in `content/zones/`** (23 files), **and corresponding NPC files**:

1. Determine whether the zone already has a Fixer NPC in one of its rooms. Check `content/zones/<zone>.yaml` for spawn entries with template IDs containing `"fixer"`.
2. Add a new room with `danger_level: safe`, lore-appropriate name/title/description, connected to the existing safe cluster.
3. Add three spawns to the new brothel room:
   - `<zone>_brothel_keeper` (new template, see below)
   - `<zone>_black_market_merchant` (new template, see below)  
   - The zone's existing Fixer template ID (remove Fixer spawn from its current room)
4. If a zone has NO fixer yet, create one (template ID: `<zone>_fixer`, npc_type: `fixer`).

**NPC Template files to create per zone:**

Create `content/npcs/non_combat/<zone>.yaml` entries (or add to existing file for that zone):

**brothel_keeper template pattern:**
```yaml
- id: <zone>_brothel_keeper
  name: "<lore-appropriate name>"
  npc_type: brothel_keeper
  npc_role: brothel_keeper
  type: human
  description: "<brief lore description>"
  level: 3
  max_hp: 20
  ac: 10
  awareness: 8
  disposition: neutral
  personality: neutral
  respawn_delay: "15m"
  abilities:
    brutality: 8
    grit: 10
    quickness: 11
    reasoning: 10
    savvy: 13
    flair: 14
  brothel:
    rest_cost: <lower than zone motel cost>
    disease_chance: 0.15
    robbery_chance: 0.20
    disease_pool:
      - street_fever
      - crotch_rot
      - swamp_itch
      - track_rash
      - gutter_flu
      - rust_pox
      - neon_blight
      - wet_lung
      - chrome_mange
      - black_tongue
    flair_bonus_duration: "24h"
  loot:
    currency:
      min: 0
      max: 0
```

**black_market_merchant template pattern:**
```yaml
- id: <zone>_black_market_merchant
  name: "<lore-appropriate name>"
  npc_type: merchant
  npc_role: merchant
  type: human
  description: "<brief lore description>"
  level: 4
  max_hp: 22
  ac: 11
  awareness: 10
  disposition: neutral
  personality: neutral
  respawn_delay: "15m"
  abilities:
    brutality: 9
    grit: 11
    quickness: 12
    reasoning: 12
    savvy: 14
    flair: 11
  merchant:
    merchant_type: black_market
    buy_markup: 1.5
    sell_discount: 0.6
  loot:
    currency:
      min: 10
      max: 50
```

**Motel rest costs by zone** (check existing motel NPC templates for each zone to find current rest costs, then set brothel cost lower — typically 50-70% of motel cost):

For each zone, read its existing motel NPC YAML to find `rest_cost`, then set brothel `rest_cost` to approximately 60% (rounded to nearest 5).

**Zone name and lore guidance:**
- Use the zone's existing lore theme for room names and NPC names
- Room IDs: `<zone>_brothel`
- Room titles: thematic (e.g., "The Rusty Latch" for rustbucket_ridge, "The Velvet Underground" for downtown)

**Known Fixer locations** (from existing content — these must be MOVED to the brothel room):
- `aloha`: `aloha_fixer` in `aloha_the_bazaar`
- `downtown`: `downtown_fixer` in `downtown_underground`
- `felony_flats`: `felony_flats_fixer` in `flats_mechanic_lot`
- `the_couve`: `the_couve_fixer` in `couve_the_crossing`
- `oregon_country_fair`: `juggalo_fixer` in `the_gathering_ground`, `tweaker_fixer` in `tweaker_command_post`, `wook_fixer` in `the_wook_council_fire` (pick one per zone logic, or put all 3 in the brothel)
- `wooklyn`: `wook_fixer` in `tofteville_market`
- All other zones: check zone YAML for existing fixer; create `<zone>_fixer` template if none found.

**Commit** when all zone content is in place and `mise exec -- go build ./...` passes (content is YAML — no unit test required, but the build loads all YAML at startup so a build+integration test pass confirms correctness).

---

## Completion Checklist

- [ ] Task 1: BrothelConfig + brothel_keeper validated, tests pass
- [ ] Task 2: motel + brothel POI types, tests pass
- [ ] Task 3: 10 disease YAMLs + flair_bonus_1 + black_market merchant type, tests pass
- [ ] Task 4: applyLongRestEffects extracted + handleBrothelRest implemented, all tests pass
- [ ] Task 5: all zones have brothel room + NPC templates, build passes
- [ ] Full test suite: `mise exec -- go test ./...` green
- [ ] Feature marked `done` in `docs/features/index.yaml`
