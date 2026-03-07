# Saving Rolls Design

## Goal

Implement PF2E-style saving rolls (Toughness/Hustle/Cool) using the existing proficiency infrastructure.

## Save Names (Gunchete Setting)

| PF2E Name | Gunchete Name | Ability Score | Combatant Field |
|-----------|---------------|---------------|-----------------|
| Fortitude | Toughness     | Grit          | GritMod         |
| Reflex    | Hustle        | Quickness     | QuicknessMod    |
| Will      | Cool          | Savvy         | SavvyMod        |

## Data Model

Reuse `character_proficiencies` table — "toughness", "hustle", "cool" are new categories alongside existing weapon/armor ones. No new migration.

Each job YAML gains entries in its `proficiencies` map:
```yaml
proficiencies:
  toughness: trained
  hustle: trained
  cool: trained
```

Existing backfill-at-login logic picks these up automatically. `PlayerSession.Proficiencies` already carries the full category→rank map.

## Mechanical Formula

```
save_total = 1d20 + ability_mod + CombatProficiencyBonus(level, rank)
```

4-tier outcome via existing `OutcomeFor(total, dc)`:
- crit_success (total ≥ dc + 10)
- success (total ≥ dc)
- failure (total < dc)
- crit_failure (total ≤ dc - 10)

## Components

### Combatant struct (`internal/game/combat/combat.go`)

New fields:
```go
GritMod      int
QuicknessMod int
SavvyMod     int
ToughnessRank string
HustleRank    string
CoolRank      string
```

### Save resolver (`internal/game/combat/resolver.go`)

New function:
```go
func ResolveSave(saveType string, combatant *Combatant, dc int, src rand.Source) Outcome
```

Updates grenade Reflex save to call `ResolveSave("hustle", target, grenade.SaveDC, src)`.

### Job YAMLs (`content/jobs/*.yaml`)

Each job gets toughness/hustle/cool proficiency ranks in its `proficiencies` section. Combat-focused jobs (aggressor, etc.) get `expert` Toughness; agile jobs get `expert` Hustle; mental/social jobs get `expert` Cool.

### Combat startup (`internal/gameserver/combat_handler.go`)

Wire GritMod, QuicknessMod, SavvyMod and ToughnessRank, HustleRank, CoolRank from session into the Combatant at combat start.

### Character sheet

New `--- Saves ---` section showing computed totals:
```
Toughness: +5  Hustle: +3  Cool: +2
```

Totals computed as: `ability_mod + CombatProficiencyBonus(level, rank)` (no d20 — these are the static bonus shown on the sheet).

## Testing

- Property-based: `ResolveSave` with trained rank at high level always beats DC 5
- Unit: save outcome tiers correct for boundary values
- Unit: grenade resolver uses hustle save
- Property-based: ability mods and proficiency bonus flow correctly into save total
