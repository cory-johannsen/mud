# Combat Stage 2 — NPC Definitions + Room Entities Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** YAML-defined NPC templates loaded at startup; NPC instances tracked per room; `look` shows NPCs; `examine <target>` shows NPC detail.

**Architecture:** A new `internal/game/npc` package provides `Template` (YAML schema), `Instance` (runtime entity), and a concurrent-safe `Manager` that tracks instances by room ID. The gameserver loads templates at startup, spawns initial instances, and the `WorldHandler` queries the NPC Manager when building `RoomView`. A new `ExamineRequest` proto message and `NpcView` response enable the `examine` command.

**Tech Stack:** Go, YAML (`gopkg.in/yaml.v3`), Protocol Buffers, PostgreSQL (migration only — instances in-memory for Stage 2), `pgregory.net/rapid` for property-based tests.

---

### Task 1: NPC Template Model + YAML Loading

**Files:**
- Create: `internal/game/npc/template.go`
- Create: `internal/game/npc/npc_test.go`

**Step 1: Write the failing tests**

Create `internal/game/npc/npc_test.go`:

```go
package npc_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestLoadTemplates_ValidDir(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: ganger
name: Ganger
description: A street tough with a scar across his cheek.
level: 1
max_hp: 18
ac: 14
perception: 5
abilities:
  strength: 14
  dexterity: 12
  constitution: 14
  intelligence: 8
  wisdom: 10
  charisma: 8
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ganger.yaml"), []byte(yaml), 0644))

	templates, err := npc.LoadTemplates(dir)
	require.NoError(t, err)
	require.Len(t, templates, 1)

	tmpl := templates[0]
	assert.Equal(t, "ganger", tmpl.ID)
	assert.Equal(t, "Ganger", tmpl.Name)
	assert.Equal(t, 1, tmpl.Level)
	assert.Equal(t, 18, tmpl.MaxHP)
	assert.Equal(t, 14, tmpl.AC)
	assert.Equal(t, 5, tmpl.Perception)
	assert.Equal(t, 14, tmpl.Abilities.Strength)
}

func TestLoadTemplates_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	templates, err := npc.LoadTemplates(dir)
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLoadTemplates_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::invalid"), 0644))
	_, err := npc.LoadTemplates(dir)
	assert.Error(t, err)
}

func TestTemplate_Property_IDAndNameNonEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z0-9_]{0,15}`).Draw(rt, "id")
		name := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz "))).
			Filter(func(s string) bool { return len(s) > 0 }).
			Draw(rt, "name")
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		maxHP := rapid.IntRange(1, 300).Draw(rt, "max_hp")
		ac := rapid.IntRange(10, 30).Draw(rt, "ac")

		tmpl := &npc.Template{
			ID:    id,
			Name:  name,
			Level: level,
			MaxHP: maxHP,
			AC:    ac,
		}
		assert.NotEmpty(rt, tmpl.ID)
		assert.NotEmpty(rt, tmpl.Name)
		assert.Greater(rt, tmpl.Level, 0)
		assert.Greater(rt, tmpl.MaxHP, 0)
	})
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... 2>&1 | head -20
```
Expected: build error — package does not exist.

**Step 3: Write the implementation**

Create `internal/game/npc/template.go`:

```go
// Package npc provides NPC template definitions and live instance management.
package npc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Abilities holds the six core ability scores for an NPC template.
type Abilities struct {
	Strength     int `yaml:"strength"`
	Dexterity    int `yaml:"dexterity"`
	Constitution int `yaml:"constitution"`
	Intelligence int `yaml:"intelligence"`
	Wisdom       int `yaml:"wisdom"`
	Charisma     int `yaml:"charisma"`
}

// Template defines a reusable NPC archetype loaded from YAML.
type Template struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Level       int       `yaml:"level"`
	MaxHP       int       `yaml:"max_hp"`
	AC          int       `yaml:"ac"`
	Perception  int       `yaml:"perception"`
	Abilities   Abilities `yaml:"abilities"`
}

// Validate checks that the template satisfies basic invariants.
//
// Postcondition: Returns nil if valid, or an error on the first violation.
func (t *Template) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("npc template: id must not be empty")
	}
	if t.Name == "" {
		return fmt.Errorf("npc template %q: name must not be empty", t.ID)
	}
	if t.Level < 1 {
		return fmt.Errorf("npc template %q: level must be >= 1", t.ID)
	}
	if t.MaxHP < 1 {
		return fmt.Errorf("npc template %q: max_hp must be >= 1", t.ID)
	}
	if t.AC < 10 {
		return fmt.Errorf("npc template %q: ac must be >= 10", t.ID)
	}
	return nil
}

