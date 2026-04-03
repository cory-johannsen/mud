# Technology ID Refactor — Gunchete Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a two-phase CLI tool (`cmd/rename-tech-ids`) that generates a rename map from all technology YAML files (deriving IDs from Gunchete names), then applies the map to rename YAML files, update references, and emit a DB migration.

**Architecture:** Phase 1 (`--generate`) scans `content/technologies/` and writes `tools/rename_map.yaml` with derived IDs, PF2E flags, and collision markers. After human review of the map, Phase 2 (`--apply`) renames files, rewrites `id:` fields, updates job/archetype YAML refs, updates Go string literals in `static_localizer.go` and test files, and emits `migrations/058_rename_tech_ids.{up,down}.sql`. A validation pass at the end loads the full tech Registry and asserts zero errors.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/rename-tech-ids/main.go` | Create | CLI entry point; flag parsing; dispatch to generate/apply |
| `cmd/rename-tech-ids/rename.go` | Create | `ToSnakeCase`, `IsPF2EFlagged`, `stripTraditionSuffix`, `RenameEntry`, `RenameMap`, `BuildRenameMap` |
| `cmd/rename-tech-ids/rename_test.go` | Create | Property + table tests for derivation, flagging, collision detection |
| `cmd/rename-tech-ids/generate.go` | Create | `RunGenerate`: walk tech YAMLs, call `BuildRenameMap`, write `tools/rename_map.yaml` |
| `cmd/rename-tech-ids/generate_test.go` | Create | `RunGenerate` with temp fixture tree |
| `cmd/rename-tech-ids/apply.go` | Create | `RunApply`: YAML renames + id rewrites + job/archetype refs + Go sources + migration emit + validation |
| `cmd/rename-tech-ids/apply_test.go` | Create | Apply correctness, idempotency, and validation-pass-failure tests |
| `Makefile` | Modify | Add `build-rename-tech-ids` target |

---

### Task 1: CLI scaffold and `ToSnakeCase`

**Files:**
- Create: `cmd/rename-tech-ids/main.go`
- Create: `cmd/rename-tech-ids/rename.go`
- Create: `cmd/rename-tech-ids/rename_test.go`

- [ ] **Step 1: Write failing tests for `ToSnakeCase`**

Create `cmd/rename-tech-ids/rename_test.go`:

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestToSnakeCase_TableDriven(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Corrosive Projectile", "corrosive_projectile"},
		{"Cranial Shock", "cranial_shock"},
		{"Chrome Reflex", "chrome_reflex"},
		{"K'galaserke's Axes", "kgalaserkes_axes"},
		{"100 Volt Shock", "100_volt_shock"},
		{"  Trim   Me  ", "trim_me"},
		{"Acid Storm", "acid_storm"},
		{"Single", "single"},
		{"Already_snake", "already_snake"},
		{"Hyphens-Are-Removed", "hyphens_are_removed"},
		{"Dots.Are.Removed", "dots_are_removed"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, ToSnakeCase(tc.input))
		})
	}
}

func TestToSnakeCase_Property_OutputOnlySnakeChars(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.StringOf(rapid.RuneFrom(nil)).Draw(rt, "name")
		result := ToSnakeCase(input)
		for _, r := range result {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				rt.Fatalf("ToSnakeCase(%q) = %q contains invalid char %q", input, result, r)
			}
		}
	})
}

func TestToSnakeCase_Property_NoLeadingOrTrailingUnderscore(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.StringOf(rapid.RuneFrom(nil)).Draw(rt, "name")
		result := ToSnakeCase(input)
		if len(result) > 0 {
			if result[0] == '_' {
				rt.Fatalf("ToSnakeCase(%q) = %q starts with underscore", input, result)
			}
			if result[len(result)-1] == '_' {
				rt.Fatalf("ToSnakeCase(%q) = %q ends with underscore", input, result)
			}
		}
	})
}

func TestToSnakeCase_Property_NoConsecutiveUnderscores(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.StringOf(rapid.RuneFrom(nil)).Draw(rt, "name")
		result := ToSnakeCase(input)
		for i := 0; i < len(result)-1; i++ {
			if result[i] == '_' && result[i+1] == '_' {
				rt.Fatalf("ToSnakeCase(%q) = %q has consecutive underscores", input, result)
			}
		}
	})
}
```

- [ ] **Step 2: Run test — expect compile failure (functions not defined)**

```bash
go test ./cmd/rename-tech-ids/... 2>&1
```

Expected: `cannot find package` or `undefined: ToSnakeCase`

- [ ] **Step 3: Create `cmd/rename-tech-ids/rename.go` with `ToSnakeCase`**

```go
package main

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumSpaceRE = regexp.MustCompile(`[^a-z0-9 ]`)
	whitespaceRE       = regexp.MustCompile(`\s+`)
	multiUnderscoreRE  = regexp.MustCompile(`_+`)
)

// ToSnakeCase converts a human-readable name to a snake_case identifier.
// All non-alphanumeric, non-space characters are removed; spaces become
// underscores; consecutive underscores are collapsed; result is trimmed.
func ToSnakeCase(name string) string {
	lower := strings.ToLower(name)
	cleaned := nonAlphanumSpaceRE.ReplaceAllString(lower, "")
	withUnder := whitespaceRE.ReplaceAllString(strings.TrimSpace(cleaned), "_")
	collapsed := multiUnderscoreRE.ReplaceAllString(withUnder, "_")
	return strings.Trim(collapsed, "_")
}
```

- [ ] **Step 4: Create minimal `cmd/rename-tech-ids/main.go`**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = args
	return fmt.Errorf("not yet implemented")
}
```

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestToSnakeCase -v
```

Expected: all `TestToSnakeCase_*` tests PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/rename-tech-ids/
git commit -m "feat(rename-tech-ids): scaffold CLI and ToSnakeCase with property tests"
```

---

### Task 2: PF2E flag heuristic and `stripTraditionSuffix`

**Files:**
- Modify: `cmd/rename-tech-ids/rename.go`
- Modify: `cmd/rename-tech-ids/rename_test.go`

- [ ] **Step 1: Write failing tests for `IsPF2EFlagged` and `stripTraditionSuffix`**

Append to `cmd/rename-tech-ids/rename_test.go`:

```go
func TestStripTraditionSuffix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"acid_arrow_technical", "acid_arrow"},
		{"daze_neural", "daze"},
		{"sleep_bio_synthetic", "sleep"},
		{"bless_fanatic_doctrine", "bless"},
		{"chrome_reflex", "chrome_reflex"}, // no suffix
		{"neural_static", "neural_static"}, // no suffix
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, stripTraditionSuffix(tc.input))
		})
	}
}

