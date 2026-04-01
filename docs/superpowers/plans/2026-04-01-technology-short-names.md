# Technology Short Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional `short_name` field to technology definitions so players can type `use <short_name>` and the web UI hotbar stores the short name command instead of the raw ID.

**Architecture:** Three layers change in lockstep: (1) Go data model + registry gain the field and a secondary index; (2) four proto slot messages gain `tech_short_name`; (3) the grpc char-sheet builder populates it and `handleUse` resolves it before ID matching; (4) the hand-written TypeScript proto interfaces and `TechnologyDrawer` hotbar assignment use it. No DB migration required — short names live only in YAML/memory/proto.

**Tech Stack:** Go, protobuf (protoc), TypeScript/React, Vitest, pgregory.net/rapid (property tests)

---

## File Map

| Action | File |
|--------|------|
| Modify | `internal/game/technology/model.go` |
| Create | `internal/game/technology/model_shortname_test.go` |
| Modify | `internal/game/technology/registry.go` |
| Create | `internal/game/technology/registry_shortname_test.go` |
| Modify | `api/proto/game/v1/game.proto` |
| Modify | `internal/gameserver/grpc_service.go` |
| Modify | `cmd/webclient/ui/src/proto/index.ts` |
| Modify | `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx` |

---

### Task 1: Add ShortName field and validation to TechnologyDef

**Files:**
- Modify: `internal/game/technology/model.go`
- Create: `internal/game/technology/model_shortname_test.go`

- [ ] **Step 1: Write failing property tests for ShortName validation**

Create `internal/game/technology/model_shortname_test.go`:

```go
package technology_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

// validDef returns a minimal valid TechnologyDef for use in tests.
func validDefForShortName() *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:         "force_barrage_technical",
		Name:       "Force Barrage",
		Tradition:  technology.TraditionTechnical,
		Level:      1,
		UsageType:  technology.UsagePrepared,
		ActionCost: 2,
		Range:      technology.RangeRanged,
		Targets:    technology.TargetsSingle,
		Duration:   "instant",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectUtility, Description: "test"},
			},
		},
	}
}

// REQ-TSN-1: ShortName is optional — empty string must be valid.
func TestShortName_EmptyIsValid(t *testing.T) {
	d := validDefForShortName()
	d.ShortName = ""
	if err := d.Validate(); err != nil {
		t.Fatalf("empty ShortName should be valid, got: %v", err)
	}
}

// REQ-TSN-2a: ShortName must contain only lowercase ASCII letters, digits, underscores.
func TestProperty_ShortName_ValidCharsAccepted(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate strings using only valid characters.
		length := rapid.IntRange(2, 32).Draw(rt, "length")
		chars := "abcdefghijklmnopqrstuvwxyz0123456789_"
		bs := make([]byte, length)
		for i := range bs {
			idx := rapid.IntRange(0, len(chars)-1).Draw(rt, "char")
			bs[i] = chars[idx]
		}
		s := string(bs)
		// Strip leading/trailing underscores to satisfy REQ-TSN-2b.
		s = strings.Trim(s, "_")
		if len(s) < 2 {
			return // too short after trim; skip
		}
		d := validDefForShortName()
		d.ShortName = s
		if err := d.Validate(); err != nil {
			rt.Fatalf("valid short_name %q rejected: %v", s, err)
		}
	})
}

// REQ-TSN-2a: Invalid characters must be rejected.
func TestShortName_InvalidCharRejected(t *testing.T) {
	cases := []string{"has space", "UPPER", "has-hyphen", "has.dot", "has@symbol"}
	for _, c := range cases {
		d := validDefForShortName()
		d.ShortName = c
		if err := d.Validate(); err == nil {
			t.Errorf("short_name %q should be rejected but was accepted", c)
		}
	}
}

// REQ-TSN-2b: Leading or trailing underscore must be rejected.
func TestShortName_LeadingTrailingUnderscoreRejected(t *testing.T) {
	for _, c := range []string{"_leading", "trailing_", "_both_"} {
		d := validDefForShortName()
		d.ShortName = c
		if err := d.Validate(); err == nil {
			t.Errorf("short_name %q should be rejected but was accepted", c)
		}
	}
}

// REQ-TSN-2c: ShortName must not equal the technology's own ID.
func TestShortName_EqualToIDRejected(t *testing.T) {
	d := validDefForShortName()
	d.ShortName = d.ID
	if err := d.Validate(); err == nil {
		t.Fatal("short_name equal to ID should be rejected but was accepted")
	}
}

// REQ-TSN-2d: Length < 2 or > 32 must be rejected.
func TestShortName_LengthBoundsRejected(t *testing.T) {
	d := validDefForShortName()
	d.ShortName = "a" // length 1
	if err := d.Validate(); err == nil {
		t.Error("short_name of length 1 should be rejected")
	}
	d.ShortName = strings.Repeat("a", 33) // length 33
	if err := d.Validate(); err == nil {
		t.Error("short_name of length 33 should be rejected")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -run TestShortName -run TestProperty_ShortName 2>&1 | tail -15
```