// LoadTemplates reads all *.yaml files in dir and returns the parsed templates.
//
// Precondition: dir must be a readable directory.
// Postcondition: Returns all templates or an error on the first parse/validate failure.
func LoadTemplates(dir string) ([]*Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading npc dir %q: %w", dir, err)
	}

	var templates []*Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}

		var tmpl Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", path, err)
		}

		if err := tmpl.Validate(); err != nil {
			return nil, fmt.Errorf("validating %q: %w", path, err)
		}

		templates = append(templates, &tmpl)
	}
	return templates, nil
}
```

**Step 4: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v 2>&1
```
Expected: all `TestLoadTemplates_*` and `TestTemplate_Property_*` PASS.

**Step 5: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/npc_test.go
git commit -m "feat(npc): Template model and YAML loader (COMBAT Stage 2, Task 1)"
```

---

### Task 2: NPC Instance Model

**Files:**
- Create: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/npc_test.go` (append instance tests)

**Step 1: Append instance tests to `npc_test.go`**

```go
func TestNewInstance_SetsFieldsFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID:          "ganger",
		Name:        "Ganger",
		Description: "A scarred street tough.",
		Level:       1,
		MaxHP:       18,
		AC:          14,
		Perception:  5,
	}

	inst := npc.NewInstance("inst-1", tmpl, "room-alley")
	assert.Equal(t, "inst-1", inst.ID)
	assert.Equal(t, "ganger", inst.TemplateID)
	assert.Equal(t, "Ganger", inst.Name)
	assert.Equal(t, "A scarred street tough.", inst.Description)
	assert.Equal(t, "room-alley", inst.RoomID)
	assert.Equal(t, 18, inst.CurrentHP)
	assert.Equal(t, 18, inst.MaxHP)
	assert.Equal(t, 14, inst.AC)
	assert.False(t, inst.IsDead())
}

func TestInstance_IsDead(t *testing.T) {
	tmpl := &npc.Template{ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10}
	inst := npc.NewInstance("i1", tmpl, "room-1")
	inst.CurrentHP = 0
	assert.True(t, inst.IsDead())
	inst.CurrentHP = -5
	assert.True(t, inst.IsDead())
}

func TestInstance_HealthDescription(t *testing.T) {
	tmpl := &npc.Template{ID: "t", Name: "T", Level: 1, MaxHP: 100, AC: 10}
	tests := []struct {
		hp   int
		want string
	}{
		{100, "unharmed"},
		{90, "barely scratched"},
		{70, "lightly wounded"},
		{50, "moderately wounded"},
		{25, "heavily wounded"},
		{10, "critically wounded"},
		{0, "dead"},
	}
	for _, tc := range tests {
		inst := npc.NewInstance("i", tmpl, "r")
		inst.CurrentHP = tc.hp
		assert.Equal(t, tc.want, inst.HealthDescription(), "hp=%d", tc.hp)
	}
}

func TestInstance_Property_HealthDescriptionNonEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxHP := rapid.IntRange(1, 300).Draw(rt, "max_hp")
		currentHP := rapid.IntRange(-50, maxHP).Draw(rt, "current_hp")
		tmpl := &npc.Template{ID: "t", Name: "T", Level: 1, MaxHP: maxHP, AC: 10}
		inst := npc.NewInstance("i", tmpl, "r")
		inst.CurrentHP = currentHP
		assert.NotEmpty(rt, inst.HealthDescription())
	})
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestNewInstance -v 2>&1 | head -10
```
Expected: compile error — `npc.NewInstance` undefined.

**Step 3: Write the implementation**

Create `internal/game/npc/instance.go`:

```go
package npc

// Instance is a live NPC entity occupying a room.
type Instance struct {
	// ID uniquely identifies this runtime instance.
	ID string
	// TemplateID is the source template's ID.
	TemplateID string
	// Name is copied from the template for display.
	Name string
	// Description is copied from the template.
	Description string
	// RoomID is the room this instance currently occupies.
	RoomID string
	// CurrentHP is the instance's current hit points.
	CurrentHP int
	// MaxHP is the instance's maximum hit points.
	MaxHP int
	// AC is the instance's armor class.
	AC int
	// Level is the instance's level.
	Level int
	// Perception is the instance's perception modifier.
	Perception int
}

// NewInstance creates a live NPC instance from a template, placed in roomID.
//
// Precondition: id must be non-empty; tmpl must be non-nil; roomID must be non-empty.
// Postcondition: CurrentHP equals tmpl.MaxHP.
func NewInstance(id string, tmpl *Template, roomID string) *Instance {
	return &Instance{
		ID:          id,
		TemplateID:  tmpl.ID,
		Name:        tmpl.Name,
		Description: tmpl.Description,
		RoomID:      roomID,
		CurrentHP:   tmpl.MaxHP,
		MaxHP:       tmpl.MaxHP,
		AC:          tmpl.AC,
		Level:       tmpl.Level,
		Perception:  tmpl.Perception,
	}
}

// IsDead reports whether the instance has zero or fewer hit points.
func (i *Instance) IsDead() bool {
	return i.CurrentHP <= 0
}

// HealthDescription returns a visible health state string suitable for examine output.
//
// Postcondition: Returns a non-empty string.
func (i *Instance) HealthDescription() string {
	if i.CurrentHP <= 0 {
		return "dead"
	}
	pct := float64(i.CurrentHP) / float64(i.MaxHP)
	switch {
	case pct >= 1.0:
		return "unharmed"
	case pct >= 0.85:
		return "barely scratched"
	case pct >= 0.60:
		return "lightly wounded"
	case pct >= 0.40:
		return "moderately wounded"
	case pct >= 0.20:
		return "heavily wounded"
	default:
		return "critically wounded"
	}
}
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v 2>&1
```
Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/game/npc/instance.go internal/game/npc/npc_test.go
git commit -m "feat(npc): Instance model with HealthDescription (Task 2)"
```

---

### Task 3: NPC Manager (concurrent-safe, per-room tracking)

**Files:**
- Create: `internal/game/npc/manager.go`
- Modify: `internal/game/npc/npc_test.go` (append manager tests)

**Step 1: Append manager tests to `npc_test.go`**

```go
func TestManager_SpawnAndList(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 18, AC: 14}
	mgr := npc.NewManager()

	inst, err := mgr.Spawn(tmpl, "room-alley")
	require.NoError(t, err)
	assert.NotEmpty(t, inst.ID)
	assert.Equal(t, "room-alley", inst.RoomID)

	list := mgr.InstancesInRoom("room-alley")
	require.Len(t, list, 1)
	assert.Equal(t, inst.ID, list[0].ID)
}

func TestManager_InstancesInRoom_Empty(t *testing.T) {
	mgr := npc.NewManager()
	assert.Empty(t, mgr.InstancesInRoom("nonexistent-room"))
}

func TestManager_Remove(t *testing.T) {
	tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()
	inst, _ := mgr.Spawn(tmpl, "room-1")

	require.NoError(t, mgr.Remove(inst.ID))
	assert.Empty(t, mgr.InstancesInRoom("room-1"))
}

func TestManager_Remove_NotFound(t *testing.T) {
	mgr := npc.NewManager()
	assert.Error(t, mgr.Remove("nonexistent"))
}

func TestManager_Get(t *testing.T) {
	tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()
	inst, _ := mgr.Spawn(tmpl, "room-1")

	got, ok := mgr.Get(inst.ID)
	assert.True(t, ok)
	assert.Equal(t, inst.ID, got.ID)

	_, ok = mgr.Get("missing")
	assert.False(t, ok)
}

func TestManager_FindInRoom_PrefixMatch(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()
	inst, _ := mgr.Spawn(tmpl, "room-1")

	found := mgr.FindInRoom("room-1", "gan")
	require.NotNil(t, found)
	assert.Equal(t, inst.ID, found.ID)

	notFound := mgr.FindInRoom("room-1", "xyz")
	assert.Nil(t, notFound)
}

func TestManager_Property_SpawnProducesUniqueIDs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(rt, "n")
		tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
		mgr := npc.NewManager()
		ids := make(map[string]bool)
		for i := 0; i < n; i++ {
			inst, err := mgr.Spawn(tmpl, "room-1")
			require.NoError(rt, err)
			assert.False(rt, ids[inst.ID], "duplicate ID: %s", inst.ID)
			ids[inst.ID] = true
		}
	})
}
```

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestManager -v 2>&1 | head -10
```
Expected: compile error — `npc.NewManager` undefined.

