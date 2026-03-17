# Job Technology Grants — Phase 1 Design Spec

**Date:** 2026-03-17

---

## Goal

Wire archetype-owned slot progression + job-owned tech pools into the existing `AssignTechnologies` pipeline. Phase 1 uses the 4 existing tech definitions as placeholder content. No new tech definitions are created.

---

## Context

`AssignTechnologies` in `internal/gameserver/technology_assignment.go` currently reads grants exclusively from `job.TechnologyGrants`. No job YAML currently has `technology_grants` set. The `Archetype` struct has `InnateTechnologies` but no `TechnologyGrants` field.

**Out of scope (Phase 2):** Expanded tech library; higher-level slots and pools; spontaneous use-count tracking.

---

## Archetype → PF2E Class Mapping

| Archetype | PF2E Class | Tradition | Grant Style |
|---|---|---|---|
| nerd | Wizard | Technical | Prepared, many slots |
| zealot | Cleric | Fanatic Doctrine | Prepared |
| naturalist | Druid | Bio-Synthetic | Prepared |
| schemer | Witch | Neural | Prepared |
| influencer | Bard | Neural | Spontaneous |
| drifter | Ranger | Bio-Synthetic | Prepared, fewer slots |
| aggressor | Fighter | — | None |
| criminal | Rogue | — | None |

---

## Feature 1: Archetype struct extension

Add to `internal/game/ruleset/archetype.go`:

```go
type Archetype struct {
    ID                 string              `yaml:"id"`
    Name               string              `yaml:"name"`
    Description        string              `yaml:"description"`
    KeyAbility         string              `yaml:"key_ability"`
    HitPointsPerLevel  int                 `yaml:"hit_points_per_level"`
    AbilityBoosts      *AbilityBoostGrant  `yaml:"ability_boosts"`
    InnateTechnologies []InnateGrant       `yaml:"innate_technologies,omitempty"`
    TechnologyGrants   *TechnologyGrants   `yaml:"technology_grants,omitempty"`
    LevelUpGrants      map[int]*TechnologyGrants `yaml:"level_up_grants,omitempty"`
}
```

`LoadArchetypes` does NOT call `Validate()` on archetype grants — archetypes define only slot counts (no pool), so standalone validation would always fail. Validation of the merged result (archetype + job) occurs at runtime in `AssignTechnologies`.

---

## Feature 2: MergeGrants function

Add to `internal/game/ruleset/technology_grants.go`:

```go
// MergeGrants combines archetype-level grants (slot progression) with job-level grants
// (fixed techs, pool options, optional extra slots).
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned grant is the union of both; nil if both are nil.
func MergeGrants(archetype, job *TechnologyGrants) *TechnologyGrants {
    if archetype == nil && job == nil {
        return nil
    }
    if archetype == nil {
        return job
    }
    if job == nil {
        return archetype
    }
    merged := &TechnologyGrants{}

    // Hardwired: union
    merged.Hardwired = append(append([]string(nil), archetype.Hardwired...), job.Hardwired...)

    // Prepared: merge slot counts (sum), fixed (union), pool (union)
    if archetype.Prepared != nil || job.Prepared != nil {
        merged.Prepared = mergePreparedGrants(archetype.Prepared, job.Prepared)
    }

    // Spontaneous: merge known/uses (sum), fixed (union), pool (union)
    if archetype.Spontaneous != nil || job.Spontaneous != nil {
        merged.Spontaneous = mergeSpontaneousGrants(archetype.Spontaneous, job.Spontaneous)
    }

    return merged
}

func mergePreparedGrants(a, b *PreparedGrants) *PreparedGrants {
    out := &PreparedGrants{SlotsByLevel: make(map[int]int)}
    if a != nil {
        for lvl, n := range a.SlotsByLevel {
            out.SlotsByLevel[lvl] += n
        }
        out.Fixed = append(out.Fixed, a.Fixed...)
        out.Pool = append(out.Pool, a.Pool...)
    }
    if b != nil {
        for lvl, n := range b.SlotsByLevel {
            out.SlotsByLevel[lvl] += n
        }
        out.Fixed = append(out.Fixed, b.Fixed...)
        out.Pool = append(out.Pool, b.Pool...)
    }
    return out
}

func mergeSpontaneousGrants(a, b *SpontaneousGrants) *SpontaneousGrants {
    out := &SpontaneousGrants{
        KnownByLevel: make(map[int]int),
        UsesByLevel:  make(map[int]int),
    }
    if a != nil {
        for lvl, n := range a.KnownByLevel { out.KnownByLevel[lvl] += n }
        for lvl, n := range a.UsesByLevel  { out.UsesByLevel[lvl] += n }
        out.Fixed = append(out.Fixed, a.Fixed...)
        out.Pool  = append(out.Pool,  a.Pool...)
    }
    if b != nil {
        for lvl, n := range b.KnownByLevel { out.KnownByLevel[lvl] += n }
        for lvl, n := range b.UsesByLevel  { out.UsesByLevel[lvl] += n }
        out.Fixed = append(out.Fixed, b.Fixed...)
        out.Pool  = append(out.Pool,  b.Pool...)
    }
    return out
}
```

