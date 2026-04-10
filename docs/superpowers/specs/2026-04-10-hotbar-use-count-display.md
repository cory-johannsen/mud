# Spec: Hotbar Use Count Display and Recharge Hover

**GitHub Issue:** cory-johannsen/mud#20
**Date:** 2026-04-10

## Context

Hotbar buttons currently show only a name tooltip using the native HTML `title` attribute. Use state for technologies and feats exists server-side and is sent in `CharacterSheetView`, but `HotbarSlot` proto contains no use-count fields — it only carries `kind`, `ref`, `display_name`, and `description`. The `TechnologyDrawer` already renders use pips (`UsePips` component) from `InnateSlotView`/`PreparedSlotView`/`SpontaneousUsePoolView`. This feature extends that data to the hotbar.

---

## REQ-1: Proto — extend HotbarSlot with use state

- REQ-1a: `HotbarSlot` MUST gain three new fields: `int32 uses_remaining`, `int32 max_uses`, `string recharge_condition`
- REQ-1b: `uses_remaining = 0` AND `max_uses = 0` MUST indicate unlimited (no badge shown)
- REQ-1c: `uses_remaining = 0` AND `max_uses > 0` MUST indicate fully expended
- REQ-1d: `recharge_condition` MUST be a human-readable string (e.g. `"Recharges on rest"`, `"1 per combat"`, `"Daily"`) sourced from the feat/technology definition

## REQ-2: Server — populate use state when building HotbarUpdateEvent

- REQ-2a: `buildHotbarSlot` (or equivalent) MUST resolve use state for each slot kind:
  - **feat** slots: look up `sess.ActiveFeatUses[ref]` for `uses_remaining`; `max_uses` from `FeatDef.PreparedUses`; `recharge_condition` from `FeatDef.RechargeCondition` (new field)
  - **technology (innate)** slots: use `sess.InnateTechs[ref].UsesRemaining` / `MaxUses`; `recharge_condition` from tech definition
  - **technology (prepared)** slots: count non-expended prepared slots of matching `tech_id` for `uses_remaining`; `max_uses` = total prepared slots for that tech; `recharge_condition` from tech definition
  - **technology (spontaneous)** slots: use `sess.SpontaneousUsePools[level].Remaining` / `Max`; `recharge_condition` from tech definition
  - **command** and **consumable** slots: `uses_remaining = 0`, `max_uses = 0` (unlimited)
- REQ-2b: `HotbarUpdateEvent` MUST be re-sent with updated use state after any activation that changes use counts (feat activation, technology activation, rest)
- REQ-2c: `FeatDef` MUST gain a `RechargeCondition string` field populated in feat YAML definitions for all limited-use feats

## REQ-3: Web UI — use count badge on hotbar buttons

- REQ-3a: When `max_uses > 0`, a use count badge MUST be rendered on the hotbar button showing `uses_remaining` / `max_uses`
- REQ-3b: The badge MUST be positioned in the bottom-right corner of the hotbar slot
- REQ-3c: When `uses_remaining == 0` AND `max_uses > 0`, the button MUST appear visually dimmed (reduced opacity) to indicate exhaustion
- REQ-3d: Badge MUST use the existing `UsePips` component or a style consistent with `TechnologyDrawer`
- REQ-3e: Unlimited slots (`max_uses == 0`) MUST show no badge

## REQ-4: Web UI — rich hover tooltip on hotbar buttons

- REQ-4a: The native `title` attribute tooltip MUST be replaced with a portal-based custom tooltip matching the `RoomTooltip` style (`#1a1a1a` background, `#444` border, monospace font)
- REQ-4b: The tooltip MUST show: action name, description, uses remaining / max (if limited), recharge condition (if `recharge_condition` is non-empty)
- REQ-4c: The tooltip MUST appear on hover after the same delay as `RoomTooltip`
- REQ-4d: The tooltip MUST be viewport-clamped (no overflow off screen edges)
- REQ-4e: Edit hint (`right-click to edit`) MUST remain in the tooltip

## Files to Modify

- `api/proto/game/v1/game.proto` — add fields to `HotbarSlot`; add `RechargeCondition` to `FeatDef`
- `api/proto/game/v1/game.pb.go` — regenerate or hand-edit to match proto changes
- `internal/gameserver/grpc_service.go` — populate new `HotbarSlot` fields in hotbar build logic; re-send `HotbarUpdateEvent` after activation
- `content/feats/*.yaml` — add `recharge_condition` to all limited-use feat definitions
- `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx` — add badge rendering, replace `title` with portal tooltip
- `cmd/webclient/ui/src/proto/index.ts` — add new `HotbarSlot` fields to TypeScript types
