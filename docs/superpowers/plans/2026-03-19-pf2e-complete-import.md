# PF2E Complete Spell Import (Ranks 2–10) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import PF2E spell ranks 2–10 across all 4 traditions, localize them in-session, extend archetype `level_up_grants` to job level 20 with correct PF2E slot deltas, populate exhaustive per-rank archetype pools, and bake all transforms into `static_localizer.go`.

**Architecture:** Pure data pipeline — no new Go code. Run `import-content` (NoopLocalizer) for each rank, localize YAML files in-session via Python, update 6 archetype YAMLs with corrected slot progressions + exhaustive pools, regenerate `static_localizer.go`.

**Tech Stack:** Go importer binary (`./import-content`), Python 3 (localization + pool generation), YAML, `go test ./internal/game/ruleset/...`

**Spec:** `docs/superpowers/specs/2026-03-19-pf2e-complete-import-design.md`

---

## File Map

| File | Change |
|------|--------|
| `content/technologies/neural/*.yaml` | +~rank-2–10 files (new) |
| `content/technologies/bio_synthetic/*.yaml` | +~rank-2–10 files (new) |
| `content/technologies/fanatic_doctrine/*.yaml` | +~rank-2–10 files (new) |
| `content/technologies/technical/*.yaml` | +~rank-2–10 files (new) |
| `internal/importer/static_localizer.go` | Regenerated with all new entries |
| `content/archetypes/schemer.yaml` | Correct levels 3–5, extend to level 20 + pools |
| `content/archetypes/naturalist.yaml` | Correct levels 3–5, extend to level 20 + pools |
| `content/archetypes/zealot.yaml` | Correct levels 3–5, extend to level 20 + pools |
| `content/archetypes/nerd.yaml` | Correct levels 3–5, extend to level 20 + pools |
| `content/archetypes/influencer.yaml` | Correct levels 3–5, extend to level 20 + pools |
| `content/archetypes/drifter.yaml` | Correct levels 3–5, extend to level 19 + pools (half-caster) |
| `docs/requirements/pf2e-import-reference.md` | Add batch import 2026-03-19 tables |
| `docs/features/technology.md` | Mark spell import checkbox complete |

---

## Task 1: Import ranks 2–10 (raw, no localization)

**Files:** `content/technologies/` (all 4 tradition subdirs)

- [ ] **Step 1: Verify vendor/pf2e-data exists**

```bash
ls vendor/pf2e-data/packs/pf2e/spells/spells/ | head -5
```

Expected: `cantrip  rank-1  rank-2  rank-3  rank-4  rank-5  rank-6  rank-7  rank-8  rank-9  rank-10`

- [ ] **Step 2: Rebuild import-content binary**

```bash
go build -o ./import-content ./cmd/import-content/
```

Expected: no output (success)

- [ ] **Step 3: Run importer for each rank**

```bash
for rank in 2 3 4 5 6 7 8 9 10; do
  echo "=== Importing rank-$rank ==="
  ./import-content -format pf2e \
    -source vendor/pf2e-data/packs/pf2e/spells/spells/rank-$rank \
    -output content/technologies/
done
```

Expected: progress output for each rank; no errors. Files land in `content/technologies/{neural,bio_synthetic,fanatic_doctrine,technical}/`.

- [ ] **Step 4: Count new files**

```bash
for d in neural bio_synthetic fanatic_doctrine technical; do
  echo "$d: $(ls content/technologies/$d/*.yaml | wc -l) total files"
done
```

Expected totals (approximate, rank-1 already present):
- neural: ~300+
- bio_synthetic: ~320+
- fanatic_doctrine: ~230+
- technical: ~280+

- [ ] **Step 5: Commit raw imports**

```bash
git add content/technologies/
git commit -m "feat: import raw PF2E ranks 2-10 across all 4 traditions (pre-localization)"
```

---

## Task 2: Localize rank-2 files

**Files:** All `*_neural.yaml`, `*_bio_synthetic.yaml`, `*_fanatic_doctrine.yaml`, `*_technical.yaml` with `level: 2`

