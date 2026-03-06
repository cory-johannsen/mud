package telnet

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

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

// For H=24: scrollBottom=16, dividerRow=17, firstRow=18, lastRow=23, promptRow=24.

func TestInitScreen_ContainsRequiredSequences(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	conn, client := newSplitConn(t, W, H)
	go func() { _ = conn.InitScreen() }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, "\033[?25l")                                           // hide cursor
	assert.Contains(t, out, "\033[2J")                                             // clear screen
	assert.Contains(t, out, fmt.Sprintf("\033[1;%dr", lo.scrollBottom))            // scroll region 1..scrollBottom
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.dividerRow))              // room divider row
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.promptRow))               // cursor at prompt row
	assert.Contains(t, out, "\033[?25h")                                           // show cursor
	assert.Contains(t, out, strings.Repeat("═", W))                               // divider chars
}

func TestWriteRoom_WritesToRowsBelowScrollRegion(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	conn, client := newSplitConn(t, W, H)
	go func() { _ = conn.WriteRoom("Nexus Hub\nA bustling crossroads.\nExits: N S E W") }()
	out := readAll(t, client, 500*time.Millisecond)

	// No cursor save/restore — not needed with below-scroll-region layout.
	assert.NotContains(t, out, "\033[s")
	assert.NotContains(t, out, "\033[u")

	// Room divider must be redrawn.
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.dividerRow))
	assert.Contains(t, out, strings.Repeat("═", W))

	// Content must appear; room rows advanced via \r\n from dividerRow (no absolute firstRow position).
	assert.Contains(t, out, "Nexus Hub")
	assert.NotContains(t, out, fmt.Sprintf("\033[%d;1H", lo.firstRow), "room rows use \\r\\n not absolute position")

	// Cursor left at prompt row.
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.promptRow))
}

func TestWriteRoom_ClearsExactly6Lines(t *testing.T) {
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.WriteRoom("line1\nline2") }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Equal(t, 6, strings.Count(out, "\033[2K"), "WriteRoom must clear exactly 6 lines")
}

func TestWriteConsole_ContainsText(t *testing.T) {
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.WriteConsole("You swing at the goblin.") }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, "You swing at the goblin.")
	// Prompt row must be referenced
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", 24))
}

func TestWritePromptSplit_AtRowH(t *testing.T) {
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.WritePromptSplit("> ") }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", 24))
	assert.Contains(t, out, "> ")
}

func TestPropertyWriteRoom_Always6LineClearSequences(t *testing.T) {
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

		require.Equal(t, 6, strings.Count(out, "\033[2K"),
			"WriteRoom must always clear exactly 6 lines regardless of content")
	})
}
