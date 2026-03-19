# PF2E Spell Import Tool Design

**Date:** 2026-03-18
**Sub-project:** PF2E Import — Technology Content Generation

---

## Goal

Extend the existing `import-content` CLI tool with a `-format pf2e` mode that reads PF2E spell compendium JSON files, converts them to Gunchete `TechnologyDef` YAML files, and optionally localizes names and descriptions into Gunchete lore via the Claude API.

---

## Context

The game currently has 17 technology YAML files across 5 traditions. Job pools reference specific tech IDs in their `technology_grants`. To populate job/archetype pools with a full spell library, all PF2E spells must first be imported and converted. Localization ensures imported content fits the Gunchete cyberpunk aesthetic rather than reading as raw PF2E fantasy text.

---

## Architecture

The pipeline for each spell JSON file:

```
Parse → Convert → [Localize] → Write
```

1. **Parse** — unmarshal PF2E spell JSON → `PF2ESpell`
2. **Convert** — apply mechanical mapping rules → `[]TechData` (one per matching tradition)
3. **Localize** _(optional, `-localize` flag)_ — call Claude API to translate names/descriptions into Gunchete lore
4. **Write** — output one YAML file per `TechData` to `<output>/<tradition>/<id>.yaml`

A new `internal/importer/pf2e/` package mirrors the existing `internal/importer/gomud/` structure exactly. The existing `importer.go` gains a parallel `RunTech` method. The `source.go` gains a `TechSource` interface and `TechData` struct alongside the existing `Source`/`ZoneData`. All `gomud` code is unchanged.

---

## Design

### REQ-PF1
A new `internal/importer/pf2e/` package MUST be created containing `model.go`, `parser.go`, `converter.go`, and `source.go`.

### REQ-PF2
`model.go` MUST define Go structs that unmarshal standard PF2E compendium spell JSON: `PF2ESpell`, `SpellSystem`, `SpellTraits`, `SpellDamage`, `SpellSave`, and `SpellTime`.

### REQ-PF3
`parser.go` MUST expose `ParseSpell(data []byte) (*PF2ESpell, error)`.

### REQ-PF4
`converter.go` MUST expose `ConvertSpell(spell *PF2ESpell) ([]*importer.TechData, []string, error)` returning one `TechData` per matching Gunchete tradition, a slice of warning strings, and an error. Spells with no matching tradition MUST be skipped with a warning (not an error).

### REQ-PF5
The following tradition mapping MUST be applied:
- `occult` → `neural`
- `primal` → `bio_synthetic`
- `arcane` → `technical`
- `divine` → `fanatic_doctrine`

### REQ-PF6
When a spell maps to more than one tradition, the technology ID MUST be `<base_id>_<tradition>` (e.g., `fireball_technical`, `fireball_neural`). When a spell maps to exactly one tradition, the ID MUST be `<base_id>` with no suffix.

### REQ-PF7
The following mechanical conversion rules MUST be applied:

**Action cost** — parse `system.time.value`:
- `"1"` → 1, `"2"` → 2, `"3"` → 3
- `"reaction"` or `"free"` → 0
- Unrecognized → 2 (default) with warning

**Range** — parse `system.range.value`:
- `"touch"` or `"melee"` → `melee`
- `"self"` or `""` → `self`
- Contains `"emanation"`, `"burst"`, or `"cone"` → `zone`
- Numeric feet value → `ranged`
- Unrecognized → `ranged` with warning

**Targets** — derived from range and `system.target.value`:
- Range is `zone` → `zone`
- Target contains `"all enemies"` → `all_enemies`
- Target contains `"all allies"` → `all_allies`
- All other cases → `single`

**Duration** — parse `system.duration.value`:
- `"instant"`, `"instantaneous"`, or `""` → `instant`
- `"1 round"` or `"sustained"` → `rounds:1`
- `"N rounds"` → `rounds:N`
- `"1 minute"` → `minutes:1`
- Unrecognized → `instant` with warning

