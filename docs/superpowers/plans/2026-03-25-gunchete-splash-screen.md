# Gunchete Splash Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the existing block-letter GUNCHETE splash screen with a three-column banner: AK-47 ASCII art (bright green, 14 cols) | GUNCHETE medium ASCII font (bright cyan, 52 cols) | machete ASCII art (bright yellow, 14 cols) = 80 cols exactly.

**Architecture:** All changes are confined to `buildWelcomeBanner()` in `internal/frontend/handlers/auth.go`. The function assembles three fixed-width column slices (left weapon, title, right weapon), pads each to uniform height, then concatenates row-by-row with ANSI color codes and resets. Tests live alongside the function in `auth_test.go`.

**Tech Stack:** Go, ANSI escape codes via `internal/frontend/telnet` constants (`BrightGreen`, `BrightCyan`, `BrightYellow`, `Reset`), `telnet.StripANSI` for test helpers.

**Spec:** `docs/superpowers/specs/2026-03-25-gunchete-splash-screen-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/frontend/handlers/auth.go` | Modify | Replace `buildWelcomeBanner()` implementation |
| `internal/frontend/handlers/auth_test.go` | Modify | Add/update banner unit tests |

---

## Task 1: Write failing tests for the new banner

**Files:**
- Modify: `internal/frontend/handlers/auth_test.go`

The existing `TestWelcomeBannerContainsKeyElements` test checks stripped content only. Add five new tests that will fail until the new banner is implemented. Note: `require` is already imported in this file.

- [ ] **Step 1: Add the five failing tests**

Open `internal/frontend/handlers/auth_test.go`. After the existing `TestWelcomeBannerContainsKeyElements` function, add:

```go
// TestBannerContainsBrightCyanAsciiArt asserts that the banner contains at least 4
// independently-colorized BrightCyan segments with non-trivial ASCII art content.
// Each title row is wrapped with its own BrightCyan+Reset pair, so we count occurrences.
func TestBannerContainsBrightCyanAsciiArt(t *testing.T) {
	banner := buildWelcomeBanner()
	// Count how many BrightCyan segments contain non-trivial content (≥3 visible chars).
	artRows := 0
	remaining := banner
	for {
		idx := strings.Index(remaining, telnet.BrightCyan)
		if idx == -1 {
			break
		}
		after := remaining[idx+len(telnet.BrightCyan):]
		resetIdx := strings.Index(after, telnet.Reset)
		if resetIdx == -1 {
			break
		}
		segment := after[:resetIdx]
		if len(strings.TrimSpace(segment)) >= 3 {
			artRows++
		}
		remaining = after[resetIdx+len(telnet.Reset):]
	}
	assert.GreaterOrEqual(t, artRows, 4,
		"banner must contain at least 4 BrightCyan segments with non-trivial ASCII art content")
}

// TestBannerContainsBrightGreen asserts that the AK-47 color code is present.
func TestBannerContainsBrightGreen(t *testing.T) {
	banner := buildWelcomeBanner()
	assert.Contains(t, banner, telnet.BrightGreen, "banner must contain BrightGreen for AK-47")
}

// TestBannerContainsBrightYellow asserts that the machete color code is present.
func TestBannerContainsBrightYellow(t *testing.T) {
	banner := buildWelcomeBanner()
	assert.Contains(t, banner, telnet.BrightYellow, "banner must contain BrightYellow for machete")
}

// TestBannerLineWidthMax80 asserts that every line is at most 80 visible characters.
func TestBannerLineWidthMax80(t *testing.T) {
	banner := buildWelcomeBanner()
	for i, line := range strings.Split(banner, "\n") {
		visible := telnet.StripANSI(line)
		assert.LessOrEqual(t, len(visible), 80,
			"line %d exceeds 80 visible chars: %q", i+1, visible)
	}
}

// TestBannerColorReset asserts that every color code is followed by Reset
// before the next color code or end of banner string.
func TestBannerColorReset(t *testing.T) {
	banner := buildWelcomeBanner()
	colors := []string{
		telnet.BrightGreen,
		telnet.BrightCyan,
		telnet.BrightYellow,
	}
	for _, color := range colors {
		start := 0
		for {
			idx := strings.Index(banner[start:], color)
			if idx == -1 {
				break
			}
			abs := start + idx
			after := banner[abs+len(color):]
			resetIdx := strings.Index(after, telnet.Reset)
			require.Greater(t, resetIdx, -1,
				"color code %q at position %d must be followed by Reset", color, abs)
			// No other color code should appear before the Reset.
			for _, other := range colors {
				otherIdx := strings.Index(after, other)
				if otherIdx != -1 {
					assert.True(t, otherIdx == -1 || otherIdx > resetIdx,
						"color %q appears before Reset after color %q", other, color)
				}
			}
			start = abs + len(color)
		}
	}
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/ -run "TestBannerContainsBrightCyanAsciiArt|TestBannerContainsBrightGreen|TestBannerContainsBrightYellow|TestBannerLineWidthMax80|TestBannerColorReset" -v 2>&1 | tail -30
```

