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
	fmt.Fprintf(&buf, "\033[8;%dr", h-2)              // set scroll region rows 8..(H-2)
	fmt.Fprintf(&buf, "\033[7;1H%s", divider)         // top divider at row 7
	fmt.Fprintf(&buf, "\033[%d;1H%s", h-1, divider)  // bottom divider at row H-1
	fmt.Fprintf(&buf, "\033[%d;1H", h)                // move to input row
	buf.WriteString("\033[?25h")                       // show cursor

	return c.writeRaw(buf.String())
}

// WriteRoom renders content into the pinned room region (rows 1-6).
// Content is split on newlines; only the first roomRegionRows lines are used.
// The cursor is saved before and restored after writing.
//
// Precondition: conn.splitScreen must be true.
// Postcondition: Rows 1-6 contain the room content; cursor is restored to prior position.
func (c *Conn) WriteRoom(content string) error {
	c.mu.Lock()
	w := c.width
	c.mu.Unlock()

	lines := strings.Split(content, "\n")

	var buf strings.Builder
	buf.WriteString("\033[s")    // save cursor
	buf.WriteString("\033[1;1H") // move to row 1

	for row := 0; row < roomRegionRows; row++ {
		buf.WriteString("\033[2K") // clear line
		if row < len(lines) {
			line := lines[row]
			if w > 0 && len(line) > w {
				line = line[:w]
			}
			buf.WriteString(line)
		}
		if row < roomRegionRows-1 {
			buf.WriteString("\r\n")
		}
	}

	buf.WriteString("\033[u") // restore cursor

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
	c.mu.Unlock()

	divider := strings.Repeat("═", w)

	var buf strings.Builder
	// Position at bottom of scroll region and scroll up
	fmt.Fprintf(&buf, "\033[%d;1H\r\n", h-2)
	// Write wrapped text
	for _, line := range wrapText(text, w) {
		buf.WriteString(line)
		buf.WriteString("\r\n")
	}
	// Redraw bottom divider
	fmt.Fprintf(&buf, "\033[%d;1H%s", h-1, divider)
	// Redraw input row with preserved input
	fmt.Fprintf(&buf, "\033[%d;1H\033[2K%s", h, input)

	return c.writeRaw(buf.String())
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

// wrapText splits text into lines of at most width bytes (ASCII assumption).
// Lines already shorter than width are returned unchanged.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var result []string
	for _, line := range strings.Split(text, "\n") {
		for len(line) > width {
			result = append(result, line[:width])
			line = line[width:]
		}
		result = append(result, line)
	}
	return result
}