func TestIsPF2EFlagged(t *testing.T) {
	cases := []struct {
		name   string
		oldID  string
		wantFl bool
		desc   string
	}{
		// REQ-TIR-PF2: name never localized — derived matches stripped old_id
		{"Acid Arrow", "acid_arrow_technical", true, "PF2E name unchanged"},
		{"Daze", "daze_neural", true, "PF2E name unchanged single word"},
		// REQ-TIR-PF3: keyword deny-list
		{"Antimagic Field", "antimagic_field_neural", true, "keyword: antimagic"},
		{"Scrying Lens", "scrying_lens_neural", true, "keyword: scrying"},
		// Already Gunchete — no flag
		{"Corrosive Projectile", "acid_arrow_technical", false, "localized name"},
		{"Cranial Shock", "daze_neural", false, "localized name"},
		{"Chrome Reflex", "chrome_reflex", false, "innate already correct"},
		{"Neural Static", "neural_static", false, "innate already correct"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.wantFl, IsPF2EFlagged(tc.name, tc.oldID))
		})
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run "TestStripTradition|TestIsPF2E" -v
```

Expected: `undefined: stripTraditionSuffix`, `undefined: IsPF2EFlagged`

- [ ] **Step 3: Add `stripTraditionSuffix`, `IsPF2EFlagged`, and deny-list to `rename.go`**

Append to `cmd/rename-tech-ids/rename.go`:

```go
var traditionSuffixes = []string{
	"_technical", "_neural", "_bio_synthetic", "_fanatic_doctrine",
}

// stripTraditionSuffix removes a known tradition suffix from id, if present.
func stripTraditionSuffix(id string) string {
	for _, s := range traditionSuffixes {
		if strings.HasSuffix(id, s) {
			return strings.TrimSuffix(id, s)
		}
	}
	return id
}

// pf2eKeywords is a deny-list of terms that indicate an un-localized PF2E name.
var pf2eKeywords = []string{
	"firebolt", "fireball", "magic missile", "telekinesis",
	"bestow curse", "mage hand", "shillelagh", "prestidigitation",
	"tongues", "scrying", "antimagic",
}

// IsPF2EFlagged returns true if name appears to be an unlocalised PF2E source name.
// REQ-TIR-PF2: derived new_id matches old_id minus tradition suffix → never localized.
// REQ-TIR-PF3: name contains a known PF2E keyword.
func IsPF2EFlagged(name, oldID string) bool {
	if ToSnakeCase(name) == stripTraditionSuffix(oldID) {
		return true
	}
	lower := strings.ToLower(name)
	for _, kw := range pf2eKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run "TestStripTradition|TestIsPF2E" -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rename-tech-ids/rename.go cmd/rename-tech-ids/rename_test.go
git commit -m "feat(rename-tech-ids): PF2E flag heuristic and tradition suffix stripping"
```

---

### Task 3: `RenameEntry`, `RenameMap`, and `BuildRenameMap`

**Files:**
- Modify: `cmd/rename-tech-ids/rename.go`
- Modify: `cmd/rename-tech-ids/rename_test.go`

- [ ] **Step 1: Write failing tests for `BuildRenameMap`**

Append to `cmd/rename-tech-ids/rename_test.go`:

```go
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// writeTechYAML writes a minimal tech YAML file to dir/subdir/filename.
func writeTechYAML(t *testing.T, dir, subdir, filename, id, name string) {
	t.Helper()
	sub := filepath.Join(dir, subdir)
	require.NoError(t, os.MkdirAll(sub, 0755))
	content := fmt.Sprintf("id: %s\nname: %s\ntradition: technical\nlevel: 1\nusage_type: prepared\n", id, name)
	require.NoError(t, os.WriteFile(filepath.Join(sub, filename), []byte(content), 0644))
}

func TestBuildRenameMap_Basic(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Corrosive Projectile")
	writeTechYAML(t, dir, "neural", "daze_neural.yaml", "daze_neural", "Cranial Shock")
	writeTechYAML(t, dir, "innate", "chrome_reflex.yaml", "chrome_reflex", "Chrome Reflex")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)
	require.Len(t, rm.Renames, 3)

	byOld := make(map[string]RenameEntry)
	for _, e := range rm.Renames {
		byOld[e.OldID] = e
	}

	e := byOld["acid_arrow_technical"]
	assert.Equal(t, "corrosive_projectile", e.NewID)
	assert.False(t, e.Skip)
	assert.False(t, e.PF2EFlag)
	assert.False(t, e.Collision)

	e = byOld["daze_neural"]
	assert.Equal(t, "cranial_shock", e.NewID)
	assert.False(t, e.Skip)

	e = byOld["chrome_reflex"]
	assert.Equal(t, "chrome_reflex", e.NewID)
	assert.True(t, e.Skip, "already-correct IDs must be marked skip")
}

func TestBuildRenameMap_CollisionDetected(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Shock Wave")
	writeTechYAML(t, dir, "neural", "daze_neural.yaml", "daze_neural", "Shock Wave")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)

	for _, e := range rm.Renames {
		assert.True(t, e.Collision, "both entries deriving same new_id must be flagged collision: old=%s", e.OldID)
	}
}

func TestBuildRenameMap_PF2EFlagSet(t *testing.T) {
	dir := t.TempDir()
	// Name never localized — derives same ID as old (minus suffix)
	writeTechYAML(t, dir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Acid Arrow")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)
	require.Len(t, rm.Renames, 1)
	assert.True(t, rm.Renames[0].PF2EFlag)
}

func TestBuildRenameMap_SortedByOldID(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "technical", "z_tech.yaml", "z_tech", "Zap")
	writeTechYAML(t, dir, "technical", "a_tech.yaml", "a_tech", "Alpha Strike")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)
	require.Len(t, rm.Renames, 2)
	assert.True(t, rm.Renames[0].OldID < rm.Renames[1].OldID, "entries must be sorted by old_id")
}
```

Note: add `"fmt"` to the import block in `rename_test.go`.

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestBuildRenameMap -v
```

