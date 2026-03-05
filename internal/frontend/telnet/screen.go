package telnet

import (
	"fmt"
	"strings"
	"time"
)

const roomRegionRows = 6

// InitScreen initializes the split-screen layout.
// Sends: hide cursor, clear screen, set scroll region (rows 8..H-2),
// draw top divider (row 7), draw bottom divider (row H-1), show cursor.
//
// Precondition: conn.splitScreen must be true; conn.width and conn.height must be > 0.
// Postcondition: Terminal is configured for three-region layout.
func (c *Conn) InitScreen() error {
	c.mu.Lock()
	w := c.width
	h := c.height
	c.mu.Unlock()

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	buf.WriteString("\033[?25l")                      // hide cursor
	buf.WriteString("\033[2J\033[H")                  // clear screen, home
	buf.WriteString("\033[?6l")                       // DECOM off — cursor addresses are screen-absolute
	fmt.Fprintf(&buf, "\033[8;%dr", h-2)              // set scroll region rows 8..(H-2)
	fmt.Fprintf(&buf, "\033[7;1H%s", divider)         // top divider at row 7
	fmt.Fprintf(&buf, "\033[%d;1H%s", h-1, divider)  // bottom divider at row H-1
	fmt.Fprintf(&buf, "\033[%d;1H", h)                // move to input row
	buf.WriteString("\033[?25h")                       // show cursor

	return c.writeRaw(buf.String())
}

// WriteRoom renders content into the pinned room region (rows 1-6).
// Content is split on newlines; only the first roomRegionRows lines are used.
// DECOM is forced off so absolute row addresses always refer to the screen top.
// Cursor is left at row H; WritePromptSplit must always be called after.
//
// Precondition: conn.splitScreen must be true; conn.width and conn.height must be > 0.
// Postcondition: Rows 1-6 contain the room content; row 7 holds the top divider; cursor is at row H.
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

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	buf.WriteString("\033[?6l") // DECOM off — not sandwiched in save/restore so it sticks

	for row := 0; row < roomRegionRows; row++ {
		fmt.Fprintf(&buf, "\033[%d;1H", row+1) // absolute row positioning
		buf.WriteString("\033[2K")              // clear line
		if row < len(lines) {
			line := lines[row]
			if w > 0 && visualWidth(line) > w {
				line = truncateToVisualWidth(line, w)
			}
			buf.WriteString(line)
		}
	}
	// Redraw top divider (defensive — WriteConsole may have corrupted it)
	fmt.Fprintf(&buf, "\033[7;1H%s", divider)
	// Leave cursor at input row; WritePromptSplit (always called after) finalises it.
	fmt.Fprintf(&buf, "\033[%d;1H", h)

	return c.writeRaw(buf.String())
}

// WriteConsole writes a message into the VT100 scroll region and redraws
// the bottom divider and prompt row. The player's partially-typed input is preserved.
//
// Precondition: conn.splitScreen must be true; conn.height must be > 0.
// Postcondition: Message appears in console; prompt is redrawn at row H.
func (c *Conn) WriteConsole(text string) error {
	c.mu.Lock()
	h := c.height
	w := c.width
	input := c.inputBuf
	room := c.roomBuf
	c.mu.Unlock()

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	// Strip trailing newlines from the text before wrapping. A trailing \n
	// would produce an empty element from wrapText, causing a second \r\n
	// scroll that leaves a blank row between the message and the bottom divider.
	trimmed := strings.TrimRight(text, "\r\n")
	// Position at bottom of scroll region; scroll up before each line so the
	// line lands at the bottom of the region without leaving a trailing blank.
	fmt.Fprintf(&buf, "\033[%d;1H", h-2)
	for _, line := range wrapText(trimmed, w) {
		buf.WriteString("\r\n")
		buf.WriteString(line)
	}
	// Redraw bottom divider
	fmt.Fprintf(&buf, "\033[%d;1H%s", h-1, divider)
	// Redraw input row with preserved input
	fmt.Fprintf(&buf, "\033[%d;1H\033[2K%s", h, input)

	// Redraw the pinned room region (rows 1-6) and top divider (row 7).
	// This corrects any corruption caused by spurious terminal scrolls
	// (e.g., from client-side echo of \r\n at the last terminal row).
	if room != "" {
		appendRoomRedraw(&buf, room, w)
	}

	return c.writeRaw(buf.String())
}

// appendRoomRedraw writes the room region (rows 1-6) and top divider (row 7)
// into buf without moving the cursor from its post-input position.
// It uses cursor save/restore so the caller's cursor position is unchanged.
func appendRoomRedraw(buf *strings.Builder, content string, w int) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")

	divider := strings.Repeat("═", w)

	buf.WriteString("\033[?6l") // DECOM off — not sandwiched in save/restore so it sticks

	for row := 0; row < roomRegionRows; row++ {
		fmt.Fprintf(buf, "\033[%d;1H", row+1) // absolute row positioning
		buf.WriteString("\033[2K")             // clear line
		if row < len(lines) {
			line := lines[row]
			if w > 0 && visualWidth(line) > w {
				line = truncateToVisualWidth(line, w)
			}
			buf.WriteString(line)
		}
	}

	// Redraw top divider at row 7
	fmt.Fprintf(buf, "\033[7;1H%s", divider)
	// Caller (WriteConsole) repositions cursor to row H via its own fmt.Fprintf.
}

// WritePromptSplit writes the prompt and buffered input at the input row (row H).
//
// Precondition: conn.splitScreen must be true; conn.height must be > 0.
// Postcondition: Prompt appears at row H with cursor after prompt+input.
func (c *Conn) WritePromptSplit(prompt string) error {
	c.mu.Lock()
	h := c.height
	input := c.inputBuf
	c.mu.Unlock()

	var buf strings.Builder
	fmt.Fprintf(&buf, "\033[%d;1H\033[2K%s%s", h, prompt, input)

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
// Precondition: s must be a valid string; the connection must be open.
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