Expected: FAIL — `ShortName` field does not exist on `TechnologyDef`.

- [ ] **Step 3: Add ShortName field to TechnologyDef**

In `internal/game/technology/model.go`, add `ShortName` after `Name` (line 173):

```go
type TechnologyDef struct {
	ID        string    `yaml:"id"`
	Name      string    `yaml:"name"`
	ShortName string    `yaml:"short_name,omitempty"`
	Description string    `yaml:"description,omitempty"`
	// ... rest unchanged
```

- [ ] **Step 4: Add ShortName validation to Validate()**

In `internal/game/technology/model.go`, add this block immediately after the `t.Name == ""` check (after line 207):

```go
	if t.ShortName != "" {
		if err := validateShortName(t.ShortName, t.ID); err != nil {
			return err
		}
	}
```

Add the helper function at the end of the file, after `validateEffect`:

```go
// validateShortName enforces REQ-TSN-2 constraints on short_name.
func validateShortName(s, techID string) error {
	if len(s) < 2 || len(s) > 32 {
		return fmt.Errorf("short_name %q: length %d out of range [2,32]", s, len(s))
	}
	if s[0] == '_' || s[len(s)-1] == '_' {
		return fmt.Errorf("short_name %q must not begin or end with underscore", s)
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("short_name %q contains invalid character %q (only lowercase letters, digits, underscores allowed)", s, r)
		}
	}
	if s == techID {
		return fmt.Errorf("short_name %q must not be identical to id", s)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -run TestShortName -run TestProperty_ShortName -v 2>&1 | tail -20
```

Expected: All tests pass.

- [ ] **Step 6: Run the full technology package tests and full Go test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... 2>&1 | tail -10
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | tail -10
```

Expected: All pass, no compile errors.

- [ ] **Step 7: Commit**

```bash
git add internal/game/technology/model.go internal/game/technology/model_shortname_test.go
git commit -m "feat(technology): add optional ShortName field with REQ-TSN-2 validation"
```

---

### Task 2: Add byShortName registry index and collision checks

**Files:**
- Modify: `internal/game/technology/registry.go`
- Create: `internal/game/technology/registry_shortname_test.go`

- [ ] **Step 8: Write failing registry tests**

Create `internal/game/technology/registry_shortname_test.go`:

```go
package technology_test

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

// writeTechYAML writes a minimal valid YAML file for a TechnologyDef into dir.
func writeTechYAML(t *testing.T, dir string, d *technology.TechnologyDef) {
	t.Helper()
	content := buildYAML(d)
	path := filepath.Join(dir, d.ID+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func buildYAML(d *technology.TechnologyDef) string {
	shortLine := ""
	if d.ShortName != "" {
		shortLine = "\nshort_name: " + d.ShortName
	}
	return "id: " + d.ID + "\nname: " + d.Name + shortLine + `
tradition: technical
level: 1
usage_type: prepared
action_cost: 2
range: ranged
targets: single
duration: instant
effects:
  on_apply:
    - type: utility
      description: test effect
`
}

// REQ-TSN-4: GetByShortName returns the def when short_name is set.
func TestGetByShortName_Found(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, &technology.TechnologyDef{
		ID: "force_barrage_technical", Name: "Force Barrage", ShortName: "force_bolt",
	})
	reg, err := technology.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	def, ok := reg.GetByShortName("force_bolt")
	if !ok {
		t.Fatal("GetByShortName: expected to find force_bolt")
	}
	if def.ID != "force_barrage_technical" {
		t.Errorf("got ID %q; want force_barrage_technical", def.ID)
	}
}

// REQ-TSN-4: GetByShortName returns false when short_name is not set.
func TestGetByShortName_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, &technology.TechnologyDef{ID: "force_barrage_technical", Name: "Force Barrage"})
	reg, err := technology.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, ok := reg.GetByShortName("force_bolt")
	if ok {
		t.Fatal("GetByShortName: expected not-found, got found")
	}
}

