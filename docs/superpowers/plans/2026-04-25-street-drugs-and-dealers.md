# Street Drugs and Dealers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Author drug content and dealer NPC instances on top of the existing consumable + merchant substrate. Add four small mechanic extensions: a `DrugBlock` typed substructure on `ConsumableEffect`; a buff-expiry → aftermath dispatch keyed by a `Source` sentinel; a per-character drug-use ledger (`character_drug_use`) for the future addiction system; and an NPC `archetype` field validating dealer types against the merchant config. Five exemplar drugs and three exemplar dealers.

**Spec:** [docs/superpowers/specs/2026-04-25-street-drugs-and-dealers.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-street-drugs-and-dealers.md) (PR [#285](https://github.com/cory-johannsen/mud/pull/285))

**Architecture:** Three layers, all sitting on top of already-shipped infrastructure. (1) Content + schema — `DrugBlock{category, severity, aftermath, addiction_potency}` added to `ConsumableEffect`; loader auto-tags `drug` items and applies severity-default aftermath when omitted. (2) Runtime — `ApplyConsumable` tags each applied buff condition with `Source = "drug:<item_id>:<condition_id>"` and a generated `drug_use_id` so aftermath is single-fired per consumption event. The condition-expiry tick consults `Source`; on the *last originally-scheduled* expiry for a `drug_use_id`, the runtime applies the drug's `aftermath` list with `Source = "drug_aftermath:<item_id>"` so they do not cascade. (3) Persistence — a new `character_drug_use` table records every consumption with category, addiction-potency, `used_at`, and source (`dealer` / `looted` / `crafted` / `gift` / `unknown`). Dealer NPCs ride on the existing `MerchantConfig.MerchantType` enum (`drugs` and `black_market` already valid); a new `archetype` field on `NPCTemplate` validates that the merchant type matches.

**Tech Stack:** Go (`internal/game/inventory/`, `internal/game/condition/`, `internal/game/npc/`, `internal/storage/postgres/`, `internal/gameserver/`), Postgres, telnet, React/TypeScript (`cmd/webclient/ui/src/`).

**Prerequisite:** None hard. #252 is a soft dep — the condition catalog (`sickened`, `drained`, `fatigued`, `frightened`, `quickened`, etc.) is shared. Existing merchant pipeline at `internal/game/npc/merchant.go` and `MerchantConfig` already supports drug merchants; this plan does not modify pricing.

**Note on spec PR**: Spec is on PR #285, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/inventory/consumable.go` (`DrugBlock` struct; ledger insert hook in `ApplyConsumable`) |
| Modify | `internal/game/inventory/consumable_test.go` |
| Modify | `internal/game/inventory/item.go` (`ItemDef.Tags`; `drug` auto-tag) |
| Modify | `internal/game/inventory/item_test.go` |
| Modify | `internal/game/condition/active.go` (`Source string`; `DrugUseID string`; `DrugAftermath` table) |
| Modify | `internal/game/condition/tick.go` (aftermath dispatch on expiry) |
| Modify | `internal/game/condition/tick_test.go` |
| Modify | `internal/game/npc/template.go` (`Archetype string`; loader validation) |
| Modify | `internal/game/npc/template_test.go` |
| Create | `internal/storage/postgres/drug_use.go` |
| Create | `internal/storage/postgres/drug_use_test.go` |
| Modify | `internal/gameserver/grpc_service.go` (set `source` on buy / loot paths) |
| Modify | `internal/gameserver/grpc_service_use_test.go` |
| Create | `migrations/NNN_character_drug_use.up.sql`, `.down.sql` |
| Create | `content/items/drugs/combat_juice.yaml`, `nightshade_tab.yaml`, `crash_capsule.yaml`, `wide_awake.yaml`, `cog_oil.yaml` |
| Create | `content/npcs/dealers/` (3 dealer YAML files; rooms TBD with user) |
| Modify | `cmd/webclient/ui/src/game/inventory/ItemBadge.tsx` |
| Modify | `cmd/webclient/ui/src/game/character/ConditionBadge.tsx` |
| Modify | `internal/frontend/telnet/inventory_render.go` |
| Modify | `internal/frontend/telnet/status_render.go` |
| Modify | `docs/architecture/items.md` (or new doc) |

---

### Task 1: `DrugBlock` schema + auto-tag

**Files:**
- Modify: `internal/game/inventory/consumable.go`
- Modify: `internal/game/inventory/item.go`
- Modify: `internal/game/inventory/consumable_test.go` and `item_test.go`

- [ ] **Step 1: Failing tests** (DRUG-1, DRUG-2, DRUG-3, DRUG-4):

```go
func TestDrugBlock_LoadsAllFields(t *testing.T) {
    item, _ := inventory.LoadItem([]byte(combatJuiceYAML))
    require.Equal(t, "combat_enhancer", item.Effect.Drug.Category)
    require.Equal(t, "moderate",        item.Effect.Drug.Severity)
    require.Equal(t, 1,                 item.Effect.Drug.AddictionPotency)
    require.NotEmpty(t, item.Effect.Drug.Aftermath)
}

func TestDrugBlock_SeverityDefaultMild(t *testing.T) {
    item, _ := inventory.LoadItem([]byte(`
id: cog_oil
effect:
  drug: { category: nootropic, severity: mild }
  conditions: [{ id: focused, duration: "30m" }]
`))
    require.Len(t, item.Effect.Drug.Aftermath, 1)
    require.Equal(t, "sickened", item.Effect.Drug.Aftermath[0].ConditionID)
    require.Equal(t, 1,          item.Effect.Drug.Aftermath[0].Stacks)
    require.Equal(t, 5*time.Minute, item.Effect.Drug.Aftermath[0].Duration)
}

func TestDrugBlock_SeverityDefaultModerate(t *testing.T) { ... }
func TestDrugBlock_SeverityDefaultSevere(t *testing.T)   { ... }

func TestItemAutoTagDrug_WhenDrugBlockPresent(t *testing.T) {
    item, _ := inventory.LoadItem([]byte(combatJuiceYAML))
    require.Contains(t, item.Tags, "drug")
}

func TestItemRejectsDrugTagWithoutBlock(t *testing.T) {
    _, err := inventory.LoadItem([]byte(`
id: bogus
tags: [drug]
effect:
  conditions: [{ id: focused, duration: "5m" }]
`))
    require.Error(t, err)
    require.Contains(t, err.Error(), "drug tag without Drug block")
}
```

- [ ] **Step 2: Implement** the schema:

```go
type ConsumableEffect struct {
    Heal              *Heal
    Conditions        []ConditionEffect
    RemoveConditions  []string
    ConsumeCheck      *ConsumeCheck
    Drug              *DrugBlock // NEW
}

type DrugBlock struct {
    Category         string // stimulant | depressant | psychoactive | nootropic | combat_enhancer
    Severity         string // mild | moderate | severe
    Aftermath        []ConditionEffect
    AddictionPotency int    // default 1
}

type ItemDef struct {
    // ... existing ...
    Tags []string
}
```

- [ ] **Step 3: Severity-default fill-in** in the loader:

```go
var severityDefaults = map[string][]ConditionEffect{
    "mild":     {{ConditionID: "sickened", Stacks: 1, Duration: 5*time.Minute}},
    "moderate": {{ConditionID: "sickened", Stacks: 2, Duration: 15*time.Minute}, {ConditionID: "fatigued", Duration: time.Hour}},
    "severe":   {{ConditionID: "sickened", Stacks: 2, Duration: 30*time.Minute}, {ConditionID: "fatigued", Duration: 2*time.Hour}, {ConditionID: "drained", Stacks: 1, Duration: 24*time.Hour}},
}

func resolveAftermath(d *DrugBlock) {
    if len(d.Aftermath) == 0 && d.Severity != "" {
        d.Aftermath = append([]ConditionEffect(nil), severityDefaults[d.Severity]...)
    }
}
```

- [ ] **Step 4: Auto-tag** + consistency check (DRUG-4):

```go
if item.Effect != nil && item.Effect.Drug != nil && !slices.Contains(item.Tags, "drug") {
    item.Tags = append(item.Tags, "drug")
}
if slices.Contains(item.Tags, "drug") && (item.Effect == nil || item.Effect.Drug == nil) {
    return nil, errors.New("item has 'drug' tag without Drug block")
}
```

---

### Task 2: Aftermath dispatch on expiry

**Files:**
- Modify: `internal/game/condition/active.go` (`Source` + `DrugUseID` fields)
- Modify: `internal/game/condition/tick.go`
- Modify: `internal/game/condition/tick_test.go`

- [ ] **Step 1: Add fields**:

```go
type ActiveCondition struct {
    // ... existing ...
    Source    string // e.g., "drug:combat_juice:strength_boost"
    DrugUseID string // shared across all conditions from one consumption
    LatestExpiryAt time.Time // captured at apply time so we know which is "last originally-scheduled"
}
```

- [ ] **Step 2: Failing tests** (DRUG-5, DRUG-6, DRUG-7, DRUG-8):

```go
func TestExpiryDispatch_DrugSourceFiresAftermath(t *testing.T) {
    cat := newCatalog()
    char := newChar(t)
    inv := newInventory(t, withItem(combatJuice))
    inv.Consume(char, "combat_juice", "dealer")
    advanceTime(t, char, 5*time.Minute+time.Second) // buff expires
    require.True(t, char.HasActive("sickened"), "DRUG-6 aftermath fires")
}

func TestExpiryDispatch_FiresOnceEvenWithMultipleBuffConditions(t *testing.T) {
    cat := newCatalog()
    char := newChar(t)
    inv := newInventory(t, withItem(twoBuffDrug))
    inv.Consume(char, "two_buff_drug", "dealer")
    advanceTime(t, char, 1*time.Minute+time.Second)  // first buff expires
    require.False(t, char.HasActive("sickened"), "aftermath waits for last expiry")
    advanceTime(t, char, 5*time.Minute+time.Second)  // second buff expires
    require.True(t, char.HasActive("sickened"), "aftermath fires on last expiry (DRUG-7)")
    // Apply once and only once.
    require.Equal(t, 1, char.AftermathFireCount())
}

func TestExpiryDispatch_NonDrugSourceDoesNotFireAftermath(t *testing.T) {
    char := newChar(t)
    char.ApplyCondition(condition.Active{ConditionID: "frightened", Source: "spell:fear"})
    advanceTime(t, char, time.Hour)
    require.Equal(t, 0, char.AftermathFireCount(), "DRUG-8 unrelated source does not cascade")
}

func TestExpiryDispatch_AftermathDoesNotCascade(t *testing.T) {
    char := newChar(t)
    inv := newInventory(t, withItem(severeDrug))
    inv.Consume(char, "severe_drug", "dealer")
    advanceTime(t, char, expiryAndThen24h)
    require.Equal(t, 1, char.AftermathFireCount(), "aftermath conditions don't trigger another aftermath")
}

func TestExpiryDispatch_LatestOriginallyScheduledTracking(t *testing.T) {
    // Q1: even if one buff is removed early by a follow-up consumable,
    // aftermath fires when the latest *originally-scheduled* expiry would have hit.
    char := newChar(t)
    inv := newInventory(t, withItem(twoBuffDrug))
    drugUseID := inv.Consume(char, "two_buff_drug", "dealer")
    char.RemoveCondition(...) // remove the first buff early, simulating a `remove_conditions` from a different consumable
    advanceTime(t, char, lateExpiry+time.Second) // would have been the last expiry
    require.True(t, char.HasActive("sickened"))
}
```

- [ ] **Step 3: Implement** the dispatch:

```go
func (cs *ConditionStore) Tick(now time.Time) {
    for _, ac := range cs.active {
        if ac.DurationRemaining(now) > 0 { continue }
        cs.removeAC(ac)
        if isDrugSource(ac.Source) && cs.isLastOriginallyScheduledForDrugUse(ac.DrugUseID, now) {
            cs.applyAftermath(ac.itemID(), ac.DrugUseID)
        }
    }
}

func (cs *ConditionStore) isLastOriginallyScheduledForDrugUse(drugUseID string, now time.Time) bool {
    // Look up all conditions ever applied with this drug_use_id; if their LatestExpiryAt <= now, this is the last.
    return cs.allDrugConditionsExpired(drugUseID, now)
}
```

`allDrugConditionsExpired` reads from a per-character drug-use registry that captures the original expiry timestamps at consumption time. The registry is in-memory (rebuilt at session restore from `character_drug_use` plus active conditions) since aftermath is itself a transient effect.

---

### Task 3: `character_drug_use` table + repository

**Files:**
- Create: `migrations/NNN_character_drug_use.up.sql`, `.down.sql`
- Create: `internal/storage/postgres/drug_use.go`
- Create: `internal/storage/postgres/drug_use_test.go`

- [ ] **Step 1: Author migration** (DRUG-9):

```sql
-- migrations/NNN_character_drug_use.up.sql
CREATE TABLE character_drug_use (
    id                BIGSERIAL PRIMARY KEY,
    character_id      TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    item_id           TEXT NOT NULL,
    category          TEXT NOT NULL,
    addiction_potency INTEGER NOT NULL DEFAULT 1,
    used_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    source            TEXT NOT NULL DEFAULT 'unknown'
);
CREATE INDEX cdu_by_char_used_at ON character_drug_use(character_id, used_at DESC);
```

- [ ] **Step 2: Failing tests** (DRUG-10, DRUG-11):

```go
func TestDrugUse_RecordsRowOnConsume(t *testing.T) {
    s := newPGStore(t)
    require.NoError(t, s.Record("c1", "combat_juice", "combat_enhancer", 1, "dealer"))
    rows, _ := s.RecentUse("c1", time.Now().Add(-time.Hour))
    require.Len(t, rows, 1)
    require.Equal(t, "combat_juice", rows[0].ItemID)
    require.Equal(t, "dealer", rows[0].Source)
}

func TestDrugUse_TotalPotencySumsRecent(t *testing.T) {
    s := newPGStore(t)
    s.Record("c1", "a", "stimulant", 1, "dealer")
    s.Record("c1", "b", "stimulant", 3, "dealer")
    n, _ := s.TotalPotency("c1", time.Now().Add(-time.Hour))
    require.Equal(t, 4, n)
}
```

- [ ] **Step 3: Implement** the repo:

```go
type DrugUseEvent struct {
    ID               int64
    ItemID           string
    Category         string
    AddictionPotency int
    UsedAt           time.Time
    Source           string
}

type Store interface {
    Record(charID, itemID, category string, potency int, source string) error
    RecentUse(charID string, since time.Time) ([]DrugUseEvent, error)
    TotalPotency(charID string, since time.Time) (int, error)
}
```

- [ ] **Step 4: Wire `ApplyConsumable`** to call `Store.Record(...)` whenever the consumed item has a `Drug` block. Source defaults to `"unknown"`; `handleBuy` sets `"dealer"`; loot path sets `"looted"`; future crafting sets `"crafted"`; gift/quest grants set `"gift"`.

---

### Task 4: NPC `archetype` field + dealer validation

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/template_test.go`

- [ ] **Step 1: Failing tests** (DRUG-12, DRUG-13, DRUG-15):

```go
func TestArchetype_StreetDealerRequiresDrugsMerchant(t *testing.T) {
    _, err := npc.LoadTemplate(`
id: jenkins_the_bad
archetype: street_dealer
merchant: { merchant_type: weapons }
`)
    require.Error(t, err)
    require.Contains(t, err.Error(), "merchant_type must be 'drugs' for street_dealer")
}

func TestArchetype_BlackMarketRequiresBlackMarketMerchant(t *testing.T) { ... }

func TestArchetype_DealerHasNoCombatProfile(t *testing.T) {
    tmpl, _ := npc.LoadTemplate(streetDealerYAML)
    require.Empty(t, tmpl.Abilities, "DRUG-15 dealers are non-combatant by default")
}
```

- [ ] **Step 2: Implement** the field:

```go
type Template struct {
    // ... existing ...
    Archetype string // street_dealer | black_market_dealer | (future)
}

func (t *Template) Validate() error {
    switch t.Archetype {
    case "":
        return nil
    case "street_dealer":
        if t.Merchant == nil || t.Merchant.MerchantType != "drugs" {
            return fmt.Errorf("merchant_type must be 'drugs' for street_dealer")
        }
    case "black_market_dealer":
        if t.Merchant == nil || t.Merchant.MerchantType != "black_market" {
            return fmt.Errorf("merchant_type must be 'black_market' for black_market_dealer")
        }
    default:
        return fmt.Errorf("unknown archetype %q", t.Archetype)
    }
    if t.Archetype != "" && len(t.Abilities) > 0 {
        return fmt.Errorf("dealers must not declare combat abilities (DRUG-15)")
    }
    return nil
}
```

---

### Task 5: Five exemplar drugs

**Files:**
- Create: `content/items/drugs/combat_juice.yaml`, `nightshade_tab.yaml`, `crash_capsule.yaml`, `wide_awake.yaml`, `cog_oil.yaml`

- [ ] **Step 1: Author** the five exemplars per DRUG-16:

```yaml
# content/items/drugs/combat_juice.yaml
id: combat_juice
display_name: Combat Juice
description: |
  Crude self-injecting stim. Brutally effective for a few minutes —
  punishingly bad afterwards. Not legal in any zone with rule of law.
weight: 0.1
stack: 5
base_price: 75
effect:
  drug:
    category: combat_enhancer
    severity: moderate
    addiction_potency: 2
    aftermath:
      - { condition_id: sickened, stacks: 2, duration: "30m" }
  conditions:
    - { condition_id: brutality_boost, stacks: 2, duration: "5m" }
    - { condition_id: quickness_boost, stacks: 2, duration: "5m" }
```

(Plus the four other YAML files matching the spec table in DRUG-16.)

- [ ] **Step 2: Confirm** that `brutality_boost` / `quickness_boost` etc. are valid condition IDs in the catalog. If not, either author them now or substitute the canonical PF2E equivalents (e.g., `quickened 1`, status bonus via a typed-bonus condition).

- [ ] **Step 3: Validate** by loading content and running smoke tests; all five drugs auto-tag `drug` per Task 1.

---

### Task 6: Three exemplar dealer NPCs

**Files:**
- Create: `content/npcs/dealers/jenkins_street_corner.yaml`, `gilda_warlord_dealer.yaml`, `marlowe_black_market.yaml`

- [ ] **Step 1: Checkpoint (DRUG-13).** Confirm with user the rooms/zones for each of the three dealers. Suggested defaults:
  - `jenkins_street_corner` — Desperate Streets / Armed & Dangerous zone street.
  - `gilda_warlord_dealer` — Warlord Territory zone.
  - `marlowe_black_market` — Apex Predator faction enclave with `min_faction_tier_id` gate.

- [ ] **Step 2: Author** the three YAML files:

```yaml
# content/npcs/dealers/jenkins_street_corner.yaml
id: jenkins_street_corner
display_name: Jenkins
description: A jittery man with too many pockets, leaning against a graffiti'd wall.
archetype: street_dealer
disposition: neutral
merchant:
  merchant_type: drugs
  inventory:
    - { item_id: combat_juice,   stock: 5,  replenish_rate: 1, replenish_period: "1h" }
    - { item_id: nightshade_tab, stock: 10, replenish_rate: 2, replenish_period: "30m" }
    - { item_id: wide_awake,     stock: 3,  replenish_rate: 1, replenish_period: "1h" }
  sell_margin: 1.4
  budget: 500
```

- [ ] **Step 3: Place** the dealer in the chosen room via that zone's YAML (`spawns:` block).

- [ ] **Step 4: Smoke test** end-to-end: enter the room, run `talk jenkins`, run `buy combat_juice`, verify the credit deduction, verify the inventory item appears with the `drug` tag, verify `character_drug_use` records nothing yet (not until `use`), then `use combat_juice` and verify both the buff conditions and (after time advance) the aftermath conditions.

---

### Task 7: UI surfacing — drug badges + 💊 prefix + telnet `[drug]`

**Files:**
- Modify: `cmd/webclient/ui/src/game/inventory/ItemBadge.tsx`
- Modify: `cmd/webclient/ui/src/game/character/ConditionBadge.tsx`
- Modify: `internal/frontend/telnet/inventory_render.go`
- Modify: `internal/frontend/telnet/status_render.go`

- [ ] **Step 1: Failing component tests** (DRUG-21, DRUG-22):

```ts
test("ItemBadge shows Drug badge for items with drug tag", () => {
  render(<ItemBadge item={{ tags: ["drug"], displayName: "Combat Juice" }} />);
  expect(screen.getByText(/Drug/i)).toBeVisible();
});

test("ConditionBadge prefixes drug-source conditions with 💊", () => {
  render(<ConditionBadge condition={{ id: "brutality_boost", source: "drug:combat_juice:..." }} />);
  expect(screen.getByText(/💊/)).toBeVisible();
});

test("ConditionBadge does NOT prefix non-drug-source conditions", () => {
  render(<ConditionBadge condition={{ id: "frightened", source: "spell:fear" }} />);
  expect(screen.queryByText(/💊/)).toBeNull();
});
```

- [ ] **Step 2: Failing telnet tests** (DRUG-23):

```go
func TestInventoryRender_DrugItemMarkedSuffix(t *testing.T) {
    out := renderInventory(t, withItem(combatJuiceWithTagDrug))
    require.Contains(t, out, "Combat Juice [drug]")
}

func TestStatusRender_DrugConditionMarkedSuffix(t *testing.T) {
    out := renderStatus(t, withActiveCondition("brutality_boost", "drug:combat_juice:..."))
    require.Contains(t, out, "Brutality +2 [drug]")
}
```

- [ ] **Step 3: Implement** both surfaces. The web condition badge reads the `source` field from the active-condition record (already serialized for the client per the existing condition surface).

---

### Task 8: Integration test + architecture docs

**Files:**
- Create: `internal/gameserver/grpc_service_drug_integration_test.go`
- Modify: `docs/architecture/items.md` (or create `docs/architecture/drugs.md`)

- [ ] **Step 1: Failing integration test** (DRUG-26):

```go
func TestDrugIntegration_BuyConsumExpireAftermathLedger(t *testing.T) {
    s := newServer(t, withDealer("jenkins"), withDrug("combat_juice"))
    char := newCharacter(t, s, withCredits(1000))
    enterRoom(s, char, "jenkins_corner")
    s.HandleBuy(ctx, &gamev1.BuyRequest{NpcUid: "jenkins", ItemId: "combat_juice", Qty: 1})
    require.Less(t, char.Credits, 1000)
    s.HandleUse(ctx, &gamev1.UseRequest{ItemId: "combat_juice"})
    require.True(t, char.HasActive("brutality_boost"))
    advanceTime(t, char, 5*time.Minute+time.Second)
    require.False(t, char.HasActive("brutality_boost"))
    require.True(t, char.HasActive("sickened"))
    rows, _ := s.DrugUseStore().RecentUse(char.ID, time.Now().Add(-time.Hour))
    require.Len(t, rows, 1)
    require.Equal(t, "dealer", rows[0].Source)
}
```

- [ ] **Step 2: Architecture doc** — section explaining:
  - The `DrugBlock` schema and severity-default fill-in.
  - The `Source = "drug:..."` sentinel and the aftermath dispatch contract.
  - The `character_drug_use` table and the `Store` API.
  - The `archetype` field on NPC templates and the merchant-type validation.
  - The future addiction-mechanic seam and how a follow-on spec will read `TotalPotency` and `RecentUse`.
  - Authoring guidance for new drugs (severity → suggested aftermath, addiction-potency tuning).
  - Dealer authoring guidance (margin ladder, tier gating).

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
make migrate-up && make migrate-down
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff.
- Telnet smoke test: see Task 6 step 4.
- Web smoke test: open the inventory panel, verify the Drug badge on Combat Juice; consume; verify the 💊 prefix on the Brutality boost condition; wait; verify the 💊 sickened condition appears.

---

## Rollout / Open Questions Resolved at Plan Time

- **DRUG-Q1**: Aftermath fires on the *latest originally-scheduled* expiry, captured at apply time. Even if a buff is removed early by an unrelated `remove_conditions`, the aftermath still fires at the original schedule.
- **DRUG-Q2**: No further addiction-mechanic hints in the schema. The ledger is the seam.
- **DRUG-Q3**: Black-market dealers reuse the existing `min_faction_tier_id` gate. New gating mechanisms are a follow-on.
- **DRUG-Q4**: Drugs are player-only in v1. NPC poisoning / force-feed is a future skill action.
- **DRUG-Q5**: `archetype` stays narrow to merchant flavor in v1. Combat / behavior archetypes belong elsewhere.

## Non-Goals Reaffirmed

Per spec §2.2:

- No addiction mechanic (tolerance / withdrawal / recovery).
- No drug crafting or synthesis.
- No drug-related quests.
- No NPC reactions to intoxicated players.
- No dealer haggling beyond the existing merchant negotiation.
- No pharmacy / legal-medication parallel.
