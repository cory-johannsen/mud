---
title: AoE Drawing in Combat
issue: https://github.com/cory-johannsen/mud/issues/250
date: 2026-04-24
status: spec
prefix: AOE
depends_on:
  - "#247 Cover bonuses in combat (positional model)"
related:
  - "#249 Targeting system in combat (validation pipeline; not yet specced)"
  - "#267 Visibility / line-of-sight system (will affect cell-inclusion when shipped)"
---

# AoE Drawing in Combat

## 1. Summary

Add area-of-effect (AoE) **template placement** to combat for actions whose effect spans more than one cell. Players choose where the template lands (and, for cones, where it points) before confirming the action; the resolver applies the action's effects against every combatant whose cell falls inside the template.

The current pipeline supports **bursts only** — a square radius around a target cell — and the radius value is the only AoE knob declared on content (`aoe_radius` on `TechnologyDef` / `ClassFeatureDef` / `FeatDef`, `aoe_radius` on `Explosive`). The grid-resolver path is wired (see `internal/gameserver/grpc_service.go:8573` REQ-AOE-2 and `internal/game/combat/aoe.go:CombatantsInRadius`), but the loader, wire format, geometry, and UI all assume burst. This spec extends every layer to also cover **cones** and **lines**, and ships the **placement / preview UX** that today's bridge handler explicitly defers (`internal/frontend/handlers/bridge_handlers.go:900`).

## 2. Goals & Non-Goals

### 2.1 Goals

- AOE-G1: Support three template shapes — burst, cone, line — uniformly across content, wire format, resolver, and both client surfaces.
- AOE-G2: Players place the template on the combat grid before the action commits, and may cancel without spending AP.
- AOE-G3: Affected cells and combatants are visibly highlighted during placement on both surfaces (telnet ASCII, web grid).
- AOE-G4: All existing burst content (10+ class features, explosives) keeps working unchanged after the migration.
- AOE-G5: Resolution reads template shape from a single source of truth: the action's content definition.

### 2.2 Non-Goals

- AOE-NG1: Line-of-sight / occlusion checks for AoE cells. Deferred to #267; this spec adds an explicit no-op extension point so #267 can plug in.
- AOE-NG2: Friendly-fire toggles, exclusion masks, or "save for half" mechanics. Effect application stays as-is — every combatant in the template receives the action's effect.
- AOE-NG3: Multi-stage templates (e.g. "burst around moving caster every round"). Templates are placed once at action submission and resolved at the next round tick.
- AOE-NG4: Authoring tools for new AoE content. Existing YAML shape (with new optional fields) is sufficient.
- AOE-NG5: Diagonal-cost geometry. Chebyshev distance is preserved (5 ft per cell, no √2 penalty), matching `combat.CombatRange`.

## 3. Glossary