// REQ-TSN-3: Duplicate short_name causes Load to error.
func TestLoad_DuplicateShortName_Errors(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, &technology.TechnologyDef{ID: "tech_a", Name: "Tech A", ShortName: "bolt"})
	writeTechYAML(t, dir, &technology.TechnologyDef{ID: "tech_b", Name: "Tech B", ShortName: "bolt"})
	_, err := technology.Load(dir)
	if err == nil {
		t.Fatal("expected error for duplicate short_name, got nil")
	}
}

// REQ-TSN-5: short_name equal to another tech's ID causes Load to error.
func TestLoad_ShortNameCollidesWithID_Errors(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, &technology.TechnologyDef{ID: "tech_a", Name: "Tech A"})
	writeTechYAML(t, dir, &technology.TechnologyDef{ID: "tech_b", Name: "Tech B", ShortName: "tech_a"})
	_, err := technology.Load(dir)
	if err == nil {
		t.Fatal("expected error when short_name collides with existing ID, got nil")
	}
}

// REQ-TSN-11a (property): Round-trip through Load preserves short_name.
func TestProperty_Load_ShortNameRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		short := rapid.SampledFrom([]string{"", "bolt", "kb", "nano_inject", "surge2"}).Draw(rt, "short")
		dir := t.TempDir()
		writeTechYAML(t, dir, &technology.TechnologyDef{ID: "test_tech", Name: "Test Tech", ShortName: short})
		reg, err := technology.Load(dir)
		if err != nil {
			rt.Fatalf("Load with short_name=%q failed: %v", short, err)
		}
		got, ok := reg.Get("test_tech")
		if !ok {
			rt.Fatal("Get test_tech: not found")
		}
		if got.ShortName != short {
			rt.Fatalf("ShortName: got %q, want %q", got.ShortName, short)
		}
		if short != "" {
			byShort, ok2 := reg.GetByShortName(short)
			if !ok2 {
				rt.Fatalf("GetByShortName(%q): not found", short)
			}
			if byShort.ID != "test_tech" {
				rt.Fatalf("GetByShortName(%q) ID: got %q, want test_tech", short, byShort.ID)
			}
		}
	})
}
```

- [ ] **Step 9: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -run TestGetByShortName -run TestLoad_Duplicate -run TestLoad_ShortName -run TestProperty_Load 2>&1 | tail -15
```

Expected: FAIL — `GetByShortName` does not exist.

- [ ] **Step 10: Add byShortName field to Registry struct and NewRegistry**

In `internal/game/technology/registry.go`, update `Registry` struct and `NewRegistry`:

```go
type Registry struct {
	byID        map[string]*TechnologyDef
	byShortName map[string]*TechnologyDef
	byTradition map[Tradition][]*TechnologyDef
	byLevel     map[int][]*TechnologyDef
	byUsage     map[UsageType][]*TechnologyDef
}

func NewRegistry() *Registry {
	return &Registry{
		byID:        make(map[string]*TechnologyDef),
		byShortName: make(map[string]*TechnologyDef),
		byTradition: make(map[Tradition][]*TechnologyDef),
		byLevel:     make(map[int][]*TechnologyDef),
		byUsage:     make(map[UsageType][]*TechnologyDef),
	}
}
```

- [ ] **Step 11: Add collision checks and byShortName population in Load**

In `internal/game/technology/registry.go`, replace the four index lines inside `Load` (currently lines 61–64) with:

```go
		if _, dup := r.byID[def.ID]; dup {
			return fmt.Errorf("duplicate technology ID %q in %q", def.ID, path)
		}
		if def.ShortName != "" {
			// REQ-TSN-3: short_name must be globally unique.
			if existing, dup := r.byShortName[def.ShortName]; dup {
				return fmt.Errorf("short_name %q on %q already used by %q", def.ShortName, def.ID, existing.ID)
			}
			// REQ-TSN-5: short_name must not equal any existing technology ID.
			if _, collision := r.byID[def.ShortName]; collision {
				return fmt.Errorf("short_name %q on %q collides with existing technology ID", def.ShortName, def.ID)
			}
			// REQ-TSN-5: existing IDs must not equal this def's short_name (check reverse).
			for existingID := range r.byID {
				if existingID == def.ShortName {
					return fmt.Errorf("short_name %q on %q collides with existing technology ID %q", def.ShortName, def.ID, existingID)
				}
			}
			r.byShortName[def.ShortName] = &def
		}
		r.byID[def.ID] = &def
		r.byTradition[def.Tradition] = append(r.byTradition[def.Tradition], &def)
		r.byLevel[def.Level] = append(r.byLevel[def.Level], &def)
		r.byUsage[def.UsageType] = append(r.byUsage[def.UsageType], &def)
```