**Resolution**:
- `system.save.value` non-empty → `save`; save type mapped: Fortitude→`toughness`, Reflex→`hustle`, Will→`cool`
- `system.traits.value` contains `"attack"` → `attack`
- Otherwise → `none`

**Save DC**: `TechnologyDef.SaveDC` MUST be set to `15` for all save-based spells. PF2E spell JSON does not carry a DC value; 15 is the canonical default per `pf2e-import-reference.md`.

**Effects — placement by resolution** (tiers are mutually exclusive; only one set is populated per spell):
- Save-based: `on_crit_success` (omitted — empty means no effect), `on_success` (half-step damage), `on_failure` (full damage/conditions), `on_crit_failure` (double damage/conditions)
- Attack-based: `on_miss` (omitted), `on_hit` (full damage/conditions), `on_crit_hit` (double damage/conditions)
- No-roll: all effects in `on_apply`

**Half-step damage convention** (for `on_success` of basic saves): use the next smaller die size (e.g., `1d6` → `1d3`, `2d6` → `2d3`, `1d8` → `1d4`). If no smaller die step exists (e.g., `1d4`), use `1` flat.

**Damage effects**: each entry in `system.damage` becomes a `damage` effect with `dice` from the value field and `damage_type` from the type field.

**Condition effects**: conditions identified from `system.traits.value` or spell description keywords MUST be emitted as `condition` effects with `condition_id` set to the snake_case condition name, `value: 1`, and `duration` set to the spell's duration. Recognized condition keywords: `slowed`, `immobilized`, `blinded`, `fleeing`, `frightened`, `stunned`, `flat-footed`.

**Fallback**: when no structured damage or recognized condition is present, emit one `utility` effect with `description` set to the spell's description text (first 200 characters, trimmed).

**Amped effects**: `AmpedEffects` and `AmpedLevel` are NOT populated by this importer. Heightened/amped spell variants are out of scope (see Non-Goals).

### REQ-PF8
`source.go` MUST implement `TechSource` (defined in REQ-PF9) with `Load(sourceDir string) ([]*TechData, []string, error)`. It MUST walk the source directory, parse each `.json` file via `ParseSpell`, convert via `ConvertSpell`, and collect results and all warnings. Non-JSON files and files that fail to parse MUST be skipped with a warning.

### REQ-PF9
`internal/importer/source.go` MUST define:
```go
type TechData struct {
    Def       *technology.TechnologyDef
    Tradition string
}

type TechSource interface {
    Load(sourceDir string) ([]*TechData, []string, error)
}
```

### REQ-PF10
`internal/importer/importer.go` MUST gain a `RunTech(sourceDir, outputDir string, localizer Localizer) error` method that:
- Loads `[]*TechData` via `TechSource.Load`; propagates load errors
- Calls `localizer.Localize(ctx, def)` for each `TechData`; on error, skips that def with a warning
- Validates each `TechnologyDef` via `def.Validate()` before writing; invalid defs are skipped with a warning
- Writes each valid `TechData` as `<outputDir>/<tradition>/<def.ID>.yaml`
- Creates subdirectories as needed (mode 0755)

### REQ-PF11
A `Localizer` interface MUST be defined in `internal/importer/localizer.go`:
```go
type Localizer interface {
    Localize(ctx context.Context, def *technology.TechnologyDef) error
}
```
Two implementations MUST exist:
- `NoopLocalizer` — returns nil without modifying `def` (used when `-localize` flag is absent and in all tests)
- `ClaudeLocalizer` — calls the Claude API (`claude-sonnet-4-6`) to rewrite `def.Name` and `def.Description` in Gunchete lore style; all other fields unchanged

