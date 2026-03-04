# Skills Stage 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the skills system (data model, DB, assignment, `skills` command) without feats or abilities.

**Architecture:** Single `content/skills.yaml` defines all 17 Gunchete skills. Each job YAML gets a `skills` block with fixed and choice entries. A new `character_skills` DB table stores per-character proficiency ranks. The `skills` command follows the full CMD-1 through CMD-7 pipeline.

**Tech Stack:** Go, PostgreSQL (pgx/v5), protobuf, YAML, Lua (none needed here)

**Design doc:** `docs/plans/2026-03-03-skills-stage1-design.md`

---

## Task 1: Skill content file

**Files:**
- Create: `content/skills.yaml`

**Step 1: Create the file**

```yaml
skills:
  - id: parkour
    name: Parkour
    ability: quickness
    pf2e: acrobatics
    description: "Movement through ruins, vaults, chases, and tight spaces."
  - id: ghosting
    name: Ghosting
    ability: quickness
    pf2e: stealth
    description: "Staying unseen and unheard in hostile territory."
  - id: grift
    name: Grift
    ability: quickness
    pf2e: thievery
    description: "Pickpocketing, lockpicking, and sleight of hand."
  - id: muscle
    name: Muscle
    ability: brutality
    pf2e: athletics
    description: "Climbing, swimming, lifting, and breaking things."
  - id: tech_lore
    name: Tech Lore
    ability: reasoning
    pf2e: arcana
    description: "Electronics, hacking, and technical knowledge."
  - id: rigging
    name: Rigging
    ability: reasoning
    pf2e: crafting
    description: "Building and fixing weapons and gear from scrap."
  - id: conspiracy
    name: Conspiracy
    ability: reasoning
    pf2e: occultism
    description: "Cults, underground networks, and secret knowledge."
  - id: factions
    name: Factions
    ability: reasoning
    pf2e: society
    description: "Gang hierarchies, political power structures."
  - id: intel
    name: Intel
    ability: reasoning
    pf2e: lore
    description: "Specific knowledge: Gang Intel, Territory Intel, etc."
  - id: patch_job
    name: Patch Job
    ability: savvy
    pf2e: medicine
    description: "Field medicine, trauma care, and stimulant use."
  - id: wasteland
    name: Wasteland
    ability: savvy
    pf2e: nature
    description: "Navigating ruins, reading weather, urban terrain."
  - id: gang_codes
    name: Gang Codes
    ability: savvy
    pf2e: religion
    description: "Crew rituals, street sign languages, and loyalties."
  - id: scavenging
    name: Scavenging
    ability: savvy
    pf2e: survival
    description: "Finding food, water, shelter, and usable junk."
  - id: hustle
    name: Hustle
    ability: flair
    pf2e: deception
    description: "Lies, cons, and disguises."
  - id: smooth_talk
    name: Smooth Talk
    ability: flair
    pf2e: diplomacy
    description: "Negotiation, de-escalation, and calling in favors."
  - id: hard_look
    name: Hard Look
    ability: flair
    pf2e: intimidation
    description: "Fear, coercion, and dominance displays."
  - id: rep
    name: Rep
    ability: flair
    pf2e: performance
    description: "Street reputation, showmanship, and crowd work."
```

**Step 2: Commit**

```bash
git add content/skills.yaml
git commit -m "content: add 17 Gunchete skill definitions"
```

---

## Task 2: Skill ruleset type and loader

**Files:**
- Create: `internal/game/ruleset/skill.go`
- Create: `internal/game/ruleset/skill_test.go`

**Step 1: Write the failing test**

```go
// internal/game/ruleset/skill_test.go
package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadSkills_LoadsAll17(t *testing.T) {
	skills, err := ruleset.LoadSkills("../../../content/skills.yaml")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 17 {
		t.Fatalf("expected 17 skills, got %d", len(skills))
	}
}

func TestLoadSkills_FieldsPopulated(t *testing.T) {
	skills, err := ruleset.LoadSkills("../../../content/skills.yaml")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	byID := make(map[string]*ruleset.Skill, len(skills))
	for _, s := range skills {
		byID[s.ID] = s
	}
	parkour, ok := byID["parkour"]
	if !ok {
		t.Fatal("parkour skill not found")
	}
	if parkour.Name != "Parkour" {
		t.Errorf("expected Name=Parkour, got %q", parkour.Name)
	}
	if parkour.Ability != "quickness" {
		t.Errorf("expected Ability=quickness, got %q", parkour.Ability)
	}
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/ruleset/ -run TestLoadSkills -v
```
Expected: FAIL — `ruleset.LoadSkills undefined`

**Step 3: Implement**

```go
// internal/game/ruleset/skill.go
package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Skill defines one Gunchete skill and its P2FE equivalent.
type Skill struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Ability     string `yaml:"ability"`
	PF2E        string `yaml:"pf2e"`
	Description string `yaml:"description"`
}

// skillsFile is the top-level YAML structure for content/skills.yaml.
type skillsFile struct {
	Skills []*Skill `yaml:"skills"`
}

// LoadSkills reads the skills master YAML file and returns all skill definitions.
//
// Precondition: path must be a readable file containing valid YAML.
// Postcondition: Returns all skills or a non-nil error.
func LoadSkills(path string) ([]*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skills file %s: %w", path, err)
	}
	var f skillsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing skills file %s: %w", path, err)
	}
	return f.Skills, nil
}
```