**Step 3: Write the implementation**

Create `internal/game/npc/manager.go`:

```go
package npc

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Manager tracks all live NPC instances by ID and by room.
// All methods are safe for concurrent use.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance       // instanceID → Instance
	roomSets  map[string]map[string]bool // roomID → set of instanceIDs
	counter   atomic.Uint64
}

// NewManager creates an empty NPC Manager.
func NewManager() *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		roomSets:  make(map[string]map[string]bool),
	}
}

// Spawn creates a new Instance from tmpl and places it in roomID.
//
// Precondition: tmpl must be non-nil; roomID must be non-empty.
// Postcondition: Returns a new Instance with a unique ID registered in roomID.
func (m *Manager) Spawn(tmpl *Template, roomID string) (*Instance, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("npc.Manager.Spawn: tmpl must not be nil")
	}
	if roomID == "" {
		return nil, fmt.Errorf("npc.Manager.Spawn: roomID must not be empty")
	}

	n := m.counter.Add(1)
	id := fmt.Sprintf("%s-%s-%d", tmpl.ID, roomID, n)
	inst := NewInstance(id, tmpl, roomID)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.instances[id] = inst
	if m.roomSets[roomID] == nil {
		m.roomSets[roomID] = make(map[string]bool)
	}
	m.roomSets[roomID][id] = true

	return inst, nil
}

// Remove deletes an instance by ID.
//
// Precondition: id must be non-empty.
// Postcondition: Returns an error if the instance is not found.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("npc instance %q not found", id)
	}

	if rs, ok := m.roomSets[inst.RoomID]; ok {
		delete(rs, id)
		if len(rs) == 0 {
			delete(m.roomSets, inst.RoomID)
		}
	}
	delete(m.instances, id)
	return nil
}

// Get returns the instance with the given ID.
//
// Postcondition: Returns (inst, true) if found, or (nil, false) otherwise.
func (m *Manager) Get(id string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	return inst, ok
}

// InstancesInRoom returns a snapshot of all live instances in roomID.
//
// Postcondition: Returns a non-nil slice (may be empty).
func (m *Manager) InstancesInRoom(roomID string) []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids, ok := m.roomSets[roomID]
	if !ok {
		return []*Instance{}
	}

	out := make([]*Instance, 0, len(ids))
	for id := range ids {
		if inst, ok := m.instances[id]; ok {
			out = append(out, inst)
		}
	}
	return out
}

// FindInRoom returns the first instance in roomID whose Name has target as a
// case-insensitive prefix. Returns nil if no match is found.
func (m *Manager) FindInRoom(roomID, target string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids, ok := m.roomSets[roomID]
	if !ok {
		return nil
	}

	lower := strings.ToLower(target)
	for id := range ids {
		inst, ok := m.instances[id]
		if !ok {
			continue
		}
		if strings.HasPrefix(strings.ToLower(inst.Name), lower) {
			return inst
		}
	}
	return nil
}
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v 2>&1
```
Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/game/npc/manager.go internal/game/npc/npc_test.go
git commit -m "feat(npc): concurrent-safe Manager with per-room tracking (Task 3)"
```

---

### Task 4: NPC YAML Content Files

**Files:**
- Create: `content/npcs/ganger.yaml`
- Create: `content/npcs/lieutenant.yaml`
- Create: `content/npcs/scavenger.yaml`

**Step 1: Create the YAML files**

`content/npcs/ganger.yaml`:
```yaml
id: ganger
name: Ganger
description: A street tough with a scar across one cheek. He eyes you with territorial suspicion.
level: 1
max_hp: 18
ac: 14
perception: 5
abilities:
  strength: 14
  dexterity: 12
  constitution: 14
  intelligence: 8
  wisdom: 10
  charisma: 8
```

`content/npcs/lieutenant.yaml`:
```yaml
id: lieutenant
name: Gang Lieutenant
description: A seasoned enforcer in a battered leather coat. His posture says he has been in more fights than you have.
level: 3
max_hp: 42
ac: 16
perception: 8
abilities:
  strength: 16
  dexterity: 14
  constitution: 16
  intelligence: 10
  wisdom: 12
  charisma: 12
```

`content/npcs/scavenger.yaml`:
```yaml
id: scavenger
name: Scavenger
description: A wiry figure in mismatched armor, picking through the rubble with practiced efficiency.
level: 1
max_hp: 14
ac: 13
perception: 7
abilities:
  strength: 10
  dexterity: 16
  constitution: 12
  intelligence: 12
  wisdom: 14
  charisma: 8
