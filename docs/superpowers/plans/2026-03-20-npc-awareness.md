# NPC Awareness Terminology Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the `perception` field on NPC Template/Instance structs and all YAML files to `awareness`, eliminating the ambiguity with the player-facing Awareness stat.

**Architecture:** Pure rename — no behavior change. Update struct fields, YAML tags, all call sites, and all 45 NPC YAML files. The proto field `Perception` in NPC-related messages is out of scope (proto field names are API surface; only internal Go and YAML are in scope).

**Tech Stack:** Go, YAML

---

## File Structure

- Modify: `internal/game/npc/template.go` — rename `Perception int` field + yaml tag
- Modify: `internal/game/npc/instance.go` — rename `Perception int` field + copy in `NewInstance`
- Modify: `internal/gameserver/combat_handler.go` — 2 call sites (`inst.Perception`)
- Modify: `internal/gameserver/grpc_service.go` — 3 call sites (`inst.Perception`)
- Modify: `content/npcs/*.yaml` — 45 files: `perception:` → `awareness:`
- Update: `docs/features/npc-awareness.md` — mark complete

---

### Task 1: Rename struct fields and update all Go call sites

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write a failing compilation test**

In `internal/game/npc/npc_awareness_test.go`:

```go
package npc_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

// TestAwarenessFieldExists verifies that Template and Instance expose Awareness, not Perception.
//
// Precondition: npc package is compiled.
// Postcondition: Both Template.Awareness and Instance.Awareness are accessible int fields.
func TestAwarenessFieldExists(t *testing.T) {
	tmpl := &npc.Template{Awareness: 10}
	if tmpl.Awareness != 10 {
		t.Errorf("Template.Awareness = %d; want 10", tmpl.Awareness)
	}
	inst := &npc.Instance{Awareness: 5}
	if inst.Awareness != 5 {
		t.Errorf("Instance.Awareness = %d; want 5", inst.Awareness)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game/npc/... 2>&1 | head -20`
Expected: compile error — `unknown field Awareness`

- [ ] **Step 3: Rename Template.Perception → Template.Awareness**

In `internal/game/npc/template.go` line 50, change:
```go
Perception  int       `yaml:"perception"`
```
to:
```go
Awareness   int       `yaml:"awareness"`
```

- [ ] **Step 4: Rename Instance.Perception → Instance.Awareness**

In `internal/game/npc/instance.go`:
- Line 39-40: update comment from `Perception` to `Awareness`
- Line 40: rename field `Perception int` → `Awareness int`
- Line 170: update copy `Perception: tmpl.Perception` → `Awareness: tmpl.Awareness`

- [ ] **Step 5: Update combat_handler.go call sites**

Replace all `inst.Perception` with `inst.Awareness` (2 occurrences at lines ~1112 and ~1738).

- [ ] **Step 6: Update grpc_service.go call sites**

Replace all `inst.Perception` with `inst.Awareness` (3 occurrences at lines ~3500, ~5998, ~6606-6607).

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./internal/game/npc/... ./internal/gameserver/... 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 8: Run full suite**

Run: `go test ./... 2>&1 | grep -E "FAIL|ok"`
Expected: all ok

- [ ] **Step 9: Commit**

```bash
git add internal/game/npc/ internal/gameserver/combat_handler.go internal/gameserver/grpc_service.go
git commit -m "feat: rename NPC Perception field to Awareness in Go structs and call sites"
```

---

### Task 2: Update all NPC YAML files

**Files:**
- Modify: `content/npcs/*.yaml` (45 files) — `perception:` → `awareness:`

- [ ] **Step 1: Verify YAML files use the old key**

Run: `grep -rl "perception:" content/npcs/ | wc -l`
Expected: 45

- [ ] **Step 2: Rename the key in all files with sed**

Run:
```bash
sed -i 's/^perception:/awareness:/' content/npcs/*.yaml
```

- [ ] **Step 3: Verify rename succeeded**

Run: `grep -r "perception:" content/npcs/ | wc -l`
Expected: 0

Run: `grep -r "awareness:" content/npcs/ | wc -l`
Expected: 45

- [ ] **Step 4: Run full test suite to confirm YAML still loads**

Run: `go test ./... 2>&1 | grep -E "FAIL|ok"`
Expected: all ok

- [ ] **Step 5: Commit**

```bash
git add content/npcs/
git commit -m "feat: rename perception to awareness in all NPC YAML files (45 files)"
```

---

### Task 3: Update feature doc and mark complete

**Files:**
- Modify: `docs/features/npc-awareness.md`
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Update npc-awareness.md**

Add a completion note at the bottom of the file:

```markdown

## Implementation

Completed 2026-03-20. Renamed `Template.Perception` and `Instance.Perception` to `Awareness` in Go structs, updated all call sites in `combat_handler.go` and `grpc_service.go`, updated all 45 NPC YAML files from `perception:` to `awareness:`.
```

- [ ] **Step 2: Update index.yaml status**

In `docs/features/index.yaml`, change `status: planned` → `status: complete` for the `npc-awareness` slug.

- [ ] **Step 3: Commit**

```bash
git add docs/features/npc-awareness.md docs/features/index.yaml
git commit -m "docs: mark npc-awareness complete"
```