**Step 4: Run to verify pass**

```bash
go test ./internal/game/ruleset/ -run TestLoadSkills -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/game/ruleset/skill.go internal/game/ruleset/skill_test.go
git commit -m "feat: add Skill type and LoadSkills loader"
```

---

## Task 3: Extend Job struct with SkillGrants

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/job_registry_test.go` (add skill grant test)

**Step 1: Write the failing test**

Add to `internal/game/ruleset/job_registry_test.go`:

```go
func TestJob_SkillGrantsLoaded(t *testing.T) {
	reg, err := ruleset.NewJobRegistry("../../../content/jobs", "../../../content/archetypes")
	if err != nil {
		t.Fatalf("NewJobRegistry: %v", err)
	}
	// anarchist.yaml will have skills.fixed populated after Task 4
	job, ok := reg.ByID("anarchist")
	if !ok {
		t.Fatal("anarchist job not found")
	}
	if job.SkillGrants == nil {
		t.Fatal("SkillGrants must not be nil after loading")
	}
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/ruleset/ -run TestJob_SkillGrantsLoaded -v
```
Expected: FAIL — `job.SkillGrants undefined`

**Step 3: Extend Job struct**

Add to `internal/game/ruleset/job.go`:

```go
// SkillChoices defines a pool of skills the player picks from at creation.
type SkillChoices struct {
	Pool  []string `yaml:"pool"`
	Count int      `yaml:"count"`
}

// SkillGrants defines the skill proficiencies a job grants at creation.
type SkillGrants struct {
	Fixed   []string     `yaml:"fixed"`
	Choices *SkillChoices `yaml:"choices"`
}
```

Add the field to the `Job` struct (after `Proficiencies`):

```go
SkillGrants *SkillGrants `yaml:"skills"`
```

**Step 4: Run to verify pass**

```bash
go test ./internal/game/ruleset/ -run TestJob_SkillGrantsLoaded -v
```
Expected: PASS (SkillGrants will be nil until Task 4 adds YAML, but field exists)

**Step 5: Commit**

```bash
git add internal/game/ruleset/job.go internal/game/ruleset/job_registry_test.go
git commit -m "feat: add SkillGrants to Job struct"
```

---

## Task 4: Add skill grants to all 76 job YAMLs

**Files:**
- Modify: `content/jobs/*.yaml` (all 76)

Use the following skill assignments as the canonical mapping. Each job gets skills that fit its archetype and role. The pattern is: 2 fixed skills + choice pool of 4–6 with count 2 (adjust per job).

Run this script to apply the bulk update:

```bash
cat > /tmp/add_skills.py << 'EOF'
import os, re

# Skill grant assignments per job ID
# Format: (fixed_list, pool_list, count)
GRANTS = {
    "anarchist":        (["hard_look", "smooth_talk"], ["hustle", "rep", "factions", "conspiracy"], 2),
    "antifa":           (["hard_look", "muscle"], ["hustle", "factions", "ghosting", "conspiracy"], 2),
    "bagman":           (["hustle", "factions"], ["grift", "smooth_talk", "ghosting", "intel"], 2),
    "beat_down_artist": (["muscle", "hard_look"], ["parkour", "ghosting", "rep", "scavenging"], 2),
    "beggar":           (["hustle", "scavenging"], ["smooth_talk", "ghosting", "wasteland", "intel"], 2),
    "believer":         (["gang_codes", "smooth_talk"], ["factions", "hard_look", "rep", "conspiracy"], 2),
    "boot_gun":         (["muscle", "hard_look"], ["parkour", "ghosting", "grift", "scavenging"], 2),
    "boot_machete":     (["muscle", "hard_look"], ["parkour", "ghosting", "grift", "scavenging"], 2),
    "bureaucrat":       (["factions", "smooth_talk"], ["intel", "hustle", "conspiracy", "rep"], 2),
    "car_jacker":       (["grift", "parkour"], ["ghosting", "muscle", "hustle", "tech_lore"], 2),
    "contract_killer":  (["ghosting", "hard_look"], ["muscle", "parkour", "grift", "wasteland"], 2),
    "cooker":           (["rigging", "tech_lore"], ["scavenging", "patch_job", "conspiracy", "hustle"], 2),
    "cop":              (["hard_look", "factions"], ["smooth_talk", "intel", "muscle", "ghosting"], 2),
    "cult_leader":      (["rep", "gang_codes"], ["smooth_talk", "hard_look", "conspiracy", "factions"], 2),
    "dealer":           (["hustle", "factions"], ["grift", "smooth_talk", "ghosting", "intel"], 2),
    "detective":        (["intel", "smooth_talk"], ["factions", "hustle", "ghosting", "tech_lore"], 2),
    "driver":           (["parkour", "tech_lore"], ["muscle", "scavenging", "ghosting", "rigging"], 2),
    "engineer":         (["tech_lore", "rigging"], ["scavenging", "patch_job", "intel", "wasteland"], 2),
    "entertainer":      (["rep", "smooth_talk"], ["hustle", "factions", "hard_look", "gang_codes"], 2),
    "exotic_dancer":    (["rep", "hustle"], ["smooth_talk", "hard_look", "grift", "parkour"], 2),
    "fixer":            (["factions", "intel"], ["hustle", "smooth_talk", "grift", "conspiracy"], 2),
    "gangbanger":       (["hard_look", "muscle"], ["grift", "parkour", "ghosting", "gang_codes"], 2),
    "getaway_driver":   (["parkour", "tech_lore"], ["ghosting", "muscle", "scavenging", "rigging"], 2),
    "grease_monkey":    (["rigging", "tech_lore"], ["scavenging", "muscle", "patch_job", "wasteland"], 2),
    "grifter":          (["hustle", "grift"], ["smooth_talk", "ghosting", "factions", "rep"], 2),
    "gun_dealer":       (["factions", "intel"], ["hustle", "tech_lore", "rigging", "smooth_talk"], 2),
    "gunfighter":       (["hard_look", "muscle"], ["parkour", "ghosting", "grift", "wasteland"], 2),
    "hacker":           (["tech_lore", "rigging"], ["intel", "conspiracy", "ghosting", "factions"], 2),
    "hitman":           (["ghosting", "hard_look"], ["muscle", "parkour", "grift", "wasteland"], 2),
    "homeless":         (["scavenging", "wasteland"], ["hustle", "ghosting", "smooth_talk", "gang_codes"], 2),
    "hustler":          (["hustle", "grift"], ["smooth_talk", "ghosting", "rep", "factions"], 2),
    "informant":        (["intel", "hustle"], ["factions", "ghosting", "smooth_talk", "conspiracy"], 2),
    "influencer":       (["rep", "smooth_talk"], ["hustle", "factions", "hard_look", "gang_codes"], 2),
    "jailbird":         (["muscle", "gang_codes"], ["hard_look", "grift", "ghosting", "hustle"], 2),
    "journalist":       (["intel", "smooth_talk"], ["factions", "hustle", "conspiracy", "rep"], 2),
    "junker":           (["scavenging", "rigging"], ["tech_lore", "wasteland", "muscle", "patch_job"], 2),
    "killer":           (["ghosting", "hard_look"], ["muscle", "parkour", "grift", "wasteland"], 2),
    "loan_shark":       (["hard_look", "factions"], ["hustle", "smooth_talk", "intel", "grift"], 2),
    "lookout":          (["ghosting", "intel"], ["parkour", "wasteland", "factions", "gang_codes"], 2),
    "machete_fighter":  (["muscle", "hard_look"], ["parkour", "ghosting", "grift", "wasteland"], 2),
    "mechanic":         (["rigging", "tech_lore"], ["scavenging", "muscle", "patch_job", "wasteland"], 2),
    "medic":            (["patch_job", "scavenging"], ["tech_lore", "rigging", "smooth_talk", "wasteland"], 2),
    "mercenary":        (["muscle", "hard_look"], ["parkour", "ghosting", "wasteland", "scavenging"], 2),
    "mule":             (["muscle", "scavenging"], ["wasteland", "parkour", "ghosting", "gang_codes"], 2),
    "operative":        (["ghosting", "intel"], ["grift", "parkour", "tech_lore", "factions"], 2),
    "organizer":        (["factions", "smooth_talk"], ["intel", "rep", "hustle", "gang_codes"], 2),
    "outlaw":           (["grift", "ghosting"], ["parkour", "muscle", "wasteland", "hustle"], 2),
    "pickpocket":       (["grift", "ghosting"], ["parkour", "hustle", "smooth_talk", "rep"], 2),
    "pit_fighter":      (["muscle", "hard_look"], ["parkour", "rep", "ghosting", "gang_codes"], 2),
    "politico":         (["factions", "smooth_talk"], ["hustle", "rep", "intel", "conspiracy"], 2),
    "preacher":         (["rep", "smooth_talk"], ["gang_codes", "hard_look", "factions", "conspiracy"], 2),
    "press_ganger":     (["hard_look", "factions"], ["muscle", "smooth_talk", "hustle", "gang_codes"], 2),
    "privateer":        (["muscle", "parkour"], ["ghosting", "wasteland", "scavenging", "grift"], 2),
    "professor":        (["intel", "tech_lore"], ["factions", "smooth_talk", "rigging", "conspiracy"], 2),
    "propagandist":     (["rep", "hustle"], ["smooth_talk", "factions", "conspiracy", "hard_look"], 2),
    "punk":             (["hard_look", "muscle"], ["parkour", "grift", "ghosting", "rep"], 2),
    "pusher":           (["hustle", "factions"], ["grift", "smooth_talk", "ghosting", "gang_codes"], 2),
    "raider":           (["muscle", "hard_look"], ["parkour", "ghosting", "scavenging", "wasteland"], 2),
    "ratcatcher":       (["scavenging", "wasteland"], ["ghosting", "muscle", "grift", "patch_job"], 2),
    "runner":           (["parkour", "ghosting"], ["grift", "hustle", "wasteland", "scavenging"], 2),
    "scavenger":        (["scavenging", "wasteland"], ["rigging", "muscle", "ghosting", "patch_job"], 2),
    "scrapper":         (["muscle", "rigging"], ["scavenging", "wasteland", "hard_look", "tech_lore"], 2),
    "screamer":         (["rep", "hard_look"], ["hustle", "smooth_talk", "gang_codes", "factions"], 2),
    "sharpshooter":     (["ghosting", "wasteland"], ["muscle", "parkour", "scavenging", "intel"], 2),
    "skimmer":          (["grift", "hustle"], ["ghosting", "parkour", "smooth_talk", "factions"], 2),
    "smuggler":         (["grift", "ghosting"], ["parkour", "hustle", "factions", "wasteland"], 2),
    "snitch":           (["intel", "hustle"], ["smooth_talk", "factions", "ghosting", "conspiracy"], 2),
    "soldier":          (["muscle", "hard_look"], ["parkour", "ghosting", "wasteland", "scavenging"], 2),
    "spy":              (["ghosting", "intel"], ["grift", "hustle", "tech_lore", "factions"], 2),
    "street_doc":       (["patch_job", "scavenging"], ["tech_lore", "rigging", "smooth_talk", "intel"], 2),
    "street_rat":       (["grift", "ghosting"], ["parkour", "hustle", "scavenging", "wasteland"], 2),
    "tagger":           (["rep", "parkour"], ["ghosting", "grift", "hustle", "factions"], 2),
    "thug":             (["muscle", "hard_look"], ["parkour", "grift", "ghosting", "gang_codes"], 2),
    "vigilante":        (["hard_look", "intel"], ["muscle", "parkour", "factions", "ghosting"], 2),
    "warlord":          (["hard_look", "factions"], ["muscle", "gang_codes", "rep", "smooth_talk"], 2),
    "waste_picker":     (["scavenging", "wasteland"], ["rigging", "ghosting", "muscle", "patch_job"], 2),
    "wrench":           (["rigging", "tech_lore"], ["scavenging", "muscle", "patch_job", "wasteland"], 2),
}

jobs_dir = "content/jobs"
for fname in sorted(os.listdir(jobs_dir)):
    if not fname.endswith(".yaml"):
        continue
    job_id = fname[:-5]
    path = os.path.join(jobs_dir, fname)
    with open(path) as f:
        content = f.read()
    if "skills:" in content:
        print(f"  SKIP {job_id} (already has skills:)")
        continue
    grant = GRANTS.get(job_id)
    if not grant:
        print(f"  WARN {job_id} not in GRANTS map — skipping")
        continue
    fixed, pool, count = grant
    fixed_yaml = "\n".join(f"    - {s}" for s in fixed)
    pool_yaml  = "\n".join(f"      - {s}" for s in pool)
    block = f"""skills:
  fixed:
{fixed_yaml}
  choices:
    count: {count}
    pool:
{pool_yaml}
"""
    with open(path, "a") as f:
        f.write(block)
    print(f"  OK  {job_id}")

print("Done.")
EOF
python3 /tmp/add_skills.py
```

**Step 2: Verify a sample file**

```bash
tail -15 content/jobs/anarchist.yaml
```

Expected: `skills:` block with `fixed:` and `choices:` entries.

**Step 3: Run existing tests**

```bash
go test ./internal/game/ruleset/ -v
```
Expected: all pass (no code changed, only YAML content)

**Step 4: Commit**

```bash
git add content/jobs/
git commit -m "content: add skill grants to all 76 job definitions"
```

---

## Task 5: DB migration for character_skills

**Files:**
- Create: `migrations/011_character_skills.up.sql`
- Create: `migrations/011_character_skills.down.sql`

**Step 1: Write migration files**

`migrations/011_character_skills.up.sql`:
```sql
CREATE TABLE character_skills (
    character_id BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id     TEXT    NOT NULL,
    proficiency  TEXT    NOT NULL DEFAULT 'untrained',
    PRIMARY KEY (character_id, skill_id)
);
```

`migrations/011_character_skills.down.sql`:
```sql
DROP TABLE IF EXISTS character_skills;
```

**Step 2: Commit**

```bash
git add migrations/011_character_skills.up.sql migrations/011_character_skills.down.sql
git commit -m "feat: add character_skills migration"
```

---

## Task 6: Storage layer for character skills

**Files:**
- Create: `internal/storage/postgres/character_skills.go`
- Create: `internal/storage/postgres/character_skills_test.go`

**Step 1: Write the failing test**

```go
// internal/storage/postgres/character_skills_test.go
package postgres_test

import (
	"context"
	"testing"
)

func TestCharacterSkillsRepository_SetAndGet(t *testing.T) {
	// Uses the existing test DB setup pattern from character_test.go
	ctx := context.Background()
	db := testDB(t)
	repo := NewCharacterSkillsRepository(db)
	charRepo := NewCharacterRepository(db)

	ch := createTestCharacter(t, charRepo, ctx)

	skills := map[string]string{
		"parkour":  "trained",
		"ghosting": "untrained",
		"muscle":   "trained",
	}
	if err := repo.SetAll(ctx, ch.ID, skills); err != nil {
		t.Fatalf("SetAll: %v", err)
	}

	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != len(skills) {
		t.Fatalf("expected %d skills, got %d", len(skills), len(got))
	}
	if got["parkour"] != "trained" {
		t.Errorf("expected parkour=trained, got %q", got["parkour"])
	}
}

func TestCharacterSkillsRepository_HasSkills(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewCharacterSkillsRepository(db)
	charRepo := NewCharacterRepository(db)

	ch := createTestCharacter(t, charRepo, ctx)

	has, err := repo.HasSkills(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasSkills: %v", err)
	}
	if has {
		t.Error("expected HasSkills=false for new character")
	}

	if err := repo.SetAll(ctx, ch.ID, map[string]string{"parkour": "untrained"}); err != nil {
		t.Fatalf("SetAll: %v", err)
	}
	has, err = repo.HasSkills(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasSkills: %v", err)
	}
	if !has {
		t.Error("expected HasSkills=true after SetAll")
	}
}
```

Look at `internal/storage/postgres/character_test.go` to understand the `testDB` and `createTestCharacter` helpers and replicate the same pattern.

**Step 2: Run to verify failure**

```bash
go test ./internal/storage/postgres/ -run TestCharacterSkills -v
```
Expected: FAIL — `NewCharacterSkillsRepository undefined`

**Step 3: Implement**

```go
// internal/storage/postgres/character_skills.go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterSkillsRepository persists per-character skill proficiency ranks.
type CharacterSkillsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterSkillsRepository creates a repository backed by the given pool.
func NewCharacterSkillsRepository(db *pgxpool.Pool) *CharacterSkillsRepository {
	return &CharacterSkillsRepository{db: db}
}

// HasSkills reports whether the character has any rows in character_skills.
//
// Precondition: characterID > 0.
// Postcondition: Returns true if at least one skill row exists.
func (r *CharacterSkillsRepository) HasSkills(ctx context.Context, characterID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM character_skills WHERE character_id = $1`, characterID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("HasSkills: %w", err)
	}
	return count > 0, nil
}

// GetAll returns all skill proficiency ranks for a character.
//
// Precondition: characterID > 0.
// Postcondition: Returns a map of skill_id → proficiency (may be empty).
func (r *CharacterSkillsRepository) GetAll(ctx context.Context, characterID int64) (map[string]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT skill_id, proficiency FROM character_skills WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAll skills: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var id, prof string
		if err := rows.Scan(&id, &prof); err != nil {
			return nil, fmt.Errorf("scanning skill row: %w", err)
		}
		out[id] = prof
	}
	return out, rows.Err()
}

// SetAll writes the complete skill map for a character, replacing any existing rows.
//
// Precondition: characterID > 0; skills must not be nil.
// Postcondition: character_skills rows match skills exactly.
func (r *CharacterSkillsRepository) SetAll(ctx context.Context, characterID int64, skills map[string]string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM character_skills WHERE character_id = $1`, characterID,
	); err != nil {
		return fmt.Errorf("deleting old skills: %w", err)
	}

	for skillID, prof := range skills {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_skills (character_id, skill_id, proficiency) VALUES ($1, $2, $3)`,
			characterID, skillID, prof,
		); err != nil {
			return fmt.Errorf("inserting skill %s: %w", skillID, err)
		}
	}
	return tx.Commit(ctx)
}
```

**Step 4: Run to verify pass**

```bash
go test ./internal/storage/postgres/ -run TestCharacterSkills -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/postgres/character_skills.go internal/storage/postgres/character_skills_test.go
git commit -m "feat: add CharacterSkillsRepository (SetAll, GetAll, HasSkills)"
```

---

## Task 7: Extend Character model and builder

**Files:**
- Modify: `internal/game/character/model.go`
- Modify: `internal/game/character/builder.go`
- Modify: `internal/game/character/builder_test.go` (or create if needed)

**Step 1: Add Skills field to Character**

In `internal/game/character/model.go`, add to the `Character` struct after `CurrentHP`:

```go
// Skills maps skill_id to proficiency rank for this character.
// Populated after loading from the DB or after creation.
Skills map[string]string
```

**Step 2: Write failing builder test**

```go
// In internal/game/character/builder_test.go (add to existing file)
func TestBuildSkillsFromJob_Fixed(t *testing.T) {
	skills := character.BuildSkillsFromJob(&ruleset.Job{
		SkillGrants: &ruleset.SkillGrants{
			Fixed: []string{"muscle", "patch_job"},
		},
	}, []string{"parkour", "gangcodes"})

	// 17 total skills
	if len(skills) != 17 {
		t.Fatalf("expected 17 skills, got %d", len(skills))
	}
	if skills["muscle"] != "trained" {
		t.Errorf("expected muscle=trained, got %q", skills["muscle"])
	}
	if skills["parkour"] != "untrained" {
		t.Errorf("expected parkour=untrained, got %q", skills["parkour"])
	}
}

func TestBuildSkillsFromJob_Choices(t *testing.T) {
	skills := character.BuildSkillsFromJob(&ruleset.Job{
		SkillGrants: &ruleset.SkillGrants{
			Fixed: []string{"muscle"},
			Choices: &ruleset.SkillChoices{
				Pool:  []string{"parkour", "ghosting", "grift"},
				Count: 2,
			},
		},
	}, []string{"parkour", "ghosting"})

	if skills["parkour"] != "trained" {
		t.Errorf("expected parkour=trained")
	}
	if skills["ghosting"] != "trained" {
		t.Errorf("expected ghosting=trained")
	}
	if skills["grift"] != "untrained" {
		t.Errorf("expected grift=untrained, got %q", skills["grift"])
	}
}
```

The `BuildSkillsFromJob` function takes a `*ruleset.Job` and a `[]string` of player-chosen skill IDs. It must know all 17 skill IDs. Pass them as a parameter or load from the registry — accept a `[]string` of all skill IDs as a third parameter.

Update the signature: `BuildSkillsFromJob(job *ruleset.Job, allSkillIDs []string, chosen []string) map[string]string`

**Step 3: Run to verify failure**

```bash
go test ./internal/game/character/ -run TestBuildSkills -v
```
Expected: FAIL

**Step 4: Implement in builder.go**

Add to `internal/game/character/builder.go`:

```go
// BuildSkillsFromJob constructs a full skill map for a new character.
// Fixed skills from the job are set to "trained". Chosen skills (player-selected
// from the choices pool) are set to "trained". All others are "untrained".
//
// Precondition: allSkillIDs contains all 17 skill IDs; chosen is a subset of job.SkillGrants.Choices.Pool.
// Postcondition: Returns a map with exactly len(allSkillIDs) entries.
func BuildSkillsFromJob(job *ruleset.Job, allSkillIDs []string, chosen []string) map[string]string {
	trained := make(map[string]bool)

	if job.SkillGrants != nil {
		for _, id := range job.SkillGrants.Fixed {
			trained[id] = true
		}
	}
	for _, id := range chosen {
		trained[id] = true
	}

	out := make(map[string]string, len(allSkillIDs))
	for _, id := range allSkillIDs {
		if trained[id] {
			out[id] = "trained"
		} else {
			out[id] = "untrained"
		}
	}
	return out
}
```

**Step 5: Run to verify pass**

```bash
go test ./internal/game/character/ -run TestBuildSkills -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/game/character/model.go internal/game/character/builder.go internal/game/character/builder_test.go
git commit -m "feat: add Skills field to Character, BuildSkillsFromJob helper"
```

---

## Task 8: Skill selection in character creation and backfill

This task wires skill selection into the game server's character creation flow and session startup backfill. The entry point is `internal/gameserver/grpc_service.go`.

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

**Step 1: Locate character creation finalization**

Search for where `BuildWithJob` is called and a character is saved. This is the point to add skill assignment.

```bash
grep -n "BuildWithJob\|character\.Create\|charRepo" internal/gameserver/grpc_service.go | head -20
```

**Step 2: Inject dependencies**

The `GameServiceServer` struct needs access to:
- `*postgres.CharacterSkillsRepository`
- `[]*ruleset.Skill` (all 17 skill definitions)
- `*ruleset.JobRegistry` (already present — check with `grep JobRegistry internal/gameserver/grpc_service.go`)

Add `CharacterSkillsRepo *postgres.CharacterSkillsRepository` and `AllSkills []*ruleset.Skill` to the server struct and its constructor if not already present. Wire them from `main.go`/`cmd/gameserver/`.

**Step 3: Add skill choice prompt helper**

The creation flow uses a prompt/response loop. Add a helper function to manage the sequential choice loop:

```go
// skillChoiceLoop sends sequential prompts to collect skill choices from the player.
// Returns the chosen skill IDs, or an error if the stream closes.
func skillChoiceLoop(stream grpc.BidiStreamingServer[gamev1.ClientMessage, gamev1.ServerEvent],
    pool []string, count int, reqID string) ([]string, error) {

    chosen := []string{}
    remaining := make([]string, len(pool))
    copy(remaining, pool)

    for len(chosen) < count {
        // Build prompt text
        lines := []string{fmt.Sprintf("Choose %d additional skill(s) (%d remaining):", count, count-len(chosen))}
        for i, id := range remaining {
            lines = append(lines, fmt.Sprintf("  %d. %s", i+1, id))
        }
        prompt := strings.Join(lines, "\r\n")

        // Send prompt event
        if err := stream.Send(&gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Message{
                Message: &gamev1.MessageEvent{Content: prompt},
            },
        }); err != nil {
            return nil, err
        }

        // Wait for input
        msg, err := stream.Recv()
        if err != nil {
            return nil, err
        }
        // Parse number
        raw := strings.TrimSpace(msg.GetPayload()...)  // extract text from appropriate message type
        n, err := strconv.Atoi(raw)
        if err != nil || n < 1 || n > len(remaining) {
            // Re-prompt on invalid input (loop continues)
            continue
        }
        chosen = append(chosen, remaining[n-1])
        remaining = append(remaining[:n-1], remaining[n:]...)
    }
    return chosen, nil
}
```

Adapt the stream/message extraction to match the actual message structure in this codebase. Look at how existing creation prompts (region, job selection) receive player input — the pattern is already established.

**Step 4: Wire into creation finalization**

After `BuildWithJob` succeeds and the character is saved:

```go
// Determine skill choices
var chosen []string
if job.SkillGrants != nil && job.SkillGrants.Choices != nil && job.SkillGrants.Choices.Count > 0 {
    chosen, err = skillChoiceLoop(stream, job.SkillGrants.Choices.Pool, job.SkillGrants.Choices.Count, reqID)
    if err != nil {
        return err
    }
}

allSkillIDs := make([]string, len(s.AllSkills))
for i, sk := range s.AllSkills {
    allSkillIDs[i] = sk.ID
}
skillMap := character.BuildSkillsFromJob(job, allSkillIDs, chosen)
if err := s.CharacterSkillsRepo.SetAll(ctx, savedChar.ID, skillMap); err != nil {
    return fmt.Errorf("saving skills: %w", err)
}
```

**Step 5: Wire backfill into session start**

Find where a player enters the world (after character selection, before the first look command). Add:

```go
has, err := s.CharacterSkillsRepo.HasSkills(ctx, char.ID)
if err != nil {
    return fmt.Errorf("checking skills: %w", err)
}
if !has {
    job, ok := s.JobRegistry.ByID(char.Class)
    if !ok {
        return fmt.Errorf("job not found: %s", char.Class)
    }
    var chosen []string
    if job.SkillGrants != nil && job.SkillGrants.Choices != nil && job.SkillGrants.Choices.Count > 0 {
        chosen, err = skillChoiceLoop(stream, job.SkillGrants.Choices.Pool, job.SkillGrants.Choices.Count, reqID)
        if err != nil {
            return err
        }
    }
    allSkillIDs := make([]string, len(s.AllSkills))
    for i, sk := range s.AllSkills {
        allSkillIDs[i] = sk.ID
    }
    skillMap := character.BuildSkillsFromJob(job, allSkillIDs, chosen)
    if err := s.CharacterSkillsRepo.SetAll(ctx, char.ID, skillMap); err != nil {
        return fmt.Errorf("backfill skills: %w", err)
    }
}
```

**Step 6: Run all tests**

```bash
go test ./...
```
Expected: all pass

**Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat: wire skill selection into character creation and session backfill"
```

---

## Task 9: `skills` command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go` (CMD-1, CMD-2)
- Create: `internal/game/command/skills.go` (CMD-3)
- Modify: `api/proto/game/v1/game.proto` (CMD-4)
- Modify: `internal/frontend/handlers/bridge_handlers.go` (CMD-5)
- Modify: `internal/frontend/handlers/text_renderer.go` (rendering)
- Modify: `internal/gameserver/grpc_service.go` (CMD-6)
- Create: `internal/game/command/skills_test.go`

### CMD-1 and CMD-2: Register the command

In `internal/game/command/commands.go`:

Add constant:
```go
HandlerSkills = "skills"
```

Add to `BuiltinCommands()`:
```go
{
    Name:        "skills",
    Aliases:     []string{"sk"},
    Description: "Display your skill proficiencies.",
    Category:    CategoryWorld,
    Handler:     HandlerSkills,
},
```

### CMD-3: Handle function with TDD

**Write failing test** in `internal/game/command/skills_test.go`:

```go
package command_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleSkills_ReturnsNonEmpty(t *testing.T) {
	result := command.HandleSkills()
	if result == "" {
		t.Error("HandleSkills must return non-empty string")
	}
}
```

Run: `go test ./internal/game/command/ -run TestHandleSkills -v` → FAIL

Implement `internal/game/command/skills.go`:

```go
package command

// HandleSkills returns the skills command client acknowledgement.
// The actual skill data is returned by the server in a SkillsResponse.
//
// Postcondition: Returns a non-empty string.
func HandleSkills() string {
	return "Reviewing your skills..."
}
```

Run: `go test ./internal/game/command/ -run TestHandleSkills -v` → PASS

### CMD-4: Proto message

In `api/proto/game/v1/game.proto`:

Add request message (near `MapRequest`):
```protobuf
// SkillsRequest is sent by the client to view skill proficiencies.
message SkillsRequest {}

// SkillEntry represents one skill with its current proficiency rank.
message SkillEntry {
  string skill_id    = 1;
  string name        = 2;
  string ability     = 3;
  string proficiency = 4;
}

// SkillsResponse contains all skill entries for the character.
message SkillsResponse {
  repeated SkillEntry skills = 1;
}
```

Add to `ClientMessage` oneof:
```protobuf
SkillsRequest skills = 38;
```

Add to `ServerEvent` oneof:
```protobuf
SkillsResponse skills_response = <next_number>;
```

Run:
```bash
make proto
```
Expected: regenerated files compile cleanly.

### CMD-5: Bridge handler

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap`:
```go
command.HandlerSkills: bridgeSkills,
```

Add function:
```go
func bridgeSkills(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Skills{Skills: &gamev1.SkillsRequest{}},
    }}, nil
}
```

Run:
```bash
go test ./internal/frontend/handlers/ -run TestAllCommandHandlersAreWired -v
```
Expected: PASS

### CMD-5: Renderer

Add `RenderSkillsResponse` to `internal/frontend/handlers/text_renderer.go`:

```go
// RenderSkillsResponse formats a SkillsResponse as colored telnet text.
// Skills are grouped by ability score. Trained skills are highlighted in cyan.
//
// Precondition: sr must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderSkillsResponse(sr *gamev1.SkillsResponse) string {
    // Ability display order and labels
    abilityOrder := []string{"quickness", "brutality", "reasoning", "savvy", "flair"}
    abilityLabel := map[string]string{
        "quickness": "Quickness",
        "brutality": "Brutality",
        "reasoning": "Reasoning",
        "savvy":     "Savvy",
        "flair":     "Flair",
    }

    // Group skills by ability
    byAbility := make(map[string][]*gamev1.SkillEntry)
    for _, sk := range sr.Skills {
        byAbility[sk.Ability] = append(byAbility[sk.Ability], sk)
    }

    var sb strings.Builder
    sb.WriteString(telnet.Colorize(telnet.BrightWhite, "=== Skills ==="))
    sb.WriteString("\r\n")

    for _, ability := range abilityOrder {
        skills, ok := byAbility[ability]
        if !ok {
            continue
        }
        sb.WriteString(fmt.Sprintf("\r\n%s:\r\n", abilityLabel[ability]))
        for _, sk := range skills {
            name := fmt.Sprintf("  %-14s", sk.Name)
            prof := sk.Proficiency
            if prof == "trained" || prof == "expert" || prof == "master" || prof == "legendary" {
                sb.WriteString(telnet.Colorize(telnet.Cyan, name))
                sb.WriteString(telnet.Colorize(telnet.Cyan, prof))
            } else {
                sb.WriteString(telnet.Colorize(telnet.Dim, name))
                sb.WriteString(telnet.Colorize(telnet.Dim, prof))
            }
            sb.WriteString("\r\n")
        }
    }
    return sb.String()
}
```

Add a test in `internal/frontend/handlers/text_renderer_test.go`:

```go
func TestRenderSkillsResponse_GroupedByAbility(t *testing.T) {
    sr := &gamev1.SkillsResponse{
        Skills: []*gamev1.SkillEntry{
            {SkillId: "parkour", Name: "Parkour", Ability: "quickness", Proficiency: "trained"},
            {SkillId: "muscle",  Name: "Muscle",  Ability: "brutality", Proficiency: "untrained"},
        },
    }
    out := handlers.RenderSkillsResponse(sr)
    if !strings.Contains(out, "Quickness") {
        t.Error("expected Quickness section")
    }
    if !strings.Contains(out, "Parkour") {
        t.Error("expected Parkour skill")
    }
    if !strings.Contains(out, "Brutality") {
        t.Error("expected Brutality section")
    }
}
```

### CMD-6: gRPC dispatch

In `internal/gameserver/grpc_service.go`, add to the type switch:

```go
case *gamev1.ClientMessage_Skills:
    return s.handleSkills(uid)
```

Add handler:

```go
// handleSkills returns all skill proficiencies for the player's current character.
//
// Precondition: uid must resolve to an active session with a character.
// Postcondition: Returns a ServerEvent with SkillsResponse.
func (s *GameServiceServer) handleSkills(uid string) (*gamev1.ServerEvent, error) {
    char, err := s.getCharacter(uid)
    if err != nil {
        return nil, err
    }

    skills, err := s.CharacterSkillsRepo.GetAll(context.Background(), char.ID)
    if err != nil {
        return nil, fmt.Errorf("getting skills: %w", err)
    }

    // Build ordered entries using AllSkills for canonical order and names
    entries := make([]*gamev1.SkillEntry, 0, len(s.AllSkills))
    for _, sk := range s.AllSkills {
        prof, ok := skills[sk.ID]
        if !ok {
            prof = "untrained"
        }
        entries = append(entries, &gamev1.SkillEntry{
            SkillId:     sk.ID,
            Name:        sk.Name,
            Ability:     sk.Ability,
            Proficiency: prof,
        })
    }

    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_SkillsResponse{
            SkillsResponse: &gamev1.SkillsResponse{Skills: entries},
        },
    }, nil
}
```

Wire the `SkillsResponse` rendering in the frontend event handler (wherever `MapResponse` is handled, add a parallel case for `SkillsResponse`).

**Step 7: Run all tests**

```bash
go test ./...
```
Expected: all pass

**Step 8: Commit**

```bash
git add \
  internal/game/command/commands.go \
  internal/game/command/skills.go \
  internal/game/command/skills_test.go \
  api/proto/game/v1/game.proto \
  internal/gameserver/gamev1/ \
  internal/frontend/handlers/bridge_handlers.go \
  internal/frontend/handlers/text_renderer.go \
  internal/frontend/handlers/text_renderer_test.go \
  internal/gameserver/grpc_service.go
git commit -m "feat: implement skills command end-to-end (CMD-1 through CMD-7)"
```

---

## Task 10: Deploy and verify

**Step 1: Final test run**

```bash
go test ./...
```
Expected: all pass

**Step 2: Deploy**

```bash
make docker-push
# Then update deployments (see DEPLOY-1 in AGENTS.md):
TAG=$(git rev-parse --short HEAD)
kubectl set image deployment/frontend frontend=registry.johannsen.cloud:5000/mud-frontend:$TAG -n mud
kubectl set image deployment/gameserver gameserver=registry.johannsen.cloud:5000/mud-gameserver:$TAG -n mud
kubectl rollout status deployment/frontend -n mud --timeout=60s
kubectl rollout status deployment/gameserver -n mud --timeout=60s
```

**Step 3: Verify in-game**

Log in, select a character (new or existing). For existing characters: expect the skill selection prompt before entering the world. Once in-game:

```
> skills
=== Skills ===

Quickness:
  Parkour        untrained
  Ghosting       trained
  ...
```

**Step 4: Update FEATURES.md**

Mark the skills sub-item as done in `docs/requirements/FEATURES.md`.

**Step 5: Final commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark skills stage 1 as complete"
```
