# Zone Map Architecture Documentation

**Slug:** zone-map-architecture-docs
**Status:** planned
**Priority:** 456
**Category:** meta
**Effort:** S

## Overview

Zone maps (room layouts, connections, and spatial structure) must be documented in the architecture documents and kept up to date whenever the map layout changes. This ensures the architecture docs remain the authoritative reference for zone topology and that engineers can reason about spatial game state without reading raw YAML.

## Requirements

- REQ-ZMA-1: Each zone MUST have a corresponding map diagram in `docs/architecture/` showing rooms, connections, danger levels, and safe-room markers.
- REQ-ZMA-2: When a zone's room layout changes (rooms added, removed, or connections modified), the corresponding architecture map MUST be updated in the same commit.
- REQ-ZMA-3: Map diagrams MUST use a consistent notation: rooms as labeled nodes, exits as directed edges, danger level as node annotation, safe rooms marked with `[S]`.
- REQ-ZMA-4: The architecture index (`docs/architecture/README.md` or equivalent) MUST include a link to each zone map diagram.

## Implementation Notes

- Diagrams may be Mermaid graphs embedded in Markdown (preferred for diffability) or ASCII art.
- Existing zones (Gunchete, etc.) require retroactive diagrams as part of the initial implementation.
- The AGENTS.md rule SYSREG-3 ("Agents MUST maintain architecture diagrams for all features and core systems") already mandates this; this feature formalizes it specifically for zone maps with an explicit update obligation on map changes.
