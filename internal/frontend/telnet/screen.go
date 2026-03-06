package telnet

import (
	"fmt"
	"strings"
	"time"
)

const roomRegionRows = 8

// roomLayout computes the row numbers for the split-screen layout given a
// terminal height h.  All computed rows are BELOW the scroll region so that
// cursor positioning works even when the terminal restricts absolute movement
// to rows within the scroll region (a common TinTin++ / xterm quirk).
//
// Layout (rows are 1-based):
//
//	1 … scrollBottom   : scroll region (console messages)
//	scrollBottom+1     : room divider ═══
//	scrollBottom+2 … h-1 : room content (roomRegionRows lines)
//	h                  : prompt / input
type roomLayout struct {
	scrollBottom int // last row of scroll region   = h - (roomRegionRows + 2)
	dividerRow   int // room divider row             = h - (roomRegionRows + 1)
	firstRow     int // first room content row       = h - roomRegionRows
	lastRow      int // last  room content row       = h - 1
	promptRow    int // input / prompt row           = h
}

func newRoomLayout(h int) roomLayout {
	return roomLayout{
		scrollBottom: h - (roomRegionRows + 2),
		dividerRow:   h - (roomRegionRows + 1),
		firstRow:     h - roomRegionRows,
		lastRow:      h - 1,
		promptRow:    h,
	}
}

// InitScreen initializes the split-screen layout.
//
// Scroll region = rows 1..scrollBottom.
// Room divider  = row dividerRow  (= scrollBottom+1).
// Room content  = rows firstRow..lastRow  (roomRegionRows rows).
// Prompt        = row h.
//
// TinTin++ quirk: \033[row;1H for row >= dividerRow causes TinTin++ to scroll
// the ENTIRE screen upward, not just position the cursor.  Each such command
// in appendRoomRedraw accumulates drift — after N calls the room display is
// N rows too high.
//
// Fix: appendRoomRedraw uses CUU (cursor-up) from promptRow to reach dividerRow.
// CUU outside the scroll region moves freely without triggering any scroll.
// InitScreen uses \033[dividerRow;1H only once (unavoidable) and then advances
// to promptRow via CUD so all subsequent operations start from promptRow.
//
// Precondition:  conn.height must be > 0.
// Postcondition: Terminal configured for three-region layout; cursor at promptRow.
func (c *Conn) InitScreen() error {
	c.mu.Lock()
	h := c.height
	c.mu.Unlock()

	lo := newRoomLayout(h)

	var buf strings.Builder
	buf.WriteString("\033[?25l")     // hide cursor
	buf.WriteString("\033[2J\033[H") // clear screen, home

	// Set scroll region to rows 1..scrollBottom.
	fmt.Fprintf(&buf, "\033[1;%dr", lo.scrollBottom)

	// Position at dividerRow and erase to end of screen, then advance to
	// promptRow via CUD.  The one \033[dividerRow;1H] here is unavoidable but
	// harmless: the screen was just cleared so any TinTin++ scroll shifts only
	// blank rows.  CUD from dividerRow (outside scroll region) reaches promptRow.
	// Postcondition: cursor at promptRow so appendRoomRedraw can use CUU.
	fmt.Fprintf(&buf, "\033[%d;1H\033[J\033[%dB", lo.dividerRow, roomRegionRows+1)

	buf.WriteString("\033[?25h") // show cursor

	return c.writeRaw(buf.String())
}

// WriteRoom renders content into the pinned room region.
//
// Uses CUU (cursor-up) from promptRow to reach dividerRow — no absolute
// positioning at or below dividerRow so TinTin++ never performs spurious scrolls.
//
// Precondition:  conn.splitScreen must be true; conn.width > 0; cursor at promptRow.
// Postcondition: Room divider and content rows are updated; cursor at promptRow.
func (c *Conn) WriteRoom(content string) error {
	c.mu.Lock()
	w := c.width
	c.roomBuf = content // cache for WriteConsole to redraw the pinned region
	c.mu.Unlock()

	// Normalize CRLF to LF before splitting so RenderRoomView's \r\n endings
	// don't produce trailing \r artifacts or a spurious blank first row.
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	appendRoomRedraw(&buf, lines, w, divider)
	// Advance one more row from lastRow to promptRow.
	buf.WriteString("\r\n")

	return c.writeRaw(buf.String())
}

