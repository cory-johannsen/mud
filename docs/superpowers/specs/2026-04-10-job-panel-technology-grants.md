# Spec: Job Panel Technology Grants and Selections Display

**GitHub Issue:** cory-johannsen/mud#35
**Date:** 2026-04-10

## Context

The `JobGrantsResponse` proto already carries both `feat_grants` and `tech_grants`, and `JobDrawer.tsx` already renders both. However, the player reports that the Job panel does not display technology grants. Investigation revealed issue #30 (only level-1 feat grant shown; all higher-level grants missing) likely affects technology grants equally — the same server-side or client-side bug that suppresses feat grants past level 1 would suppress tech grants too.

This spec defines the complete, correct display contract for technology grants and selections in the Job panel, independent of the #30 fix, so both can be verified together.

---

## REQ-1: Technology grants grouped by level alongside feats

- REQ-1a: The Job panel MUST display technology grants at every level the player has reached, grouped by grant level alongside feat grants for that level
- REQ-1b: Each tech grant entry MUST show: tech name, tech type (`Hardwired`, `Prepared`, `Spontaneous`), and tech level (e.g. `Level 1`)
- REQ-1c: Tech type labels MUST use consistent color-coding matching the Technology drawer: hardwired (#88ccff), prepared (#ffcc88), spontaneous (#cc88ff)
- REQ-1d: Grants MUST be sorted by `grant_level` ascending (level 1 grants first)

## REQ-2: Technology pool selections displayed

- REQ-2a: When a player selected a technology from a pool at a given level, the Job panel MUST show their actual selection (not the pool label)
- REQ-2b: When a player has NOT yet made a selection from a pool, the panel MUST show a placeholder label indicating a selection is pending (e.g. `[Pending selection]`)
- REQ-2c: Prepared slot grants that are pool choices MUST follow REQ-2a/2b

## REQ-3: Prepared slot counts displayed

- REQ-3a: When a level-up grant adds prepared slots (e.g. `+1 prepared slot at level 1`), the Job panel MUST display this as a slot count grant entry distinct from a specific tech grant
- REQ-3b: The display label MUST be human-readable: e.g. `+1 Prepared Slot (Level 1 tech)`

## REQ-4: Spontaneous use pool grants displayed

- REQ-4a: When a level-up grant increases a spontaneous use pool (e.g. `+1 use of level-1 techs`), the Job panel MUST display this as a use grant entry
- REQ-4b: The display label MUST be human-readable: e.g. `+1 Use (Level 1 tech)`

## REQ-5: Server completeness

- REQ-5a: `handleJobGrants()` MUST return tech grants for ALL levels up to and including the player's current level — creation grants at level 1 AND all `LevelUpGrants` from job and archetype definitions
- REQ-5b: This requirement is shared with issue #30; fixing #30's server-side gap MUST also restore tech grant completeness

## REQ-6: No change to proto

- REQ-6a: The existing `JobTechGrant` proto fields (`grant_level`, `tech_id`, `tech_name`, `tech_level`, `tech_type`) are sufficient; no new proto fields are required
- REQ-6b: Slot count and use pool grants MAY be represented as synthetic `JobTechGrant` entries with descriptive `tech_name` values and `tech_type` set to `"prepared_slot"` or `"spontaneous_use"` respectively

## Files to Modify

- `internal/gameserver/grpc_service.go` or `grpc_service_job_grants.go` — ensure `handleJobGrants()` emits all tech grants at all levels (shared fix with #30)
- `cmd/webclient/ui/src/game/drawers/JobDrawer.tsx` — verify REQ-1 through REQ-4 display requirements are met; fix any rendering gaps
