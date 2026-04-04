# Technology Short Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional `short_name` field to technology definitions so players can type `use <short_name>` instead of `use <full_tech_id>`, and the web UI stores the shorter command in hotbar slots.

**Architecture:** `ShortName` is added to `TechnologyDef` with format validation in `Validate()`. The `Registry` maintains a `byShortName` secondary index populated in a two-pass `Load()` that checks both uniqueness and collision with other tech IDs. `handleUse()` resolves short names once at function entry, normalising `abilityID` to the canonical ID before all existing lookup paths. The four proto slot view messages each gain a `short_name` field populated in `buildCharacterSheet`. The web UI's three `handlePick` functions prefer `shortName` over `techId` when building the hotbar command text.

**Tech Stack:** Go 1.23, `pgregory.net/rapid` property-based tests, Protocol Buffers / protoc, TypeScript / React

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/game/technology/model.go` | Modify | Add `ShortName` field; add format validation in `Validate()` |
| `internal/game/technology/model_test.go` | Modify | Property tests for `Validate()` short-name constraints |
| `internal/game/technology/registry.go` | Modify | Add `byShortName` index; `GetByShortName()`; two-pass `Load()`; update `Register()` |
| `internal/game/technology/registry_test.go` | Modify | Load success + uniqueness + ID-collision + index tests |
| `api/proto/game/v1/game.proto` | Modify | Add `short_name` to four slot view messages |
| `internal/gameserver/gamev1/game.pb.go` | Regenerate | `make proto` |
| `cmd/webclient/ui/src/proto.ts` | Regenerate | `make proto-ts` |
| `internal/gameserver/grpc_service.go` | Modify | Populate `ShortName` in `buildCharacterSheet`; resolve short name in `handleUse()` |
| `internal/gameserver/grpc_service_tsn_test.go` | Create | `handleUse` short-name resolution test |
| `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx` | Modify | Three `handlePick` functions prefer `shortName || techId` |

---

### Task 1: TechnologyDef ShortName Field and Validation

**Files:**
- Modify: `internal/game/technology/model.go`
- Modify: `internal/game/technology/model_test.go`

- [ ] **Step 1: Write failing tests for Validate() short-name constraints**

Add to `internal/game/technology/model_test.go`:

```go
import "regexp"

// REQ-TSN-2: Validate() enforces short_name format constraints.
func TestValidate_ShortName_InvalidCases(t *testing.T) {
    cases := []struct {
        name      string
        shortName string
    }{
        {"too_short", "a"},
        {"too_long", strings.Repeat("a", 33)},
        {"uppercase", "NeuralShock"},
        {"space", "neural shock"},
        {"leading_underscore", "_neural"},
        {"trailing_underscore", "neural_"},
        {"same_as_id", "test-tech"},
        {"dash_disallowed", "neural-shock"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            def := validDef()
            def.ShortName = tc.shortName
            err := def.Validate()
            assert.Error(t, err, "expected error for short_name %q", tc.shortName)
        })
    }
}

// REQ-TSN-11d: property test — Validate() accepts all well-formed short names.
func TestProperty_Validate_ShortName_ValidAccepted(t *testing.T) {
    validShortName := regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,30}[a-z0-9]$`)
    rapid.Check(t, func(rt *rapid.T) {
        sn := rapid.StringMatching(`[a-z0-9][a-z0-9_]{0,30}[a-z0-9]`).Draw(rt, "shortName")
        if !validShortName.MatchString(sn) {
            rt.Skip() // rapid occasionally generates edge cases; skip them
        }
        def := validDef()
        def.ShortName = sn
        def.ID = "different_id" // ensure ShortName != ID constraint passes
        err := def.Validate()
        assert.NoError(rt, err, "valid short_name %q should pass Validate()", sn)
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -run "TestValidate_ShortName|TestProperty_Validate_ShortName" -v 2>&1 | tail -20
```