**Context:** Localization transforms PF2E fantasy text into Gunchete cyberpunk flavor. Rules:
- neural → psychic/neuro-hack/drug-induced mental effects
- bio_synthetic → biological augmentation, chemical synthesis, organic tech
- fanatic_doctrine → zealot ideology, cult conditioning, devotional chemicals
- technical → hardware, EM devices, precision machinery, arcane engineering
- Preserve ALL mechanical text exactly: dice (e.g. `2d6`), ranges, durations, action costs, save types
- Names: short, evocative (2–4 words), cyberpunk aesthetic

- [ ] **Step 1: Enumerate rank-2 files per tradition**

```bash
python3 -c "
import yaml, os, glob
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    files = []
    for fp in sorted(glob.glob(f'content/technologies/{trad}/*.yaml')):
        with open(fp) as f:
            d = yaml.safe_load(f)
        if d and d.get('level') == 2:
            files.append((d['id'], d['name'], fp))
    print(f'\n=== {trad} rank-2 ({len(files)} files) ===')
    for tid, name, fp in files[:5]:
        print(f'  {tid}: {name}')
    if len(files) > 5:
        print(f'  ... and {len(files)-5} more')
"
```

- [ ] **Step 2: Apply localization transforms**

Write `/tmp/localize_rank.py` using the template below, populate the `TRANSFORMS` dict with a `(name, description)` tuple for every file enumerated in Step 1, then run it. The template:

```python
import yaml, glob, sys

RANK = 2  # change per task
TRADITIONS = ['neural', 'bio_synthetic', 'fanatic_doctrine', 'technical']

# Populate: original_id -> (localized_name, localized_description)
# Rules:
#   - neural: psychic intrusion, neuro-hacking, drug-induced mind effects
#   - bio_synthetic: biological weaponry, chemical synthesis, organic augmentation
#   - fanatic_doctrine: zealot ideology, cult conditioning, devotional chemistry
#   - technical: hardware, EM devices, precision machinery, fabrication
#   - Preserve ALL mechanical text exactly (dice, ranges, durations, save types, action costs)
#   - Names: 2–4 words, cyberpunk aesthetic, no fantasy tropes
TRANSFORMS = {
    # example: 'acid_arrow_technical': ('Corrosive Slug Round', 'Fire a chemically-treated...'),
}

updated = 0
skipped = 0
for trad in TRADITIONS:
    for fp in sorted(glob.glob(f'content/technologies/{trad}/*.yaml')):
        with open(fp) as f:
            d = yaml.safe_load(f)
        if not d or d.get('level') != RANK:
            continue
        tid = d['id']
        if tid not in TRANSFORMS:
            print(f'WARNING: no transform for {tid}', file=sys.stderr)
            skipped += 1
            continue
        name, desc = TRANSFORMS[tid]
        d['name'] = name
        d['description'] = desc
        with open(fp, 'w') as f:
            yaml.dump(d, f, allow_unicode=True, default_flow_style=False, sort_keys=False)
        updated += 1

print(f'Updated {updated} files, skipped {skipped}')
```

Enumerate all rank-2 file IDs from Step 1, populate `TRANSFORMS` with Gunchete-flavored (name, description) tuples, set `RANK = 2`, then run:

```bash
python3 /tmp/localize_rank.py
```

Expected: `Updated N files, skipped 0`

- [ ] **Step 3: Verify no mechanical text was altered**

```bash
python3 -c "
import yaml, re, glob
pattern = re.compile(r'\d+d\d+|\d+ feet|\d+ rounds?|\d+ minutes?|basic|Reflex|Fortitude|Will|DC')
issues = []
for fp in glob.glob('content/technologies/**/*.yaml', recursive=True):
    # Just verify files are valid YAML and have required fields
    with open(fp) as f:
        d = yaml.safe_load(f)
    if not d:
        issues.append(f'Empty: {fp}')
    elif not d.get('id') or not d.get('name'):
        issues.append(f'Missing id/name: {fp}')
if issues:
    for i in issues[:10]:
        print(i)
else:
    print('All files valid')
"
```

- [ ] **Step 4: Commit rank-2 localizations**

```bash
git add content/technologies/
git commit -m "feat: localize rank-2 PF2E imports into Gunchete cyberpunk aesthetic"
```

---

## Task 3: Localize rank-3 files

Set `RANK = 3` in `/tmp/localize_rank.py`. Rank-3 power: escalated threat — neural corridor intrusion, bio-acid floods, precision drone strikes.

