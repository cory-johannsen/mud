# Console Scroll (PgUp/PgDn) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow players to scroll back through the Console section using PgUp/PgDn, with live messages silently accumulating and a "[N new messages]" indicator shown while scrolled.

**Architecture:** `telnet.Conn` gains a 1000-line ring buffer (`consoleBuf`), a `scrollOffset` (lines scrolled back), and a `pendingNew` counter. `WriteConsole` always appends to the buffer; when `scrollOffset > 0` it increments `pendingNew` instead of rendering. `tryReadEscapeSeq` is extended to recognize `ESC [ 5 ~` / `ESC [ 6 ~`. `commandLoop` dispatches PgUp/PgDn sentinels to `ScrollUp`/`ScrollDown` on `Conn`.

**Tech Stack:** Go, `pgregory.net/rapid` for property-based tests, VT100 escape sequences

---

### Task 1: Extend `tryReadEscapeSeq` for PgUp/PgDn

**Files:**
- Modify: `internal/frontend/telnet/conn.go:172-193`
- Modify: `internal/frontend/telnet/conn_test.go` (add new test cases)

**Step 1: Write the failing tests**

Add to `internal/frontend/telnet/conn_test.go`:

```go
func TestTryReadEscapeSeq_PgUp(t *testing.T) {
    // ESC already consumed; feed [ 5 ~
    raw := &bytes.Buffer{}
    raw.Write([]byte{'[', '5', '~'})
    c := &Conn{reader: bufio.NewReader(raw)}
    got := c.tryReadEscapeSeq()
    assert.Equal(t, "\x00PGUP", got)
}

func TestTryReadEscapeSeq_PgDn(t *testing.T) {
    raw := &bytes.Buffer{}
    raw.Write([]byte{'[', '6', '~'})
    c := &Conn{reader: bufio.NewReader(raw)}
    got := c.tryReadEscapeSeq()
    assert.Equal(t, "\x00PGDN", got)
}

func TestTryReadEscapeSeq_UnrecognizedDigitSwallowed(t *testing.T) {
    // ESC [ 3 ~ — unrecognized, should return "" and consume all bytes
    raw := &bytes.Buffer{}
    raw.Write([]byte{'[', '3', '~'})
    c := &Conn{reader: bufio.NewReader(raw)}
    got := c.tryReadEscapeSeq()
    assert.Equal(t, "", got)
    assert.Equal(t, 0, raw.Len()) // all bytes consumed
}

func TestTryReadEscapeSeq_ArrowsUnchanged(t *testing.T) {
    for _, tc := range []struct{ in byte; want string }{
        {'A', "\x00UP"}, {'B', "\x00DOWN"},
    } {
        raw := &bytes.Buffer{}
        raw.Write([]byte{'[', tc.in})
        c := &Conn{reader: bufio.NewReader(raw)}
        assert.Equal(t, tc.want, c.tryReadEscapeSeq())
    }
}
```

**Step 2: Run to verify failures**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/telnet/... -run "TestTryReadEscapeSeq_PgUp|TestTryReadEscapeSeq_PgDn|TestTryReadEscapeSeq_UnrecognizedDigit" -v
```

Expected: FAIL (assertions fail — `\x00PGUP` not returned)

**Step 3: Implement**

Replace `tryReadEscapeSeq` in `internal/frontend/telnet/conn.go`:

```go
// tryReadEscapeSeq attempts to read a VT100 CSI escape sequence after ESC (0x1B)
// has been consumed. Returns a sentinel string if the sequence is recognized,
// or "" to indicate an unrecognized sequence (all bytes consumed).
//
// Recognized sequences:
//   ESC [ A    → "\x00UP"
//   ESC [ B    → "\x00DOWN"
//   ESC [ 5 ~  → "\x00PGUP"
//   ESC [ 6 ~  → "\x00PGDN"
//
// Precondition: ESC byte has already been read from c.reader.
// Postcondition: The full CSI sequence has been consumed.
func (c *Conn) tryReadEscapeSeq() string {
	next, err := c.reader.ReadByte()
	if err != nil || next != '[' {
		return ""
	}
	final, err := c.reader.ReadByte()
	if err != nil {
		return ""
	}
	switch final {
	case 'A':
		return "\x00UP"
	case 'B':
		return "\x00DOWN"
	case '5', '6':
		// PgUp/PgDn: one more byte expected ('~')
		term, err := c.reader.ReadByte()
		if err != nil || term != '~' {
			return ""
		}
		if final == '5' {
			return "\x00PGUP"
		}
		return "\x00PGDN"
	default:
		return ""
	}
}
```

**Step 4: Run tests to verify pass**

```bash
go test ./internal/frontend/telnet/... -run "TestTryReadEscapeSeq" -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/frontend/telnet/conn.go internal/frontend/telnet/conn_test.go
git commit -m "feat: extend tryReadEscapeSeq to recognize PgUp/PgDn sequences"
```

---

### Task 2: Add scroll state to `Conn`

**Files:**
- Modify: `internal/frontend/telnet/conn.go:33-50` (Conn struct)

**Step 1: Add fields to the struct**

In the `Conn` struct, inside the `// Split-screen state (guarded by mu)` block, add:

```go
// Scroll buffer (guarded by mu)
consoleBuf   []string // ring buffer of console lines, max consoleBufMax
scrollOffset int      // lines scrolled back from live; 0 = live
pendingNew   int      // new lines received while scrolled back
```

Also add the constant near the top of `conn.go` (after imports):

```go
// consoleBufMax is the maximum number of console lines retained in the scroll buffer.
const consoleBufMax = 1000
```

**Step 2: Write failing tests for ring buffer behavior**

Add to `internal/frontend/telnet/conn_test.go`:

```go
func TestConsoleBuf_RingTruncatesAt1000(t *testing.T) {
    c := &Conn{}
    for i := 0; i < 1100; i++ {
        c.appendConsoleLine(fmt.Sprintf("line %d", i))
    }
    c.mu.Lock()
    n := len(c.consoleBuf)
    c.mu.Unlock()
    assert.Equal(t, consoleBufMax, n)
    // oldest entry should be line 100 (lines 0-99 were dropped)
    c.mu.Lock()
    first := c.consoleBuf[0]
    c.mu.Unlock()
    assert.Equal(t, "line 100", first)
}

func TestConsoleBuf_PendingNewIncrementsWhenScrolled(t *testing.T) {
    c := &Conn{}
    c.mu.Lock()
    c.scrollOffset = 5
    c.mu.Unlock()
    c.appendConsoleLine("msg")
    c.mu.Lock()
    pn := c.pendingNew
    c.mu.Unlock()
    assert.Equal(t, 1, pn)
}

func TestConsoleBuf_PendingNewNotIncrementedWhenLive(t *testing.T) {
    c := &Conn{}
    c.appendConsoleLine("msg")
    c.mu.Lock()
    pn := c.pendingNew
    c.mu.Unlock()
    assert.Equal(t, 0, pn)
}
```

**Step 3: Run to verify failures**

```bash
go test ./internal/frontend/telnet/... -run "TestConsoleBuf" -v
```

Expected: FAIL (appendConsoleLine undefined)

**Step 4: Implement `appendConsoleLine`**

Add to `internal/frontend/telnet/conn.go`:

```go
// appendConsoleLine appends a line to the console scroll buffer, enforcing the
// ring limit by dropping the oldest entry when at capacity.
// If scrollOffset > 0, increments pendingNew instead of triggering a redraw.
//
// Precondition: line must be a single display line (no embedded newlines).
// Postcondition: consoleBuf length <= consoleBufMax.
func (c *Conn) appendConsoleLine(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consoleBuf = append(c.consoleBuf, line)
	if len(c.consoleBuf) > consoleBufMax {
		c.consoleBuf = c.consoleBuf[len(c.consoleBuf)-consoleBufMax:]
	}
	if c.scrollOffset > 0 {
		c.pendingNew++
	}
}
```

**Step 5: Run tests to verify pass**

```bash
go test ./internal/frontend/telnet/... -run "TestConsoleBuf" -v
```

Expected: All PASS

**Step 6: Commit**

```bash
git add internal/frontend/telnet/conn.go internal/frontend/telnet/conn_test.go
git commit -m "feat: add consoleBuf ring buffer and scroll state to Conn"
```

---

### Task 3: Implement `redrawConsole` and wire `WriteConsole` to buffer

**Files:**
- Modify: `internal/frontend/telnet/screen.go`

