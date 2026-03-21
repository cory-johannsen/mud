# Factions

Extends the existing Team system (Gun/Machete) into a full faction system with per-faction reputation tiers, zone ownership, faction-exclusive items and quests, price discounts, faction-gated rooms, and reputation services via Fixer NPCs.

Design spec: `docs/superpowers/specs/2026-03-21-factions-design.md`

## Requirements

- [ ] REQ-FA-1: `FactionDef` MUST have all specified exported fields with YAML tags.
- [ ] REQ-FA-2: `FactionTier` MUST have exported fields: `ID`, `Label`, `MinRep`, `PriceDiscount`.
- [ ] REQ-FA-3: `FactionExclusiveItems` MUST have exported fields: `TierID`, `ItemIDs`.
- [ ] REQ-FA-4: `FactionGatedRoom` MUST have exported fields: `RoomID`, `MinTierID`.
- [ ] REQ-FA-5: `FactionDef.Validate()` MUST reject invalid tier counts, non-zero first tier MinRep, non-increasing MinRep, out-of-range PriceDiscount, and invalid TierID references.
- [ ] REQ-FA-6: `FactionConfig` loaded from `content/faction_config.yaml`; all fields including `RepChangeCosts map[int]int` (keyed 1–4 by tier index) MUST be > 0; fatal startup error otherwise.
- [ ] REQ-FA-7: `FactionRegistry` loaded from `content/factions/` at startup, injected into `GameServiceServer`.
- [ ] REQ-FA-8: Startup cross-validation: unknown references fatal; exclusive item IDs globally unique across all factions; gated rooms must be in zones with non-empty `FactionID`.
- [ ] REQ-FA-9: `PlayerSession` gains `FactionID string` and `FactionRep map[string]int`.
- [ ] REQ-FA-10: `npc.Template` gains `FactionID string` (YAML: `faction_id`).
- [ ] REQ-FA-11: `SpawnInstance` MUST propagate `FactionID` from template to instance.
- [ ] REQ-FA-12: `world.Zone` gains `FactionID string` (YAML: `faction_id`).
- [ ] REQ-FA-13: `world.Room` gains `MinFactionTierID string` (YAML: `min_faction_tier_id`).
- [ ] REQ-FA-14: `TierFor` returns highest qualifying tier; defaults to first tier.
- [ ] REQ-FA-15: `IsHostile` returns true iff factionB is in factionA's hostile list.
- [ ] REQ-FA-16: `DiscountFor` delegates to `TierFor`.
- [ ] REQ-FA-17: `IsEnemyOf` returns true iff NPC has a non-empty FactionID hostile to the player's faction.
- [ ] REQ-FA-18: `CanEnterRoom` returns false when room is gated and player's faction/tier is insufficient.
- [ ] REQ-FA-19: `CanBuyItem` returns false for exclusive items the player's tier has not yet unlocked.
- [ ] REQ-FA-20: Enemy NPC kill awards rep to player's faction; amount = `npc.Level * RepPerNPCLevel`.
- [ ] REQ-FA-21: `QuestRewards` gains `FactionRepXP int` (amends REQ-QU-3); `quest.Service.Complete` calls `AwardRep` when non-zero.
- [ ] REQ-FA-22: `character_faction_rep` table with PK `(character_id, faction_id)`.
- [ ] REQ-FA-23: `FactionRepository` interface with `SaveRep` and `LoadRep`.
- [ ] REQ-FA-24: `LoadRep` called at login; absent rows = 0 rep.
- [ ] REQ-FA-25: `AwardRep` updates session, persists, returns tier-up message if threshold crossed.
- [ ] REQ-FA-26: `characters` table gains `faction_id TEXT NOT NULL DEFAULT ''`; persisted at creation, loaded at login.
- [ ] REQ-FA-27: Enemy faction combat NPCs treated as hostile by combat target-selection.
- [ ] REQ-FA-28: Enemy faction non-combat NPCs refuse all interaction with standard message.
- [ ] REQ-FA-29: Blocked room entry sends `"Only <TierLabel> members of <FactionName> may enter here."`.
- [ ] REQ-FA-30: Blocked item purchase sends `"You need to be a <TierLabel> of <FactionName> to buy that."`.
- [ ] REQ-FA-31: Merchant `list` shows gated items with `[<TierLabel>]` suffix.
- [ ] REQ-FA-32: Faction merchant discount applied as `floor(float64(baseCost) * merchantMargin * (1 - discount))`; same rounding convention as `ComputeBuyPrice`.
- [ ] REQ-FA-33: Non-hostile non-allied players MUST pay full price (no discount).
- [ ] REQ-FA-34: `change_rep <faction>` handled via Fixer NPC `talk` flow.
- [ ] REQ-FA-35: `change_rep` cost uses `FactionConfig.RepChangeCosts[tierIndex] * FixerConfig.NPCVariance * ZoneMultiplier` (floor, not BaseCosts); unavailable at max tier.
- [ ] REQ-FA-36: `change_rep` rep award capped at `nextTier.MinRep - 1` to prevent tier-skipping; partial award (not refusal) when cap applies.
- [ ] REQ-FA-37: `faction` shows player's faction, tier label (from `FactionTier.Label`), rep, progress to next tier (or `"Max tier reached"`), and active perks.
- [ ] REQ-FA-38: `faction info <id>` shows faction name, zone, tier names with thresholds and discounts.
- [ ] REQ-FA-39: `faction standing` shows rep and tier for all factions in `sess.FactionRep`; hostile factions annotated.
