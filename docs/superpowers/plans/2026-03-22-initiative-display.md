---
# Initiative Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an Initiative row immediately below the Awareness row in the Saves section of the character sheet.

**Architecture:** Single-function change in `text_renderer.go` — append one `slPlain` row after the existing Awareness row. Initiative modifier = `(quickness - 10) / 2`, rendered via the existing `signedInt()` helper. No proto, schema, or DB changes.

**Tech Stack:** Go, testify, rapid (property-based testing)

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/frontend/handlers/text_renderer.go` (line ~773) |
| Modify | `internal/frontend/handlers/text_renderer_test.go` |

---

### Task 1: Write a failing test for Initiative display

**Files:**
- Modify: `internal/frontend/handlers/text_renderer_test.go`

- [ ] **Step 1: Add a targeted test for the Initiative row**

Find the existing `TestRenderCharacterSheet_ShowsAwareness` test (or the block that tests the Saves section) in `text_renderer_test.go` and add the following test immediately after it:

```go
func TestRenderCharacterSheet_ShowsInitiative(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Quickness: 14, // (14-10)/2 = +2
	}
	result := RenderCharacterSheet(csv, 80)
	assert.Contains(t, result, "Initiative: +2")
}

func TestRenderCharacterSheet_InitiativeNegative(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Quickness: 7, // (7-10)/2 = -1 (integer division truncates toward zero in Go: -3/2 = -1)
	}
	result := RenderCharacterSheet(csv, 80)
	assert.Contains(t, result, "Initiative: -1")
}

func TestRenderCharacterSheet_InitiativeZero(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Quickness: 10, // (10-10)/2 = +0
	}
	result := RenderCharacterSheet(csv, 80)
	assert.Contains(t, result, "Initiative: +0")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsInitiative -v
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_InitiativeNegative -v
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_InitiativeZero -v
```

Expected: all three FAIL — `"Initiative"` not found in output.

---

### Task 2: Implement the Initiative row

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` (lines 772–773)

- [ ] **Step 1: Locate the Awareness row**

In `RenderCharacterSheet`, find this block (around line 772):

```go
left = append(left, slPlain(fmt.Sprintf("Awareness: %s",
    signedInt(int(csv.GetAwareness())))))

left = append(left, slPlain(""))
```

- [ ] **Step 2: Insert the Initiative row immediately after the Awareness row**

Change it to:

```go
left = append(left, slPlain(fmt.Sprintf("Awareness: %s",
    signedInt(int(csv.GetAwareness())))))
left = append(left, slPlain(fmt.Sprintf("Initiative: %s",
    signedInt((int(csv.GetQuickness())-10)/2))))

left = append(left, slPlain(""))
```

- [ ] **Step 3: Run the new tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsInitiative -v
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_InitiativeNegative -v
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_InitiativeZero -v
```

Expected: all three PASS.

- [ ] **Step 4: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./...
```

Expected: 100% pass, no regressions.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat(ui): add Initiative row to character sheet Saves section

Displays (quickness-10)/2 as a signed modifier immediately below
the Awareness row. No proto or DB changes required.

Closes initiative-display feature."
```

---

## Verification Checklist

- [ ] `Initiative: +2` appears in output for Quickness 14
- [ ] `Initiative: -1` appears in output for Quickness 7
- [ ] `Initiative: +0` appears in output for Quickness 10
- [ ] Initiative row appears immediately below Awareness row (not before it, not after the blank line)
- [ ] Full test suite passes with zero failures
- [ ] No proto, schema, or DB files were modified