Expected: compile error or FAIL (field doesn't exist yet).

- [ ] **Step 3: Add ShortName field and import regexp to model.go**

In `internal/game/technology/model.go`, add import `"regexp"` and package-level variable after existing package-level vars:

```go
var shortNameRE = regexp.MustCompile(`^[a-z0-9_]+$`)
```

Add `ShortName` field to `TechnologyDef` after the `ID` field:

```go
type TechnologyDef struct {
    ID        string    `yaml:"id"`
    ShortName string    `yaml:"short_name,omitempty"`
    Name      string    `yaml:"name"`
    // ... (rest unchanged)
```

- [ ] **Step 4: Add ShortName validation to Validate()**

In `Validate()`, after the `t.ID == ""` check, add:

```go
if t.ShortName != "" {
    if len(t.ShortName) < 2 || len(t.ShortName) > 32 {
        return fmt.Errorf("short_name %q must be between 2 and 32 characters", t.ShortName)
    }
    if !shortNameRE.MatchString(t.ShortName) {
        return fmt.Errorf("short_name %q must contain only lowercase letters, digits, and underscores", t.ShortName)
    }
    if strings.HasPrefix(t.ShortName, "_") || strings.HasSuffix(t.ShortName, "_") {
        return fmt.Errorf("short_name %q must not begin or end with an underscore", t.ShortName)
    }
    if t.ShortName == t.ID {
        return fmt.Errorf("short_name %q must not be identical to the technology id", t.ShortName)
    }
}
```

`strings` is already imported in `model.go`. Confirm with `grep '"strings"' internal/game/technology/model.go`; add import if missing.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/game/technology/... -run "TestValidate_ShortName|TestProperty_Validate_ShortName" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 6: Run full technology test suite**

```bash
go test ./internal/game/technology/... -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/game/technology/model.go internal/game/technology/model_test.go
git commit -m "feat(tsn): add ShortName field and Validate() constraints to TechnologyDef"
```

---

### Task 2: Registry byShortName Index and Collision Enforcement

**Files:**
- Modify: `internal/game/technology/registry.go`
- Modify: `internal/game/technology/registry_test.go`

- [ ] **Step 1: Write failing tests for registry short-name behavior**

Add to `internal/game/technology/registry_test.go`. These tests use `os.MkdirTemp` to write isolated YAML fixtures:

```go
import (
    "os"
    "path/filepath"
)

// helper: write a single YAML file to a temp dir
func writeTechYAML(t *testing.T, dir, filename, content string) {
    t.Helper()
    err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0600)
    require.NoError(t, err)
}

const baseTechYAML = `id: tech_alpha
name: Alpha Tech
tradition: technical
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: self
duration: instant
resolution: none
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`

// REQ-TSN-11a: Load() with valid short_name indexes the tech by short name.
func TestLoad_ShortName_IndexedCorrectly(t *testing.T) {
    dir := t.TempDir()
    writeTechYAML(t, dir, "alpha.yaml", baseTechYAML+`short_name: ta
`)
    reg, err := technology.Load(dir)
    require.NoError(t, err)
    def, ok := reg.GetByShortName("ta")
    require.True(t, ok)
    assert.Equal(t, "tech_alpha", def.ID)
}

// REQ-TSN-11b: Load() returns error on duplicate short names.
func TestLoad_ShortName_DuplicateReturnsError(t *testing.T) {
    dir := t.TempDir()
    writeTechYAML(t, dir, "alpha.yaml", baseTechYAML+`short_name: ta
`)
    writeTechYAML(t, dir, "beta.yaml", `id: tech_beta
name: Beta Tech
tradition: neural
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: self
duration: instant
resolution: none
short_name: ta
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`)
    _, err := technology.Load(dir)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "ta")
}

// REQ-TSN-11c: Load() returns error when short_name equals another tech's id.
func TestLoad_ShortName_CollidesWithOtherID_ReturnsError(t *testing.T) {
    dir := t.TempDir()
    writeTechYAML(t, dir, "alpha.yaml", baseTechYAML)
    writeTechYAML(t, dir, "beta.yaml", `id: tech_beta
name: Beta Tech
tradition: neural
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: self
duration: instant
resolution: none
short_name: tech_alpha
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`)
    _, err := technology.Load(dir)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "tech_alpha")
}

// REQ-TSN-11a (property): GetByShortName returns the correct def for any loaded short name.
func TestProperty_Registry_GetByShortName_RoundTrip(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        // Generate a valid short name distinct from the tech ID
        sn := rapid.StringMatching(`[a-z][a-z0-9_]{1,30}[a-z0-9]`).Draw(rt, "shortName")
        dir := t.TempDir()
        writeTechYAML(t, dir, "tech.yaml", fmt.Sprintf(`id: tech_roundtrip
name: Roundtrip Tech
tradition: technical
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: self
duration: instant
resolution: none
short_name: %s
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`, sn))
        reg, err := technology.Load(dir)
        require.NoError(rt, err)
        def, ok := reg.GetByShortName(sn)
        assert.True(rt, ok)
        if ok {
            assert.Equal(rt, "tech_roundtrip", def.ID)
        }
    })
}

// GetByShortName returns (nil, false) for unknown short name.
func TestGetByShortName_Unknown_ReturnsFalse(t *testing.T) {
    reg := technology.NewRegistry()
    def, ok := reg.GetByShortName("nope")
    assert.False(t, ok)
    assert.Nil(t, def)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/game/technology/... -run "TestLoad_ShortName|TestGetByShortName|TestProperty_Registry_GetByShortName" -v 2>&1 | tail -20
```

Expected: compile error (`GetByShortName` undefined).

- [ ] **Step 3: Update Registry struct and NewRegistry()**

In `internal/game/technology/registry.go`, update `Registry` struct:

```go
type Registry struct {
    byID        map[string]*TechnologyDef
    byShortName map[string]*TechnologyDef
    byTradition map[Tradition][]*TechnologyDef
    byLevel     map[int][]*TechnologyDef
    byUsage     map[UsageType][]*TechnologyDef
}
```

Update `NewRegistry()`:

```go
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

- [ ] **Step 4: Rewrite Load() with two-pass approach**

Replace the `Load` function body:

```go
func Load(dir string) (*Registry, error) {
    // First pass: parse and validate all defs.
    var defs []*TechnologyDef
    err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return fmt.Errorf("walking %q: %w", path, err)
        }
        if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
            return nil
        }
        data, err := os.ReadFile(path)
        if err != nil {
            return fmt.Errorf("reading %q: %w", path, err)
        }
        var def TechnologyDef
        dec := yaml.NewDecoder(bytes.NewReader(data))
        dec.KnownFields(true)
        if err := dec.Decode(&def); err != nil {
            return fmt.Errorf("parsing %q: %w", path, err)
        }
        if err := def.Validate(); err != nil {
            return fmt.Errorf("validating %q: %w", path, err)
        }
        defs = append(defs, &def)
        return nil
    })
    if err != nil {
        return nil, err
    }

    r := NewRegistry()
    // Build byID first so second pass can check short_name vs. all IDs.
    for _, def := range defs {
        r.byID[def.ID] = def
    }
    // Second pass: populate secondary indexes; enforce short_name uniqueness and ID collision.
    for _, def := range defs {
        r.byTradition[def.Tradition] = append(r.byTradition[def.Tradition], def)
        r.byLevel[def.Level] = append(r.byLevel[def.Level], def)
        r.byUsage[def.UsageType] = append(r.byUsage[def.UsageType], def)
        if def.ShortName == "" {
            continue
        }
        if existing, dup := r.byShortName[def.ShortName]; dup {
            return nil, fmt.Errorf("duplicate short_name %q on %q and %q", def.ShortName, existing.ID, def.ID)
        }
        if other, col := r.byID[def.ShortName]; col {
            return nil, fmt.Errorf("short_name %q on %q collides with existing technology id %q", def.ShortName, def.ID, other.ID)
        }
        r.byShortName[def.ShortName] = def
    }
    return r, nil
}
```

- [ ] **Step 5: Add GetByShortName() and update Register()**

After `Get()`, add:

```go
// GetByShortName returns the TechnologyDef for the given short name, or (nil, false) if not found.
func (r *Registry) GetByShortName(short string) (*TechnologyDef, bool) {
    d, ok := r.byShortName[short]
    return d, ok
}
```

Update `Register()` to populate `byShortName`:

```go
func (r *Registry) Register(def *TechnologyDef) {
    r.byID[def.ID] = def
    if def.ShortName != "" {
        r.byShortName[def.ShortName] = def
    }
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/game/technology/... -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/game/technology/registry.go internal/game/technology/registry_test.go
git commit -m "feat(tsn): add byShortName index, GetByShortName(), and two-pass Load() collision checks"
```

---

### Task 3: Proto short_name Fields and buildCharacterSheet Population

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go` (via `make proto`)
- Regenerate: `cmd/webclient/ui/src/proto.ts` (via `make proto-ts`)
- Modify: `internal/gameserver/grpc_service.go` (buildCharacterSheet)

- [ ] **Step 1: Add short_name to proto messages**

In `api/proto/game/v1/game.proto`, add `short_name` to four messages:

**PreparedSlotView** (after field 5):
```protobuf
message PreparedSlotView {
    string tech_id          = 1;
    bool   expended         = 2;
    string tech_name        = 3;
    string description      = 4;
    string effects_summary  = 5;
    string short_name       = 6;
}
```

**HardwiredSlotView** (after field 4):
```protobuf
message HardwiredSlotView {
    string tech_id          = 1;
    string tech_name        = 2;
    string description      = 3;
    string effects_summary  = 4;
    string short_name       = 5;
}
```

**SpontaneousKnownEntry** (after field 5):
```protobuf
message SpontaneousKnownEntry {
    string tech_id          = 1;
    string tech_name        = 2;
    int32  tech_level       = 3;
    string description      = 4;
    string effects_summary  = 5;
    string short_name       = 6;
}
```

**InnateSlotView** (after field 7):
```protobuf
message InnateSlotView {
    string tech_id          = 1;
    int32  uses_remaining   = 2;
    int32  max_uses         = 3;
    string tech_name        = 4;
    string description      = 5;
    bool   is_reaction      = 6;
    string effects_summary  = 7;
    string short_name       = 8;
}
```

- [ ] **Step 2: Regenerate Go proto bindings**

```bash
cd /home/cjohannsen/src/mud
make proto 2>&1 | tail -10
```

Expected: no errors. `internal/gameserver/gamev1/game.pb.go` is updated.

- [ ] **Step 3: Regenerate TypeScript proto bindings**

```bash
make proto-ts 2>&1 | tail -10
```

Expected: no errors. `cmd/webclient/ui/src/proto.ts` is updated.

- [ ] **Step 4: Populate ShortName in buildCharacterSheet**

In `internal/gameserver/grpc_service.go`, locate the four places in `buildCharacterSheet` that build slot view protos and add `ShortName` population.

**PreparedSlotView** (~line 5825): change to:
```go
techShortName := ""
if def, ok := s.techRegistry.Get(slot.TechID); ok {
    techShortName = def.ShortName
}
view.PreparedSlots = append(view.PreparedSlots, &gamev1.PreparedSlotView{
    TechId:         slot.TechID,
    Expended:       slot.Expended,
    TechName:       techName,
    Description:    techDesc,
    EffectsSummary: techFX,
    ShortName:      techShortName,
})
```

Note: the existing code already does `if def, ok := s.techRegistry.Get(...)` to set `techName`, `techDesc`, `techFX`. Extract `def.ShortName` inside that same `if` block:

```go
techShortName := ""
if s.techRegistry != nil {
    if def, ok := s.techRegistry.Get(slot.TechID); ok {
        techName = def.Name
        techDesc = def.Description
        techFX = technology.FormatEffectsSummary(def)
        techShortName = def.ShortName
    }
}
view.PreparedSlots = append(view.PreparedSlots, &gamev1.PreparedSlotView{
    TechId:         slot.TechID,
    Expended:       slot.Expended,
    TechName:       techName,
    Description:    techDesc,
    EffectsSummary: techFX,
    ShortName:      techShortName,
})
```

**SpontaneousKnownEntry** (~line 5871): update the existing registry lookup:
```go
techShortName := ""
if s.techRegistry != nil {
    if def, ok := s.techRegistry.Get(tid); ok {
        techName = def.Name
        techDesc = def.Description
        techFX = technology.FormatEffectsSummary(def)
        techShortName = def.ShortName
    }
}
view.SpontaneousKnown = append(view.SpontaneousKnown, &gamev1.SpontaneousKnownEntry{
    TechId:         tid,
    TechName:       techName,
    TechLevel:      int32(lvl),
    Description:    techDesc,
    EffectsSummary: techFX,
    ShortName:      techShortName,
})
```

**InnateSlotView** (~line 5901): update the existing registry lookup:
```go
techShortName := ""
if s.techRegistry != nil {
    if def, ok := s.techRegistry.Get(id); ok {
        techName = def.Name
        techDesc = def.Description
        techFX = technology.FormatEffectsSummary(def)
        isReaction = def.Reaction != nil
        techShortName = def.ShortName
    }
}
view.InnateSlots = append(view.InnateSlots, &gamev1.InnateSlotView{
    TechId:         id,
    UsesRemaining:  int32(slot.UsesRemaining),
    MaxUses:        int32(slot.MaxUses),
    TechName:       techName,
    Description:    techDesc,
    EffectsSummary: techFX,
    IsReaction:     isReaction,
    ShortName:      techShortName,
})
```

**HardwiredSlotView** (~line 5928): update the existing registry lookup:
```go
techShortName := ""
if s.techRegistry != nil {
    if def, ok := s.techRegistry.Get(id); ok {
        techName = def.Name
        techDesc = def.Description
        techFX = technology.FormatEffectsSummary(def)
        techShortName = def.ShortName
    }
}
view.HardwiredSlots = append(view.HardwiredSlots, &gamev1.HardwiredSlotView{
    TechId:         id,
    TechName:       techName,
    Description:    techDesc,
    EffectsSummary: techFX,
    ShortName:      techShortName,
})
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
go build ./internal/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 6: Run full test suite**

```bash
go test ./internal/gameserver/... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go cmd/webclient/ui/src/proto.ts internal/gameserver/grpc_service.go
git commit -m "feat(tsn): add short_name to proto slot views and populate in buildCharacterSheet"
```

---

### Task 4: handleUse Short-Name Resolution

**Files:**
- Create: `internal/gameserver/grpc_service_tsn_test.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing test for handleUse short-name resolution**

Create `internal/gameserver/grpc_service_tsn_test.go`:

```go
package gameserver

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/character"
    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/technology"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap/zaptest"
)

// REQ-TSN-6: handleUse resolves a short name to the canonical tech ID.
// Scenario: player knows "tech_alpha" as spontaneous; pool for level 1 has 0 remaining.
// "use ta" (short name) must resolve to "tech_alpha" and return "No level 1 uses remaining."
// If resolution failed, the message would be "You don't know ta."
func TestHandleUse_ShortName_ResolvesToCanonicalID(t *testing.T) {
    worldMgr, sessMgr := testWorldAndSession(t)
    logger := zaptest.NewLogger(t)

    techReg := technology.NewRegistry()
    techReg.Register(&technology.TechnologyDef{
        ID:        "tech_alpha",
        ShortName: "ta",
        Name:      "Alpha Technology",
        Tradition: technology.TraditionTechnical,
        Level:     1,
        UsageType: technology.UsageHardwired,
        Range:     technology.RangeSelf,
        Targets:   technology.TargetsSelf,
        Duration:  "instant",
        Effects: technology.TieredEffects{
            OnApply: []technology.TechEffect{{Type: technology.EffectUtility, UtilityType: "unlock"}},
        },
    })

    svc := newTestGameServiceServer(
        worldMgr, sessMgr,
        command.DefaultRegistry(),
        NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
        NewChatHandler(sessMgr),
        logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, techReg, nil, nil,
        nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
        nil,
        nil,
        nil, nil,
    )

    _, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID:         "u_tsn",
        Username:    "TSNPlayer",
        CharName:    "TSNPlayer",
        CharacterID: 0,
        RoomID:      "room_a",
        Abilities:   character.AbilityScores{},
        Role:        "player",
    })
    require.NoError(t, err)

    sess, ok := sessMgr.GetPlayer("u_tsn")
    require.True(t, ok)
    sess.SpontaneousTechs = map[int][]string{1: {"tech_alpha"}}
    sess.SpontaneousUsePools = map[int]session.SpontaneousUsePool{1: {Remaining: 0, Max: 2}}

    event, err := svc.handleUse("u_tsn", "ta", "")
    require.NoError(t, err)
    require.NotNil(t, event)

    msgEvt := event.GetPayload().(*gamev1.ServerEvent_MessageEvent)
    require.NotNil(t, msgEvt)
    assert.Equal(t, "No level 1 uses remaining.", msgEvt.MessageEvent.Text)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/gameserver/... -run "TestHandleUse_ShortName_ResolvesToCanonicalID" -v 2>&1 | tail -20
```

Expected: FAIL with message "You don't know ta." — proving resolution isn't implemented yet.

- [ ] **Step 3: Add short-name resolution to handleUse()**

In `internal/gameserver/grpc_service.go`, inside `handleUse()` (line ~7071), after the substance item check block (ending at line ~7085) and before the `if s.characterFeatsRepo == nil` early-exit check, add:

```go
// REQ-TSN-6: resolve short name to canonical tech ID before all lookup paths.
if s.techRegistry != nil {
    if def, ok := s.techRegistry.GetByShortName(abilityID); ok {
        abilityID = def.ID
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/gameserver/... -run "TestHandleUse_ShortName_ResolvesToCanonicalID" -v 2>&1 | tail -20
```

Expected: PASS with message "No level 1 uses remaining."

- [ ] **Step 5: Run full gameserver test suite**

```bash
go test ./internal/gameserver/... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_tsn_test.go
git commit -m "feat(tsn): resolve technology short names in handleUse()"
```

---

### Task 5: TechnologyDrawer Short-Name Hotbar Assignment

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx`

- [ ] **Step 1: Update PreparedItem handlePick**

In `TechnologyDrawer.tsx`, locate `PreparedItem` function (~line 70). Add short name extraction and update `handlePick`:

Find:
```typescript
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${techId}` })
    setPicking(false)
  }
```

Replace with:
```typescript
  const techId = slot.techId ?? slot.tech_id ?? ''
  const shortName = slot.shortName ?? slot.short_name ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${shortName || techId}` })
    setPicking(false)
  }
