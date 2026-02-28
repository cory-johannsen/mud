# NPC Unique Name Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Assign letter suffixes (A, B, C…) to NPC instance names at spawn time so multiple NPCs of the same type in a room are distinguishable.

**Architecture:** `Manager.Spawn()` counts existing live instances of the same `TemplateID` in the target room before registration. If the new spawn would be the second instance, it renames the existing first instance to `"<Name> A"` and assigns `"<Name> B"` to the new one. For the third and beyond, it assigns the next letter. Single-instance rooms get no suffix.

**Tech Stack:** Go, `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify` for assertions.

---

### Task 1: Write failing tests for suffix assignment in Manager.Spawn

**Files:**
- Modify: `internal/game/npc/npc_test.go`

**Step 1: Add failing tests at the end of `npc_test.go`**

Append these test functions:

```go
func TestManager_Spawn_SingleInstance_NoSuffix(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	inst, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	assert.Equal(t, "Ganger", inst.Name)
}

func TestManager_Spawn_TwoInstances_LetterSuffix(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	first, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)

	second, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)

	// First instance must have been renamed to A
	got, ok := mgr.Get(first.ID)
	require.True(t, ok)
	assert.Equal(t, "Ganger A", got.Name)

	// Second instance gets B
	assert.Equal(t, "Ganger B", second.Name)
}

func TestManager_Spawn_ThreeInstances_LetterSuffixes(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	first, _ := mgr.Spawn(tmpl, "room-1")
	second, _ := mgr.Spawn(tmpl, "room-1")
	third, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)

	gotFirst, _ := mgr.Get(first.ID)
	gotSecond, _ := mgr.Get(second.ID)

	assert.Equal(t, "Ganger A", gotFirst.Name)
	assert.Equal(t, "Ganger B", gotSecond.Name)
	assert.Equal(t, "Ganger C", third.Name)
}

func TestManager_Spawn_DifferentTemplates_NoSuffix(t *testing.T) {
	tmplA := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	tmplB := &npc.Template{ID: "scavenger", Name: "Scavenger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	g, err := mgr.Spawn(tmplA, "room-1")
	require.NoError(t, err)
	s, err := mgr.Spawn(tmplB, "room-1")
	require.NoError(t, err)

	assert.Equal(t, "Ganger", g.Name)
	assert.Equal(t, "Scavenger", s.Name)
}

func TestManager_Spawn_DifferentRooms_NoSuffix(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	inst1, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	inst2, err := mgr.Spawn(tmpl, "room-2")
	require.NoError(t, err)

	assert.Equal(t, "Ganger", inst1.Name)
	assert.Equal(t, "Ganger", inst2.Name)
}

func TestManager_Spawn_Property_SuffixesAreUnique(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
		mgr := npc.NewManager()

		ids := make([]string, 0, n)
		for i := 0; i < n; i++ {
			inst, err := mgr.Spawn(tmpl, "room-1")
			require.NoError(rt, err)
			ids = append(ids, inst.ID)
		}

		names := make(map[string]bool)
		for _, id := range ids {
			inst, ok := mgr.Get(id)
			require.True(rt, ok)
			assert.False(rt, names[inst.Name], "duplicate name: %s", inst.Name)
			names[inst.Name] = true
		}
	})
}
```

**Step 2: Run the tests to confirm they fail**

```
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestManager_Spawn_Single|TestManager_Spawn_Two|TestManager_Spawn_Three|TestManager_Spawn_Different|TestManager_Spawn_Property_Suffixes" -v
```

Expected: FAIL — `"Ganger"` instead of `"Ganger A"` / `"Ganger B"`.

**Step 3: Commit the failing tests**

```bash
git add internal/game/npc/npc_test.go
git commit -m "test: add failing tests for NPC letter suffix on spawn"
```

---

### Task 2: Implement suffix assignment in Manager.Spawn

**Files:**
- Modify: `internal/game/npc/manager.go:31-53`

**Step 1: Replace the `Spawn` method body**

The new `Spawn` must:
1. Count existing live instances with the same `TemplateID` in `roomID` (under the write lock)
2. If count == 0: register with base name (no suffix)
3. If count == 1: rename the existing instance to `"<Name> A"`, assign `"<Name> B"` to new instance
4. If count >= 2: assign `"<Name> <Letter>"` where letter = `'A' + count`

Replace the `Spawn` method (lines 27–53) with:

```go
// Spawn creates a new Instance from tmpl and places it in roomID.
// If multiple instances of the same template occupy the room, each is assigned
// a unique uppercase letter suffix (A, B, C, …). A single instance has no suffix.
//
// Precondition: tmpl must be non-nil; roomID must be non-empty.
// Postcondition: Returns a new Instance with a unique ID registered in roomID.
//   Existing instances of the same template in the room may be renamed.
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

	// Count existing live instances of the same template in this room.
	var sameTemplate []*Instance
	if ids, ok := m.roomSets[roomID]; ok {
		for existingID := range ids {
			if existing, ok := m.instances[existingID]; ok && existing.TemplateID == tmpl.ID {
				sameTemplate = append(sameTemplate, existing)
			}
		}
	}

	count := len(sameTemplate)
	switch count {
	case 0:
		// Single instance — no suffix.
	case 1:
		// Second instance arriving: rename the first to A, assign B to new.
		sameTemplate[0].Name = tmpl.Name + " A"
		inst.Name = tmpl.Name + " B"
	default:
		// Third or beyond: existing instances already have suffixes.
		inst.Name = fmt.Sprintf("%s %c", tmpl.Name, 'A'+rune(count))
	}

	m.instances[id] = inst
	if m.roomSets[roomID] == nil {
		m.roomSets[roomID] = make(map[string]bool)
	}
	m.roomSets[roomID][id] = true

	return inst, nil
}
```

Note: `fmt` is already imported. No new imports needed.

**Step 2: Run the new tests**

```
go test ./internal/game/npc/... -run "TestManager_Spawn_Single|TestManager_Spawn_Two|TestManager_Spawn_Three|TestManager_Spawn_Different|TestManager_Spawn_Property_Suffixes" -v
```

Expected: all PASS.

**Step 3: Run the full npc package tests to check for regressions**

```
go test ./internal/game/npc/... -v
```

Expected: all PASS. The existing `TestManager_SpawnAndList` test checks `inst.RoomID` and `inst.ID` but not `inst.Name`, so it will still pass.

**Step 4: Run the full test suite**

```
go test ./... 2>&1
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/game/npc/manager.go
git commit -m "feat: assign letter suffix to NPC names when multiple of same type share a room"
```
