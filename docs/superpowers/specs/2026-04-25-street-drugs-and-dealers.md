---
title: Street Drugs and Dealers
issue: https://github.com/cory-johannsen/mud/issues/258
date: 2026-04-25
status: spec
prefix: DRUG
depends_on: []
related:
  - "Existing Consumable model and `MerchantConfig` (MerchantType already supports `drugs` and `black_market`)"
  - "#252 Non-combat actions (condition catalog target for buffs and side-effects)"
  - "#257 Social encounters (dealer encounter format)"
---

# Street Drugs and Dealers

## 1. Summary

The runtime substrate for drugs and dealer NPCs already exists:

- `ItemDef.Effect *ConsumableEffect` (`internal/game/inventory/item.go:35`) supports `Heal`, `Conditions []ConditionEffect` (id + duration), `RemoveConditions`, and `ConsumeCheck` (PF2E four-tier flat-check).
- `inventory.ApplyConsumable` (`consumable.go:120`) orchestrates the full apply-on-use flow that's already wired into `handleUse` (`grpc_service.go:8333-8351`).
- `npc.MerchantConfig.MerchantType` (`noncombat.go:11-20`) already enumerates `"drugs"` and `"black_market"` as valid types.
- Wanted-level price surcharge is already baked into `ComputeBuyPrice` (`merchant.go:78-79`), so a high-wanted player buying from a black-market dealer already pays more.

What is missing is content and four small mechanic extensions:

1. **Drug content.** No items in `content/items/` are tagged or authored as drugs.
2. **Dealer NPC instances.** Although `MerchantType: drugs / black_market` is a valid config, no NPC template carries it.
3. **Side-effect on expiry.** Today a `ConsumableEffect` applies conditions for a duration; there is no notion of "and when each condition expires, apply this aftermath". The issue asks for sickened/drained/withdrawal effects that fire after the buff fades.
4. **Repeated-use tracking.** The issue calls out "addiction risk (future extension point)" — even if the addiction mechanic is deferred, the *counter* to drive future addiction needs to be recorded now so we don't re-migrate later.

This spec adds:
- A `Drug` item subtype (a typed `ItemDef.Effect.Drug` block) with declared buff conditions, post-expiry side-effect conditions, and category/severity metadata.
- A `Dealer` NPC archetype as a thin wrapper around `MerchantConfig` with a fixed `MerchantType: drugs` (street dealer) or `black_market` (high-end fixer). One template per archetype with reusable randomized inventories.
- A per-character drug-use ledger (`character_drug_use`) that records every drug consumption event so the future addiction system can read it without re-migrating.
- Five exemplar drugs and three exemplar dealer NPCs.

## 2. Goals & Non-Goals

### 2.1 Goals

- DRUG-G1: A drug is an `ItemDef` whose `Effect` declares both immediate buff conditions and post-expiry side-effect conditions.
- DRUG-G2: Side-effect conditions fire automatically when the corresponding buff condition's duration elapses, without manual cleanup.
- DRUG-G3: Dealer NPCs sell drugs through the existing merchant pipeline; pricing honors the existing wanted-level surcharge and faction-based tier gating.
- DRUG-G4: Per-character drug-use is recorded with timestamps so the future addiction system has historical data on day one.
- DRUG-G5: Five drugs and three dealer NPCs are authored as exemplars.
- DRUG-G6: Existing consumables continue to work unchanged.

### 2.2 Non-Goals

- DRUG-NG1: Addiction mechanics (tolerance buildup, withdrawal severity, addiction recovery). Out of scope; the ledger from DRUG-G4 is the seam.
- DRUG-NG2: Drug crafting / synthesis recipes.
- DRUG-NG3: Drug-related quests or storyline arcs (dealer takedown quests, etc.).
- DRUG-NG4: Police / NPC reactions to visibly intoxicated players.
- DRUG-NG5: Dealer haggling / bartering. Existing merchant negotiation suffices.
- DRUG-NG6: Pharmacy / legal-medication parallel system. Drugs in this spec are explicitly the illegal / black-market subset.

## 3. Glossary

- **Drug**: a consumable `ItemDef` whose `Effect.Drug` block is non-nil.
- **Buff condition**: a condition applied at use-time and held for the declared duration (e.g., `+2 status to Brutality`).
- **Side-effect condition**: a condition applied automatically when a buff condition expires (e.g., `sickened 1` for 30 minutes).
- **Dealer**: an NPC whose `MerchantConfig.MerchantType` is `drugs` or `black_market`.
- **Use ledger**: the `character_drug_use` table recording every consumption event.

