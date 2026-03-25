# Gunchete Splash Screen v2 — Stacked Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three-column weapon-flanking-title banner with a stacked layout: horizontal AK-47 art above the original GUNCHETE block-letter title, horizontal machete art below it.

**Architecture:** All changes are in `buildWelcomeBanner()` in `internal/frontend/handlers/auth.go`. The function is rewritten to emit: AK-47 art block (BrightGreen, row-by-row), blank line, original Unicode block-letter title (Bold+BrightCyan), blank line, machete art block (BrightYellow, row-by-row), blank line, then the unchanged subtitle/version/commands footer. Tests live in `auth_test.go` (package `handlers`, white-box access to `buildWelcomeBanner`).

**Tech Stack:** Go, ANSI constants from `internal/frontend/telnet` (`BrightGreen`, `BrightCyan`, `BrightYellow`, `Bold`, `Reset`, `Dim`, `Green`), `telnet.StripANSI` for visible-width measurement in tests.

**Spec:** `docs/superpowers/specs/2026-03-25-gunchete-splash-screen-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/frontend/handlers/auth.go` | Modify | Replace `buildWelcomeBanner()` with stacked layout |
| `internal/frontend/handlers/auth_test.go` | Modify | Add ordering tests (TEST-6, TEST-7); update TestBannerColorReset to check `Bold` |

---

## Task 1: Update tests for new layout

**Files:**
- Modify: `internal/frontend/handlers/auth_test.go`

The existing tests (`TestBannerContainsBrightCyanAsciiArt`, `TestBannerContainsBrightGreen`, `TestBannerContainsBrightYellow`, `TestBannerLineWidthMax80`, `TestBannerColorReset`) mostly cover the new layout already. Two changes needed:

1. `TestBannerColorReset` currently only checks three color codes — it must also check `telnet.Bold`.
2. Two new ordering tests are needed (TEST-6 and TEST-7): gun above title, machete below title.

- [ ] **Step 1: Update TestBannerColorReset to include Bold**

In `auth_test.go`, find `TestBannerColorReset`. The `colors` slice currently contains three entries. Add `telnet.Bold`:

```go
colors := []string{
    telnet.BrightGreen,
    telnet.BrightCyan,
    telnet.BrightYellow,
    telnet.Bold,
}
```

The test body does not need any other changes — the loop already handles any number of color codes.

- [ ] **Step 2: Run the updated test to verify it currently fails (Bold not reset in three-column impl)**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -run TestBannerColorReset -v 2>&1 | tail -10
```

Expected: FAIL (Bold is used but the current three-column banner may not use Bold at all — check whether it passes or fails; either is fine, record the result).

- [ ] **Step 3: Add TestBannerGunAboveTitle**

After `TestBannerColorReset`, add:

```go
// TestBannerGunAboveTitle asserts that the BrightGreen (AK-47) block appears
// on an earlier line than the first BrightCyan (title) line.
func TestBannerGunAboveTitle(t *testing.T) {
	banner := buildWelcomeBanner()
	lines := strings.Split(banner, "\n")
	firstGreen := -1
	firstCyan := -1
	for i, line := range lines {
		if firstGreen == -1 && strings.Contains(line, telnet.BrightGreen) {
			firstGreen = i
		}
		if firstCyan == -1 && strings.Contains(line, telnet.BrightCyan) {
			firstCyan = i
		}
	}
	require.Greater(t, firstGreen, -1, "BrightGreen (AK-47) must appear in banner")
	require.Greater(t, firstCyan, -1, "BrightCyan (title) must appear in banner")
	assert.Less(t, firstGreen, firstCyan,
		"BrightGreen (AK-47) line %d must be before BrightCyan (title) line %d", firstGreen, firstCyan)
}
```

- [ ] **Step 4: Add TestBannerMacheteBelowTitle**

```go
// TestBannerMacheteBelowTitle asserts that the BrightYellow (machete) block appears
// on a later line than the last BrightCyan (title) line.
func TestBannerMacheteBelowTitle(t *testing.T) {
	banner := buildWelcomeBanner()
	lines := strings.Split(banner, "\n")
	lastCyan := -1
	firstYellow := -1
	for i, line := range lines {
		if strings.Contains(line, telnet.BrightCyan) {
			lastCyan = i
		}
		if firstYellow == -1 && strings.Contains(line, telnet.BrightYellow) {
			firstYellow = i
		}
	}
	require.Greater(t, lastCyan, -1, "BrightCyan (title) must appear in banner")
	require.Greater(t, firstYellow, -1, "BrightYellow (machete) must appear in banner")
	assert.Less(t, lastCyan, firstYellow,
		"last BrightCyan (title) line %d must be before BrightYellow (machete) line %d", lastCyan, firstYellow)
}
```

- [ ] **Step 5: Run new tests to confirm they fail against current banner**

```bash
mise run go test ./internal/frontend/handlers/... -run "TestBannerGunAboveTitle|TestBannerMacheteBelowTitle" -v 2>&1 | tail -15
```

Expected: FAIL — current banner has BrightGreen/BrightYellow on the same rows as BrightCyan (three-column layout).

- [ ] **Step 6: Commit the updated tests**

```bash
git add internal/frontend/handlers/auth_test.go
git commit -m "test(splash): add ordering tests and Bold check for stacked banner layout"
```

---

## Task 2: Rewrite buildWelcomeBanner with stacked layout

**Files:**
- Modify: `internal/frontend/handlers/auth.go`

Replace the entire `buildWelcomeBanner()` function body. The original title art (Unicode block letters) comes from `git show 4ec04c85~1:internal/frontend/handlers/auth.go` — it is the `██` style, 6 lines, ~72 chars wide.

- [ ] **Step 1: Design and verify the AK-47 art**

The AK-47 art must be a horizontal side-profile silhouette, ~58 visible chars wide, 5 rows tall. Use the following art (verified character counts shown):

```
        ____________________________________________         (52 chars)
  ,--. |____________________________________________|===>    (57 chars)
 (    )|  [======================================]  |        (52 chars)
  `--' |____________________________________________|        (53 chars)
        |___|                                                 (12 chars)
```