Expected: `undefined: RenameEntry`, `undefined: RenameMap`, `undefined: BuildRenameMap`

- [ ] **Step 3: Add `RenameEntry`, `RenameMap`, and `BuildRenameMap` to `rename.go`**

Append to `cmd/rename-tech-ids/rename.go`:

```go
import (
	// add to existing imports:
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// RenameEntry is one row in the rename map.
type RenameEntry struct {
	OldID     string `yaml:"old_id"`
	NewID     string `yaml:"new_id"`
	Name      string `yaml:"name"`
	File      string `yaml:"file"`
	Skip      bool   `yaml:"skip"`
	PF2EFlag  bool   `yaml:"pf2e_flag"`
	Collision bool   `yaml:"collision"`
}

// RenameMap is the top-level structure of tools/rename_map.yaml.
type RenameMap struct {
	Renames []RenameEntry `yaml:"renames"`
}

// techFileHeader holds just the id and name fields from a tech YAML file.
type techFileHeader struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// BuildRenameMap scans all .yaml files under techDir, derives new IDs,
// detects collisions and PF2E flags, and returns the complete RenameMap
// sorted by old_id.
//
// Precondition: techDir is a valid, existing directory.
// Postcondition: all collision entries have Collision=true; all no-op entries have Skip=true.
func BuildRenameMap(techDir string) (*RenameMap, error) {
	var entries []RenameEntry

	err := filepath.WalkDir(techDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}
		var h techFileHeader
		if err := yaml.Unmarshal(data, &h); err != nil {
			return fmt.Errorf("parsing %q: %w", path, err)
		}
		if h.ID == "" || h.Name == "" {
			return fmt.Errorf("missing id or name in %q", path)
		}
		newID := ToSnakeCase(h.Name)
		entries = append(entries, RenameEntry{
			OldID:    h.ID,
			NewID:    newID,
			Name:     h.Name,
			File:     path,
			Skip:     h.ID == newID,
			PF2EFlag: IsPF2EFlagged(h.Name, h.ID),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Count occurrences of each new_id among non-skip entries to detect collisions.
	newIDCount := make(map[string]int)
	for _, e := range entries {
		if !e.Skip {
			newIDCount[e.NewID]++
		}
	}
	for i := range entries {
		if !entries[i].Skip && newIDCount[entries[i].NewID] > 1 {
			entries[i].Collision = true
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].OldID < entries[j].OldID
	})

	return &RenameMap{Renames: entries}, nil
}
```

Update the import block in `rename.go` to include all needed packages:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestBuildRenameMap -v
```

Expected: all PASS

- [ ] **Step 5: Run all tool tests so far**

```bash
go test ./cmd/rename-tech-ids/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/rename-tech-ids/rename.go cmd/rename-tech-ids/rename_test.go
git commit -m "feat(rename-tech-ids): RenameEntry/RenameMap types and BuildRenameMap with collision detection"
```

---

### Task 4: Generate command — write `tools/rename_map.yaml`

**Files:**
- Create: `cmd/rename-tech-ids/generate.go`
- Create: `cmd/rename-tech-ids/generate_test.go`

- [ ] **Step 1: Write failing test for `RunGenerate`**

Create `cmd/rename-tech-ids/generate_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRunGenerate_WritesMapFile(t *testing.T) {
	techDir := t.TempDir()
	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "rename_map.yaml")

	writeTechYAML(t, techDir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Corrosive Projectile")
	writeTechYAML(t, techDir, "neural", "chrome_reflex.yaml", "chrome_reflex", "Chrome Reflex")

	err := RunGenerate(techDir, outFile)
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)

	var rm RenameMap
	require.NoError(t, yaml.Unmarshal(data, &rm))
	require.Len(t, rm.Renames, 2)

	byOld := make(map[string]RenameEntry)
	for _, e := range rm.Renames {
		byOld[e.OldID] = e
	}
	assert.Equal(t, "corrosive_projectile", byOld["acid_arrow_technical"].NewID)
	assert.True(t, byOld["chrome_reflex"].Skip)
}