## 4. Requirements

### 4.1 Drug Content Schema

- DRUG-1: A new optional struct `DrugBlock` MUST be added to `ConsumableEffect` with fields:
  - `category` (string): one of `stimulant`, `depressant`, `psychoactive`, `nootropic`, `combat_enhancer`. Drives flavor text, dealer tags, and future addiction grouping.
  - `severity` (string): one of `mild`, `moderate`, `severe`. Drives default side-effect intensity if author omits explicit blocks.
  - `aftermath` (list of `ConditionEffect`): conditions to apply when *any* buff condition from this consumable's `Conditions` list expires. May be empty (`severity = mild` default produces a single `sickened 1` for 5 minutes when omitted).
  - `addiction_potency` (int, default `1`): how heavily this drug entry weighs into the per-character ledger. Lower is less addictive; higher accumulates faster. The actual addiction system reads this in a future spec.
- DRUG-2: The loader MUST validate that every `aftermath[*].condition_id` resolves and every duration parses. When `severity` is set and `aftermath` is empty, a default aftermath MUST be auto-populated per the table in DRUG-3.
- DRUG-3: Severity defaults table for omitted aftermath:
  - `mild` → `sickened 1` for 5 minutes.
  - `moderate` → `sickened 2` for 15 minutes plus `fatigued` for 1 hour.
  - `severe` → `sickened 2` for 30 minutes plus `fatigued` for 2 hours plus `drained 1` for 24 hours.
- DRUG-4: A new `ItemDef.Tags` field MUST gain a recognized tag `drug`. The loader MUST auto-tag any item whose `Effect.Drug` block is non-nil and reject any item that has the `drug` tag without the `Drug` block (consistency check).

### 4.2 Buff Expiry → Aftermath Wiring

- DRUG-5: The condition-expiry tick (existing per-round / per-tick pipeline that decrements `ActiveCondition.DurationRemaining`) MUST consult a new per-character per-condition metadata field `Source string`. When a condition is applied via `ApplyConsumable` from a `Drug` effect, the `Source` MUST be set to a sentinel `drug:<item_id>:<condition_id>`.
- DRUG-6: When a condition with a `drug:<item_id>:<condition_id>` source expires, the runtime MUST look up the item's `Effect.Drug.aftermath` list and apply those conditions to the same character. Aftermath conditions MUST themselves carry a `Source = "drug_aftermath:<item_id>"` so they are not double-cascaded.
- DRUG-7: When multiple buff conditions from the same drug overlap and expire on different ticks, the aftermath fires on the *last* expiry only (single aftermath per consumption event). A use-tracking key `drug_use_id` MUST be added to `ActiveCondition` to identify conditions belonging to the same consumption event.
- DRUG-8: Existing condition application paths (skill actions, equipment, abilities, spells) MUST be unaffected — the aftermath dispatch only fires when `Source` matches the `drug:` sentinel.

### 4.3 Use Ledger

- DRUG-9: A new database table `character_drug_use` MUST be migrated with columns: `id` (PK), `character_id` (FK), `item_id` (text), `category` (text), `addiction_potency` (int), `used_at` (timestamp), `source` (text — `dealer`, `looted`, `crafted`, `gift`).
- DRUG-10: `inventory.ApplyConsumable` MUST insert a row into `character_drug_use` whenever the consumed item has a `Drug` block. The `source` column is computed by the caller (defaults to `unknown`); merchant purchase paths set `dealer`, loot paths set `looted`.
- DRUG-11: A new repository `internal/storage/postgres/drug_use.go` MUST expose `RecentUse(characterID, since time.Time) []DrugUseEvent` and `TotalPotency(characterID, since time.Time) int` for the future addiction subsystem.

### 4.4 Dealer NPC Archetype

- DRUG-12: A new NPC template field `archetype string` MUST recognize `street_dealer` and `black_market_dealer`. When set, the loader MUST validate that `MerchantConfig.MerchantType` matches (`drugs` for street, `black_market` for fixer-tier).
- DRUG-13: Three exemplar dealer NPCs MUST be authored:
  - One `street_dealer` placed in a Desperate Streets / Armed & Dangerous zone.
  - One `street_dealer` placed in a Warlord Territory zone with rarer combat-enhancer drugs.
  - One `black_market_dealer` placed in an Apex Predator faction enclave with severe-tier drugs and a faction-tier gate (per existing `min_faction_tier_id`).