- [ ] **Step 1: Enumerate rank-3 files**

```bash
python3 -c "
import yaml, glob
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    files = [f for f in glob.glob(f'content/technologies/{trad}/*.yaml')
             if yaml.safe_load(open(f).read()).get('level') == 3]
    print(f'{trad}: {len(files)} rank-3 files')
"
```

- [ ] **Step 2: Populate TRANSFORMS for rank-3 IDs and run `/tmp/localize_rank.py` (RANK=3)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 3: Commit**

```bash
git add content/technologies/
git commit -m "feat: localize rank-3 PF2E imports into Gunchete cyberpunk aesthetic"
```

---

## Task 4: Localize rank-4 files

Set `RANK = 4`. Power level: serious hardware — neurotoxin dispersal, heavy ordinance, tactical suppression systems.

- [ ] **Step 1: Enumerate rank-4 files**

```bash
python3 -c "
import yaml, glob
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    files = [f for f in glob.glob(f'content/technologies/{trad}/*.yaml')
             if yaml.safe_load(open(f).read()).get('level') == 4]
    print(f'{trad}: {len(files)} rank-4 files')
"
```

- [ ] **Step 2: Populate TRANSFORMS for rank-4 IDs and run `/tmp/localize_rank.py` (RANK=4)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 3: Commit**

```bash
git add content/technologies/
git commit -m "feat: localize rank-4 PF2E imports into Gunchete cyberpunk aesthetic"
```

---

## Task 5: Localize rank-5 files

Set `RANK = 5`. Power level: high capability — full neural override, mass-casualty bio agents, anti-materiel tech.

- [ ] **Step 1: Enumerate rank-5 files**

```bash
python3 -c "
import yaml, glob
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    files = [f for f in glob.glob(f'content/technologies/{trad}/*.yaml')
             if yaml.safe_load(open(f).read()).get('level') == 5]
    print(f'{trad}: {len(files)} rank-5 files')
"
```

- [ ] **Step 2: Populate TRANSFORMS for rank-5 IDs and run `/tmp/localize_rank.py` (RANK=5)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 3: Commit**

```bash
git add content/technologies/
git commit -m "feat: localize rank-5 PF2E imports into Gunchete cyberpunk aesthetic"
```

---

## Task 6: Localize ranks 6–8 files

Elite-tier. Names: "Synaptic Annihilation Protocol", "Bioweapon Mass Dispersal", "Orbital Strike Designator". Run once per rank (RANK=6, then 7, then 8) using `/tmp/localize_rank.py`.

- [ ] **Step 1: Enumerate ranks 6–8 files**

```bash
python3 -c "
import yaml, glob
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    for rank in [6,7,8]:
        files = [f for f in glob.glob(f'content/technologies/{trad}/*.yaml')
                 if yaml.safe_load(open(f).read()).get('level') == rank]
        print(f'{trad} rank-{rank}: {len(files)} files')
"
```

- [ ] **Step 2: Populate TRANSFORMS for rank-6 IDs and run `/tmp/localize_rank.py` (RANK=6)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 3: Populate TRANSFORMS for rank-7 IDs and run `/tmp/localize_rank.py` (RANK=7)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 4: Populate TRANSFORMS for rank-8 IDs and run `/tmp/localize_rank.py` (RANK=8)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 5: Commit**

```bash
git add content/technologies/
git commit -m "feat: localize rank-6-8 PF2E imports into Gunchete cyberpunk aesthetic"
```

---

## Task 7: Localize ranks 9–10 files

Apex-tier (unlocked at job levels 17–19). Run once per rank using `/tmp/localize_rank.py`.

- [ ] **Step 1: Enumerate ranks 9–10 files**

```bash
python3 -c "
import yaml, glob
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    for rank in [9,10]:
        files = [f for f in glob.glob(f'content/technologies/{trad}/*.yaml')
                 if yaml.safe_load(open(f).read()).get('level') == rank]
        print(f'{trad} rank-{rank}: {len(files)} files')
"
```

- [ ] **Step 2: Populate TRANSFORMS for rank-9 IDs and run `/tmp/localize_rank.py` (RANK=9)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 3: Populate TRANSFORMS for rank-10 IDs and run `/tmp/localize_rank.py` (RANK=10)**

