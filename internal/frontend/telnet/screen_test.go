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

func TestInitScreen_ContainsRequiredSequences(t *testing.T) {
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.InitScreen() }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, "\033[?25l")                            // hide cursor
	assert.Contains(t, out, "\033[2J")                              // clear screen
	assert.Contains(t, out, fmt.Sprintf("\033[8;%dr", 22))          // scroll region 8..22 (H-2=22)
	assert.Contains(t, out, "\033[7;1H")                            // top divider row 7
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", 23))          // bottom divider row H-1=23
	assert.Contains(t, out, "\033[?25h")                            // show cursor
	assert.Contains(t, out, strings.Repeat("═", 80))                // divider chars
}

func TestWriteRoom_StartsWithSaveCursorAndRow1(t *testing.T) {
	conn, client := newSplitConn(t, 80, 24)
	go func() { _ = conn.WriteRoom("Nexus Hub\nA bustling crossroads.\nExits: N S E W") }()
	out := readAll(t, client, 500*time.Millisecond)

	assert.Contains(t, out, "\033[s")    // save cursor
	assert.Contains(t, out, "\033[1;1H") // move to row 1
	assert.Contains(t, out, "Nexus Hub")
	assert.Contains(t, out, "\033[u") // restore cursor
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