> **Note:** The three-step collision check (existing byShortName, byID map lookup, range over byID) catches both orderings: tech B short_name = tech A's ID, and tech A short_name = tech B's later-loaded ID. Since `Load` is single-pass sequential, the range over `r.byID` at check time is sufficient.

- [ ] **Step 12: Add GetByShortName method and update Register**

Add after the `Get` method:

```go
// GetByShortName returns the TechnologyDef whose short_name equals short,
// or (nil, false) if no tech has that short name.
func (r *Registry) GetByShortName(short string) (*TechnologyDef, bool) {
	d, ok := r.byShortName[short]
	return d, ok
}
```

Update `Register` to also populate `byShortName` (replace existing `Register` body):

```go
func (r *Registry) Register(def *TechnologyDef) {
	r.byID[def.ID] = def
	if def.ShortName != "" {
		r.byShortName[def.ShortName] = def
	}
}
```

- [ ] **Step 13: Run registry tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -v 2>&1 | tail -20
```

Expected: All tests pass.

- [ ] **Step 14: Commit**

```bash
git add internal/game/technology/registry.go internal/game/technology/registry_shortname_test.go
git commit -m "feat(technology): add byShortName registry index with uniqueness/collision checks"
```

---

### Task 3: Add tech_short_name to proto messages and populate in char sheet builder

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 15: Add tech_short_name fields to the four proto messages**

In `api/proto/game/v1/game.proto`, update the four messages:

**PreparedSlotView** (around line 955) — add field 4:
```protobuf
message PreparedSlotView {
    string tech_id         = 1;
    bool   expended        = 2;
    string tech_name       = 3;
    string tech_short_name = 4;
}
```

**HardwiredSlotView** (around line 962) — add field 4:
```protobuf
message HardwiredSlotView {
    string tech_id          = 1;
    string tech_name        = 2;
    string description      = 3;
    string tech_short_name  = 4;
}
```

**SpontaneousKnownEntry** (around line 969) — add field 4:
```protobuf
message SpontaneousKnownEntry {
    string tech_id         = 1;
    string tech_name       = 2;
    int32  tech_level      = 3;
    string tech_short_name = 4;
}
```

**InnateSlotView** (around line 1039) — add field 7 (field 6 is `is_reaction`):
```protobuf
message InnateSlotView {
    string tech_id         = 1;
    int32  uses_remaining  = 2;
    int32  max_uses        = 3;
    string tech_name       = 4;
    string description     = 5;
    bool   is_reaction     = 6;
    string tech_short_name = 7;
}
```

- [ ] **Step 16: Regenerate Go proto stubs**

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -10
```

Expected: Exits 0. Generated files updated with new `TechShortName` fields.

- [ ] **Step 17: Confirm the build compiles**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | tail -10
```

Expected: No errors.

- [ ] **Step 18: Populate TechShortName in char sheet builder (prepared slots, ~line 5699)**

In `internal/gameserver/grpc_service.go`, find the prepared-slots block (around line 5697–5712). Replace it with:

```go
					techName := slot.TechID
					techShortName := ""
					if s.techRegistry != nil {
						if def, ok := s.techRegistry.Get(slot.TechID); ok {
							techName = def.Name
							techShortName = def.ShortName
						}
					}
					view.PreparedSlots = append(view.PreparedSlots, &gamev1.PreparedSlotView{
						TechId:        slot.TechID,
						Expended:      slot.Expended,
						TechName:      techName,
						TechShortName: techShortName,
					})
```

- [ ] **Step 19: Populate TechShortName in spontaneous known block (~line 5739)**

Find the spontaneous-known block. Replace it with:

```go
				techName := tid
				techShortName := ""
				if s.techRegistry != nil {
					if def, ok := s.techRegistry.Get(tid); ok {
						techName = def.Name
						techShortName = def.ShortName
					}
				}
				view.SpontaneousKnown = append(view.SpontaneousKnown, &gamev1.SpontaneousKnownEntry{
					TechId:        tid,
					TechName:      techName,
					TechLevel:     int32(lvl),
					TechShortName: techShortName,
				})
