# Tradition Innate Technologies as Cantrips — Design Spec

## Goal

Every character whose archetype has a technology tradition must automatically receive all innate technologies for that tradition at character creation, with unlimited uses. This mirrors PF2E cantrips: a Wizard gets all cantrips for their school; an Engineer gets all innate techs for their tradition. Region innate technologies remain a bonus granted on top of the tradition pool.

## Background

The current system supports `archetype.InnateTechnologies []InnateGrant` and `AssignTechnologies` already processes this field. The gate introduced in migration 062 ensures only tech-capable archetypes receive innate grants. The remaining work is purely content: populate the archetype YAMLs and create missing innate tech files for the neural and fanatic_doctrine traditions.

**Current tradition innate tech inventory:**

| Tradition | Count | IDs |
|-----------|-------|-----|
| technical | 5 | atmospheric_surge, blackout_pulse, seismic_sense, arc_lights, pressure_burst |
| bio_synthetic | 4 | moisture_reclaim, viscous_spray, nanite_infusion, acid_spit |
| neural | 2 | terror_broadcast, chrome_reflex |
| fanatic_doctrine | 0 | — |

## Tech-Capability Reference

| Archetype | Tradition |
|-----------|-----------|
| nerd | technical |
| naturalist | bio_synthetic |
| drifter | bio_synthetic |
| schemer | neural |
| influencer | neural |
| zealot | fanatic_doctrine |
| aggressor | — (non-tech) |
| criminal | — (non-tech) |

## Changes

### 1. New Innate Tech YAML Files

Three new neural innate tech files and five new fanatic_doctrine innate tech files in `content/technologies/innate/`.

**Neural additions:**

**`neural_flare.yaml`** — Single target pain overload; target saves vs Cool DC 15 or Sickened 1 for 1 round; crit failure adds Slowed 1.
- action_cost: 2, range: single, targets: single, resolution: save, save_type: cool, save_dc: 15

**`static_veil.yaml`** — Self-targeted mesh interference burst; grants Concealed for 1 round.
- action_cost: 1, range: self, targets: single, resolution: none

**`synapse_tap.yaml`** — Passive emotional state reader; grants +1 circumstance bonus on Deception and Intimidation checks against creatures in zone.
- action_cost: 0, passive: true, range: zone, targets: all

**Fanatic Doctrine additions:**

**`doctrine_ward.yaml`** — Passive faith-hardened subdermal plating; reduce all incoming damage by 1.
- action_cost: 0, passive: true, range: self, targets: single

**`martyrs_resolve.yaml`** — Reaction on reaching 0 HP; stabilize at 1 HP. Once per scene.
- action_cost: 1, reaction triggers: on_reduced_to_zero_hp, resolution: none

**`righteous_condemnation.yaml`** — Mark a single target; your attacks deal +1d4 additional damage against them for 1 round.
- action_cost: 1, range: single, targets: single, duration: rounds:1, resolution: none

**`fervor_pulse.yaml`** — Zone radiate zealous frequency; all enemies save vs Cool DC 15 or Frightened 1 for 1 round; crit failure adds Fleeing 1.
- action_cost: 2, range: zone, targets: all_enemies, resolution: save, save_type: cool, save_dc: 15

**`litany_of_iron.yaml`** — Steel your will; gain +2 circumstance bonus to all saving throws for 1 round.
- action_cost: 1, range: self, targets: single, duration: rounds:1, resolution: none

### 2. Archetype YAML Updates

Six archetype YAMLs in `content/archetypes/` gain an `innate_technologies` block. All grants use `uses_per_day: 0` (unlimited).

**`nerd.yaml`** — technical tradition:
```yaml
innate_technologies:
  - id: atmospheric_surge
    uses_per_day: 0
  - id: blackout_pulse
    uses_per_day: 0
  - id: seismic_sense
    uses_per_day: 0
  - id: arc_lights
    uses_per_day: 0
  - id: pressure_burst
    uses_per_day: 0
```

**`naturalist.yaml`** — bio_synthetic tradition:
```yaml
innate_technologies:
  - id: moisture_reclaim
    uses_per_day: 0
  - id: viscous_spray
    uses_per_day: 0
  - id: nanite_infusion
    uses_per_day: 0
  - id: acid_spit
    uses_per_day: 0
```

**`drifter.yaml`** — bio_synthetic tradition (same as naturalist):
```yaml
innate_technologies:
  - id: moisture_reclaim
    uses_per_day: 0
  - id: viscous_spray
    uses_per_day: 0
  - id: nanite_infusion
    uses_per_day: 0
  - id: acid_spit
    uses_per_day: 0
```

