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
// Room divider  = row dividerRow  (= scrollBottom+1, the one safe absolute position).
// Room content  = rows firstRow..lastRow  (roomRegionRows rows).
// Prompt        = row h.
//
// TinTin++ quirk: \033[row;1H for row > scrollBottom+1 causes row-scrollBottom-1
// scroll operations instead of positioning.  Only \033[dividerRow;1H] (row =
// scrollBottom+1) is safe.  All other below-scroll rows are reached via \r\n
// from dividerRow.
//
// Precondition:  conn.width and conn.height must be > 0.
// Postcondition: Terminal is configured for three-region layout.
func (c *Conn) InitScreen() error {
	c.mu.Lock()
	w := c.width
	h := c.height
	c.mu.Unlock()

	lo := newRoomLayout(h)
	divider := strings.Repeat("═", w)

	var buf strings.Builder
	buf.WriteString("\033[?25l")     // hide cursor
	buf.WriteString("\033[2J\033[H") // clear screen, home

	// Set scroll region to rows 1..scrollBottom.
	fmt.Fprintf(&buf, "\033[1;%dr", lo.scrollBottom)

	// Position at dividerRow (safe: scrollBottom+1 = 0 scroll ops), then erase
	// from there to end of screen — clears all room rows and prompt row in one
	// command without any \r\n that could trigger TinTin++ scroll ops.
	// Then write the divider over the just-cleared dividerRow.
	// Cursor ends at end of divider line; WriteRoom will reposition.
	fmt.Fprintf(&buf, "\033[%d;1H\033[J%s", lo.dividerRow, divider)

	buf.WriteString("\033[?25h") // show cursor

	return c.writeRaw(buf.String())
}

// WriteRoom renders content into the pinned room region.
//
// All rows are reached from dividerRow via \r\n.  \033[dividerRow;1H is the
// only absolute cursor-position command used; it is the one safe position
// (scrollBottom+1 = 0 TinTin++ scroll ops).  All other room rows and the
// prompt row are reached by advancing with \r\n, which does not scroll when
// the cursor is below the scroll region.
//
// Precondition:  conn.splitScreen must be true; conn.width and conn.height > 0.
// Postcondition: Room divider and content rows are updated; cursor at promptRow.
func (c *Conn) WriteRoom(content string) error {
	c.mu.Lock()
	w := c.width
	h := c.height
	c.roomBuf = content // cache for WriteConsole to redraw the pinned region
	c.mu.Unlock()

	// Normalize CRLF to LF before splitting so RenderRoomView's \r\n endings
	// don't produce trailing \r artifacts or a spurious blank first row.
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")

	lo := newRoomLayout(h)
	divider := strings.Repeat("═", w)

	var buf strings.Builder
	appendRoomRedraw(&buf, lines, w, lo, divider)
	// Advance one more row from lastRow to promptRow.
	buf.WriteString("\r\n")

	return c.writeRaw(buf.String())
}

// WriteConsole writes a message into the VT100 scroll region and redraws
// the room divider, room content, and prompt row.
// The player's partially-typed input is preserved.
//
// Precondition:  conn.splitScreen must be true; conn.height must be > 0.
// Postcondition: Message appears in scroll region; room and prompt are redrawn.
func (c *Conn) WriteConsole(text string) error {
	c.mu.Lock()
	h := c.height
	w := c.width
	input := c.inputBuf
	room := c.roomBuf
	c.mu.Unlock()

	lo := newRoomLayout(h)
	divider := strings.Repeat("═", w)

	var buf strings.Builder
	// Strip trailing newlines from the text before wrapping. A trailing \n
	// would produce an empty element from wrapText, causing a second \r\n
	// scroll that leaves a blank row between the message and the divider.
	trimmed := strings.TrimRight(text, "\r\n")
	// Position at bottom of scroll region; scroll up before each line so the
	// line lands at the bottom without leaving a trailing blank.
	fmt.Fprintf(&buf, "\033[%d;1H", lo.scrollBottom)
	for _, line := range wrapText(trimmed, w) {
		buf.WriteString("\r\n")
		buf.WriteString(line)
	}

	// Redraw room divider and content. Normalize cached room to lines first.
	var roomLines []string
	if room != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(room, "\r\n", "\n"), "\r", "")
		roomLines = strings.Split(strings.TrimSpace(normalized), "\n")
	}
	appendRoomRedraw(&buf, roomLines, w, lo, divider)

	// Advance from lastRow to promptRow and redraw input.
	buf.WriteString("\r\n\033[2K")
	buf.WriteString(input)

	return c.writeRaw(buf.String())
}

// appendRoomRedraw writes the divider and room content rows into buf.
// Positions at dividerRow (the only safe absolute position below the scroll
// region) and uses \r\n to advance to each subsequent row.  \r\n below the
// scroll region advances the cursor without triggering any scroll.
// Precondition: lines are already split and normalized (no \r\n).
// Postcondition: cursor is at lastRow (roomRegionRows \r\n steps from dividerRow).
func appendRoomRedraw(buf *strings.Builder, lines []string, w int, lo roomLayout, divider string) {
	fmt.Fprintf(buf, "\033[%d;1H%s", lo.dividerRow, divider)
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