```

**Step 2: Verify templates load cleanly**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v 2>&1
```
All existing tests still PASS (no new tests in this task).

**Step 3: Commit**

```bash
git add content/npcs/
git commit -m "content(npc): add ganger, lieutenant, scavenger NPC templates (Task 4)"
```

---

### Task 5: Proto — NpcInfo, ExamineRequest, NpcView

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/` (run protoc)

**Step 1: Edit the proto file**

Add to `game.proto`:

In `ClientMessage.payload` oneof, add:
```proto
ExamineRequest examine = 10;
```

In `ServerEvent.payload` oneof, add:
```proto
NpcView npc_view = 10;
```

In `RoomView`, add field:
```proto
repeated NpcInfo npcs = 6;
```

Add new messages after `CharacterInfo`:
```proto
// NpcInfo summarises an NPC visible in the room.
message NpcInfo {
  string instance_id = 1;
  string name        = 2;
}

// ExamineRequest asks the server for detail on a named target.
message ExamineRequest {
  string target = 1;
}

// NpcView delivers detailed NPC information in response to examine.
message NpcView {
  string instance_id        = 1;
  string name               = 2;
  string description        = 3;
  string health_description = 4;
  int32  level              = 5;
}
```

**Step 2: Regenerate the Go bindings**

```
cd /home/cjohannsen/src/mud && mise run proto
```
Expected: regenerated files in `internal/gameserver/gamev1/`.

If `mise run proto` is not available, check the Makefile or run:
```
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/game/v1/game.proto
```

**Step 3: Verify the project still builds**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: build succeeds (no errors; new fields are unused until Task 6).

**Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(proto): add NpcInfo, ExamineRequest, NpcView messages (Task 5)"
```

---

### Task 6: Migration 005 — npc_instances table

**Files:**
- Create: `migrations/005_npc_instances.up.sql`
- Create: `migrations/005_npc_instances.down.sql`

**Step 1: Write the migration files**

`migrations/005_npc_instances.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS npc_instances (
    id          TEXT        NOT NULL PRIMARY KEY,
    template_id TEXT        NOT NULL,
    room_id     TEXT        NOT NULL,
    current_hp  INT         NOT NULL,
    conditions  JSONB       NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS npc_instances_room_id_idx ON npc_instances (room_id);
```

`migrations/005_npc_instances.down.sql`:
```sql
DROP TABLE IF EXISTS npc_instances;
```

**Step 2: Verify the migration runs**

The project uses `golang-migrate`. Run the migration against the dev database:
```
cd /home/cjohannsen/src/mud && mise run migrate-up
```
If no `mise` task, check the Makefile or README for the migrate command. Alternatively, just verify the SQL is syntactically valid:
```
cd /home/cjohannsen/src/mud && psql "$DATABASE_URL" -c '\i migrations/005_npc_instances.up.sql' 2>&1
```

**Step 3: Commit**

```bash
git add migrations/005_npc_instances.up.sql migrations/005_npc_instances.down.sql
git commit -m "feat(db): migration 005 npc_instances table (Task 6)"
```

---

### Task 7: WorldHandler — include NPCs in RoomView

**Files:**
- Modify: `internal/gameserver/world_handler.go`

**Step 1: Update `WorldHandler` struct and constructor**

In `internal/gameserver/world_handler.go`, add `npcMgr` field and update `NewWorldHandler`:

```go
import "github.com/cory-johannsen/mud/internal/game/npc"

type WorldHandler struct {
	world    *world.Manager
	sessions *session.Manager
	npcMgr   *npc.Manager
}

func NewWorldHandler(worldMgr *world.Manager, sessMgr *session.Manager, npcMgr *npc.Manager) *WorldHandler {
	return &WorldHandler{
		world:    worldMgr,
		sessions: sessMgr,
		npcMgr:   npcMgr,
	}
}
```

**Step 2: Update `buildRoomView` to populate `NpcInfo`**

Replace the existing `buildRoomView` return statement construction:

```go
func (h *WorldHandler) buildRoomView(uid string, room *world.Room) *gamev1.RoomView {
	players := h.sessions.PlayersInRoom(room.ID)
	sess, _ := h.sessions.GetPlayer(uid)
	var otherPlayers []string
	for _, p := range players {
		if sess != nil && p == sess.CharName {
			continue
		}
		otherPlayers = append(otherPlayers, p)
	}

	visibleExits := room.VisibleExits()
	exitInfos := make([]*gamev1.ExitInfo, 0, len(visibleExits))
	for _, e := range visibleExits {
		exitInfos = append(exitInfos, &gamev1.ExitInfo{
			Direction:    string(e.Direction),
			TargetRoomId: e.TargetRoom,
			Locked:       e.Locked,
			Hidden:       e.Hidden,
		})
	}

	instances := h.npcMgr.InstancesInRoom(room.ID)
	npcInfos := make([]*gamev1.NpcInfo, 0, len(instances))
	for _, inst := range instances {
		if !inst.IsDead() {
			npcInfos = append(npcInfos, &gamev1.NpcInfo{
				InstanceId: inst.ID,
				Name:       inst.Name,
			})
		}
	}

	return &gamev1.RoomView{
		RoomId:      room.ID,
		Title:       room.Title,
		Description: room.Description,
		Exits:       exitInfos,
		Players:     otherPlayers,
		Npcs:        npcInfos,
	}
}
```

**Step 3: Fix the call site in `grpc_service.go`**

`NewWorldHandler` now requires `npcMgr`. The gameserver will be wired in Task 9. For now, verify the package compiles:

```
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1
```
Expected: compile error on `NewWorldHandler` call in `cmd/gameserver/main.go` — this will be fixed in Task 9.

**Step 4: Commit**

```bash
git add internal/gameserver/world_handler.go
git commit -m "feat(world): include NPC instances in RoomView (Task 7)"
```

---

### Task 8: Examine Command — Handler + Registration

**Files:**
- Create: `internal/gameserver/npc_handler.go`
- Modify: `internal/game/command/commands.go`
- Modify: `internal/gameserver/grpc_service.go`

**Step 1: Add the `examine` command constant and registration**

In `internal/game/command/commands.go`, add:
```go
HandlerExamine = "examine"
```

In `BuiltinCommands()`, append:
```go
{Name: "examine", Aliases: []string{"ex", "look at"}, Help: "Examine an NPC or object", Category: CategoryWorld, Handler: HandlerExamine},
```

**Step 2: Create the NPC handler**

Create `internal/gameserver/npc_handler.go`:

```go
package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// NPCHandler handles examine and future NPC-related commands.
type NPCHandler struct {
	npcMgr   *npc.Manager
	sessions *session.Manager
}

// NewNPCHandler creates an NPCHandler.
//
// Precondition: npcMgr and sessions must be non-nil.
func NewNPCHandler(npcMgr *npc.Manager, sessions *session.Manager) *NPCHandler {
	return &NPCHandler{npcMgr: npcMgr, sessions: sessions}
}

// Examine looks up an NPC by name prefix in the player's room and returns its detail view.
//
// Precondition: uid must be a valid connected player; target must be non-empty.
// Postcondition: Returns NpcView or an error if the target is not found.
func (h *NPCHandler) Examine(uid, target string) (*gamev1.NpcView, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}

	return &gamev1.NpcView{
		InstanceId:        inst.ID,
		Name:              inst.Name,
		Description:       inst.Description,
		HealthDescription: inst.HealthDescription(),
		Level:             int32(inst.Level),
	}, nil
}
```

**Step 3: Wire examine into `grpc_service.go`**

Add `npcH *NPCHandler` to `GameServiceServer`:
```go
type GameServiceServer struct {
	// ... existing fields ...
	npcH      *NPCHandler
}
```

Update `NewGameServiceServer` signature to accept `npcHandler *NPCHandler` and set `npcH: npcHandler`.

In `dispatch`, add:
```go
case *gamev1.ClientMessage_Examine:
    return s.handleExamine(uid, p.Examine)
```

Add handler:
```go
func (s *GameServiceServer) handleExamine(uid string, req *gamev1.ExamineRequest) (*gamev1.ServerEvent, error) {
	view, err := s.npcH.Examine(uid, req.Target)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_NpcView{NpcView: view},
	}, nil
}
```

**Step 4: Verify build**

```
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1
```
Expected: compiles (main.go wiring will be done in Task 9).

**Step 5: Commit**

```bash
git add internal/gameserver/npc_handler.go internal/game/command/commands.go internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): examine command and NPCHandler (Task 8)"
```

---

### Task 9: Wire NPC Loading into Gameserver Startup