```

- [ ] **Step 2: Update InnateItem handlePick**

Locate `InnateItem` function (~line 116). Add short name extraction and update `handlePick`:

Find:
```typescript
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const remaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
  const max = slot.maxUses ?? slot.max_uses ?? 0
  const exhausted = max > 0 && remaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${techId}` })
    setPicking(false)
  }
```

Replace with:
```typescript
  const techId = slot.techId ?? slot.tech_id ?? ''
  const shortName = slot.shortName ?? slot.short_name ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const remaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
  const max = slot.maxUses ?? slot.max_uses ?? 0
  const exhausted = max > 0 && remaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${shortName || techId}` })
    setPicking(false)
  }
```

- [ ] **Step 3: Update SpontaneousItem handlePick**

Locate `SpontaneousItem` function (~line 161). Add short name extraction and update `handlePick`:

Find:
```typescript
  const techId = entry.techId ?? entry.tech_id ?? ''
  const name = entry.techName ?? entry.tech_name ?? techId
  const exhausted = poolRemaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${techId}` })
    setPicking(false)
  }
```

Replace with:
```typescript
  const techId = entry.techId ?? entry.tech_id ?? ''
  const shortName = entry.shortName ?? entry.short_name ?? ''
  const name = entry.techName ?? entry.tech_name ?? techId
  const exhausted = poolRemaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${shortName || techId}` })
    setPicking(false)
  }
