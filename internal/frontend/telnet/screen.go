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
// Room divider  = row dividerRow.
// Room content  = rows firstRow..lastRow  (roomRegionRows rows).
// Prompt        = row h.
//
// All room/divider/prompt rows are BELOW the scroll region so that cursor
// positioning works regardless of whether the terminal restricts movement to
// the scroll region.
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
	buf.WriteString("\033[?25l")    // hide cursor
	buf.WriteString("\033[2J\033[H") // clear screen, home

	// Set scroll region to rows 1..scrollBottom.  The cursor moves to home
	// after DECSTBM; home is row 1 (DECOM off, which is the default).
	fmt.Fprintf(&buf, "\033[1;%dr", lo.scrollBottom)

	// Draw room divider and blank the room content rows.
	// These rows are all BELOW the scroll region so absolute positioning works.
	fmt.Fprintf(&buf, "\033[%d;1H%s", lo.dividerRow, divider)
	for row := lo.firstRow; row <= lo.lastRow; row++ {
		fmt.Fprintf(&buf, "\033[%d;1H\033[2K", row)
	}

	// Position cursor at input row.
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	buf.WriteString("\033[?25h") // show cursor

	return c.writeRaw(buf.String())
}

// WriteRoom renders content into the pinned room region.
// The room region sits BELOW the scroll region (rows firstRow..lastRow) so
// that absolute cursor positioning is never restricted by the scroll region.
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
	// Disable auto-wrap so a full-width line does not leave a pending-wrap state
	// that would shift subsequent absolute cursor-position commands by one row.
	buf.WriteString("\033[?7l")

	// Redraw room divider (absolute positioning to dividerRow works in TinTin++).
	fmt.Fprintf(&buf, "\033[%d;1H%s", lo.dividerRow, divider)

	// Write each room content row with absolute positioning.
	// DECAWM is off so full-width lines clip at column W without pending-wrap.
	for i := 0; i < roomRegionRows; i++ {
		fmt.Fprintf(&buf, "\033[%d;1H\033[2K", lo.firstRow+i)
		if i < len(lines) {
			line := lines[i]
			if w > 0 && visualWidth(line) > w {
				line = truncateToVisualWidth(line, w)
			}
			buf.WriteString(line)
		}
	}

	// Leave cursor at prompt row; WritePromptSplit (always called after) writes it.
	fmt.Fprintf(&buf, "\033[%d;1H", lo.promptRow)
	// Re-enable auto-wrap for scroll-region console output.
	buf.WriteString("\033[?7h")

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

	// Redraw room divider and content using appendRoomRedraw (handles divider + \r\n advancement).
	if room != "" {
		appendRoomRedraw(&buf, room, w, lo, divider)
	} else {
		fmt.Fprintf(&buf, "\033[%d;1H%s", lo.dividerRow, divider)
	}

	// Redraw input row with preserved input.
	fmt.Fprintf(&buf, "\033[%d;1H\033[2K%s", lo.promptRow, input)

	return c.writeRaw(buf.String())
}

// appendRoomRedraw writes room content rows into buf.
// All rows are below the scroll region so absolute positioning is reliable.
// appendRoomRedraw writes the divider and room content rows into buf.
// Caller must have already written any preceding content; this function
// positions at dividerRow and uses \r\n advancement to avoid pending-wrap
// interaction with absolute cursor-position commands.
func appendRoomRedraw(buf *strings.Builder, content string, w int, lo roomLayout, divider string) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")

	buf.WriteString("\033[?7l")
	fmt.Fprintf(buf, "\033[%d;1H%s", lo.dividerRow, divider)
	for i := 0; i < roomRegionRows; i++ {
		fmt.Fprintf(buf, "\033[%d;1H\033[2K", lo.firstRow+i)
		if i < len(lines) {
			line := lines[i]
			if w > 0 && visualWidth(line) > w {
				line = truncateToVisualWidth(line, w)
			}
			buf.WriteString(line)
		}
	}
	buf.WriteString("\033[?7h")
}

// WritePromptSplit writes the prompt and buffered input at the prompt row (row H).
//
// Precondition:  conn.splitScreen must be true; conn.height must be > 0.
// Postcondition: Prompt appears at row H with cursor after prompt+input.
func (c *Conn) WritePromptSplit(prompt string) error {
	c.mu.Lock()
	h := c.height
	input := c.inputBuf
	c.mu.Unlock()

	lo := newRoomLayout(h)
	var buf strings.Builder
	fmt.Fprintf(&buf, "\033[%d;1H\033[2K%s%s", lo.promptRow, prompt, input)

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
