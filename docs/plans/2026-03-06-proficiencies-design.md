# Proficiencies Design

**Date:** 2026-03-06
**Status:** Approved

## Problem

Job YAMLs already declare armor and weapon proficiencies (`simple_weapons: trained`, `light_armor: trained`, etc.) but this data is loaded and then discarded — no DB table stores it per character, no combat or AC calculation consults it, and no player-facing command exposes it. As a result every character attacks and defends as if fully proficient regardless of their job.

## Approach

Full pipeline in one pass:
- Add explicit `proficiency_category` field to all weapon and armor YAML files
- Create `character_proficiencies` DB table; backfill at login from job YAML
- Apply PF2E strict proficiency bonus in combat attack roll and AC calculation
- Add `proficiencies` command and update character sheet

No Go struct changes to `Job` are needed — `Proficiencies map[string]string` already parses from YAML.

---

## Section 1: Data Model

### Weapon YAML changes

Add `proficiency_category` to each of the 33 weapon files. Valid categories:
`simple_weapons`, `simple_ranged`, `martial_weapons`, `martial_ranged`, `martial_melee`, `unarmed`, `specialized`

```yaml
id: anti_materiel_rifle
proficiency_category: martial_ranged
group: firearm
...
```

### Armor YAML changes

Add `proficiency_category` to each armor file. Valid categories:
`unarmored`, `light_armor`, `medium_armor`, `heavy_armor`

```yaml
id: armored_gauntlets
proficiency_category: heavy_armor
group: plate
...
```

### DB table

Migration `018_character_proficiencies.up.sql`:

```sql
CREATE TABLE character_proficiencies (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    category     TEXT   NOT NULL,
    rank         TEXT   NOT NULL DEFAULT 'untrained',
    PRIMARY KEY (character_id, category)
);
```

### Backfill at login

Same pattern as skills/feats/class features: read `job.Proficiencies` map, upsert any missing rows using `ON CONFLICT DO NOTHING`. All characters implicitly receive `unarmored: trained` (PF2E baseline — every character can fight unarmored).

---

## Section 2: Mechanical Effects

### Proficiency bonus formula (PF2E strict)

```
func ProficiencyBonus(level int, rank string) int {
    switch rank {
    case "trained":  return level + 2
    case "expert":   return level + 4
    case "master":   return level + 6
    case "legendary": return level + 8
    default:         return 0  // untrained
    }
}
```

Replaces the current unconditional `ProficiencyBonus(level)` call in `internal/game/combat/resolver.go`.

### Weapon attack roll

`Combatant` struct gains `WeaponProficiencyRank string`.

```
atkMod = abilityMod + ProficiencyBonus(level, combatant.WeaponProficiencyRank)
```

The grpc session layer populates `WeaponProficiencyRank` by:
1. Looking up the equipped weapon's `proficiency_category`
2. Looking up the character's stored rank for that category

### Armor AC

`Combatant` struct gains `ArmorProficiencyRank string`.

```
AC = 10 + min(DexMod, armor.DexCap) + armor.ACBonus + ProficiencyBonus(level, combatant.ArmorProficiencyRank)
```

Unarmored characters use the `unarmored` category (always trained for all characters). The grpc session layer populates `ArmorProficiencyRank` using the equipped body armor's `proficiency_category`.

### Armor check penalty

`EquipmentStats.CheckPenalty` already flows through the skill check resolver — no new wiring needed.

---

## Section 3: Display

### `proficiencies` command

New player command (`HandlerProficiencies`) following CMD-1 through CMD-7.

Output format:
```
Armor Proficiencies
  Unarmored     [trained]   +3
  Light Armor   [trained]   +3
  Medium Armor  [untrained]  +0
  Heavy Armor   [untrained]  +0

Weapon Proficiencies
  Simple Weapons   [trained]   +3
  Simple Ranged    [trained]   +3
  Martial Weapons  [untrained]  +0
  Martial Ranged   [untrained]  +0
  Martial Melee    [untrained]  +0
  Unarmed          [trained]   +3
```

Bonus shown as `+N` where N = `ProficiencyBonus(characterLevel, rank)`.

### Character sheet

Add a Proficiencies section to the character sheet output. At width ≥ 73 the existing two-column layout places Proficiencies as a third block below Skills.

---

## Section 4: Testing

### Unit tests (TDD, property-based where applicable)

- `ProficiencyBonus(level, rank)`: trained > untrained at all levels; expert > trained; level scales correctly
- Weapon YAML validation: every weapon has a valid `proficiency_category`; unknown category fails validation
- Armor YAML validation: every armor has a valid `proficiency_category`
- `CharacterProficienciesRepository.GetAll` / `Upsert`: same pattern as `CharacterSkillsRepository`; idempotent on conflict
- Combat resolver: trained attacker adds `level+2`; untrained adds 0
- AC calculation: trained armor adds `level+2`; untrained armor adds 0; unarmored always trained

### Integration / backfill tests

- Character login with no proficiency rows → rows created from job YAML
- Character login with existing rows → no duplicates (idempotent)

### Ruleset / content tests

- All 76 job YAMLs reference only valid proficiency category strings
- All 33 weapon YAMLs have `proficiency_category` set to a valid value
- All armor YAMLs have `proficiency_category` set to a valid value