**Step 1: Write failing test for `redrawConsole` slice selection**

Add to `internal/frontend/telnet/conn_test.go`:

```go
func TestRedrawConsole_SliceSelection(t *testing.T) {
    // Build a conn with known consoleBuf and scrollOffset.
    // Verify the correct lines are selected for rendering.
    // We test the slice logic via a helper rather than full VT100 output.
    buf := make([]string, 50)
    for i := range buf {
        buf[i] = fmt.Sprintf("line-%d", i)
    }
    c := &Conn{consoleBuf: buf, scrollOffset: 0, height: 24, width: 80}
    // consoleHeight = 24 - RoomRegionRows - 2 = 24 - 10 - 2 = 12
    lines := c.consoleSlice()
    assert.Equal(t, 12, len(lines))
    assert.Equal(t, "line-49", lines[len(lines)-1]) // most recent at bottom

    // Scrolled back one page (12 lines)
    c.mu.Lock()
    c.scrollOffset = 12
    c.mu.Unlock()
    lines = c.consoleSlice()
    assert.Equal(t, 12, len(lines))
    assert.Equal(t, "line-37", lines[len(lines)-1]) // 12 lines back
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/frontend/telnet/... -run "TestRedrawConsole_SliceSelection" -v
```

Expected: FAIL (consoleSlice undefined)

**Step 3: Implement `consoleSlice` and `redrawConsole`**

Add to `internal/frontend/telnet/screen.go`:

```go
// consoleHeight returns the number of rows available for console output.
// consoleHeight = termHeight - roomRegionRows (room) - 1 (divider) - 1 (prompt)
func (c *Conn) consoleHeight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	h := c.height
	if h <= roomRegionRows+2 {
		return 1
	}
	return h - roomRegionRows - 2
}

// consoleSlice returns the slice of consoleBuf lines to display, accounting for scrollOffset.
// The returned slice has at most consoleHeight() entries, ending at
// len(consoleBuf)-scrollOffset (clamped to bounds).
//
// Precondition: c.mu need NOT be held by caller.
// Postcondition: Returns a copy-safe slice (may alias consoleBuf backing array — do not mutate).
func (c *Conn) consoleSlice() []string {
	c.mu.Lock()
	buf := c.consoleBuf
	offset := c.scrollOffset
	c.mu.Unlock()

	ch := c.consoleHeight()
	end := len(buf) - offset
	if end < 0 {
		end = 0
	}
	if end > len(buf) {
		end = len(buf)
	}
	start := end - ch
	if start < 0 {
		start = 0
	}
	return buf[start:end]
}

// redrawConsole redraws the console region from the scroll buffer.
// When scrolled back, appends a dim status line at the bottom of the console region.
//
// Precondition: conn must be in split-screen mode; height/width must be set.
// Postcondition: Console region rows are rewritten; prompt row is restored.
func (c *Conn) redrawConsole() error {
	c.mu.Lock()
	h := c.height
	w := c.width
	input := c.inputBuf
	room := c.roomBuf
	offset := c.scrollOffset
	pending := c.pendingNew
	c.mu.Unlock()

	lo := newRoomLayout(h)
	ch := c.consoleHeight()
	lines := c.consoleSlice()
	divider := strings.Repeat("═", w)

	var roomLines []string
	if room != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(room, "\r\n", "\n"), "\r", "")
		roomLines = strings.Split(strings.TrimSpace(normalized), "\n")
	}

	var buf strings.Builder

	// Clear console region (rows consoleTop .. promptRow-1)
	for row := lo.consoleTop; row < lo.promptRow; row++ {
		fmt.Fprintf(&buf, "\033[%d;1H\033[2K", row)
	}

	// Write buffered lines into console rows
	for i, line := range lines {
		row := lo.consoleTop + (ch - len(lines)) + i
		if row >= lo.promptRow {
			break
		}
		fmt.Fprintf(&buf, "\033[%d;1H", row)
		if w > 0 && visualWidth(line) > w {
			line = truncateToVisualWidth(line, w)
		}
		buf.WriteString(line)
	}

	// Status line when scrolled back (last console row above prompt)
	if offset > 0 {
		statusRow := lo.promptRow - 1
		var status string
		if pending > 0 {
			status = fmt.Sprintf("[scrolled back — %d new message(s)]", pending)
		} else {
			status = "[scrolled back]"
		}
		fmt.Fprintf(&buf, "\033[%d;1H\033[2K", statusRow)
		buf.WriteString("\033[2m") // dim
		buf.WriteString(status)
		buf.WriteString("\033[0m") // reset
	}

	// Restore room region and prompt
	appendRoomRedraw(&buf, roomLines, w, divider)
	fmt.Fprintf(&buf, "\033[%d;1H\033[2K", lo.promptRow)
	buf.WriteString(input)

	return c.writeRaw(buf.String())
}
```