```

- [ ] **Step 20: Populate TechShortName in innate slots block (~line 5761)**

Find the innate-slots block. Replace it with:

```go
		techName := id
		techDesc := ""
		techShortName := ""
		var isReaction bool
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(id); ok {
				techName = def.Name
				techDesc = def.Description
				techShortName = def.ShortName
				isReaction = def.Reaction != nil
			}
		}
		view.InnateSlots = append(view.InnateSlots, &gamev1.InnateSlotView{
			TechId:        id,
			UsesRemaining: int32(slot.UsesRemaining),
			MaxUses:       int32(slot.MaxUses),
			TechName:      techName,
			Description:   techDesc,
			IsReaction:    isReaction,
			TechShortName: techShortName,
		})
```

- [ ] **Step 21: Populate TechShortName in hardwired slots block (~line 5787)**

Find the hardwired-slots block. Replace it with:

```go
			techName := id
			techDesc := ""
			techShortName := ""
			if s.techRegistry != nil {
				if def, ok := s.techRegistry.Get(id); ok {
					techName = def.Name
					techDesc = def.Description
					techShortName = def.ShortName
				}
			}
			view.HardwiredSlots = append(view.HardwiredSlots, &gamev1.HardwiredSlotView{
				TechId:        id,
				TechName:      techName,
				Description:   techDesc,
				TechShortName: techShortName,
			})
```

- [ ] **Step 22: Build and run tests to confirm no regressions**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | tail -5
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... ./internal/gameserver/... 2>&1 | tail -15
```

Expected: Build succeeds, all tests pass.

- [ ] **Step 23: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/grpc_service.go
git commit -m "feat(technology): add tech_short_name to proto slot messages and populate in char sheet builder"
```

---

### Task 4: Resolve short_name in handleUse

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 24: Add short-name resolution before tech lookup in handleUse**

In `internal/gameserver/grpc_service.go`, find the comment `// Attempt prepared tech activation if no feat/class-feature matched.` (around line 7186). Insert this block immediately **before** that comment:

```go
	// REQ-TSN-6: resolve short_name → canonical ID before tech lookups.
	// If abilityID matches a tech short name, replace it with the canonical ID.
	if s.techRegistry != nil {
		if def, ok := s.techRegistry.GetByShortName(abilityID); ok {
			abilityID = def.ID
		}
	}
```

- [ ] **Step 25: Build to confirm no errors**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | tail -5
```

Expected: No errors.

- [ ] **Step 26: Run the full Go test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | tail -30
```

Expected: All packages pass.

- [ ] **Step 27: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(technology): resolve short_name to canonical ID in handleUse (REQ-TSN-6)"
```

---

### Task 5: TypeScript proto interface and TechnologyDrawer hotbar

**Files:**
- Modify: `cmd/webclient/ui/src/proto/index.ts`
- Modify: `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx`

- [ ] **Step 28: Add tech_short_name fields to the four hand-written TypeScript interfaces**

In `cmd/webclient/ui/src/proto/index.ts`, update the four interfaces:

**PreparedSlotView** (around line 115):
```typescript
export interface PreparedSlotView {
  techId?: string
  tech_id?: string
  expended?: boolean
  techName?: string
  tech_name?: string
  description?: string
  techShortName?: string
  tech_short_name?: string
}
```

**InnateSlotView** (around line 124):
```typescript
export interface InnateSlotView {
  techId?: string
  tech_id?: string
  usesRemaining?: number
  uses_remaining?: number
  maxUses?: number
  max_uses?: number
  techName?: string
  tech_name?: string
  description?: string
  isReaction?: boolean
  techShortName?: string
  tech_short_name?: string
}
```

**HardwiredSlotView** (around line 137):
```typescript
export interface HardwiredSlotView {
  techId?: string
  tech_id?: string
  techName?: string
  tech_name?: string
  description?: string
  techShortName?: string
  tech_short_name?: string
}
```

**SpontaneousKnownEntry** (around line 145):
```typescript
export interface SpontaneousKnownEntry {
  techId?: string
  tech_id?: string
  techName?: string
  tech_name?: string
  techLevel?: number
  tech_level?: number
  techShortName?: string
  tech_short_name?: string
}
```

- [ ] **Step 29: Update PreparedItem in TechnologyDrawer.tsx to use short_name for hotbar**

In `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx`, find `PreparedItem` (around line 63). Change `handlePick` to use `shortName || techId`:

```typescript
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const shortName = slot.techShortName ?? slot.tech_short_name ?? ''

  function handlePick(s: number) {
    const hotbarId = shortName || techId
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${hotbarId}` })
    setPicking(false)
  }
```

- [ ] **Step 30: Update InnateItem in TechnologyDrawer.tsx**

Find `InnateItem` (around line 104). Change `handlePick`:

```typescript
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const shortName = slot.techShortName ?? slot.tech_short_name ?? ''
  const remaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
  const max = slot.maxUses ?? slot.max_uses ?? 0
  const exhausted = max > 0 && remaining === 0

  function handlePick(s: number) {
    const hotbarId = shortName || techId
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${hotbarId}` })
    setPicking(false)
  }