`MergeGrants` is also used for `LevelUpGrants`: at each level key, merge `archetype.LevelUpGrants[lvl]` with `job.LevelUpGrants[lvl]`.

Add a `MergeLevelUpGrants` helper:

```go
// MergeLevelUpGrants merges two level-keyed grant maps key by key.
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned map contains all keys from both inputs;
// keys present in both are merged via MergeGrants.
func MergeLevelUpGrants(archetype, job map[int]*TechnologyGrants) map[int]*TechnologyGrants {
    if len(archetype) == 0 && len(job) == 0 {
        return nil
    }
    out := make(map[int]*TechnologyGrants)
    for lvl, g := range archetype {
        out[lvl] = g
    }
    for lvl, g := range job {
        if existing, ok := out[lvl]; ok {
            out[lvl] = MergeGrants(existing, g)
        } else {
            out[lvl] = g
        }
    }
    return out
}
```

---

## Feature 3: AssignTechnologies update

In `internal/gameserver/technology_assignment.go`, update `AssignTechnologies` to merge before processing. The existing early-return guard `if job == nil || job.TechnologyGrants == nil` becomes `if job == nil` — archetype grants can now provide slots even when `job.TechnologyGrants` is nil.

New preamble (replaces lines 77–80 of current implementation):

```go
if job == nil {
    return nil
}

var archetypeGrants *ruleset.TechnologyGrants
if archetype != nil {
    archetypeGrants = archetype.TechnologyGrants
}
grants := ruleset.MergeGrants(archetypeGrants, job.TechnologyGrants)

// Validate merged grants before processing.
if grants != nil {
    if err := grants.Validate(); err != nil {
        return fmt.Errorf("AssignTechnologies: invalid merged grants for job %s: %w", job.ID, err)
    }
}

// rest of function uses merged `grants` instead of `job.TechnologyGrants`
```

Updated postcondition: If both `archetype.TechnologyGrants` and `job.TechnologyGrants` are nil, `grants` is nil and all session tech fields remain nil (innate assignment still proceeds).

The innate technology block (archetype.InnateTechnologies) remains unchanged — it is not part of `TechnologyGrants`.

---

## Feature 4: Archetype YAML content

Slot progression follows PF2E spell slot tables. Phase 1 defines levels 1–5 only. Higher-level tech slots (tech level 2+) require Phase 2 tech definitions and are omitted here.

**`content/archetypes/nerd.yaml`** — add:
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
level_up_grants:
  2:
    prepared:
      slots_by_level:
        1: 1
  3:
    prepared:
      slots_by_level:
        1: 1
  4:
    prepared:
      slots_by_level:
        1: 1
  5:
    prepared:
      slots_by_level:
        1: 1
```

**`content/archetypes/zealot.yaml`** — add:
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
level_up_grants:
  2:
    prepared:
      slots_by_level:
        1: 1
  3:
    prepared:
      slots_by_level:
        1: 1
  4:
    prepared:
      slots_by_level:
        1: 1
  5:
    prepared:
      slots_by_level:
        1: 1
```

**`content/archetypes/naturalist.yaml`** — add:
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
level_up_grants:
  2:
    prepared:
      slots_by_level:
        1: 1
  3:
    prepared:
      slots_by_level:
        1: 1
  4:
    prepared:
      slots_by_level:
        1: 1
  5:
    prepared:
      slots_by_level:
        1: 1
```

**`content/archetypes/schemer.yaml`** — add:
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
level_up_grants:
  2:
    prepared:
      slots_by_level:
        1: 1
  3:
    prepared:
      slots_by_level:
        1: 1
  4:
    prepared:
      slots_by_level:
        1: 1
  5:
    prepared:
      slots_by_level:
        1: 1
```

**`content/archetypes/influencer.yaml`** — add (Bard: spontaneous, Neural tradition):
```yaml
technology_grants:
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 2
level_up_grants:
  2:
    spontaneous:
      uses_by_level:
        1: 1
  3:
    spontaneous:
      uses_by_level:
        1: 1
  4:
    spontaneous:
      uses_by_level:
        1: 1
  5:
    spontaneous:
      uses_by_level:
        1: 1
```

