# Archetype Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add three new archetypes (Zealot, Naturalist, Schemer), dissolve Normie, and redistribute 26 jobs across archetypes to balance counts and improve lore fit.

**Architecture:** Pure content changes — new/deleted archetype YAML files, updated `archetype:` field in job YAML files, and a DB migration that clears stale archetype ability boost choices for characters whose job moved archetypes. No Go code changes required; the system loads archetypes and jobs dynamically from disk.

**Tech Stack:** YAML content files, PostgreSQL migration (SQL), Go test suite (`go test ./internal/game/ruleset/...`)

---

### Task 1: Create three new archetype YAML files

**Files:**
- Create: `content/archetypes/zealot.yaml`
- Create: `content/archetypes/naturalist.yaml`
- Create: `content/archetypes/schemer.yaml`

**Step 1: Create `content/archetypes/zealot.yaml`**

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

**Step 2: Create `content/archetypes/naturalist.yaml`**

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

**Step 3: Create `content/archetypes/schemer.yaml`**

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

**Step 4: Run the existing archetype tests to verify the new files load correctly**

```bash
go test ./internal/game/ruleset/... -run "TestLoadArchetypes|TestArchetypeYAML|TestAllArchetypesHave" -v
```

Expected: all PASS (the tests load from `content/archetypes` and validate key abilities, ability_boosts, and job coverage).

Note: `TestAllArchetypesHaveJobsForBothTeams` will FAIL for the new archetypes until jobs are assigned in Task 2. That is expected and acceptable at this stage.

**Step 5: Commit**

```bash
git add content/archetypes/zealot.yaml content/archetypes/naturalist.yaml content/archetypes/schemer.yaml
git commit -m "feat: add Zealot, Naturalist, and Schemer archetype YAML files"
```

---

### Task 2: Delete normie.yaml and reassign all Normie jobs

**Files:**
- Delete: `content/archetypes/normie.yaml`
- Modify: `content/jobs/cult_leader.yaml` — change `archetype: normie` → `archetype: zealot`
- Modify: `content/jobs/hired_help.yaml` — change `archetype: normie` → `archetype: zealot`
- Modify: `content/jobs/medic.yaml` — change `archetype: normie` → `archetype: zealot`
- Modify: `content/jobs/maker.yaml` — change `archetype: normie` → `archetype: schemer`
- Modify: `content/jobs/salesman.yaml` — change `archetype: normie` → `archetype: schemer`
- Modify: `content/jobs/exterminator.yaml` — change `archetype: normie` → `archetype: naturalist`
- Modify: `content/jobs/driver.yaml` — change `archetype: normie` → `archetype: drifter`
- Modify: `content/jobs/pilot.yaml` — change `archetype: normie` → `archetype: drifter`
- Modify: `content/jobs/hanger_on.yaml` — change `archetype: normie` → `archetype: criminal`

In each job YAML, find the line `archetype: normie` and replace it with the new archetype.

**Step 1: Delete normie.yaml**

```bash
rm content/archetypes/normie.yaml
```

**Step 2: Update each Normie job file**

For each file listed above, open it and change the `archetype:` line. Example for `cult_leader.yaml`:
```yaml
archetype: zealot   # was: normie
```

**Step 3: Verify no job still references normie**

```bash
grep -r "archetype: normie" content/jobs/
```

Expected: no output.

**Step 4: Run tests**

```bash
go test ./internal/game/ruleset/... -v 2>&1 | tail -20
```

Expected: all previously-passing tests still pass. `TestAllArchetypesHaveJobsForBothTeams` may still fail for new archetypes (jobs not fully populated yet).

**Step 5: Commit**

```bash
git add content/archetypes/normie.yaml content/jobs/
git commit -m "feat: dissolve Normie archetype; redistribute its 9 jobs"
```

Note: `git add content/archetypes/normie.yaml` stages the deletion.

---

### Task 3: Reassign jobs to Zealot

**Files to modify** (change `archetype:` field only):
- `content/jobs/pastor.yaml` — `nerd` → `zealot`
- `content/jobs/believer.yaml` — `nerd` → `zealot`
- `content/jobs/street_preacher.yaml` — `drifter` → `zealot`
- `content/jobs/trainee.yaml` — `aggressor` → `zealot`
- `content/jobs/guard.yaml` — `aggressor` → `zealot`
- `content/jobs/follower.yaml` — `influencer` → `zealot`
- `content/jobs/vigilante.yaml` — `criminal` → `zealot`

**Step 1: Update each file**

In each file, find the `archetype:` line and change its value to `zealot`.

**Step 2: Verify**

```bash
grep -h "^archetype:" content/jobs/pastor.yaml content/jobs/believer.yaml \
  content/jobs/street_preacher.yaml content/jobs/trainee.yaml \
  content/jobs/guard.yaml content/jobs/follower.yaml content/jobs/vigilante.yaml
```

Expected: all lines read `archetype: zealot`.

**Step 3: Run tests**