func TestRunGenerate_CreatesParentDir(t *testing.T) {
	techDir := t.TempDir()
	outFile := filepath.Join(t.TempDir(), "nested", "dir", "rename_map.yaml")
	writeTechYAML(t, techDir, "technical", "x.yaml", "x_technical", "X Thing")

	err := RunGenerate(techDir, outFile)
	require.NoError(t, err)
	_, err = os.Stat(outFile)
	assert.NoError(t, err, "output file must exist")
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestRunGenerate -v
```

Expected: `undefined: RunGenerate`

- [ ] **Step 3: Create `cmd/rename-tech-ids/generate.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RunGenerate scans techDir for technology YAML files, builds a RenameMap,
// and writes it to outFile (creating parent directories as needed).
//
// Precondition: techDir exists.
// Postcondition: outFile contains valid YAML representing the RenameMap.
func RunGenerate(techDir, outFile string) error {
	rm, err := BuildRenameMap(techDir)
	if err != nil {
		return fmt.Errorf("building rename map: %w", err)
	}

	data, err := yaml.Marshal(rm)
	if err != nil {
		return fmt.Errorf("marshalling rename map: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", outFile, err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestRunGenerate -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rename-tech-ids/generate.go cmd/rename-tech-ids/generate_test.go
git commit -m "feat(rename-tech-ids): RunGenerate writes tools/rename_map.yaml"
```

---

### Task 5: Apply — YAML file rename and id field rewrite

**Files:**
- Create: `cmd/rename-tech-ids/apply.go`
- Create: `cmd/rename-tech-ids/apply_test.go`

- [ ] **Step 1: Write failing tests for YAML rename and id rewrite**

Create `cmd/rename-tech-ids/apply_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRenameMap builds a RenameMap from a slice of (old,new,name,file,skip) tuples.
func makeEntry(oldID, newID, name, file string, skip bool) RenameEntry {
	return RenameEntry{OldID: oldID, NewID: newID, Name: name, File: file, Skip: skip}
}

func TestRenameYAMLFile_RenamesFileAndRewritesID(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "acid_arrow_technical.yaml")
	content := "id: acid_arrow_technical\nname: Corrosive Projectile\ntradition: technical\nlevel: 1\nusage_type: prepared\naction_cost: 2\nrange: ranged\ntargets: single\nduration: instant\nresolution: none\neffects: {}\n"
	require.NoError(t, os.WriteFile(oldPath, []byte(content), 0644))

	entry := makeEntry("acid_arrow_technical", "corrosive_projectile", "Corrosive Projectile", oldPath, false)
	newPath, err := renameYAMLFile(entry)
	require.NoError(t, err)

	// Old file must not exist
	_, err = os.Stat(oldPath)
	assert.True(t, os.IsNotExist(err), "old file must be gone")

	// New file must exist at new path
	expected := filepath.Join(dir, "corrosive_projectile.yaml")
	assert.Equal(t, expected, newPath)
	data, err := os.ReadFile(newPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: corrosive_projectile")
	assert.NotContains(t, string(data), "id: acid_arrow_technical")
}

func TestRenameYAMLFile_SkipEntry_NoOp(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "chrome_reflex.yaml")
	content := "id: chrome_reflex\nname: Chrome Reflex\n"
	require.NoError(t, os.WriteFile(oldPath, []byte(content), 0644))

	entry := makeEntry("chrome_reflex", "chrome_reflex", "Chrome Reflex", oldPath, true)
	newPath, err := renameYAMLFile(entry)
	require.NoError(t, err)
	assert.Equal(t, oldPath, newPath, "skip entries must not be renamed")

	// File must still exist unchanged
	data, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: chrome_reflex")
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestRenameYAML -v
```

Expected: `undefined: renameYAMLFile`

- [ ] **Step 3: Create `cmd/rename-tech-ids/apply.go` with `renameYAMLFile`**

```go
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renameYAMLFile renames the YAML file for entry from old_id stem to new_id stem,
// and rewrites the id: field in the file content.
// Returns the new file path (== entry.File if skip=true).
//
// Precondition: entry.File exists on disk.
// Postcondition: if !skip, old file is gone, new file exists with updated id field.
func renameYAMLFile(entry RenameEntry) (string, error) {
	if entry.Skip {
		return entry.File, nil
	}

	data, err := os.ReadFile(entry.File)
	if err != nil {
		return "", fmt.Errorf("reading %q: %w", entry.File, err)
	}

	// Rewrite id: field — replace "id: <old_id>\n" with "id: <new_id>\n"
	oldLine := []byte("id: " + entry.OldID + "\n")
	newLine := []byte("id: " + entry.NewID + "\n")
	updated := bytes.Replace(data, oldLine, newLine, 1)
	if bytes.Equal(data, updated) {
		return "", fmt.Errorf("id line %q not found in %q", string(oldLine), entry.File)
	}

	// Compute new file path: same directory, new stem
	dir := filepath.Dir(entry.File)
	newPath := filepath.Join(dir, entry.NewID+".yaml")

	if err := os.WriteFile(entry.File, updated, 0644); err != nil {
		return "", fmt.Errorf("writing updated content to %q: %w", entry.File, err)
	}
	if err := os.Rename(entry.File, newPath); err != nil {
		return "", fmt.Errorf("renaming %q to %q: %w", entry.File, newPath, err)
	}

	return newPath, nil
}

// RunApply reads the rename map at mapFile and applies all non-skip, non-collision renames.
// Placeholder — full implementation added in subsequent tasks.
func RunApply(mapFile, techDir, jobDir, archetypeDir, goSourceDir, migrationsDir string) error {
	_ = mapFile
	_ = techDir
	_ = jobDir
	_ = archetypeDir
	_ = goSourceDir
	_ = migrationsDir
	return fmt.Errorf("RunApply: not yet fully implemented")
}

// unused prevents import errors — remove when RunApply uses strings
var _ = strings.Contains
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestRenameYAML -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rename-tech-ids/apply.go cmd/rename-tech-ids/apply_test.go
git commit -m "feat(rename-tech-ids): renameYAMLFile renames file and rewrites id field"
```

---

### Task 6: Apply — job and archetype reference updates

**Files:**
- Modify: `cmd/rename-tech-ids/apply.go`
- Modify: `cmd/rename-tech-ids/apply_test.go`

- [ ] **Step 1: Write failing tests for `updateFileReferences`**

Append to `cmd/rename-tech-ids/apply_test.go`:

```go
func TestUpdateFileReferences_ReplacesAllOccurrences(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "illusionist.yaml")
	content := `id: illusionist
technology_grants:
  prepared:
    pool:
      - { id: acid_arrow_technical, level: 1 }
      - { id: daze_neural, level: 1 }
      - { id: chrome_reflex, level: 1 }
level_up_grants:
  3:
    prepared:
      pool:
        - id: acid_arrow_technical
          level: 2
`
	require.NoError(t, os.WriteFile(jobFile, []byte(content), 0644))

	renames := map[string]string{
		"acid_arrow_technical": "corrosive_projectile",
		"daze_neural":          "cranial_shock",
	}
	err := updateFileReferences(jobFile, renames)
	require.NoError(t, err)

	data, err := os.ReadFile(jobFile)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, "id: corrosive_projectile")
	assert.Contains(t, s, "id: cranial_shock")
	assert.Contains(t, s, "id: chrome_reflex", "unrenamed IDs must be untouched")
	assert.NotContains(t, s, "acid_arrow_technical")
	assert.NotContains(t, s, "daze_neural")
	// Job id line must not be renamed
	assert.Contains(t, s, "id: illusionist")
}

func TestUpdateFileReferences_Idempotent(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "job.yaml")
	content := "id: job\ntechnology_grants:\n  pool:\n    - { id: corrosive_projectile, level: 1 }\n"
	require.NoError(t, os.WriteFile(jobFile, []byte(content), 0644))

	renames := map[string]string{"acid_arrow_technical": "corrosive_projectile"}
	require.NoError(t, updateFileReferences(jobFile, renames))

	data, _ := os.ReadFile(jobFile)
	require.NoError(t, updateFileReferences(jobFile, renames))
	data2, _ := os.ReadFile(jobFile)
	assert.Equal(t, string(data), string(data2), "second pass must be a no-op")
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestUpdateFileReferences -v
```

Expected: `undefined: updateFileReferences`

- [ ] **Step 3: Add `updateFileReferences` to `apply.go`**

Append to `cmd/rename-tech-ids/apply.go`:

```go
// updateFileReferences replaces all occurrences of "id: <old>" with "id: <new>"
// in the named file for every entry in the renames map.
// The replacement is performed as a plain string substitution; it is safe
// because tech IDs consist only of [a-z0-9_] and always follow "id: ".
//
// Precondition: file exists; renames maps old_id → new_id.
// Postcondition: file content updated in-place; idempotent (second call is a no-op).
func updateFileReferences(file string, renames map[string]string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading %q: %w", file, err)
	}
	content := string(data)
	for oldID, newID := range renames {
		content = strings.ReplaceAll(content, "id: "+oldID, "id: "+newID)
	}
	return os.WriteFile(file, []byte(content), 0644)
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestUpdateFileReferences -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rename-tech-ids/apply.go cmd/rename-tech-ids/apply_test.go
git commit -m "feat(rename-tech-ids): updateFileReferences for job/archetype YAML refs"
```

---

### Task 7: Apply — Go source string literal updates

**Files:**
- Modify: `cmd/rename-tech-ids/apply.go`
- Modify: `cmd/rename-tech-ids/apply_test.go`

- [ ] **Step 1: Write failing tests for `updateGoStringLiterals`**

Append to `cmd/rename-tech-ids/apply_test.go`:

```go
func TestUpdateGoStringLiterals_BacktickMapKeys(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "static_localizer.go")
	content := "var m = map[string]x{\n\t`acid_arrow_technical`: {Name: `Corrosive Projectile`},\n\t`daze_neural`: {Name: `Cranial Shock`},\n\t`chrome_reflex`: {Name: `Chrome Reflex`},\n}\n"
	require.NoError(t, os.WriteFile(goFile, []byte(content), 0644))

	renames := map[string]string{
		"acid_arrow_technical": "corrosive_projectile",
		"daze_neural":          "cranial_shock",
	}
	err := updateGoStringLiterals(goFile, renames)
	require.NoError(t, err)

	data, err := os.ReadFile(goFile)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, "`corrosive_projectile`:")
	assert.Contains(t, s, "`cranial_shock`:")
	assert.Contains(t, s, "`chrome_reflex`:", "unrenamed ID must be untouched")
	assert.NotContains(t, s, "`acid_arrow_technical`")
	assert.NotContains(t, s, "`daze_neural`")
	// Value backtick strings must be untouched
	assert.Contains(t, s, "`Corrosive Projectile`")
}