**Step 4: Wire `WriteConsole` to call `appendConsoleLine`**

In `WriteConsole` (`screen.go`), after `c.mu.Unlock()` and before building the VT100 buffer, add:

```go
// Append each wrapped line to the scroll buffer.
// Must do this before the scrollOffset check so buffer is always current.
wrappedLines := wrapText(strings.TrimRight(text, "\r\n"), w)
for _, l := range wrappedLines {
    c.appendConsoleLine(l)
}

// If scrolled back, don't render — pendingNew was already incremented by appendConsoleLine.
c.mu.Lock()
scrolled := c.scrollOffset > 0
c.mu.Unlock()
if scrolled {
    return nil
}
```

Place this block right after the initial `c.mu.Unlock()` in `WriteConsole`, before `lo := newRoomLayout(h)`. Then replace the `lines := wrapText(trimmed, w)` call further down with just `lines := wrappedLines` to avoid double-wrapping.

**Step 5: Run tests**

```bash
go test ./internal/frontend/telnet/... -run "TestRedrawConsole|TestConsoleBuf" -v
```

Expected: All PASS

**Step 6: Commit**

```bash
git add internal/frontend/telnet/screen.go internal/frontend/telnet/conn_test.go
git commit -m "feat: implement redrawConsole and wire WriteConsole to scroll buffer"
```

---

### Task 4: Implement `ScrollUp` / `ScrollDown` on `Conn`

**Files:**
- Modify: `internal/frontend/telnet/screen.go`
- Modify: `internal/frontend/telnet/conn_test.go`

**Step 1: Write failing tests**

```go
func TestScrollUp_IncrementsOffset(t *testing.T) {
    // Fill 100 lines, terminal height 24 → consoleHeight=12.
    c := &Conn{height: 24, width: 80}
    for i := 0; i < 100; i++ {
        c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
    }
    // ScrollUp on a conn without a real network conn — just test state change.
    // We call the internal state method, not the full redraw path.
    c.mu.Lock()
    c.scrollOffset = 0
    c.mu.Unlock()
    c.scrollUpState()
    c.mu.Lock()
    off := c.scrollOffset
    c.mu.Unlock()
    assert.Equal(t, 12, off) // one page = consoleHeight
}

func TestScrollDown_DecrementsOffset(t *testing.T) {
    c := &Conn{height: 24, width: 80}
    for i := 0; i < 100; i++ {
        c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
    }
    c.mu.Lock()
    c.scrollOffset = 12
    c.pendingNew = 5
    c.mu.Unlock()
    c.scrollDownState()
    c.mu.Lock()
    off := c.scrollOffset
    pn := c.pendingNew
    c.mu.Unlock()
    assert.Equal(t, 0, off)
    assert.Equal(t, 0, pn) // pendingNew cleared when returning to live
}

func TestScrollUp_ClampsAtBufferBound(t *testing.T) {
    c := &Conn{height: 24, width: 80}
    // Only 5 lines in buffer, consoleHeight=12 — can't scroll back more than 5
    for i := 0; i < 5; i++ {
        c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
    }
    c.scrollUpState()
    c.mu.Lock()
    off := c.scrollOffset
    c.mu.Unlock()
    assert.Equal(t, 5, off) // clamped to len(buf)
}

func TestScrollDown_ClampsAtZero(t *testing.T) {
    c := &Conn{height: 24, width: 80}
    c.mu.Lock()
    c.scrollOffset = 0
    c.mu.Unlock()
    c.scrollDownState()
    c.mu.Lock()
    off := c.scrollOffset
    c.mu.Unlock()
    assert.Equal(t, 0, off)
}
```

