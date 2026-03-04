# Character Sheet Enhancement — Design Document

**Date:** 2026-03-04
**Status:** Approved

---

## Overview

Add skills, feats, and class features sections to the existing `sheet`/`char` command output.
All three sections display in full — skills show all entries regardless of rank; feats and class
features show the full granted list with `[active]` tags on activatable entries.

---

## Proto Changes (`CharacterSheetView`)

Add three new repeated fields to the existing `CharacterSheetView` message, reusing already-defined
entry types:

```proto
repeated SkillEntry skills = 15;
repeated FeatEntry feats = 16;
repeated ClassFeatureEntry class_features = 17;
```

No new proto messages are needed.

---

## Server Handler (`handleChar`)

After building the existing sheet view, populate the three new fields by:

1. Fetching skill proficiencies from `characterSkillsRepo.GetAll`
2. Resolving via `skillRegistry` into `SkillEntry` messages (same logic as `handleSkills`)
3. Fetching feat IDs from `characterFeatsRepo.GetAll`
4. Resolving via `featRegistry` into `FeatEntry` messages (same logic as `handleFeats`)
5. Fetching class feature IDs from `characterClassFeaturesRepo.GetAll`
6. Resolving via `classFeatureRegistry` into `ClassFeatureEntry` messages (same logic as `handleClassFeatures`)

No new DB tables or repositories required.

---

## Renderer (`RenderCharacterSheet`)

Three new sections appended after the existing currency/equipment sections:

### Skills Section
- Two-column layout (name left, rank right)
- All skills displayed; rank label color-coded:
  - Untrained → dim/gray
  - Trained → white
  - Expert → cyan
  - Master → yellow
  - Legendary → magenta

### Feats Section
- One feat per line: `Name [active]` (where applicable) — `Short description`
- `[active]` tag in yellow

### Class Features Section
- Grouped: archetype features first, then job-specific feature
- Same `[active]` tag convention as the `cf` command

---

## Scope

- No new command
- No new proto messages
- No new DB tables or repositories
- Pure extension of existing `CharacterSheetView` + `handleChar` + `RenderCharacterSheet`