- DRUG-14: Dealers MUST use the existing `MerchantConfig.Inventory` shape with item ids referencing the new drug content. Stock and budget MUST replenish per the existing `ReplenishRate` mechanism.
- DRUG-15: Dealer NPCs MUST NOT initially carry combat profiles (no `Abilities`). They are non-combatant merchants by default. A future ticket can layer combat / aggression for "the dealer turns hostile if you fail a deal".

### 4.5 Exemplar Drug Content

- DRUG-16: At least five drug items MUST be authored under `content/items/drugs/`:
  - `combat_juice` — `combat_enhancer`, `moderate`. Buff: `+2 status to Brutality and Quickness` for 5 minutes. Custom aftermath: `sickened 2` for 30 minutes.
  - `nightshade_tab` — `psychoactive`, `mild`. Buff: `+2 status to Reasoning` for 10 minutes. Default mild aftermath.
  - `crash_capsule` — `depressant`, `severe`. Buff: removes `frightened` and `fleeing`, applies `+5 status to Will saves` for 1 minute. Default severe aftermath.
  - `wide_awake` — `stimulant`, `moderate`. Buff: `quickened 1` for 1 round. Default moderate aftermath.
  - `cog_oil` — `nootropic`, `mild`. Buff: `+1 status to skill checks` for 30 minutes. Default mild aftermath.
- DRUG-17: Exemplar drugs MUST include `Tags: [drug]` (via DRUG-4 auto-tag) and reference the existing condition catalog from spec #252's NCA-4.

### 4.6 Pricing & Commerce

- DRUG-18: Drug prices MUST honor the existing `ComputeBuyPrice` formula — base price × sell margin × wanted surcharge × (1 − negotiate). No new pricing math is introduced.
- DRUG-19: Black-market dealers MUST set a higher `SellMargin` (≥ 1.5×) than street dealers, reflecting risk premium. Authoring guidance documented in `docs/architecture/`.
- DRUG-20: Drug purchases MUST deduct credits as for any merchant transaction; receipt narrative SHOULD flag the drug name and dealer name plainly so the player has a record.

### 4.7 UI Surfacing

- DRUG-21: The web inventory panel MUST show a "Drug" badge on items that carry the `drug` tag.
- DRUG-22: The web character status panel MUST show active drug-source conditions distinctly from other conditions (proposal: a "💊" icon prefix). This is a cosmetic discrimination against the existing condition badges, not new state.
- DRUG-23: Telnet `inventory` and `who` outputs MUST mark drug-source items / conditions with a `[drug]` suffix to mirror DRUG-21/22.

### 4.8 Tests

- DRUG-24: Existing consumable tests MUST pass unchanged.
- DRUG-25: New unit tests in `internal/game/inventory/consumable_test.go` MUST cover:
  - DrugBlock parsing and severity-default fill-in.
  - Aftermath fires once per consumption event (DRUG-7).
  - Use-ledger row inserted with correct fields on consume.
- DRUG-26: A new integration test MUST exercise: buy a drug from a street dealer → consume → observe buff condition → wait for expiry → observe aftermath condition → query `character_drug_use` and assert one row.

## 5. Architecture

### 5.1 Where the new code lives

```
content/items/drugs/
  *.yaml                                     # 5+ exemplars

content/npcs/dealers/
  *.yaml                                     # 3+ exemplars

internal/game/inventory/
  consumable.go                              # ConsumableEffect.Drug *DrugBlock; ledger insert hook

internal/game/condition/
  active.go                                  # Source string + drug_use_id; aftermath dispatch on expiry

internal/game/npc/
  template.go                                # Archetype string; loader validation

internal/storage/postgres/
  drug_use.go                                # NEW repository

internal/gameserver/
  grpc_service.go                            # handleUse path tags drug consumption with source

migrations/
  NNN_character_drug_use.up.sql / .down.sql

cmd/webclient/ui/src/game/inventory/
  ItemBadge.tsx, ConditionBadge.tsx          # `drug` tag badge / 💊 prefix

internal/frontend/telnet/
  inventory_render.go, status_render.go      # `[drug]` suffix
```

### 5.2 Use → buff → aftermath flow