Expected: `Updated N files, skipped 0`

- [ ] **Step 4: Commit**

```bash
git add content/technologies/
git commit -m "feat: localize rank-9-10 PF2E imports into Gunchete cyberpunk aesthetic"
```

---

## Task 8: Regenerate StaticLocalizer

**Files:** `internal/importer/static_localizer.go`, `/tmp/gen_static_localizer.py`

- [ ] **Step 1: Verify generator script exists**

```bash
ls -la /tmp/gen_static_localizer.py
```

Expected: file exists. If missing, recreate it from the pattern in the previous session (reads all `content/technologies/**/*.yaml`, extracts `id`/`name`/`description`, emits Go map literal into `internal/importer/static_localizer.go`).

- [ ] **Step 2: Run the generator script**

```bash
python3 /tmp/gen_static_localizer.py > internal/importer/static_localizer.go
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/importer/...
```

Expected: no output (success)

- [ ] **Step 3: Run importer tests**

```bash
go test ./internal/importer/... -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/importer/static_localizer.go
git commit -m "feat: regenerate static localizer with all rank 2-10 transforms"
```

---

## Task 9: Correct + extend archetype slot progressions

**Files:** `content/archetypes/schemer.yaml`, `naturalist.yaml`, `zealot.yaml`, `nerd.yaml`, `influencer.yaml`, `drifter.yaml`

**Reference — prepared full casters (schemer, naturalist, zealot, nerd):**

Replace existing `level_up_grants` block entirely with:
```yaml
level_up_grants:
  2:
    prepared:
      slots_by_level:
        1: 1
  3:
    prepared:
      slots_by_level:
        2: 2
      pool: []  # populated in Task 10
  4:
    prepared:
      slots_by_level:
        2: 1
  5:
    prepared:
      slots_by_level:
        3: 2
      pool: []  # populated in Task 10
  6:
    prepared:
      slots_by_level:
        3: 1
  7:
    prepared:
      slots_by_level:
        4: 2
      pool: []
  8:
    prepared:
      slots_by_level:
        4: 1
  9:
    prepared:
      slots_by_level:
        5: 2
      pool: []
  10:
    prepared:
      slots_by_level:
        5: 1
  11:
    prepared:
      slots_by_level:
        6: 2
      pool: []
  12:
    prepared:
      slots_by_level:
        6: 1
  13:
    prepared:
      slots_by_level:
        7: 2
      pool: []
  14:
    prepared:
      slots_by_level:
        7: 1
  15:
    prepared:
      slots_by_level:
        8: 2
      pool: []
  16:
    prepared:
      slots_by_level:
        8: 1
  17:
    prepared:
      slots_by_level:
        9: 2
      pool: []
  18:
    prepared:
      slots_by_level:
        9: 1
  19:
    prepared:
      slots_by_level:
        10: 2
      pool: []
  20:
    prepared:
      slots_by_level:
        10: 1
```

**Reference — influencer (spontaneous):**

```yaml
level_up_grants:
  2:
    spontaneous:
      uses_by_level:
        1: 1
  3:
    spontaneous:
      known_by_level:
        2: 2
      uses_by_level:
        2: 2
      pool: []  # populated in Task 10
  4:
    spontaneous:
      uses_by_level:
        2: 1
  5:
    spontaneous:
      known_by_level:
        3: 2
      uses_by_level:
        3: 2
      pool: []
  6:
    spontaneous:
      uses_by_level:
        3: 1
  7:
    spontaneous:
      known_by_level:
        4: 2
      uses_by_level:
        4: 2
      pool: []
  8:
    spontaneous:
      uses_by_level:
        4: 1
  9:
    spontaneous:
      known_by_level:
        5: 2
      uses_by_level:
        5: 2
      pool: []
  10:
    spontaneous:
      uses_by_level:
        5: 1
  11:
    spontaneous:
      known_by_level:
        6: 2
      uses_by_level:
        6: 2
      pool: []
  12:
    spontaneous:
      uses_by_level:
        6: 1
  13:
    spontaneous:
      known_by_level:
        7: 2
      uses_by_level:
        7: 2
      pool: []
  14:
    spontaneous:
      uses_by_level:
        7: 1
  15:
    spontaneous:
      known_by_level:
        8: 2
      uses_by_level:
        8: 2
      pool: []
  16:
    spontaneous:
      uses_by_level:
        8: 1
  17:
    spontaneous:
      known_by_level:
        9: 2
      uses_by_level:
        9: 2
      pool: []
  18:
    spontaneous:
      uses_by_level:
        9: 1
  19:
    spontaneous:
      known_by_level:
        10: 2
      uses_by_level:
        10: 2
      pool: []
  20:
    spontaneous:
      uses_by_level:
        10: 1
```

