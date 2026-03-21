# EQ Command 2-Column Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reformat the `eq` command output so weapons render full-width at the top and the 8 armor slots + 11 accessory slots render side-by-side in two columns below.

**Architecture:** Add a `width int` parameter to `HandleEquipment` in `internal/game/command/equipment.go`. At `width >= 60`, a private `zipCols` helper assembles armor (left) and accessories (right) into a two-column grid. Below 60 the existing single-column layout is unchanged. The grpc caller in `grpc_service.go` passes `80` (telnet baseline). Existing tests pass `0` to exercise the single-column path unchanged.

**Tech Stack:** Go 1.26, `github.com/stretchr/testify`, `pgregory.net/rapid`.

---

## File Map

| File | Change |
|------|--------|
| `internal/game/command/equipment.go` | Add `width int` param; add `zipCols` helper; add 2-col branch |
| `internal/game/command/equipment_test.go` | Update all existing calls to pass `0`; add 2-col tests |
| `internal/gameserver/grpc_service.go` | Pass `80` to `HandleEquipment` |

---

## Task 1: Add `width` parameter and update all callers + existing tests

**Files:**
- Modify: `internal/game/command/equipment.go`
- Modify: `internal/game/command/equipment_test.go`
- Modify: `internal/gameserver/grpc_service.go`

**Before starting**, confirm the exact call site in grpc_service.go:
```bash
grep -n "HandleEquipment" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go
```
And confirm all test call sites:
```bash
grep -n "HandleEquipment" /home/cjohannsen/src/mud/internal/game/command/equipment_test.go
```

- [ ] **Step 1: Update `HandleEquipment` signature**

In `internal/game/command/equipment.go`, change line 46:
```go
// Before:
func HandleEquipment(sess *session.PlayerSession) string {

// After:
func HandleEquipment(sess *session.PlayerSession, width int) string {
```

No other changes to the function body yet.

- [ ] **Step 2: Update grpc caller**

In `internal/gameserver/grpc_service.go`, find the line:
```go
return messageEvent(command.HandleEquipment(sess)), nil
```
Change to:
```go
return messageEvent(command.HandleEquipment(sess, 80)), nil
```

- [ ] **Step 3: Update all test calls to pass `0`**

In `internal/game/command/equipment_test.go`, replace every `command.HandleEquipment(sess)` with `command.HandleEquipment(sess, 0)`. There are exactly 15 call sites. Use a global replace.

**Important:** `TestProperty_HandleEquipment_AlwaysHasAllSections` checks for `=== Armor ===` and `=== Accessories ===` — these headers only appear in the single-col path (`width < 60`). After updating to `width=0` the test continues to pass correctly. Add this comment above the test:
```go
// NOTE: intentionally tests single-col path only (width=0).
// The 2-col path (width>=60) does not emit === Armor === or === Accessories === headers.
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... ./internal/gameserver/... -v 2>&1 | tail -20
```

Expected: all pass (behaviour unchanged — width=0 falls through to existing single-col path, which is the next task to implement but the existing code produces the same result since we haven't added the branch yet; the signature change itself is the only change here).

**Note:** The build will fail until the width parameter is threaded through — fix the grpc caller and tests first, then run.

- [ ] **Step 5: Run full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/command/equipment.go internal/game/command/equipment_test.go internal/gameserver/grpc_service.go
git commit -m "refactor: add width parameter to HandleEquipment; callers pass 0 or 80"
```

---

## Task 2: Implement 2-column layout

**Files:**
- Modify: `internal/game/command/equipment.go`
- Modify: `internal/game/command/equipment_test.go`

- [ ] **Step 1: Write failing tests for 2-col output**

Append to `internal/game/command/equipment_test.go`:

```go
// TestHandleEquipment_TwoCol_ArmorAppearsInLeftColumn verifies that at width>=60
// all 8 armor slot labels appear in the output.
func TestHandleEquipment_TwoCol_ArmorAppearsInLeftColumn(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80)

	armorLabels := []string{"Head:", "Torso:", "Left Arm:", "Right Arm:", "Hands:", "Left Leg:", "Right Leg:", "Feet:"}
	for _, label := range armorLabels {
		if !strings.Contains(result, label) {
			t.Errorf("expected armor label %q in 2-col output:\n%s", label, result)
		}
	}
}