```
player: use combat_juice
   │
   ▼
handleUse → ApplyConsumable
   │
   ├── insert character_drug_use row (item_id, category, potency, source=dealer)
   │
   ├── apply Effect.Conditions (buff conditions) with
   │       Source = "drug:combat_juice:strength_boost"
   │       drug_use_id = <generated UUID>
   │
   ▼
round/tick loop decrements DurationRemaining
   │
   ▼
condition expires: source matches "drug:..." sentinel
   │
   ├── consult Effect.Drug.aftermath
   ├── if last buff condition for this drug_use_id → apply aftermath conditions
   │       Source = "drug_aftermath:combat_juice"
   │
   ▼
aftermath conditions tick down normally; do not cascade
```

### 5.3 Single sources of truth

- Drug declaration: `ItemDef.Effect.Drug` only.
- Aftermath dispatch: condition-expiry tick consulting `Source` sentinel.
- Use ledger: `character_drug_use` table only.
- Pricing: existing `ComputeBuyPrice`, no override.

## 6. Open Questions

- DRUG-Q1: When multiple buff conditions from the same drug have different durations, "fire on the last expiry" (DRUG-7) is one option; "fire when *all* have expired" is equivalent but the wording matters when one buff is removed early (e.g., a `remove_conditions` from a follow-up consumable). Recommendation: fire when the last *originally-scheduled* buff would have expired, even if one was removed early. Detect by tracking the latest expiry timestamp at consumption time, not by counting active conditions.
- DRUG-Q2: Should the future addiction system (NG1) be hinted in the schema now beyond `addiction_potency`? Recommendation: no — the ledger is enough to migrate later.
- DRUG-Q3: Black-market dealer faction-tier gates — should they share a tier with the existing `min_faction_tier_id` mechanism or be separate (e.g., a "criminal contacts" gate)? Recommendation: reuse `min_faction_tier_id`. New faction gates can be a follow-on.
- DRUG-Q4: Do drugs work on NPCs (e.g., a player force-feeds an NPC `crash_capsule`)? PF2E says yes for poisons but not typically for performance drugs. Recommendation: v1 is player-only. The existing skill-action system (#252) can layer a "force-feed" action later.
- DRUG-Q5: Should the dealer's `archetype` field grow into a richer NPC subtype (ranged, courier, ambush) or stay narrowly scoped to merchant flavor? Recommendation: narrow scope here. Combat / aggression archetypes belong to the npc behavior system.

## 7. Acceptance

- [ ] Five exemplar drugs load, validate, and consume end-to-end.
- [ ] Three exemplar dealers load and sell their stock per existing `MerchantConfig` mechanics.
- [ ] Buff conditions apply on use; aftermath conditions apply on the last buff expiry; both visible on character status surface.
- [ ] `character_drug_use` records one row per consumption with correct category/potency/source.
- [ ] Black-market dealer behind a faction-tier gate refuses sales below tier; refuses based on the existing gating mechanism.
- [ ] Drug items and drug-source conditions surface a distinct visual on web and a `[drug]` suffix on telnet.
- [ ] Migration applies and rolls back cleanly.

## 8. Out-of-Scope Follow-Ons

- DRUG-F1: Addiction mechanic (tolerance, withdrawal severity, recovery rolls).
- DRUG-F2: Drug crafting / synthesis.
- DRUG-F3: Quests centered on dealers / takedowns / supply chains.
- DRUG-F4: NPC reactions to visibly intoxicated players (police, faction enforcers).
- DRUG-F5: Drugs usable as poison-equivalent against NPCs (force-feed, contact poison, dart laced).
- DRUG-F6: Pharmacy / legal-medication parallel system.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/258
- Consumable model: `internal/game/inventory/consumable.go:10-26,120` (`ConsumableEffect`, `ApplyConsumable`)
- Item def: `internal/game/inventory/item.go:35` (`Effect *ConsumableEffect`)
- Use handler: `internal/gameserver/grpc_service.go:8333-8351` (`handleUse`)
- Merchant config: `internal/game/npc/noncombat.go:11-20` (`MerchantConfig`, `MerchantType`)
- Merchant pricing: `internal/game/npc/merchant.go:78-79` (`ComputeBuyPrice`)
- Faction tier gating: existing `min_faction_tier_id` on rooms / merchants
- Condition catalog: `docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md` (NCA-4)
- PF2E drug examples: `vendor/pf2e-data/packs/pf2e/equipment/diluted-hype.json`, `dreamtime-tea.json`