- **Template**: a set of grid cells the action affects. Three shape kinds: burst, cone, line.
- **Anchor**: the cell that positions a template. For burst it is the center; for cone it is the apex (caster's cell by default); for line it is the origin.
- **Facing**: a direction (one of 8 compass octants) used by cone and line templates.
- **Origin**: the casting combatant's grid cell.
- **Affected cells**: the set of grid cells inside the template at resolution time.
- **Affected combatants**: living combatants whose `(GridX, GridY)` is in *affected cells*.

## 4. Requirements

### 4.1 Content Model

- AOE-1: `TechnologyDef`, `ClassFeatureDef`, `FeatDef`, and `Explosive` MUST gain an optional `aoe_shape` field with allowed values `burst` (default), `cone`, `line`.
- AOE-2: `TechnologyDef`, `ClassFeatureDef`, `FeatDef`, and `Explosive` MUST gain an optional `aoe_length` field, in feet, used as cone length and line length. The existing `aoe_radius` field continues to denote burst radius.
- AOE-3: `TechnologyDef`, `ClassFeatureDef`, `FeatDef`, and `Explosive` MUST gain an optional `aoe_width` field, in feet, used by the line shape only. Defaults to 5 (one cell wide). Ignored for burst and cone.
- AOE-4: When `aoe_shape` is omitted on existing content, the loader MUST treat it as `burst` so all current `aoe_radius`-only content continues to resolve identically.
- AOE-5: Validation MUST reject content where `aoe_shape == "burst"` and `aoe_radius <= 0`, where `aoe_shape == "cone"` and `aoe_length <= 0`, and where `aoe_shape == "line"` and (`aoe_length <= 0` or `aoe_width <= 0`).
- AOE-6: Validation MUST reject content where `aoe_length` or `aoe_width` is set and `aoe_shape == "burst"`, and where `aoe_radius` is set and `aoe_shape != "burst"`. Mixing dimensions across shapes signals authoring error.

### 4.2 Geometry

- AOE-7: A new package `internal/game/grid` (or `internal/game/combat/geometry.go` if a separate package introduces unwanted coupling) MUST provide pure-function helpers:
  - `BurstCells(center Cell, radiusFt int) []Cell` — Chebyshev square, equivalent to today's `CombatantsInRadius` cell-set. Bounded to the 10×10 grid (web) / 20×20 grid (telnet) by the caller; helper itself returns the unbounded set.
  - `ConeCells(apex Cell, facing Direction, lengthFt int) []Cell` — PF2E-style cone: 90° emanation aligned with the chosen octant; cells whose Chebyshev distance from apex is <= `lengthFt/5` AND fall within the cone's wedge.
  - `LineCells(origin Cell, facing Direction, lengthFt int, widthFt int) []Cell` — Bresenham line of `lengthFt/5` cells, thickened to `widthFt/5` cells perpendicular to facing.
- AOE-8: `Direction` MUST enumerate the 8 compass octants: N, NE, E, SE, S, SW, W, NW. A `FacingFrom(from Cell, to Cell) Direction` helper MUST round to the nearest octant, preferring axial in ties.
- AOE-9: All three helpers MUST be deterministic, side-effect free, and covered by property-based tests under `testdata/rapid/` per SWENG-5a.
- AOE-10: All three helpers MUST exclude the apex/origin cell from their output for cone and line (the casting combatant is not affected by their own cone/line); burst includes the center cell.
- AOE-11: Geometry helpers MUST NOT consult cover, walls, or visibility. Occlusion is a separate concern (NG1) and lives in the resolver.

### 4.3 Wire Format

- AOE-12: A new proto message `AoeTemplate` MUST be added with fields `shape` (enum BURST/CONE/LINE), `anchor_x`, `anchor_y` (int32), `facing` (enum N..NW), and `cells` (repeated `GridCell{x,y}`). The server MAY ignore `cells` on input (re-derived from shape+anchor+facing+content) but MUST populate it on outbound combat events for client rendering.
- AOE-13: `UseRequest` MUST gain an optional `template` field of type `AoeTemplate`. When the activated feat/tech declares `aoe_shape != burst`, the server MUST treat a missing `template` as a validation error and return a clear "AoE template required" message, except when the existing `target_x`/`target_y` fields are populated AND the content is `burst` (back-compat per AOE-4).
- AOE-14: Throw-explosive submission MUST use the same `template` field via a new `ThrowRequest` (or extend the existing throw request — see §6) so explosives, techs, and feats share the AoE wire path.
- AOE-15: `ActionRequest` MUST NOT gain AoE fields. Generic combat actions remain non-AoE; AoE actions go through `UseRequest` (techs/feats) and the explosive throw path.
- AOE-16: Existing `target_x`/`target_y` fields on `UseRequest` (`api/proto/game/v1/game.proto:1186-1187`) MUST be retained as-is for back-compat. The loader rule from AOE-4 means a burst feat sent with the old fields and no `template` still resolves correctly.

### 4.4 Resolver

- AOE-17: The grid-resolver path in `internal/gameserver/grpc_service.go` (currently `CombatantsInRadius` for burst at REQ-AOE-2) MUST be replaced with a single `affectedCells := geometry.CellsForTemplate(template, content)` call, then `affectedCombatants := combat.CombatantsInCells(cbt, affectedCells)`.
- AOE-18: `combat.CombatantsInCells(cbt, []Cell) []*Combatant` MUST be added next to `CombatantsInRadius` in `internal/game/combat/aoe.go` and MUST return only living combatants (mirroring `CombatantsInRadius`'s `IsDead()` filter).
- AOE-19: `CombatantsInRadius` MUST be retained as a thin wrapper that calls `CombatantsInCells(cbt, BurstCells(center, radiusFt))` so existing call sites and tests are unaffected.
- AOE-20: When the placed template intersects no living combatant, the action MUST still consume AP (and any tech/feat resource cost) and MUST emit a narrative line confirming the placement was executed but hit nothing — consistent with how missed single-target attacks are handled.
- AOE-21: A `PostFilterAffectedCells(cells []Cell, ctx ResolveContext) []Cell` extension point MUST be added to the resolver and called between `CellsForTemplate` and `CombatantsInCells`. v1 implementation is identity (no-op). #267 will plug visibility/occlusion into this seam without further resolver edits.

### 4.5 Telnet UI

- AOE-22: When a player invokes a feat/tech/throw whose content declares `aoe_shape != burst`, the telnet handler MUST enter an **AoE placement mode** for that player session, suppressing normal combat input until the template is confirmed or cancelled.
- AOE-23: Placement mode MUST render the combat grid (`internal/frontend/telnet/combat_grid.go`) with the affected cells highlighted using a distinct glyph layer (proposal: cells highlighted with `+` overlay; combatants in affected cells shown in inverse video). Placement render MUST NOT mutate the underlying `Combat` state.
- AOE-24: Placement input MUST accept:
  - `aim <x> <y>` — set anchor (burst, line) or facing target (cone).
  - `face <dir>` — set facing for cone/line, where `<dir>` is one of `n,ne,e,se,s,sw,w,nw`.
  - `confirm` — submit the action with the current template; transitions out of placement mode and dispatches the gRPC request.
  - `cancel` — abort placement, restore normal input, and refund nothing (no AP charged yet).
- AOE-25: Placement mode MUST display a one-line legend above the grid: `[AoE: <shape> (<n> cells, <m> targets)] aim/face/confirm/cancel`.
- AOE-26: For burst with the legacy `target_x`/`target_y` fields populated by an existing macro/alias, telnet MUST skip placement mode and dispatch directly (preserves existing aliases).
- AOE-27: Placement mode MUST time out after 60 seconds of no input and auto-`cancel`, to prevent a player from holding up combat round resolution indefinitely.

### 4.6 Web UI

- AOE-28: The web client (`cmd/webclient/ui/src/game/panels/MapPanel.tsx`) MUST gain a placement state distinct from the existing `hoveredCell` state: `aoePlacement: { shape, anchor, facing, cells } | null`. The state MUST be entered when the player clicks an AoE-bearing action button and exited on confirm or cancel.
- AOE-29: While `aoePlacement` is active, mouse motion over the grid MUST update `anchor` (for burst/line) or `facing` (for cone, computed via `FacingFrom(origin, hoveredCell)`); affected cells MUST be re-derived in the client and rendered with a translucent highlight (proposal: 30% opacity tinted by shape — burst red, cone amber, line blue).
- AOE-30: A confirm button (and Enter key) MUST submit the template via gRPC; an X button (and Esc) MUST cancel without submitting. The action button that triggered placement MUST remain visibly "armed" until confirm/cancel resolves.
- AOE-31: The client MUST mirror the server's geometry helpers for preview only. Resolution remains authoritative on the server: the wire format includes the cell list (AOE-12) so the client preview never drifts from server resolution.
- AOE-32: When the action's content is burst-only AND the player triple-clicks (or shift-clicks) a target combatant, the client MAY skip placement mode and emit a burst centered on that combatant — purely a convenience shortcut, equivalent to the legacy `target_x`/`target_y` path.

### 4.7 Content Migration

- AOE-33: All existing class features with `aoe_radius` (currently 9 entries in `content/class_features.yaml` at lines 77, 93, 380, 663, 725, 754, 850, 953, 969) MUST receive an explicit `aoe_shape: burst` for clarity. This is a documentation pass — semantics are unchanged by AOE-4.
- AOE-34: At least one class feature, one technology, and one explosive MUST be migrated to `aoe_shape: cone` or `aoe_shape: line` as exemplars and to exercise the new path end-to-end. Selection is a content-team call; the implementer MUST coordinate with the user before picking which content to migrate.
- AOE-35: The `grpc_service_aoe_test.go` skip (currently `t.Skip("AoE feat integration test: enable once a feat with aoe_radius > 0 is defined in YAML")`) MUST be removed and the test re-enabled, exercising one burst, one cone, and one line action.

## 5. Architecture

### 5.1 Layer diagram

```
content/*.yaml  ──▶  loader (ruleset/feat.go, technology/model.go,
                              ruleset/class_feature.go, inventory/explosive.go)
                       │  AoeShape, AoeRadius, AoeLength, AoeWidth
                       ▼
                  TechnologyDef / ClassFeatureDef / FeatDef / Explosive
                       │
                       ▼
            internal/game/combat/geometry.go (NEW)
                       │  BurstCells / ConeCells / LineCells / FacingFrom
                       ▼
            internal/game/combat/aoe.go
                       │  CombatantsInCells (NEW), CombatantsInRadius (now wrapper)
                       ▼
            internal/gameserver/grpc_service.go
                       │  CellsForTemplate → PostFilterAffectedCells (no-op)
                       │  → CombatantsInCells → effect application
                       ▲
            api/proto/game/v1/game.proto
                  AoeTemplate, UseRequest.template (NEW)
                       ▲
        ┌──────────────┴──────────────┐
        │                             │
  cmd/webclient (MapPanel.tsx)   internal/frontend/telnet
   aoePlacement state              AoE placement mode
   client-side preview             aim/face/confirm/cancel
```

### 5.2 Where the new code lives

- `internal/game/combat/geometry.go` — geometry helpers (AOE-7..AOE-11).
- `internal/game/combat/aoe.go` — `CombatantsInCells` added; `CombatantsInRadius` becomes a wrapper.
- `internal/game/ruleset/feat.go`, `internal/game/ruleset/class_feature.go`, `internal/game/technology/model.go`, `internal/game/inventory/explosive.go` — new YAML fields and validation.
- `api/proto/game/v1/game.proto` — `AoeTemplate` message and `UseRequest.template` field.
- `internal/gameserver/grpc_service.go` — resolver swap to template-based path; `PostFilterAffectedCells` extension point.
- `internal/frontend/telnet/combat_grid.go` and a new `internal/frontend/telnet/aoe_placement.go` — telnet placement mode, render overlay.
- `cmd/webclient/ui/src/game/panels/MapPanel.tsx` plus a new `cmd/webclient/ui/src/game/aoe/` directory — placement state, preview rendering, geometry mirror.

### 5.3 Cross-cutting concerns

- **Determinism (SWENG-5a):** Every geometry helper is a pure function over integer inputs. Property tests assert idempotence, symmetry where applicable, and bounds.
- **Functional core / imperative shell (SWENG-3):** All cell math is functional. The only mutating layer is the placement-mode state machines on the two clients, which are isolated to one struct each.
- **Single source of truth (SWENG-1):** Shape geometry lives in Go; the web client mirrors it for preview only. The server-emitted `cells` field on `AoeTemplate` is the authoritative set used to render results.

## 6. Open Questions

- AOE-Q1: Should explosives (currently `aoe_radius` on `Explosive`) ship cone/line variants in v1, or stay burst-only and only get the explicit `aoe_shape: burst` migration? Recommendation: burst-only for explosives in v1; revisit when a content reason emerges.
- AOE-Q2: For cone, is "90° wedge aligned to octant" the right PF2E approximation given the Chebyshev grid, or do we want the standard PF2E "burst whose cells are within range and within the cone's angle from apex"? Recommendation: 90° octant wedge — matches our 8-direction facing model and avoids fractional-angle math on an integer grid.
- AOE-Q3: Should the existing `ThrowRequest` (or whatever the explosive-throw RPC is called — needs verification) be modified, or do explosives keep going through the legacy `target_x`/`target_y` path until a content reason forces the migration? Recommendation: defer until AOE-Q1 is answered.
- AOE-Q4: Telnet placement mode blocks input — what happens if the player issues a `quit` or a chat command during placement? Recommendation: chat commands pass through; combat-affecting commands (move, use, attack) are rejected with "you are placing an area; confirm or cancel first".

## 7. Acceptance

- [ ] All existing combat tests pass without modification.
- [ ] New geometry property tests pass under `testdata/rapid/`.
- [ ] `grpc_service_aoe_test.go` is no longer skipped and exercises burst, cone, and line.
- [ ] At least one cone or line content example resolves end-to-end on both telnet and web in a manual test.
- [ ] Existing burst content (the 9 class features at the call sites in §4.7) resolves identically before and after migration — verified by targeted regression test.
- [ ] Telnet placement mode timeout (AOE-27) prevents indefinite combat stall — verified by integration test.
- [ ] Web placement mode preview matches server-resolved cells for a representative sample (burst@2 cells, cone@30ft N, line@25ftx5ft E) — verified by client unit test against fixed seeds.

## 8. Out-of-Scope Follow-Ons

- AOE-F1: Visibility/LoS filtering of affected cells — wired via `PostFilterAffectedCells` (AOE-21) when #267 ships.
- AOE-F2: Friendly-fire toggles and "save for half" damage scaling — separate ticket; effect application is unchanged here.
- AOE-F3: Authoring UI for new AoE content — current YAML workflow is sufficient.
- AOE-F4: Animated AoE previews on the web client (e.g. an expanding ripple on burst confirm) — pure cosmetic; out of scope for v1.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/250
- Existing AoE resolver path: `internal/gameserver/grpc_service.go:8573` (REQ-AOE-2 comment)
- Existing burst geometry: `internal/game/combat/aoe.go:7` (`CombatantsInRadius`)
- Existing AoE content: `content/class_features.yaml:77,93,380,663,725,754,850,953,969`
- Existing AoE proto fields: `api/proto/game/v1/game.proto:1186-1187`
- Deferred-UI marker: `internal/frontend/handlers/bridge_handlers.go:900`
- Distance model: `internal/game/combat/combat.go:275-293` (`CombatRange`, Chebyshev × 5 ft/square)
- Cover positional model (predecessor): `docs/superpowers/specs/2026-04-21-cover-bonuses-in-combat.md`