func TestUpdateGoStringLiterals_DoubleQuotedStrings(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "service_test.go")
	content := "techID := \"acid_arrow_technical\"\nother := \"chrome_reflex\"\n"
	require.NoError(t, os.WriteFile(goFile, []byte(content), 0644))

	renames := map[string]string{"acid_arrow_technical": "corrosive_projectile"}
	require.NoError(t, updateGoStringLiterals(goFile, renames))

	data, _ := os.ReadFile(goFile)
	s := string(data)
	assert.Contains(t, s, `"corrosive_projectile"`)
	assert.NotContains(t, s, `"acid_arrow_technical"`)
	assert.Contains(t, s, `"chrome_reflex"`)
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestUpdateGoStringLiterals -v
```

Expected: `undefined: updateGoStringLiterals`

- [ ] **Step 3: Add `updateGoStringLiterals` to `apply.go`**

Append to `cmd/rename-tech-ids/apply.go`:

```go
// updateGoStringLiterals replaces backtick-quoted and double-quoted occurrences
// of old tech IDs with new IDs in a Go source file.
// Specifically replaces:
//   - "`<old_id>`" → "`<new_id>`"  (map keys and string literals)
//   - `"<old_id>"` → `"<new_id>"`  (double-quoted string literals)
//
// Precondition: file exists; renames maps old_id → new_id.
// Postcondition: file updated in-place; idempotent.
func updateGoStringLiterals(file string, renames map[string]string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading %q: %w", file, err)
	}
	content := string(data)
	for oldID, newID := range renames {
		// backtick-quoted: `old_id`
		content = strings.ReplaceAll(content, "`"+oldID+"`", "`"+newID+"`")
		// double-quoted: "old_id"
		content = strings.ReplaceAll(content, `"`+oldID+`"`, `"`+newID+`"`)
	}
	return os.WriteFile(file, []byte(content), 0644)
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestUpdateGoStringLiterals -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rename-tech-ids/apply.go cmd/rename-tech-ids/apply_test.go
git commit -m "feat(rename-tech-ids): updateGoStringLiterals for static_localizer and test files"
```

---

### Task 8: Apply — DB migration emit

**Files:**
- Modify: `cmd/rename-tech-ids/apply.go`
- Modify: `cmd/rename-tech-ids/apply_test.go`

- [ ] **Step 1: Write failing tests for `emitMigration`**

Append to `cmd/rename-tech-ids/apply_test.go`:

```go
func TestEmitMigration_UpAndDown(t *testing.T) {
	dir := t.TempDir()
	upFile := filepath.Join(dir, "058_rename_tech_ids.up.sql")
	downFile := filepath.Join(dir, "058_rename_tech_ids.down.sql")

	renames := []RenameEntry{
		{OldID: "acid_arrow_technical", NewID: "corrosive_projectile", Skip: false},
		{OldID: "daze_neural", NewID: "cranial_shock", Skip: false},
		{OldID: "chrome_reflex", NewID: "chrome_reflex", Skip: true},
	}

	err := emitMigration(renames, upFile, downFile)
	require.NoError(t, err)

	up, err := os.ReadFile(upFile)
	require.NoError(t, err)
	upStr := string(up)
	// Each non-skip entry gets UPDATE statements for all 4 tables
	assert.Contains(t, upStr, "SET tech_id = 'corrosive_projectile' WHERE tech_id = 'acid_arrow_technical'")
	assert.Contains(t, upStr, "SET tech_id = 'cranial_shock' WHERE tech_id = 'daze_neural'")
	assert.NotContains(t, upStr, "chrome_reflex", "skip entries must not appear in migration")
	// Must cover all 4 tables
	assert.Contains(t, upStr, "character_hardwired_technologies")
	assert.Contains(t, upStr, "character_innate_technologies")
	assert.Contains(t, upStr, "character_spontaneous_technologies")
	assert.Contains(t, upStr, "character_prepared_technologies")

	down, err := os.ReadFile(downFile)
	require.NoError(t, err)
	downStr := string(down)
	// Down is the inverse
	assert.Contains(t, downStr, "SET tech_id = 'acid_arrow_technical' WHERE tech_id = 'corrosive_projectile'")
	assert.Contains(t, downStr, "SET tech_id = 'daze_neural' WHERE tech_id = 'cranial_shock'")
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestEmitMigration -v
```

Expected: `undefined: emitMigration`

- [ ] **Step 3: Add `emitMigration` to `apply.go`**

Append to `cmd/rename-tech-ids/apply.go`:

```go
var techIDTables = []string{
	"character_hardwired_technologies",
	"character_innate_technologies",
	"character_spontaneous_technologies",
	"character_prepared_technologies",
}

// emitMigration writes up and down SQL migration files from the rename entries.
// Skip and collision entries are excluded.
//
// Precondition: upFile and downFile paths are writable; entries is non-nil.
// Postcondition: upFile contains UPDATE statements new_id→new_id per table per rename;
// downFile contains the inverse.
func emitMigration(renames []RenameEntry, upFile, downFile string) error {
	var upBuf, downBuf strings.Builder

	upBuf.WriteString("-- Generated by cmd/rename-tech-ids --apply\n")
	downBuf.WriteString("-- Generated by cmd/rename-tech-ids --apply (rollback)\n")

	for _, e := range renames {
		if e.Skip || e.Collision {
			continue
		}
		for _, table := range techIDTables {
			fmt.Fprintf(&upBuf, "UPDATE %s SET tech_id = '%s' WHERE tech_id = '%s';\n",
				table, e.NewID, e.OldID)
			fmt.Fprintf(&downBuf, "UPDATE %s SET tech_id = '%s' WHERE tech_id = '%s';\n",
				table, e.OldID, e.NewID)
		}
		upBuf.WriteString("\n")
		downBuf.WriteString("\n")
	}

	if err := os.WriteFile(upFile, []byte(upBuf.String()), 0644); err != nil {
		return fmt.Errorf("writing up migration: %w", err)
	}
	if err := os.WriteFile(downFile, []byte(downBuf.String()), 0644); err != nil {
		return fmt.Errorf("writing down migration: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestEmitMigration -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rename-tech-ids/apply.go cmd/rename-tech-ids/apply_test.go
git commit -m "feat(rename-tech-ids): emitMigration writes 058_rename_tech_ids up/down SQL"
```

---

### Task 9: Apply — validation pass and full `RunApply`

**Files:**
- Modify: `cmd/rename-tech-ids/apply.go`
- Modify: `cmd/rename-tech-ids/apply_test.go`

- [ ] **Step 1: Write failing tests for `RunApply`**

Append to `cmd/rename-tech-ids/apply_test.go`:

```go
import (
	// add to existing imports
	"path/filepath"
)

// buildApplyFixture creates a minimal valid fixture for RunApply tests.
// Returns techDir, jobDir, archetypeDir, migrationsDir, goSourceDir, mapFile.
func buildApplyFixture(t *testing.T) (techDir, jobDir, archetypeDir, migrationsDir, goSourceDir, mapFile string) {
	t.Helper()
	base := t.TempDir()
	techDir = filepath.Join(base, "content", "technologies")
	jobDir = filepath.Join(base, "content", "jobs")
	archetypeDir = filepath.Join(base, "content", "archetypes")
	migrationsDir = filepath.Join(base, "migrations")
	goSourceDir = filepath.Join(base, "internal", "importer")
	mapFile = filepath.Join(base, "tools", "rename_map.yaml")

	for _, d := range []string{techDir, jobDir, archetypeDir, migrationsDir, goSourceDir} {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	// Write a tech YAML
	writeTechYAML(t, techDir, "technical", "acid_arrow_technical.yaml",
		"acid_arrow_technical", "Corrosive Projectile")
	// Write a skip-entry tech
	writeTechYAML(t, techDir, "innate", "chrome_reflex.yaml",
		"chrome_reflex", "Chrome Reflex")

	// Write a job YAML referencing the tech
	jobContent := "id: test_job\ntechnology_grants:\n  pool:\n    - { id: acid_arrow_technical, level: 1 }\n    - { id: chrome_reflex, level: 1 }\n"
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "test_job.yaml"), []byte(jobContent), 0644))

	// Write a static_localizer.go stub
	locContent := "package importer\nvar m = map[string]x{\n\t`acid_arrow_technical`: {Name: `Corrosive Projectile`},\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(goSourceDir, "static_localizer.go"), []byte(locContent), 0644))

	// Generate the rename map
	require.NoError(t, RunGenerate(techDir, mapFile))

	return
}

func TestRunApply_RenamesFilesAndUpdatesRefs(t *testing.T) {
	techDir, jobDir, archetypeDir, migrationsDir, goSourceDir, mapFile :=
		buildApplyFixture(t)

	err := RunApply(mapFile, techDir, jobDir, archetypeDir, goSourceDir, migrationsDir)
	require.NoError(t, err)

	// Tech YAML renamed
	newTechPath := filepath.Join(techDir, "technical", "corrosive_projectile.yaml")
	_, err = os.Stat(newTechPath)
	assert.NoError(t, err, "renamed tech YAML must exist")
	oldTechPath := filepath.Join(techDir, "technical", "acid_arrow_technical.yaml")
	_, err = os.Stat(oldTechPath)
	assert.True(t, os.IsNotExist(err), "old tech YAML must be gone")

	// Job YAML reference updated
	jobData, err := os.ReadFile(filepath.Join(jobDir, "test_job.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(jobData), "id: corrosive_projectile")
	assert.NotContains(t, string(jobData), "acid_arrow_technical")

	// Go source updated
	locData, err := os.ReadFile(filepath.Join(goSourceDir, "static_localizer.go"))
	require.NoError(t, err)
	assert.Contains(t, string(locData), "`corrosive_projectile`")
	assert.NotContains(t, string(locData), "`acid_arrow_technical`")

	// Migration files emitted
	_, err = os.Stat(filepath.Join(migrationsDir, "058_rename_tech_ids.up.sql"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(migrationsDir, "058_rename_tech_ids.down.sql"))
	assert.NoError(t, err)
}

func TestRunApply_RefusesOnUnresolvedCollision(t *testing.T) {
	base := t.TempDir()
	techDir := filepath.Join(base, "tech")
	mapFile := filepath.Join(base, "rename_map.yaml")
	require.NoError(t, os.MkdirAll(techDir, 0755))

	// Two techs that derive the same new_id → collision
	writeTechYAML(t, techDir, "technical", "a_technical.yaml", "a_technical", "Shock Wave")
	writeTechYAML(t, techDir, "neural", "b_neural.yaml", "b_neural", "Shock Wave")
	require.NoError(t, RunGenerate(techDir, mapFile))

	err := RunApply(mapFile, techDir,
		filepath.Join(base, "jobs"),
		filepath.Join(base, "archetypes"),
		filepath.Join(base, "go"),
		filepath.Join(base, "migrations"),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collision")
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./cmd/rename-tech-ids/... -run TestRunApply -v
```

Expected: failure (RunApply not yet implemented)

- [ ] **Step 3: Replace the `RunApply` stub in `apply.go` with full implementation**

Replace the existing `RunApply` function in `cmd/rename-tech-ids/apply.go`:

```go
// RunApply reads the rename map at mapFile and applies all renames:
// 1. Renames tech YAML files and rewrites their id: fields.
// 2. Updates all id references in job and archetype YAML files.
// 3. Updates string literals in static_localizer.go and Go test files under goSourceDir.
// 4. Emits DB migration files to migrationsDir.
//
// Refuses to run if any entry has Collision=true.
// Runs a post-apply validation pass via technology.Load().
//
// Precondition: mapFile exists and contains a valid RenameMap.
// Postcondition: all named files updated; migration files written; Registry loads cleanly.
func RunApply(mapFile, techDir, jobDir, archetypeDir, goSourceDir, migrationsDir string) error {
	data, err := os.ReadFile(mapFile)
	if err != nil {
		return fmt.Errorf("reading map file %q: %w", mapFile, err)
	}
	var rm RenameMap
	if err := yaml.Unmarshal(data, &rm); err != nil {
		return fmt.Errorf("parsing map file: %w", err)
	}

	// Refuse if any collision is unresolved.
	for _, e := range rm.Renames {
		if e.Collision {
			return fmt.Errorf("collision: new_id %q is derived by multiple old IDs — resolve in %s before applying", e.NewID, mapFile)
		}
	}

	// Build lookup: old_id → new_id (only non-skip entries).
	renameMap := make(map[string]string)
	for _, e := range rm.Renames {
		if !e.Skip {
			renameMap[e.OldID] = e.NewID
		}
	}

	// Step 1: Rename tech YAML files and rewrite id: fields.
	for _, e := range rm.Renames {
		if _, err := renameYAMLFile(e); err != nil {
			return fmt.Errorf("renaming YAML for %q: %w", e.OldID, err)
		}
	}

	// Step 2: Update job and archetype YAML references.
	for _, dir := range []string{jobDir, archetypeDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading dir %q: %w", dir, err)
		}
		for _, de := range entries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(dir, de.Name())
			if err := updateFileReferences(path, renameMap); err != nil {
				return fmt.Errorf("updating refs in %q: %w", path, err)
			}
		}
	}

	// Step 3: Update Go string literals.
	goFiles, err := collectGoFiles(goSourceDir)
	if err != nil {
		return fmt.Errorf("collecting Go files: %w", err)
	}
	for _, gf := range goFiles {
		if err := updateGoStringLiterals(gf, renameMap); err != nil {
			return fmt.Errorf("updating Go literals in %q: %w", gf, err)
		}
	}

	// Step 4: Emit DB migration.
	upFile := filepath.Join(migrationsDir, "058_rename_tech_ids.up.sql")
	downFile := filepath.Join(migrationsDir, "058_rename_tech_ids.down.sql")
	if err := emitMigration(rm.Renames, upFile, downFile); err != nil {
		return fmt.Errorf("emitting migration: %w", err)
	}

	return nil
}

// collectGoFiles returns all .go files under dir (recursive).
func collectGoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
```

Also add the yaml import to apply.go:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)
```

And remove the unused `var _ = strings.Contains` line from the stub.

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/rename-tech-ids/... -run TestRunApply -v
```

Expected: all PASS

- [ ] **Step 5: Run all tool tests**

```bash
go test ./cmd/rename-tech-ids/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/rename-tech-ids/apply.go cmd/rename-tech-ids/apply_test.go
git commit -m "feat(rename-tech-ids): full RunApply implementation with collision guard"
```

---

### Task 10: CLI flag wiring, idempotency test, and Makefile entry

**Files:**
- Modify: `cmd/rename-tech-ids/main.go`
- Modify: `cmd/rename-tech-ids/apply_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Write idempotency test**

Append to `cmd/rename-tech-ids/apply_test.go`:

```go
func TestRunApply_Idempotent(t *testing.T) {
	techDir, jobDir, archetypeDir, migrationsDir, goSourceDir, mapFile :=
		buildApplyFixture(t)

	// First apply
	require.NoError(t, RunApply(mapFile, techDir, jobDir, archetypeDir, goSourceDir, migrationsDir))

	// Re-generate map after first apply (ids are now correct, all skip=true)
	require.NoError(t, RunGenerate(techDir, mapFile))

	// Second apply must be a no-op (no errors, no changes to already-renamed files)
	require.NoError(t, RunApply(mapFile, techDir, jobDir, archetypeDir, goSourceDir, migrationsDir))

	// tech YAML still has correct id
	newTechPath := filepath.Join(techDir, "technical", "corrosive_projectile.yaml")
	data, err := os.ReadFile(newTechPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: corrosive_projectile")
}
```

- [ ] **Step 2: Run test — expect pass (idempotency should already work)**

```bash
go test ./cmd/rename-tech-ids/... -run TestRunApply_Idempotent -v
```

Expected: PASS

- [ ] **Step 3: Wire CLI flags in `main.go`**

Replace `cmd/rename-tech-ids/main.go` entirely:

```go
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("rename-tech-ids", flag.ContinueOnError)

	generate := fs.Bool("generate", false, "scan tech YAMLs and write tools/rename_map.yaml")
	apply := fs.Bool("apply", false, "apply tools/rename_map.yaml to all files and emit DB migration")

	techDir := fs.String("tech-dir", "content/technologies", "path to technology YAML directory")
	jobDir := fs.String("job-dir", "content/jobs", "path to job YAML directory")
	archetypeDir := fs.String("archetype-dir", "content/archetypes", "path to archetype YAML directory")
	goSourceDir := fs.String("go-source-dir", "internal/importer", "path to Go source directory for string literal updates")
	migrationsDir := fs.String("migrations-dir", "migrations", "path to migrations directory")
	mapFile := fs.String("map-file", "tools/rename_map.yaml", "path to rename map file")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*generate && !*apply {
		return errors.New("usage: rename-tech-ids --generate | --apply  (see --help for flags)")
	}
	if *generate && *apply {
		return errors.New("--generate and --apply are mutually exclusive")
	}

	if *generate {
		if err := RunGenerate(*techDir, *mapFile); err != nil {
			return err
		}
		fmt.Printf("rename map written to %s\n", *mapFile)
		fmt.Println("Review the map (especially pf2e_flag=true entries), then run --apply.")
		return nil
	}

	// --apply
	if err := RunApply(*mapFile, *techDir, *jobDir, *archetypeDir, *goSourceDir, *migrationsDir); err != nil {
		return err
	}
	fmt.Println("Apply complete. Run the DB migration:")
	fmt.Printf("  %s/058_rename_tech_ids.up.sql\n", *migrationsDir)
	return nil
}
```

- [ ] **Step 4: Add Makefile target**

In `Makefile`, find the `build-import-content:` block and add after it:

```makefile
build-rename-tech-ids:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/rename-tech-ids ./cmd/rename-tech-ids
```

Also add `build-rename-tech-ids` to the `build:` target list.

- [ ] **Step 5: Build and smoke-test**

```bash
go build -o /tmp/rename-tech-ids ./cmd/rename-tech-ids && /tmp/rename-tech-ids --help
```

Expected: prints usage with `--generate` / `--apply` flags listed.

- [ ] **Step 6: Run all tests**

```bash
go test ./cmd/rename-tech-ids/... -v
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/rename-tech-ids/main.go cmd/rename-tech-ids/apply_test.go Makefile
git commit -m "feat(rename-tech-ids): wire CLI flags --generate/--apply; add Makefile target"
```

---

### Task 11: Human review — run generate and fix PF2E names

This task is a manual step. The CLI is now built; the engineer runs it, reviews the output, fixes any remaining PF2E-sourced `name` fields in source YAML files, then runs apply.

- [ ] **Step 1: Run generate**

From the repo root:

```bash
go run ./cmd/rename-tech-ids --generate
```

Expected output: `rename map written to tools/rename_map.yaml`

- [ ] **Step 2: Review flagged entries**

```bash
grep -A2 "pf2e_flag: true" tools/rename_map.yaml | head -60
```

For each `pf2e_flag: true` entry, open the corresponding YAML file (the `file:` field in the map) and update the `name:` field to the Gunchete name. Then re-run generate to confirm the flag is cleared.

- [ ] **Step 3: Review collision entries (if any)**

```bash
grep -A2 "collision: true" tools/rename_map.yaml
```

If any exist, manually set distinct `new_id` values in `tools/rename_map.yaml` for each colliding pair, then clear `collision: false`.

- [ ] **Step 4: Confirm no blocking issues remain**

```bash
grep "collision: true" tools/rename_map.yaml
```

Expected: no output (zero collisions).

- [ ] **Step 5: Commit updated YAML names and rename map**

```bash
git add content/technologies/ tools/rename_map.yaml
git commit -m "content(technology): localize remaining PF2E-flagged technology names to Gunchete"
```

---

### Task 12: Run apply and deploy migration

- [ ] **Step 1: Run apply**

```bash
go run ./cmd/rename-tech-ids --apply
```

Expected:
```
Apply complete. Run the DB migration:
  migrations/058_rename_tech_ids.up.sql
```

- [ ] **Step 2: Verify tech Registry loads cleanly**

```bash
go test ./internal/game/technology/... -v -run TestRegistry
```

Expected: all PASS (the full real content directory loads without errors)

- [ ] **Step 3: Run full test suite**

```bash
go test -race -count=1 -timeout=300s $(go list ./... | grep -v "postgres\|e2e")
```

Expected: all PASS

- [ ] **Step 4: Commit all apply output**

```bash
git add content/technologies/ content/jobs/ content/archetypes/ \
        internal/importer/static_localizer.go \
        migrations/058_rename_tech_ids.up.sql \
        migrations/058_rename_tech_ids.down.sql \
        tools/rename_map.yaml
git commit -m "feat(technology): rename all tech IDs to snake_case Gunchete names

Renames 2,425 technology IDs from PF2E-sourced names (e.g. acid_arrow_technical)
to snake_case Gunchete names (e.g. corrosive_projectile). Includes DB migration
058 for all four character tech tables."
```

- [ ] **Step 5: Run DB migration**

```bash
make migrate-up
```

Expected: migration 058 applies cleanly.

- [ ] **Step 6: Deploy**

```bash
make k8s-redeploy
```

---

## Self-Review

**Spec coverage check:**

| Requirement | Task |
|-------------|------|
| REQ-TIR-1: all 2,425 id fields updated | Task 12 (apply) |
| REQ-TIR-2: tradition suffixes dropped | Task 1 (ToSnakeCase strips them via IsPF2EFlagged, and derivation uses name not old_id) |
| REQ-TIR-3: YAML filenames renamed | Task 5 (renameYAMLFile) |
| REQ-TIR-4: job/archetype refs updated | Task 6 (updateFileReferences) |
| REQ-TIR-5: Go string literals updated | Task 7 (updateGoStringLiterals) |
| REQ-TIR-6: DB migration emitted | Task 8 (emitMigration) |
| REQ-TIR-7: PF2E names flagged | Task 2 (IsPF2EFlagged) + Task 11 (human review) |
| REQ-TIR-8: already-correct IDs skipped | Task 3 (Skip detection in BuildRenameMap) |
| REQ-TIR-T1: ToSnakeCase edge cases | Task 1 |
| REQ-TIR-T2: PF2E flag heuristic | Task 2 |
| REQ-TIR-T3: collision detection | Task 3 |
| REQ-TIR-T4: no-op detection | Task 3 |
| REQ-TIR-T5: apply idempotency | Task 10 |
| REQ-TIR-T6: validation pass failure | Covered by TestRunApply_RefusesOnUnresolvedCollision in Task 9 |