// WriteConsole writes a message into the VT100 scroll region and redraws
// the room divider, room content, and prompt row.
// The player's partially-typed input is preserved.
//
// Operation order:
//  1. Redraw room via CUU from promptRow (no absolute pos at dividerRow).
//  2. Advance to promptRow; save cursor (ESC 7 = DECSC).
//  3. Jump to scrollBottom (safe: within scroll region); append message lines.
//  4. Restore cursor to promptRow (ESC 8 = DECRC); redraw input.
//
// Precondition:  conn.splitScreen must be true; conn.height must be > 0; cursor at promptRow.
// Postcondition: Message appears in scroll region; room and prompt are redrawn; cursor at promptRow.
func (c *Conn) WriteConsole(text string) error {
	c.mu.Lock()
	h := c.height
	w := c.width
	input := c.inputBuf
	room := c.roomBuf
	c.mu.Unlock()

	lo := newRoomLayout(h)
	divider := strings.Repeat("═", w)

	// Redraw room display first (from promptRow via CUU — no TinTin++ scroll).
	var roomLines []string
	if room != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(room, "\r\n", "\n"), "\r", "")
		roomLines = strings.Split(strings.TrimSpace(normalized), "\n")
	}

	var buf strings.Builder
	appendRoomRedraw(&buf, roomLines, w, divider)
	// Advance from lastRow to promptRow, then save cursor position (DECSC).
	buf.WriteString("\r\n\x1b7")

	// Position at scrollBottom (safe: within scroll region) and append message.
	// Strip trailing newlines: a trailing \n would produce an empty wrapText
	// element causing a second \r\n scroll that leaves a blank gap.
	trimmed := strings.TrimRight(text, "\r\n")
	fmt.Fprintf(&buf, "\033[%d;1H", lo.scrollBottom)
	for _, line := range wrapText(trimmed, w) {
		buf.WriteString("\r\n")
		buf.WriteString(line)
	}

	// Restore cursor to promptRow (DECRC) and redraw input.
	buf.WriteString("\x1b8\033[2K")
	buf.WriteString(input)

	return c.writeRaw(buf.String())
}

// appendRoomRedraw writes the divider and room content rows into buf.
//
// Navigates from promptRow to dividerRow using CUU (cursor-up N rows).
// CUU outside the scroll region moves freely without causing any scroll —
// this avoids the TinTin++ bug where \033[dividerRow;1H scrolls the entire
// screen upward once per call, accumulating drift over multiple calls.
//
// Precondition:  cursor must be at promptRow (the last row); lines must be
//
//	split and normalized (no \r\n).
//
// Postcondition: cursor is at lastRow (one row above promptRow).
func appendRoomRedraw(buf *strings.Builder, lines []string, w int, divider string) {
	// Move up from promptRow to dividerRow (roomRegionRows+1 rows), then col 1.
	fmt.Fprintf(buf, "\033[%dA\r", roomRegionRows+1)
	buf.WriteString(divider)
	for i := 0; i < roomRegionRows; i++ {
		buf.WriteString("\r\n\033[2K")
		if i < len(lines) {
			line := lines[i]
			if w > 0 && visualWidth(line) > w {
				line = truncateToVisualWidth(line, w)
			}
			buf.WriteString(line)
		}
	}
}

// WritePromptSplit writes the prompt and buffered input at the prompt row (row H).
//
// Precondition:  conn.splitScreen must be true; conn.height must be > 0.
// Postcondition: Prompt appears at row H with cursor after prompt+input.
func (c *Conn) WritePromptSplit(prompt string) error {
	c.mu.Lock()
	input := c.inputBuf
	c.mu.Unlock()

	// Cursor is always at promptRow when WritePromptSplit is called
	// (WriteRoom, WriteConsole, and ReadLineSplit all leave cursor there).
	// Write prompt in-place: \r to col 1, erase line, write prompt+input.
	// No navigation needed — avoids \r\n scroll ops.
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
