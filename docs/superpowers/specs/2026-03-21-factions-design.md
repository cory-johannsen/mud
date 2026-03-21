# Factions Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `factions` (priority 390)
**Dependencies:** `wanted-clearing` (for `FixerConfig`, `NPCType == "fixer"`); `quests` (for `QuestRewards.FactionRepXP`); `non-human-npcs` (for `npc.Template.FactionID`)

---

## Overview

Extends the existing Team system (Gun/Machete) into a full faction system with per-faction reputation tiers, zone ownership, faction-exclusive items and quests, price discounts, faction-gated rooms, and reputation services via Fixer NPCs. Faction identity is chosen at character creation and is immutable. Zone control is static, defined in content YAML.

---

## 1. Data Model

### 1.1 Faction Definition YAML

Faction definitions live in `content/factions/*.yaml`. Each file defines one faction.

- REQ-FA-1: `FactionDef` MUST have exported fields with YAML tags: `ID string` (YAML: `id`), `Name string` (YAML: `name`), `ZoneID string` (YAML: `zone_id`), `HostileFactions []string` (YAML: `hostile_factions`; faction IDs), `Tiers []FactionTier` (YAML: `tiers`), `ExclusiveItems []FactionExclusiveItems` (YAML: `exclusive_items`), `GatedRooms []FactionGatedRoom` (YAML: `gated_rooms`).
- REQ-FA-2: `FactionTier` MUST have exported fields: `ID string` (YAML: `id`), `Label string` (YAML: `label`), `MinRep int` (YAML: `min_rep`), `PriceDiscount float64` (YAML: `price_discount`; fraction in [0,1]).
- REQ-FA-3: `FactionExclusiveItems` MUST have exported fields: `TierID string` (YAML: `tier_id`), `ItemIDs []string` (YAML: `item_ids`).
- REQ-FA-4: `FactionGatedRoom` MUST have exported fields: `RoomID string` (YAML: `room_id`), `MinTierID string` (YAML: `min_tier_id`).
- REQ-FA-5: `FactionDef.Validate()` MUST return an error if: `ID` or `Name` is empty; `ZoneID` is empty; `Tiers` does not contain exactly 4 entries; the first tier does not have `MinRep == 0`; tier `MinRep` values are not strictly increasing; any `PriceDiscount` is outside `[0, 1]`; any `TierID` in `ExclusiveItems` or `GatedRooms` does not reference a tier in `Tiers`.

### 1.2 Faction Config

- REQ-FA-6: A `FactionConfig` struct MUST be loaded from `content/faction_config.yaml` with fields `RepPerNPCLevel int` (YAML: `rep_per_npc_level`) and `RepPerFixerService int` (YAML: `rep_per_fixer_service`). Both values MUST be > 0; a fatal startup error MUST be raised otherwise.

### 1.3 Faction Registry

- REQ-FA-7: `FactionRegistry` MUST be a `map[string]*FactionDef` loaded from `content/factions/` at server startup and injected into `GameServiceServer`.
- REQ-FA-8: `FactionRegistry.Validate()` at startup MUST cross-check: all `ZoneID` values exist in the world zone set; all `HostileFactions` IDs exist in `FactionRegistry`; all `ExclusiveItems[].ItemIDs` exist in `ItemRegistry`; all `GatedRooms[].RoomID` values exist in the world room set; all `MinTierID` values on `GatedRooms` reference a valid tier ID within the faction. Any unknown reference MUST cause a fatal startup error.

### 1.4 Runtime Types

