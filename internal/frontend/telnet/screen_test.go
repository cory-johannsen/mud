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

	assert.Contains(t, out, "\033[?25l")                                // hide cursor
	assert.Contains(t, out, "\033[2J")                                 // clear screen
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", lo.promptRow))  // abs pos to promptRow
	assert.Contains(t, out, "\033[?25h")                               // show cursor
	// No DECSTBM (scroll region) — full-screen default avoids TinTin++ DECOM offset quirk.
	assert.NotContains(t, out, "\033[?7l")
	assert.NotContains(t, out, "\x1b7") // no DECSC
	assert.NotContains(t, out, "\x1b8") // no DECRC
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

func TestWriteRoom_ClearsRoomRegionRowsPlusOne(t *testing.T) {
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

// TestWriteConsole_ClearsHotbarRowBeforeScroll is the regression test for BUG-38.
// Before this fix, WriteConsole scrolled the terminal without first clearing row h-1 (hotbar),
// so the hotbar content scrolled into the visible console region on every message.
// The fix: clear the hotbar row BEFORE issuing any \r\n scrolls so that only blank content
// scrolls upward, not the hotbar text.
func TestWriteConsole_ClearsHotbarRowBeforeScroll(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	conn, client := newSplitConn(t, W, H)

	// Pre-populate hotbar with visible content that would be detectable if scrolled.
	conn.mu.Lock()
	conn.hotbarBuf = [10]string{"stride", "", "", "", "", "", "", "", "", ""}
	conn.mu.Unlock()

	go func() { _ = conn.WriteConsole("combat message") }()
	out := readAll(t, client, 500*time.Millisecond)

	// The hotbar row (h-1) must be cleared BEFORE any \r\n scroll.
	hotbarClear := fmt.Sprintf("\033[%d;1H\033[2K", lo.hotbarRow)
	require.Contains(t, out, hotbarClear, "WriteConsole must clear hotbar row before scrolling")

	// The hotbar clear must precede the prompt-row positioning (which triggers the scroll).
	promptPos := fmt.Sprintf("\033[%d;1H\033[2K", lo.promptRow)
	clearIdx := strings.Index(out, hotbarClear)
	promptIdx := strings.Index(out, promptPos)
	require.Less(t, clearIdx, promptIdx,
		"hotbar row clear must come before the prompt-row scroll position")
}

func TestWriteHotbar_RendersAtRowHMinus1(t *testing.T) {
	const W, H = 80, 24
	conn, client := newSplitConn(t, W, H)
	var slots [10]string
	slots[0] = "look"
	slots[9] = "status"
	go func() { _ = conn.WriteHotbar(slots) }()
	out := readAll(t, client, 500*time.Millisecond)

	// Must position at row H-1 = 23.
	assert.Contains(t, out, fmt.Sprintf("\033[%d;1H", H-1))
	// Must contain slot labels.
	assert.Contains(t, out, "loo") // truncated "look"
}

func TestWriteHotbar_EmptySlotsRenderDash(t *testing.T) {
	const W, H = 80, 24
	conn, client := newSplitConn(t, W, H)
	go func() { _ = conn.WriteHotbar([10]string{}) }()
	out := readAll(t, client, 500*time.Millisecond)
	assert.Contains(t, out, "---")
}

func TestWriteHotbar_RendersActivationKey0ForSlot10(t *testing.T) {
	const W, H = 80, 24
	conn, client := newSplitConn(t, W, H)
	var slots [10]string
	slots[9] = "status"
	go func() { _ = conn.WriteHotbar(slots) }()
	out := readAll(t, client, 500*time.Millisecond)
	// Slot 10 activation key is "0".
	assert.Contains(t, out, "0:")
}

func TestConsoleHeight_ShrinksWithHotbar(t *testing.T) {
	const W, H = 80, 24
	conn, _ := newSplitConn(t, W, H)
	// With hotbar: console height = H - roomRegionRows - 3
	// = 24 - 10 - 3 = 11
	assert.Equal(t, 11, conn.consoleHeight())
}

func TestPropertyWriteHotbar_FitsWithinWidth(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		w := rapid.IntRange(40, 220).Draw(rt, "width")
		h := rapid.IntRange(20, 50).Draw(rt, "height")
		var slots [10]string
		for i := range slots {
			slots[i] = rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz"))).Draw(rt, fmt.Sprintf("slot%d", i))
		}

		conn, client := newSplitConn(t, w, h)
		go func() {
			_ = conn.WriteHotbar(slots)
			conn.Close()
		}()
		out := readAll(t, client, 2*time.Second)

		// Extract the content line (after the cursor positioning sequence).
		// The hotbar line itself must fit within w visible characters.
		// Strip ANSI for width check.
		lines := strings.Split(StripANSI(out), "\n")
		for _, line := range lines {
			line = strings.TrimRight(line, "\r")
			if strings.Contains(line, "[") {
				assert.LessOrEqual(t, len(line), w,
					"hotbar line must not exceed terminal width")
			}
		}
	})
}

func TestPropertyWriteHotbar_AtLeastOneSlotWhenWidthAtLeast40(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		w := rapid.IntRange(40, 220).Draw(rt, "width")
		h := rapid.IntRange(20, 50).Draw(rt, "height")

		conn, client := newSplitConn(t, w, h)
		var slots [10]string
		for i := range slots {
			slots[i] = "abc"
		}
		go func() {
			_ = conn.WriteHotbar(slots)
			conn.Close()
		}()
		out := readAll(t, client, 2*time.Second)
		// At least one slot segment [N:abc] must appear.
		assert.Contains(t, out, ":", "at least one slot must be rendered at width >= 40")
	})
}

func TestPropertyWriteHotbar_AllTenSlotsWhenWidthAtLeast90(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		w := rapid.IntRange(90, 220).Draw(rt, "width")
		h := rapid.IntRange(20, 50).Draw(rt, "height")

		conn, client := newSplitConn(t, w, h)
		var slots [10]string
		for i := range slots {
			slots[i] = "abc"
		}
		go func() {
			_ = conn.WriteHotbar(slots)
			conn.Close()
		}()
		out := readAll(t, client, 2*time.Second)
		plain := StripANSI(out)
		// All 10 slot activation keys must appear.
		for _, key := range []string{"1:", "2:", "3:", "4:", "5:", "6:", "7:", "8:", "9:", "0:"} {
			assert.Contains(t, plain, key, "all 10 slots must be rendered when width >= 90")
		}
	})
}
