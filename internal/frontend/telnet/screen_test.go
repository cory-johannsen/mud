package telnet

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"net"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newSplitConn creates a Conn with split-screen fields pre-set for testing.
func newSplitConn(t *testing.T, w, h int) (*Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	conn := NewConn(server, 2*time.Second, 2*time.Second)
	conn.mu.Lock()
	conn.width = w
	conn.height = h
	conn.splitScreen = true
	conn.mu.Unlock()
	t.Cleanup(func() {
		client.Close()
		conn.Close()
	})
	return conn, client
}

// readAll drains bytes from client until read deadline expires.
func readAll(t *testing.T, client net.Conn, d time.Duration) string {
	t.Helper()
	_ = client.SetReadDeadline(time.Now().Add(d))
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := client.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}

// For H=24: firstRow=1, lastRow=8, dividerRow=9, scrollTop=10, scrollBottom=24, promptRow=24.

func TestInitScreen_ContainsRequiredSequences(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	conn, client := newSplitConn(t, W, H)
	go func() { _ = conn.InitScreen() }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, "\033[?25l")                                          // hide cursor
	assert.Contains(t, out, "\033[2J")                                            // clear screen
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.promptRow))              // safe abs pos to promptRow (before scroll region)
	assert.Contains(t, out, "\x1b7")                                              // DECSC: save cursor at promptRow
	assert.Contains(t, out, fmt.Sprintf("\033[%d;%dr", lo.scrollTop, lo.scrollBottom)) // scroll region scrollTop..h
	assert.Contains(t, out, "\x1b8")                                              // DECRC: restore cursor to promptRow
	assert.Contains(t, out, "\033[?25h")                                          // show cursor
	// InitScreen does NOT draw the divider — WriteRoom is the sole divider drawer.
	assert.NotContains(t, out, strings.Repeat("═", W))
}

func TestWriteRoom_DrawsDividerAndContent(t *testing.T) {
	const W, H = 80, 24
	conn, client := newSplitConn(t, W, H)
	go func() { _ = conn.WriteRoom("Nexus Hub\nA bustling crossroads.\nExits: N S E W") }()
	out := readAll(t, client, 500*time.Millisecond)

	// No ANSI cursor save/restore — WriteRoom uses absolute positioning only.
	assert.NotContains(t, out, "\033[s")
	assert.NotContains(t, out, "\033[u")

	// Room drawn via absolute position to row 1 (above scroll region, safe).
	assert.Contains(t, out, "\033[1;1H")

	// Divider must be drawn.
	assert.Contains(t, out, strings.Repeat("═", W))

	// Content must be present.
	assert.Contains(t, out, "Nexus Hub")
}

func TestWriteRoom_ClearsExactly9Lines(t *testing.T) {
	// appendRoomRedraw clears roomRegionRows content lines + 1 divider line = roomRegionRows+1.
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.WriteRoom("line1\nline2") }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Equal(t, roomRegionRows+1, strings.Count(out, "\033[2K"),
		"WriteRoom must clear exactly roomRegionRows+1 lines (room content + divider)")
}

func TestWriteConsole_ContainsText(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	conn, client := newSplitConn(t, W, H)
	go func() { _ = conn.WriteConsole("You swing at the goblin.") }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, "You swing at the goblin.")
	// Room redrawn via abs pos to row 1 (safe: above scroll region).
	assert.Contains(t, out, "\033[1;1H")
	// WriteConsole navigates to promptRow (= scrollBottom) to write messages.
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.promptRow))
}

func TestWritePromptSplit_AtRowH(t *testing.T) {
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.WritePromptSplit("> ") }()
	out := readAll(t, client, 500*time.Millisecond)

	// Prompt is written in-place: \r to col 1, erase line, prompt text.
	assert.Contains(t, out, "\r\033[2K> ")
}

func TestPropertyWriteRoom_AlwaysRoomRegionRowsPlusOneClearSequences(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		lineCount := rapid.IntRange(0, 20).Draw(rt, "lines")
		var lines []string
		for i := 0; i < lineCount; i++ {
			lines = append(lines, "line")
		}

		conn, client := newSplitConn(t, 80, 24)
		go func() {
			_ = conn.WriteRoom(strings.Join(lines, "\n"))
			conn.Close() // Close after write so readAll drains immediately via EOF
		}()
		out := readAll(t, client, 2*time.Second)

		require.Equal(t, roomRegionRows+1, strings.Count(out, "\033[2K"),
			"WriteRoom must always clear exactly roomRegionRows+1 lines regardless of content")
	})
}
