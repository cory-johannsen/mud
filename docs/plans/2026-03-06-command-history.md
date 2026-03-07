# Command History & Shift+Arrow Console Scroll Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Up/Down arrows navigate command history; Shift+Up/Shift+Down scroll the console.

**Architecture:** Two tasks — (1) add history ring buffer to `Conn` with `AppendHistory`/`HistoryUp`/`HistoryDown`/`SetInputLine`, extend `tryReadEscapeSeq` for Shift+arrow sentinels; (2) rewire `game_bridge.go` to use history on plain arrows and scroll on Shift+arrows.

**Tech Stack:** Go, `internal/frontend/telnet` (conn.go, screen.go), `internal/frontend/handlers/game_bridge.go`, `pgregory.net/rapid` for property-based tests.

---

### Task 1: Command history ring buffer and Shift+arrow escape sequences

**Files:**
- Modify: `internal/frontend/telnet/conn.go`
- Modify: `internal/frontend/telnet/screen.go`
- Modify: `internal/frontend/telnet/conn_test.go`

#### Step 1a: Write failing tests first

Add to `internal/frontend/telnet/conn_test.go`:

```go
func TestConn_History_AppendAndNavigate(t *testing.T) {
    c := &Conn{}
    c.AppendHistory("look")
    c.AppendHistory("north")
    c.AppendHistory("inventory")

    // HistoryUp from live position returns most recent
    got, ok := c.HistoryUp()
    require.True(t, ok)
    assert.Equal(t, "inventory", got)

    got, ok = c.HistoryUp()
    require.True(t, ok)
    assert.Equal(t, "north", got)

    got, ok = c.HistoryUp()
    require.True(t, ok)
    assert.Equal(t, "look", got)

    // At oldest entry, HistoryUp is a no-op
    got2, ok2 := c.HistoryUp()
    assert.False(t, ok2)
    assert.Equal(t, "", got2)

    // HistoryDown from oldest moves forward
    got, ok = c.HistoryDown()
    require.True(t, ok)
    assert.Equal(t, "north", got)

    got, ok = c.HistoryDown()
    require.True(t, ok)
    assert.Equal(t, "inventory", got)

    // HistoryDown at live position returns "", false
    got, ok = c.HistoryDown()
    assert.False(t, ok)
    assert.Equal(t, "", got)
}

func TestConn_History_ResetOnSubmit(t *testing.T) {
    c := &Conn{}
    c.AppendHistory("look")
    c.AppendHistory("north")
    _, _ = c.HistoryUp() // navigate back
    c.AppendHistory("inventory") // new command resets cursor to live
    got, ok := c.HistoryUp()
    require.True(t, ok)
    assert.Equal(t, "inventory", got) // most recent is the new command
}

func TestProperty_History_ReverseOrder(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        n := rapid.IntRange(1, 20).Draw(rt, "n")
        c := &Conn{}
        cmds := make([]string, n)
        for i := range cmds {
            cmds[i] = fmt.Sprintf("cmd%d", i)
            c.AppendHistory(cmds[i])
        }
        // Navigate up n times; should get commands in reverse order
        for i := n - 1; i >= 0; i-- {
            got, ok := c.HistoryUp()
            if !ok {
                rt.Fatalf("HistoryUp returned false at index %d", i)
            }
            if got != cmds[i] {
                rt.Fatalf("at index %d: want %q, got %q", i, cmds[i], got)
            }
        }
    })
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/frontend/telnet/... -run TestConn_History -v`
Expected: FAIL (methods don't exist yet)

#### Step 1b: Add fields to `Conn`

In `internal/frontend/telnet/conn.go`, add to the `Conn` struct (after `pendingNew int`):

```go
// Command history (guarded by mu)
cmdHistory []string // ring buffer, max cmdHistoryMax entries
historyIdx int      // cursor; len(cmdHistory) = live (past end)
```

Add constant near `consoleBufMax`:
```go
// cmdHistoryMax is the maximum number of commands retained in the history buffer.
const cmdHistoryMax = 100
```

#### Step 1c: Implement history methods

Add to `internal/frontend/telnet/conn.go`:

```go
// AppendHistory adds a command to the history buffer and resets the cursor to
// the live (past-end) position.
// Precondition: cmd must be non-empty.
// Postcondition: history contains cmd; historyIdx == len(cmdHistory).
func (c *Conn) AppendHistory(cmd string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cmdHistory = append(c.cmdHistory, cmd)
    if len(c.cmdHistory) > cmdHistoryMax {
        c.cmdHistory = c.cmdHistory[len(c.cmdHistory)-cmdHistoryMax:]
    }
    c.historyIdx = len(c.cmdHistory)
}

// HistoryUp moves the cursor one step back in history and returns the command.
// Returns ("", false) when already at the oldest entry.
// Postcondition: historyIdx decremented by 1 if > 0.
func (c *Conn) HistoryUp() (string, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.historyIdx == 0 {
        return "", false
    }
    c.historyIdx--
    return c.cmdHistory[c.historyIdx], true
}

// HistoryDown moves the cursor one step forward in history.
// Returns the command at the new position, or ("", false) if now at live position.
// Postcondition: historyIdx incremented by 1 if < len(cmdHistory).
func (c *Conn) HistoryDown() (string, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.historyIdx >= len(c.cmdHistory) {
        return "", false
    }
    c.historyIdx++
    if c.historyIdx == len(c.cmdHistory) {
        return "", false
    }
    return c.cmdHistory[c.historyIdx], true
}
```

#### Step 1d: Add `SetInputLine` to `screen.go`

Add to `internal/frontend/telnet/screen.go`:

```go
// SetInputLine replaces the current prompt input area with s and updates inputBuf.
// Used by command history navigation to display a recalled command.
//
// Precondition: splitScreen must be true; prompt is the current prompt string.
// Postcondition: inputBuf == s; prompt row redrawn with prompt+s.
func (c *Conn) SetInputLine(prompt, s string) error {
    c.mu.Lock()
    c.inputBuf = s
    c.mu.Unlock()
    var buf strings.Builder
    fmt.Fprintf(&buf, "\r\033[2K%s%s", prompt, s)
    return c.writeRaw(buf.String())
}
```

#### Step 1e: Extend `tryReadEscapeSeq` for Shift+arrow

In `internal/frontend/telnet/conn.go`, the `tryReadEscapeSeq` function currently reads `ESC [ <final>`. Shift+Up sends `ESC [ 1 ; 2 A` and Shift+Down sends `ESC [ 1 ; 2 B`.

The current `default` branch already reads parameter bytes. Extend the `'1'` case before the default:

Replace the switch in `tryReadEscapeSeq`:
```go
switch final {
case 'A':
    return "\x00UP"
case 'B':
    return "\x00DOWN"
case '1':
    // Could be ESC [ 1 ; 2 A (Shift+Up) or ESC [ 1 ; 2 B (Shift+Down)
    // Read remaining bytes: expect ; 2 A or ; 2 B
    b1, err := c.reader.ReadByte() // expect ';'
    if err != nil {
        return ""
    }
    b2, err := c.reader.ReadByte() // expect '2'
    if err != nil {
        return ""
    }
    b3, err := c.reader.ReadByte() // expect 'A' or 'B'
    if err != nil {
        return ""
    }
    if b1 == ';' && b2 == '2' && b3 == 'A' {
        return "\x00SHIFT_UP"
    }
    if b1 == ';' && b2 == '2' && b3 == 'B' {
        return "\x00SHIFT_DOWN"
    }
    return ""
case '5', '6':
    term, err := c.reader.ReadByte()
    if err != nil || term != '~' {
        return ""
    }
    if final == '5' {
        return "\x00PGUP"
    }
    return "\x00PGDN"
default:
    // consume parameter bytes
    ...
}
```

Add test for escape sequence parsing in `conn_test.go`:

```go
func TestTryReadEscapeSeq_ShiftArrows(t *testing.T) {
    tests := []struct {
        input    []byte
        sentinel string
    }{
        {[]byte{'[', '1', ';', '2', 'A'}, "\x00SHIFT_UP"},
        {[]byte{'[', '1', ';', '2', 'B'}, "\x00SHIFT_DOWN"},
        {[]byte{'[', 'A'}, "\x00UP"},
        {[]byte{'[', 'B'}, "\x00DOWN"},
    }
    for _, tt := range tests {
        c := &Conn{reader: bufio.NewReader(bytes.NewReader(tt.input))}
        got := c.tryReadEscapeSeq()
        assert.Equal(t, tt.sentinel, got, "input %q", tt.input)
    }
}
```

#### Step 1f: Run all tests

```
cd /home/cjohannsen/src/mud && go test ./internal/frontend/telnet/... -timeout 60s -v 2>&1 | tail -30
```

Expected: all pass.

#### Step 1g: Commit

```bash
git add internal/frontend/telnet/conn.go internal/frontend/telnet/screen.go internal/frontend/telnet/conn_test.go
git commit -m "feat: add command history ring buffer and Shift+arrow escape sequences"
```

---

### Task 2: Rewire game_bridge.go

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go:379-406`

#### Step 2a: Replace sentinel handling

In `game_bridge.go`, find the sentinel switch (lines ~380-407). Replace it:

```go
switch line {
case "\x00UP":
    if conn.IsSplitScreen() {
        if cmd, ok := conn.HistoryUp(); ok {
            _ = conn.SetInputLine(buildPrompt(), cmd)
        }
    } else {
        _ = conn.WritePrompt(buildPrompt())
    }
    continue
case "\x00DOWN":
    if conn.IsSplitScreen() {
        cmd, _ := conn.HistoryDown()
        _ = conn.SetInputLine(buildPrompt(), cmd)
    } else {
        _ = conn.WritePrompt(buildPrompt())
    }
    continue
case "\x00SHIFT_UP":
    if conn.IsSplitScreen() {
        _ = conn.ScrollUpLine()
    }
    continue
case "\x00SHIFT_DOWN":
    if conn.IsSplitScreen() && conn.IsScrolledBack() {
        _ = conn.ScrollDownLine()
    } else if conn.IsSplitScreen() {
        _ = conn.WritePromptSplit(buildPrompt())
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
```

#### Step 2b: Append to history on each submitted command

After `line = strings.TrimSpace(line)` and before the `if line == ""` check, add:

```go
if line != "" && conn.IsSplitScreen() {
    conn.AppendHistory(line)
}
```

#### Step 2c: Run tests

```
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -timeout 60s 2>&1 | tail -5
go test ./... -timeout 180s 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: all ok (except pre-existing postgres timeout).

#### Step 2d: Update FEATURES.md

Change:
```
- [ ] Up/down arrow show scroll back through the previously entered commands, Shift+Up/Shift-Down should scroll the console.
```
to:
```
- [x] Up/down arrow show scroll back through the previously entered commands, Shift+Up/Shift-Down should scroll the console.
```

#### Step 2e: Commit and deploy

```bash
git add internal/frontend/handlers/game_bridge.go docs/requirements/FEATURES.md
git commit -m "feat: up/down arrow command history; Shift+arrow console scroll"
make k8s-redeploy
```
