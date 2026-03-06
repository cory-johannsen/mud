package telnet

import (
	"fmt"
	"strings"
	"time"
)

// RoomRegionRows is the number of content rows in the pinned room region.
// Exported so the renderer can enforce this limit when building room text.
const RoomRegionRows = 10

// roomRegionRows is an internal alias kept for use within this package.
const roomRegionRows = RoomRegionRows

// roomLayout computes the row numbers for the split-screen layout given a
// terminal height h.
//
// Layout (rows are 1-based, NO scroll region — full-screen default):
//
//	1 … roomRegionRows   : room content (fixed by redraw after every scroll)
//	roomRegionRows+1     : room divider ═══
//	roomRegionRows+2 … h : console messages + prompt
//	h                    : prompt / input
//
// Why no DECSTBM (scroll region command):
//
//	TinTin++ applies DECOM-like coordinate offsets when a non-1-based scroll
//	region is active: \033[1;1H is remapped to physical row scrollTop rather
//	than row 1.  This silently shifts all our room-region writes 9 rows down,
//	placing them inside the scrolling area where they get scrolled away.
//	By keeping the default full-screen scroll region (1..h) we avoid all
//	TinTin++ scroll-region quirks: absolute positioning works everywhere, and
//	\r\n scrolls rows 1..h predictably.
//
//	After each WriteConsole the room region is redrawn with appendRoomRedraw
//	(absolute to row 1, always safe) to restore any rows that scrolled up.
type roomLayout struct {
	firstRow   int // first room content row  = 1
	lastRow    int // last room content row   = roomRegionRows
	dividerRow int // room divider row        = roomRegionRows + 1
	consoleTop int // first console row       = roomRegionRows + 2
	promptRow  int // input / prompt row      = h
}

func newRoomLayout(h int) roomLayout {
	return roomLayout{
		firstRow:   1,
		lastRow:    roomRegionRows,
		dividerRow: roomRegionRows + 1,
		consoleTop: roomRegionRows + 2,
		promptRow:  h,
	}
}

// InitScreen initializes the split-screen layout.
//
// No DECSTBM is sent: the terminal keeps its default full-screen scroll region
// (rows 1..h) to avoid TinTin++'s DECOM coordinate-offset quirk.
//
// Precondition:  conn.height must be > 0.
// Postcondition: Screen cleared; room rows 1..dividerRow blank; cursor at promptRow.
func (c *Conn) InitScreen() error {
	c.mu.Lock()
	h := c.height
	c.mu.Unlock()

	lo := newRoomLayout(h)

	var buf strings.Builder
	buf.WriteString("\033[?25l")     // hide cursor
	buf.WriteString("\033[2J\033[H") // clear screen; cursor home (1,1)

	// Draw blank room region + divider (rows 1..dividerRow).
	// Default scroll region = 1..h → absolute positioning anywhere is safe.
	for i := 0; i < roomRegionRows; i++ {
		buf.WriteString("\033[2K\r\n") // clear room content row, advance
	}
	// Cursor now at row dividerRow (= roomRegionRows+1).
	buf.WriteString("\033[2K") // clear divider row (WriteRoom draws the actual ═══)

	// Navigate to promptRow (safe: default full-screen scroll region).
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	buf.WriteString("\033[?25h") // show cursor
	return c.writeRaw(buf.String())
}

// consoleHeight returns the number of rows available for console output.
// Formula: termHeight - roomRegionRows (room) - 1 (divider) - 1 (prompt)
//
// Postcondition: Returns at least 1.
func (c *Conn) consoleHeight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	h := c.height
	if h <= roomRegionRows+2 {
		return 1
	}
	return h - roomRegionRows - 2
}

// consoleSlice returns the slice of consoleBuf lines to display given the current
// scrollOffset. The returned slice has at most consoleHeight() entries, ending at
// len(consoleBuf)-scrollOffset (clamped to valid bounds).
//
// Precondition: c.mu must NOT be held by caller.
// Postcondition: returned slice may alias consoleBuf backing array; do not mutate.
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
// When scrolled back (scrollOffset > 0), a dim status line is shown at the
// last console row: "[scrolled back — N new message(s)]" or "[scrolled back]".
//
// Precondition: conn must be in split-screen mode; height and width must be set.
// Postcondition: Console region rows rewritten; room region and prompt restored.
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

	// Write buffered lines into console rows (bottom-aligned)
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

	// Status line when scrolled back (occupies last console row above prompt)
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