```bash
go test ./internal/game/ruleset/... -run "TestAllArchetypesHaveJobsForBothTeams" -v
```

Zealot should now pass this test (has jobs for both gun and machete teams). Verify:

```bash
grep "^team:" content/jobs/pastor.yaml content/jobs/believer.yaml \
  content/jobs/street_preacher.yaml content/jobs/trainee.yaml \
  content/jobs/guard.yaml content/jobs/follower.yaml content/jobs/vigilante.yaml
```

If any Zealot job only covers one team, note it — the test requires both `gun` and `machete` team jobs per archetype.

**Step 4: Commit**

```bash
git add content/jobs/pastor.yaml content/jobs/believer.yaml content/jobs/street_preacher.yaml \
  content/jobs/trainee.yaml content/jobs/guard.yaml content/jobs/follower.yaml content/jobs/vigilante.yaml
git commit -m "feat: reassign 7 jobs to Zealot archetype"
```

---

### Task 4: Reassign jobs to Naturalist

**Files to modify:**
- `content/jobs/hippie.yaml` — `nerd` → `naturalist`
- `content/jobs/freegan.yaml` — `drifter` → `naturalist`
- `content/jobs/tracker.yaml` — `drifter` → `naturalist`
- `content/jobs/rancher.yaml` — `drifter` → `naturalist`
- `content/jobs/hobo.yaml` — `criminal` → `naturalist`
- `content/jobs/laborer.yaml` — `aggressor` → `naturalist`
- `content/jobs/fallen_trustafarian.yaml` — `influencer` → `naturalist`

**Step 1: Update each file**

In each file, find `archetype:` and change the value to `naturalist`.

**Step 2: Verify**

```bash
grep -h "^archetype:" content/jobs/hippie.yaml content/jobs/freegan.yaml \
  content/jobs/tracker.yaml content/jobs/rancher.yaml content/jobs/hobo.yaml \
  content/jobs/laborer.yaml content/jobs/fallen_trustafarian.yaml
```

Expected: all read `archetype: naturalist`.

**Step 3: Run tests**

```bash
go test ./internal/game/ruleset/... -run "TestAllArchetypesHaveJobsForBothTeams" -v
```

Naturalist should now pass.

**Step 4: Commit**

```bash
git add content/jobs/hippie.yaml content/jobs/freegan.yaml content/jobs/tracker.yaml \
  content/jobs/rancher.yaml content/jobs/hobo.yaml content/jobs/laborer.yaml \
  content/jobs/fallen_trustafarian.yaml
git commit -m "feat: reassign 7 jobs to Naturalist archetype"
```

---

### Task 5: Reassign jobs to Schemer

**Files to modify:**
- `content/jobs/narcomancer.yaml` — `nerd` → `schemer`
- `content/jobs/illusionist.yaml` — `drifter` → `schemer`
- `content/jobs/grifter.yaml` — `criminal` → `schemer`
- `content/jobs/dealer.yaml` — `drifter` → `schemer`
- `content/jobs/mall_ninja.yaml` — `aggressor` → `schemer`
- `content/jobs/shit_stirrer.yaml` — `influencer` → `schemer`

**Step 1: Update each file**

Change `archetype:` to `schemer` in each file.

**Step 2: Verify**

```bash
grep -h "^archetype:" content/jobs/narcomancer.yaml content/jobs/illusionist.yaml \
  content/jobs/grifter.yaml content/jobs/dealer.yaml content/jobs/mall_ninja.yaml \
  content/jobs/shit_stirrer.yaml
```

Expected: all read `archetype: schemer`.

**Step 3: Run tests**

```bash
go test ./internal/game/ruleset/... -run "TestAllArchetypesHaveJobsForBothTeams" -v
```

Schemer should now pass.

**Step 4: Commit**

```bash
git add content/jobs/narcomancer.yaml content/jobs/illusionist.yaml content/jobs/grifter.yaml \
  content/jobs/dealer.yaml content/jobs/mall_ninja.yaml content/jobs/shit_stirrer.yaml
git commit -m "feat: reassign 6 jobs to Schemer archetype"
```

---

### Task 6: Redistribute Aggressor and Influencer overflow jobs

**Files to modify:**
- `content/jobs/goon.yaml` — `influencer` → `aggressor`
- `content/jobs/muscle.yaml` — `criminal` → `aggressor`
- `content/jobs/pirate.yaml` — `aggressor` → `drifter`
- `content/jobs/free_spirit.yaml` — `criminal` → `drifter`
- `content/jobs/contract_killer.yaml` — `aggressor` → `criminal`
- `content/jobs/specialist.yaml` — `aggressor` → `nerd`

**Step 1: Update each file**

Change `archetype:` in each file to the new value shown above.

**Step 2: Verify counts**

```bash
grep -h "^archetype:" content/jobs/*.yaml | sort | uniq -c | sort -rn
```

Expected approximate counts: Aggressor ~12, Influencer ~10, Criminal ~9, Nerd ~9, Drifter ~10, Zealot ~10, Naturalist ~8, Schemer ~8.

