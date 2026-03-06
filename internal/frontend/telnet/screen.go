package telnet

import (
	"fmt"
	"strings"
	"time"
)

const roomRegionRows = 8

// roomLayout computes the row numbers for the split-screen layout given a
// terminal height h.
//
// Layout (rows are 1-based):
//
//	1 … roomRegionRows   : room content (fixed, above scroll region)
//	roomRegionRows+1     : room divider ═══ (fixed)
//	roomRegionRows+2 … h : scroll region (console messages + prompt)
//	h                    : prompt / input (bottom of scroll region)
//
// Safe cursor positioning:
//   - Rows 1..dividerRow are ABOVE the scroll region. Absolute addressing
//     there never triggers TinTin++'s spurious full-screen scroll (which only
//     fires for rows > scrollBottom).
//   - Row h = scrollBottom. Also safe since it is AT the scroll region bottom,
//     not below it.
type roomLayout struct {
	firstRow     int // first room content row  = 1
	lastRow      int // last room content row   = roomRegionRows
	dividerRow   int // room divider row        = roomRegionRows + 1
	scrollTop    int // first scroll region row = roomRegionRows + 2
	scrollBottom int // last scroll region row  = h  (= promptRow)
	promptRow    int // input / prompt row      = h
}

func newRoomLayout(h int) roomLayout {
	return roomLayout{
		firstRow:     1,
		lastRow:      roomRegionRows,
		dividerRow:   roomRegionRows + 1,
		scrollTop:    roomRegionRows + 2,
		scrollBottom: h,
		promptRow:    h,
	}
}

// InitScreen initializes the split-screen layout.
//
// Layout: room region (rows 1..roomRegionRows+1) is pinned ABOVE the scroll
// region (rows scrollTop..h). The prompt lives at row h (= scrollBottom).
//
// TinTin++ quirk: \033[row;1H for row > scrollBottom causes TinTin++ to scroll
// the ENTIRE screen upward.  In this layout scrollBottom = h, so no row is ever
// > scrollBottom — absolute positioning is unconditionally safe.
//
// Precondition:  conn.height must be > 0.
// Postcondition: Terminal configured; scroll region = scrollTop..h; cursor at promptRow.
func (c *Conn) InitScreen() error {
	c.mu.Lock()
	h := c.height
	c.mu.Unlock()

	lo := newRoomLayout(h)

	var buf strings.Builder
	buf.WriteString("\033[?25l")     // hide cursor
	buf.WriteString("\033[2J\033[H") // clear screen; cursor home (1,1)

	// Draw blank room region and divider (rows 1..dividerRow).
	// Before DECSTBM the scroll region is 1..h (default = full screen), so
	// cursor movement here is safe everywhere.
	for i := 0; i < roomRegionRows; i++ {
		buf.WriteString("\033[2K\r\n") // clear room content row, advance
	}
	// Cursor now at row dividerRow (= roomRegionRows+1).
	buf.WriteString("\033[2K") // clear divider row (WriteRoom draws the actual ═══)

	// Navigate to promptRow BEFORE setting scroll region.
	// Scroll region = 1..h (full screen) ⟹ \033[h;1H is safe.
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	// Save cursor at promptRow (DECSC = ESC 7).
	buf.WriteString("\x1b7")
	// Set scroll region scrollTop..h. DECSTBM resets cursor to (1,1).
	fmt.Fprintf(&buf, "\033[%d;%dr", lo.scrollTop, lo.scrollBottom)
	// Restore cursor to promptRow (DECRC = ESC 8).
	buf.WriteString("\x1b8")

	buf.WriteString("\033[?25h") // show cursor; cursor ends at promptRow
	return c.writeRaw(buf.String())
}

// WriteRoom renders content into the pinned room region (rows 1..dividerRow).
//
// Uses \033[1;1H] (absolute, safe: row 1 is above the scroll region) to
// reach the room region — no CUU required, no TinTin++ scroll triggered.
//
// Precondition:  conn.splitScreen must be true; conn.width > 0; conn.height > 0.
// Postcondition: Room divider and content rows updated; cursor at promptRow.
func (c *Conn) WriteRoom(content string) error {
	c.mu.Lock()
	w := c.width
	h := c.height
	c.roomBuf = content // cache for WriteConsole to redraw the pinned region
	c.mu.Unlock()

	lo := newRoomLayout(h)

	// Normalize CRLF to LF before splitting.
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	appendRoomRedraw(&buf, lines, w, divider)
	// Navigate from dividerRow to promptRow (safe: promptRow = scrollBottom).
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)

	return c.writeRaw(buf.String())
}

// WriteConsole writes a message into the scroll region and redraws the room
// region and prompt row.
//
// Operation order:
//  1. Redraw room via \033[1;1H] (safe: above scroll region).
//  2. Navigate to promptRow (= scrollBottom); save cursor (DECSC).
//  3. Write message lines with \r\n — each \r\n at scrollBottom scrolls the
//     scroll region up; cursor stays at promptRow throughout.
//  4. One extra \r\n pushes the last message line above the prompt row.
//  5. Restore cursor to promptRow (DECRC); redraw input.
//
// Precondition:  conn.splitScreen must be true; conn.height > 0; conn.width > 0.
// Postcondition: Message appears in scroll region; room and prompt redrawn; cursor at promptRow.
func (c *Conn) WriteConsole(text string) error {
	c.mu.Lock()
	h := c.height
	w := c.width
	input := c.inputBuf
	room := c.roomBuf
	c.mu.Unlock()

	lo := newRoomLayout(h)
	divider := strings.Repeat("═", w)

	var roomLines []string
	if room != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(room, "\r\n", "\n"), "\r", "")
		roomLines = strings.Split(strings.TrimSpace(normalized), "\n")
	}

	var buf strings.Builder
	appendRoomRedraw(&buf, roomLines, w, divider)

	// Navigate from dividerRow to promptRow (= scrollBottom; safe: not > scrollBottom).
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	// Save cursor at promptRow (DECSC).
	buf.WriteString("\x1b7")

	// Write message lines. Each \r\n at promptRow (= scrollBottom) scrolls the
	// scroll region up by one row, keeping the cursor at promptRow.
	trimmed := strings.TrimRight(text, "\r\n")
	lines := wrapText(trimmed, w)
	for _, line := range lines {
		buf.WriteString("\r\n")
		buf.WriteString(line)
	}
	// One extra scroll to push the last message line above the prompt row,
	// leaving row h blank for the input redraw.
	if len(lines) > 0 {
		buf.WriteString("\r\n")
	}

	// Restore cursor to promptRow (DECRC) and redraw input.
	buf.WriteString("\x1b8\033[2K")
	buf.WriteString(input)

	return c.writeRaw(buf.String())
}

// appendRoomRedraw writes the room content rows and divider into the fixed
// region at the TOP of the screen (rows 1..roomRegionRows+1).
//
// Positions via \033[1;1H] (absolute, safe: row 1 is ABOVE the scroll region,
// so TinTin++ never performs a spurious scroll for this address).
//
// Precondition:  lines must be split and normalized (no \r\n); w >= 0.
// Postcondition: cursor is at row dividerRow (= roomRegionRows+1), after divider text.
func appendRoomRedraw(buf *strings.Builder, lines []string, w int, divider string) {
	// Absolute position to row 1, col 1 (above scroll region — always safe).
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
		buf.WriteString("\r\n") // advance to next row (no scroll: above scroll region)
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