// WriteRoom renders content into the pinned room region (rows 1..dividerRow).
//
// Uses \033[1;1H] (absolute, always safe with full-screen scroll region) to
// reach row 1 for room rendering.
//
// Precondition:  conn.splitScreen must be true; conn.width > 0; conn.height > 0.
// Postcondition: Room divider and content rows updated; cursor at promptRow.
func (c *Conn) WriteRoom(content string) error {
	c.mu.Lock()
	w := c.width
	h := c.height
	c.roomBuf = content // cache for WriteConsole to redraw after scrolling
	c.mu.Unlock()

	lo := newRoomLayout(h)

	// Normalize CRLF to LF before splitting.
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	appendRoomRedraw(&buf, lines, w, divider)
	// Navigate to promptRow (absolute, safe with full-screen scroll region).
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)

	return c.writeRaw(buf.String())
}

// WriteConsole writes a message into the console area and redraws the room
// region and prompt row.
//
// Operation order:
//  1. Position at promptRow; write message lines with \r\n — each \r\n at
//     the last row of the full-screen scroll region (row h) scrolls the entire
//     screen up by one row, making room for the new content.
//  2. One extra \r\n pushes the last message line above the prompt row.
//  3. Redraw room region with appendRoomRedraw (\033[1;1H], always safe).
//     This restores any room rows that were shifted up by the scrolling.
//  4. Navigate to promptRow; redraw input.
//
// Precondition:  conn.splitScreen must be true; conn.height > 0; conn.width > 0.
// Postcondition: Message in console area; room and prompt redrawn; cursor at promptRow.
func (c *Conn) WriteConsole(text string) error {
	c.mu.Lock()
	h := c.height
	w := c.width
	input := c.inputBuf
	room := c.roomBuf
	c.mu.Unlock()

	// Buffer lines for scroll history.
	wrappedLines := wrapText(strings.TrimRight(text, "\r\n"), w)
	for _, l := range wrappedLines {
		c.appendConsoleLine(l)
	}

	// If scrolled back, skip rendering — pendingNew was incremented by appendConsoleLine.
	c.mu.Lock()
	scrolled := c.scrollOffset > 0
	c.mu.Unlock()
	if scrolled {
		return nil
	}

	lo := newRoomLayout(h)
	divider := strings.Repeat("═", w)

	var roomLines []string
	if room != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(room, "\r\n", "\n"), "\r", "")
		roomLines = strings.Split(strings.TrimSpace(normalized), "\n")
	}

	var buf strings.Builder

	// Position at promptRow (= last row of full-screen scroll region).
	// Clear the line first so the current prompt/input is not scrolled into
	// the console area — otherwise the prompt text appears as a console line.
	// Each subsequent \r\n scrolls the entire screen up by one row.
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	buf.WriteString("\033[2K")
	lines := wrappedLines
	for _, line := range lines {
		buf.WriteString("\r\n")
		buf.WriteString(line)
	}
	// One extra scroll pushes the last message line above the prompt row,
	// leaving row h blank for the input redraw.
	if len(lines) > 0 {
		buf.WriteString("\r\n")
	}

	// Restore room region (rows 1..dividerRow) — the scrolls above shifted
	// room rows upward; appendRoomRedraw rewrites them at their canonical positions.
	appendRoomRedraw(&buf, roomLines, w, divider)

	// Navigate to promptRow and redraw input.
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	buf.WriteString("\033[2K")
	buf.WriteString(input)

	return c.writeRaw(buf.String())
}

// appendRoomRedraw writes the room content rows and divider into the fixed
// region at the TOP of the screen (rows 1..roomRegionRows+1).
//
// Positions via \033[1;1H] (absolute — always safe since we never set a
// non-standard scroll region; the default full-screen region imposes no
// DECOM offsets or TinTin++ spurious-scroll triggers).
//
// Precondition:  lines must be split and normalized (no \r\n); w >= 0.
// Postcondition: cursor is at row dividerRow (= roomRegionRows+1), after divider text.
func appendRoomRedraw(buf *strings.Builder, lines []string, w int, divider string) {
	// Absolute position to row 1, col 1 (always safe with full-screen scroll region).
	buf.WriteString("\033[1;1H")
	for i := 0; i < roomRegionRows; i++ {
		buf.WriteString("\r\033[2K")
		if i < len(lines) {
			line := lines[i]
			if w > 0 && visualWidth(line) > w {
				line = truncateToVisualWidth(line, w)
			}
			buf.WriteString(line)
		}
		buf.WriteString("\r\n") // advance to next row (no scroll above promptRow)
	}
	// Cursor now at row roomRegionRows+1 = dividerRow.
	buf.WriteString("\r\033[2K")
	buf.WriteString(divider)
	// Cursor at dividerRow, col after divider.
}

