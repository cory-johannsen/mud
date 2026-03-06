# Archetype Expansion Design

**Date:** 2026-03-06
**Status:** Approved

## Problem

The current six archetypes (Aggressor, Influencer, Nerd, Drifter, Criminal, Normie) do not cover the full set of PF2E Player Core classes. Three classes have no analog: Cleric, Druid, and Witch. Normie has no PF2E equivalent and an unclear identity. Aggressor (17 jobs) and Influencer (14 jobs) are over-populated relative to others (9–12). This uneven distribution gives players a skewed set of choices at character creation.

## Approach

- Add three new archetypes: **Zealot** (Cleric), **Naturalist** (Druid), **Schemer** (Witch)
- Dissolve **Normie**: redistribute its 9 jobs lore-first across existing and new archetypes
- Redistribute jobs from Aggressor and Influencer to balance counts
- One DB migration clears stale archetype boost choices for characters whose job moves archetypes

No code changes are required. The system loads archetypes and jobs from YAML dynamically.

---

## Section 1: New Archetypes

### Zealot (Cleric)
Faith, community, righteous force, and healing in the post-collapse city.

```yaml
id: zealot
name: Zealot
description: "True believers and community enforcers. Zealots draw power from conviction — religious, political, or tribal — and use it to protect their flock and punish the wicked."
key_ability: grit
hit_points_per_level: 8
ability_boosts:
  fixed: [grit, flair]
  free: 2
```

Jobs: `cult_leader`, `hired_help`, `medic`, `pastor`, `street_preacher`, `believer`, `trainee`, `guard`, `follower`, `vigilante` (10 total)

### Naturalist (Druid)
Ecology, survival, and post-collapse nature; those who understand and work with living systems.

```yaml
id: naturalist
name: Naturalist
description: "When civilization collapses, someone has to understand what grows in the cracks. Naturalists are survivors, foragers, and those who read the land better than any map."
key_ability: reasoning
hit_points_per_level: 8
ability_boosts:
  fixed: [grit, reasoning]
  free: 2
```

Jobs: `exterminator`, `hippie`, `freegan`, `tracker`, `rancher`, `hobo`, `laborer`, `fallen_trustafarian` (8 total)

### Schemer (Witch)
Manipulation, dark knowledge, street pharmacology, and effects that look like hexes to outsiders.

```yaml
id: schemer
name: Schemer
description: "Knowledge is power, and Schemers collect both. Through chemical mastery, psychological manipulation, and forbidden insight, they bend situations — and people — to their will."
key_ability: savvy
hit_points_per_level: 6
ability_boosts:
  fixed: [savvy, flair]
  free: 2
```

Jobs: `maker`, `salesman`, `narcomancer`, `illusionist`, `grifter`, `dealer`, `mall_ninja`, `shit_stirrer` (8 total)

---

## Section 2: Normie Dissolution

`content/archetypes/normie.yaml` is deleted. Its 9 jobs are redistributed:

| Job | Old Archetype | New Archetype |
|-----|--------------|--------------|
| cult_leader | normie | zealot |
| hired_help | normie | zealot |
| medic | normie | zealot |
| maker | normie | schemer |
| salesman | normie | schemer |
| exterminator | normie | naturalist |
| driver | normie | drifter |
| pilot | normie | drifter |
| hanger_on | normie | criminal |

---

## Section 3: Full Job Reassignment Table

All job moves (archetype field updated in job YAML):

| Job | From | To |
|-----|------|----|
| pastor | nerd | zealot |
| believer | nerd | zealot |
| street_preacher | drifter | zealot |
| trainee | aggressor | zealot |
| guard | aggressor | zealot |
| follower | influencer | zealot |
| vigilante | criminal | zealot |
| hippie | nerd | naturalist |
| freegan | drifter | naturalist |
| tracker | drifter | naturalist |
| rancher | drifter | naturalist |
| hobo | criminal | naturalist |
| laborer | aggressor | naturalist |
| fallen_trustafarian | influencer | naturalist |
| narcomancer | nerd | schemer |
| illusionist | drifter | schemer |
| grifter | criminal | schemer |
| dealer | drifter | schemer |
| mall_ninja | aggressor | schemer |
| shit_stirrer | influencer | schemer |
| goon | influencer | aggressor |
| muscle | criminal | aggressor |
| pirate | aggressor | drifter |
| free_spirit | criminal | drifter |
| contract_killer | aggressor | criminal |
| specialist | aggressor | nerd |

---

## Section 4: Final Job Counts

| Archetype | Jobs |
|-----------|------|
| Aggressor | 12 |
| Influencer | 10 |
| Criminal | 9 |
| Nerd | 9 |
| Drifter | 10 |
| Zealot | 10 |
| Naturalist | 8 |
| Schemer | 8 |
| **Total** | **76** |

---

## Section 5: DB Migration

Characters whose job moves archetype have stale `character_ability_boosts` rows for `source='archetype'`. These are cleared so players are re-prompted at next login.

Migration file: `migrations/017_clear_reassigned_archetype_boosts.up.sql`

```sql
DELETE FROM character_ability_boosts
WHERE source = 'archetype'
AND character_id IN (
    SELECT id FROM characters
    WHERE class IN (
        'pastor','believer','street_preacher','trainee','guard','follower','vigilante',
        'hippie','freegan','tracker','rancher','hobo','laborer','fallen_trustafarian',
        'narcomancer','illusionist','grifter','dealer','mall_ninja','shit_stirrer',
        'goon','muscle','pirate','free_spirit','contract_killer','specialist',
        'cult_leader','hired_help','medic','maker','salesman','exterminator',
        'driver','pilot','hanger_on'
    )
);
```

Down migration: no-op (cleared data is not recoverable; players will be re-prompted).

```sql
-- no-op: cannot restore deleted ability boost choices
SELECT 1;
```

---

## Section 6: Testing

- **Load**: all 8 archetypes load; each has non-nil `AbilityBoosts`
- **No orphan jobs**: every job YAML references a valid archetype ID; none references `normie`
- **Job count invariant**: total job count remains 76; each job appears in exactly one archetype
- **Character creation**: archetype selection presents exactly 8 options; job list filtered correctly per archetype
- **Ability boost re-prompt**: a character with a reassigned job is re-prompted for archetype boosts at login after migration clears their old choices