### REQ-PF12
`ClaudeLocalizer` MUST construct its prompt as follows:
- **System prompt**: instructs the model to act as a Gunchete lore writer; includes the full contents of `docs/requirements/pf2e-import-reference.md` and a sample of up to 10 existing technology names+descriptions loaded from `content/technologies/`
- **User prompt**: provides the current `Name` and `Description` and requests a JSON response of exactly this form: `{"name": "...", "description": "..."}` with the localized values; instructs the model to preserve mechanical text (dice expressions, range, duration) and rewrite only flavor/lore text
- **Response parsing**: the response body MUST be parsed as JSON to extract `name` and `description`; if parsing fails, the original values are kept and a warning is emitted (no error returned)
- **API failure**: if the Claude API call itself returns an error, `Localize` MUST return that error

### REQ-PF13
`cmd/import-content/main.go` MUST be extended with:
- `-format pf2e` — selects the PF2E import path
- `-output` — already present; when `-format pf2e` and `-output` is unset, default to `content/technologies/`
- `-localize` — boolean flag; enables Claude API localization stage
- `-anthropic-key` — string flag; also read from `ANTHROPIC_API_KEY` env var; required when `-localize` is set

### REQ-PF14
When `-localize` is set but no API key is available (flag unset and env var absent), the tool MUST exit with a clear error message before processing any files.

---

## Files

| File | Change |
|------|--------|
| `internal/importer/source.go` | Add `TechData`, `TechSource` interface |
| `internal/importer/localizer.go` | New: `Localizer` interface, `NoopLocalizer`, `ClaudeLocalizer` |
| `internal/importer/importer.go` | Add `RunTech` method |
| `internal/importer/pf2e/model.go` | New: PF2E spell JSON structs |
| `internal/importer/pf2e/parser.go` | New: `ParseSpell` |
| `internal/importer/pf2e/converter.go` | New: `ConvertSpell` |
| `internal/importer/pf2e/source.go` | New: `TechSource` implementation |
| `internal/importer/pf2e/testdata/` | New: fixture JSON spell files |
| `internal/importer/pf2e/parser_test.go` | New: parse tests |
| `internal/importer/pf2e/converter_test.go` | New: conversion rule tests (property-based) |
| `internal/importer/pf2e/source_test.go` | New: source walk tests with testdata fixtures |
| `internal/importer/importer_test.go` | Add `RunTech` integration test |
| `cmd/import-content/main.go` | Add `-format pf2e`, `-localize`, `-anthropic-key`, default output |
| `docs/requirements/pf2e-import-reference.md` | Correct `divine` → `fanatic_doctrine` mapping |

No schema changes. No game server changes.

---

## Testing

- **REQ-PF3 (unit)**: `ParseSpell` round-trips valid JSON; returns error on malformed input
- **REQ-PF4/PF5/PF6 (property)**: for all tradition combinations, `ConvertSpell` returns the correct number of `TechData` with correct IDs and tradition values; warnings slice populated for unknown traditions
- **REQ-PF7 (unit + property)**: each mapping rule tested individually — action cost parsing, range mapping, target derivation, duration parsing, save type mapping, half-step die reduction, condition extraction, fallback utility effect; property test over generated `PF2ESpell` values validates no panics
- **REQ-PF8 (unit)**: `source.Load` with testdata fixtures; verifies count, tradition routing, warning emission for non-JSON and unparseable files
- **REQ-PF10 (integration)**: `RunTech` with testdata fixtures, `NoopLocalizer`, and a temp output dir; verifies correct files written to correct subdirectories; invalid defs skipped with warning
- **REQ-PF11 (unit)**: `NoopLocalizer.Localize` returns nil and does not modify def
- **REQ-PF13/PF14 (unit)**: CLI flag parsing; missing API key with `-localize` exits with error before processing

---

## Non-Goals

- Importing non-spell PF2E content (feats, items, ancestry abilities)
- Populating job/archetype YAML pools (Sub-project 2)
- Heightened/amped spell variants (`AmpedEffects`, `AmpedLevel`)
- Real-time Foundry MCP integration (source is file-based JSON)
