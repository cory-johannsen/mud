# Design: General-Purpose Content Importer

**Date:** 2026-02-18
**Status:** Approved

## Summary

A general-purpose CLI tool (`cmd/import-content`) that reads MUD content assets from an external source format and converts them into the project's zone YAML format. The initial source format is `gomud` (from github.com/cory-johannsen/gomud).

## Motivation

The gomud project contains a complete zone (Rustbucket Ridge, 35 rooms across 6 areas) with rich descriptions and exit graphs. Importing this content bootstraps the game world without hand-authoring YAML. Making the tool format-agnostic allows future imports from other sources.

## Architecture

```
cmd/import-content/
  main.go                   ← CLI entry point

internal/importer/
  importer.go               ← Importer interface + top-level runner
  converter.go              ← nameToID() utility, shared conversion helpers
  source.go                 ← Source interface (pluggable per format)
  gomud/
    source.go               ← GomudSource implements Source
    parser.go               ← Parses zone/area/room YAML into gomud model types
    model.go                ← GomudZone, GomudArea, GomudRoom Go structs
    converter.go            ← Converts gomud model → project yamlZone/yamlRoom
```

The `Source` interface abstracts over input formats. New source formats are added as new packages under `internal/importer/` with no changes to the runner.

## CLI Interface

```
import-content -format gomud -source <dir> -output <dir> [-start-room <name>]
```

**Flags:**
- `-format`: source format identifier; currently only `gomud`
- `-source`: directory containing `zones/`, `areas/`, `rooms/` subdirectories (gomud layout)
- `-output`: directory to write zone YAML files; one file per zone
- `-start-room`: optional display name override for the zone's start room; default is first room listed in the zone file

**Example:**
```
go run ./cmd/import-content \
  -format gomud \
  -source /path/to/gomud/assets \
  -output content/zones
```

## Conversion Rules

**Name → ID:**
Lowercase all characters, replace spaces with `_`, strip all non-alphanumeric/underscore characters.
- `"Grinder's Row"` → `grinders_row`
- `"The Rusty Oasis"` → `the_rusty_oasis`
- `"Scrapshack 23"` → `scrapshack_23`

**Directions:** Lowercased verbatim (`North` → `north`, `Southwest` → `southwest`).

**Zone:** gomud `name` and `description` map directly; `id` = nameToID(name); `start_room` = ID of first room in zone's room list, or `-start-room` override.

**Rooms:** gomud `name` → `title`; `description` carried as-is; `objects` silently dropped.

**Exits:** each exit's `target` display name is resolved via the name→ID map. Exits whose target has no corresponding room file emit a warning to stderr and are dropped (not a fatal error — the source data has inconsistencies such as `"Grinder's Way"` referencing a non-existent room).

**Areas:** each room's area membership is stored as `properties.area` (string) on the room, preserving grouping without model changes.

**Output:** one `<zone_id>.yaml` per zone, written to the output directory, valid per `world.LoadZoneFromFile`.

## Error Handling

| Condition | Behavior |
|-----------|----------|
| Missing `zones/`, `areas/`, or `rooms/` subdir | Fatal error with clear message |
| Unparseable YAML file | Fatal error naming the offending file |
| Exit target with no matching room | Warning to stderr; exit dropped |
| Output zone fails `world.LoadZoneFromBytes` validation | Fatal error |

## Testing

Per SWENG-5 (TDD) and SWENG-5a (property-based testing):

- **`nameToID` (property-based):** output is always lowercase; never contains spaces or apostrophes; idempotent (`nameToID(nameToID(x)) == nameToID(x)`)
- **`GomudParser` (unit):** fixture YAML for each type — zone, area, room with exits, room with empty exits, room with objects
- **`GomudConverter` (unit):** exit resolution, direction lowercasing, area property assignment, unknown-target warning/drop behavior
- **Integration:** synthetic 3-room / 2-area / 1-zone dataset through full pipeline, validated with `world.LoadZoneFromBytes`