**Step 2: Run to verify failures**

```bash
go test ./internal/frontend/telnet/... -run "TestScrollUp|TestScrollDown" -v
```

Expected: FAIL (scrollUpState/scrollDownState undefined)

**Step 3: Implement state helpers and public methods**

Add to `internal/frontend/telnet/screen.go`:

```go
// scrollUpState adjusts scrollOffset up by one page (consoleHeight lines),
// clamped to the number of buffered lines.
//
// Precondition: none.
// Postcondition: scrollOffset increased by consoleHeight, clamped to len(consoleBuf).
func (c *Conn) scrollUpState() {
	ch := c.consoleHeight()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scrollOffset += ch
	maxOffset := len(c.consoleBuf)
	if c.scrollOffset > maxOffset {
		c.scrollOffset = maxOffset
	}
}

// scrollDownState adjusts scrollOffset down by one page (consoleHeight lines),
// clamped to 0. Clears pendingNew when returning to live view.
//
// Precondition: none.
// Postcondition: scrollOffset decreased by consoleHeight (min 0); pendingNew=0 if live.
func (c *Conn) scrollDownState() {
	ch := c.consoleHeight()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scrollOffset -= ch
	if c.scrollOffset < 0 {
		c.scrollOffset = 0
	}
	if c.scrollOffset == 0 {
		c.pendingNew = 0
	}
}

// ScrollUp scrolls the console region back by one page and redraws.
//
// Precondition: conn must be in split-screen mode.
// Postcondition: Console region shows older content; status line rendered.
func (c *Conn) ScrollUp() error {
	c.scrollUpState()
	return c.redrawConsole()
}

// ScrollDown scrolls the console region forward by one page and redraws.
// When returning to live view, clears the pending-new counter.
//
// Precondition: conn must be in split-screen mode.
// Postcondition: Console region shows newer content (or live if at offset 0).
func (c *Conn) ScrollDown() error {
	c.scrollDownState()
	return c.redrawConsole()
}
```

**Step 4: Run tests**

```bash
go test ./internal/frontend/telnet/... -run "TestScrollUp|TestScrollDown" -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/frontend/telnet/screen.go internal/frontend/telnet/conn_test.go
git commit -m "feat: add ScrollUp/ScrollDown with page clamping to Conn"
```

---

### Task 5: Wire PgUp/PgDn sentinels in `commandLoop`

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go:379-390`

**Step 1: Find the sentinel handling block**

The current block at ~line 379:

```go
// Handle arrow key sentinels — no command history implemented yet.
if line == "\x00UP" || line == "\x00DOWN" {
    if conn.IsSplitScreen() {
        _ = conn.WritePromptSplit(buildPrompt())
    } else {
        _ = conn.WritePrompt(buildPrompt())
    }
    continue
}
```

**Step 2: Replace with scroll-aware handler**

```go
// Handle navigation sentinels.
switch line {
case "\x00UP", "\x00DOWN":
    // Arrow keys: redraw prompt only (no command history yet).
    if conn.IsSplitScreen() {
        _ = conn.WritePromptSplit(buildPrompt())
    } else {
        _ = conn.WritePrompt(buildPrompt())
    }
    continue
case "\x00PGUP":
    if conn.IsSplitScreen() {
        _ = conn.ScrollUp()
    }
    continue
case "\x00PGDN":
    if conn.IsSplitScreen() {
        _ = conn.ScrollDown()
    }
    continue
}

// Any real command while scrolled back: snap to live first.
if conn.IsSplitScreen() {
    if err := conn.SnapToLive(); err != nil {
        h.logger.Debug("snap to live failed", zap.Error(err))
    }
}
```

Note: `SnapToLive` is a new method (Task 6).

**Step 3: Commit placeholder (after Task 6)**

Wait until Task 6 adds `SnapToLive`, then commit both together.

---

### Task 6: Add `SnapToLive` and integrate

**Files:**
- Modify: `internal/frontend/telnet/screen.go`
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/game_bridge_test.go`

**Step 1: Write failing test**

Add to `internal/frontend/telnet/conn_test.go`:

```go
func TestSnapToLive_ClearsScrollAndPending(t *testing.T) {
    c := &Conn{height: 24, width: 80}
    c.mu.Lock()
    c.scrollOffset = 24
    c.pendingNew = 7
    c.mu.Unlock()
    c.snapToLiveState()
    c.mu.Lock()
    off := c.scrollOffset
    pn := c.pendingNew
    c.mu.Unlock()
    assert.Equal(t, 0, off)
    assert.Equal(t, 0, pn)
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/frontend/telnet/... -run "TestSnapToLive" -v
```

Expected: FAIL

**Step 3: Implement**

Add to `internal/frontend/telnet/screen.go`:

```go
// snapToLiveState clears scrollOffset and pendingNew without redrawing.
func (c *Conn) snapToLiveState() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.scrollOffset = 0
    c.pendingNew = 0
}

// SnapToLive returns to live view (scrollOffset=0) and redraws the console.
//
// Precondition: conn must be in split-screen mode.
// Postcondition: scrollOffset=0, pendingNew=0, console region shows latest lines.
func (c *Conn) SnapToLive() error {
    c.snapToLiveState()
    return c.redrawConsole()
}
```

**Step 4: Apply the `commandLoop` change from Task 5**

Apply the full switch block from Task 5 to `internal/frontend/handlers/game_bridge.go`.

**Step 5: Run all tests**

```bash
go test ./internal/frontend/... -v 2>&1 | tail -30
```

Expected: All PASS

**Step 6: Commit**

```bash
git add internal/frontend/telnet/screen.go internal/frontend/telnet/conn_test.go \
        internal/frontend/handlers/game_bridge.go
git commit -m "feat: wire PgUp/PgDn sentinels to ScrollUp/ScrollDown in commandLoop"
```

---

### Task 7: Integration test and full test suite

**Files:**
- Modify: `internal/frontend/telnet/conn_test.go`

**Step 1: Write integration test**

```go
func TestIntegration_ConsoleScroll(t *testing.T) {
    // Build a Conn with known state (no real network).
    c := &Conn{height: 24, width: 80}

    // Write 200 lines into the buffer directly.
    for i := 0; i < 200; i++ {
        c.appendConsoleLine(fmt.Sprintf("line-%d", i))
    }

    // consoleHeight = 24 - 10 - 2 = 12
    // Scroll up one page.
    c.scrollUpState()
    c.mu.Lock()
    off := c.scrollOffset
    c.mu.Unlock()
    assert.Equal(t, 12, off)

    // consoleSlice should show lines 176-187 (200-12-12 .. 200-12-1)
    slice := c.consoleSlice()
    assert.Equal(t, 12, len(slice))
    assert.Equal(t, "line-176", slice[0])
    assert.Equal(t, "line-187", slice[len(slice)-1])

    // Append more lines while scrolled — pendingNew should increment.
    c.appendConsoleLine("new-0")
    c.appendConsoleLine("new-1")
    c.mu.Lock()
    pn := c.pendingNew
    c.mu.Unlock()
    assert.Equal(t, 2, pn)

    // Scroll down to live — pendingNew cleared.
    c.scrollDownState()
    c.mu.Lock()
    off = c.scrollOffset
    pn = c.pendingNew
    c.mu.Unlock()
    assert.Equal(t, 0, off)
    assert.Equal(t, 0, pn)
}
```

**Step 2: Run**

```bash
go test ./internal/frontend/telnet/... -run "TestIntegration_ConsoleScroll" -v
```

Expected: PASS

**Step 3: Run the full test suite**

```bash
go test ./... 2>&1 | tail -40
```

Expected: All PASS (or pre-existing failures only)

**Step 4: Commit**

```bash
git add internal/frontend/telnet/conn_test.go
git commit -m "test: add integration test for console scroll buffer"
```

---

### Task 8: Deploy and mark done

**Step 1: Deploy**

```bash
make k8s-redeploy
kubectl rollout restart deployment/frontend deployment/gameserver -n mud
kubectl rollout status deployment/frontend deployment/gameserver -n mud --timeout=60s
```

**Step 2: Mark bug fixed in FEATURES.md**

Change in `docs/requirements/FEATURES.md`:
```
- [ ] PgUp/PgDn should scroll the Console section so the user can look back
```
to:
```
- [x] PgUp/PgDn should scroll the Console section so the user can look back
```

**Step 3: Commit and push**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark PgUp/PgDn console scroll as complete"
git push
```