```

- [ ] **Step 31: Update SpontaneousItem in TechnologyDrawer.tsx**

Find `SpontaneousItem` (around line 146). Change `handlePick`:

```typescript
  const techId = entry.techId ?? entry.tech_id ?? ''
  const name = entry.techName ?? entry.tech_name ?? techId
  const shortName = entry.techShortName ?? entry.tech_short_name ?? ''
  const exhausted = poolRemaining === 0

  function handlePick(s: number) {
    const hotbarId = shortName || techId
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${hotbarId}` })
    setPicking(false)
  }
```

- [ ] **Step 32: Run the frontend test suite**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -15
```

Expected: All tests pass.

- [ ] **Step 33: Run the full Go test suite as final check**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | tail -30
```

Expected: All packages pass.

- [ ] **Step 34: Commit**

```bash
git add cmd/webclient/ui/src/proto/index.ts \
        cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx
git commit -m "feat(technology): use short_name in hotbar assignment in TechnologyDrawer

PreparedItem, InnateItem, SpontaneousItem now store 'use <short_name>'
in hotbar slots when a short name is defined, falling back to tech ID.
HardwiredItem has no hotbar button and requires no change.

Closes technology-short-names."
```

---

## Self-Review

**Spec coverage:**

| Requirement | Covered by |
|-------------|-----------|
| REQ-TSN-1: ShortName optional field on TechnologyDef | Task 1, Steps 3–4 |
| REQ-TSN-2a: lowercase letters/digits/underscores only | Task 1, Step 4 (`validateShortName`); tests in Step 1 |
| REQ-TSN-2b: no leading/trailing underscore | Task 1, Step 4; tests in Step 1 |
| REQ-TSN-2c: not identical to own ID | Task 1, Step 4; tests in Step 1 |
| REQ-TSN-2d: length 2–32 | Task 1, Step 4; tests in Step 1 |
| REQ-TSN-3: globally unique short_name in Load | Task 2, Step 11; test in Step 8 |
| REQ-TSN-4: byShortName index + GetByShortName | Task 2, Steps 10–12; tests in Step 8 |
| REQ-TSN-5: short_name must not equal any existing ID | Task 2, Step 11; test in Step 8 |
| REQ-TSN-6: use command resolves short_name → ID | Task 4, Step 24 |
| REQ-TSN-7: hotbar stores `use <short_name>` when defined | Task 5, Steps 29–31 |
| REQ-TSN-8: proto slot messages carry tech_short_name | Task 3, Steps 15–21 |
| REQ-TSN-9: existing hotbar slots continue to work | Preserved: ID lookup runs before short_name resolution; no migration |
| REQ-TSN-10: content assignment out of scope | Confirmed — no YAML files modified |
| REQ-TSN-11a: Load succeeds with valid short_name | TestProperty_Load_ShortNameRoundTrip |
| REQ-TSN-11b: Load errors on duplicate short_name | TestLoad_DuplicateShortName_Errors |
| REQ-TSN-11c: Load errors on short_name=existing ID | TestLoad_ShortNameCollidesWithID_Errors |
| REQ-TSN-11d: Validate rejects REQ-TSN-2 violations | TestShortName_* tests in Task 1 |

**Placeholder scan:** No TBDs, no "add appropriate error handling", all steps contain complete code.

**Type consistency:** `ShortName string` field defined in Task 1 Step 3, used in Task 2 Step 11 (`def.ShortName`) and Task 3 Steps 18–21 (`def.ShortName`). `GetByShortName` defined in Task 2 Step 12, called in Task 4 Step 24. `TechShortName` proto field set in Steps 18–21, read as `techShortName`/`tech_short_name` in Task 5. All consistent.