// TestHandleEquipment_TwoCol_AccessoriesAppearInRightColumn verifies that at width>=60
// neck and all 10 ring slots appear in the output.
func TestHandleEquipment_TwoCol_AccessoriesAppearInRightColumn(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80)

	if !strings.Contains(result, "Neck:") {
		t.Errorf("expected 'Neck:' in 2-col output:\n%s", result)
	}
	for i := 1; i <= 5; i++ {
		leftLabel := fmt.Sprintf("Left Hand Ring %d:", i)
		rightLabel := fmt.Sprintf("Right Hand Ring %d:", i)
		if !strings.Contains(result, leftLabel) {
			t.Errorf("expected %q in 2-col output:\n%s", leftLabel, result)
		}
		if !strings.Contains(result, rightLabel) {
			t.Errorf("expected %q in 2-col output:\n%s", rightLabel, result)
		}
	}
}

// TestHandleEquipment_TwoCol_HasColumnSeparator verifies that the " | " column divider
// appears in the 2-col output.
func TestHandleEquipment_TwoCol_HasColumnSeparator(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80)

	if !strings.Contains(result, " | ") {
		t.Errorf("expected ' | ' column separator in 2-col output:\n%s", result)
	}
}

// TestHandleEquipment_TwoCol_NoSeparatorAtWidth40 verifies that narrow terminals
// get the single-column layout (no " | " separator).
func TestHandleEquipment_TwoCol_NoSeparatorAtWidth40(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 40)

	if strings.Contains(result, " | ") {
		t.Errorf("did not expect ' | ' separator at width=40:\n%s", result)
	}
}

// TestHandleEquipment_TwoCol_WeaponsBeforeColumns verifies weapons section appears
// before the 2-col grid (i.e. no " | " separator precedes the weapons header).
func TestHandleEquipment_TwoCol_WeaponsBeforeColumns(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80)

	weaponsIdx := strings.Index(result, "=== Weapons ===")
	separatorIdx := strings.Index(result, " | ")
	if weaponsIdx < 0 {
		t.Fatalf("expected '=== Weapons ===' in output:\n%s", result)
	}
	if separatorIdx < 0 {
		t.Fatalf("expected ' | ' separator in output:\n%s", result)
	}
	if weaponsIdx > separatorIdx {
		t.Errorf("expected '=== Weapons ===' before ' | ', weapons at %d, separator at %d", weaponsIdx, separatorIdx)
	}
}

// TestHandleEquipment_TwoCol_EquippedArmorAppearsInOutput verifies that an equipped
// armor item's name appears when 2-col mode is active.
func TestHandleEquipment_TwoCol_EquippedArmorAppearsInOutput(t *testing.T) {
	sess := newTestSessionWithBackpack()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{Name: "Tactical Vest"}

	result := command.HandleEquipment(sess, 80)

	if !strings.Contains(result, "Tactical Vest") {
		t.Errorf("expected 'Tactical Vest' in 2-col output:\n%s", result)
	}
}

// TestProperty_HandleEquipment_TwoCol_NeverPanics verifies that HandleEquipment never
// panics for any combination of width and session state.
func TestProperty_HandleEquipment_TwoCol_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := &session.PlayerSession{
			UID:        "test-uid",
			CharName:   "Tester",
			LoadoutSet: inventory.NewLoadoutSet(),
			Equipment:  inventory.NewEquipment(),
			Backpack:   inventory.NewBackpack(20, 100.0),
		}
		width := rapid.IntRange(0, 200).Draw(rt, "width")
		_ = command.HandleEquipment(sess, width)
	})
}
```

- [ ] **Step 2: Run to verify failures**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run "TestHandleEquipment_TwoCol|TestProperty_HandleEquipment_TwoCol" -v 2>&1 | head -20
```

Expected: FAIL — `" | "` not found (layout not yet implemented).

- [ ] **Step 3: Add `zipCols` helper to `equipment.go`**

Add these two private functions immediately before `HandleEquipment`:

```go
// equipSlotLine formats a single slot row as "  <label padded to labelW> <item>".
//
// Precondition: labelW > 0; item is the slot display value.
// Postcondition: returns a string with no trailing newline.
func equipSlotLine(label, item string, labelW int) string {
	return fmt.Sprintf("  %-*s %s", labelW, label, item)
}

// zipCols zips two string slices into a two-column layout.
// Each row is: left padded to leftW chars + " | " + right.
//
// Precondition: leftW > 0; all strings in left and right are ASCII-only
// (byte length == visible width). If any left[i] is longer than leftW, the
// separator will still appear but column alignment will be broken for that row.
// Postcondition: returns a string with one \n-terminated row per max(len(left), len(right)).
func zipCols(left, right []string, leftW int) string {
	var sb strings.Builder
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		pad := leftW - len(l)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(l)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(" | ")
		sb.WriteString(r)
		sb.WriteString("\n")
	}
	return sb.String()
}
```