- REQ-FA-9: `PlayerSession` MUST gain `FactionID string` (the player's faction; set at character creation; immutable during a session) and `FactionRep map[string]int` (faction ID → rep score; absent keys treated as 0).
- REQ-FA-10: `npc.Template` MUST gain `FactionID string` (YAML: `faction_id`; empty means no faction affiliation).
- REQ-FA-11: `npc.Instance` MUST propagate `FactionID` from its template at spawn.
- REQ-FA-12: `world.Zone` MUST gain `FactionID string` (YAML: `faction_id`; empty means no faction ownership).
- REQ-FA-13: `world.Room` MUST gain `MinFactionTierID string` (YAML: `min_faction_tier_id`; empty means no gating).

---

## 2. Faction Registry Operations

The `faction.Service` package at `internal/game/faction/` MUST provide:

- REQ-FA-14: `TierFor(factionID string, rep int) *FactionTier` — returns the highest tier in the faction whose `MinRep <= rep`. MUST return the first tier (Outsider) when rep is 0 or no higher tier qualifies.
- REQ-FA-15: `IsHostile(factionA, factionB string) bool` — returns true iff `factionB` appears in `factionA.HostileFactions`.
- REQ-FA-16: `DiscountFor(factionID string, rep int) float64` — returns `TierFor(factionID, rep).PriceDiscount`.
- REQ-FA-17: `IsEnemyOf(sess *session.PlayerSession, npcFactionID string) bool` — returns true iff `npcFactionID` is non-empty and `IsHostile(npcFactionID, sess.FactionID)` is true.
- REQ-FA-18: `CanEnterRoom(sess *session.PlayerSession, room *world.Room, zone *world.Zone) bool` — returns true if `room.MinFactionTierID` is empty, OR `zone.FactionID == sess.FactionID` AND the player's tier order index is >= the required tier's order index within the faction.
- REQ-FA-19: `CanBuyItem(sess *session.PlayerSession, itemDefID string) bool` — returns false if the item appears in any faction's `ExclusiveItems` list for a tier the player has not yet reached within that faction; returns true otherwise. An item in the player's own faction's exclusive list is purchasable iff the player's current tier order index >= the required tier's order index.

---

## 3. Reputation

### 3.1 Earning Rep

- REQ-FA-20: Killing an NPC whose `FactionID` is hostile to the player's faction MUST award rep to the player's own faction: `amount = npc.Level * FactionConfig.RepPerNPCLevel`. The NPC death handler MUST call `faction.Service.AwardRep(ctx, sess, characterID, sess.FactionID, amount)` when `IsHostile(npc.FactionID, sess.FactionID)` is true.
- REQ-FA-21: `QuestRewards` MUST gain an optional `FactionRepXP int` field (YAML: `faction_rep_xp`; default 0). `quest.Service.Complete` MUST call `faction.Service.AwardRep` with the player's `FactionID` and the quest's `FactionRepXP` value when non-zero.

### 3.2 Rep Persistence

- REQ-FA-22: A `character_faction_rep` table MUST be created with columns `character_id BIGINT NOT NULL REFERENCES characters(id)`, `faction_id TEXT NOT NULL`, `rep INT NOT NULL DEFAULT 0`, and `PRIMARY KEY (character_id, faction_id)`.
- REQ-FA-23: `FactionRepository` MUST be an interface with methods: `SaveRep(ctx context.Context, characterID int64, factionID string, rep int) error` and `LoadRep(ctx context.Context, characterID int64) (map[string]int, error)`.
- REQ-FA-24: `LoadRep` MUST be called at login to populate `sess.FactionRep`. Absent rows are treated as 0 rep (Outsider tier).
- REQ-FA-25: `faction.Service.AwardRep` MUST update `sess.FactionRep[factionID]`, persist via `SaveRep`, and return any tier-up message if the new rep crosses a tier threshold (e.g. `"You are now a Gunhand of Team Gun!"`).

### 3.3 Character Creation Persistence

- REQ-FA-26: The `characters` table MUST gain a `faction_id TEXT NOT NULL DEFAULT ''` column. At character creation, the chosen faction ID MUST be persisted to this column and loaded into `sess.FactionID` at login.

---

## 4. Access Control

### 4.1 NPC Hostility

- REQ-FA-27: Combat NPCs with `IsEnemyOf(sess, npc.FactionID) == true` MUST be treated as hostile targets by the combat engine's target-selection logic (equivalent to `disposition: hostile`). No special message is required; standard combat engagement applies.
- REQ-FA-28: Non-combat NPCs (merchants, healers, quest givers, fixers) with `IsEnemyOf(sess, npc.FactionID) == true` MUST refuse all interaction, sending `"<Name> eyes you coldly. 'We don't serve your kind here.'"` and returning without executing any service logic.

### 4.2 Room Entry Gating

- REQ-FA-29: When a player attempts to move into a room where `CanEnterRoom` returns false, the movement MUST be blocked and the player MUST receive: `"Only <MinTierLabel> members of <FactionName> may enter here."` where `MinTierLabel` is the label of the required tier and `FactionName` is the owning faction's name.

### 4.3 Item Purchase Gating

- REQ-FA-30: When a player attempts to buy an item for which `CanBuyItem` returns false, the purchase MUST be blocked and the player MUST receive: `"You need to be a <TierLabel> of <FactionName> to buy that."` where `TierLabel` is the minimum required tier's label.
- REQ-FA-31: When a player browses a faction merchant's inventory (`list` command), items the player cannot yet buy MUST be shown with a `[<TierLabel>]` suffix so players can see what rep tier unlocks them.

---

## 5. Faction Economy

### 5.1 Price Discounts

- REQ-FA-32: When a merchant NPC has `FactionID` matching `sess.FactionID`, the buy handler MUST apply the faction discount: `finalPrice = int(math.Ceil(float64(baseCost * merchantMargin) * (1.0 - faction.DiscountFor(sess.FactionID, rep))))`.
- REQ-FA-33: Players whose `FactionID` does not match the merchant's `FactionID` and are not hostile pay full price (`merchantMargin` only, no discount applied).

### 5.2 `change_rep` Command

- REQ-FA-34: The `change_rep <faction>` command MUST be handled by the `talk <npc>` flow when the NPC is a Fixer (`NPCType == "fixer"`). The Fixer displays faction rep services alongside wanted-clearing services.
- REQ-FA-35: The cost of `change_rep` MUST use the FixerConfig formula: `cost = FixerConfig.BaseCosts[currentTierIndex] * FixerConfig.NPCVariance * ZoneMultiplier`, where `currentTierIndex` is the 1-based index of the player's current tier (1=Outsider, 2=second tier, 3=third tier, 4=max tier). A player already at max tier MUST receive `"You've reached the highest standing with your faction."` and the service MUST be unavailable.
- REQ-FA-36: On successful `change_rep` payment, `faction.Service.AwardRep` MUST be called with `FactionConfig.RepPerFixerService` rep. The awarded amount MUST NOT raise the player's rep above `(nextTier.MinRep - 1)` if a next tier exists, preventing tier-skipping via fixer services.

---

## 6. `faction` Command

- REQ-FA-37: The `faction` command (no subcommand) MUST display the player's faction name, current tier label, rep score, rep required for next tier (or `"Max tier reached"` if at Exalted/Iron/Edge), and active perks (discount percentage and any unlocked gated rooms).
- REQ-FA-38: `faction info <id>` MUST display the faction's name, zone, tier names with their `min_rep` thresholds and price discounts.
- REQ-FA-39: `faction standing` MUST display the player's rep and tier label for every faction ID present in `sess.FactionRep`, annotating hostile factions with `(hostile)`.

---

## 7. Requirements Summary

- REQ-FA-1: `FactionDef` MUST have all specified exported fields with YAML tags.
- REQ-FA-2: `FactionTier` MUST have exported fields: `ID`, `Label`, `MinRep`, `PriceDiscount`.
- REQ-FA-3: `FactionExclusiveItems` MUST have exported fields: `TierID`, `ItemIDs`.
- REQ-FA-4: `FactionGatedRoom` MUST have exported fields: `RoomID`, `MinTierID`.
- REQ-FA-5: `FactionDef.Validate()` MUST reject invalid tier counts, non-zero first tier MinRep, non-increasing MinRep, out-of-range PriceDiscount, and invalid TierID references.
- REQ-FA-6: `FactionConfig` loaded from `content/faction_config.yaml`; both fields MUST be > 0.
- REQ-FA-7: `FactionRegistry` loaded from `content/factions/` at startup, injected into `GameServiceServer`.
- REQ-FA-8: Startup cross-validation of all faction definition references; unknown references are fatal errors.
- REQ-FA-9: `PlayerSession` gains `FactionID string` and `FactionRep map[string]int`.
- REQ-FA-10: `npc.Template` gains `FactionID string` (YAML: `faction_id`).
- REQ-FA-11: `SpawnInstance` MUST propagate `FactionID` from template to instance.
- REQ-FA-12: `world.Zone` gains `FactionID string` (YAML: `faction_id`).
- REQ-FA-13: `world.Room` gains `MinFactionTierID string` (YAML: `min_faction_tier_id`).
- REQ-FA-14: `TierFor` returns highest qualifying tier; defaults to first tier.
- REQ-FA-15: `IsHostile` returns true iff factionB is in factionA's hostile list.
- REQ-FA-16: `DiscountFor` delegates to `TierFor`.
- REQ-FA-17: `IsEnemyOf` returns true iff NPC has a non-empty FactionID hostile to the player's faction.
- REQ-FA-18: `CanEnterRoom` returns false when room is gated and player's faction/tier is insufficient.
- REQ-FA-19: `CanBuyItem` returns false for exclusive items the player's tier has not yet unlocked.
- REQ-FA-20: Enemy NPC kill awards rep to player's faction; amount = `npc.Level * RepPerNPCLevel`.
- REQ-FA-21: `QuestRewards` gains `FactionRepXP int`; `quest.Service.Complete` calls `AwardRep` when non-zero.
- REQ-FA-22: `character_faction_rep` table with PK `(character_id, faction_id)`.
- REQ-FA-23: `FactionRepository` interface with `SaveRep` and `LoadRep`.
- REQ-FA-24: `LoadRep` called at login; absent rows = 0 rep.
- REQ-FA-25: `AwardRep` updates session, persists, returns tier-up message if threshold crossed.
- REQ-FA-26: `characters` table gains `faction_id TEXT NOT NULL DEFAULT ''`; persisted at creation, loaded at login.
- REQ-FA-27: Enemy faction combat NPCs treated as hostile by combat target-selection.
- REQ-FA-28: Enemy faction non-combat NPCs refuse all interaction with standard message.
- REQ-FA-29: Blocked room entry sends `"Only <TierLabel> members of <FactionName> may enter here."`.
- REQ-FA-30: Blocked item purchase sends `"You need to be a <TierLabel> of <FactionName> to buy that."`.
- REQ-FA-31: Merchant `list` shows gated items with `[<TierLabel>]` suffix.
- REQ-FA-32: Faction merchant buy price applies `(1.0 - discount)` multiplier for allied players.
- REQ-FA-33: Non-hostile non-allied players pay full price (no discount).
- REQ-FA-34: `change_rep <faction>` handled via Fixer NPC `talk` flow.
- REQ-FA-35: `change_rep` cost uses FixerConfig formula; max-tier players receive unavailability message.
- REQ-FA-36: `change_rep` rep award capped at `nextTier.MinRep - 1` to prevent tier-skipping.
- REQ-FA-37: `faction` shows player's faction, tier, rep, progress to next tier, and active perks.
- REQ-FA-38: `faction info <id>` shows faction name, zone, tier names with thresholds and discounts.
- REQ-FA-39: `faction standing` shows rep and tier for all factions in `sess.FactionRep`; hostile factions annotated.
