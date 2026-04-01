# Technology Short Names

**Slug:** technology-short-names
**Status:** spec
**Priority:** 483
**Category:** ui
**Effort:** S

## Overview

Add an optional `short_name` field to technology definitions so players can type `use <short_name>` instead of `use <full_tech_id>`. The web UI Technology tab MUST store `use <short_name>` in hotbar slots when a short name is defined, making hotbar activation more ergonomic.

## Dependencies

- `technology` — technology data model and `use` command
- `web-client` — Technologies drawer hotbar slot assignment

## Spec

### REQ-TSN-1: Short Name Field on TechnologyDef

The `TechnologyDef` struct in `internal/game/technology/model.go` MUST include an optional `ShortName string` field with YAML tag `short_name,omitempty`. Technologies without a short name remain fully functional under their existing ID.

### REQ-TSN-2: Short Name Format

When present, `short_name` MUST satisfy all of the following constraints (enforced by `TechnologyDef.Validate()`):

- REQ-TSN-2a: Contains only lowercase ASCII letters, digits, and underscores.
- REQ-TSN-2b: Does not begin or end with an underscore.
- REQ-TSN-2c: Is not identical to the technology's own `id`.
- REQ-TSN-2d: Length is between 2 and 32 characters inclusive.

### REQ-TSN-3: Short Name Uniqueness

The `Registry` MUST enforce that no two loaded `TechnologyDef` entries share the same `short_name`. A duplicate short name MUST cause `Registry.Load()` to return an error, aborting startup.

### REQ-TSN-4: Registry Secondary Index

`Registry` MUST maintain a `byShortName map[string]*TechnologyDef` index populated during `Load()`. The existing `Get(id string)` method MUST remain unchanged. A new `GetByShortName(short string) (*TechnologyDef, bool)` method MUST be added for short-name lookups.

### REQ-TSN-5: Short Name Collision with Existing IDs

A technology's `short_name` MUST NOT equal any other technology's `id`. `Registry.Load()` MUST check for this collision and return an error if found.

### REQ-TSN-6: `use` Command Resolution

The `handleUse()` function in `internal/gameserver/grpc_service.go` MUST resolve the technology identifier using the following ordered lookup for all usage types (prepared, spontaneous, innate):

1. Attempt exact match against technology ID (existing behavior, preserved).
2. If no match, attempt lookup via `Registry.GetByShortName()`.

Both lookups MUST be case-sensitive. The resolved `TechnologyDef` MUST then proceed through the existing activation pathway unchanged.

### REQ-TSN-7: Hotbar Assignment in Technology Drawer

In `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx`, when assigning a technology to a hotbar slot, the stored command text MUST be:

- `use <short_name>` if the technology's `TechDef` (or proto view) exposes a non-empty short name.
- `use <tech_id>` otherwise (existing behavior).

### REQ-TSN-8: Short Name in Proto Technology View

The proto message that carries technology data to the web client (the technology slot view) MUST include a `short_name string` field so that `TechnologyDrawer.tsx` can read it without a separate lookup. The field MUST be empty string when no short name is defined.

### REQ-TSN-9: Existing Hotbar Slots Are Not Migrated

Hotbar slots already persisted in the database with `use <tech_id>` MUST continue to function. No data migration is required; the `use` command resolves by ID first (REQ-TSN-6), so existing slots remain valid.

### REQ-TSN-10: Content Assignment is Out of Scope

This feature defines the mechanism only. Assigning `short_name` values to existing technology YAML files is NOT required as part of this feature; short names are added to individual technology files opportunistically.

### REQ-TSN-11: Property Tests

Property-based tests MUST be added in `internal/game/technology/` to verify:

- REQ-TSN-11a: `Registry.Load()` succeeds with a valid short name and indexes it correctly.
- REQ-TSN-11b: `Registry.Load()` returns an error on duplicate short names.
- REQ-TSN-11c: `Registry.Load()` returns an error when a short name collides with another technology's ID.
- REQ-TSN-11d: `TechnologyDef.Validate()` rejects short names that violate REQ-TSN-2 constraints.

## Implementation Notes

- **Files to modify:**
  - `internal/game/technology/model.go` — add `ShortName` field and validation
  - `internal/game/technology/registry.go` — add `byShortName` index and `GetByShortName` method; enforce uniqueness and collision checks in `Load()`
  - `internal/gameserver/grpc_service.go` — extend `handleUse()` fallback chain (lines ~7195, ~7225, ~7260) to try short name after ID miss
  - `api/proto/game/v1/game.proto` — add `short_name` to the technology slot proto message
  - `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx` — use `short_name` when building hotbar command text (lines 77, 121, 169)
- The registry uses `yaml.KnownFields(true)` strict unmarshalling — the Go struct field MUST be added before any YAML file uses `short_name`
- Feat/class feature `use` resolution already uses `strings.EqualFold`; technology resolution currently uses exact match only — REQ-TSN-6 keeps the case-sensitive contract, consistent with current technology behavior