**`content/archetypes/drifter.yaml`** — add (Ranger: small prepared list, Bio-Synthetic):
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 1
level_up_grants:
  3:
    prepared:
      slots_by_level:
        1: 1
  5:
    prepared:
      slots_by_level:
        1: 1
```

`aggressor.yaml` and `criminal.yaml` get no `technology_grants`.

---

## Feature 5: Job YAML content

Each tech-using job gets a `technology_grants` entry using the single available tech for their tradition. Jobs that deviate from their archetype baseline (extra slots, different fixed tech) override only those fields; the merge handles the rest.

**Tradition → available tech at level 1:**
- Technical (nerd): `neural_shock`
- Fanatic Doctrine (zealot): `battle_fervor`
- Bio-Synthetic (naturalist, drifter): `acid_spray`
- Neural (influencer, schemer): `mind_spike`

### Nerd jobs (9 jobs) — Technical tradition, prepared

All nerd jobs:
```yaml
technology_grants:
  prepared:
    pool:
      - id: neural_shock
        level: 1
```

Exception — `engineer` gets 1 extra slot at level 1:
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 1
    pool:
      - id: neural_shock
        level: 1
```

### Zealot jobs (10 jobs) — Fanatic Doctrine tradition, prepared

All zealot jobs:
```yaml
technology_grants:
  prepared:
    pool:
      - id: battle_fervor
        level: 1
```

Exception — `medic` gets 1 extra slot:
```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 1
    pool:
      - id: battle_fervor
        level: 1
```

### Naturalist jobs (8 jobs) — Bio-Synthetic tradition, prepared

All naturalist jobs:
```yaml
technology_grants:
  prepared:
    pool:
      - id: acid_spray
        level: 1
```

### Schemer jobs (8 jobs) — Neural tradition, prepared

All schemer jobs:
```yaml
technology_grants:
  prepared:
    pool:
      - id: mind_spike
        level: 1
```

### Influencer jobs (10 jobs) — Neural tradition, spontaneous

All influencer jobs:
```yaml
technology_grants:
  spontaneous:
    pool:
      - id: mind_spike
        level: 1
```

### Drifter jobs (10 jobs) — Bio-Synthetic tradition, prepared

All drifter jobs:
```yaml
technology_grants:
  prepared:
    pool:
      - id: acid_spray
        level: 1
```

---

## Feature 6: Validation

`LoadArchetypes` does NOT call `Validate()` on archetype grants — archetypes define only slot counts (no pool), so standalone validation would always fail. Validation of the merged result (archetype + job) happens in `AssignTechnologies` before processing. If validation fails, `AssignTechnologies` returns a wrapped error.

Note: Archetypes define only slots (no pool). Jobs define only pool (no slots, except extra-slot exceptions). Merging archetype slots + job pool produces a valid grant when pool size ≥ total merged slots. Validation of the merged result happens at runtime in `AssignTechnologies` — not at job or archetype load time. Extra-slot jobs (e.g., `engineer`) must supply enough pool entries to satisfy their own extra slots; the archetype's base slots are covered by the job's base pool contribution.

---

## Testing

- **REQ-JTG1**: `MergeGrants(nil, nil)` returns nil
- **REQ-JTG2**: `MergeGrants(a, nil)` returns a unchanged; `MergeGrants(nil, b)` returns b unchanged
- **REQ-JTG3**: Merged slot counts equal sum of archetype + job slots per level
- **REQ-JTG4**: Merged fixed = union; merged pool = union
- **REQ-JTG5** (property): For any two valid `TechnologyGrants`, merged hardwired length = len(a.Hardwired) + len(b.Hardwired)
- **REQ-JTG6**: `AssignTechnologies` returns a wrapped error when the merged grants fail `Validate()` (e.g., merged slots exceed merged pool+fixed)
- **REQ-JTG7**: `AssignTechnologies` with merged grants calls `Validate()` on the merged result before processing
- **REQ-JTG8**: `MergeLevelUpGrants` merges maps key-by-key; keys present in only one map pass through unchanged

---

## Constraints

- No new technology YAML definitions (Phase 2)
- `aggressor` and `criminal` archetypes: no changes
- `MergeGrants` is pure (no I/O, no side effects)
- `AssignTechnologies` signature unchanged — merge is internal
- `LoadArchetypes` does NOT validate archetype `TechnologyGrants` (archetypes have slots but no pool; standalone validation would always fail)
