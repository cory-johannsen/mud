# Innate Technology Cantrip Parity — Design Spec

## Goal

Innate technologies must behave like PF2E cantrips: unlimited use, available only to characters whose archetype has a technology tradition. The current implementation grants them with limited uses to all archetypes regardless of tech tradition.

## Background

Eleven innate technologies are defined in `content/technologies/innate/`. Each region grants one innate tech to characters created in that region. The intent was always unlimited use for tech-capable characters only — this was not implemented correctly during the initial technology feature.

**Current problems:**
- 7 of 11 regions grant innate techs with `uses_per_day: 1` instead of `0` (unlimited)
- `AssignTechnologies` grants region innate techs to all archetypes, including aggressor and criminal, which have no technology tradition
- Existing characters in the DB have `max_uses = 1` and may have innate slots that should not exist

## Tech-Capability Definition

A character is technology-capable if `technology.DominantTradition(archetype.ID) != ""`. The current mapping in `flavor.go`:

| Archetype | Tradition | Tech-Capable |
|-----------|-----------|--------------|
| nerd | technical | yes |
| naturalist | bio_synthetic | yes |
| drifter | bio_synthetic | yes |
| schemer | neural | yes |
| influencer | neural | yes |
| zealot | fanatic_doctrine | yes |
| aggressor | — | **no** |
| criminal | — | **no** |

## Changes

### 1. Region YAML Files

Set `uses_per_day: 0` on all 7 regions that currently have `uses_per_day: 1`:

| File | Region | Tech |
|------|--------|------|
| `content/regions/southeast_portland.yaml` | Southeast Portland | nanite_infusion |
| `content/regions/pearl_district.yaml` | Pearl District | pressure_burst |
| `content/regions/gresham_outskirts.yaml` | Gresham Outskirts | acid_spit |
| `content/regions/north_portland.yaml` | North Portland | terror_broadcast |
| `content/regions/pacific_northwest.yaml` | Pacific Northwest | atmospheric_surge |
| `content/regions/south.yaml` | South | viscous_spray |
| `content/regions/southern_california.yaml` | Southern California | chrome_reflex |

The 4 already-correct regions (old_town, mountain, northeast, midwest) are unchanged.

### 2. `AssignTechnologies` Gate (`internal/gameserver/technology_assignment.go`)

The region innate grant loop gains a tech-capability guard:

```go
if region != nil && technology.DominantTradition(archetype.ID) != "" {
    for _, grant := range region.InnateTechnologies {
        sess.InnateTechs[grant.ID] = &session.InnateSlot{
            MaxUses:       grant.UsesPerDay,
            UsesRemaining: grant.UsesPerDay,
        }
        if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
            return fmt.Errorf("AssignTechnologies innate (region) %s: %w", grant.ID, err)
        }
    }
}
```

The archetype innate grant loop receives the same guard (currently unused but must be consistent).

### 3. DB Migration

A SQL migration runs at deploy time with two statements, in order:

**Step 1 — Remove innate tech rows for non-tech-capable characters.**

Aggressor and criminal archetype jobs must be identified by job ID. The aggressor jobs are: beat_down_artist, boot_gun, boot_machete, gangster, goon, grunt, mercenary, muscle, roid_rager, soldier, street_fighter, thug. The criminal jobs are: beggar, car_jacker, contract_killer, gambler, hanger_on, hooker, smuggler, thief, tomb_raider.

```sql
DELETE FROM character_innate_techs
WHERE character_id IN (
    SELECT id FROM characters
    WHERE class IN (
        'beat_down_artist','boot_gun','boot_machete','gangster','goon','grunt',
        'mercenary','muscle','roid_rager','soldier','street_fighter','thug',
        'beggar','car_jacker','contract_killer','gambler','hanger_on','hooker',
        'smuggler','thief','tomb_raider'
    )
);
```

**Step 2 — Set all remaining innate tech rows to unlimited.**

```sql
UPDATE character_innate_techs SET max_uses = 0, uses_remaining = 0;
```

The migration is embedded as a Go migration file in the existing migration system and runs automatically on server startup before any player sessions load.

## Testing

- Property test: for all 76 job IDs, `AssignTechnologies` grants innate techs if and only if `DominantTradition(archetype) != ""`
- Property test: all granted innate slots have `MaxUses == 0` and `UsesRemaining == 0`
- Unit test: tech-capable character from each of the 7 fixed regions gets correct unlimited innate slot
- Unit test: aggressor and criminal characters get zero innate slots
- Migration test: existing rows with `max_uses = 1` are updated to `0` after migration runs