**`schemer.yaml`** — neural tradition:
```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 0
  - id: chrome_reflex
    uses_per_day: 0
  - id: neural_flare
    uses_per_day: 0
  - id: static_veil
    uses_per_day: 0
  - id: synapse_tap
    uses_per_day: 0
```

**`influencer.yaml`** — neural tradition (same as schemer):
```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 0
  - id: chrome_reflex
    uses_per_day: 0
  - id: neural_flare
    uses_per_day: 0
  - id: static_veil
    uses_per_day: 0
  - id: synapse_tap
    uses_per_day: 0
```

**`zealot.yaml`** — fanatic_doctrine tradition:
```yaml
innate_technologies:
  - id: doctrine_ward
    uses_per_day: 0
  - id: martyrs_resolve
    uses_per_day: 0
  - id: righteous_condemnation
    uses_per_day: 0
  - id: fervor_pulse
    uses_per_day: 0
  - id: litany_of_iron
    uses_per_day: 0
```

### 3. DB Migration (063)

Existing characters do not have tradition innate techs because archetype `innate_technologies` was empty at character creation. Migration 063 inserts missing rows for all tech-capable characters.

**Migration file:** `migrations/063_tradition_innate_techs.up.sql`

The migration uses a cross-join between characters and their tradition's innate tech list, inserting only rows that do not already exist:

```sql
-- REQ-TIT-1: Insert missing tradition innate techs for tech-capable characters.
-- ON CONFLICT DO NOTHING makes this idempotent.

-- technical tradition: nerd jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('atmospheric_surge'), ('blackout_pulse'), ('seismic_sense'), ('arc_lights'), ('pressure_burst')
) AS t(tech_id)
WHERE c.class IN (
    'natural_mystic', 'specialist', 'detective', 'journalist', 'hoarder',
    'grease_monkey', 'narc', 'engineer', 'cooker'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- bio_synthetic tradition: naturalist and drifter jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('moisture_reclaim'), ('viscous_spray'), ('nanite_infusion'), ('acid_spit')
) AS t(tech_id)
WHERE c.class IN (
    'rancher', 'hippie', 'laborer', 'hobo', 'tracker', 'freegan',
    'exterminator', 'fallen_trustafarian',
    'scout', 'cop', 'psychopath', 'driver', 'bagman', 'pilot',
    'warden', 'stalker', 'pirate', 'free_spirit'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- neural tradition: schemer and influencer jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('terror_broadcast'), ('chrome_reflex'), ('neural_flare'), ('static_veil'), ('synapse_tap')
) AS t(tech_id)
WHERE c.class IN (
    'narcomancer', 'maker', 'grifter', 'dealer', 'shit_stirrer',
    'salesman', 'mall_ninja', 'illusionist',
    'karen', 'politician', 'libertarian', 'entertainer', 'antifa',
    'bureaucrat', 'exotic_dancer', 'schmoozer', 'extortionist', 'anarchist'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- fanatic_doctrine tradition: zealot jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('doctrine_ward'), ('martyrs_resolve'), ('righteous_condemnation'),
    ('fervor_pulse'), ('litany_of_iron')
) AS t(tech_id)
WHERE c.class IN (
    'cult_leader', 'street_preacher', 'medic', 'guard', 'believer',
    'hired_help', 'vigilante', 'follower', 'trainee', 'pastor'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;
```

**Down migration:** No-op — the inserted rows are correct and removing them would be destructive.

## Requirements

- REQ-TIT-1: All tech-capable archetypes MUST have `innate_technologies` populated with all tradition innate techs at `uses_per_day: 0`.
- REQ-TIT-2: New innate tech YAML files MUST be created for the 3 missing neural techs and 5 missing fanatic_doctrine techs.
- REQ-TIT-3: `AssignTechnologies` MUST NOT require code changes; the archetype YAML content change is sufficient.
- REQ-TIT-4: Migration 063 MUST insert missing tradition innate tech rows for all existing tech-capable characters using `ON CONFLICT DO NOTHING` for idempotency.
- REQ-TIT-5: Region innate techs MUST continue to be granted as a bonus on top of tradition innate techs.
- REQ-TIT-6: All new innate techs MUST be level 1 with `usage_type: innate`.

## Testing

- Property test: for all tech-capable job IDs, `AssignTechnologies` grants at minimum all tradition innate techs for their archetype's tradition.
- Property test: all granted tradition innate slots have `MaxUses == 0` and `UsesRemaining == 0`.
- Unit test: a nerd character receives all 5 technical innate techs.
- Unit test: a zealot character receives all 5 fanatic_doctrine innate techs.
- Unit test: region innate tech is granted in addition to (not instead of) tradition innate techs.
- Unit test: aggressor and criminal characters receive zero tradition innate techs.