- [ ] **Step 4: Add 2-col branch to `HandleEquipment`**

Replace the current armor + accessories section (lines 62–82 in the original) with a conditional that selects single-col or 2-col based on `width`:

```go
// HandleEquipment displays the complete equipment state for the player's session.
//
// When width >= 60, armor and accessory slots are rendered in two side-by-side columns.
// When width < 60, all slots are rendered in a single column (existing behaviour).
//
// Precondition: sess must not be nil; sess.LoadoutSet and sess.Equipment must not be nil.
// Postcondition: Returns a formatted multi-section string showing all weapon presets
// and all armor and accessory slots.
func HandleEquipment(sess *session.PlayerSession, width int) string {
	var sb strings.Builder

	// === Weapons === (always full-width)
	sb.WriteString("=== Weapons ===\n")
	ls := sess.LoadoutSet
	for i, preset := range ls.Presets {
		label := fmt.Sprintf("Preset %d", i+1)
		if i == ls.Active {
			label += " [active]"
		}
		sb.WriteString(label + ":\n")
		sb.WriteString("  " + inventory.SlotDisplayName("main") + ": " + formatEquippedWeapon(preset.MainHand) + "\n")
		sb.WriteString("  " + inventory.SlotDisplayName("off") + ":  " + formatEquippedWeapon(preset.OffHand) + "\n")
	}

	eq := sess.Equipment

	if width >= 60 {
		// Two-column layout: armor left, accessories right.
		// leftW is the padded width of a left-column slot line.
		const armorLabelW = 12
		const accLabelW = 21
		// leftW = 2 (indent) + armorLabelW (12) + 1 (space) + max expected item name (~20).
		// Must be >= the longest left-column line to keep columns aligned.
		// At 36, items up to 21 chars fit cleanly; longer names still appear but break alignment.
		const leftW = 36

		var leftLines []string
		for _, slot := range displayArmorSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			leftLines = append(leftLines, equipSlotLine(label, formatSlottedItem(eq.Armor[slot]), armorLabelW))
		}

		var rightLines []string
		neckLabel := inventory.SlotDisplayName("neck") + ":"
		rightLines = append(rightLines, equipSlotLine(neckLabel, formatSlottedItem(eq.Accessories[inventory.SlotNeck]), accLabelW))
		for _, slot := range displayLeftRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			rightLines = append(rightLines, equipSlotLine(label, formatSlottedItem(eq.Accessories[slot]), accLabelW))
		}
		for _, slot := range displayRightRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			rightLines = append(rightLines, equipSlotLine(label, formatSlottedItem(eq.Accessories[slot]), accLabelW))
		}

		sb.WriteString("\n")
		sb.WriteString(zipCols(leftLines, rightLines, leftW))
	} else {
		// Single-column layout (original behaviour).
		sb.WriteString("\n=== Armor ===\n")
		for _, slot := range displayArmorSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			sb.WriteString(fmt.Sprintf("  %-12s %s\n", label, formatSlottedItem(eq.Armor[slot])))
		}

		sb.WriteString("\n=== Accessories ===\n")
		neckLabel := inventory.SlotDisplayName("neck") + ":"
		sb.WriteString(fmt.Sprintf("  %-21s %s\n", neckLabel, formatSlottedItem(eq.Accessories[inventory.SlotNeck])))
		for _, slot := range displayLeftRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			sb.WriteString(fmt.Sprintf("  %-21s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
		}
		for _, slot := range displayRightRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			sb.WriteString(fmt.Sprintf("  %-21s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
```

- [ ] **Step 5: Run all equipment tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -v 2>&1 | tail -30
```

Expected: all pass. The existing tests use `width=0` (< 60) and exercise the single-col path unchanged. The new tests use `width=80`.

- [ ] **Step 6: Run full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/game/command/equipment.go internal/game/command/equipment_test.go
git commit -m "feat: 2-column eq layout — armor left, accessories right at width>=60"
```

---

## Completion Checklist

- [ ] `HandleEquipment(sess, width int)` signature
- [ ] `zipCols` and `equipSlotLine` private helpers in `equipment.go`
- [ ] 2-col branch active at `width >= 60`; single-col fallback below 60
- [ ] `grpc_service.go` passes `width=80`
- [ ] All existing tests updated to pass `0` (single-col, no regressions)
- [ ] New 2-col tests: separator present at ≥60, absent at <60; all labels visible; no panic property test
- [ ] `go test ./...` 100% green
