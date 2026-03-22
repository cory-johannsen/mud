# Wanted Level Clearing

Active methods for players to reduce their Wanted level faster than passive time-based decay. See `docs/superpowers/specs/2026-03-20-wanted-clearing-design.md` for the full design spec.

## Requirements

- [x] Fixer NPC type
  - REQ-WC-1: `FixerConfig.NPCVariance` MUST be > 0; fatal load error otherwise.
  - REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4; fatal load error otherwise.
  - REQ-WC-2a: `FixerConfig.BaseCosts` MUST contain all keys 1–4 with positive values; fatal load error otherwise.
  - REQ-WC-2b: `GuardConfig.MaxBribeWantedLevel` MUST be in range 1–4 when `Bribeable` is true; fatal load error otherwise.
  - REQ-WC-3: Fixers MUST default to `flee` on combat start and MUST NOT enter the initiative order.
  - REQ-WC-4: The `change_rep` command MUST NOT be implemented here; reserved for `factions` feature.
  - [x] `Fixer *FixerConfig` field added to NPC Template struct
  - [x] `Template.Validate()` updated to recognize `"fixer"` type
  - [x] Named fixer NPC in Rustbucket Ridge (name/room TBD)
- [x] Bribe mechanic (`bribe [npc]` / `bribe confirm`)
  - REQ-WC-5: MUST fail if player WantedLevel is 0.
  - REQ-WC-6: MUST fail if insufficient credits.
  - REQ-WC-7: MUST fail if no bribeable NPC present.
  - REQ-WC-8: MUST fail if player WantedLevel exceeds NPC's bribe level cap.
  - REQ-WC-9: Two-step confirm flow MUST be used before deducting credits.
  - REQ-WC-9a: MUST disambiguate when multiple bribeable NPCs present.
  - [x] `GuardConfig.Bribeable` and `GuardConfig.MaxBribeWantedLevel` fields
  - [x] Zone multiplier table applied to bribe cost
- [x] Surrender mechanic (`surrender` / `release <player>`)
  - REQ-WC-10: Detained player MUST NOT move, use commands, or be targeted.
  - REQ-WC-11: Detained player MUST be visible to all room occupants.
  - REQ-WC-12: `surrender` MUST fail if no guard present.
  - REQ-WC-13: `surrender` MUST fail if WantedLevel is 0.
  - REQ-WC-14: Detention evaluated against in-game clock (1 real min = 1 in-game hour).
  - REQ-WC-14a: `DetainedUntil` MUST be persisted and restored on reconnect.
  - REQ-WC-14b: Detention expiring offline MUST complete on next connect.
  - REQ-WC-14c: 5-second grace window after detention before guards re-evaluate WantedLevel.
  - REQ-WC-15: Successful `release` MUST remove `detained` but MUST NOT modify WantedLevel.
  - REQ-WC-16: `release` MUST be available to any player in the room.
  - REQ-WC-16a: Release DC uses room's danger level at time of attempt.
  - [x] `detained` condition YAML (`content/conditions/detained.yaml`)
  - [x] `prevents_movement`, `prevents_commands`, `prevents_targeting` fields on condition Definition
  - [x] `DetainedUntil` field on PlayerSession, persisted to DB
- [x] Quest-based clearing
  - REQ-WC-17: Quest `wanted_reduction: N` MUST decrement WantedLevel by N, clamped to 0.
  - [x] `wanted_reduction` field on quest schema (wiring deferred to `quests` feature)
  - [x] `FixerConfig.ClearRecordQuestID` field (wiring deferred to `quests` feature)