**Step 3: Run full ruleset test suite**

```bash
go test ./internal/game/ruleset/... -v 2>&1 | tail -30
```

Expected: all tests PASS.

**Step 4: Commit**

```bash
git add content/jobs/goon.yaml content/jobs/muscle.yaml content/jobs/pirate.yaml \
  content/jobs/free_spirit.yaml content/jobs/contract_killer.yaml content/jobs/specialist.yaml
git commit -m "feat: balance Aggressor and Influencer by redistributing 6 jobs"
```

---

### Task 7: DB migration — clear stale archetype boost choices

**Files:**
- Create: `migrations/017_clear_reassigned_archetype_boosts.up.sql`
- Create: `migrations/017_clear_reassigned_archetype_boosts.down.sql`

**Step 1: Create up migration**

`migrations/017_clear_reassigned_archetype_boosts.up.sql`:

```sql
-- Clear archetype ability boost choices for characters whose job moved to a
-- different archetype. These choices were made under the old archetype's boost
-- pool and are now invalid. Players will be re-prompted at next login.
DELETE FROM character_ability_boosts
WHERE source = 'archetype'
AND character_id IN (
    SELECT id FROM characters
    WHERE class IN (
        'pastor', 'believer', 'street_preacher', 'trainee', 'guard', 'follower', 'vigilante',
        'hippie', 'freegan', 'tracker', 'rancher', 'hobo', 'laborer', 'fallen_trustafarian',
        'narcomancer', 'illusionist', 'grifter', 'dealer', 'mall_ninja', 'shit_stirrer',
        'goon', 'muscle', 'pirate', 'free_spirit', 'contract_killer', 'specialist',
        'cult_leader', 'hired_help', 'medic', 'maker', 'salesman', 'exterminator',
        'driver', 'pilot', 'hanger_on'
    )
);
```

**Step 2: Create down migration**

`migrations/017_clear_reassigned_archetype_boosts.down.sql`:

```sql
-- No-op: deleted ability boost choices cannot be restored.
-- Players whose archetype boost choices were cleared will simply be re-prompted at login.
SELECT 1;
```

**Step 3: Run migration**

```bash
make migrate CONFIG=configs/prod.yaml
```

Expected output: `migrated up to version=17 dirty=false [...]`

**Step 4: Verify**

```bash
make migrate CONFIG=configs/prod.yaml
```

Expected: `no changes (version=17 dirty=false [...])`  — idempotent.

**Step 5: Commit**

```bash
git add migrations/017_clear_reassigned_archetype_boosts.up.sql \
        migrations/017_clear_reassigned_archetype_boosts.down.sql
git commit -m "feat: migration 017 — clear stale archetype boost choices for reassigned jobs"
```

---

### Task 8: Update tests and deploy

**Files:**
- Modify: `internal/game/ruleset/loader_test.go` — update archetype count assertions if any hardcode 6
- Modify: `docs/requirements/FEATURES.md` — mark feature done

**Step 1: Check for hardcoded archetype counts in tests**

```bash
grep -rn "normie\|Normie\|6.*archetype\|archetype.*6" internal/ --include="*.go" | grep -v "_test.go:"
grep -rn "normie\|Normie" internal/ --include="*.go"
```

Fix any test that hardcodes the count of archetypes (was 6, now 8) or references `normie`.

**Step 2: Run the full fast test suite**

```bash
go test -race -count=1 -timeout=300s $(go list ./... | grep -v 'github.com/cory-johannsen/mud/internal/storage/postgres') 2>&1 | tail -20
```

Expected: all PASS (pre-existing race in `TestSession_FavoredTargetPromptedWhenMissing_InvalidInput` is known and unrelated).

**Step 3: Mark FEATURES.md done**

In `docs/requirements/FEATURES.md`, change:
```
- [ ] Expanded Archetype options to match PF2E
  - Full list of Player Core classes that need supported:
    - [ ] Bard -> Influencer
    ...
  - Normie doesn't match up with a PF2E Core class and should be reworked
  - Many existing jobs should be reassigned to the new Archetype that they most closely match
    - Each archetype should have the same number of Jobs to select from (or as close as possible)
```
to:
```
- [x] Expanded Archetype options to match PF2E
  - Full list of Player Core classes that need supported:
    - [x] Bard -> Influencer
    - [x] Cleric -> Zealot
    - [x] Druid -> Naturalist
    - [x] Fighter -> Aggressor
    - [x] Ranger -> Drifter
    - [x] Rogue -> Criminal
    - [x] Witch -> Schemer
    - [x] Wizard -> Nerd
  - [x] Normie dissolved; jobs redistributed
  - [x] Jobs redistributed for balance (~8-12 per archetype)
```

**Step 4: Deploy**

```bash
make k8s-redeploy
```

Expected: `Release "mud" has been upgraded. Happy Helming!`

**Step 5: Commit and push**

```bash
git add docs/requirements/FEATURES.md internal/
git commit -m "docs: mark archetype expansion as complete"
git push
```