**Verify widths before coding.** Count each row's visible characters:
- Row 1: 8 spaces + 44 underscores = 52 ✓
- Row 2: `  ,--. ` (7) + `|____________________________________________|` (46) + `===>` (4) = 57 ✓
- Row 3: ` (    )` (7) + `|  [======================================]  |` (46) = 53 ✓
- Row 4: `  ` + `` `--' `` (6) + `|____________________________________________|` (46) = 52 ✓
- Row 5: 8 spaces + `|___|` (5) = 13 ✓

All rows are ≤ 80. Good.

- [ ] **Step 2: Design and verify the machete art**

The machete art must be a horizontal side-profile silhouette, ~58 visible chars wide, 4 rows tall. Blade on the left (long and flat), handle+guard on the right.

```
  __________________________________________________________,    (60 chars)
 /__________________________________________________________|     (59 chars)
 |                                                          |=|   (62 chars — OVER 80? No, 62 is fine)
  \_________________________________________________________|_|   (62 chars)
```

Wait, let me recount row 3: ` |` (2) + 58 spaces + `|=|` (3) = 63. That's fine (< 80).

Use this art:
```
  _______________________________________________________,
 /________________________________________________________|
 |                                                        |=|
  \________________________________________________________|_|
```

Count:
- Row 1: 2 + 55 underscores + `,` = 58 ✓
- Row 2: ` /` + 56 underscores + `|` = 58 ✓
- Row 3: ` |` + 56 spaces + `|=|` = 61 ✓
- Row 4: ` \` + 56 underscores + `|_|` = 61 ✓

All ≤ 80. Good.

**IMPORTANT:** The implementer MUST verify all character counts by running:
```go
for i, row := range ak47 {
    if n := len(telnet.StripANSI(row)); n > 80 {
        panic(fmt.Sprintf("ak47 row %d is %d chars", i, n))
    }
}
```
or equivalent, before committing. The counts above are illustrative — adjust art until all rows are ≤ 80 visible chars.

- [ ] **Step 3: Replace buildWelcomeBanner()**

Replace the entire function with:

```go
// buildWelcomeBanner returns the connection banner with the current version embedded.
//
// Layout (top to bottom):
//   1. Horizontal AK-47 ASCII art (BrightGreen)
//   2. GUNCHETE Unicode block-letter title (Bold + BrightCyan)
//   3. Horizontal machete ASCII art (BrightYellow)
//   4. Subtitle, version, instructions (unchanged)
//
// All weapon art rows are ≤ 80 visible characters.
// Precondition: none.
// Postcondition: returns a complete, non-empty banner string.
func buildWelcomeBanner() string {
	// AK-47 horizontal art — side profile, barrel pointing right.
	// Each row MUST be ≤ 80 visible characters (verified by TestBannerLineWidthMax80).
	ak47 := []string{
		`        ____________________________________________`,
		`  ,--. |____________________________________________|===>`,
		` (    )|  [======================================]  |`,
		`  ` + "`" + `--' |____________________________________________|`,
		`        |___|`,
	}

	// Machete horizontal art — blade on left, handle+guard on right.
	// Each row MUST be ≤ 80 visible characters.
	machete := []string{
		`  _______________________________________________________,`,
		` /________________________________________________________|`,
		` |                                                        |=|`,
		`  \________________________________________________________|_|`,
	}

	var sb strings.Builder
	sb.WriteString("\n")

	// AK-47 block
	for _, row := range ak47 {
		sb.WriteString(telnet.BrightGreen + row + telnet.Reset + "\n")
	}

	sb.WriteString("\n")

	// Original GUNCHETE Unicode block-letter title (Bold + BrightCyan).
	// Single Reset after BrightCyan clears both Bold and BrightCyan.
	sb.WriteString(telnet.Bold + telnet.BrightCyan + `
  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗
 ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝
 ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗
 ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝
 ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗
  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝` + telnet.Reset + "\n")

	sb.WriteString("\n")

	// Machete block
	for _, row := range machete {
		sb.WriteString(telnet.BrightYellow + row + telnet.Reset + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(telnet.BrightYellow + `  Post-Collapse Portland, OR — A Dystopian Sci-Fi MUD` + telnet.Reset + "\n")
	sb.WriteString(telnet.Dim + `  ` + version.Version + telnet.Reset + "\n")
	sb.WriteString("\n")
	sb.WriteString(`  Type ` + telnet.Green + `login` + telnet.Reset + ` to connect.` + "\n")
	sb.WriteString(`  Type ` + telnet.Green + `register` + telnet.Reset + ` to create an account.` + "\n")
	sb.WriteString(`  Type ` + telnet.Green + `quit` + telnet.Reset + ` to disconnect.` + "\n")

	return sb.String()
}
```

**Note on the title block:** The title string literal begins with a `\n` immediately after the opening backtick. This means the first `\n` after `BrightCyan` is part of the literal and the title lines start on the next line. The `Reset` comes after the last title line. `TestBannerColorReset` will verify `Bold` and `BrightCyan` are both followed by `Reset` — since `Bold` immediately precedes `BrightCyan` and `BrightCyan` has a `Reset`, the `Bold` check in `TestBannerColorReset` will find `Reset` after scanning past `BrightCyan` (which itself precedes `Reset`). **If this causes `TestBannerColorReset` to fail for `Bold`** (because the scanner finds `BrightCyan` before `Reset` after seeing `Bold`), split the title into per-row wrapping:

```go
title := []string{
    `  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗`,
    ` ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝`,
    ` ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗`,
    ` ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝`,
    ` ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗`,
    `  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝`,
}
for _, row := range title {
    sb.WriteString(telnet.Bold + telnet.BrightCyan + row + telnet.Reset + "\n")
}
```

Use whichever form makes the tests pass. The per-row form is safer.

- [ ] **Step 4: Run all banner tests**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -run "TestBanner|TestWelcome" -v 2>&1 | tail -30
```

Expected: all pass. If any fail:
- `TestBannerLineWidthMax80` failing → a weapon art row is too wide; count characters and trim
- `TestBannerColorReset` failing for `Bold` → use the per-row title form described above
- `TestBannerGunAboveTitle` or `TestBannerMacheteBelowTitle` failing → check ordering in `buildWelcomeBanner`
- `TestBannerContainsBrightCyanAsciiArt` failing (needs ≥4 BrightCyan segments) → use per-row title form (each row is its own BrightCyan segment)

- [ ] **Step 5: Run full test suite**

```bash
mise run go test ./... 2>&1 | tail -20
```

Expected: all pass (no regressions).

- [ ] **Step 6: Commit**

```bash
git add internal/frontend/handlers/auth.go
git commit -m "feat(splash): rewrite banner to stacked layout — AK-47 above, machete below GUNCHETE title"
```

---

## ASCII Art Reference

The art below is the design target. Exact characters MUST be adjusted until all rows are ≤ 80 visible chars and the silhouettes are recognizable. Use `telnet.StripANSI(row)` to measure.

### AK-47 (BrightGreen)

```
        ____________________________________________
  ,--. |____________________________________________|===>
 (    )|  [======================================]  |
  `--' |____________________________________________|
        |___|
```

Feature checklist:
- [ ] Long horizontal barrel visible (the `|===>` at row 2)
- [ ] Rectangular receiver body (the `|__|` structure)
- [ ] Magazine curve visible (the `( )` at left)
- [ ] Stock suggestion at left (the `,--.'` and `` `--' ``)

### Machete (BrightYellow)

```
  _______________________________________________________,
 /________________________________________________________|
 |                                                        |=|
  \________________________________________________________|_|
```

Feature checklist:
- [ ] Long flat blade on the left (the `___` expanse)
- [ ] Narrow tip (the `,` at top-right of blade)
- [ ] Guard/cross-piece (the `|=|` structure)
- [ ] Short handle stub (the `|_|` at right)
