# Gunchete Splash Screen v2 — Stacked Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three-column weapon-flanking-title banner with a stacked layout: horizontal AK-47 art above the original GUNCHETE block-letter title, horizontal machete art below it.

**Architecture:** All changes are in `buildWelcomeBanner()` in `internal/frontend/handlers/auth.go`. The function is rewritten to emit: AK-47 art block (BrightGreen, row-by-row), blank line, original Unicode block-letter title (per-row Bold+BrightCyan+Reset), blank line, machete art block (BrightYellow, row-by-row), blank line, then the unchanged subtitle/version/commands footer. Tests live in `auth_test.go` (package `handlers`, white-box access to `buildWelcomeBanner`).

**Tech Stack:** Go, ANSI constants from `internal/frontend/telnet` (`BrightGreen`, `BrightCyan`, `BrightYellow`, `Bold`, `Reset`, `Dim`, `Green`), `telnet.StripANSI` for visible-width measurement in tests.

**Spec:** `docs/superpowers/specs/2026-03-25-gunchete-splash-screen-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/frontend/handlers/auth.go` | Modify | Replace `buildWelcomeBanner()` with stacked layout |
| `internal/frontend/handlers/auth_test.go` | Modify | Add `TestBannerBoldReset` and two ordering tests |

---

## Key design constraint: Bold+BrightCyan adjacency

The title rows use `telnet.Bold + telnet.BrightCyan + row + telnet.Reset`. The existing `TestBannerColorReset` checks that no other color from its `colors` slice appears before `Reset` after each color code. If `Bold` were added to that slice, the test would fail on every title row because `BrightCyan` appears between `Bold` and `Reset`.

**Therefore:** Do NOT add `Bold` to `TestBannerColorReset`. Instead, add a dedicated `TestBannerBoldReset` that checks `Bold` → `Reset` without the inter-color constraint (since the spec explicitly permits the `Bold+BrightCyan+Reset` pattern).

**Therefore:** The title MUST use the per-row form — each title row wrapped individually in `Bold+BrightCyan+row+Reset`. This also ensures `TestBannerContainsBrightCyanAsciiArt` (which counts BrightCyan-delimited segments ≥ 4) continues to pass.

---

## Task 1: Update tests for new layout

**Files:**
- Modify: `internal/frontend/handlers/auth_test.go`

- [ ] **Step 1: Add TestBannerBoldReset**

In `auth_test.go`, after `TestBannerColorReset` (ends around line 377), add:

```go
// TestBannerBoldReset asserts that every Bold occurrence is followed by Reset
// before the end of the banner. Bold is always immediately followed by BrightCyan
// on title rows (per spec IMPL-2), so this test does not require Reset before BrightCyan.
func TestBannerBoldReset(t *testing.T) {
	banner := buildWelcomeBanner()
	remaining := banner
	for {
		idx := strings.Index(remaining, telnet.Bold)
		if idx == -1 {
			break
		}
		after := remaining[idx+len(telnet.Bold):]
		resetIdx := strings.Index(after, telnet.Reset)
		assert.Greater(t, resetIdx, -1,
			"Bold at byte offset %d must be followed by Reset", idx)
		if resetIdx == -1 {
			break
		}
		remaining = after[resetIdx+len(telnet.Reset):]
	}
}
```

- [ ] **Step 2: Add TestBannerGunAboveTitle**

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
		"BrightGreen (AK-47) line %d must be before BrightCyan (title) line %d",
		firstGreen, firstCyan)
}
```

- [ ] **Step 3: Add TestBannerMacheteBelowTitle**

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
		"last BrightCyan (title) line %d must be before BrightYellow (machete) line %d",
		lastCyan, firstYellow)
}
```

- [ ] **Step 4: Run new tests to confirm they fail against the current three-column banner**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -run "TestBannerGunAboveTitle|TestBannerMacheteBelowTitle|TestBannerBoldReset" -v 2>&1 | tail -20
```

Expected: `TestBannerGunAboveTitle` and `TestBannerMacheteBelowTitle` FAIL (current banner has BrightGreen/BrightYellow on the same rows as BrightCyan). `TestBannerBoldReset` may PASS (current banner may not use Bold) or FAIL — either is fine.

- [ ] **Step 5: Commit the test additions**

```bash
git add internal/frontend/handlers/auth_test.go
git commit -m "test(splash): add Bold reset, gun-above-title, and machete-below-title tests"
```

---

## Task 2: Rewrite buildWelcomeBanner with stacked layout

**Files:**
- Modify: `internal/frontend/handlers/auth.go`

Replace the entire `buildWelcomeBanner()` function body.

- [ ] **Step 1: Verify AK-47 art character counts**

The AK-47 art rows and their verified visible character counts:

```
Row 1: `        ____________________________________________`
        8 spaces + 44 underscores = 52 chars ✓