```

- [ ] **Step 4: Build the web client to verify no TypeScript errors**

```bash
cd /home/cjohannsen/src/mud
make build-webclient 2>&1 | tail -20
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx
git commit -m "feat(tsn): use short_name in TechnologyDrawer hotbar slot assignment"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|-------------|------|
| REQ-TSN-1: ShortName field on TechnologyDef | Task 1 |
| REQ-TSN-2a/b/c/d: ShortName format constraints | Task 1 |
| REQ-TSN-3: Registry uniqueness enforcement | Task 2 |
| REQ-TSN-4: byShortName index + GetByShortName() | Task 2 |
| REQ-TSN-5: short_name must not equal any other tech's id | Task 2 |
| REQ-TSN-6: handleUse() resolution order | Task 4 |
| REQ-TSN-7: Hotbar assignment uses short_name when available | Task 5 |
| REQ-TSN-8: short_name in proto technology view | Task 3 |
| REQ-TSN-9: Existing hotbar slots remain valid | Covered by resolution order (ID first via exact feat/class-feature path, short-name only remaps tech IDs) |
| REQ-TSN-10: Content assignment out of scope | Not implemented (correct) |
| REQ-TSN-11a/b/c/d: Property tests | Tasks 1 and 2 |

All requirements covered.

### Type Consistency

- `technology.TechnologyDef.ShortName string` — defined Task 1, used in Task 2 (Register), Task 3 (buildCharacterSheet), Task 4 (GetByShortName returns this def)
- `technology.Registry.GetByShortName(short string) (*TechnologyDef, bool)` — defined Task 2, called in Task 4
- `gamev1.PreparedSlotView.ShortName` / `HardwiredSlotView.ShortName` / `SpontaneousKnownEntry.ShortName` / `InnateSlotView.ShortName` — generated Task 3, read in Task 5 as `slot.shortName ?? slot.short_name ?? ''`
- `session.SpontaneousUsePool` used in Task 4 test — type exists in `internal/game/session`

### Placeholder Scan

No TODOs, TBDs, or "implement later" patterns present. All code steps are complete.