// WritePromptSplit writes the prompt and buffered input at the prompt row (row h).
//
// Precondition:  conn.splitScreen must be true; cursor must be at promptRow.
// Postcondition: Prompt appears at row h with cursor after prompt+input.
func (c *Conn) WritePromptSplit(prompt string) error {
	c.mu.Lock()
	input := c.inputBuf
	c.mu.Unlock()

	// Cursor is always at promptRow when WritePromptSplit is called.
	// Write prompt in-place: \r to col 1, erase line, write prompt+input.
	var buf strings.Builder
	fmt.Fprintf(&buf, "\r\033[2K%s%s", prompt, input)

	return c.writeRaw(buf.String())
}

// EnableSplitScreen marks the connection as operating in split-screen mode.
//
// Postcondition: IsSplitScreen() returns true.
func (c *Conn) EnableSplitScreen() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.splitScreen = true
}

// SetInputBuf updates the buffered player input for redraw after server events.
func (c *Conn) SetInputBuf(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputBuf = s
}

// writeRaw writes a raw string to the connection without line-terminator wrapping.
//
// Precondition:  s must be a valid string; the connection must be open.
// Postcondition: All bytes of s are written to the underlying connection.
func (c *Conn) writeRaw(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := c.raw.Write([]byte(s))
	return err
}

// visualWidth returns the number of visible characters in s, ignoring ANSI
// escape sequences of the form \033[...m.
func visualWidth(s string) int {
	w := 0
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		w++
		i++
	}
	return w
}

// truncateToVisualWidth truncates s to at most maxW visible characters,
// preserving complete ANSI escape sequences and appending a final reset.
func truncateToVisualWidth(s string, maxW int) string {
	result := make([]byte, 0, len(s))
	w := 0
	i := 0
	for i < len(s) && w < maxW {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				result = append(result, s[i:j+1]...)
				i = j + 1
				continue
			}
		}
		result = append(result, s[i])
		w++
		i++
	}
	// Close any open ANSI sequences with a reset.
	if w == maxW && i < len(s) {
		result = append(result, "\033[0m"...)
	}
	return string(result)
}

// scrollUpState adjusts scrollOffset backward by one page (consoleHeight lines),
// clamped to len(consoleBuf) so we cannot scroll past the oldest buffered line.
//
// Precondition: none.
// Postcondition: scrollOffset <= len(consoleBuf).
func (c *Conn) scrollUpState() {
	ch := c.consoleHeight()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scrollOffset += ch
	if c.scrollOffset > len(c.consoleBuf) {
		c.scrollOffset = len(c.consoleBuf)
	}
}

// scrollDownState adjusts scrollOffset forward by one page (consoleHeight lines),
// clamped to 0 (live view). Clears pendingNew when returning to live.
//
// Precondition: none.
// Postcondition: scrollOffset >= 0; pendingNew == 0 if scrollOffset == 0.
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
// Postcondition: Console region shows older content; status line rendered if scrolled.
func (c *Conn) ScrollUp() error {
	c.scrollUpState()
	return c.redrawConsole()
}

// ScrollDown scrolls the console region forward by one page and redraws.
// When returning to live view (scrollOffset == 0), pendingNew is cleared.
//
// Precondition: conn must be in split-screen mode.
// Postcondition: Console shows newer content; pendingNew cleared if live.
func (c *Conn) ScrollDown() error {
	c.scrollDownState()
	return c.redrawConsole()
}

// snapToLiveState clears scrollOffset and pendingNew without triggering a redraw.
//
// Postcondition: scrollOffset == 0; pendingNew == 0.
func (c *Conn) snapToLiveState() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scrollOffset = 0
	c.pendingNew = 0
}

// SnapToLive returns to live view (scrollOffset=0) and redraws the console.
//
// Precondition: conn must be in split-screen mode.
// Postcondition: scrollOffset=0, pendingNew=0, console shows latest buffered lines.
func (c *Conn) SnapToLive() error {
	c.snapToLiveState()
	return c.redrawConsole()
}

// WrapText splits text into lines of at most width visible characters.
// ANSI escape sequences are preserved and not counted toward the width.
// This is the exported form of wrapText.
func WrapText(text string, width int) []string { return wrapText(text, width) }

// wrapText splits text into lines of at most width visible characters.
// ANSI escape sequences are preserved and not counted toward the width.
// Lines already shorter than width are returned unchanged.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var result []string
	for _, line := range strings.Split(text, "\n") {
		for visualWidth(line) > width {
			result = append(result, truncateToVisualWidth(line, width))
			// Advance past the bytes that were included in the truncated output.
			// Count visible chars to find the split point.
			i, w := 0, 0
			for i < len(line) && w < width {
				if line[i] == '\033' && i+1 < len(line) && line[i+1] == '[' {
					j := i + 2
					for j < len(line) && line[j] != 'm' {
						j++
					}
					if j < len(line) {
						i = j + 1
						continue
					}
				}
				w++
				i++
			}
			line = line[i:]
		}
		result = append(result, line)
	}
	return result
}