**Files:**
- Modify: `cmd/gameserver/main.go`

**Step 1: Read `cmd/gameserver/main.go` to understand current startup flow**

Look for: flag parsing, world manager init, dice roller init, `NewGameServiceServer` call.

**Step 2: Add NPC wiring**

The startup sequence additions (in order after world manager init):

```go
// Load NPC templates
npcsDir := flag.String("npcs-dir", "content/npcs", "Directory containing NPC YAML templates")
// ... after flag.Parse() ...

npcTemplates, err := npc.LoadTemplates(*npcsDir)
if err != nil {
    logger.Fatal("loading npc templates", zap.Error(err))
}
logger.Info("loaded npc templates", zap.Int("count", len(npcTemplates)))

npcMgr := npc.NewManager()

// Spawn initial NPC instances: one of each template in the start room.
startRoom := worldMgr.StartRoom()
if startRoom != nil {
    for _, tmpl := range npcTemplates {
        if _, err := npcMgr.Spawn(tmpl, startRoom.ID); err != nil {
            logger.Fatal("spawning npc", zap.String("template", tmpl.ID), zap.Error(err))
        }
        logger.Info("spawned npc", zap.String("template", tmpl.ID), zap.String("room", startRoom.ID))
    }
}

// Update WorldHandler construction
worldH := NewWorldHandler(worldMgr, sessMgr, npcMgr)

// Update NPCHandler construction
npcH := NewNPCHandler(npcMgr, sessMgr)

// Update GameServiceServer construction to pass npcH
```

Also update `NewGameServiceServer` call to pass `npcH`.

**Step 3: Verify the full build**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: zero errors.

**Step 4: Run all unit tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... ./internal/gameserver/... ./internal/game/command/... -v 2>&1
```
Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/gameserver/main.go
git commit -m "feat(gameserver): load NPC templates and spawn instances at startup (Task 9)"
```

---

### Task 10: Frontend — Render NPCs in Look Output + Examine Command

**Files:**
- Read: `internal/frontend/handlers/` (find the look response renderer)
- Modify: whichever handler file renders `RoomView`
- Read: frontend command parsing to understand how `examine <target>` gets sent

**Step 1: Find how `RoomView` is rendered**

```
grep -r "RoomView\|room_view\|Title\|Description" internal/frontend/ --include="*.go" -l
```

**Step 2: Update the look renderer to show NPCs**

In the file that renders `RoomView`, after the players list, add NPC output. The pattern will be something like:

```go
// After rendering players:
if len(view.Npcs) > 0 {
    sb.WriteString("\r\nNPCs here: ")
    names := make([]string, 0, len(view.Npcs))
    for _, n := range view.Npcs {
        names = append(names, n.Name)
    }
    sb.WriteString(strings.Join(names, ", "))
}
```

**Step 3: Add `examine` to the frontend command dispatch**

Find where commands like `look`, `say`, `who` are dispatched to gRPC. Add:

```go
case "examine", "ex":
    target := strings.TrimSpace(strings.TrimPrefix(input, cmd+" "))
    if target == "" {
        // show error
        continue
    }
    msg = &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_Examine{
            Examine: &gamev1.ExamineRequest{Target: target},
        },
    }
```

**Step 4: Handle `NpcView` response in the frontend event renderer**

In the switch/case that handles `ServerEvent` payloads, add:

```go
case *gamev1.ServerEvent_NpcView:
    v := p.NpcView
    fmt.Fprintf(out, "%s\r\n%s\r\nCondition: %s  Level: %d\r\n",
        v.Name, v.Description, v.HealthDescription, v.Level)
```

**Step 5: Build and verify**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: zero errors.

**Step 6: Commit**

```bash
git add internal/frontend/
git commit -m "feat(frontend): render NPCs in look output; add examine command (Task 10)"
```

---

### Task 11: Final Verification + Push

**Step 1: Run all tests**

```
cd /home/cjohannsen/src/mud && go test ./... 2>&1
```
Expected: all PASS (skip/warn on Postgres integration tests if no DB available).

**Step 2: Run the race detector on npc package**

```
cd /home/cjohannsen/src/mud && go test -race ./internal/game/npc/... -v 2>&1
```
Expected: PASS with no race conditions detected.

**Step 3: Build all binaries**

```
cd /home/cjohannsen/src/mud && go build ./cmd/... 2>&1
```
Expected: zero errors.

**Step 4: Push**

```bash
git push origin main
```