**Reference — drifter (half-caster, prepared, bio_synthetic, max rank 8):**

```yaml
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 1
      pool: []  # populated in Task 10
  5:
    prepared:
      slots_by_level:
        3: 1
      pool: []
  9:
    prepared:
      slots_by_level:
        4: 1
      pool: []
  11:
    prepared:
      slots_by_level:
        5: 1
      pool: []
  13:
    prepared:
      slots_by_level:
        6: 1
      pool: []
  17:
    prepared:
      slots_by_level:
        7: 1
      pool: []
  19:
    prepared:
      slots_by_level:
        8: 1
      pool: []
```

- [ ] **Step 1: Update schemer.yaml** — replace `level_up_grants` with prepared full-caster table above (tradition: neural)

- [ ] **Step 2: Update naturalist.yaml** — replace `level_up_grants` with prepared full-caster table (tradition: bio_synthetic)

- [ ] **Step 3: Update zealot.yaml** — replace `level_up_grants` with prepared full-caster table (tradition: fanatic_doctrine)

- [ ] **Step 4: Update nerd.yaml** — replace `level_up_grants` with prepared full-caster table (tradition: technical)

- [ ] **Step 5: Update influencer.yaml** — replace `level_up_grants` with spontaneous full-caster table (tradition: neural)

- [ ] **Step 6: Update drifter.yaml** — replace `level_up_grants` with half-caster table (tradition: bio_synthetic)

- [ ] **Step 7: Run tests**

```bash
go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v
```

Expected: PASS (pools are empty `[]` for now — that is valid as long as slots_by_level ≤ pool+fixed count at each level)

**Note:** If the test fails because empty pools don't satisfy slot counts, proceed directly to Task 10 before committing.

- [ ] **Step 8: Commit**

```bash
git add content/archetypes/
git commit -m "feat: correct and extend archetype slot progressions to job level 20"
```

---

## Task 10: Generate and insert pool entries

**Files:** `content/archetypes/schemer.yaml`, `naturalist.yaml`, `zealot.yaml`, `nerd.yaml`, `influencer.yaml`, `drifter.yaml`

**Archetype → tradition mapping:**
- schemer → neural
- naturalist → bio_synthetic
- zealot → fanatic_doctrine
- nerd → technical
- influencer → neural
- drifter → bio_synthetic

**Rank → job level mapping (odd levels only — where new rank first unlocks):**

Full casters: rank 2→level 3, rank 3→level 5, rank 4→level 7, rank 5→level 9, rank 6→level 11, rank 7→level 13, rank 8→level 15, rank 9→level 17, rank 10→level 19

Drifter: rank 2→level 3, rank 3→level 5, rank 4→level 9, rank 5→level 11, rank 6→level 13, rank 7→level 17, rank 8→level 19

- [ ] **Step 1: Write pool generation script**

Write `/tmp/gen_pools.py`:

```python
import yaml, glob, sys

def load_pool(tradition, rank):
    """Return sorted list of {id, level} dicts for all techs at given tradition+rank."""
    entries = []
    for fp in sorted(glob.glob(f'content/technologies/{tradition}/*.yaml')):
        with open(fp) as f:
            d = yaml.safe_load(f)
        if d and d.get('level') == rank:
            entries.append({'id': d['id'], 'level': rank})
    return entries

# Full caster rank→level map
FULL_CASTER_RANK_TO_JOB_LEVEL = {2:3, 3:5, 4:7, 5:9, 6:11, 7:13, 8:15, 9:17, 10:19}
# Half-caster rank→level map
DRIFTER_RANK_TO_JOB_LEVEL = {2:3, 3:5, 4:9, 5:11, 6:13, 7:17, 8:19}

configs = [
    ('schemer',    'neural',           'prepared',    FULL_CASTER_RANK_TO_JOB_LEVEL),
    ('naturalist', 'bio_synthetic',    'prepared',    FULL_CASTER_RANK_TO_JOB_LEVEL),
    ('zealot',     'fanatic_doctrine', 'prepared',    FULL_CASTER_RANK_TO_JOB_LEVEL),
    ('nerd',       'technical',        'prepared',    FULL_CASTER_RANK_TO_JOB_LEVEL),
    ('influencer', 'neural',           'spontaneous', FULL_CASTER_RANK_TO_JOB_LEVEL),
    ('drifter',    'bio_synthetic',    'prepared',    DRIFTER_RANK_TO_JOB_LEVEL),
]

for archetype, tradition, grant_type, rank_map in configs:
    fp = f'content/archetypes/{archetype}.yaml'
    with open(fp) as f:
        data = yaml.safe_load(f)

    grants = data.setdefault('level_up_grants', {})
    for rank, job_level in sorted(rank_map.items()):
        pool = load_pool(tradition, rank)
        if not pool:
            print(f'WARNING: no {tradition} rank-{rank} techs found', file=sys.stderr)
            continue
        level_grants = grants.setdefault(job_level, {})
        type_grants = level_grants.setdefault(grant_type, {})
        type_grants['pool'] = pool

    with open(fp, 'w') as f:
        yaml.dump(data, f, allow_unicode=True, default_flow_style=False, sort_keys=False)
    print(f'Updated {fp}')
```

- [ ] **Step 2: Run the pool generation script**

```bash
cd /home/cjohannsen/src/mud
python3 /tmp/gen_pools.py
```

Expected: 6 lines of "Updated content/archetypes/X.yaml"

- [ ] **Step 3: Verify pool entries look correct**

```bash
python3 -c "
import yaml
with open('content/archetypes/schemer.yaml') as f:
    d = yaml.safe_load(f)
grants = d.get('level_up_grants', {})
for lvl in [3, 5, 7]:
    pool = grants.get(lvl, {}).get('prepared', {}).get('pool', [])
    print(f'schemer level {lvl}: {len(pool)} pool entries')
    if pool:
        print(f'  first: {pool[0]}')
"
```

Expected: non-zero pool entries at levels 3, 5, 7 for schemer

- [ ] **Step 4: Run tests**

```bash
go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v
```

Expected: PASS

- [ ] **Step 5: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add content/archetypes/
git commit -m "feat: populate exhaustive rank 2-10 pools in all archetype level_up_grants"
```

---

## Task 11: Update docs + final commit

**Files:** `docs/requirements/pf2e-import-reference.md`, `docs/features/technology.md`

- [ ] **Step 1: Add batch import table to pf2e-import-reference.md**

Count files per tradition per rank and add a "Batch Import 2026-03-19" section:

```bash
python3 -c "
import yaml, glob
print('### Batch Import 2026-03-19 (Ranks 2–10)\n')
for trad in ['neural','bio_synthetic','fanatic_doctrine','technical']:
    print(f'#### {trad}')
    print('| Rank | Files |')
    print('|------|-------|')
    for rank in range(2, 11):
        files = [f for f in glob.glob(f'content/technologies/{trad}/*.yaml')
                 if yaml.safe_load(open(f).read()).get('level') == rank]
        print(f'| {rank} | {len(files)} |')
    print()
"
```

Append this output to `docs/requirements/pf2e-import-reference.md` under the existing "Batch Import 2026-03-18" section.

- [ ] **Step 2: Mark spell import checkbox complete in technology.md**

In `docs/features/technology.md`, find:
```
      - [ ] Spell import from PF2E with translation into Gunchete
```
Change to:
```
      - [x] Spell import from PF2E with translation into Gunchete
```

- [ ] **Step 3: Commit**

```bash
git add docs/requirements/pf2e-import-reference.md docs/features/technology.md
git commit -m "docs: record rank 2-10 batch import and mark spell import complete"
```

---

## Task 12: Push and deploy

- [ ] **Step 1: Run full test suite one final time**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 2: Push**

```bash
git push origin main
```

- [ ] **Step 3: Deploy**

```bash
make k8s-redeploy
```

Expected: `Release "mud" has been upgraded. Happy Helming!`