Expected: Several FAIL results. `TestBannerContainsBrightGreen` and `TestBannerContainsBrightYellow` will fail (current banner doesn't use those colors). `TestBannerContainsBrightCyanAsciiArt` will fail (current banner wraps all title rows in a single BrightCyan block rather than per-row). `TestBannerLineWidthMax80` will fail (current block-letter lines exceed 80 chars).

- [ ] **Step 3: Commit the failing tests**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/auth_test.go
git commit -m "test(splash): add failing tests for Gunchete weapon banner"
```

---

## Task 2: Implement the new banner

**Files:**
- Modify: `internal/frontend/handlers/auth.go`

Replace the body of `buildWelcomeBanner()` with the three-column weapon banner. The implementation assembles three `[]string` slices (left=AK-47, center=GUNCHETE title, right=machete), pads them to equal height, then joins row-by-row.

> **Critical — column widths:** Every row in `ak47` and `machete` must be **exactly 14 visible characters**. Every row in `title` must be **exactly 52 visible characters**. Use the debug helper in Step 2 to verify before committing. Wrong widths cause visual misalignment and will fail `TestBannerLineWidthMax80`.

- [ ] **Step 1: Replace `buildWelcomeBanner()` with the new implementation**

Replace the existing function body (lines 36–53 of `auth.go`) with:

```go
// buildWelcomeBanner returns the connection banner with the current version embedded.
// Layout: AK-47 (14 cols, bright green) | GUNCHETE title (52 cols, bright cyan) | machete (14 cols, bright yellow)
// Total visible width per art row: exactly 80 characters.
func buildWelcomeBanner() string {
	// AK-47 ASCII art — each row must be exactly 14 visible characters.
	// Verify with: len(telnet.StripANSI(row)) == 14
	ak47 := []string{
		` _________    `,
		`|  _  |   |=> `,
		`|_| |_|===|   `,
		`  | |  \__/   `,
		`  |_|         `,
		` _| |_        `,
		`|_____|       `,
	}

	// GUNCHETE medium ASCII title — each row must be exactly 52 visible characters.
	// Verify with: len(telnet.StripANSI(row)) == 52
	// Note: row 2 contains a backtick and requires string concatenation to embed in Go source.
	title := []string{
		`  ___  _   _ _  _  ___  _  _  ___  ___ ___          `,
		` / __|| | | | \| |/ __|| || || __||_  |_  |         `,
		`| (_ || |_| | .` + "`" + `| | (__ | __ || _|  / /  / /        `,
		` \___/ \___/|_|\_|\___||_||_||___| /_/  /_/         `,
		`                                                    `,
		`                                                    `,
		`                                                    `,
	}

	// Machete ASCII art — each row must be exactly 14 visible characters.
	// Verify with: len(telnet.StripANSI(row)) == 14
	machete := []string{
		`         _    `,
		`        / \   `,
		`  _____/   \  `,
		` /__________\ `,
		`      |       `,
		`      |       `,
		`     (_)      `,
	}

	// Normalize column heights by padding shorter slices with blank rows at the bottom.
	maxRows := len(ak47)
	if len(title) > maxRows {
		maxRows = len(title)
	}
	if len(machete) > maxRows {
		maxRows = len(machete)
	}
	pad := func(col []string, width, count int) []string {
		blank := strings.Repeat(" ", width)
		for len(col) < count {
			col = append(col, blank)
		}
		return col
	}
	ak47 = pad(ak47, 14, maxRows)
	title = pad(title, 52, maxRows)
	machete = pad(machete, 14, maxRows)

	// Build banner rows. Each row: BrightGreen+ak47+Reset | BrightCyan+title+Reset | BrightYellow+machete+Reset
	var sb strings.Builder
	sb.WriteString("\n")
	for i := 0; i < maxRows; i++ {
		sb.WriteString(telnet.BrightGreen + ak47[i] + telnet.Reset)
		sb.WriteString(telnet.BrightCyan + title[i] + telnet.Reset)
		sb.WriteString(telnet.BrightYellow + machete[i] + telnet.Reset)
		sb.WriteString("\n")
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

The `version` package is already imported in `auth.go` (line 18) — no import changes needed.

- [ ] **Step 2: Verify column widths with the debug helper**

Add this temporary snippet inside `buildWelcomeBanner()` just before `return sb.String()`, run the test suite, check output, then remove it:

```go
// Temporary debug — REMOVE BEFORE COMMIT
for i, row := range ak47 {
    if n := len(telnet.StripANSI(row)); n != 14 {
        panic(fmt.Sprintf("ak47[%d] is %d chars, want 14: %q", i, n, row))
    }
}
for i, row := range title {
    if n := len(telnet.StripANSI(row)); n != 52 {
        panic(fmt.Sprintf("title[%d] is %d chars, want 52: %q", i, n, row))
    }
}
for i, row := range machete {
    if n := len(telnet.StripANSI(row)); n != 14 {
        panic(fmt.Sprintf("machete[%d] is %d chars, want 14: %q", i, n, row))
    }
}
```

Run:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go run ./cmd/frontend/... 2>&1 | head -5
```

Or simply run tests — the panic will surface in test output if any row has the wrong width. Fix any rows that panic, then remove the debug block.

- [ ] **Step 3: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/ -v 2>&1 | tail -40
```

Expected: All tests PASS including the five new banner tests and the existing `TestWelcomeBannerContainsKeyElements`.

- [ ] **Step 4: Run the full project test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All packages pass (required by SWENG-6 before committing).

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/auth.go
git commit -m "feat(splash): replace block-letter title with weapon flanked Gunchete banner"
```

---

## Task 3: Visual verification

- [ ] **Step 1: Final check — verify no regressions in auth integration tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/ -run "TestHandleSession" -v 2>&1 | tail -20
```

Expected: All `TestHandleSession_*` tests pass (these exercise the full auth flow end-to-end over a real telnet connection, including reading through the banner).