Row 2: `  ,--. |____________________________________________|===>`
        "  ,--. " (7) + "|" (1) + 44 underscores (44) + "|" (1) + "===>" (4) = 57 chars ✓

Row 3: ` (    )|  [======================================]  |`
        " (    )" (7) + "|  [" (4) + 38 equals (38) + "]  |" (4) = 53 chars ✓

Row 4: "  `--' |____________________________________________|"
        "  `--' " (7) + "|" (1) + 44 underscores (44) + "|" (1) = 53 chars ✓
        (Note: row 4 uses a backtick — requires string concatenation in Go source)

Row 5: `        |___|`
        8 spaces + "|___|" (5) = 13 chars ✓
```

All rows are ≤ 80 visible chars. ✓

- [ ] **Step 2: Verify machete art character counts**

```
Row 1: `  _______________________________________________________,`
        2 spaces + 55 underscores + "," = 58 chars ✓

Row 2: ` /________________________________________________________|`
        " /" (2) + 56 underscores + "|" (1) = 59 chars ✓

Row 3: ` |                                                        |=|`
        " |" (2) + 56 spaces + "|=|" (3) = 61 chars ✓

Row 4: `  \________________________________________________________|_|`
        "  \" (3) + 56 underscores + "|_|" (3) = 62 chars ✓
```

All rows are ≤ 80 visible chars. ✓

- [ ] **Step 3: Replace buildWelcomeBanner()**

Replace the entire function with:

```go
// buildWelcomeBanner returns the connection banner with the current version embedded.
//
// Layout (top to bottom):
//  1. Horizontal AK-47 ASCII art (BrightGreen, per-row)
//  2. GUNCHETE Unicode block-letter title (Bold + BrightCyan, per-row)
//  3. Horizontal machete ASCII art (BrightYellow, per-row)
//  4. Subtitle, version, instructions (unchanged)
//
// Each row is independently wrapped: color + row + Reset.
// For title rows: Bold + BrightCyan + row + Reset (single Reset clears both).
// Precondition: none.
// Postcondition: returns a complete, non-empty banner string.
func buildWelcomeBanner() string {
	// AK-47 horizontal art — side profile, barrel pointing right.
	// Each row is ≤ 80 visible characters (verified by TestBannerLineWidthMax80).
	ak47 := []string{
		`        ____________________________________________`,
		`  ,--. |____________________________________________|===>`,
		` (    )|  [======================================]  |`,
		`  ` + "`" + `--' |____________________________________________|`,
		`        |___|`,
	}

	// GUNCHETE Unicode block-letter title — original art, per-row.
	// Each row wrapped independently so TestBannerContainsBrightCyanAsciiArt
	// counts ≥ 4 distinct BrightCyan segments.
	title := []string{
		`  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗`,
		` ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝`,
		` ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗`,
		` ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝`,
		` ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗`,
		`  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝`,
	}

	// Machete horizontal art — blade on left, handle+guard on right.
	// Each row is ≤ 80 visible characters.
	machete := []string{
		`  _______________________________________________________,`,
		` /________________________________________________________|`,
		` |                                                        |=|`,
		`  \________________________________________________________|_|`,
	}

	var sb strings.Builder
	sb.WriteString("\n")

	// AK-47 block: each row independently colorized.
	for _, row := range ak47 {
		sb.WriteString(telnet.BrightGreen + row + telnet.Reset + "\n")
	}

	sb.WriteString("\n")

	// Title block: Bold + BrightCyan per row. Single Reset clears both attributes.
	for _, row := range title {
		sb.WriteString(telnet.Bold + telnet.BrightCyan + row + telnet.Reset + "\n")
	}

	sb.WriteString("\n")

	// Machete block: each row independently colorized.
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

- [ ] **Step 4: Run all banner tests**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/frontend/handlers/... -run "TestBanner|TestWelcome" -v 2>&1 | tail -30
```

Expected: all pass. Diagnose failures by test name:

- `TestBannerLineWidthMax80` → a weapon art row is too wide; use `telnet.StripANSI(row)` to measure and trim
- `TestBannerColorReset` → a BrightGreen/BrightCyan/BrightYellow row has a color appearing before its Reset; check the weapon art slices for stray color codes
- `TestBannerBoldReset` → a title row's Bold has no Reset; verify per-row loop uses `telnet.Bold + telnet.BrightCyan + row + telnet.Reset`
- `TestBannerContainsBrightCyanAsciiArt` → the per-row title loop must produce ≥ 4 BrightCyan segments; 6 title rows each independently wrapped = 6 segments, so this passes
- `TestBannerGunAboveTitle` or `TestBannerMacheteBelowTitle` → check ordering of `ak47`, `title`, `machete` loops in the function

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./... 2>&1 | tail -20
```

Expected: all pass (no regressions).

- [ ] **Step 6: Commit**

```bash
git add internal/frontend/handlers/auth.go
git commit -m "feat(splash): rewrite banner to stacked layout — AK-47 above, machete below GUNCHETE title"
```
